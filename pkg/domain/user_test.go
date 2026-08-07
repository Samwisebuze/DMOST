package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	type args struct {
		FirstName string
		LastName  string
		Email     domain.Email
		Handle    string
	}

	validArgs := args{
		FirstName: "first",
		LastName:  "last",
		Email: func() domain.Email {
			v, _ := domain.NewEmail("valid@example.org")
			return v
		}(),
		Handle: "username",
	}

	tests := map[string]struct {
		argsMod func(*args)
		wantErr bool
		assert  require.ValueAssertionFunc
	}{
		"success": {
			assert: func(tt require.TestingT, v interface{}, _ ...interface{}) {
				var u domain.User
				require.NotPanics(tt, func() {
					u = v.(domain.User)
				})

				assert.Equal(tt, validArgs.FirstName, u.FirstName(), "first name mismatch, check golden values")
				assert.Equal(tt, validArgs.LastName, u.LastName(), "last name mismatch, check golden values")
				assert.EqualValues(tt, validArgs.Handle, *u.Handle(), "handle mismatch, check golden values")
				assert.Equal(tt, validArgs.Email, u.Email(), "email mismatch, check golden values")
				assert.NotZero(tt, u.CreatedAt(), "creation timestamp should be set")
				assert.Equal(t, time.UTC, u.CreatedAt().Location())
				assert.NotZero(tt, u.ID(), "id must be generated")
			},
		},
		"id is uuid v7": {
			assert: func(tt require.TestingT, v interface{}, _ ...interface{}) {
				var u domain.User
				require.NotPanics(tt, func() {
					u = v.(domain.User)
				})
				uid, err := uuid.Parse(u.ID().String())
				assert.NoError(tt, err, "id is not a valid uuid")

				assert.Equal(tt, uuid.Version(7), uid.Version())
			},
		},
		"handle optional": {
			argsMod: func(a *args) {
				a.Handle = ""
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
				a.Email = domain.Email{}
			},
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			args := validArgs
			if tc.argsMod != nil {
				tc.argsMod(&args)
			}

			got, err := domain.NewUser(args.FirstName, args.LastName, args.Email, args.Handle)
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, domain.ErrInvalid)
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

// mustUser is the domain package's own fixture; internal/test imports domain,
// so this file cannot use it without an import cycle.
func mustUser(t *testing.T) domain.User {
	t.Helper()
	email, err := domain.NewEmail("valid@example.org")
	require.NoError(t, err)
	usr, err := domain.NewUser("first", "last", email, "handle")
	require.NoError(t, err)
	return usr
}

func TestUser_ChangeEmail(t *testing.T) {
	t.Run("replaces the address", func(t *testing.T) {
		usr := mustUser(t)
		next, err := domain.NewEmail("next@example.org")
		require.NoError(t, err)

		require.NoError(t, usr.ChangeEmail(next))
		assert.Equal(t, next, usr.Email())
	})

	t.Run("rejects the zero email", func(t *testing.T) {
		usr := mustUser(t)
		before := usr.Email()

		require.ErrorIs(t, usr.ChangeEmail(domain.Email{}), domain.ErrInvalid)
		assert.Equal(t, before, usr.Email(), "a rejected change must not mutate the user")
	})
}

func TestUser_Rename(t *testing.T) {
	t.Run("replaces both name parts", func(t *testing.T) {
		usr := mustUser(t)

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
				usr := mustUser(t)

				require.ErrorIs(t, usr.Rename(args[0], args[1]), domain.ErrInvalid)
				assert.Equal(t, "first", usr.FirstName(), "a rejected rename must not mutate the user")
				assert.Equal(t, "last", usr.LastName())
			})
		}
	})
}

func TestUser_SetHandle(t *testing.T) {
	t.Run("replaces the handle", func(t *testing.T) {
		usr := mustUser(t)

		require.NoError(t, usr.SetHandle("next"))
		require.NotNil(t, usr.Handle())
		assert.Equal(t, "next", *usr.Handle())
	})

	t.Run("an empty handle clears it", func(t *testing.T) {
		usr := mustUser(t)

		require.NoError(t, usr.SetHandle(""))
		assert.Nil(t, usr.Handle(), "handles are nilable so a user can have none")
	})

	t.Run("rejects a blank handle", func(t *testing.T) {
		usr := mustUser(t)

		require.ErrorIs(t, usr.SetHandle("   "), domain.ErrInvalid)
		require.NotNil(t, usr.Handle(), "a rejected change must not mutate the user")
		assert.Equal(t, "handle", *usr.Handle())
	})
}

func TestUser_MutatorsPreserveIdentity(t *testing.T) {
	// The load-modify-save cycle relies on this: nothing a caller can edit
	// touches the ID or CreatedAt stamped at construction.
	usr := mustUser(t)
	id, createdAt := usr.ID(), usr.CreatedAt()

	next, err := domain.NewEmail("next@example.org")
	require.NoError(t, err)
	require.NoError(t, usr.ChangeEmail(next))
	require.NoError(t, usr.Rename("Ada", "Lovelace"))
	require.NoError(t, usr.SetHandle("ada"))

	assert.Equal(t, id, usr.ID())
	assert.Equal(t, createdAt, usr.CreatedAt())
}
