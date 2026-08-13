package character

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

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
// The sheet stays opaque here on purpose. What counts as a *well-formed* sheet
// is a wire-contract rule — the shape lives in docs/jsonschema and is enforced
// against the generated v1alpha types in
// [github.com/samwisebuze/dmost/internal/dto/v1alpha/mapper], which is where
// this constructor is called from. The domain's own rule is the narrower one
// that survives a change of schema version: a Character has a sheet, and that
// sheet is a JSON object.
//
// Returns an error wrapping [common.ErrInvalid] if data is empty, is not valid
// JSON, or is valid JSON that is not an object.
func NewCharacter(data json.RawMessage) (Character, error) {
	if err := validateSheet(data); err != nil {
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
	if err := validateSheet(data); err != nil {
		return err
	}

	c.data = slices.Clone(data)
	return nil
}

func validateSheet(data json.RawMessage) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: character sheet required", common.ErrInvalid)
	}
	if !json.Valid(trimmed) {
		return fmt.Errorf("%w: character sheet must be valid JSON", common.ErrInvalid)
	}
	// A bare `4` or `"x"` is valid JSON and not a character sheet. Checking the
	// first byte is enough once json.Valid has passed.
	if trimmed[0] != '{' {
		return fmt.Errorf("%w: character sheet must be a JSON object", common.ErrInvalid)
	}
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
// The copy is deliberate: [json.RawMessage] is a slice, so handing back the
// field itself would let a caller edit a Character in place and sidestep the
// read-only rule the other getters keep. It is also what makes a repository's
// stored copy safe — see [github.com/samwisebuze/dmost/internal/infra/inmem].
func (c Character) Data() json.RawMessage { return slices.Clone(c.data) }
