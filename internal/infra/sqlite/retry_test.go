package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sqlitedriver "modernc.org/sqlite"
)

// lockedErr returns a real SQLITE_BUSY from the driver, for the tests below to
// retry. [sqlitedriver.Error] has only unexported fields, so it cannot be built
// by hand; the shortest route to one is genuine write contention.
//
// It has to be a file-backed database. On the shared-cache in-memory default
// the driver does not report a table-lock conflict at all — it calls
// sqlite3_unlock_notify and parks the goroutine until the holder releases — so
// an open transaction there would hang this helper instead of failing the
// second writer. See [DB.Open], which caps that pool so two connections never
// reach the situation.
//
// BusyTimeout is cleared so the second writer gives up immediately rather than
// spending the default five seconds getting to the same answer.
func lockedErr(t *testing.T) error {
	t.Helper()

	db := NewDB()
	db.DSN = filepath.Join(t.TempDir(), "dmost.db")
	db.BusyTimeout = 0
	require.NoError(t, db.Open())
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	ctx := context.Background()

	tx, err := db.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	// Takes the write lock and keeps it until the rollback above.
	_, err = tx.ExecContext(ctx,
		`INSERT INTO characters (id, data, created_at, version) VALUES ('a', '{}', '2026-01-01T00:00:00.000000000Z', 1)`)
	require.NoError(t, err)

	// A second connection, refused because the first is still writing.
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO characters (id, data, created_at, version) VALUES ('b', '{}', '2026-01-01T00:00:00.000000000Z', 1)`)
	require.Error(t, err, "a second writer must not get past an open write transaction")

	var serr *sqlitedriver.Error
	require.ErrorAs(t, err, &serr, "expected a driver error, got %v", err)
	require.True(t, isLocked(err), "expected a busy or locked code, got %d (%v)", serr.Code(), err)

	return err
}

func TestIsLocked(t *testing.T) {
	t.Parallel()

	t.Run("recognizes a locked database", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isLocked(lockedErr(t)))
	})

	t.Run("ignores everything else", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isLocked(nil))
		assert.False(t, isLocked(sql.ErrNoRows))
		assert.False(t, isLocked(errors.New("boom")))
		assert.False(t, isLocked(context.Canceled))
	})

	t.Run("sees through a wrapped error", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isLocked(fmt.Errorf("sqlite: save character: %w", lockedErr(t))))
	})
}

func TestRetry(t *testing.T) {
	t.Parallel()

	t.Run("returns a success without retrying", func(t *testing.T) {
		t.Parallel()
		var calls int
		require.NoError(t, retry(context.Background(), func() error {
			calls++
			return nil
		}))
		assert.Equal(t, 1, calls)
	})

	t.Run("returns a non-lock error immediately", func(t *testing.T) {
		t.Parallel()
		// sql.ErrNoRows is how a failed compare-and-set arrives. Retrying it
		// would turn a reported lost update into a slow one.
		var calls int
		err := retry(context.Background(), func() error {
			calls++
			return sql.ErrNoRows
		})
		require.ErrorIs(t, err, sql.ErrNoRows)
		assert.Equal(t, 1, calls, "a conflict is an answer, not a condition to wait out")
	})

	t.Run("re-issues a locked statement until it succeeds", func(t *testing.T) {
		t.Parallel()
		locked := lockedErr(t)

		var calls int
		require.NoError(t, retry(context.Background(), func() error {
			calls++
			if calls < 3 {
				return locked
			}
			return nil
		}))
		assert.Equal(t, 3, calls)
	})

	t.Run("gives up after maxRetries and returns the last error", func(t *testing.T) {
		t.Parallel()
		locked := lockedErr(t)

		var calls int
		err := retry(context.Background(), func() error {
			calls++
			return locked
		})
		assert.Equal(t, maxRetries, calls)
		assert.Equal(t, locked, err, "the caller should see what SQLite said")
	})

	t.Run("stops when the context is cancelled", func(t *testing.T) {
		t.Parallel()
		locked := lockedErr(t)

		ctx, cancel := context.WithCancel(context.Background())
		var calls int
		err := retry(ctx, func() error {
			calls++
			cancel()
			return locked
		})
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, calls, "a cancelled request must not go on waiting")
	})
}
