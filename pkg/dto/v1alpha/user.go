package v1alpha

const ContentTypeJSON string = "application/json"
const ContentTypeUserJSON string = "application/vnd.dmost.user.v1alpha+json"
const ContentTypeUserListJSON string = "application/vnd.dmost.user_list.v1alpha+json"

// CreateUserRequest — flat, simple, "alpha" feel
type CreateUserRequest struct {
	Name     string `json:"name"` // "First Last" combined
	Email    string `json:"email"`
	Username string `json:"username"` // maps to domain "handle"
}

type UserResponse struct {
	ID        string  `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Username  *string `json:"username"`
	CreatedAt string  `json:"created_at"`
}

type UsersListResponse Collection[UserResponse]
