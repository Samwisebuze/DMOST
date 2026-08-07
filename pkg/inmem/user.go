package inmem

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/samwisebuze/dmost/pkg/domain"
)

type UserRepository struct {
	data map[domain.UserID]*domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		data: map[domain.UserID]*domain.User{},
	}
}

var _ domain.UserRepository = (*UserRepository)(nil)

// Save implements [domain.Repository].
func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	if _, found := r.data[u.ID()]; found {
		return errors.New("id collision")
	}

	for _, usr := range r.data {
		if usr.Email().Equal(u.Email()) {
			return domain.ErrExists
		}
		// Handles are optional; compare the values, not the pointers, and let
		// two users without a handle coexist.
		if a, b := usr.Handle(), u.Handle(); a != nil && b != nil && *a == *b {
			return domain.ErrExists
		}
	}

	cpy := *u
	r.data[u.ID()] = &cpy
	return nil
}

// Find implements [domain.Repository].
func (r *UserRepository) Find(_ context.Context, id domain.UserID) (domain.User, error) {
	u, found := r.data[id]
	if !found {
		return domain.User{}, domain.ErrNotFound
	}
	return *u, nil
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

// Delete implements [domain.Repository].
func (r *UserRepository) Delete(ctx context.Context, id domain.UserID) error {
	if _, found := r.data[id]; !found {
		return nil
	}

	delete(r.data, id)
	return nil
}
