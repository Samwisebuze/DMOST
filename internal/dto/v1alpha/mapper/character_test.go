package mapper_test

import (
	"encoding/json"
	"testing"

	dto "github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha/mapper"
	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rules below live nowhere else: the service and the handler both delegate
// to ApplyCharacterPatchRequest and only pass its error along.

func TestApplyCharacterPatchRequest_MergesWithoutDisturbingSiblings(t *testing.T) {
	t.Parallel()

	chr := test.MustCharacter(t, test.MustCharacterSheetNamed(t, "Bramble"))
	before := decodeSheet(t, chr.Data())

	err := mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
		Patch: json.RawMessage(`{"identity":{"character_name":"Vex"}}`),
	})
	require.NoError(t, err)

	after := decodeSheet(t, chr.Data())
	assert.Equal(t, "Vex", after["identity"].(map[string]any)["character_name"])

	// Every other section, and every other key of the one that was patched,
	// must be exactly what it was.
	for key, want := range before {
		if key == "identity" {
			continue
		}
		assert.Equal(t, want, after[key], "patching identity must not disturb %q", key)
	}
	for key, want := range before["identity"].(map[string]any) {
		if key == "character_name" {
			continue
		}
		assert.Equal(t, want, after["identity"].(map[string]any)[key], "identity.%s must survive", key)
	}
}

// The property the whole resource exists to keep: a field the generated
// v1alpha type has no home for is still there after a patch that does not
// name it. Merging reorders keys; it must not lose them.
func TestApplyCharacterPatchRequest_KeepsFieldsTheSchemaDoesNotKnow(t *testing.T) {
	t.Parallel()

	sheet := decodeSheet(t, test.MustCharacterSheet(t))
	sheet["house_rule_notes"] = map[string]any{"crit": "max damage"}
	raw, err := json.Marshal(sheet)
	require.NoError(t, err)

	chr := test.MustCharacter(t, raw)

	require.NoError(t, mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
		Patch: json.RawMessage(`{"identity":{"character_name":"Vex"}}`),
	}))

	after := decodeSheet(t, chr.Data())
	assert.Equal(t, map[string]any{"crit": "max damage"}, after["house_rule_notes"])
}

func TestApplyCharacterPatchRequest_RejectsServerOwnedKeys(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"_id", "doc_type", "schema_version", "created_at", "updated_at", "doc_revision", "owner_user_id"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			chr := test.MustCharacter(t, test.MustCharacterSheet(t))
			before := string(chr.Data())

			err := mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
				Patch: json.RawMessage(`{"` + key + `":"hijacked"}`),
			})

			require.ErrorIs(t, err, common.ErrInvalid)
			assert.ErrorContains(t, err, key, "the rejection must name the offending key")
			assert.Equal(t, before, string(chr.Data()), "a rejected patch must not reach the sheet")
		})
	}
}

// Nested keys of the same name belong to whatever section holds them.
func TestApplyCharacterPatchRequest_AllowsServerOwnedNamesBelowTheTopLevel(t *testing.T) {
	t.Parallel()

	chr := test.MustCharacter(t, test.MustCharacterSheet(t))

	err := mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
		Patch: json.RawMessage(`{"identity":{"created_at":"whenever"}}`),
	})
	require.NoError(t, err)

	after := decodeSheet(t, chr.Data())
	assert.Equal(t, "whenever", after["identity"].(map[string]any)["created_at"])
}

// campaign_id is the client's, so null must clear it rather than be refused.
func TestApplyCharacterPatchRequest_NullDeletesAClientOwnedKey(t *testing.T) {
	t.Parallel()

	chr := test.MustCharacter(t, test.MustCharacterSheet(t))

	require.NoError(t, mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
		Patch: json.RawMessage(`{"campaign_id":"night-below"}`),
	}))
	require.Contains(t, decodeSheet(t, chr.Data()), "campaign_id")

	require.NoError(t, mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
		Patch: json.RawMessage(`{"campaign_id":null}`),
	}))
	assert.NotContains(t, decodeSheet(t, chr.Data()), "campaign_id")
}

// The merged result goes through the same schema gate a replacement does, so a
// patch that deletes a required section is refused even though the patch
// document itself is well formed.
func TestApplyCharacterPatchRequest_RejectsAMergeTheSchemaRefuses(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"deletes a required section": `{"vitals":null}`,
		"breaks a required field":    `{"schema_version":"not-a-version"}`,
		"writes the wrong type":      `{"identity":{"character_name":42}}`,
		"empties a min-items array":  `{"classes":[]}`,
	}

	for name, patch := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			chr := test.MustCharacter(t, test.MustCharacterSheet(t))
			before := string(chr.Data())

			err := mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
				Patch: json.RawMessage(patch),
			})

			require.ErrorIs(t, err, common.ErrInvalid)
			assert.Equal(t, before, string(chr.Data()), "a refused merge must not reach the sheet")
		})
	}
}

// RFC 7396 would have a non-object patch replace the whole document, which
// would leave the character holding a sheet that is a number.
func TestApplyCharacterPatchRequest_RejectsANonObjectPatch(t *testing.T) {
	t.Parallel()

	for _, patch := range []string{`5`, `"bar"`, `["c"]`, `true`} {
		t.Run(patch, func(t *testing.T) {
			t.Parallel()

			chr := test.MustCharacter(t, test.MustCharacterSheet(t))
			before := string(chr.Data())

			err := mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
				Patch: json.RawMessage(patch),
			})

			require.ErrorIs(t, err, common.ErrInvalid)
			assert.Equal(t, before, string(chr.Data()))
		})
	}
}

// A version-only patch has to be legal, or a client with nothing to change but
// a stale read gets a 400. Both spellings reach the mapper in practice: nil
// from a decode that saw no "patch" key, and the literal null that a nil
// json.RawMessage encodes as on its way back out.
func TestApplyCharacterPatchRequest_LeavesTheSheetAloneWhenThePatchIsOmitted(t *testing.T) {
	t.Parallel()

	for name, patch := range map[string]json.RawMessage{
		"absent": nil,
		"null":   json.RawMessage(`null`),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			chr := test.MustCharacter(t, test.MustCharacterSheet(t))
			before := string(chr.Data())

			require.NoError(t, mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{Patch: patch}))
			assert.Equal(t, before, string(chr.Data()))
		})
	}
}

func TestApplyCharacterPatchRequest_RejectsAnUndecodablePatch(t *testing.T) {
	t.Parallel()

	chr := test.MustCharacter(t, test.MustCharacterSheet(t))

	err := mapper.ApplyCharacterPatchRequest(chr, dto.PatchCharacterRequest{
		Patch: json.RawMessage(`{"identity":`),
	})
	assert.ErrorIs(t, err, common.ErrInvalid)
}

func decodeSheet(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc
}
