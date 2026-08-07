package services_test

import (
	"context"
	"testing"

	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/app/services"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// seed puts one user in a fresh repository and returns both.
func seed(t *testing.T, email, handle string) (*services.UserService, *domain.User) {
	t.Helper()
	repo := inmem.NewUserRepository()
	usr := test.MustUser(t, email, handle)
	require.NoError(t, repo.Save(context.Background(), usr))
	return services.NewUserService(repo), usr
}

func TestUserService_Update(t *testing.T) {
	t.Run("applies the populated fields", func(t *testing.T) {
		sut, usr := seed(t, "a@example.org", "alice")

		got, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
			Name:     ptr("Ada Lovelace"),
			Email:    ptr("ada@example.org"),
			Username: ptr("ada"),
		})
		require.NoError(t, err)

		assert.Equal(t, "Ada", got.FirstName())
		assert.Equal(t, "Lovelace", got.LastName())
		assert.Equal(t, "ada@example.org", got.Email().String())
		require.NotNil(t, got.Handle())
		assert.Equal(t, "ada", *got.Handle())
	})

	t.Run("leaves omitted fields unchanged", func(t *testing.T) {
		sut, usr := seed(t, "a@example.org", "alice")

		got, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
			Email: ptr("moved@example.org"),
		})
		require.NoError(t, err)

		assert.Equal(t, "moved@example.org", got.Email().String())
		assert.Equal(t, usr.FirstName(), got.FirstName())
		require.NotNil(t, got.Handle())
		assert.Equal(t, "alice", *got.Handle())
	})

	t.Run("an empty username clears the handle", func(t *testing.T) {
		// Omitted and empty are distinct in UpdateUserRequest: the first means
		// "unchanged", the second means "remove it".
		sut, usr := seed(t, "a@example.org", "alice")

		got, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
			Username: ptr(""),
		})
		require.NoError(t, err)
		assert.Nil(t, got.Handle())
	})

	t.Run("preserves created_at across the round trip", func(t *testing.T) {
		// CreatedAt rides along on the loaded aggregate; no request can carry
		// it, so no client can move it.
		sut, usr := seed(t, "a@example.org", "alice")

		got, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
			Name: ptr("Ada Lovelace"),
		})
		require.NoError(t, err)

		assert.Equal(t, usr.ID(), got.ID())
		assert.Equal(t, usr.CreatedAt(), got.CreatedAt())
	})

	t.Run("does not insert a second record", func(t *testing.T) {
		repo := inmem.NewUserRepository()
		usr := test.MustUser(t, "a@example.org", "alice")
		require.NoError(t, repo.Save(context.Background(), usr))
		sut := services.NewUserService(repo)

		_, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: ptr("Ada Lovelace")})
		require.NoError(t, err)

		users, err := sut.FindAll(context.Background())
		require.NoError(t, err)
		assert.Len(t, users, 1)
	})

	t.Run("reports a collision with another user", func(t *testing.T) {
		repo := inmem.NewUserRepository()
		require.NoError(t, repo.Save(context.Background(), test.MustUser(t, "taken@example.org", "taken")))
		usr := test.MustUser(t, "b@example.org", "bob")
		require.NoError(t, repo.Save(context.Background(), usr))
		sut := services.NewUserService(repo)

		_, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Email: ptr("taken@example.org")})
		require.ErrorIs(t, err, domain.ErrExists)
	})

	t.Run("accepts the version the client last read", func(t *testing.T) {
		sut, usr := seed(t, "a@example.org", "alice")

		got, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
			Name:    ptr("Ada Lovelace"),
			Version: ptr(usr.Version()),
		})
		require.NoError(t, err)
		assert.Equal(t, "Ada", got.FirstName())
		assert.EqualValues(t, usr.Version()+1, got.Version(), "the response carries the new revision")
	})

	t.Run("reports a stale version", func(t *testing.T) {
		// The client read version 1, someone else wrote, and now its edit is
		// based on a state that no longer exists.
		sut, usr := seed(t, "a@example.org", "alice")
		stale := usr.Version()

		_, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: ptr("Ada Lovelace")})
		require.NoError(t, err)

		_, err = sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
			Name:    ptr("Grace Hopper"),
			Version: ptr(stale),
		})
		require.ErrorIs(t, err, domain.ErrConflict)

		users, err := sut.FindAll(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Ada", users[0].FirstName(), "the losing edit must not land")
	})

	t.Run("an omitted version means last writer wins", func(t *testing.T) {
		sut, usr := seed(t, "a@example.org", "alice")

		_, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: ptr("Ada Lovelace")})
		require.NoError(t, err)

		got, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: ptr("Grace Hopper")})
		require.NoError(t, err, "an unconditional update is allowed to overwrite")
		assert.Equal(t, "Grace", got.FirstName())
	})

	t.Run("reports an unknown user", func(t *testing.T) {
		sut, _ := seed(t, "a@example.org", "alice")

		_, err := sut.Update(context.Background(), domain.NewUserID(), v1alpha.UpdateUserRequest{Name: ptr("Ada Lovelace")})
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("rejects an invalid edit without touching the store", func(t *testing.T) {
		sut, usr := seed(t, "a@example.org", "alice")

		_, err := sut.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: ptr("Cher")})
		require.ErrorIs(t, err, domain.ErrInvalid)

		users, err := sut.FindAll(context.Background())
		require.NoError(t, err)
		require.Len(t, users, 1)
		assert.Equal(t, "first", users[0].FirstName(), "validation runs before the save")
	})
}
