package mapper

import (
	"fmt"
	"strings"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain"
	dto "github.com/samwisebuze/dmost/pkg/dto/v1alpha"
)

var userFactory domain.UserFactory

// Inbound: JSON → Domain
func UserFromCreateRequest(req dto.CreateUserRequest) (domain.User, error) {
	// v1alpha sends "Name" as "First Last"; split it for the domain
	parts := strings.SplitN(strings.TrimSpace(req.Name), " ", 2)
	firstName := parts[0]
	lastName := ""
	if len(parts) > 1 {
		lastName = parts[1]
	}

	email, err := domain.NewEmail(req.Email)
	if err != nil {
		return domain.User{}, fmt.Errorf("%w: %w", domain.ErrInvalid, err)
	}

	// "Username" in v1alpha maps to "Handle" in domain
	return domain.NewUser(firstName, lastName, email, req.Username)
}

// Inbound: JSON → Domain
func UserResponseToUser(res dto.UserResponse) domain.User {
	email, _ := domain.NewEmail(res.Email)
	createdAt, _ := time.Parse(time.RFC3339, res.CreatedAt)
	return userFactory.Rehydrate(domain.UserID(res.ID), res.FirstName, res.LastName, email, res.Username, createdAt)
}

// Outbound: Domain → JSON
func UserToResponse(u domain.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        u.ID().String(),
		FirstName: u.FirstName(),
		LastName:  u.LastName(),
		Email:     u.Email().String(),
		Username:  u.Handle(),
		CreatedAt: u.CreatedAt().Format(time.RFC3339),
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
