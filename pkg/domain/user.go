package domain

import (
	"context"
	"fmt"
	"time"
)

const (
	maxHandleChars = 64
)

type UserFilter struct{}

type UserRepository interface {
	// Save persists u, inserting it or replacing the existing User with the
	// same [UserID]. Callers mutate an existing User by loading it with Find,
	// applying changes, and passing it back.
	//
	// A replacement is a compare-and-set on Version: it succeeds only if the
	// stored User is still at the version u was loaded at, and on success
	// advances u to the newly stored version.
	//
	// Returns [ErrExists] if another User already holds the email or username,
	// and [ErrConflict] if the stored User has moved on.
	Save(context.Context, *User) error

	// FindAll returns all Users matching the filter.
	FindAll(context.Context, UserFilter) ([]User, error)

	// Find returns the user matching the UserID.
	//
	// Returns [ErrNotFound] if no such entity exists.
	Find(context.Context, UserID) (User, error)

	// Delete removes a user from the collection.
	Delete(context.Context, UserID) error
}

type User struct {
	Aggregate[UserID]

	firstName string
	lastName  string
	email     Email
	handle    UserHandle // v1alpha calls this "username"
}

func NewUser(firstName, lastName string, email Email) (User, error) {
	if err := validateName(firstName, lastName); err != nil {
		return User{}, err
	}
	if email.IsZero() {
		return User{}, fmt.Errorf("%w: email required", ErrInvalid)
	}

	return User{
		Aggregate: newAggregate(NewUserID()),
		firstName: firstName,
		lastName:  lastName,
		email:     email,
	}, nil
}

// RehydrateUser skips validation. Only repositories should call this.
// It lives in the domain package so it can access unexported fields.
func rehydrateUser(id UserID, firstName, lastName string, email Email, handle UserHandle, createdAt time.Time, version uint64) User {
	return User{
		Aggregate: rehydrateAggregate(id, createdAt, version),
		firstName: firstName, lastName: lastName,
		email: email, handle: handle,
	}
}

// Getters — domain exposes read-only access.
// ID and CreatedAt are promoted from the embedded [Aggregate].
func (u User) FirstName() string  { return u.firstName }
func (u User) LastName() string   { return u.lastName }
func (u User) Email() Email       { return u.email }
func (u User) Handle() UserHandle { return u.handle }

// Mutators — the only way to edit a User outside the domain. They take pointer
// receivers (the getters stay value receivers) and run the same rules as
// [NewUser], so callers never reach for [UserFactory.Rehydrate] to make an
// edit. Identity and CreatedAt are not among them: they are set once, at
// construction, and ride along through a load-modify-save cycle.

// ChangeEmail replaces the user's address.
//
// Returns an error wrapping [ErrInvalid] if e is the zero [Email]. Uniqueness
// is a collection-wide rule, so the repository — not this method — reports a
// conflict with another user's address.
func (u *User) ChangeEmail(e Email) error {
	if e.IsZero() {
		return fmt.Errorf("%w: email required", ErrInvalid)
	}
	u.email = e
	return nil
}

// Rename replaces both name parts together, because a User with one of them
// blank is not a state the domain allows.
//
// Returns an error wrapping [ErrInvalid] if either part is empty.
func (u *User) Rename(first, last string) error {
	if err := validateName(first, last); err != nil {
		return err
	}
	u.firstName, u.lastName = first, last
	return nil
}

// SetHandle sets the optional handle. An empty string clears it — handles are
// nilable precisely so a user can have none.
//
// Returns an error wrapping [ErrInvalid] if handle is non-empty but blank.
func (u *User) SetHandle(handle string) error {
	h, err := NewUserHandle(handle)
	if err != nil {
		return err
	}

	u.handle = h
	return nil
}

func validateName(first, last string) error {
	if first == "" || last == "" {
		return fmt.Errorf("%w: first and last name required", ErrInvalid)
	}
	return nil
}
