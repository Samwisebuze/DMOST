package inmem_test

import (
	"context"
	"slices"
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

	t.Run("rejects an id collision", func(t *testing.T) {
		sut := inmem.NewUserRepository()
		id := domain.NewUserID()
		require.NoError(t, sut.Save(context.Background(), test.MustRehydrateUser(t, id, 1)))

		err := sut.Save(context.Background(), test.MustRehydrateUser(t, id, 2))
		require.Error(t, err)
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
	assert.Equal(t, *usr.Handle(), *got.Handle())

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
