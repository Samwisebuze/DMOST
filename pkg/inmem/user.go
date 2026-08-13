package inmem

import (
	"context"
	"slices"
	"sync"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	domain "github.com/samwisebuze/dmost/pkg/domain/user"
)

type UserRepository struct {
	mu   sync.RWMutex
	data map[domain.UserID]*domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		data: map[domain.UserID]*domain.User{},
	}
}

var _ domain.UserRepository = (*UserRepository)(nil)

// userFactory is the domain's door for adapters: Save reaches through it to
// advance an aggregate's version, which is not something callers may do.
var userFactory domain.UserFactory

// Save implements [domain.Repository].
func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	// The lock spans the duplicate scan and the insert: splitting them would
	// let two concurrent Saves both pass the uniqueness check.
	r.mu.Lock()
	defer r.mu.Unlock()

	// An existing ID is the update path, not a collision — but only if the
	// caller loaded the revision still on record. Two callers that both read
	// version N and write back would otherwise silently lose one edit; the
	// second one's compare-and-set fails here instead.
	cur, update := r.data[u.ID()]
	if update && !cur.Version().Equal(u.Version()) {
		return common.ErrConflict
	}

	for _, usr := range r.data {
		// Skipping the record being written keeps a User from conflicting with
		// its own email or handle.
		if usr.ID().Equal(u.ID()) {
			continue
		}
		if usr.Email().Equal(u.Email()) {
			return common.ErrExists
		}
		if u.Handle().Equal(usr.Handle()) {
			return common.ErrExists
		}
	}

	// An insert stores the version the aggregate was constructed with; only a
	// replacement advances it. Advancing u itself, not just the stored copy,
	// lets the caller edit and save again without reloading.
	if update {
		userFactory.NextVersion(u)
	}

	cpy := *u
	r.data[u.ID()] = &cpy
	return nil
}

// Find implements [domain.Repository].
func (r *UserRepository) Find(_ context.Context, id domain.UserID) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, found := r.data[id]
	if !found {
		return domain.User{}, common.ErrNotFound
	}
	return *u, nil
}

// FindAll implements [domain.Repository].
func (r *UserRepository) FindAll(_ context.Context, _ domain.UserFilter) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	users := make([]domain.User, 0, len(r.data))
	for _, u := range r.data {
		users = append(users, *u)
	}

	slices.SortStableFunc(users, func(a, b domain.User) int { return a.ID().Compare(b.ID()) })
	return users, nil
}

// Delete implements [domain.Repository].
func (r *UserRepository) Delete(ctx context.Context, id domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}
