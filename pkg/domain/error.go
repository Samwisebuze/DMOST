package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid  = errors.New("invalid")
	ErrExists   = fmt.Errorf("%w: resource exists", ErrInvalid)
	ErrNotFound = fmt.Errorf("resource not found")

	// ErrConflict reports a lost update: the caller's aggregate was loaded at a
	// version the store has since moved past. It does not wrap ErrInvalid —
	// nothing about the request is malformed, the caller simply raced someone
	// else and should reload and retry.
	ErrConflict = errors.New("version conflict")
)
