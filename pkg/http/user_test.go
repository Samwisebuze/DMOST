package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/samwisebuze/dmost/pkg/http/problem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeUserService struct {
	FindAllFn func(context.Context) ([]domain.User, error)
	CreateFn  func(context.Context, v1alpha.CreateUserRequest) (domain.User, error)
}

// FindAll implements [domain.Repository].
func (f FakeUserService) FindAll(ctx context.Context) ([]domain.User, error) {
	return f.FindAllFn(ctx)
}

// Save implements [domain.Repository].
func (f FakeUserService) Create(ctx context.Context, r v1alpha.CreateUserRequest) (domain.User, error) {
	return f.CreateFn(ctx, r)
}

var _ app.UserService = FakeUserService{}

func TestCreateHandler_ReturnsSuccess(t *testing.T) {
	app := app.New()
	app.UserService = FakeUserService{
		CreateFn: func(ctx context.Context, cur v1alpha.CreateUserRequest) (domain.User, error) {
			return domain.User{}, nil
		},
	}
	handler := http.CreateHandler(app)
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
	assert.Equal(t, resp.StatusCode, 201)
	assert.Equal(t, resp.Header.Get("Content-Type"), v1alpha.ContentTypeUserJSON)
}

func TestCreateHandler_Returns500OnError(t *testing.T) {
	app := app.New()
	app.UserService = FakeUserService{
		CreateFn: func(ctx context.Context, cur v1alpha.CreateUserRequest) (domain.User, error) {
			return domain.User{}, assert.AnError
		},
	}

	handler := http.CreateHandler(app)
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
	assert.Equal(t, resp.StatusCode, 500)
	assert.Equal(t, resp.Header.Get("Content-Type"), problem.ContentTypeJSON)
}

func TestCreateHandler_Returns422OnErrInvalid(t *testing.T) {
	var app app.App
	app.UserService = FakeUserService{
		CreateFn: func(ctx context.Context, cur v1alpha.CreateUserRequest) (domain.User, error) {
			return domain.User{}, domain.ErrInvalid
		},
	}

	handler := http.CreateHandler(&app)
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
	assert.Equal(t, resp.StatusCode, 422)
	assert.Equal(t, resp.Header.Get("Content-Type"), problem.ContentTypeJSON)
}

func TestCreateHandler_Returns400OnInvalidRequest(t *testing.T) {
	var app app.App

	handler := http.CreateHandler(&app)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", strings.NewReader(""))
	handler(w, r)
	resp := w.Result()
	assert.Equal(t, resp.StatusCode, 400)
	assert.Equal(t, resp.Header.Get("Content-Type"), problem.ContentTypeJSON)
}
