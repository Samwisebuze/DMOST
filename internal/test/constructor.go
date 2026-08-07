package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samwisebuze/dmost/pkg/domain"
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

// MustUser builds a valid user with a caller-chosen email and handle so
// uniqueness assertions can vary one field at a time. An empty handle leaves
// the user's handle nil.
func MustUser(t testing.TB, email, handle string) *domain.User {
	t.Helper()
	addr, err := domain.NewEmail(email)
	require.NoError(t, err)
	usr, err := domain.NewUser("first", "last", addr, handle)
	require.NoError(t, err)
	return &usr
}

// MustRehydrateUser builds a user with a caller-chosen ID so ordering
// assertions do not depend on generation order.
func MustRehydrateUser(t testing.TB, id domain.UserID, n int) *domain.User {
	t.Helper()
	email, err := domain.NewEmail(fmt.Sprintf("user%d@example.org", n))
	require.NoError(t, err)
	handle := fmt.Sprintf("handle%d", n)
	usr := domain.UserFactory{}.Rehydrate(id, "first", "last", email, &handle, time.Now())
	return &usr
}
