package test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/stretchr/testify/require"
)

// characterSheet is the smallest document that satisfies every required field
// of docs/jsonschema/character/v1alpha — the schema has a dozen of them nested
// several levels deep, so building one inline in each test is not worth it.
//
// Embedded rather than read at runtime: go:embed resolves relative to this
// file, while os.ReadFile would resolve relative to whichever package's test
// binary is running.
//
//go:embed testdata/character.v1alpha.json
var characterSheet []byte

// MustCharacterSheet returns a sheet that passes v1alpha validation. Callers
// that need to tell two characters apart should edit the returned bytes rather
// than assume anything about the contents.
func MustCharacterSheet(t testing.TB) json.RawMessage {
	t.Helper()
	require.True(t, json.Valid(characterSheet), "the embedded fixture must be valid JSON")
	return json.RawMessage(characterSheet)
}

// MustCharacterSheetNamed returns the fixture with a caller-chosen character
// name, so assertions can tell one stored sheet from another.
func MustCharacterSheetNamed(t testing.TB, name string) json.RawMessage {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(MustCharacterSheet(t), &doc))

	identity, ok := doc["identity"].(map[string]any)
	require.True(t, ok, "the fixture must carry an identity object")
	identity["character_name"] = name

	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

// MustCharacter builds a valid Character through the domain's constructor.
func MustCharacter(t testing.TB, sheet json.RawMessage) *character.Character {
	t.Helper()
	c, err := character.NewCharacter(sheet)
	require.NoError(t, err)
	return &c
}
