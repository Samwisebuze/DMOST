package inmem

import (
	"context"
	"slices"
	"sync"

	domain "github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

type CharacterRepository struct {
	mu   sync.RWMutex
	data map[domain.CharacterID]*domain.Character
}

func NewCharacterRepository() *CharacterRepository {
	return &CharacterRepository{
		data: map[domain.CharacterID]*domain.Character{},
	}
}

var _ domain.CharacterRepository = (*CharacterRepository)(nil)

// characterFactory is the domain's door for adapters: Save reaches through it
// to advance an aggregate's version, which is not something callers may do,
// and to rebuild a Character that shares no state with the one handed in.
var characterFactory domain.CharacterFactory

// Save implements [domain.CharacterRepository].
func (r *CharacterRepository) Save(_ context.Context, entity *domain.Character) error {
	// The lock spans the version check and the insert: splitting them would let
	// two concurrent Saves both pass the compare-and-set.
	r.mu.Lock()
	defer r.mu.Unlock()

	// An existing ID is the update path, not a collision — but only if the
	// caller loaded the revision still on record. Two callers that both read
	// version N and write back would otherwise silently lose one edit; the
	// second one's compare-and-set fails here instead.
	//
	// A Character has no field that is unique collection-wide, so unlike
	// [UserRepository.Save] there is nothing to scan the other records for.
	cur, update := r.data[entity.ID()]
	if update && !cur.Version().Equal(entity.Version()) {
		return common.ErrConflict
	}

	// An insert stores the version the aggregate was constructed with; only a
	// replacement advances it. Advancing c itself, not just the stored copy,
	// lets the caller edit and save again without reloading.
	if update {
		characterFactory.NextVersion(entity)
	}

	// Rehydrate rather than *c: the sheet is a [json.RawMessage], so a struct
	// copy would leave the caller holding the same backing array as the stored
	// record and able to edit it behind the repository's back.
	cpy := characterFactory.Rehydrate(entity.ID(), entity.Data(), entity.CreatedAt(), entity.Version())
	r.data[entity.ID()] = &cpy
	return nil
}

// Find implements [domain.CharacterRepository].
func (r *CharacterRepository) Find(_ context.Context, id domain.CharacterID) (domain.Character, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, found := r.data[id]
	if !found {
		return domain.Character{}, common.ErrNotFound
	}
	// Copied on the way out for the same reason it is copied on the way in.
	return characterFactory.Rehydrate(entity.ID(), entity.Data(), entity.CreatedAt(), entity.Version()), nil
}

// List implements [character.CharacterRepository].
func (r *CharacterRepository) List(context.Context) ([]domain.Character, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var entities []domain.Character
	for _, e := range r.data {
		entities = append(entities, *e)
	}

	slices.SortStableFunc(entities, func(a, b domain.Character) int { return a.ID().Compare(b.ID()) })
	return entities, nil

}
