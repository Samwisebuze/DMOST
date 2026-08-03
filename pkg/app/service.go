package app

import (
	"context"

	"github.com/samwisebuze/dmost/pkg/domain"
	"github.com/samwisebuze/dmost/pkg/dto/v1alpha"
)

// Service implements use-cases.
type UserService interface {
	Create(context.Context, v1alpha.CreateUserRequest) (domain.User, error)
	FindAll(context.Context) ([]domain.User, error)
}
