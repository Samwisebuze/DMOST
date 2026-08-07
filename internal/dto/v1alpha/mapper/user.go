// Package mapper is the only bridge between the [v1alpha] wire types and the
// domain. Every translation in either direction lives here, so that a change
// to the v1alpha contract stops at this boundary instead of reaching the
// domain — and so that no v1alpha type leaks past it.
//
// The DTO package is imported aliased as dto so that call sites read as a
// direction (dto.CreateUserRequest to domain.User) rather than as a version.
package mapper

import (
	"errors"
	"fmt"
	"strings"
	"time"

	dto "github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/domain"
)

var userFactory domain.UserFactory

// Inbound: JSON → Domain
func UserFromCreateRequest(req dto.CreateUserRequest) (domain.User, error) {
	var errs []error
	firstName, lastName := splitName(req.Name)

	email, err := domain.NewEmail(req.Email)
	if err != nil {
		errs = append(errs, err)
	}

	usr, err := domain.NewUser(firstName, lastName, email)
	if err != nil {
		errs = append(errs, err)
	}
	// "Username" in v1alpha maps to "Handle" in domain
	errs = append(errs, usr.SetHandle(req.Username))

	if err := errors.Join(errs...); err != nil {
		return domain.User{}, err
	}

	return usr, nil
}

// Inbound: JSON → Domain
//
// ApplyUpdateRequest applies req's populated fields to u through the domain's
// mutators, leaving omitted ones untouched. It takes an existing User rather
// than building one so identity and CreatedAt ride along on the aggregate the
// caller loaded, instead of coming from the request.
func ApplyUpdateRequest(u *domain.User, req dto.UpdateUserRequest) error {
	if req.Name != nil {
		if err := u.Rename(splitName(*req.Name)); err != nil {
			return err
		}
	}
	if req.Email != nil {
		email, err := domain.NewEmail(*req.Email)
		if err != nil {
			return fmt.Errorf("%w: %w", domain.ErrInvalid, err)
		}
		if err := u.ChangeEmail(email); err != nil {
			return err
		}
	}
	if req.Username != nil {
		if err := u.SetHandle(*req.Username); err != nil {
			return err
		}
	}
	return nil
}

// splitName splits v1alpha's combined "First Last" into the domain's two
// fields. A name with no space yields an empty last name, which the domain
// rejects — the split does not decide validity.
func splitName(name string) (first, last string) {
	parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

// Inbound: JSON → Domain
func UserResponseToUser(res dto.UserResponse) domain.User {
	email, _ := domain.NewEmail(res.Email)
	username, _ := domain.NewUserHandle(*res.Username)
	createdAt, _ := time.Parse(time.RFC3339, res.CreatedAt)
	return userFactory.Rehydrate(domain.UserID(res.ID), res.FirstName, res.LastName, email, username, createdAt, res.Version)
}

// Outbound: Domain → JSON
func UserToResponse(u domain.User) dto.UserResponse {
	var handle *string
	if !u.Handle().IsZero() {
		handle = new(u.Handle().String())
	}
	return dto.UserResponse{
		ID:        u.ID().String(),
		FirstName: u.FirstName(),
		LastName:  u.LastName(),
		Email:     u.Email().String(),
		Username:  handle,
		CreatedAt: u.CreatedAt().Format(time.RFC3339),
		// Without this the client reads version 0 and can never make a
		// conditional update stick.
		Version: u.Version(),
	}
}

func UserCollectionToResponse(usrs []domain.User) dto.UsersListResponse {
	data := make([]dto.UserResponse, 0, len(usrs))
	for _, u := range usrs {
		data = append(data, UserToResponse(u))
	}
	return dto.UsersListResponse{
		Data:  data,
		Count: len(data),
	}
}

func UserListResponseToUsers(res dto.UsersListResponse) []domain.User {
	data := make([]domain.User, 0, res.Count)
	for _, u := range res.Data {
		data = append(data, UserResponseToUser(u))
	}
	return data
}
