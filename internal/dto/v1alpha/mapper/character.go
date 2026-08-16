package mapper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	dto "github.com/samwisebuze/dmost/internal/dto/v1alpha"
	schema "github.com/samwisebuze/dmost/internal/dto/v1alpha/character"
	"github.com/samwisebuze/dmost/internal/jsonmerge"
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
	if rawOmitted(req.Sheet) {
		return nil
	}
	if err := validateSheet(req.Sheet); err != nil {
		return err
	}

	return c.ReplaceSheet(req.Sheet)
}

// Inbound: JSON → Domain
//
// ApplyCharacterPatchRequest merges req's JSON Merge Patch (RFC 7396) into c's
// stored sheet, leaving c untouched if the request omits one. Like
// [ApplyCharacterUpdateRequest] it takes a loaded Character so identity,
// CreatedAt, and Version ride along on the aggregate rather than coming from
// the request.
//
// The merged sheet goes through the same schema gate a replacement does, which
// is what catches a patch that deletes a required section or writes a value
// the schema refuses. Note that the sheet stored afterwards is *re-encoded*,
// not the client's bytes with an edit spliced in — see
// [dto.PatchCharacterRequest] for what that costs and what it does not.
//
// Errors wrap [common.ErrInvalid] throughout: everything that can fail here is
// something about the request.
func ApplyCharacterPatchRequest(c *character.Character, req dto.PatchCharacterRequest) error {
	if rawOmitted(req.Patch) {
		return nil
	}

	// RFC 7396 says a non-object patch replaces the whole document, so without
	// this a patch of `5` would leave the character holding a sheet that is
	// the number 5. A sheet is an object; a patch against one has to be too.
	if !isJSONObject(req.Patch) {
		return fmt.Errorf("%w: character patch must be a JSON object", common.ErrInvalid)
	}

	if err := rejectServerOwnedKeys(req.Patch); err != nil {
		return err
	}

	merged, err := jsonmerge.Apply(c.Data(), req.Patch)
	if err != nil {
		// c.Data() came out of the aggregate and decodes by construction, so
		// a failure here is the client's patch.
		return fmt.Errorf("%w: %w", common.ErrInvalid, err)
	}

	// The gate runs on the *result*, not on the patch, and it has to: a patch
	// is legal JSON in isolation while the sheet it produces is not. Without
	// this, `{"vitals":null}` stores a document the schema refuses, which GET
	// then hands back and PUT then rejects — the store would be able to hold
	// sheets its own API will not accept.
	if err := validateSheet(merged); err != nil {
		return err
	}

	return c.ReplaceSheet(merged)
}

// serverOwnedSheetKeys are the top-level sheet fields a client may not write.
//
// They are the document's own bookkeeping — who it is, which contract it
// speaks, and when it changed — and the aggregate is authoritative over the
// ones that overlap it (see the "_id" description in
// docs/jsonschema/character/v1alpha/character.schema.json). Letting a patch
// reach them would let a client rewrite its own history in place.
//
// "campaign_id" is deliberately absent: it is the client's to set, and to
// clear by patching it to null. "owner_user_id" is present only because there
// is no auth in the system yet, so a sheet patch should not be the first way
// ownership changes hands; revisit it when there is something to authorize
// against.
var serverOwnedSheetKeys = []string{
	"_id",
	"doc_type",
	"schema_version",
	"created_at",
	"updated_at",
	"doc_revision",
	"owner_user_id",
}

// rejectServerOwnedKeys refuses a patch that names one of
// [serverOwnedSheetKeys] at the top level.
//
// Only the top level, because that is where those fields live. A nested
// "created_at" belongs to whatever section holds it — an inventory item's, say
// — and is the client's like the rest of that section.
//
// Refusing beats stripping: a client that thinks it is setting created_at has
// a bug, and silently dropping the field would let it keep believing the write
// landed.
func rejectServerOwnedKeys(patch json.RawMessage) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(patch, &top); err != nil {
		return fmt.Errorf("%w: %w", common.ErrInvalid, err)
	}

	// Ranged over the slice rather than the map so the message a client gets
	// for a patch naming several is the same one every time.
	for _, key := range serverOwnedSheetKeys {
		if _, ok := top[key]; ok {
			return fmt.Errorf("%w: %q is server-owned and cannot be patched", common.ErrInvalid, key)
		}
	}

	return nil
}

// isJSONObject reports whether raw is a JSON object. It assumes raw is valid
// JSON, which the decode in [rejectServerOwnedKeys] and [jsonmerge.Apply] both
// go on to confirm.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
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

// rawOmitted reports whether a request left one of its raw JSON documents
// alone — the sheet on [dto.UpdateCharacterRequest], the patch on
// [dto.PatchCharacterRequest].
//
// A JSON null counts as omitted, and has to: both fields are a
// [json.RawMessage] rather than a pointer, and a nil one *encodes* as null, so
// a client sending nothing but a version puts "sheet":null (or "patch":null)
// on the wire — which decodes back as the four literal bytes, not as nil.
// Reading those as content would fail validation and turn a legal
// version-only request into a 400. The pointer fields on
// [dto.UpdateUserRequest] round-trip nil as nil and need none of this.
//
// Null is the only reading available anyway: a Character must have a sheet, so
// there is no "clear it" for null to mean.
func rawOmitted(raw json.RawMessage) bool {
	return raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
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
