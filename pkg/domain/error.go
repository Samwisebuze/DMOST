package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid  = errors.New("invalid")
	ErrExists   = fmt.Errorf("%w: resource exists", ErrInvalid)
	ErrNotFound = fmt.Errorf("resource not found")
)
