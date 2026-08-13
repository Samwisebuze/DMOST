package character

import (
	"encoding/json"
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/domain/internal/lock"
)

// CharacterFactory allows infrastructure to reconstruct characters without
// accessing unexported fields directly.
type CharacterFactory struct{}

func (CharacterFactory) Rehydrate(id CharacterID, data json.RawMessage, createdAt time.Time, version common.Version) Character {
	return rehydrateCharacter(id, data, createdAt, version)
}

// NextVersion advances c to the version it holds once persisted. Only a
// repository should call it, and only after its compare-and-set on Version has
// passed, so the aggregate the caller holds keeps matching the stored revision
// and can be edited again without a reload.
//
// This is the only door to [common.Aggregate.NextVersion] from outside the
// domain: that method wants a [lock.Key], and an adapter cannot import the
// package that mints one.
func (CharacterFactory) NextVersion(c *Character) { c.Aggregate.NextVersion(lock.New()) }
