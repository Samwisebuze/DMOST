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

func TestUsersAPI_Update(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	usr := MustCreateUser(t, cli)

	name := "Ada Lovelace"
	got, err := cli.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{
		Name:    &name,
		Version: ptr(usr.Version()),
	})
	require.NoError(t, err)

	assert.Equal(t, "Ada", got.FirstName())
	assert.Equal(t, "Lovelace", got.LastName())
	assert.Equal(t, usr.ID(), got.ID())
	assert.Equal(t, usr.Email(), got.Email(), "an omitted field must survive the round trip")
	assert.Equal(t, usr.CreatedAt(), got.CreatedAt())
	assert.Equal(t, usr.Version()+1, got.Version(), "the client must learn the new revision")
}

func TestUsersAPI_UpdateRejectsAStaleVersion(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	usr := MustCreateUser(t, cli)
	stale := usr.Version()

	first := "Ada Lovelace"
	_, err := cli.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: &first, Version: ptr(stale)})
	require.NoError(t, err)

	// A second client still holding the pre-update representation.
	second := "Grace Hopper"
	_, err = cli.Update(context.Background(), usr.ID(), v1alpha.UpdateUserRequest{Name: &second, Version: ptr(stale)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Conflict", "a lost update must surface as 409, not a silent overwrite")

	got, err := cli.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Ada", got[0].FirstName(), "the losing write must not land")
}

func TestUsersAPI_UpdateUnknownUser(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	name := "Ada Lovelace"
	_, err := cli.Update(context.Background(), domain.NewUserID(), v1alpha.UpdateUserRequest{Name: &name})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
}

func ptr[T any](v T) *T { return &v }

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
