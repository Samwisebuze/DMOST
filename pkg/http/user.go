package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha/mapper"
	"github.com/samwisebuze/dmost/pkg/http/problem"
)

func (s *Server) registerUserRoutes(router *mux.Router) {
	r := router.PathPrefix("/users").Subrouter()
	r.Handle("", CreateHandler(s.app)).Methods(http.MethodPost)
	r.Handle("", ListHandler(s.app)).Methods(http.MethodGet)
}

type Client struct {
	client     *http.Client
	urls       urlBuilder
	usrFactory domain.UserFactory

	server string
}

type clientOption func(*Client)

func NewClient(opts ...clientOption) *Client {
	c := &Client{
		client: &http.Client{},
		server: "localhost",
	}
	for _, o := range opts {
		o(c)
	}

	c.urls.Server = c.server
	return c
}

func WithServer(server string) clientOption {
	return func(c *Client) {
		c.server = server
	}
}

type urlBuilder struct {
	Server string
}

func (u urlBuilder) Create() string {
	return fmt.Sprintf("%s/users", u.Server)
}

func (u urlBuilder) ListAll() string {
	return fmt.Sprintf("%s/users", u.Server)
}

func CreateHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req v1alpha.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			problem.New().
				Detail(err.Error()).
				Title("invalid_request").
				Wrap(err).
				Status(http.StatusBadRequest).
				WriteTo(w)
			return
		}

		usr, err := app.UserService.Create(r.Context(), req)
		if errors.Is(err, domain.ErrInvalid) {
			problem.New().
				Wrap(err).
				Of(http.StatusUnprocessableEntity).
				WriteTo(w)
			return
		}
		if err != nil {
			slog.Error(err.Error())
			problem.New().
				Wrap(err).
				Of(http.StatusInternalServerError).
				WriteTo(w)
			return
		}

		data := mapper.UserToResponse(usr)
		w.Header().Set("Content-Type", v1alpha.ContentTypeUserJSON)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(data)
	}
}

func (c *Client) Create(ctx context.Context, req v1alpha.CreateUserRequest) (domain.User, error) {
	raw, err := json.Marshal(req)
	resp, err := c.client.Post(c.urls.Create(), v1alpha.ContentTypeJSON, bytes.NewBuffer(raw))
	if err != nil {
		return domain.User{}, fmt.Errorf("POST %q failed: %w", c.urls.Create(), err)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if resp.StatusCode != 201 {
		var err problem.Problem
		if err := decoder.Decode(&err); err != nil {
			return domain.User{}, fmt.Errorf("POST %q: unprocessable response [code=%q]: %w", c.urls.Create(), resp.Status, err)
		}
		return domain.User{}, fmt.Errorf("POST %q: %w", c.urls.Create(), &err)
	}

	data, err := decode[v1alpha.UserResponse](resp)
	if err != nil {
		return domain.User{}, fmt.Errorf("POST %q: %w", c.urls.Create(), err)
	}

	return mapper.UserResponseToUser(data), nil
}

func ListHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := app.UserService.FindAll(r.Context())
		if err != nil {
			problem.New().
				WrapSilent(err).
				Of(http.StatusInternalServerError).
				WriteTo(w)
			return
		}

		res := mapper.UserCollectionToResponse(users)
		w.Header().Set("Content-Type", v1alpha.ContentTypeUserListJSON)
		json.NewEncoder(w).Encode(res)
	}
}

func (c *Client) ListAll(ctx context.Context) ([]domain.User, error) {
	resp, err := c.client.Get(c.urls.ListAll())
	if err != nil {
		return nil, fmt.Errorf("client error: %w", err)
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	if resp.StatusCode != 200 {
		var err *problem.Problem
		if err := decoder.Decode(err); err != nil {
			return nil, fmt.Errorf("unprocessable response [code=%q content_type=%s, content_len=%v]: %w", resp.Status, resp.Header.Get("Content-Type"), resp.ContentLength, err)
		}
		return nil, err
	}

	data, err := decode[v1alpha.UsersListResponse](resp)
	if err != nil {
		return nil, err
	}

	return mapper.UserListResponseToUsers(data), nil
}

func decode[T any](r *http.Response) (T, error) {
	var data T
	if r.ContentLength == 0 {
		return data, errors.New("empty response")
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		var zero T
		return zero, fmt.Errorf("undecodable content: %w", err)
	}

	return data, nil
}
