package character_test

import (
	"encoding/json"
	"testing"

	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// editSheet returns the fixture with one edit applied, so a case can say what
// makes a sheet wrong instead of carrying a second copy of a large document.
func editSheet(t *testing.T, edit func(map[string]any)) json.RawMessage {
	t.Helper()

	var doc map[string]any
	require.NoError(t, json.Unmarshal(test.MustCharacterSheet(t), &doc))
	edit(doc)

	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

type sheetCase struct {
	sheet   func(*testing.T) json.RawMessage
	wantErr string   // empty means the sheet is accepted
	wantsay []string // fragments the message must name, for schema failures
	lax     bool     // encoding/json accepts it; only the stricter parse rejects
}

func literal(sheet string) func(*testing.T) json.RawMessage {
	return func(*testing.T) json.RawMessage { return json.RawMessage(sheet) }
}

// sheetCases is the rule validateSheet enforces, run against both doors into
// it. The `lax` group is the part encoding/json would have let through:
// json.Valid says yes to every one of them and the compiled schema says no.
func sheetCases() map[string]sheetCase {
	return map[string]sheetCase{
		"the full fixture": {
			sheet: func(t *testing.T) json.RawMessage { return test.MustCharacterSheet(t) },
		},
		"a field the schema does not know": {
			// The root schema is open (additionalProperties: true), which is
			// what lets a client round-trip data this version has no home for.
			sheet: func(t *testing.T) json.RawMessage {
				return editSheet(t, func(doc map[string]any) {
					doc["house_rule_notes"] = map[string]any{"crit": "max damage"}
				})
			},
		},
		"a required section missing": {
			sheet: func(t *testing.T) json.RawMessage {
				return editSheet(t, func(doc map[string]any) { delete(doc, "abilities") })
			},
			wantErr: "character sheet does not match the v1alpha schema",
			wantsay: []string{"abilities"},
		},
		"a required field missing from a nested section": {
			sheet: func(t *testing.T) json.RawMessage {
				return editSheet(t, func(doc map[string]any) {
					delete(doc["abilities"].(map[string]any), "strength")
				})
			},
			wantErr: "character sheet does not match the v1alpha schema",
			wantsay: []string{"/abilities", "strength"},
		},
		"a nested field of the wrong type": {
			sheet: func(t *testing.T) json.RawMessage {
				return editSheet(t, func(doc map[string]any) {
					doc["abilities"].(map[string]any)["strength"] = "very"
				})
			},
			wantErr: "character sheet does not match the v1alpha schema",
			// The failure is three files deep, past two $refs. The message has
			// to name the field, not the section it sits in.
			wantsay: []string{"/abilities/strength", "should be object"},
		},
		"a const the schema pins": {
			sheet: func(t *testing.T) json.RawMessage {
				return editSheet(t, func(doc map[string]any) { doc["doc_type"] = "monster" })
			},
			wantErr: "character sheet does not match the v1alpha schema",
			wantsay: []string{"/doc_type"},
		},
		"empty object":          {sheet: literal(`{}`), wantErr: "character sheet does not match the v1alpha schema", wantsay: []string{"missing"}},
		"empty":                 {sheet: literal(``), wantErr: "character sheet required"},
		"whitespace only":       {sheet: literal("  \n "), wantErr: "character sheet required"},
		"truncated":             {sheet: literal(`{"a":`), wantErr: "character sheet must be valid JSON"},
		"trailing comma":        {sheet: literal(`{"a":1,}`), wantErr: "character sheet must be valid JSON"},
		"trailing bytes":        {sheet: literal(`{"a":1} {"b":2}`), wantErr: "character sheet must be valid JSON"},
		"duplicate keys":        {sheet: literal(`{"a":1,"a":2}`), wantErr: "character sheet must be valid JSON", lax: true},
		"invalid utf-8":         {sheet: literal("{\"a\":\"\xff\"}"), wantErr: "character sheet must be valid JSON", lax: true},
		"number":                {sheet: literal(`4`), wantErr: "character sheet must be a JSON object"},
		"string":                {sheet: literal(`"x"`), wantErr: "character sheet must be a JSON object"},
		"array":                 {sheet: literal(`[{"a":1}]`), wantErr: "character sheet must be a JSON object"},
		"null":                  {sheet: literal(`null`), wantErr: "character sheet must be a JSON object"},
		"boolean":               {sheet: literal(`true`), wantErr: "character sheet must be a JSON object"},
		"an object in a string": {sheet: literal(`"{\"a\":1}"`), wantErr: "character sheet must be a JSON object"},
	}
}

func TestNewCharacter(t *testing.T) {
	t.Parallel()

	for name, tc := range sheetCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sheet := tc.sheet(t)
			c, err := character.NewCharacter(sheet)
			if tc.wantErr != "" {
				require.ErrorIs(t, err, common.ErrInvalid)
				assert.ErrorContains(t, err, tc.wantErr)
				for _, fragment := range tc.wantsay {
					assert.ErrorContains(t, err, fragment, "the message must locate the failure")
				}
				assert.Empty(t, c.ID(), "a rejected sheet must not yield an identified Character")
				return
			}

			require.NoError(t, err)
			assert.JSONEq(t, string(sheet), string(c.Data()))
		})
	}
}

func TestCharacterReplaceSheet(t *testing.T) {
	t.Parallel()

	for name, tc := range sheetCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			original := test.MustCharacterSheetNamed(t, "Bruenor")
			c := test.MustCharacter(t, original)

			err := c.ReplaceSheet(tc.sheet(t))
			if tc.wantErr != "" {
				require.ErrorIs(t, err, common.ErrInvalid)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.JSONEq(t, string(original), string(c.Data()), "a rejected sheet must leave the old one in place")
				return
			}

			require.NoError(t, err)
			assert.JSONEq(t, string(tc.sheet(t)), string(c.Data()))
		})
	}
}

// TestValidateSheetIsStricterThanEncodingJSON pins the parser half of what the
// compiled schema buys: these sheets satisfy [json.Valid], so a hand-rolled
// first-byte check accepted them, and their meaning depends on which decoder
// reads them back.
func TestValidateSheetIsStricterThanEncodingJSON(t *testing.T) {
	t.Parallel()

	var strict int
	for name, tc := range sheetCases() {
		if !tc.lax {
			continue
		}
		strict++
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sheet := tc.sheet(t)
			require.True(t, json.Valid(sheet), "fixture is only interesting if encoding/json accepts it")
			_, err := character.NewCharacter(sheet)
			require.ErrorIs(t, err, common.ErrInvalid)
		})
	}
	require.NotZero(t, strict, "the strict cases were removed from sheetCases")
}

// TestValidateSheetBoundsItsComplaints keeps a rejection readable: a sheet that
// is wrong everywhere must not answer with everything that is wrong with it.
func TestValidateSheetBoundsItsComplaints(t *testing.T) {
	t.Parallel()

	_, err := character.NewCharacter(json.RawMessage(`{"doc_type":"monster"}`))
	require.ErrorIs(t, err, common.ErrInvalid)
	assert.ErrorContains(t, err, "and ", "the surplus must be counted, not listed")
	assert.LessOrEqual(t, len(err.Error()), 600, "a rejection has to stay readable")
}
