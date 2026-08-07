package inmem_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Save(t *testing.T) {
	t.Run("stores a new user", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustNewUser(t)
		require.NoError(t, sut.Save(context.Background(), usr))

		got, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.Equal(t, usr.ID(), got.ID())
	})

	t.Run("stores distinct users", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "a@example.org", "alice")))
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "b@example.org", "bob")))

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		assert.Len(t, users, 2)
	})

	t.Run("replaces the user holding the same id", func(t *testing.T) {
		// Save is an upsert keyed by UserID: a known ID is an update, not a
		// collision.
		sut := inmem.NewUserRepository()
		id := domain.NewUserID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateUser(t, id, 1)))

		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateUser(t, id, 2)))

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		require.Len(t, users, 1, "an update must not insert a second record")
		assert.Equal(t, "user2@example.org", users[0].Email().String())
	})

	t.Run("re-saving a user unchanged succeeds", func(t *testing.T) {
		// The uniqueness scan skips the record being written, so a user cannot
		// collide with its own email or handle.
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, sut.Save(context.Background(), usr))
	})

	t.Run("rejects an edit taking another user's email", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "taken@example.org", "alice")))
		usr := test.MustUser(t, "b@example.org", "bob")
		require.NoError(t, sut.Save(context.Background(), usr))

		taken, err := domain.NewEmail("taken@example.org")
		require.NoError(t, err)
		require.NoError(t, usr.ChangeEmail(taken))

		require.ErrorIs(t, sut.Save(context.Background(), usr), domain.ErrExists)

		stored, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.Equal(t, "b@example.org", stored.Email().String(), "a rejected update must not be applied")
	})

	t.Run("rejects an edit taking another user's handle", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "a@example.org", "taken")))
		usr := test.MustUser(t, "b@example.org", "bob")
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, usr.SetHandle("taken"))

		require.ErrorIs(t, sut.Save(context.Background(), usr), domain.ErrExists)
	})

	t.Run("an insert keeps the constructed version", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, sut.Save(context.Background(), usr))

		got, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.EqualValues(t, 1, got.Version(), "a first write is not a replacement")
	})

	t.Run("an update advances the version", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, usr.Rename("Ada", "Lovelace"))
		require.NoError(t, sut.Save(context.Background(), usr))

		assert.EqualValues(t, 2, usr.Version(), "the caller's aggregate must track the stored revision")
		got, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.EqualValues(t, 2, got.Version())
	})

	t.Run("consecutive updates need no reload", func(t *testing.T) {
		// Save advances the caller's aggregate, so the same variable stays
		// savable.
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, sut.Save(context.Background(), usr))

		for i := range 3 {
			require.NoError(t, usr.SetHandle(fmt.Sprintf("alice%d", i)))
			require.NoError(t, sut.Save(context.Background(), usr))
		}
		assert.EqualValues(t, 4, usr.Version())
	})

	t.Run("rejects a write from a stale version", func(t *testing.T) {
		// Two callers load the same revision; the second to write loses.
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, sut.Save(context.Background(), usr))

		first, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		second, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)

		require.NoError(t, first.Rename("Ada", "Lovelace"))
		require.NoError(t, sut.Save(context.Background(), &first))

		require.NoError(t, second.Rename("Grace", "Hopper"))
		require.ErrorIs(t, sut.Save(context.Background(), &second), domain.ErrConflict)

		got, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.Equal(t, "Ada", got.FirstName(), "the losing write must not land")
	})

	t.Run("a rejected write leaves the version alone", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "taken@example.org", "taken")))
		usr := test.MustUser(t, "b@example.org", "bob")
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, usr.SetHandle("taken"))
		require.ErrorIs(t, sut.Save(context.Background(), usr), domain.ErrExists)

		assert.EqualValues(t, 1, usr.Version(), "nothing was persisted, so nothing was revised")
	})

	t.Run("an update preserves created_at", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, sut.Save(context.Background(), usr))

		loaded, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		require.NoError(t, loaded.Rename("Ada", "Lovelace"))
		require.NoError(t, sut.Save(context.Background(), &loaded))

		got, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.Equal(t, usr.CreatedAt(), got.CreatedAt())
		assert.Equal(t, "Ada", got.FirstName(), "the edit itself must land")
	})

	t.Run("rejects a duplicate email", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "dupe@example.org", "alice")))

		err := sut.Save(context.Background(), test.MustUser(t, "dupe@example.org", "bob"))
		require.ErrorIs(t, err, domain.ErrExists)
	})

	t.Run("rejects a duplicate email differing only by case", func(t *testing.T) {
		// NewEmail canonicalizes to lower case, so these are the same address.
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "dupe@example.org", "alice")))

		err := sut.Save(context.Background(), test.MustUser(t, "DUPE@Example.ORG", "bob"))
		require.ErrorIs(t, err, domain.ErrExists)
	})

	t.Run("rejects a duplicate handle", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "a@example.org", "shared")))

		err := sut.Save(context.Background(), test.MustUser(t, "b@example.org", "shared"))
		require.ErrorIs(t, err, domain.ErrExists)
	})

	t.Run("allows several users without a handle", func(t *testing.T) {
		// An empty handle is absent, not a value, so it cannot collide.
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "a@example.org", "")))
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "b@example.org", "")))

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		assert.Len(t, users, 2)
	})

	t.Run("leaves the repository unchanged on rejection", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "a@example.org", "alice")))

		rejected := test.MustUser(t, "a@example.org", "bob")
		require.ErrorIs(t, sut.Save(context.Background(), rejected), domain.ErrExists)

		_, err := sut.Find(context.Background(), rejected.ID())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestUserRepository_ConcurrentAccess(t *testing.T) {
	// Under pkg/http every request is its own goroutine, so the repository has
	// to survive concurrent writers. Meaningful under -race.
	const writers = 16

	sut := inmem.NewUserRepository()
	users := make([]*domain.User, writers)
	for i := range users {
		users[i] = test.MustUser(t, fmt.Sprintf("user%d@example.org", i), fmt.Sprintf("handle%d", i))
	}

	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(2 * writers)
	errs := make([]error, writers)
	for i, usr := range users {
		go func() {
			defer done.Done()
			start.Wait()
			errs[i] = sut.Save(context.Background(), usr)
		}()
		go func() {
			defer done.Done()
			start.Wait()
			_, _ = sut.FindAll(context.Background(), domain.UserFilter{})
		}()
	}
	start.Done()
	done.Wait()

	for i, err := range errs {
		require.NoError(t, err, "user %d", i)
	}
	got, err := sut.FindAll(context.Background(), domain.UserFilter{})
	require.NoError(t, err)
	assert.Len(t, got, writers)

	t.Run("only one writer wins a contested handle", func(t *testing.T) {
		sut := inmem.NewUserRepository()

		var start, done sync.WaitGroup
		start.Add(1)
		done.Add(writers)
		errs := make([]error, writers)
		for i := range writers {
			usr := test.MustUser(t, fmt.Sprintf("contested%d@example.org", i), "contested")
			go func() {
				defer done.Done()
				start.Wait()
				errs[i] = sut.Save(context.Background(), usr)
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
			require.ErrorIs(t, err, domain.ErrExists)
		}
		assert.Equal(t, 1, saved, "the uniqueness check must hold under contention")
	})

	t.Run("only one writer wins a contested update", func(t *testing.T) {
		// Every writer loads the same revision and edits it. Without the
		// compare-and-set they would all succeed and all but one edit would
		// vanish; with it, exactly one write lands per revision.
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "contested@example.org", "contested")
		require.NoError(t, sut.Save(context.Background(), usr))

		loaded := make([]domain.User, writers)
		for i := range loaded {
			got, err := sut.Find(context.Background(), usr.ID())
			require.NoError(t, err)
			require.NoError(t, got.Rename(fmt.Sprintf("writer%d", i), "last"))
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
			require.ErrorIs(t, err, domain.ErrConflict)
		}
		assert.Equal(t, 1, saved, "a lost update must be reported, not silently accepted")

		got, err := sut.Find(context.Background(), usr.ID())
		require.NoError(t, err)
		assert.EqualValues(t, 2, got.Version(), "exactly one revision was applied")
	})
}

func TestUserRepository_Find(t *testing.T) {
	sut := inmem.NewUserRepository()
	usr := test.MustNewUser(t)
	require.NoError(t, sut.Save(context.Background(), usr))

	got, err := sut.Find(context.Background(), usr.ID())
	require.NoError(t, err)
	assert.Equal(t, usr.ID(), got.ID())
	assert.Equal(t, usr.Email(), got.Email())
	assert.Equal(t, usr.CreatedAt(), got.CreatedAt())
	assert.Equal(t, usr.FirstName(), got.FirstName())
	assert.Equal(t, usr.LastName(), got.LastName())
	require.NotNil(t, got.Handle())
	assert.Equal(t, usr.Handle(), got.Handle())

	_, err = sut.Find(context.Background(), domain.NewUserID())
	require.ErrorIs(t, err, domain.ErrNotFound)

	t.Run("empty repository", func(t *testing.T) {
		empty := inmem.NewUserRepository()
		_, err := empty.Find(context.Background(), domain.NewUserID())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestUserRepository_FindAll(t *testing.T) {
	t.Run("empty repository", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		got, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("sorts UUIDv7 ids ascending", func(t *testing.T) {
		// UUIDv7 is time-ordered, so generating in sequence yields ascending ids.
		want := make([]domain.UserID, 0, 5)
		for range 5 {
			want = append(want, domain.NewUserID())
		}
		require.True(t, slices.IsSorted(want), "UUIDv7 generation should be monotonic")

		// Insert in reverse so the ordering comes from FindAll, not from Save.
		sut := inmem.NewUserRepository()
		for i := len(want) - 1; i >= 0; i-- {
			require.NoError(t, sut.Save(context.Background(), test.MustRehydrateUser(t, want[i], i)))
		}

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		require.Len(t, users, len(want))

		got := make([]domain.UserID, 0, len(users))
		for _, u := range users {
			got = append(got, u.ID())
		}
		assert.Equal(t, want, got)
	})

	t.Run("ordering is stable across calls", func(t *testing.T) {
		// Map iteration is randomized, so repeated calls must still agree.
		sut := inmem.NewUserRepository()
		for i := range 8 {
			require.NoError(t, sut.Save(context.Background(), test.MustRehydrateUser(t, domain.NewUserID(), i)))
		}

		first, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		for range 5 {
			again, err := sut.FindAll(context.Background(), domain.UserFilter{})
			require.NoError(t, err)
			assert.Equal(t, first, again)
		}
	})

	t.Run("carries whole users, not just ids", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustNewUser(t)
		require.NoError(t, sut.Save(context.Background(), usr))

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		require.Len(t, users, 1)
		assert.Equal(t, usr.Email(), users[0].Email())
		assert.Equal(t, usr.FirstName(), users[0].FirstName())
		assert.Equal(t, usr.CreatedAt(), users[0].CreatedAt())
	})
}

func TestUserRepository_Delete(t *testing.T) {
	t.Run("removes an existing user", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustNewUser(t)
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, sut.Delete(context.Background(), usr.ID()))

		_, err := sut.Find(context.Background(), usr.ID())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("is a no-op for an unknown id", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustNewUser(t)
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, sut.Delete(context.Background(), domain.NewUserID()))

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		assert.Len(t, users, 1)
	})

	t.Run("is idempotent", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustNewUser(t)
		require.NoError(t, sut.Save(context.Background(), usr))

		require.NoError(t, sut.Delete(context.Background(), usr.ID()))
		require.NoError(t, sut.Delete(context.Background(), usr.ID()))
	})

	t.Run("leaves other users in place", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		keep := test.MustUser(t, "keep@example.org", "keep")
		drop := test.MustUser(t, "drop@example.org", "drop")
		require.NoError(t, sut.Save(context.Background(), keep))
		require.NoError(t, sut.Save(context.Background(), drop))

		require.NoError(t, sut.Delete(context.Background(), drop.ID()))

		users, err := sut.FindAll(context.Background(), domain.UserFilter{})
		require.NoError(t, err)
		require.Len(t, users, 1)
		assert.Equal(t, keep.ID(), users[0].ID())
	})

	t.Run("frees the email and handle for reuse", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		usr := test.MustUser(t, "reuse@example.org", "reuse")
		require.NoError(t, sut.Save(context.Background(), usr))
		require.NoError(t, sut.Delete(context.Background(), usr.ID()))

		require.NoError(t, sut.Save(context.Background(), test.MustUser(t, "reuse@example.org", "reuse")))
	})
}
