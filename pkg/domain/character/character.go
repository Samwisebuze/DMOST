package character

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/character/schema"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

// NOTE: using internal/dto/v1alpha/character temporarily until it is clear if a domain type is warranted

type CharacterRepository interface {
	// Save persists c, inserting it or replacing the existing Character with
	// the same [CharacterID]. Callers mutate an existing Character by loading
	// it with Find, applying changes, and passing it back.
	//
	// A replacement is a compare-and-set on [common.Aggregate.Version]: it
	// succeeds only if the stored Character is still at the version c was
	// loaded at, and on success advances c to the newly stored version.
	//
	// Returns [common.ErrConflict] if the stored Character has moved on.
	Save(context.Context, *Character) error

	// Find returns the character matching the CharacterID.
	//
	// Returns [common.ErrNotFound] if no such entity exists.
	Find(context.Context, CharacterID) (Character, error)
}

type Character struct {
	common.Aggregate[CharacterID]
	data json.RawMessage
}

// NewCharacter builds a Character around an encoded character sheet.
//
// The sheet is validated against the whole v1alpha character schema, embedded
// in [github.com/samwisebuze/dmost/pkg/domain/character/schema/v1alpha]: a
// Character does not exist unless its sheet carries every required section, at
// every depth. That schema is one document assembled from a dozen files by
// $ref, and it is the same set the v1alpha wire types are generated from, so a
// sheet the DTOs can encode and one the domain will accept stay the same thing.
//
// Returns an error wrapping [common.ErrInvalid] if data is empty, does not
// parse, parses as something other than an object, or parses into an object the
// schema rejects — the last case naming the offending locations. "Parses" is
// stricter here than [encoding/json]; see sheetSchema.
func NewCharacter(data json.RawMessage) (Character, error) {
	if err := schema.Validate(data); err != nil {
		return Character{}, err
	}

	return Character{
		Aggregate: common.NewAggregate(NewCharacterID()),
		// Cloned for the same reason [Character.Data] clones on the way out:
		// the caller keeps its slice, and must not be able to edit ours.
		data: slices.Clone(data),
	}, nil
}

// ReplaceSheet swaps in a new sheet, running the same rule as [NewCharacter].
//
// It replaces rather than merges: the sheet is one document, and the domain
// has no view inside it to merge along. A caller editing one field decodes,
// edits, and re-encodes on its own side.
//
// Identity, CreatedAt, and Version are untouched — this is a mutation of the
// aggregate the caller loaded, so it rides through a load-modify-save cycle.
func (c *Character) ReplaceSheet(data json.RawMessage) error {
	if err := schema.Validate(data); err != nil {
		return err
	}

	c.data = slices.Clone(data)
	return nil
}

// rehydrateCharacter skips validation. Only repositories should reach it, via
// [CharacterFactory.Rehydrate]. It lives here so it can set unexported fields.
func rehydrateCharacter(id CharacterID, data json.RawMessage, createdAt time.Time, version common.Version) Character {
	return Character{
		Aggregate: common.RehydrateAggregate(id, createdAt, version),
		data:      slices.Clone(data),
	}
}

// Data returns the character sheet as encoded JSON.
//
// The copy is deliberate: [json.RawMessage] is a slice.
func (c Character) Data() json.RawMessage { return slices.Clone(c.data) }
