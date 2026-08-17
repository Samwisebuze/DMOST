package app

import (
	"context"

	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/user"
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
	Create(context.Context, v1alpha.CreateUserRequest) (user.User, error)

	// Update loads the User, applies the request's populated fields, and saves
	// it back. Attributes the request omits — including CreatedAt, which no
	// request can carry — survive on the loaded aggregate.
	//
	// Errors come from pkg/domain/common: ErrNotFound if no such User exists,
	// ErrExists if the edit collides with another User, ErrConflict if the
	// request carries a Version the stored User has moved past, and ErrInvalid
	// if it carries a Version no client could have read.
	Update(context.Context, user.UserID, v1alpha.UpdateUserRequest) (user.User, error)

	FindAll(context.Context) ([]user.User, error)
	Find(context.Context, string) (user.User, error)
}

// CharacterService handles CRUD for the Character resource.
//
// It is narrower than [UserService] because the port it drives is narrower:
// [character.CharacterRepository] has Save and Find and nothing else, so there
// is no listing or deletion to expose yet. The same note about v1alpha in the
// signatures applies here — see [UserService.Create].
type CharacterService interface {
	// Create validates the request's sheet against the v1alpha character
	// schema and stores it verbatim. The bytes the client sent are the bytes
	// that come back out; the schema types are a validation gate, not a model
	// the sheet is round-tripped through.
	//
	// Returns an error wrapping [common.ErrInvalid] if the sheet is missing,
	// is not JSON, or does not satisfy the schema.
	Create(context.Context, v1alpha.CreateCharacterRequest) (character.Character, error)

	// Update loads the Character and replaces its sheet with the request's, if
	// the request carries one. A sheet replaces whole — v1alpha has no patch
	// shape for it — while CreatedAt and identity survive on the loaded
	// aggregate.
	//
	// Errors come from pkg/domain/common: ErrNotFound if no such Character
	// exists, ErrConflict if the request carries a Version the stored
	// Character has moved past, and ErrInvalid if the sheet fails validation
	// or the request carries a Version no client could have read.
	Update(context.Context, character.CharacterID, v1alpha.UpdateCharacterRequest) (character.Character, error)

	// Patch loads the Character and merges the request's JSON Merge Patch
	// (RFC 7396) into its sheet, if the request carries one. It is Update's
	// partial counterpart: a patch names only what it changes, a null in it
	// deletes the key it names, and an array in it replaces the array it lands
	// on. The merged result goes through the same schema gate a replacement
	// does, so a patch that deletes a required section is refused.
	//
	// Unlike Create and Update, this does not store the client's bytes: the
	// merge decodes and re-encodes, so a patched sheet comes back with object
	// keys sorted and insignificant whitespace gone. Fields the v1alpha type
	// does not know about survive, and numbers keep their own literal text;
	// their position and the sheet's original formatting do not. Byte-for-byte
	// fidelity to what was stored is lost the first time anything is patched,
	// so a client that needs its exact bytes preserved uses Create or Update's
	// replace path instead — those store them.
	// See [v1alpha.PatchCharacterRequest].
	//
	// Errors come from pkg/domain/common: ErrNotFound if no such Character
	// exists, ErrConflict if the request carries a Version the stored
	// Character has moved past, and ErrInvalid if the patch is not an object,
	// names a server-owned field, or merges into a sheet the schema refuses.
	Patch(context.Context, character.CharacterID, v1alpha.PatchCharacterRequest) (character.Character, error)

	// Find takes the ID unparsed, so a malformed one is an ErrInvalid from
	// this layer rather than something the transport has to pre-check.
	Find(context.Context, string) (character.Character, error)
}
