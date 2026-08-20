package additionalprops

// The remedies for the two rows of the table that do not round-trip.
//
// `properties` + `additionalProperties` is not a dead end — it needs a
// hand-written decode/encode pair, and one of the three routes below needs no
// schema change at all. Each is applied to its own $defs entry so that
// propsTrue and propsTyped stay unpatched and keep documenting the defect;
// diagnosis and remedy are deliberately separate types.

import (
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// W1 — Pattern B on the generated type. No schema change.
//
// This works *because* of the bug: the generator emits no UnmarshalJSON for a
// properties+true object, so the method slot is free and we can supply both
// halves ourselves from this file. The declared field stays typed.
//
// The generated field is `AdditionalProperties interface{}`; we keep a
// map[string]interface{} in it. That the generator chose `interface{}` rather
// than a map is part of the same defect, and is why the read path below asserts
// the type rather than indexing directly.

// UnmarshalJSON decodes the declared fields and collects everything else.
func (o *OpenViaMethods) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if v, ok := raw["known"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return fmt.Errorf("known: %w", err)
		}

		o.Known = &s

		delete(raw, "known")
	}

	extras := make(map[string]interface{}, len(raw))

	for k, v := range raw {
		var value interface{}
		if err := json.Unmarshal(v, &value); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}

		extras[k] = value
	}

	o.AdditionalProperties = extras

	return nil
}

// MarshalJSON writes the extras back inline, beside the declared fields, which
// is what the generated code fails to do.
func (o OpenViaMethods) MarshalJSON() ([]byte, error) {
	out := map[string]interface{}{}

	if extras, ok := o.AdditionalProperties.(map[string]interface{}); ok {
		for k, v := range extras {
			out[k] = v
		}
	}

	// Declared fields are written last so a stray extra of the same name cannot
	// shadow them — the schema's property wins.
	if o.Known != nil {
		out["known"] = *o.Known
	}

	return json.Marshal(out)
}

// Extras returns the undeclared keys, saving every caller the type assertion
// that the generated `interface{}` field would otherwise force.
func (o OpenViaMethods) Extras() map[string]interface{} {
	extras, _ := o.AdditionalProperties.(map[string]interface{})

	return extras
}

// ---------------------------------------------------------------------------
// W2 — Pattern A, with the substituted type deliberately map-shaped.
//
// The extension sits on a $defs entry, so the generator emits
//
//	type OpenViaExtension OpenBag
//
// A *defined* type does not inherit the methods of the type it is defined from.
// Had OpenBag been a struct carrying custom MarshalJSON/UnmarshalJSON, those
// methods would be silently absent from OpenViaExtension and the round-trip
// would break in a way nothing would flag.
//
// A map shape sidesteps that entirely: encoding/json handles maps natively, so
// the behaviour survives the defined-type conversion without any methods to
// lose. The cost is that declared fields are no longer typed struct fields, so
// reading one costs a helper.

// OpenBag is an object that keeps every key it was given.
type OpenBag map[string]interface{}

// Known reads the one declared field back out. This helper is the price of the
// map shape, and the reason W1 is the better default when a schema declares
// more than one or two properties.
func (o OpenBag) Known() (string, bool) {
	s, ok := o["known"].(string)

	return s, ok
}

// ---------------------------------------------------------------------------
// W3 — the typed case needs half as much work.
//
// For `additionalProperties: {"type": "integer"}` the generator does emit an
// UnmarshalJSON, and it is correct: mapstructure inlines the remaining keys
// into a map[string]int. Only encoding is broken. So supply MarshalJSON alone —
// it is never generated, so there is nothing to collide with, and writing one
// here does not disturb the generated decoder.

// MarshalJSON writes the extras inline instead of nesting them under a literal
// "AdditionalProperties" key.
func (t TypedViaMarshal) MarshalJSON() ([]byte, error) {
	out := map[string]interface{}{}

	for k, v := range t.AdditionalProperties {
		out[k] = v
	}

	if t.Known != nil {
		out["known"] = *t.Known
	}

	return json.Marshal(out)
}
