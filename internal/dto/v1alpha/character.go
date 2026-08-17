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
// leaves the stored one unchanged; a present one replaces it whole. Editing
// part of a sheet is [PatchCharacterRequest]'s job.
type UpdateCharacterRequest struct {
	Sheet json.RawMessage `json:"sheet"`

	// Version is the revision the client last read. Supplying it makes the
	// update conditional: it is refused if someone else has written since.
	// Omitting it means last writer wins. See [UpdateUserRequest.Version].
	Version *uint64 `json:"version"`
}

// PatchCharacterRequest carries a JSON Merge Patch (RFC 7396) to apply to the
// stored sheet: objects merge recursively, a null deletes the key it names,
// and an array replaces the array it lands on rather than merging into it. An
// omitted patch leaves the sheet alone, the same way an omitted Sheet does on
// [UpdateCharacterRequest].
//
// It carries the one property this resource otherwise keeps. A create or a
// replace stores the client's own bytes, so a field the generated v1alpha type
// knows nothing about survives verbatim, in place. Merging cannot: it decodes
// both documents, merges, and re-encodes, and Go sorts object keys on the way
// out. **Unknown fields still survive** — they are carried through the decoded
// value like any other — but key order and whitespace do not. A client that
// needs its exact bytes back should PUT.
//
// The body is this envelope rather than a bare merge patch document, because
// Version has to ride along beside it; that is also why the request media type
// stays [ContentTypeJSON] and is not application/merge-patch+json, which would
// promise a body shape this is not.
type PatchCharacterRequest struct {
	Patch json.RawMessage `json:"patch"`

	// Version is the revision the client last read, with the same meaning as
	// [UpdateCharacterRequest.Version]: supplied makes the patch conditional,
	// omitted means last writer wins.
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
