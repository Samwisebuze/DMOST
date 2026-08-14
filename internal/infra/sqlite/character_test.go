package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/internal/test/repotest"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestFileDB returns an opened DB on a file-backed database in a temporary
// directory, closed on cleanup.
//
// It is the shape a deployment actually runs: on disk, in WAL, with the full
// connection pool rather than the two-connection cap [DB.Open] applies to an
// in-memory database. The locking story is different enough between the two
// that a repository worth trusting has to be run through both.
func newTestFileDB(t *testing.T) *DB {
	t.Helper()

	db := NewDB()
	db.DSN = filepath.Join(t.TempDir(), "dmost.db")
	require.NoError(t, db.Open())
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	return db
}

// TestCharacterRepository_Contract runs the port's rules, which live in
// internal/test/repotest because they are the same rules inmem answers to.
// Everything below this function is instead specific to storing a Character in
// SQLite, and would be meaningless against another backend.
func TestCharacterRepository_Contract(t *testing.T) {
	t.Parallel()

	t.Run("in-memory", func(t *testing.T) {
		t.Parallel()
		repotest.RunCharacterRepositoryContract(t, func(t *testing.T) character.CharacterRepository {
			return NewCharacterRepository(newTestDB(t))
		})
	})

	t.Run("file-backed", func(t *testing.T) {
		t.Parallel()
		repotest.RunCharacterRepositoryContract(t, func(t *testing.T) character.CharacterRepository {
			return NewCharacterRepository(newTestFileDB(t))
		})
	})
}

func TestCharacterRepository_Storage(t *testing.T) {
	t.Parallel()

	t.Run("stores the sheet as text, not a blob", func(t *testing.T) {
		t.Parallel()
		// database/sql binds a []byte as a BLOB, and TEXT affinity does not
		// convert one, so a Save that passed the json.RawMessage straight
		// through would store a blob — where json_valid switches to JSONB rules.
		// The STRICT table would refuse it, but only this asserts the column
		// holds what the schema says it holds.
		db := newTestDB(t)
		sut := NewCharacterRepository(db)
		chr := test.MustCharacter(t, test.MustCharacterSheet(t))
		require.NoError(t, sut.Save(context.Background(), chr))

		var typ string
		require.NoError(t, db.db.QueryRowContext(context.Background(),
			`SELECT typeof(data) FROM characters WHERE id = ?`, chr.ID().String()).Scan(&typ))
		assert.Equal(t, "text", typ)
	})

	t.Run("stores created_at as sortable RFC 3339 in UTC", func(t *testing.T) {
		t.Parallel()
		// The fixed-width fraction is the point: a TEXT column's natural order
		// is the only order SQLite has for these, and RFC3339Nano's trimmed
		// zeros would sort ".1Z" after ".05Z".
		db := newTestDB(t)
		sut := NewCharacterRepository(db)
		chr := test.MustCharacter(t, test.MustCharacterSheet(t))
		require.NoError(t, sut.Save(context.Background(), chr))

		var stored string
		require.NoError(t, db.db.QueryRowContext(context.Background(),
			`SELECT created_at FROM characters WHERE id = ?`, chr.ID().String()).Scan(&stored))

		assert.Len(t, stored, len("2006-01-02T15:04:05.000000000Z"), "the fraction must be fixed width")
		assert.True(t, strings.HasSuffix(stored, "Z"), "the instant must be stored in UTC, got %q", stored)

		parsed, err := time.Parse(timeLayout, stored)
		require.NoError(t, err)
		assert.True(t, chr.CreatedAt().Equal(parsed))
	})

	t.Run("reports a closed database", func(t *testing.T) {
		t.Parallel()
		// A repository may be built before the pool is open — the composition
		// root wires its services first — so reaching one that is not running
		// has to be an error rather than a panic on a nil handle.
		sut := NewCharacterRepository(NewDB())
		chr := test.MustCharacter(t, test.MustCharacterSheet(t))

		require.ErrorIs(t, sut.Save(context.Background(), chr), ErrClosed)
		_, err := sut.Find(context.Background(), chr.ID())
		require.ErrorIs(t, err, ErrClosed)
	})

	t.Run("panics without a database", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() { NewCharacterRepository(nil) })
	})

	t.Run("rejects a sheet that is not JSON", func(t *testing.T) {
		t.Parallel()
		// The domain constructor makes this unreachable through Save; the CHECK
		// is the engine holding the same line, so that a bug in this package
		// cannot leave unparseable text where a sheet is promised.
		db := newTestDB(t)
		_, err := db.db.ExecContext(context.Background(),
			`INSERT INTO characters (id, data, created_at, version) VALUES (?, ?, ?, ?)`,
			character.NewCharacterID().String(), "not json",
			time.Now().UTC().Format(timeLayout), common.NewVersion().Uint64())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CHECK")
	})
}
