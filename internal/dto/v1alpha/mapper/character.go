package mapper

import (
	"encoding/json"
	"fmt"

	dto "github.com/samwisebuze/dmost/internal/dto/v1alpha"
	schema "github.com/samwisebuze/dmost/internal/dto/v1alpha/character"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

// Inbound: JSON → Domain
func CharacterFromCreateRequest(req dto.CreateCharacterRequest) (character.Character, error) {
	if err := validateSheet(req.Sheet); err != nil {
		return character.Character{}, err
	}

	return character.NewCharacter(req.Sheet)
}

// Inbound: JSON → Domain
//
// ApplyCharacterUpdateRequest applies req's sheet to c through the domain's
// mutator, leaving it untouched if the request omits one. It takes an existing
// Character rather than building one so identity and CreatedAt ride along on
// the aggregate the caller loaded, instead of coming from the request.
func ApplyCharacterUpdateRequest(c *character.Character, req dto.UpdateCharacterRequest) error {
	if req.Sheet == nil {
		return nil
	}
	if err := validateSheet(req.Sheet); err != nil {
		return err
	}

	return c.ReplaceSheet(req.Sheet)
}

// validateSheet decodes raw against the generated v1alpha schema and discards
// the result, which is the whole point: the decode is what enforces the
// schema's required fields, enums, and patterns, and the *bytes* are what the
// domain goes on to store. Re-encoding the decoded struct instead would drop
// anything the generated type has no field for.
//
// Returns an error wrapping [common.ErrInvalid], so a malformed sheet reaches
// the transport as a 400 rather than a 500.
func validateSheet(raw json.RawMessage) error {
	var sheet schema.CharacterSchema
	if err := json.Unmarshal(raw, &sheet); err != nil {
		return fmt.Errorf("%w: %w", common.ErrInvalid, err)
	}
	return nil
}
