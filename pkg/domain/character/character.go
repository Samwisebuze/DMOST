package character

import (
	"context"
	"encoding/json"
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
