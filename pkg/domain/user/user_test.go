package user_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	t.Parallel()
	type args struct {
		FirstName string
		LastName  string
		Email     user.Email
	}

	validArgs := args{
		FirstName: "first",
		LastName:  "last",
		Email:     test.MustEmail(t, "valid@example.org"),
	}

	tests := map[string]struct {
		argsMod func(*args)
		wantErr bool
		assert  require.ValueAssertionFunc
	}{
		"success": {
			assert: func(tt require.TestingT, v interface{}, _ ...interface{}) {
				var u user.User
				require.NotPanics(tt, func() {
					u = v.(user.User)
				})

				assert.Equal(tt, validArgs.FirstName, u.FirstName(), "first name mismatch, check golden values")
				assert.Equal(tt, validArgs.LastName, u.LastName(), "last name mismatch, check golden values")
				assert.Equal(tt, validArgs.Email, u.Email(), "email mismatch, check golden values")
				assert.NotZero(tt, u.CreatedAt(), "creation timestamp should be set")
				assert.Equal(t, time.UTC, u.CreatedAt().Location())
				assert.NotZero(tt, u.ID(), "id must be generated")
				// Optional Fields
				assert.Zero(tt, u.Handle())
			},
		},
		"id is uuid v7": {
			assert: func(tt require.TestingT, v interface{}, _ ...interface{}) {
				var u user.User
				require.NotPanics(tt, func() {
					u = v.(user.User)
				})
				uid, err := uuid.Parse(u.ID().String())
				assert.NoError(tt, err, "id is not a valid uuid")

				assert.Equal(tt, uuid.Version(7), uid.Version())
			},
		},
		"full name required": {
			argsMod: func(a *args) {
				a.FirstName = ""
				a.LastName = ""
			},
			wantErr: true,
		},
		"email required": {
			argsMod: func(a *args) {
				a.Email = user.Email{}
			},
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			args := validArgs
			if tc.argsMod != nil {
				tc.argsMod(&args)
			}

			got, err := user.NewUser(args.FirstName, args.LastName, args.Email)
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, common.ErrInvalid)
				return
			}
			require.NoError(t, err)
			require.NotZero(t, got)
			if tc.assert != nil {
				tc.assert(t, got)
			}
		})
	}
}
func TestUser_ChangeEmail(t *testing.T) {
	t.Parallel()
	t.Run("replaces the address", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "changeEmail")
		next, err := user.NewEmail("next@example.org")
		require.NoError(t, err)

		require.NoError(t, usr.ChangeEmail(next))
		assert.Equal(t, next, usr.Email())
	})

	t.Run("rejects the zero email", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "zeroEmail")
		before := usr.Email()

		require.ErrorIs(t, usr.ChangeEmail(user.Email{}), common.ErrInvalid)
		assert.Equal(t, before, usr.Email(), "a rejected change must not mutate the user")
	})
}

func TestUser_Rename(t *testing.T) {
	t.Parallel()
	t.Run("replaces both name parts", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "rename")

		require.NoError(t, usr.Rename("Ada", "Lovelace"))
		assert.Equal(t, "Ada", usr.FirstName())
		assert.Equal(t, "Lovelace", usr.LastName())
	})

	t.Run("rejects a missing part", func(t *testing.T) {
		for name, args := range map[string][2]string{
			"no first": {"", "Lovelace"},
			"no last":  {"Ada", ""},
			"neither":  {"", ""},
		} {
			t.Run(name, func(t *testing.T) {
				usr := test.MustUser(t, "e@example.org", "renameMissingPart")
				require.ErrorIs(t, usr.Rename(args[0], args[1]), common.ErrInvalid)
				assert.Equal(t, "first", usr.FirstName(), "a rejected rename must not mutate the user")
				assert.Equal(t, "last", usr.LastName())
			})
		}
	})
}

func TestUser_SetHandle(t *testing.T) {
	t.Parallel()
	t.Run("replaces the handle", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setHandle")

		require.NoError(t, usr.SetHandle("next"))
		require.NotZero(t, usr.Handle())
		assert.Equal(t, test.MustUserHandle(t, "next"), usr.Handle())
	})

	t.Run("an empty handle clears it", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setHandleEmpty")

		require.NoError(t, usr.SetHandle(""))
		assert.Zero(t, usr.Handle(), "handles are explicit zero so a user can have none")
	})

	t.Run("rejects a blank handle", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setHandleBlank")

		require.ErrorIs(t, usr.SetHandle("   "), common.ErrInvalid)
		require.NotZero(t, usr.Handle(), "a rejected change must not mutate the user")
		assert.Equal(t, test.MustUserHandle(t, "setHandleBlank"), usr.Handle())
	})
}

func TestUser_SetBio(t *testing.T) {
	t.Parallel()
	t.Run("replaces the bio", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setBio", user.WithBio("original"))
		require.NoError(t, usr.Profile().SetBio("modified"))
		require.NotZero(t, usr.Profile().Bio())
		assert.Equal(t, "modified", usr.Profile().Bio())
	})

	t.Run("rejects empty", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setBio", user.WithBio("original"))
		require.ErrorIs(t, usr.Profile().SetBio(""), common.ErrInvalid)
		assert.Equal(t, "original", usr.Profile().Bio(), "a rejected change must not mutate the user")
	})

	t.Run("rejects blank", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setBio", user.WithBio("original"))
		require.ErrorIs(t, usr.Profile().SetBio("\t \t \n"), common.ErrInvalid)
		assert.Equal(t, "original", usr.Profile().Bio(), "a rejected change must not mutate the user")
	})

	t.Run("rejects input > 1 MB", func(t *testing.T) {
		usr := test.MustUser(t, "e@example.org", "setBio", user.WithBio("original"))
		require.ErrorIs(t, usr.Profile().SetBio(strings.Repeat("0", 1024*1024+1)), common.ErrInvalid)
		assert.Equal(t, "original", usr.Profile().Bio(), "a rejected change must not mutate the user")
	})
}

func TestUser_MutatorsPreserveIdentity(t *testing.T) {
	// The load-modify-save cycle relies on this: nothing a caller can edit
	// touches the ID or CreatedAt stamped at construction.
	usr := test.MustUser(t, "e@example.org", "setHandle")

	id, createdAt := usr.ID(), usr.CreatedAt()

	next, err := user.NewEmail("next@example.org")
	require.NoError(t, err)
	require.NoError(t, usr.ChangeEmail(next))
	require.NoError(t, usr.Rename("Ada", "Lovelace"))
	require.NoError(t, usr.SetHandle("ada"))

	assert.Equal(t, id, usr.ID())
	assert.Equal(t, createdAt, usr.CreatedAt())
}
