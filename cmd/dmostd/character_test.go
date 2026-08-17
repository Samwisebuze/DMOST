package main_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/test"
	domain "github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MustSheet returns a schema-valid sheet under the given character
// name, carrying a field the generated v1alpha type has no home for.
//
// Marshalling a map yields bytes that are already compact and key-sorted, so a
// transport that passes them through untouched hands them back byte for byte.
// That is what makes the assertions below equality on []byte rather than
// assert.JSONEq: re-encoding anywhere along the path would still be equivalent
// JSON, and would still have dropped the unknown field.
func MustSheet(t testing.TB, name string) json.RawMessage {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(test.MustCharacterSheetNamed(t, name), &doc))
	doc["house_rule_notes"] = "crits do max damage"
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

func MustCreateCharacter(t testing.TB, cli *http.Client, sheet json.RawMessage) domain.Character {
	t.Helper()
	chr, err := cli.CreateCharacter(context.Background(), v1alpha.CreateCharacterRequest{Sheet: sheet})
	require.NoError(t, err)
	return chr
}

func TestCharactersAPI_CreateThenFetch(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	sheet := MustSheet(t, "Bruenor")

	created := MustCreateCharacter(t, cli, sheet)
	assert.NotEmpty(t, created.ID())
	assert.Equal(t, string(sheet), string(created.Data()))

	got, err := cli.FindCharacter(context.Background(), created.ID())
	require.NoError(t, err)

	assert.Equal(t, string(sheet), string(got.Data()),
		"the client's own bytes must come back, unknown fields and all")
	assert.Equal(t, created.ID(), got.ID())
	assert.Equal(t, created.CreatedAt(), got.CreatedAt())
	assert.Equal(t, created.Version(), got.Version(),
		"a read must not move the revision the client holds")
	assert.False(t, got.Version().IsZero(),
		"a zero version means the response dropped it, leaving conditional updates impossible")
}

func TestCharactersAPI_Update(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	chr := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

	edited := MustSheet(t, "Bruenor Battlehammer")
	got, err := cli.UpdateCharacter(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
		Sheet:   edited,
		Version: ptr(chr.Version().Uint64()),
	})
	require.NoError(t, err)

	assert.Equal(t, string(edited), string(got.Data()))
	assert.Equal(t, chr.ID(), got.ID())
	assert.Equal(t, chr.CreatedAt(), got.CreatedAt())
	assert.Equal(t, chr.Version().Next(), got.Version(), "the client must learn the new revision")
}

func TestCharactersAPI_UpdateWithoutASheetLeavesTheStoredOneAlone(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	sheet := MustSheet(t, "Bruenor")
	chr := MustCreateCharacter(t, cli, sheet)

	// An omitted sheet reaches the server as JSON null, because the field is a
	// json.RawMessage rather than a pointer. It must read as "leave it alone"
	// rather than as a sheet to validate, or this is a 400.
	got, err := cli.UpdateCharacter(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
		Version: ptr(chr.Version().Uint64()),
	})
	require.NoError(t, err)

	assert.Equal(t, string(sheet), string(got.Data()))
	assert.Equal(t, chr.Version().Next(), got.Version())
}

func TestCharactersAPI_UpdateRejectsAStaleVersion(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	chr := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))
	stale := chr.Version().Uint64()

	winner := MustSheet(t, "Bruenor Battlehammer")
	_, err := cli.UpdateCharacter(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
		Sheet:   winner,
		Version: ptr(stale),
	})
	require.NoError(t, err)

	// A second client still holding the pre-update representation.
	loser := MustSheet(t, "Someone Else")
	_, err = cli.UpdateCharacter(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
		Sheet:   loser,
		Version: ptr(stale),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Conflict", "a lost update must surface as 409, not a silent overwrite")

	got, err := cli.FindCharacter(context.Background(), chr.ID())
	require.NoError(t, err)
	assert.Equal(t, string(winner), string(got.Data()), "the losing write must not land")
}

func TestCharactersAPI_CreateRejectsAMalformedSheet(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	for label, sheet := range map[string]string{
		"missing required fields": `{"_id":"abc"}`,
		"not an object":           `["nope"]`,
	} {
		t.Run(label, func(t *testing.T) {
			_, err := cli.CreateCharacter(context.Background(), v1alpha.CreateCharacterRequest{
				Sheet: json.RawMessage(sheet),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Bad Request",
				"a sheet the schema refuses is a malformed body, not a server fault")
		})
	}
}

func TestCharactersAPI_UpdateRejectsAMalformedSheet(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	sheet := MustSheet(t, "Bruenor")
	chr := MustCreateCharacter(t, cli, sheet)

	_, err := cli.UpdateCharacter(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
		Sheet: json.RawMessage(`{"_id":"abc"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad Request")

	got, err := cli.FindCharacter(context.Background(), chr.ID())
	require.NoError(t, err)
	assert.Equal(t, string(sheet), string(got.Data()), "a refused edit must not have landed")
	assert.Equal(t, chr.Version(), got.Version())
}

func TestCharactersAPI_FindUnknownCharacter(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	_, err := cli.FindCharacter(context.Background(), domain.NewCharacterID())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
}

func TestCharactersAPI_FindRejectsAMalformedID(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	_, err := cli.FindCharacter(context.Background(), domain.CharacterID("not-a-uuid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad Request")
}

func TestCharactersAPI_UpdateUnknownCharacter(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	_, err := cli.UpdateCharacter(context.Background(), domain.NewCharacterID(), v1alpha.UpdateCharacterRequest{
		Sheet: MustSheet(t, "Bruenor"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
}

// MustPatchCharacter applies patch and fails the test if the server refuses.
func MustPatchCharacter(t testing.TB, cli *http.Client, id domain.CharacterID, patch string) domain.Character {
	t.Helper()
	chr, err := cli.PatchCharacter(context.Background(), id, v1alpha.PatchCharacterRequest{
		Patch: json.RawMessage(patch),
	})
	require.NoError(t, err)
	return chr
}

// MustSheetDoc decodes a stored sheet so assertions can reach inside it.
//
// Patch responses are compared this way rather than as bytes, unlike every
// other assertion in this file: a merge re-encodes, so the bytes are equivalent
// JSON in a different key order. That is the documented cost of PATCH — see
// v1alpha.PatchCharacterRequest — and asserting on bytes here would be
// asserting on Go's map ordering rather than on the contract.
func MustSheetDoc(t testing.TB, raw json.RawMessage) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc
}

func TestCharactersAPI_Patch(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))
	before := MustSheetDoc(t, created.Data())

	got := MustPatchCharacter(t, cli, created.ID(), `{"identity":{"character_name":"Catti-brie"}}`)

	after := MustSheetDoc(t, got.Data())
	assert.Equal(t, "Catti-brie", after["identity"].(map[string]any)["character_name"])
	assert.Equal(t, created.ID(), got.ID())
	assert.Equal(t, created.Version().Next(), got.Version(),
		"a patch is a write, so the revision must move")

	for key, want := range before {
		if key == "identity" {
			continue
		}
		assert.Equal(t, want, after[key], "patching identity must not disturb %q", key)
	}

	// And it must be what a later reader sees, not just what the write echoed.
	found, err := cli.FindCharacter(context.Background(), created.ID())
	require.NoError(t, err)
	assert.Equal(t, after, MustSheetDoc(t, found.Data()))
}

// The reason PATCH exists: two clients editing unrelated sections, neither
// having to send — or even hold — the rest of the document.
func TestCharactersAPI_PatchDoesNotClobberAnUnrelatedSection(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

	MustPatchCharacter(t, cli, created.ID(), `{"campaign_id":"night-below"}`)
	got := MustPatchCharacter(t, cli, created.ID(), `{"identity":{"character_name":"Regis"}}`)

	doc := MustSheetDoc(t, got.Data())
	assert.Equal(t, "night-below", doc["campaign_id"], "the first patch must survive the second")
	assert.Equal(t, "Regis", doc["identity"].(map[string]any)["character_name"])
}

// Key order does not survive a merge; unknown fields do. That distinction is
// the whole trade-off PATCH makes, so it is asserted rather than assumed.
func TestCharactersAPI_PatchKeepsFieldsTheSchemaDoesNotKnow(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

	got := MustPatchCharacter(t, cli, created.ID(), `{"identity":{"character_name":"Regis"}}`)

	assert.Equal(t, "crits do max damage", MustSheetDoc(t, got.Data())["house_rule_notes"])
}

func TestCharactersAPI_PatchNullDeletesAKey(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

	set := MustPatchCharacter(t, cli, created.ID(), `{"campaign_id":"night-below"}`)
	require.Contains(t, MustSheetDoc(t, set.Data()), "campaign_id")

	cleared := MustPatchCharacter(t, cli, created.ID(), `{"campaign_id":null}`)
	assert.NotContains(t, MustSheetDoc(t, cleared.Data()), "campaign_id")
}

func TestCharactersAPI_PatchWithoutAPatchLeavesTheStoredSheetAlone(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

	got, err := cli.PatchCharacter(context.Background(), created.ID(), v1alpha.PatchCharacterRequest{
		Version: ptr(created.Version().Uint64()),
	})
	require.NoError(t, err)

	// Nothing was merged, so this one keeps the client's bytes.
	assert.Equal(t, string(created.Data()), string(got.Data()))
}

func TestCharactersAPI_PatchRejectsAStaleVersion(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))
	stale := created.Version().Uint64()

	MustPatchCharacter(t, cli, created.ID(), `{"identity":{"character_name":"Regis"}}`)

	_, err := cli.PatchCharacter(context.Background(), created.ID(), v1alpha.PatchCharacterRequest{
		Patch:   json.RawMessage(`{"identity":{"character_name":"Wulfgar"}}`),
		Version: ptr(stale),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Conflict")
}

func TestCharactersAPI_PatchRejectsAServerOwnedKey(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())
	created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

	_, err := cli.PatchCharacter(context.Background(), created.ID(), v1alpha.PatchCharacterRequest{
		Patch: json.RawMessage(`{"_id":"hijacked"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad Request")

	found, err := cli.FindCharacter(context.Background(), created.ID())
	require.NoError(t, err)
	assert.Equal(t, string(created.Data()), string(found.Data()), "a refused patch must change nothing")
	assert.Equal(t, created.Version(), found.Version())
}

func TestCharactersAPI_PatchRejectsAMergeTheSchemaRefuses(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"deletes a required section": `{"vitals":null}`,
		"breaks a required pattern":  `{"schema_version":"nope"}`,
		"is not an object":           `5`,
	}

	for name, patch := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := MustRunMain(t)
			defer MustCloseMain(t, m)

			cli := MustClient(t, m.HTTPServer.URL())
			created := MustCreateCharacter(t, cli, MustSheet(t, "Bruenor"))

			_, err := cli.PatchCharacter(context.Background(), created.ID(), v1alpha.PatchCharacterRequest{
				Patch: json.RawMessage(patch),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Bad Request")

			found, err := cli.FindCharacter(context.Background(), created.ID())
			require.NoError(t, err)
			assert.Equal(t, string(created.Data()), string(found.Data()))
		})
	}
}

func TestCharactersAPI_PatchUnknownCharacter(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	_, err := cli.PatchCharacter(context.Background(), domain.NewCharacterID(), v1alpha.PatchCharacterRequest{
		Patch: json.RawMessage(`{"campaign_id":"night-below"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
}
