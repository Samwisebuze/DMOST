package character_test

// What this spike proves, one test per claim:
//
//  1. Left alone, the generator degrades patternProperties to
//     map[string]interface{} — silently.
//  2. The `goJSONSchema` extension makes it emit a hand-written type instead,
//     and stops emitting its own.
//  3. The hand-written UnmarshalJSON runs on the *generated* decode path.
//  4. The hand-written rule agrees with the compiled schema, in both directions.
//  5. Round-tripping through the hand-written type preserves the wire shape.
//  6. The extension keyword is inert to the runtime validator — the same file
//     still compiles and still validates real documents.
//  7. The other class of gap (if/then/else) needs no hand-written type at all,
//     because the compiled schema already covers it.

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"dmost.spike/baseline"
	"dmost.spike/character"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fieldType(t *testing.T, v any, name string) reflect.Type {
	t.Helper()

	f, ok := reflect.TypeOf(v).FieldByName(name)
	require.True(t, ok, "no field %q on %T", name, v)

	return f.Type
}

// 1 + 2. The baseline is what you get without the extension. Both packages are
// generated from the same 14 files by the same generator in the same run; the
// only difference is the extension keyword on one property.
func TestExtensionReplacesTheUntypedMap(t *testing.T) {
	t.Parallel()

	baseSlots := fieldType(t, baseline.SlotPool{}, "Slots")
	assert.Equal(t, "baseline.SlotPoolSlots", baseSlots.String(),
		"baseline: the generator names the type after the property...")
	require.Equal(t, reflect.Map, baseSlots.Kind())
	assert.Equal(t, "map[string]interface {}",
		reflect.MapOf(baseSlots.Key(), baseSlots.Elem()).String(),
		"...but it is an untyped map underneath — the name buys nothing")

	assert.Equal(t, "character.SlotsByLevel",
		fieldType(t, character.SlotPool{}, "Slots").String(),
		"extension: the generator emits the hand-written type")

	// And it emits *nothing else* for that property: baseline declares a
	// SlotPoolSlots named type, the extension build has no such declaration.
	// (Asserted by reading the generated files, below.)
	base, err := os.ReadFile("../baseline/baseline.gen.go")
	require.NoError(t, err)
	ext, err := os.ReadFile("character.gen.go")
	require.NoError(t, err)

	assert.Contains(t, string(base), "type SlotPoolSlots map[string]interface{}")
	assert.NotContains(t, string(ext), "SlotPoolSlots",
		"the generator should leave the type entirely to the hand-written file")

	// The property-level half of the extension: a field rename and an extra tag.
	assert.Equal(t, "PoolId", func() string {
		f, _ := reflect.TypeOf(baseline.SlotPool{}).FieldByName("PoolId")

		return f.Name
	}())

	f, ok := reflect.TypeOf(character.SlotPool{}).FieldByName("PoolID")
	require.True(t, ok, "identifier override should rename the field")
	assert.Equal(t, `pool_id`, f.Tag.Get("db"), "extraTags should reach the struct tag")
	assert.Equal(t, `pool_id`, f.Tag.Get("json"), "and must not disturb the wire name")
}

const validPool = `{
  "pool_id": "p1",
  "recharge_trigger": "long_rest",
  "slots": {
    "1": { "maximum": 4, "expended": 1 },
    "3": { "maximum": 2, "expended": 0 }
  }
}`

// 3. The hand-written UnmarshalJSON is reached through the generated
// SlotPool.UnmarshalJSON, which neither imports nor mentions it.
func TestHandWrittenRuleRunsOnTheGeneratedDecodePath(t *testing.T) {
	t.Parallel()

	t.Run("valid document decodes to typed access", func(t *testing.T) {
		t.Parallel()

		var pool character.SlotPool
		require.NoError(t, json.Unmarshal([]byte(validPool), &pool))

		// int keys, typed values — no assertion, no string parsing.
		require.Len(t, pool.Slots, 2)
		require.NotNil(t, pool.Slots[1].Maximum)
		assert.Equal(t, 4, *pool.Slots[1].Maximum)
		assert.Equal(t, 2, *pool.Slots[3].Maximum)

		_, present := pool.Slots[2]
		assert.False(t, present)
	})

	for _, tc := range []struct {
		name string
		key  string
	}{
		{"level 0 is out of range", "0"},
		{"level 10 is out of range", "10"},
		{"a non-numeric key is not a level", "cantrip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			doc := `{"pool_id":"p1","recharge_trigger":"long_rest","slots":{"` + tc.key + `":{"maximum":1}}}`

			var pool character.SlotPool
			err := json.Unmarshal([]byte(doc), &pool)
			require.Error(t, err, "the hand-written rule should reject this")
			assert.Contains(t, err.Error(), "^[1-9]$")

			// The baseline accepts every one of them without complaint.
			var loose baseline.SlotPool
			require.NoError(t, json.Unmarshal([]byte(doc), &loose),
				"baseline keeps junk keys — this is the gap being closed")
		})
	}

	t.Run("the generated required-field check still runs", func(t *testing.T) {
		t.Parallel()

		var pool character.SlotPool
		err := json.Unmarshal([]byte(`{"pool_id":"p1","recharge_trigger":"long_rest"}`), &pool)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field slots in SlotPool: required",
			"substituting a type must not cost the generated validation")
	})
}

// 4. The hand-written rule and the compiled schema agree. If this test ever
// fails, the schema is right and slots.go is the bug.
func TestHandWrittenRuleAgreesWithTheCompiledSchema(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		doc   string
		valid bool
	}{
		{"levels 1-9", `{"pool_id":"p","recharge_trigger":"dawn","slots":{"1":{"maximum":4},"9":{"maximum":1}}}`, true},
		{"empty slots", `{"pool_id":"p","recharge_trigger":"dawn","slots":{}}`, true},
		{"level 0", `{"pool_id":"p","recharge_trigger":"dawn","slots":{"0":{"maximum":1}}}`, false},
		{"level 10", `{"pool_id":"p","recharge_trigger":"dawn","slots":{"10":{"maximum":1}}}`, false},
		{"named key", `{"pool_id":"p","recharge_trigger":"dawn","slots":{"pact":{"maximum":1}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			schemaErr := character.ValidateAt("spellcasting.schema.json#/$defs/slotPool", []byte(tc.doc))

			var pool character.SlotPool
			goErr := json.Unmarshal([]byte(tc.doc), &pool)

			assert.Equal(t, tc.valid, schemaErr == nil, "schema verdict: %v", schemaErr)
			assert.Equal(t, tc.valid, goErr == nil, "hand-written verdict: %v", goErr)
			assert.Equal(t, schemaErr == nil, goErr == nil,
				"the two must agree; schema=%v go=%v", schemaErr, goErr)
		})
	}
}

// 5. Marshalling back produces the same document the schema accepts.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	var pool character.SlotPool
	require.NoError(t, json.Unmarshal([]byte(validPool), &pool))

	out, err := json.Marshal(pool)
	require.NoError(t, err)

	require.NoError(t, character.ValidateAt("spellcasting.schema.json#/$defs/slotPool", out))

	var again character.SlotPool
	require.NoError(t, json.Unmarshal(out, &again))
	assert.Equal(t, pool.Slots, again.Slots)

	// String keys on the wire, ints in Go.
	var wire struct {
		Slots map[string]json.RawMessage `json:"slots"`
	}
	require.NoError(t, json.Unmarshal(out, &wire))
	assert.ElementsMatch(t, []string{"1", "3"}, keysOf(wire.Slots))
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

// 6. The extension keyword lives in the published contract file. Prove it costs
// nothing at runtime: the same patched file compiles as a schema and still
// validates a real document.
func TestExtensionKeywordIsInertToTheValidator(t *testing.T) {
	t.Parallel()

	sch, err := character.Schema()
	require.NoError(t, err, "the patched schema must still compile")
	require.NotNil(t, sch)

	// The repo's own fixture, read in place rather than copied, so this cannot
	// pass against a stale duplicate.
	doc, err := os.ReadFile("../../../../../internal/test/testdata/character.v1alpha.json")
	require.NoError(t, err)

	assert.NoError(t, character.Validate(doc),
		"a real document must still validate against the patched schema")

	// The keyword really is present in the compiled file.
	raw, err := os.ReadFile("schema/spellcasting.schema.json")
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"goJSONSchema"`)
}

// 7. The other class of gap. if/then/else is dropped silently by the
// generator, but the compiled schema enforces it — so this one needs no
// substitute type, only (optionally) a method.
func TestConditionalRulesNeedNoHandWrittenType(t *testing.T) {
	t.Parallel()

	const badExhaustion = `{"instance_id":"c1","name":"Exhaustion"}`

	// The generated type is happy to decode it.
	var cond character.ConditionInstance
	require.NoError(t, json.Unmarshal([]byte(badExhaustion), &cond),
		"the generator drops if/then/else with no error and no warning")

	// The compiled schema is not.
	require.Error(t, character.ValidateAt("vitals.schema.json#/$defs/conditionInstance", []byte(badExhaustion)),
		"the schema still enforces the rule the generator dropped")

	// The convenience method restates it, and must agree.
	require.Error(t, cond.Validate())

	for _, tc := range []struct {
		name  string
		doc   string
		valid bool
	}{
		{"Exhaustion with level", `{"instance_id":"c1","name":"Exhaustion","level":3}`, true},
		{"Exhaustion without level", `{"instance_id":"c1","name":"Exhaustion"}`, false},
		{"Prone without level", `{"instance_id":"c2","name":"Prone"}`, true},
		{"Prone with level", `{"instance_id":"c2","name":"Prone","level":2}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			schemaErr := character.ValidateAt("vitals.schema.json#/$defs/conditionInstance", []byte(tc.doc))

			var c character.ConditionInstance
			require.NoError(t, json.Unmarshal([]byte(tc.doc), &c))

			assert.Equal(t, tc.valid, schemaErr == nil, "schema: %v", schemaErr)
			assert.Equal(t, tc.valid, c.Validate() == nil, "method: %v", c.Validate())
		})
	}
}
