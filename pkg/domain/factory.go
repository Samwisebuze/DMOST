package domain

import "time"

// UserFactory allows infrastructure to reconstruct users without
// accessing unexported fields directly.
type UserFactory struct{}

func (UserFactory) Rehydrate(id UserID, firstName, lastName string, email Email, handle *string, createdAt time.Time) User {
	return rehydrateUser(id, firstName, lastName, email, handle, createdAt)
}
