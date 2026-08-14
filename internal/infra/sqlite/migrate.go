package sqlite

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"

	// migratesqlite is golang-migrate's SQLite database driver. It is imported
	// under an alias because its package name is "sqlite", the same as ours.
	//
	// It must be database/sqlite and never database/sqlite3: the former is
	// backed by modernc.org/sqlite, the same CGo-free port this package already
	// depends on, while the latter binds mattn/go-sqlite3 and would break the
	// CGO_ENABLED=0 build docker/Dockerfile needs for its scratch image.
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrationsFS carries the schema inside the binary.
//
// The release image is FROM scratch and copies only the executable, so a
// source/file driver would look at runtime for a directory that does not exist
// there. Embedding is not a preference here, it is the only form that ships.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir is migrationsFS's single directory. iofs takes the path
// separately from the FS, and the two have to agree.
const migrationsDir = "migrations"

// Migrate brings the schema up to the version this binary carries.
//
// It is safe to call on a database that is already current: golang-migrate
// reports that as [migrate.ErrNoChange], which is success here. With the
// in-memory default every Open starts from an empty database and applies
// everything; against a file-backed DSN the ordinary second start applies
// nothing, and treating that as an error would keep the daemon from booting on
// a healthy database.
//
// [DB.Open] calls this for you unless [DB.AutoMigrate] is false.
//
// Cancelling ctx stops the run at the next safe break point — between
// migrations, not part-way through one — so a cancelled Migrate still leaves
// the schema at a version, never half of one.
//
// Two things this does not do. It does not serialize against another process:
// golang-migrate's SQLite driver locks with an in-process flag, not a database
// lock, so two programs migrating the same file concurrently can still collide.
// And it does not close anything — see the comment on the deliberate absence of
// m.Close below.
func (db *DB) Migrate(ctx context.Context) error {
	handle, err := db.handle()
	if err != nil {
		return err
	}

	src, err := iofs.New(migrationsFS, migrationsDir)
	if err != nil {
		return fmt.Errorf("sqlite: read migrations: %w", err)
	}

	// The zero Config takes golang-migrate's own defaults: the version table is
	// "schema_migrations", DatabaseName is unused by this driver, and NoTxWrap
	// stays false so each migration runs inside a transaction. SQLite's DDL is
	// transactional, so a migration that fails part-way rolls back completely
	// rather than stranding half a schema behind a dirty flag.
	drv, err := migratesqlite.WithInstance(handle, &migratesqlite.Config{})
	if err != nil {
		return fmt.Errorf("sqlite: prepare migrations: %w", err)
	}

	// Both names are labels golang-migrate puts in log lines; reusing DriverName
	// keeps the database one honest.
	m, err := migrate.NewWithInstance("iofs", src, DriverName, drv)
	if err != nil {
		return fmt.Errorf("sqlite: prepare migrations: %w", err)
	}

	// Deliberately no defer m.Close(). golang-migrate's SQLite driver implements
	// Close as a call to Close on the *sql.DB handed to WithInstance — ours —
	// and migrate.Migrate.Close calls into it. Running it here would shut the
	// pool down in the middle of Open and, on the in-memory default, destroy the
	// database along with it. Nothing leaks by not calling it: the iofs driver
	// only closes its FS if it is an io.Closer, and embed.FS is not.

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			// Non-blocking against stop as well, so this goroutine cannot
			// outlive Migrate if the run finished before the cancellation
			// landed.
			select {
			case m.GracefulStop <- true:
			case <-stop:
			}
		case <-stop:
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("sqlite: migrate: %w", err)
	}

	return nil
}
