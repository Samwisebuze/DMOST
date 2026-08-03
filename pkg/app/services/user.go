package services

import (
	"context"

	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha/mapper"
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
