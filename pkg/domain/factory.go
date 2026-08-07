package domain

import "time"

// UserFactory allows infrastructure to reconstruct users without
// accessing unexported fields directly.
type UserFactory struct{}

func (UserFactory) Rehydrate(id UserID, firstName, lastName string, email Email, handle *string, createdAt time.Time, version uint64) User {
	return rehydrateUser(id, firstName, lastName, email, handle, createdAt, version)
}

// NextVersion advances u to the version it holds once persisted. Only a
// repository should call it, and only after its compare-and-set on Version has
// passed, so the aggregate the caller holds keeps matching the stored revision
// and can be edited again without a reload.
func (UserFactory) NextVersion(u *User) { u.nextVersion() }
