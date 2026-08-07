package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/stretchr/testify/require"
)

func MustUserHandle(t testing.TB, handle string) domain.UserHandle {
	t.Helper()
	h, err := domain.NewUserHandle(handle)
	require.NoError(t, err)
	return h
}

func MustEmail(t testing.TB, email string) domain.Email {
	t.Helper()
	e, err := domain.NewEmail(email)
	require.NoError(t, err)
	return e
}

func MustNewUser(t testing.TB) *domain.User {
	t.Helper()
	usr, err := domain.NewUser("fist", "last", MustEmail(t, "valid@example.org"))
	require.NoError(t, err)
	return &usr
}

// MustUser builds a valid user with a caller-chosen email and handle so
// uniqueness assertions can vary one field at a time. An empty handle leaves
// the user's handle nil.
func MustUser(t testing.TB, email, handle string) *domain.User {
	t.Helper()
	usr, err := domain.NewUser("first", "last", MustEmail(t, email))
	require.NoError(t, err)
	require.NoError(t, usr.SetHandle(handle))
	return &usr
}

// MustRehydrateUser builds a user with a caller-chosen ID so ordering
// assertions do not depend on generation order.
func MustRehydrateUser(t testing.TB, id domain.UserID, n int) *domain.User {
	t.Helper()
	email := MustEmail(t, fmt.Sprintf("user%d@example.org", n))
	handle := MustUserHandle(t, fmt.Sprintf("handle%d", n))
	usr := domain.UserFactory{}.Rehydrate(id, "first", "last", email, handle, time.Now(), 1)
	return &usr
}
