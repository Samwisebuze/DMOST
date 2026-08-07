package inmem_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func MustNewUser(t testing.TB) *domain.User {
	t.Helper()
	email, err := domain.NewEmail("valid@example.org")
	require.NoError(t, err)
	usr, err := domain.NewUser("fist", "last", email, uuid.Must(uuid.NewUUID()).String())
	require.NoError(t, err)
	return &usr
}

func TestUserRepository_Save(t *testing.T) {
	sut := inmem.NewUserRepository()
	usr := MustNewUser(t)
	require.NoError(t, sut.Save(context.Background(), usr))
}

func TestUserRepository_Find(t *testing.T) {
	sut := inmem.NewUserRepository()
	usr := MustNewUser(t)
	require.NoError(t, sut.Save(context.Background(), usr))

	got, err := sut.Find(context.Background(), usr.ID())
	require.NoError(t, err)
	assert.Equal(t, usr.ID(), got.ID())
	assert.Equal(t, usr.Email(), got.Email())
	assert.Equal(t, usr.CreatedAt(), got.CreatedAt())

	_, err = sut.Find(context.Background(), domain.NewUserID())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
