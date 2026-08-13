package v1alpha

import "encoding/json"

const ContentTypeCharacterJSON string = "application/vnd.dmost.character.v1alpha+json"

// The character sheet itself is not declared here: it is generated from
// docs/jsonschema/character/v1alpha into the character subpackage, and these
// requests carry it as raw bytes rather than as that generated struct.
//
// Two reasons. The domain stores the sheet as encoded JSON, so decoding it
// into a struct and re-encoding it would hand the repository bytes that are
// merely equivalent to what the client sent — reordered, and stripped of any
// field the generated type does not know. And the generated type earns its
// keep as a *validator* (its UnmarshalJSON enforces the schema's required
// fields, enums, and patterns), which the mapper still runs; it just throws
// the decoded value away afterwards and keeps the client's bytes.

// CreateCharacterRequest carries a whole character sheet as encoded JSON. It
// is validated against the v1alpha character schema in the mapper.
type CreateCharacterRequest struct {
	Sheet json.RawMessage `json:"sheet"`
}

// UpdateCharacterRequest mirrors [CreateCharacterRequest]. An omitted Sheet
// leaves the stored one unchanged; a present one replaces it whole, because a
// sheet is one document and v1alpha has no patch shape for it.
type UpdateCharacterRequest struct {
	Sheet json.RawMessage `json:"sheet"`

	// Version is the revision the client last read. Supplying it makes the
	// update conditional: it is refused if someone else has written since.
	// Omitting it means last writer wins. See [UpdateUserRequest.Version].
	Version *uint64 `json:"version"`
}

// CharacterResponse is a stored sheet with the aggregate's identity and
// revision alongside it.
//
// There is no list counterpart to this type — [character.CharacterRepository]
// has only Save and Find, so no endpoint can return a collection yet.
type CharacterResponse struct {
	ID string `json:"id"`

	// Sheet is the client's own bytes, carried out the same way they came in.
	// Anything that decoded them into the generated schema type and re-encoded
	// would silently drop every field that type has no home for, which is the
	// one property this resource exists to keep.
	Sheet json.RawMessage `json:"sheet"`

	CreatedAt string `json:"created_at"`

	// Version is the revision this representation was read at. Echo it in an
	// [UpdateCharacterRequest] to make the write conditional. Unsigned so a
	// negative number in a payload fails to decode rather than wrapping.
	Version uint64 `json:"version"`
}
