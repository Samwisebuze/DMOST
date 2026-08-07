package domain

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

type UserID string

func NewUserID() UserID { return UserID(uuid.Must(uuid.NewV7()).String()) }

func (uid UserID) String() string {
	return string(uid)
}

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
