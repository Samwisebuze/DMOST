package jsonmerge_test

import (
	"encoding/json"
	"testing"

	"github.com/samwisebuze/dmost/internal/jsonmerge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApply_RFC7396AppendixA is the RFC's own example table, transcribed. It
// is the specification for this package: a change that breaks a row here has
// stopped implementing JSON Merge Patch, whatever else it does.
func TestApply_RFC7396AppendixA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		original string
		patch    string
		want     string
	}{
		{`{"a":"b"}`, `{"a":"c"}`, `{"a":"c"}`},
		{`{"a":"b"}`, `{"b":"c"}`, `{"a":"b","b":"c"}`},
		{`{"a":"b"}`, `{"a":null}`, `{}`},
		{`{"a":"b","b":"c"}`, `{"a":null}`, `{"b":"c"}`},
		{`{"a":["b"]}`, `{"a":"c"}`, `{"a":"c"}`},
		{`{"a":"c"}`, `{"a":["b"]}`, `{"a":["b"]}`},
		{`{"a":{"b":"c"}}`, `{"a":{"b":"d","c":null}}`, `{"a":{"b":"d"}}`},
		{`{"a":[{"b":"c"}]}`, `{"a":[1]}`, `{"a":[1]}`},
		{`["a","b"]`, `["c","d"]`, `["c","d"]`},
		{`{"a":"b"}`, `["c"]`, `["c"]`},
		{`{"a":"foo"}`, `null`, `null`},
		{`{"a":"foo"}`, `"bar"`, `"bar"`},
		{`{"e":null}`, `{"a":1}`, `{"e":null,"a":1}`},
		{`[1,2]`, `{"a":"b","c":null}`, `{"a":"b"}`},
		{`{}`, `{"a":{"bb":{"ccc":null}}}`, `{"a":{"bb":{}}}`},
	}

	for _, tc := range tests {
		t.Run(tc.original+" + "+tc.patch, func(t *testing.T) {
			t.Parallel()

			got, err := jsonmerge.Apply(json.RawMessage(tc.original), json.RawMessage(tc.patch))
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// A patch that does not name a key must not disturb it, whether or not any
// schema knows what it is. This is the property that lets a character sheet
// carry fields the generated v1alpha type has no home for.
func TestApply_LeavesUnnamedKeysAlone(t *testing.T) {
	t.Parallel()

	got, err := jsonmerge.Apply(
		json.RawMessage(`{"known":1,"house_rule_notes":{"nested":["untouched"]}}`),
		json.RawMessage(`{"known":2}`),
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"known":2,"house_rule_notes":{"nested":["untouched"]}}`, string(got))
}

// Decoding into float64 would round this to 12345678901234567168 and re-encode
// it as 1.2345678901234568e+19. UseNumber is what stops that, and nothing else
// in the package would fail if it were dropped.
func TestApply_PreservesNumericLiterals(t *testing.T) {
	t.Parallel()

	got, err := jsonmerge.Apply(
		json.RawMessage(`{"big":12345678901234567890,"exact":1.0,"small":3}`),
		json.RawMessage(`{"small":4}`),
	)
	require.NoError(t, err)
	assert.Equal(t, `{"big":12345678901234567890,"exact":1.0,"small":4}`, string(got))
}

// The result must not carry HTML escapes the client never wrote.
func TestApply_DoesNotEscapeHTML(t *testing.T) {
	t.Parallel()

	got, err := jsonmerge.Apply(
		json.RawMessage(`{"notes":"a < b & c > d"}`),
		json.RawMessage(`{"other":1}`),
	)
	require.NoError(t, err)
	assert.Contains(t, string(got), `"a < b & c > d"`)
}

func TestApply_ReportsUndecodableDocuments(t *testing.T) {
	t.Parallel()

	t.Run("target", func(t *testing.T) {
		t.Parallel()

		_, err := jsonmerge.Apply(json.RawMessage(`{`), json.RawMessage(`{}`))
		assert.ErrorContains(t, err, "decode target")
	})

	t.Run("patch", func(t *testing.T) {
		t.Parallel()

		_, err := jsonmerge.Apply(json.RawMessage(`{}`), json.RawMessage(`{`))
		assert.ErrorContains(t, err, "decode patch")
	})
}
