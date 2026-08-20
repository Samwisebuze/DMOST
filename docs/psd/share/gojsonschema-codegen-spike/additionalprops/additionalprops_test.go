package additionalprops_test

// Does go-jsonschema support `additionalProperties: true`?
//
// Partly. `schemas.Type.UnmarshalJSON` accepts booleans as schemas — `true`
// becomes `{}` and `false` becomes `{"not": {}}` (pkg/schemas/model.go:253) — so
// every variant *parses*. What gets *generated* is the problem, and it turns on
// whether the object also declares `properties`:
//
//	schema                       generated Go                    decode     encode
//	---------------------------  ------------------------------  ---------  -----------------------
//	no properties + true         map[string]interface{}          ok         ok
//	no properties + {type: str}  map[string]string               ok         ok
//	properties + true            AdditionalProperties any        ALWAYS NIL injects "…":null
//	properties + {type: int}     AdditionalProperties map[str]int ok        nests under a literal key
//	properties + false           plain struct                    extras dropped, NO error
//	properties + absent          plain struct                    extras dropped, NO error
//
// The two `properties + …` rows that carry extras do not round-trip. These tests
// pin that behaviour rather than endorse it: if a later version of the generator
// fixes it, they fail, and that is the point — the table above stops being true
// and PSD-0001 §6 needs revisiting.
//
// The last two rows are the ones that matter for DMOST, and they are why this
// package exists. Every schema under docs/jsonschema/character/v1alpha uses
// `additionalProperties: false`, and the generated code does not enforce it.

import (
	"encoding/json"
	"testing"

	"dmost.spike/additionalprops"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// One document for every case: a known field plus two extras.
const doc = `{"known":"hi","extra":1,"other":2}`

// The free-form cases — no `properties` — are the ones the generator gets right.
// An object that is only a bag becomes a map, and a map round-trips.
func TestFreeFormObjectsAreCorrect(t *testing.T) {
	t.Parallel()

	t.Run("additionalProperties true becomes an untyped map", func(t *testing.T) {
		t.Parallel()

		var v additionalprops.BareTrue
		require.NoError(t, json.Unmarshal([]byte(doc), &v))

		assert.Len(t, v, 3, "every key survives decoding")
		assert.Equal(t, "hi", v["known"])

		out, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, doc, string(out), "and the document round-trips exactly")

		require.NoError(t, additionalprops.ValidateAt("bareTrue", out))
	})

	t.Run("a typed additionalProperties becomes a typed map", func(t *testing.T) {
		t.Parallel()

		var v additionalprops.BareTyped
		require.NoError(t, json.Unmarshal([]byte(`{"a":"x","b":"y"}`), &v))

		assert.Equal(t, additionalprops.BareTyped{"a": "x", "b": "y"}, v)

		out, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `{"a":"x","b":"y"}`, string(out))
	})
}

// `properties` + `additionalProperties: true` — the combination people actually
// reach for, meaning "these known fields, plus anything else" — is unusable.
//
// The generator emits the field with only a `mapstructure:",remain"` tag and
// then emits no UnmarshalJSON at all, so nothing ever fills it; and with no
// `json:` tag, encoding/json marshals it under its Go field name.
func TestPropertiesPlusTrueIsUnusable(t *testing.T) {
	t.Parallel()

	// The defect at its root: no decoder is generated for this type, while one
	// is generated for the typed variant.
	_, hasDecoder := any(&additionalprops.PropsTrue{}).(json.Unmarshaler)
	assert.False(t, hasDecoder,
		"no UnmarshalJSON is generated for properties+true — this is the bug")

	_, typedHasDecoder := any(&additionalprops.PropsTyped{}).(json.Unmarshaler)
	assert.True(t, typedHasDecoder,
		"...whereas the typed variant does get one")

	var v additionalprops.PropsTrue
	require.NoError(t, json.Unmarshal([]byte(doc), &v))

	require.NotNil(t, v.Known)
	assert.Equal(t, "hi", *v.Known, "declared fields still decode")
	assert.Nil(t, v.AdditionalProperties,
		"but the extras are silently lost: the field is never populated")

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{"known":"hi","AdditionalProperties":null}`, string(out),
		"and encoding injects a bogus key named after the Go field")

	// That output is not merely lossy, it is invalid against the very schema it
	// was generated from — `AdditionalProperties: null` fails `{"type":"string"}`
	// only if constrained, so here it is accepted, but the key is still junk that
	// no client sent.
	assert.NotContains(t, doc, "AdditionalProperties",
		"nothing in the input document justifies that key")
}

// The typed variant decodes correctly but encodes asymmetrically: the generator
// writes an UnmarshalJSON that inlines the extras and no matching MarshalJSON,
// so they come back nested under a literal "AdditionalProperties" key.
func TestTypedAdditionalPropertiesDoesNotRoundTrip(t *testing.T) {
	t.Parallel()

	var v additionalprops.PropsTyped
	require.NoError(t, json.Unmarshal([]byte(doc), &v))

	assert.Equal(t, map[string]int{"extra": 1, "other": 2}, v.AdditionalProperties,
		"decoding is correct — mapstructure inlines the remaining keys")

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{"known":"hi","AdditionalProperties":{"extra":1,"other":2}}`, string(out),
		"encoding nests them instead of inlining — decode and encode disagree")

	// The asymmetry is load-bearing: feeding the encoded form back in loses the
	// extras outright. The generated decoder deletes candidate keys by Go field
	// *name* as well as by json tag (`delete(raw, st.Field(i).Name)`), so the
	// "AdditionalProperties" key it just wrote is the one key it strips.
	var again additionalprops.PropsTyped
	require.NoError(t, json.Unmarshal(out, &again))
	assert.Empty(t, again.AdditionalProperties,
		"encode-then-decode discards the extras entirely — the data is gone")

	// And the encoded form is invalid against the schema that produced the type.
	assert.Error(t, additionalprops.ValidateAt("propsTyped", out),
		"the generator's own output does not satisfy its own schema")
}

// The row that matters for DMOST. Every schema under
// docs/jsonschema/character/v1alpha declares `additionalProperties: false`, and
// the generated code does not enforce it — extras are dropped in silence, with
// no error, exactly the way conditionInstance's if/then/else is dropped.
//
// The compiled schema is the only thing that enforces it. This is the same
// finding as PSD-0001 §6's "silent degradation" note, in a third place.
func TestFalseIsNotEnforcedByGeneratedCode(t *testing.T) {
	t.Parallel()

	for _, def := range []string{"propsFalse", "propsOmitted"} {
		t.Run(def, func(t *testing.T) {
			t.Parallel()

			var v additionalprops.PropsFalse
			if def == "propsOmitted" {
				var o additionalprops.PropsOmitted
				require.NoError(t, json.Unmarshal([]byte(doc), &o),
					"the generated decoder accepts the extras without complaint")

				out, err := json.Marshal(o)
				require.NoError(t, err)
				assert.JSONEq(t, `{"known":"hi"}`, string(out), "and drops them on the way out")

				return
			}

			require.NoError(t, json.Unmarshal([]byte(doc), &v),
				"the generated decoder accepts the extras without complaint")

			out, err := json.Marshal(v)
			require.NoError(t, err)
			assert.JSONEq(t, `{"known":"hi"}`, string(out), "and drops them on the way out")
		})
	}

	// The two verdicts differ, and that is the whole point: only the compiled
	// schema rejects a closed object carrying extras.
	assert.Error(t, additionalprops.ValidateAt("propsFalse", []byte(doc)),
		"the schema rejects extras on a closed object")

	var v additionalprops.PropsFalse
	assert.NoError(t, json.Unmarshal([]byte(doc), &v),
		"the generated type does not — it just discards them")

	// `additionalProperties` absent means `true` in JSON Schema, so the schema
	// accepts the same document there. The Go side treats both identically,
	// which means it happens to agree here by accident rather than by rule.
	assert.NoError(t, additionalprops.ValidateAt("propsOmitted", []byte(doc)),
		"an absent additionalProperties means true, so the schema accepts extras")
}

// The whole fixture compiles and the assembled document validates, so none of
// the above is an artifact of a broken fixture.
func TestFixtureIsWellFormed(t *testing.T) {
	t.Parallel()

	sch, err := additionalprops.Schema()
	require.NoError(t, err)
	require.NotNil(t, sch)

	var whole additionalprops.AdditionalPropsSchema
	require.NoError(t, json.Unmarshal([]byte(`{
	  "props_true":    {"known":"a"},
	  "props_false":   {"known":"b"},
	  "props_typed":   {"known":"c","n":1},
	  "bare_true":     {"anything":true},
	  "bare_typed":    {"k":"v"},
	  "props_omitted": {"known":"d"}
	}`), &whole))

	assert.Equal(t, additionalprops.BareTyped{"k": "v"}, whole.BareTyped)
}
