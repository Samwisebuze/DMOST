package services

import (
	"context"
	"fmt"

	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha/mapper"
	domain "github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

type CharacterService struct {
	Characters domain.CharacterRepository
}

func NewCharacterService(characters domain.CharacterRepository) *CharacterService {
	if characters == nil {
		panic("character repo must be set")
	}

	return &CharacterService{
		Characters: characters,
	}
}

// Find implements [app.CharacterService].
func (c *CharacterService) Find(ctx context.Context, raw string) (domain.Character, error) {
	id, err := domain.ParseCharacterID(raw)
	if err != nil {
		return domain.Character{}, err
	}

	chr, err := c.Characters.Find(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}

	return chr, nil
}

// Create implements [app.CharacterService].
func (c *CharacterService) Create(ctx context.Context, req v1alpha.CreateCharacterRequest) (domain.Character, error) {
	chr, err := mapper.CharacterFromCreateRequest(req)
	if err != nil {
		return domain.Character{}, err
	}

	if err := c.Characters.Save(ctx, &chr); err != nil {
		return domain.Character{}, err
	}

	return chr, nil
}

// Update implements [app.CharacterService].
func (c *CharacterService) Update(ctx context.Context, id domain.CharacterID, req v1alpha.UpdateCharacterRequest) (domain.Character, error) {
	chr, err := c.Characters.Find(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}

	if err := checkVersion(req.Version, chr.Version()); err != nil {
		return domain.Character{}, err
	}

	if err := mapper.ApplyCharacterUpdateRequest(&chr, req); err != nil {
		return domain.Character{}, err
	}

	// Save is an upsert keyed by CharacterID, so this replaces the record
	// loaded above rather than inserting a second one.
	if err := c.Characters.Save(ctx, &chr); err != nil {
		return domain.Character{}, err
	}

	return chr, nil
}

// Patch implements [app.CharacterService].
//
// Update's twin, differing only in what the mapper does to the sheet: this one
// merges rather than replaces. Everything around it — the load, the version
// guard, the upsert — is the same, because a partial write is still one
// load-modify-save cycle against one aggregate.
func (c *CharacterService) Patch(ctx context.Context, id domain.CharacterID, req v1alpha.PatchCharacterRequest) (domain.Character, error) {
	chr, err := c.Characters.Find(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}

	if err := checkVersion(req.Version, chr.Version()); err != nil {
		return domain.Character{}, err
	}

	if err := mapper.ApplyCharacterPatchRequest(&chr, req); err != nil {
		return domain.Character{}, err
	}

	if err := c.Characters.Save(ctx, &chr); err != nil {
		return domain.Character{}, err
	}

	return chr, nil
}

// checkVersion refuses a write whose client read a revision the stored
// Character has moved past. A nil expected means the client did not say, which
// v1alpha reads as last-writer-wins.
//
// The repository's compare-and-set only sees the window between a Find and the
// Save that follows it. Checking the client's expected version here is what
// catches the slower race: two clients that both read version N, think, and
// write back.
//
// Returns [common.ErrInvalid] for a version no client could have read (zero),
// and [common.ErrConflict] for one that has been overtaken.
func checkVersion(expected *uint64, actual common.Version) error {
	if expected == nil {
		return nil
	}

	want, err := common.ParseVersion(*expected)
	if err != nil {
		return err
	}
	if !want.Equal(actual) {
		return fmt.Errorf("%w: character was modified since version %s", common.ErrConflict, want)
	}

	return nil
}
