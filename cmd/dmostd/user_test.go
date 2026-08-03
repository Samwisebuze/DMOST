package main_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func MustCreateUser(t testing.TB, cli *http.Client) domain.User {
	t.Helper()
	uid := uuid.NewString()
	req := v1alpha.CreateUserRequest{
		Name:     "firstName lastName",
		Email:    "test" + uid + "@example.org",
		Username: "test_" + uid,
	}
	usr, err := cli.Create(context.Background(), req)
	require.NoError(t, err)
	return usr
}

func TestUsersAPI_Create(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	req := v1alpha.CreateUserRequest{
		Name:     "givenName FamilyName",
		Email:    "test@example.org",
		Username: "test_username",
	}
	res, err := cli.Create(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, req.Email, res.Email().String())
	assert.Contains(t, req.Name, res.FirstName())
	assert.Contains(t, req.Name, res.LastName())
	assert.Equal(t, req.Username, *res.Handle())
}

func TestUsersAPI_List(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	want := []domain.User{
		MustCreateUser(t, cli),
		MustCreateUser(t, cli),
	}

	got, err := cli.ListAll(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 2)
	assert.Equal(t, want, got)
}
