package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/common"
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
	// A replacement is a compare-and-set on [common.Aggregate.Version]: it
	// succeeds only if the stored User is still at the version u was loaded at,
	// and on success advances u to the newly stored version.
	//
	// Returns [common.ErrExists] if another User already holds the email or
	// username, and [common.ErrConflict] if the stored User has moved on.
	Save(context.Context, *User) error

	// FindAll returns all Users matching the filter.
	FindAll(context.Context, UserFilter) ([]User, error)

	// Find returns the user matching the UserID.
	//
	// Returns [common.ErrNotFound] if no such entity exists.
	Find(context.Context, UserID) (User, error)

	// Delete removes a user from the collection.
	Delete(context.Context, UserID) error
}

type User struct {
	common.Aggregate[UserID]

	firstName string
	lastName  string
	email     Email
	handle    UserHandle // v1alpha calls this "username"

	profile UserProfile
}

type UserOption func(*User) error

func NewUser(firstName, lastName string, email Email, opts ...UserOption) (User, error) {
	var errs []error
	if err := validateName(firstName, lastName); err != nil {
		errs = append(errs, err)
	}
	if email.IsZero() {
		errs = append(errs, fmt.Errorf("%w: email required", common.ErrInvalid))
	}

	u := User{
		Aggregate: common.NewAggregate(NewUserID()),
		firstName: firstName,
		lastName:  lastName,
		email:     email,
		profile:   UserProfile{data: make(map[string]any)},
	}

	for _, opt := range opts {
		errs = append(errs, opt(&u))
	}

	if err := errors.Join(errs...); err != nil {
		return User{}, err
	}

	return u, nil
}

func WithBio(s string) UserOption {
	return func(u *User) error {
		return u.Profile().SetBio(s)
	}
}

// RehydrateUser skips validation. Only repositories should call this.
// It lives in the domain package so it can access unexported fields.
func rehydrateUser(id UserID, firstName, lastName string, email Email, handle UserHandle, createdAt time.Time, version common.Version) User {
	profileData := make(map[string]any)
	// TODO: rehydrate profile
	return User{
		Aggregate: common.RehydrateAggregate(id, createdAt, version),
		firstName: firstName, lastName: lastName,
		email: email, handle: handle, profile: UserProfile{data: profileData},
	}
}

// Getters — domain exposes read-only access.
// ID and CreatedAt are promoted from the embedded [Aggregate].
func (u User) FirstName() string    { return u.firstName }
func (u User) LastName() string     { return u.lastName }
func (u User) Email() Email         { return u.email }
func (u User) Handle() UserHandle   { return u.handle }
func (u User) Profile() UserProfile { return u.profile }

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
		return fmt.Errorf("%w: email required", common.ErrInvalid)
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

var (
	userProfileData = struct {
		Bio string
	}{
		Bio: "bio",
	}
)

type UserProfile struct {
	data map[string]any
}

func (up UserProfile) set(key string, value any) {
	up.data[key] = value
}

func (up UserProfile) delete(key string) {
	up.data[key] = nil
}

// UserProfile - Getters & Setters
const maxBioSize = 1024 * 1024     // 1 MB
func (up UserProfile) Bio() string { return digOrZero[string](up.data, userProfileData.Bio) }
func (up UserProfile) ClearBio()   { up.delete(userProfileData.Bio) }
func (up UserProfile) SetBio(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fmt.Errorf("%w: cannot be empty", common.ErrInvalid)
	}
	if len(s) > maxBioSize {
		return fmt.Errorf("%w: must be less than 1 MB", common.ErrInvalid)
	}

	up.set(userProfileData.Bio, s)
	return nil
}

func validateName(first, last string) error {
	if first == "" || last == "" {
		return fmt.Errorf("%w: first and last name required", common.ErrInvalid)
	}
	return nil
}

func digOrZero[T any](m map[string]any, key string) T {
	var zero T
	data, exist := m[key]
	if !exist || data == nil {
		return zero
	}
	v, ok := data.(T)
	if !ok {
		return zero
	}
	return v
}
