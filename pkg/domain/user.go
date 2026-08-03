package domain

import (
	"context"
	"errors"
	"time"
)

type UserRepository interface {
	// Save inserts a new User.
	//
	// Returns [ErrExists] on duplicate email or username.
	Save(context.Context, *User) error

	// FindAll returns all Users matching the filter.
	FindAll(context.Context, UserFilter) ([]User, error)
}

type User struct {
	id        UserID
	firstName string
	lastName  string
	email     Email
	handle    *string // v1alpha calls this "username"
	createdAt time.Time
}

func NewUser(firstName, lastName string, email Email, handle string) (User, error) {
	if firstName == "" || lastName == "" {
		return User{}, errors.New("first and last name required")
	}
	if email.IsZero() {
		return User{}, errors.New("email required")
	}

	var username *string
	if handle != "" {
		username = &handle
	}
	return User{
		id:        NewUserID(),
		firstName: firstName,
		lastName:  lastName,
		email:     email,
		handle:    username,
		createdAt: time.Now().UTC(),
	}, nil
}

// RehydrateUser skips validation. Only repositories should call this.
// It lives in the domain package so it can access unexported fields.
func rehydrateUser(id UserID, firstName, lastName string, email Email, handle *string, createdAt time.Time) User {
	return User{
		id: id, firstName: firstName, lastName: lastName,
		email: email, handle: handle, createdAt: createdAt,
	}
}

// Getters — domain exposes read-only access
func (u User) ID() UserID           { return u.id }
func (u User) FirstName() string    { return u.firstName }
func (u User) LastName() string     { return u.lastName }
func (u User) Email() Email         { return u.email }
func (u User) Handle() *string      { return u.handle }
func (u User) CreatedAt() time.Time { return u.createdAt }

type UserFilter struct{}
