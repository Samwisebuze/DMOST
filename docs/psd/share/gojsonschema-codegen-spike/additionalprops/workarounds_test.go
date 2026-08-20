package additionalprops_test

// `properties` + `additionalProperties` is not a dead end. Three routes, all
// round-tripping, in increasing order of what they cost you:
//
//	W1  hand-written UnmarshalJSON + MarshalJSON on the generated type.
//	    No schema change. Keeps declared fields typed. The default choice.
//	W2  goJSONSchema substituting a map-shaped type. Puts the seam in the
//	    schema; costs typed access to declared fields.
//	W3  for a *typed* additionalProperties, MarshalJSON alone — decoding is
//	    already correct.
//
// The one thing that does NOT work is reaching for a different spelling:
// `additionalProperties: {}` is identical to `true` all the way down.

import (
	"encoding/json"
	"reflect"
	"testing"

	"dmost.spike/additionalprops"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const openDoc = `{"known":"hi","extra":1,"nested":{"x":true}}`

// fieldByName reports a struct field's type as a string, for comparing what the
// generator emitted across two schema spellings.
func fieldByName(t *testing.T, v any, name string) (string, bool) {
	t.Helper()

	f, ok := reflect.TypeOf(v).FieldByName(name)
	if !ok {
		return "", false
	}

	return f.Type.String(), true
}

// W1. The remedy works precisely because of the defect: no UnmarshalJSON is
// generated for properties+true, so the method slot is free.
func TestW1HandWrittenMethodsOnGeneratedType(t *testing.T) {
	t.Parallel()

	var v additionalprops.OpenViaMethods
	require.NoError(t, json.Unmarshal([]byte(openDoc), &v))

	require.NotNil(t, v.Known)
	assert.Equal(t, "hi", *v.Known, "the declared field stays typed")
	assert.Equal(t, map[string]interface{}{
		"extra":  float64(1),
		"nested": map[string]interface{}{"x": true},
	}, v.Extras(), "and the undeclared keys are captured")

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, openDoc, string(out), "round-trips exactly")

	// Stability matters as much as correctness: the generated code's failure
	// mode was that a second pass mangled or dropped the data.
	var again additionalprops.OpenViaMethods
	require.NoError(t, json.Unmarshal(out, &again))
	assert.Equal(t, v.Extras(), again.Extras(), "and is stable across a second pass")

	require.NoError(t, additionalprops.ValidateAt("openViaMethods", out))

	// Contrast with the unpatched type, which is the same schema shape.
	var broken additionalprops.PropsTrue
	require.NoError(t, json.Unmarshal([]byte(openDoc), &broken))
	assert.Nil(t, broken.AdditionalProperties,
		"the unpatched generated type still loses the extras")
}

// W2. The map shape is not an aesthetic choice — it is what survives the
// defined-type conversion the generator performs at a $defs entry.
func TestW2MapShapedSubstituteAtDefsEntry(t *testing.T) {
	t.Parallel()

	var v additionalprops.OpenViaExtension
	require.NoError(t, json.Unmarshal([]byte(openDoc), &v))

	known, ok := additionalprops.OpenBag(v).Known()
	require.True(t, ok)
	assert.Equal(t, "hi", known, "declared fields cost a helper here")

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, openDoc, string(out), "round-trips exactly")

	require.NoError(t, additionalprops.ValidateAt("openViaExtension", out))

	// It must also survive being reached through the parent struct, which is
	// where a lost method would actually bite.
	var whole additionalprops.AdditionalPropsSchema
	require.NoError(t, json.Unmarshal([]byte(`{"open_via_extension":`+openDoc+`}`), &whole))

	wholeOut, err := json.Marshal(whole)
	require.NoError(t, err)
	assert.JSONEq(t, `{"open_via_extension":`+openDoc+`}`, string(wholeOut),
		"and through the parent, which is where a dropped method would show up")
}

// W3. Half the work, because the generated decoder is already correct.
func TestW3MarshalOnlyForTheTypedCase(t *testing.T) {
	t.Parallel()

	const doc = `{"known":"hi","extra":1,"other":2}`

	var v additionalprops.TypedViaMarshal
	require.NoError(t, json.Unmarshal([]byte(doc), &v))
	assert.Equal(t, map[string]int{"extra": 1, "other": 2}, v.AdditionalProperties,
		"the generated UnmarshalJSON is left in place and still works")

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, doc, string(out), "and the hand-written MarshalJSON inlines them")

	var again additionalprops.TypedViaMarshal
	require.NoError(t, json.Unmarshal(out, &again))
	assert.Equal(t, v.AdditionalProperties, again.AdditionalProperties,
		"stable across a second pass, unlike the unpatched type")

	require.NoError(t, additionalprops.ValidateAt("typedViaMarshal", out))
}

// The spelling that looks like a workaround and is not. `additionalProperties:
// {}` is the same schema as `true` — the generator produces the identical dead
// field, so switching spellings buys nothing.
func TestEmptySchemaIsNotAWorkaround(t *testing.T) {
	t.Parallel()

	viaEmpty, ok := fieldByName(t, additionalprops.PropsEmptySchema{}, "AdditionalProperties")
	require.True(t, ok)

	viaTrue, ok := fieldByName(t, additionalprops.PropsTrue{}, "AdditionalProperties")
	require.True(t, ok)

	assert.Equal(t, viaTrue, viaEmpty,
		"`additionalProperties: {}` generates exactly what `true` generates")

	var v additionalprops.PropsEmptySchema
	require.NoError(t, json.Unmarshal([]byte(openDoc), &v))
	assert.Nil(t, v.AdditionalProperties, "and loses the extras the same way")
}
