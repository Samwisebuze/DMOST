package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/domain/user"
	"github.com/stretchr/testify/require"
)

func MustUserHandle(t testing.TB, handle string) user.UserHandle {
	t.Helper()
	h, err := user.NewUserHandle(handle)
	require.NoError(t, err)
	return h
}

func MustEmail(t testing.TB, email string) user.Email {
	t.Helper()
	e, err := user.NewEmail(email)
	require.NoError(t, err)
	return e
}

func MustNewUser(t testing.TB) *user.User {
	t.Helper()
	usr, err := user.NewUser("fist", "last", MustEmail(t, "valid@example.org"))
	require.NoError(t, err)
	return &usr
}

// MustUser builds a valid user with a caller-chosen email and handle so
// uniqueness assertions can vary one field at a time. An empty handle leaves
// the user's handle nil.
func MustUser(t testing.TB, email, handle string, opts ...user.UserOption) *user.User {
	t.Helper()
	usr, err := user.NewUser("first", "last", MustEmail(t, email), opts...)
	require.NoError(t, err)
	require.NoError(t, usr.SetHandle(handle))
	return &usr
}

// MustRehydrateUser builds a user with a caller-chosen ID so ordering
// assertions do not depend on generation order.
func MustRehydrateUser(t testing.TB, id user.UserID, n int) *user.User {
	t.Helper()
	email := MustEmail(t, fmt.Sprintf("user%d@example.org", n))
	handle := MustUserHandle(t, fmt.Sprintf("handle%d", n))
	usr := user.UserFactory{}.Rehydrate(id, "first", "last", email, handle, time.Now(), common.NewVersion())
	return &usr
}
