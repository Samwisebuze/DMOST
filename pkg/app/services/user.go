package services

import (
	"context"
	"fmt"

	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha/mapper"
	"github.com/samwisebuze/dmost/pkg/domain"
)

type UserService struct {
	Users domain.UserRepository
}

func NewUserService(users domain.UserRepository) *UserService {
	if users == nil {
		panic("user repo must be set")
	}

	return &UserService{
		Users: users,
	}

}

// Find implements [app.UserService].
func (u *UserService) Find(ctx context.Context, raw string) (domain.User, error) {
	id, err := domain.ParseUserID(raw)
	if err != nil {
		return domain.User{}, err
	}

	user, err := u.Users.Find(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return user, nil
}

// FindAll implements [app.UserService].
func (u *UserService) FindAll(ctx context.Context) ([]domain.User, error) {
	users, err := u.Users.FindAll(ctx, domain.UserFilter{})
	if err != nil {
		return nil, err
	}

	return users, nil
}

// Save implements [app.UserService].
func (u *UserService) Create(ctx context.Context, req v1alpha.CreateUserRequest) (domain.User, error) {
	usr, err := mapper.UserFromCreateRequest(req)
	if err != nil {
		return domain.User{}, err
	}

	if err := u.Users.Save(ctx, &usr); err != nil {
		return domain.User{}, err
	}

	return usr, nil
}

// Update implements [app.UserService].
func (u *UserService) Update(ctx context.Context, id domain.UserID, req v1alpha.UpdateUserRequest) (domain.User, error) {
	usr, err := u.Users.Find(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	// The repository's compare-and-set only sees the window between the Find
	// above and the Save below. Checking the client's expected version here is
	// what catches the slower race: two clients that both read version N,
	// think, and write back.
	if req.Version != nil && *req.Version != usr.Version() {
		return domain.User{}, fmt.Errorf("%w: user was modified since version %d", domain.ErrConflict, *req.Version)
	}

	if err := mapper.ApplyUpdateRequest(&usr, req); err != nil {
		return domain.User{}, err
	}

	// Save is an upsert keyed by UserID, so this replaces the record loaded
	// above rather than inserting a second one.
	if err := u.Users.Save(ctx, &usr); err != nil {
		return domain.User{}, err
	}

	return usr, nil
}
