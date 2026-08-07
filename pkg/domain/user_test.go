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
