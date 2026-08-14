// Package sqlite owns this program's SQLite connection pool.
//
// [DB] wraps a [database/sql.DB] and is modeled as a process, in the shape
// [github.com/samwisebuze/dmost/pkg/http.Server] established: [NewDB] returns a
// value that is already runnable on its defaults, exported fields carry the
// configuration an application may override before it starts, and Open/Close
// move it in and out of a running state. Construction connects to nothing, so
// it cannot fail and returns no error.
//
// The driver is modernc.org/sqlite, a CGo-free port. That is not a preference:
// docker/Dockerfile builds the daemon with CGO_ENABLED=0 so the binary is
// static enough for a scratch image, which a cgo binding would break.
//
// # The in-memory default
//
// [DefaultDSN] names a shared-cache in-memory database rather than the bare
// ":memory:" one might expect. Under a connection pool the bare form is a trap:
// SQLite gives every connection its own private, empty in-memory database, so a
// row written through one pooled connection is invisible to the next, and which
// connection a query lands on is not something the caller controls. Naming the
// database and asking for a shared cache makes all connections address one
// database.
//
// That has a second consequence Open handles: a shared-cache in-memory database
// exists only while at least one connection to it is open, and destroying it is
// how SQLite reclaims the memory. database/sql closes idle connections
// whenever it feels like it, so a pool that happens to drain to zero would take
// the data with it. Open therefore reserves one connection for the lifetime of
// an in-memory DB and holds it until Close.
//
// The shared cache also brings SQLite's table-level locking with it, which the
// bare form does not have, and the driver's answer to a conflict there is not
// an error: it calls sqlite3_unlock_notify and parks the calling goroutine
// until the connection holding the table lets go. busy_timeout is not consulted
// and nothing is returned, so contention between two connections on one
// in-memory database shows up as a stall rather than a failure — and two that
// end up waiting on each other are a deadlock the driver detects only in the
// cases sqlite3_unlock_notify can see. [DB.Open] therefore holds an in-memory
// pool to a single working connection; see the cap there.
//
// # Schema and migrations
//
// The schema is managed with golang-migrate, from .sql files under migrations/
// that [DB.Migrate] embeds into the binary — the release image is FROM scratch
// and carries no files to read at runtime. The database driver is
// golang-migrate's CGo-free one, for the same reason the SQL driver above is.
//
// [DB.Open] runs migrations itself unless [DB.AutoMigrate] is cleared, because
// the default database is created empty on every Open: a DB that came back from
// Open without a schema would pass every check this package makes and then fail
// at the first query.
//
// One hazard worth knowing before editing [DB.Migrate]: golang-migrate's
// database driver implements Close by closing the *sql.DB it was given, so
// closing a migrate.Migrate built on this pool would close the pool. Migrate
// does not call it, and says so at the point where a reader would expect it.
//
// # PRAGMAs belong in the DSN
//
// foreign_keys and busy_timeout are per-connection settings in SQLite. Issuing
// them once as an Exec after opening the pool configures exactly one pooled
// connection and silently misses every other one — and misses every connection
// the pool opens later. The driver applies DSN query parameters to each
// connection as it is created, which is the only place a per-connection setting
// can be stated once and hold for all of them, so [DB.Open] folds the
// configuration into the DSN rather than executing it.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DriverName is the name modernc.org/sqlite registers with database/sql.
//
// The registration happens in that package's init function, reached through
// retry.go's import of it — which is why no file here imports it blank.
const DriverName = "sqlite"

// Defaults applied by [NewDB]. They describe a database that needs nothing from
// its environment: it holds no files, survives no restart, and is ready the
// moment Open returns — the right thing for tests and for a daemon that has not
// been told where to keep its data yet.
const (
	// DefaultDSN is a shared-cache in-memory database. See the package doc for
	// why it is not ":memory:".
	DefaultDSN = "file:dmost?mode=memory&cache=shared"

	// DefaultBusyTimeout is how long a connection waits on a lock held by
	// another connection before giving up with SQLITE_BUSY. SQLite's own
	// default is zero — fail immediately — which turns ordinary write
	// contention into errors the caller has to retry by hand.
	DefaultBusyTimeout = 5 * time.Second

	// DefaultJournalMode is applied to file-backed databases only; see
	// [DB.JournalMode]. WAL lets readers and a writer proceed concurrently,
	// where the default rollback journal has them lock each other out.
	DefaultJournalMode = "WAL"
)

var (
	// ErrOpen reports Open called on a DB that is already running. Reopening
	// would leak the pool it is holding.
	ErrOpen = errors.New("sqlite: database already open")

	// ErrClosed reports an operation on a DB that has not been opened, or has
	// been closed since.
	ErrClosed = errors.New("sqlite: database not open")

	// ErrDSNRequired reports an empty DSN. [NewDB] supplies [DefaultDSN], so
	// reaching this means a caller cleared the field rather than never setting
	// it, and defaulting it back would ignore what they asked for.
	ErrDSNRequired = errors.New("sqlite: DSN is required")
)

// DB is a SQLite connection pool with a lifecycle.
//
// The zero value is not usable — it carries no DSN. Build one with [NewDB],
// override the exported fields as needed, then call [DB.Open]:
//
//	db := sqlite.NewDB()
//	db.DSN = "/var/lib/dmost/dmost.db"
//	if err := db.Open(); err != nil {
//		return err
//	}
//	defer db.Close()
//
// A DB is safe for concurrent use once open, because [database/sql.DB] is.
// Open and Close are not: they are lifecycle calls, made by whichever
// goroutine owns the program's startup and shutdown.
type DB struct {
	db        *sql.DB
	keepalive *sql.Conn // in-memory databases only; see Open
	ctx       context.Context
	cancel    context.CancelFunc

	// DSN is the database to connect to. It may be a filesystem path
	// ("/var/lib/dmost/dmost.db"), a SQLite URI ("file:dmost.db?mode=ro"), or
	// the in-memory form of either; a bare path is normalized to a URI so that
	// the configuration below can be attached to it.
	//
	// Parameters written here win over the fields below, on the assumption
	// that a caller who spelled out "_journal_mode=MEMORY" meant it.
	DSN string

	// ForeignKeys enables enforcement of foreign key constraints. SQLite
	// defaults this to off for backwards compatibility with databases written
	// before it supported them; there is no such history here, and a constraint
	// the engine does not enforce is worse than no constraint at all.
	//
	// Setting it false does not disable enforcement, it declines to ask for it,
	// leaving SQLite's default in place.
	ForeignKeys bool

	// AutoMigrate has [DB.Open] bring the schema up to date before it returns.
	//
	// It defaults to true because the default database is in-memory and is
	// created empty on every Open. A DB that returned from Open without a schema
	// would satisfy every check this package makes — it opens, it pings, it
	// answers — and then fail at the first query with "no such table". This
	// package's promise is that a DB is usable the moment Open returns, and on
	// an ephemeral database that promise has to include the schema.
	//
	// Clear it for a read-only DSN, or where something other than this program
	// owns the schema. [DB.Migrate] is then the caller's to run.
	AutoMigrate bool

	// BusyTimeout is how long to wait for a lock before returning
	// SQLITE_BUSY. Zero leaves SQLite's default (no wait) in place.
	BusyTimeout time.Duration

	// JournalMode is the journaling mode for file-backed databases: DELETE,
	// TRUNCATE, PERSIST, MEMORY, WAL, or OFF. Empty leaves the database's
	// current mode alone.
	//
	// It is ignored for in-memory databases, which cannot journal to a file
	// and always report "memory" — asking anyway is not an error, it is just a
	// statement executed to no effect on every connection.
	JournalMode string

	// Pool limits, passed to the corresponding [database/sql.DB] setters. A
	// zero leaves database/sql's own default in place rather than being
	// forwarded, so the zero value of this struct means "unconfigured", not
	// "no connections allowed".
	//
	// An unconfigured MaxOpenConns is capped at two for an in-memory database;
	// see [DB.Open]. Setting it to one for such a database deadlocks: the
	// keepalive connection holds the only slot and no query can ever get one.
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewDB returns a DB configured to run against a private in-memory database.
//
// It allocates and defaults; it does not connect. Nothing is opened, no file is
// touched, and no error is possible — mirroring [github.com/samwisebuze/dmost/pkg/http.NewServer],
// which builds a server without binding its listener.
func NewDB() *DB {
	return &DB{
		DSN:         DefaultDSN,
		ForeignKeys: true,
		AutoMigrate: true,
		BusyTimeout: DefaultBusyTimeout,
		JournalMode: DefaultJournalMode,
	}
}

// Open validates the configuration and puts the pool into a running state.
//
// It resolves the DSN, creates the parent directory of a file-backed database,
// opens the pool, applies the pool limits, and pings — so that a
// misconfiguration surfaces here rather than at the first query, arbitrarily
// later and in the middle of serving a request. For an in-memory database it
// also reserves the connection that keeps the database alive.
//
// A failure leaves the DB closed and reusable: whatever was opened along the
// way is released before the error returns, so a caller may correct the
// configuration and call Open again.
func (db *DB) Open() error {
	if db.db != nil {
		return ErrOpen
	}
	if strings.TrimSpace(db.DSN) == "" {
		return ErrDSNRequired
	}

	res, err := resolveDSN(db)
	if err != nil {
		return fmt.Errorf("sqlite: resolve dsn: %w", err)
	}

	// SQLite creates the database file but not the directory holding it, and
	// reports the missing directory as the same opaque "unable to open database
	// file" it uses for a dozen other causes.
	if dir := res.dir(); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("sqlite: create data directory: %w", err)
		}
	}

	pool, err := sql.Open(DriverName, res.conn)
	if err != nil {
		return fmt.Errorf("sqlite: open %q: %w", res.conn, err)
	}

	// Each setter is guarded: forwarding a zero would mean something specific
	// and wrong to database/sql (SetConnMaxLifetime(0) is "never expire",
	// SetMaxIdleConns(0) is "keep none"), where the field means "not set".
	if db.MaxOpenConns > 0 {
		pool.SetMaxOpenConns(db.MaxOpenConns)
	} else if res.memory {
		// A shared-cache in-memory database takes table-level locks, and the
		// driver does not report a conflict over one — it parks the goroutine on
		// sqlite3_unlock_notify until the holder releases, no matter what
		// busy_timeout says. Two connections contending therefore stall rather
		// than fail, and two waiting on each other deadlock. Serializing this
		// database's work onto one connection means they never contend, and an
		// ephemeral database that lives and dies with one process is not where
		// read throughput is won. A file-backed database keeps the full pool: it
		// is not in a shared cache, and WAL already lets readers and one writer
		// proceed together.
		//
		// Two, not one. The keepalive reserved below counts against this limit
		// and holds its connection for the lifetime of the DB, so a cap of one
		// would leave nothing for queries and block every call forever.
		//
		// This holds only because nothing here occupies one connection while
		// asking for another — not this package's repositories, and not
		// golang-migrate, whose version read, migration runs, and version
		// writes are strictly sequential. A future method that opened a
		// [database/sql.Tx] and then queried outside it would deadlock.
		pool.SetMaxOpenConns(2)
	}
	if db.MaxIdleConns > 0 {
		pool.SetMaxIdleConns(db.MaxIdleConns)
	}
	if db.ConnMaxLifetime > 0 {
		pool.SetConnMaxLifetime(db.ConnMaxLifetime)
	}
	if db.ConnMaxIdleTime > 0 {
		pool.SetConnMaxIdleTime(db.ConnMaxIdleTime)
	}

	db.db = pool
	db.ctx, db.cancel = context.WithCancel(context.Background())

	// sql.Open is lazy — it validates nothing and connects to nothing. This is
	// the first call that actually reaches SQLite, so it is where a bad DSN, an
	// unreadable file, or an invalid PRAGMA value is reported.
	if err := pool.PingContext(db.ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("sqlite: ping %q: %w", res.conn, err)
	}

	// Reserved for the lifetime of the DB: an in-memory database is destroyed
	// when its last connection closes, and the pool is free to drain to zero
	// whenever it is idle. See the package doc.
	if res.memory {
		conn, err := pool.Conn(db.ctx)
		if err != nil {
			_ = db.Close()
			return fmt.Errorf("sqlite: reserve keepalive connection: %w", err)
		}
		db.keepalive = conn
	}

	// After the keepalive, so an in-memory database cannot be collected out from
	// under the migration, and unwinding through Close like every step above so
	// that a failure here still leaves the DB closed and reusable.
	if db.AutoMigrate {
		if err := db.Migrate(db.ctx); err != nil {
			_ = db.Close()
			return err
		}
	}

	return nil
}

// Close shuts the pool down and releases the keepalive connection.
//
// It is idempotent and safe on a DB that was never opened or that failed to
// open, because it is called on exactly those paths: [DB.Open] unwinds through
// it, and cmd/dmostd's Main closes everything it holds when Run fails partway.
// Errors from the two closes are joined rather than short-circuited so that a
// failure to release one does not skip the other.
func (db *DB) Close() error {
	var errs []error

	// Before cancelling: the reserved connection goes back to the pool it came
	// from, and the pool must still be running to take it.
	if db.keepalive != nil {
		if err := db.keepalive.Close(); err != nil {
			errs = append(errs, fmt.Errorf("sqlite: close keepalive connection: %w", err))
		}
		db.keepalive = nil
	}

	if db.db != nil {
		if err := db.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("sqlite: close: %w", err))
		}
		db.db = nil
	}

	if db.cancel != nil {
		db.cancel()
		db.cancel = nil
	}
	db.ctx = nil

	return errors.Join(errs...)
}

// Ping verifies the database is reachable. It reports [ErrClosed] rather than
// panicking on a DB that is not open, so that a health check can call it
// without first having to know the program's startup state.
func (db *DB) Ping(ctx context.Context) error {
	pool, err := db.handle()
	if err != nil {
		return err
	}
	return pool.PingContext(ctx)
}

// handle returns the pool, or [ErrClosed] if this DB is not running.
//
// It is the one place that decides what an unopened DB means, so that the
// repositories in this package do not each reach into the field and answer that
// question for themselves. Nothing outside the package gets the handle: a
// caller that could take the pool could also close it, and the lifecycle here
// belongs to Open and Close.
func (db *DB) handle() (*sql.DB, error) {
	if db.db == nil {
		return nil, ErrClosed
	}
	return db.db, nil
}

// resolvedDSN is the outcome of normalizing [DB.DSN]: the string handed to the
// driver, plus the two facts [DB.Open] needs in order to decide what else to do.
type resolvedDSN struct {
	conn   string // what sql.Open receives, configuration folded in
	path   string // filesystem path; empty for an in-memory database
	memory bool
}

// dir returns the directory that must exist before the database can be opened,
// or "" when there is nothing to create.
func (r resolvedDSN) dir() string {
	if r.memory || r.path == "" {
		return ""
	}
	dir := filepath.Dir(r.path)
	if dir == "." || dir == string(filepath.Separator) {
		return ""
	}
	return dir
}

// dsnAliases are the driver's shorthand DSN keys, each paired with the alias it
// also accepts for the same PRAGMA. A caller who spelled either one has
// configured that PRAGMA themselves and we leave it untouched.
var dsnAliases = map[string]string{
	"_busy_timeout": "_timeout",
	"_foreign_keys": "_fk",
	"_journal_mode": "_journal",
}

// resolveDSN normalizes db.DSN to a SQLite URI and folds the configuration
// fields into it as driver query parameters.
//
// It is pure — it reads db and touches nothing else — which is what makes the
// normalization rules testable without a database.
func resolveDSN(db *DB) (resolvedDSN, error) {
	raw := strings.TrimSpace(db.DSN)

	// SQLite honors "mode", "cache", and friends only in the URI form, and the
	// driver strips a query string off anything else before passing the name
	// along. Promoting a bare path to a URI is what lets both forms carry
	// configuration.
	if !strings.HasPrefix(raw, "file:") {
		raw = "file:" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return resolvedDSN{}, err
	}

	// An absolute path parses into Path and is already unescaped; a relative
	// one or a bare database name parses into Opaque, which is not.
	path := u.Path
	if u.Opaque != "" {
		if path, err = url.PathUnescape(u.Opaque); err != nil {
			return resolvedDSN{}, err
		}
	}

	q := u.Query()

	// SQLite reads three spellings as in-memory: mode=memory, the special name
	// ":memory:", and an empty name (an anonymous temporary database, which is
	// on disk in principle but never at a path we could create).
	memory := q.Get("mode") == "memory" || path == ":memory:" || path == ""

	set := func(key, value string) {
		if value == "" {
			return
		}
		if q.Has(key) || q.Has(dsnAliases[key]) {
			return
		}
		q.Set(key, value)
	}

	if db.BusyTimeout > 0 {
		set("_busy_timeout", strconv.FormatInt(db.BusyTimeout.Milliseconds(), 10))
	}
	if db.ForeignKeys {
		set("_foreign_keys", "1")
	}
	if !memory {
		set("_journal_mode", db.JournalMode)
	}

	u.RawQuery = q.Encode()

	return resolvedDSN{conn: u.String(), path: path, memory: memory}, nil
}
