package test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/domain/user"
	"github.com/stretchr/testify/require"
)

// MustRehydrateCharacter builds a character with a caller-chosen ID and sheet,
// going through the factory the same way a repository would. Prefer
// [MustCharacter] unless the test needs to pin the ID or to hold a sheet the
// v1alpha schema would reject.
//
// The timestamp is stamped in UTC, matching what the domain's own constructor
// records. A local time carries a monotonic reading and a location that no
// repository storing an instant can give back, which would make the fixture
// disagree with a round trip for reasons the test is not about.
func MustRehydrateCharacter(t testing.TB, id character.CharacterID, data string) *character.Character {
	t.Helper()
	require.True(t, json.Valid([]byte(data)), "character sheet must be valid JSON")
	c := character.CharacterFactory{}.Rehydrate(id, json.RawMessage(data), time.Now().UTC(), common.NewVersion())
	return &c
}

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
