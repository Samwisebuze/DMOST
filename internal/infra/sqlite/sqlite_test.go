// These tests are in package sqlite rather than sqlite_test, unlike the rest of
// the repository. Exercising the pool means running SQL through db.db, and
// exporting an accessor for it purely so the tests could reach it would widen
// the package's API for a caller that does not exist — the repositories that
// will need the handle are going to live in this package.
package sqlite

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB returns an opened DB on an in-memory database private to this test,
// closed on cleanup.
//
// The name matters. DefaultDSN is one shared-cache database called "dmost", so
// every test using it would address the same database and see each other's
// tables — including across parallel tests, where the interleaving is not even
// deterministic.
func newTestDB(t *testing.T) *DB {
	t.Helper()

	db := NewDB()
	db.DSN = memoryDSN(t.Name())
	require.NoError(t, db.Open())
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	return db
}

func memoryDSN(name string) string {
	return fmt.Sprintf("file:%s?mode=memory&cache=shared", url.QueryEscape(name))
}

func TestNewDB(t *testing.T) {
	t.Parallel()

	t.Run("returns a runnable default configuration", func(t *testing.T) {
		t.Parallel()
		db := NewDB()

		assert.Equal(t, DefaultDSN, db.DSN)
		assert.True(t, db.ForeignKeys)
		assert.Equal(t, DefaultBusyTimeout, db.BusyTimeout)
		assert.Equal(t, DefaultJournalMode, db.JournalMode)
	})

	t.Run("connects to nothing", func(t *testing.T) {
		// Construction allocates and defaults. Anything that can fail belongs
		// in Open, where the caller gets an error to act on.
		t.Parallel()
		db := NewDB()

		assert.Nil(t, db.db)
		assert.Nil(t, db.keepalive)
		assert.ErrorIs(t, db.Ping(context.Background()), ErrClosed)
	})
}

func TestDB_Open(t *testing.T) {
	t.Parallel()

	t.Run("opens the default in-memory database", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		assert.NoError(t, db.Ping(context.Background()))
	})

	t.Run("reserves a keepalive connection for an in-memory database", func(t *testing.T) {
		t.Parallel()
		db := newTestDB(t)

		assert.NotNil(t, db.keepalive)
	})

	t.Run("does not reserve one for a file-backed database", func(t *testing.T) {
		// A file database outlives its connections, so pinning one would cost a
		// pool slot for nothing.
		t.Parallel()
		db := NewDB()
		db.DSN = filepath.Join(t.TempDir(), "dmost.db")
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		assert.Nil(t, db.keepalive)
	})

	t.Run("creates the parent directory of a file-backed database", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "var", "lib", "dmost", "dmost.db")

		db := NewDB()
		db.DSN = path
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		require.NoError(t, db.Ping(context.Background()))
		assert.FileExists(t, path)
	})

	t.Run("rejects a second Open", func(t *testing.T) {
		t.Parallel()
		db := newTestDB(t)

		assert.ErrorIs(t, db.Open(), ErrOpen)
	})

	t.Run("rejects an empty DSN", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = "  "

		assert.ErrorIs(t, db.Open(), ErrDSNRequired)
	})

	t.Run("reports an unopenable database", func(t *testing.T) {
		// mode=ro on a database that does not exist: SQLite will not create it,
		// so the failure lands on the ping inside Open rather than on the first
		// query some arbitrary time later.
		t.Parallel()
		db := NewDB()
		db.DSN = "file:" + filepath.Join(t.TempDir(), "absent.db") + "?mode=ro"

		require.Error(t, db.Open())
	})

	t.Run("leaves the DB reusable after a failure", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = "file:" + filepath.Join(t.TempDir(), "absent.db") + "?mode=ro"
		require.Error(t, db.Open())

		// Not ErrOpen: the failed attempt released everything it had acquired.
		db.DSN = memoryDSN(t.Name())
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		assert.NoError(t, db.Ping(context.Background()))
	})

	t.Run("rejects an invalid PRAGMA value", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = filepath.Join(t.TempDir(), "dmost.db")
		db.JournalMode = "NOT_A_MODE"

		require.Error(t, db.Open())
	})
}

func TestDB_Close(t *testing.T) {
	t.Parallel()

	t.Run("is safe on a DB that was never opened", func(t *testing.T) {
		// Main.Close runs on the failure path of Main.Run, which may not have
		// reached the point of opening this.
		t.Parallel()
		assert.NoError(t, NewDB().Close())
	})

	t.Run("is idempotent", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = memoryDSN(t.Name())
		require.NoError(t, db.Open())

		require.NoError(t, db.Close())
		assert.NoError(t, db.Close())
	})

	t.Run("leaves the DB closed", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = memoryDSN(t.Name())
		require.NoError(t, db.Open())
		require.NoError(t, db.Close())

		assert.ErrorIs(t, db.Ping(context.Background()), ErrClosed)
		assert.Nil(t, db.keepalive)
	})
}

func TestDB_Pool(t *testing.T) {
	t.Parallel()

	t.Run("round-trips a row", func(t *testing.T) {
		t.Parallel()
		db := newTestDB(t)
		ctx := context.Background()

		_, err := db.db.ExecContext(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`)
		require.NoError(t, err)
		_, err = db.db.ExecContext(ctx, `INSERT INTO t (id, name) VALUES (1, 'alice')`)
		require.NoError(t, err)

		var name string
		require.NoError(t, db.db.QueryRowContext(ctx, `SELECT name FROM t WHERE id = 1`).Scan(&name))
		assert.Equal(t, "alice", name)
	})

	t.Run("the in-memory database survives the pool draining", func(t *testing.T) {
		// This is the test that fails if the keepalive connection is dropped: a
		// shared-cache in-memory database is destroyed with its last
		// connection, and SetMaxIdleConns(0) tells database/sql to retire every
		// connection the moment it is returned.
		t.Parallel()
		db := newTestDB(t)
		ctx := context.Background()

		_, err := db.db.ExecContext(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY)`)
		require.NoError(t, err)
		_, err = db.db.ExecContext(ctx, `INSERT INTO t (id) VALUES (1)`)
		require.NoError(t, err)

		db.db.SetMaxIdleConns(0)
		require.Equal(t, 0, db.db.Stats().Idle, "the pool should hold no idle connections")

		var got int
		require.NoError(t, db.db.QueryRowContext(ctx, `SELECT id FROM t`).Scan(&got))
		assert.Equal(t, 1, got)
	})

	t.Run("PRAGMAs apply to every connection", func(t *testing.T) {
		// The point of folding configuration into the DSN. A one-shot Exec
		// after sql.Open would set foreign_keys on whichever single connection
		// served it and leave the other three at SQLite's default of 0.
		t.Parallel()
		db := newTestDB(t)
		ctx := context.Background()

		const conns = 4
		db.db.SetMaxOpenConns(conns + 1) // +1: the keepalive holds one

		// Held simultaneously so the pool is forced to open a distinct
		// connection for each, rather than handing the same one back.
		var (
			wg    sync.WaitGroup
			mu    sync.Mutex
			modes []int
		)
		for range conns {
			wg.Add(1)
			go func() {
				defer wg.Done()

				conn, err := db.db.Conn(ctx)
				if !assert.NoError(t, err) {
					return
				}
				defer conn.Close()

				var on int
				if !assert.NoError(t, conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&on)) {
					return
				}

				mu.Lock()
				modes = append(modes, on)
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
			}()
		}
		wg.Wait()

		require.Len(t, modes, conns)
		for _, on := range modes {
			assert.Equal(t, 1, on, "foreign_keys should be enabled on every pooled connection")
		}
	})

	t.Run("busy_timeout applies", func(t *testing.T) {
		t.Parallel()
		db := newTestDB(t)

		var ms int
		require.NoError(t, db.db.QueryRowContext(context.Background(), `PRAGMA busy_timeout`).Scan(&ms))
		assert.Equal(t, int(DefaultBusyTimeout.Milliseconds()), ms)
	})

	t.Run("a file-backed database is journaled WAL", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = filepath.Join(t.TempDir(), "dmost.db")
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		var mode string
		require.NoError(t, db.db.QueryRowContext(context.Background(), `PRAGMA journal_mode`).Scan(&mode))
		assert.Equal(t, "wal", mode)
	})

	t.Run("applies the configured pool limits", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = memoryDSN(t.Name())
		db.MaxOpenConns = 3
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		assert.Equal(t, 3, db.db.Stats().MaxOpenConnections)
	})

	t.Run("a zero pool limit is left to database/sql", func(t *testing.T) {
		// Forwarding the zero would mean "unlimited" here but "keep no idle
		// connections" for MaxIdleConns — the field means unconfigured, not a
		// value.
		//
		// File-backed, because an unconfigured in-memory database does get a cap
		// — see the next case.
		t.Parallel()
		db := NewDB()
		db.DSN = filepath.Join(t.TempDir(), "pool.db")
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		assert.Equal(t, 0, db.db.Stats().MaxOpenConnections)
	})

	t.Run("an unconfigured in-memory database is capped at two connections", func(t *testing.T) {
		// One for the keepalive, one to work with. Two connections into a
		// shared-cache database can stall on each other's table locks inside the
		// driver rather than failing, so this one never lets them.
		t.Parallel()
		db := newTestDB(t)

		assert.Equal(t, 2, db.db.Stats().MaxOpenConnections)
	})

	t.Run("a configured limit wins over the in-memory cap", func(t *testing.T) {
		// The cap is a default for an unset field, not a rule imposed on a
		// caller who asked for something.
		t.Parallel()
		db := NewDB()
		db.DSN = memoryDSN(t.Name())
		db.MaxOpenConns = 5
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		assert.Equal(t, 5, db.db.Stats().MaxOpenConnections)
	})
}

func TestResolveDSN(t *testing.T) {
	t.Parallel()

	// query returns the parsed query string of a resolved DSN, so assertions can
	// name a parameter instead of depending on the order url.Values encodes them
	// in.
	query := func(t *testing.T, res resolvedDSN) url.Values {
		t.Helper()
		u, err := url.Parse(res.conn)
		require.NoError(t, err)
		return u.Query()
	}

	t.Run("promotes a bare relative path to a URI", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: "dmost.db"})
		require.NoError(t, err)

		assert.Equal(t, "dmost.db", res.path)
		assert.False(t, res.memory)
		assert.Equal(t, "file:dmost.db", res.conn)
	})

	t.Run("promotes a bare absolute path to a URI", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: "/var/lib/dmost/dmost.db"})
		require.NoError(t, err)

		assert.Equal(t, "/var/lib/dmost/dmost.db", res.path)
		assert.Equal(t, "/var/lib/dmost", res.dir())
		assert.False(t, res.memory)
	})

	t.Run("leaves a URI alone", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: "file:dmost.db?mode=ro"})
		require.NoError(t, err)

		assert.Equal(t, "ro", query(t, res).Get("mode"))
	})

	t.Run("recognizes the in-memory spellings", func(t *testing.T) {
		t.Parallel()
		for _, dsn := range []string{DefaultDSN, ":memory:", "file::memory:", "file:", ""} {
			res, err := resolveDSN(&DB{DSN: dsn})
			require.NoError(t, err, dsn)
			assert.True(t, res.memory, dsn)
			assert.Empty(t, res.dir(), dsn)
		}
	})

	t.Run("attaches the configured PRAGMAs", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{
			DSN:         "dmost.db",
			ForeignKeys: true,
			BusyTimeout: 2500 * time.Millisecond,
			JournalMode: "WAL",
		})
		require.NoError(t, err)

		q := query(t, res)
		assert.Equal(t, "1", q.Get("_foreign_keys"))
		assert.Equal(t, "2500", q.Get("_busy_timeout"))
		assert.Equal(t, "WAL", q.Get("_journal_mode"))
	})

	t.Run("omits what is not configured", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: "dmost.db"})
		require.NoError(t, err)

		q := query(t, res)
		assert.False(t, q.Has("_foreign_keys"), "false should decline to ask, not ask for 0")
		assert.False(t, q.Has("_busy_timeout"))
		assert.False(t, q.Has("_journal_mode"))
	})

	t.Run("omits journal_mode for an in-memory database", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: DefaultDSN, JournalMode: "WAL"})
		require.NoError(t, err)

		assert.False(t, query(t, res).Has("_journal_mode"))
	})

	t.Run("preserves the DSN's own parameters", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: DefaultDSN, ForeignKeys: true})
		require.NoError(t, err)

		q := query(t, res)
		assert.Equal(t, "memory", q.Get("mode"))
		assert.Equal(t, "shared", q.Get("cache"))
		assert.Equal(t, "1", q.Get("_foreign_keys"))
	})

	t.Run("a parameter in the DSN wins over the field", func(t *testing.T) {
		t.Parallel()
		res, err := resolveDSN(&DB{
			DSN:         "file:dmost.db?_journal_mode=MEMORY&_busy_timeout=1",
			BusyTimeout: time.Minute,
			JournalMode: "WAL",
		})
		require.NoError(t, err)

		q := query(t, res)
		assert.Equal(t, "MEMORY", q.Get("_journal_mode"))
		assert.Equal(t, "1", q.Get("_busy_timeout"))
	})

	t.Run("an alias in the DSN wins over the field", func(t *testing.T) {
		// The driver accepts _fk for _foreign_keys and applies the alias last,
		// so adding the primary key alongside it would be ignored at best and
		// contradictory at worst.
		t.Parallel()
		res, err := resolveDSN(&DB{DSN: "file:dmost.db?_fk=0", ForeignKeys: true})
		require.NoError(t, err)

		q := query(t, res)
		assert.Equal(t, "0", q.Get("_fk"))
		assert.False(t, q.Has("_foreign_keys"))
	})
}
