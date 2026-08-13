package inmem_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterRepository_Save(t *testing.T) {
	t.Run("stores a new character", func(t *testing.T) {
		sut := inmem.NewCharacterRepository()
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{"name":"Bruenor"}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.Equal(t, chr.ID(), got.ID())
		assert.JSONEq(t, `{"name":"Bruenor"}`, string(got.Data()), "the sheet must round-trip")
	})

	t.Run("stores distinct characters", func(t *testing.T) {
		sut := inmem.NewCharacterRepository()
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
		sut := inmem.NewCharacterRepository()
		id := character.NewCharacterID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{"level":1}`)))

		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateCharacter(t, id, `{"level":2}`)))

		got, err := sut.Find(context.Background(), id)
		require.NoError(t, err)
		assert.JSONEq(t, `{"level":2}`, string(got.Data()))
	})

	t.Run("an insert keeps the constructed version", func(t *testing.T) {
		sut := inmem.NewCharacterRepository()
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.Equal(t, common.NewVersion(), got.Version(), "a first write is not a replacement")
	})

	t.Run("an update advances the version", func(t *testing.T) {
		sut := inmem.NewCharacterRepository()
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
		sut := inmem.NewCharacterRepository()
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
		sut := inmem.NewCharacterRepository()
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
		sut := inmem.NewCharacterRepository()
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
		sut := inmem.NewCharacterRepository()
		chr := test.MustRehydrateCharacter(t, character.NewCharacterID(), `{}`)
		require.NoError(t, sut.Save(context.Background(), chr))

		loaded, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		require.NoError(t, sut.Save(context.Background(), &loaded))

		got, err := sut.Find(context.Background(), chr.ID())
		require.NoError(t, err)
		assert.Equal(t, chr.CreatedAt(), got.CreatedAt())
	})

	t.Run("editing a loaded sheet does not reach the stored one", func(t *testing.T) {
		// The sheet is a json.RawMessage, so a shared backing array would let a
		// caller revise a stored character without going through Save.
		sut := inmem.NewCharacterRepository()
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

func TestCharacterRepository_Find(t *testing.T) {
	t.Run("reports an unknown id as not found", func(t *testing.T) {
		sut := inmem.NewCharacterRepository()

		_, err := sut.Find(context.Background(), character.NewCharacterID())
		require.ErrorIs(t, err, common.ErrNotFound)
	})
}

func TestCharacterRepository_ConcurrentAccess(t *testing.T) {
	// Under pkg/http every request is its own goroutine, so the repository has
	// to survive concurrent writers. Meaningful under -race.
	const writers = 16

	sut := inmem.NewCharacterRepository()
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

	t.Run("only one writer wins a contested update", func(t *testing.T) {
		// Every writer loads the same revision. Without the compare-and-set they
		// would all succeed and all but one write would vanish; with it, exactly
		// one lands per revision.
		sut := inmem.NewCharacterRepository()
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
