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

	// The repository's compare-and-set only sees the window between the Find
	// above and the Save below. Checking the client's expected version here is
	// what catches the slower race: two clients that both read version N,
	// think, and write back.
	if req.Version != nil {
		expected, err := common.ParseVersion(*req.Version)
		if err != nil {
			return domain.Character{}, err
		}
		if !expected.Equal(chr.Version()) {
			return domain.Character{}, fmt.Errorf("%w: character was modified since version %s", common.ErrConflict, expected)
		}
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
