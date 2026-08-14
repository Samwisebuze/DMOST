package sqlite

import (
	"context"
	"io/fs"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_Migrate(t *testing.T) {
	t.Parallel()

	t.Run("creates the characters table", func(t *testing.T) {
		t.Parallel()
		// Open alone is enough: AutoMigrate defaults on, which is what makes the
		// in-memory default usable rather than merely open.
		db := newTestDB(t)

		var name string
		require.NoError(t, db.db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'characters'`).Scan(&name))
		assert.Equal(t, "characters", name)
	})

	t.Run("records the schema version", func(t *testing.T) {
		t.Parallel()
		db := newTestDB(t)

		var (
			version int
			dirty   bool
		)
		require.NoError(t, db.db.QueryRowContext(context.Background(),
			`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
		assert.Equal(t, 1, version, "every migration in migrations/ must have been applied")
		assert.False(t, dirty, "a dirty schema means a migration failed part-way")
	})

	t.Run("is idempotent", func(t *testing.T) {
		t.Parallel()
		// golang-migrate reports "already current" as ErrNoChange. Treating that
		// as a failure would keep the daemon from starting on a healthy
		// file-backed database, where it is the ordinary case.
		db := newTestDB(t)

		require.NoError(t, db.Migrate(context.Background()))
		require.NoError(t, db.Migrate(context.Background()))
	})

	t.Run("reports a closed database", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, NewDB().Migrate(context.Background()), ErrClosed)
	})

	t.Run("leaves the schema alone when AutoMigrate is off", func(t *testing.T) {
		t.Parallel()
		db := NewDB()
		db.DSN = memoryDSN(t.Name())
		db.AutoMigrate = false
		require.NoError(t, db.Open())
		t.Cleanup(func() { assert.NoError(t, db.Close()) })

		var count int
		require.NoError(t, db.db.QueryRowContext(context.Background(),
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'characters'`).Scan(&count))
		assert.Zero(t, count, "Open must not migrate when it was told not to")

		require.NoError(t, db.Migrate(context.Background()), "the caller can still run it")
		require.NoError(t, db.db.QueryRowContext(context.Background(),
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'characters'`).Scan(&count))
		assert.Equal(t, 1, count)
	})

	t.Run("carries every migration file", func(t *testing.T) {
		t.Parallel()
		// iofs skips anything source.DefaultParse rejects, without a word — a
		// migration misnamed on the way in would simply never run. This is the
		// only thing that would notice.
		entries, err := fs.ReadDir(migrationsFS, migrationsDir)
		require.NoError(t, err)

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Equal(t, []string{
			"000001_create_characters_table.down.sql",
			"000001_create_characters_table.up.sql",
		}, names)

		src, err := iofs.New(migrationsFS, migrationsDir)
		require.NoError(t, err)
		first, err := src.First()
		require.NoError(t, err)
		assert.Equal(t, uint(1), first, "iofs must have accepted the file names, not skipped them")
	})
}
