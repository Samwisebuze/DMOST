// Package v1alpha is the first, unstable version of the wire contract. It is
// frozen against edits once a client depends on it: see
// [github.com/samwisebuze/dmost/internal/dto] for what belongs here and why a
// change means a new sibling version rather than a change to this one.
//
// Translation to and from the domain lives in the mapper subpackage.
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

// UpdateUserRequest mirrors [CreateUserRequest], but every field is a pointer:
// a field the client omits is left unchanged rather than cleared. Sending
// "username": "" is therefore distinct from omitting it, and clears the handle.
type UpdateUserRequest struct {
	Name     *string `json:"name"`     // "First Last" combined
	Email    *string `json:"email"`    //
	Username *string `json:"username"` // maps to domain "handle"

	// Version is the revision the client last read, echoed back from
	// [UserResponse]. Supplying it makes the update conditional: it is refused
	// if someone else has written since. Omitting it means last writer wins —
	// allowed, because v1alpha shipped without the field, but a client editing
	// what it just read should send it.
	Version *uint64 `json:"version"`
}

type UserResponse struct {
	ID        string  `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Username  *string `json:"username"`
	CreatedAt string  `json:"created_at"`

	// Version is the revision this representation was read at. Echo it in an
	// [UpdateUserRequest] to make the write conditional. Unsigned so a negative
	// number in a payload fails to decode rather than wrapping.
	Version uint64 `json:"version"`
}

type UsersListResponse Collection[UserResponse]
