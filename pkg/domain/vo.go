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
