package mapper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	dto "github.com/samwisebuze/dmost/internal/dto/v1alpha"
	schema "github.com/samwisebuze/dmost/internal/dto/v1alpha/character"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

var characterFactory character.CharacterFactory

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
	if sheetOmitted(req.Sheet) {
		return nil
	}
	if err := validateSheet(req.Sheet); err != nil {
		return err
	}

	return c.ReplaceSheet(req.Sheet)
}

// Outbound: Domain → JSON
func CharacterToResponse(c character.Character) dto.CharacterResponse {
	return dto.CharacterResponse{
		ID: c.ID().String(),
		// Passed through as bytes, deliberately. This is the outbound half of
		// the rule validateSheet keeps on the way in: the schema types are a
		// gate, never something the sheet is round-tripped through, so a field
		// the generated type does not know about still reaches the client.
		// [character.Character.Data] already clones, so the response owns its
		// copy and cannot reach back into the aggregate.
		Sheet:     c.Data(),
		CreatedAt: c.CreatedAt().Format(time.RFC3339),
		// Without this the client reads version 0 and can never make a
		// conditional update stick.
		Version: c.Version().Uint64(),
	}
}

// Inbound: JSON → Domain
//
// CharacterResponseToCharacter rebuilds the aggregate a response describes. It
// is the client's half of the contract — the server never decodes its own
// output — so it rehydrates rather than validating: every field here was
// issued by the server, including the sheet, which the schema already passed
// before it was stored.
func CharacterResponseToCharacter(res dto.CharacterResponse) character.Character {
	createdAt, _ := time.Parse(time.RFC3339, res.CreatedAt)
	// Rehydrating, not parsing: this is a revision the server issued, so it is
	// taken as given like every other field here.
	version := common.RehydrateVersion(res.Version)
	return characterFactory.Rehydrate(character.CharacterID(res.ID), res.Sheet, createdAt, version)
}

// sheetOmitted reports whether an update request left the sheet alone.
//
// A JSON null counts as omitted, and has to: [dto.UpdateCharacterRequest.Sheet]
// is a [json.RawMessage] rather than a pointer, and a nil one *encodes* as
// null, so a client sending nothing but a version puts "sheet":null on the
// wire — which decodes back as the four literal bytes, not as nil. Reading
// those as a sheet would fail validation and turn a legal version-only update
// into a 400. The pointer fields on [dto.UpdateUserRequest] round-trip nil as
// nil and need none of this.
//
// Null is the only reading available anyway: a Character must have a sheet, so
// there is no "clear it" for null to mean.
func sheetOmitted(sheet json.RawMessage) bool {
	return sheet == nil || bytes.Equal(bytes.TrimSpace(sheet), []byte("null"))
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
