package app

import (
	"context"

	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/domain"
)

// UserService handles CRUD for the User resource.
type UserService interface {
	// Create takes a v1alpha.CreateUserRequest rather than a domain type
	// or a hand-rolled command struct, and the service calls into
	// pkg/dto/v1alpha/mapper to reach a domain.User. That is
	// deliberate **for now**: the mapper is where request validation already lives,
	// and duplicating it as a command type would buy little while there is one
	// wire version. It does mean this layer is pinned to v1alpha. When a second
	// version lands, the fix is a version-neutral command struct per use case,
	// with each dto version mapping into it — not a second Create method.
	Create(context.Context, v1alpha.CreateUserRequest) (domain.User, error)

	// Update loads the User, applies the request's populated fields, and saves
	// it back. Attributes the request omits — including CreatedAt, which no
	// request can carry — survive on the loaded aggregate.
	//
	// Returns [domain.ErrNotFound] if no such User exists, [domain.ErrExists]
	// if the edit collides with another User, and [domain.ErrConflict] if the
	// request carries a Version the stored User has moved past.
	Update(context.Context, domain.UserID, v1alpha.UpdateUserRequest) (domain.User, error)

	FindAll(context.Context) ([]domain.User, error)
}
