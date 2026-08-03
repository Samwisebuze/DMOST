package inmem

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/samwisebuze/dmost/pkg/domain"
)

type UserRepository struct {
	data map[string]*domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		data: map[string]*domain.User{},
	}
}

var _ domain.UserRepository = (*UserRepository)(nil)

// Save implements [domain.Repository].
func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	if _, found := r.data[u.ID().String()]; found {
		return errors.New("id collision")
	}

	for _, usr := range r.data {
		if usr.Email() == u.Email() || usr.Handle() == u.Handle() {
			return domain.ErrExists
		}
	}

	cpy := *u
	r.data[u.ID().String()] = &cpy
	return nil
}

// FindAll implements [domain.Repository].
func (r *UserRepository) FindAll(_ context.Context, _ domain.UserFilter) ([]domain.User, error) {
	users := make([]domain.User, 0, len(r.data))
	for _, u := range r.data {
		users = append(users, *u)
	}

	slices.SortStableFunc(users, func(a, b domain.User) int { return strings.Compare(a.ID().String(), b.ID().String()) })
	return users, nil
}
