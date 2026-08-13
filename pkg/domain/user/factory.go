package user

import (
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/domain/internal/lock"
)

// UserFactory allows infrastructure to reconstruct users without
// accessing unexported fields directly.
type UserFactory struct{}

func (UserFactory) Rehydrate(id UserID, firstName, lastName string, email Email, handle UserHandle, createdAt time.Time, version common.Version) User {
	return rehydrateUser(id, firstName, lastName, email, handle, createdAt, version)
}

// NextVersion advances u to the version it holds once persisted. Only a
// repository should call it, and only after its compare-and-set on Version has
// passed, so the aggregate the caller holds keeps matching the stored revision
// and can be edited again without a reload.
//
// This is the only door to [common.Aggregate.NextVersion] from outside the
// domain: that method wants a [lock.Key], and an adapter cannot import the
// package that mints one.
func (UserFactory) NextVersion(u *User) { u.Aggregate.NextVersion(lock.New()) }
