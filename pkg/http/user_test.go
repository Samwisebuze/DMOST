package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	domain "github.com/samwisebuze/dmost/pkg/domain/user"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/samwisebuze/dmost/pkg/http/problem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeUserService struct {
	FindFn    func(context.Context, string) (domain.User, error)
	FindAllFn func(context.Context) ([]domain.User, error)
	CreateFn  func(context.Context, v1alpha.CreateUserRequest) (domain.User, error)
	UpdateFn  func(context.Context, domain.UserID, v1alpha.UpdateUserRequest) (domain.User, error)
}

var _ app.UserService = FakeUserService{}

// Find implements [app.UserService].
func (f FakeUserService) Find(ctx context.Context, id string) (domain.User, error) {
	return f.FindFn(ctx, id)
}

// FindAll implements [domain.Repository].
func (f FakeUserService) FindAll(ctx context.Context) ([]domain.User, error) {
	return f.FindAllFn(ctx)
}

// Save implements [domain.Repository].
func (f FakeUserService) Create(ctx context.Context, r v1alpha.CreateUserRequest) (domain.User, error) {
	return f.CreateFn(ctx, r)
}

// Update implements [app.UserService].
func (f FakeUserService) Update(ctx context.Context, id domain.UserID, r v1alpha.UpdateUserRequest) (domain.User, error) {
	return f.UpdateFn(ctx, id, r)
}

var _ app.UserService = FakeUserService{}

func TestCreateHandler_ReturnsSuccess(t *testing.T) {
	app := app.New()
	app.UserService = FakeUserService{
		CreateFn: func(ctx context.Context, cur v1alpha.CreateUserRequest) (domain.User, error) {
			return domain.User{}, nil
		},
	}
	handler := http.CreateUserHandler(app)
	bodyBytes, err := json.Marshal(v1alpha.CreateUserRequest{
		Name:     "first last",
		Email:    "valid@example.org",
		Username: "test",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer(bodyBytes))
	handler(w, r)
	resp := w.Result()
	assert.Equal(t, 201, resp.StatusCode)
	assert.Equal(t, v1alpha.ContentTypeUserJSON, resp.Header.Get("Content-Type"))
}

func TestCreateHandler_Returns500OnError(t *testing.T) {
	app := app.New()
	app.UserService = FakeUserService{
		CreateFn: func(ctx context.Context, cur v1alpha.CreateUserRequest) (domain.User, error) {
			return domain.User{}, assert.AnError
		},
	}

	handler := http.CreateUserHandler(app)
	bodyBytes, err := json.Marshal(v1alpha.CreateUserRequest{
		Name:     "first last",
		Email:    "valid@example.org",
		Username: "test",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer(bodyBytes))
	handler(w, r)
	resp := w.Result()
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
}

func TestCreateHandler_Returns422OnErrInvalid(t *testing.T) {
	var app app.App
	app.UserService = FakeUserService{
		CreateFn: func(ctx context.Context, cur v1alpha.CreateUserRequest) (domain.User, error) {
			return domain.User{}, common.ErrInvalid
		},
	}

	handler := http.CreateUserHandler(&app)
	bodyBytes, err := json.Marshal(v1alpha.CreateUserRequest{
		Name:     "first last",
		Email:    "valid@example.org",
		Username: "test",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer(bodyBytes))
	handler(w, r)
	resp := w.Result()
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
}

// updateRequest builds a PATCH carrying body, with the {id} route variable set
// as the router would have.
func updateRequest(t *testing.T, id domain.UserID, body []byte) *nethttp.Request {
	t.Helper()
	r := httptest.NewRequest(nethttp.MethodPatch, "/users/"+id.String(), bytes.NewBuffer(body))
	return mux.SetURLVars(r, map[string]string{"id": id.String()})
}

func TestUpdateHandler_MapsErrorsToStatus(t *testing.T) {
	name := "Ada Lovelace"
	body, err := json.Marshal(v1alpha.UpdateUserRequest{Name: &name})
	require.NoError(t, err)

	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"success":              {nil, nethttp.StatusOK},
		"unknown user":         {common.ErrNotFound, nethttp.StatusNotFound},
		"stale version":        {common.ErrConflict, nethttp.StatusConflict},
		"invalid edit":         {common.ErrInvalid, nethttp.StatusUnprocessableEntity},
		"duplicate email":      {common.ErrExists, nethttp.StatusUnprocessableEntity},
		"unrecognised failure": {assert.AnError, nethttp.StatusInternalServerError},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			var a app.App
			a.UserService = FakeUserService{
				UpdateFn: func(context.Context, domain.UserID, v1alpha.UpdateUserRequest) (domain.User, error) {
					return domain.User{}, tc.err
				},
			}

			w := httptest.NewRecorder()
			http.UpdateUserHandler(&a)(w, updateRequest(t, domain.NewUserID(), body))

			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.err == nil {
				assert.Equal(t, v1alpha.ContentTypeUserJSON, resp.Header.Get("Content-Type"))
				return
			}
			assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
		})
	}
}

func TestUpdateHandler_PassesTheIDAndRequestThrough(t *testing.T) {
	id := domain.NewUserID()
	version := uint64(3)
	body, err := json.Marshal(v1alpha.UpdateUserRequest{Version: &version})
	require.NoError(t, err)

	var gotID domain.UserID
	var gotReq v1alpha.UpdateUserRequest
	var a app.App
	a.UserService = FakeUserService{
		UpdateFn: func(_ context.Context, id domain.UserID, req v1alpha.UpdateUserRequest) (domain.User, error) {
			gotID, gotReq = id, req
			return domain.User{}, nil
		},
	}

	http.UpdateUserHandler(&a)(httptest.NewRecorder(), updateRequest(t, id, body))

	assert.Equal(t, id, gotID)
	require.NotNil(t, gotReq.Version, "the expected version must survive decoding, or every update is unconditional")
	assert.EqualValues(t, 3, *gotReq.Version)
	assert.Nil(t, gotReq.Name, "an omitted field must stay omitted")
}

func TestUpdateHandler_Returns400OnInvalidRequest(t *testing.T) {
	var a app.App

	w := httptest.NewRecorder()
	http.UpdateUserHandler(&a)(w, updateRequest(t, domain.NewUserID(), nil))

	resp := w.Result()
	assert.Equal(t, nethttp.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
}

func TestCreateHandler_Returns400OnInvalidRequest(t *testing.T) {
	var app app.App

	handler := http.CreateUserHandler(&app)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", strings.NewReader(""))
	handler(w, r)
	resp := w.Result()
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
}
