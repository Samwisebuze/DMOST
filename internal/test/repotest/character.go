// Package repotest holds the behavior every repository implementation owes its
// port, as tests any implementation can be run through.
//
// A repository port has rules that are not visible in its method set: Save is
// an upsert keyed by ID, an insert keeps the version the aggregate was
// constructed with while a replacement advances it, a write from a stale
// version is refused rather than silently accepted, and the caller's own
// aggregate tracks the stored revision so consecutive saves need no reload.
// Those rules were written down once, as
// [github.com/samwisebuze/dmost/internal/infra/inmem]'s tests. With a second
// backend they have to be one definition rather than two that drift, so they
// live here and both packages run them.
//
// What stays behind in each implementation's own test file is whatever is true
// of that technology rather than of the port — how a sheet is physically
// stored, what a closed connection does, how the pool behaves. If an assertion
// would be wrong for some other backend, it does not belong in here.
package repotest

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunCharacterRepositoryContract exercises newRepo against everything
// [character.CharacterRepository] promises.
//
// newRepo is called once per scenario and must return an empty repository each
// time — the scenarios assume they are alone in the store, and several of them
// count what is in it.
func RunCharacterRepositoryContract(t *testing.T, newRepo func(t *testing.T) character.CharacterRepository) {
	t.Helper()

	t.Run("Save", func(t *testing.T) { runCharacterSave(t, newRepo) })
	t.Run("Find", func(t *testing.T) { runCharacterFind(t, newRepo) })
	t.Run("ConcurrentAccess", func(t *testing.T) { runCharacterConcurrentAccess(t, newRepo) })
	t.Run("List", func(t *testing.T) { runCharacterList(t, newRepo) })
}

func runCharacterSave(t *testing.T, newRepo func(t *testing.T) character.CharacterRepository) {
	t.Helper()

	t.Run("stores a new character", func(t *testing.T) {
		sut := newRepo(t)
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"name":"Bruenor"}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.Equal(t, chr.ID(), got.ID())
		assert.JSONEq(t, `{"name":"Bruenor"}`, string(got.Data()), "the sheet must round-trip")
	})

	t.Run("stores distinct characters", func(t *testing.T) {
		sut := newRepo(t)
		a := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"name":"a"}`)
		b := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"name":"b"}`)
		require.NoError(t, sut.Save(context.Background(), a))
		require.NoError(t, sut.Save(context.Background(), b))

		gotA, err := sut.Find(context.Background(), a.ID())
		require.NoError(t, err)
		assert.JSONEq(t, `{"name":"a"}`, string(gotA.Data()))
		gotB, err := sut.Find(context.Background(), b.ID())
		require.NoError(t, err)
		assert.JSONEq(t, `{"name":"b"}`, string(gotB.Data()), "a second insert must not overwrite the first")
	})

	t.Run("replaces the character holding the same id", func(t *testing.T) {
		// Save is an upsert keyed by CharacterID: a known ID is an update, not a
		// collision.
		sut := newRepo(t)
		id := character.NewCharacterID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{"level":1}`)))

		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{"level":2}`)))

		got, err := sut.Find(context.Background(), id)
		require.NoError(t, err)
		assert.JSONEq(t, `{"level":2}`, string(got.Data()))
	})

	t.Run("an insert keeps the constructed version", func(t *testing.T) {
		sut := newRepo(t)
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.Equal(t, common.NewVersion(), got.Version(), "a first write is not a replacement")
	})

	t.Run("an update advances the version", func(t *testing.T) {
		sut := newRepo(t)
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"level":1}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		require.NoError(t, sut.Save(context.Background(), chr))

		assert.Equal(t, common.NewVersion().Next(), chr.Version(), "the caller's aggregate must track the stored revision")
		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.Equal(t, common.NewVersion().Next(), got.Version())
	})

	t.Run("consecutive updates need no reload", func(t *testing.T) {
		// Save advances the caller's aggregate, so the same variable stays
		// savable.
		sut := newRepo(t)
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		want := common.NewVersion()
		for range 3 {
			require.NoError(t, sut.Save(context.Background(), chr))
			want = want.Next()
		}
		assert.Equal(t, want, chr.Version())
	})

	t.Run("rejects a write from a stale version", func(t *testing.T) {
		// Two callers load the same revision; the second to write loses.
		sut := newRepo(t)
		id := character.NewCharacterID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{"name":"original"}`)))

		first, err := sut.Find(context.Background(), id)
		require.NoError(t, err)
		second, err := sut.Find(context.Background(), id)
		require.NoError(t, err)

		require.NoError(t, sut.Save(context.Background(), &first))
		require.ErrorIs(t, sut.Save(context.Background(), &second), common.ErrConflict)
	})

	t.Run("a rejected write leaves the version alone", func(t *testing.T) {
		sut := newRepo(t)
		id := character.NewCharacterID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{}`)))

		stale := test.MustRehydrateCharacter(t, id, `{"stale":true}`)
		require.NoError(t, sut.Save(context.Background(), stale)) // v1 -> v2
		require.ErrorIs(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{}`)), common.ErrConflict)

		got, err := sut.Find(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, common.NewVersion().Next(), got.Version(), "nothing was persisted, so nothing was revised")
		assert.JSONEq(t, `{"stale":true}`, string(got.Data()), "the losing write must not land")
	})

	t.Run("an update preserves created_at", func(t *testing.T) {
		sut := newRepo(t)
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		loaded, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		require.NoError(t, sut.Save(context.Background(), &loaded))

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		// Compared as an instant, not as a value. A repository that serializes
		// stores the moment, not the [time.Time] holding it: a round trip
		// through text drops the monotonic reading and normalizes the location,
		// which reflect.DeepEqual — and so assert.Equal — counts as a
		// difference. Preserving created_at means the instant is the same.
		assert.True(t, chr.CreatedAt().Equal(got.CreatedAt()),
			"created_at must survive an update: stored %s, want %s", got.CreatedAt(), chr.CreatedAt())
	})

	t.Run("editing a loaded sheet does not reach the stored one", func(t *testing.T) {
		// The sheet is a json.RawMessage, so a shared backing array would let a
		// caller revise a stored character without going through Save.
		sut := newRepo(t)
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"level":1}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		loaded, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		sheet := loaded.Data()
		for i := range sheet {
			sheet[i] = ' '
		}

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.JSONEq(t, `{"level":1}`, string(got.Data()))
	})
}

func runCharacterFind(t *testing.T, newRepo func(t *testing.T) character.CharacterRepository) {
	t.Helper()

	t.Run("reports an unknown id as not found", func(t *testing.T) {
		sut := newRepo(t)

		_, err := sut.Find(context.Background(), character.NewCharacterID())
		require.ErrorIs(t, err, common.ErrNotFound)
	})
}

func runCharacterList(t *testing.T, newRepo func(t *testing.T) character.CharacterRepository) {
	t.Helper()

	t.Run("lists characters", func(t *testing.T) {
		sut := newRepo(t)
		for i := range 3 {
			chr := test.MustRehydrateCharacter(t, character.CharacterID(strconv.FormatInt(int64(i), 10)), `{"level": "1"}`)
			require.NoError(t, sut.Save(context.Background(), chr))
		}

		loaded, err := sut.List(context.Background())
		require.NoError(t, err)
		assert.Len(t, loaded, 3)
	})

	t.Run("lists characters in ascending order by creation", func(t *testing.T) {
		sut := newRepo(t)
		for range 3 {
			chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"level": "1"}`)
			require.NoError(t, sut.Save(context.Background(), chr))
		}

		loaded, err := sut.List(context.Background())
		require.NoError(t, err)
		var cursor character.CharacterID = loaded[0].ID()
		for _, el := range loaded[1:] {
			assert.Equal(t, -1, cursor.Compare(el.ID())) // ASC
		}
	})
}

func runCharacterConcurrentAccess(t *testing.T, newRepo func(t *testing.T) character.CharacterRepository) {
	t.Helper()

	// Under pkg/http every request is its own goroutine, so the repository has
	// to survive concurrent writers. Meaningful under -race.
	const writers = 16

	t.Run("survives concurrent writers and readers", func(t *testing.T) {
		sut := newRepo(t)
		chars := make([]*character.Character, writers)
		for i := range chars {
			chars[i] = test.MustRehydrateCharacter(t, character.NewCharacterID(), fmt.Sprintf(`{"n":%d}`, i))
		}

		var start, done sync.WaitGroup
		start.Add(1)
		done.Add(2 * writers)
		errs := make([]error, writers)
		for i, chr := range chars {
			go func() {
				defer done.Done()
				start.Wait()
				errs[i] = sut.Save(context.Background(), chr)
			}()
			go func() {
				defer done.Done()
				start.Wait()
				_, _ = sut.Find(context.Background(), chars[0].ID())
			}()
		}
		start.Done()
		done.Wait()

		for i, err := range errs {
			require.NoError(t, err, "character %d", i)
		}
		for i, chr := range chars {
			got, err := sut.Find(context.Background(), chr.ID())
			require.NoError(t, err, "character %d", i)
			assert.JSONEq(t, fmt.Sprintf(`{"n":%d}`, i), string(got.Data()))
		}
	})

	t.Run("only one writer wins a contested update", func(t *testing.T) {
		// Every writer loads the same revision. Without the compare-and-set they
		// would all succeed and all but one write would vanish; with it, exactly
		// one lands per revision.
		sut := newRepo(t)
		id := character.NewCharacterID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{"n":-1}`)))

		loaded := make([]character.Character, writers)
		for i := range loaded {
			got, err := sut.Find(context.Background(), id)
			require.NoError(t, err)
			loaded[i] = got
		}

		var start, done sync.WaitGroup
		start.Add(1)
		done.Add(writers)
		errs := make([]error, writers)
		for i := range loaded {
			go func() {
				defer done.Done()
				start.Wait()
				errs[i] = sut.Save(context.Background(), &loaded[i])
			}()
		}
		start.Done()
		done.Wait()

		var saved int
		for _, err := range errs {
			if err == nil {
				saved++
				continue
			}
			require.ErrorIs(t, err, common.ErrConflict)
		}
		assert.Equal(t, 1, saved, "the compare-and-set must hold under contention")
	})
}
