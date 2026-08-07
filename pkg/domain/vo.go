package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type UserID string

func NewUserID() UserID { return UserID(uuid.Must(uuid.NewV7()).String()) }

func (uid UserID) String() string       { return string(uid) }
func (uid UserID) Compare(o UserID) int { return strings.Compare(string(uid), string(o)) }

// Equal reports whether two IDs identify the same user. Callers should prefer
// it over ==, so the comparison stays correct if the representation changes.
func (uid UserID) Equal(other UserID) bool {
	return uid == other
}

type Email struct {
	value string
}

func (e Email) IsZero() bool {
	return e == Email{}
}

func NewEmail(raw string) (Email, error) {
	if !strings.Contains(raw, "@") {
		return Email{}, errors.New("invalid email")
	}
	return Email{value: strings.ToLower(raw)}, nil
}
func (e Email) String() string { return e.value }

// Equal reports whether two addresses are the same. Values built by NewEmail
// are already lowercased, so this is a plain comparison of canonical forms.
func (e Email) Equal(other Email) bool {
	return e == other
}

type UserHandle struct {
	value string
}

func NewUserHandle(raw string) (UserHandle, error) {
	if raw == "" {
		return UserHandle{}, nil
	}
	if strings.TrimSpace(raw) == "" {
		return UserHandle{}, fmt.Errorf("%w: handle must not be blank", ErrInvalid)
	}
	norm := strings.ToLower(strings.TrimSpace(raw))

	if utf8.RuneCountInString(norm) > maxHandleChars {
		return UserHandle{}, fmt.Errorf("%w: exceeds max length (%v)", ErrInvalid, maxHandleChars)
	}

	return UserHandle{value: norm}, nil
}

func (uh UserHandle) String() string { return uh.value }
func (uh UserHandle) IsZero() bool   { return uh == UserHandle{} }
func (uh UserHandle) Equal(o UserHandle) bool {
	// zero represents an unset handle, they can coexist
	if uh.IsZero() || o.IsZero() {
		return false
	}

	return uh.value == o.value
}
