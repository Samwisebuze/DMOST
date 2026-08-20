package character

// Pattern A — a hand-written type substituted into the generated tree by the
// generator itself, via the `goJSONSchema` extension keyword in the schema.
//
// spellcasting.schema.json's slotPool.slots is:
//
//	{ "type": "object",
//	  "patternProperties": { "^[1-9]$": { ...maximum, expended... } },
//	  "additionalProperties": false }
//
// go-jsonschema does not implement patternProperties. Left alone it emits
//
//	type SlotPoolSlots map[string]interface{}
//
// which is not a typing of the schema so much as an admission that it gave up:
// every read needs a type assertion, and every key is accepted. The schema
// instead carries
//
//	"slots": { ..., "goJSONSchema": { "type": "SlotsByLevel" } }
//
// and the generator emits `Slots SlotsByLevel` — no SlotPoolSlots declaration
// at all — leaving this file to say what the type is. The seam lives in the
// schema, which is the source of truth, rather than in a Go-side convention
// about which declarations are safe to hand-edit.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// SlotsByLevel is spell slots keyed by level. The map key carries the
// ^[1-9]$ constraint that the generated code could not express: it is an int,
// so a caller cannot spell a key wrong, and UnmarshalJSON refuses out-of-range
// and non-numeric keys rather than silently keeping them the way an
// additionalProperties-ignoring map[string]interface{} would.
type SlotsByLevel map[int]SlotCount

// SlotCount is one level's slots. Both fields are optional in the schema, so
// both are pointers: absent and zero are different states for a slot pool.
type SlotCount struct {
	Maximum  *int `json:"maximum,omitempty"`
	Expended *int `json:"expended,omitempty"`
}

const (
	minSlotLevel = 1
	maxSlotLevel = 9
)

// UnmarshalJSON enforces the patternProperties/additionalProperties pair.
//
// It is reached through the *generated* SlotPool.UnmarshalJSON: that method
// checks its own required fields and then does `json.Unmarshal(value, &plain)`
// over a struct whose Slots field is this type, so encoding/json dispatches
// here. The hand-written rule therefore runs on the generated decode path
// without the generated file knowing anything about it.
func (s *SlotsByLevel) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	out := make(SlotsByLevel, len(raw))

	for key, val := range raw {
		level, err := strconv.Atoi(key)
		if err != nil || level < minSlotLevel || level > maxSlotLevel {
			return fmt.Errorf("slot level %q: must match ^[1-9]$", key)
		}

		var count SlotCount
		if err := json.Unmarshal(val, &count); err != nil {
			return fmt.Errorf("slot level %d: %w", level, err)
		}

		if count.Maximum != nil && *count.Maximum < 0 {
			return fmt.Errorf("slot level %d: maximum %d: must be >= 0", level, *count.Maximum)
		}

		if count.Expended != nil && *count.Expended < 0 {
			return fmt.Errorf("slot level %d: expended %d: must be >= 0", level, *count.Expended)
		}

		out[level] = count
	}

	*s = out

	return nil
}

// MarshalJSON writes the levels back as decimal string keys, in level order.
// Go's own map marshalling would sort the int keys numerically and render them
// as strings, which happens to agree here; writing it out explicitly keeps the
// wire shape pinned to the schema rather than to that coincidence.
func (s SlotsByLevel) MarshalJSON() ([]byte, error) {
	levels := make([]int, 0, len(s))
	for level := range s {
		levels = append(levels, level)
	}

	sort.Ints(levels)

	var buf bytes.Buffer

	buf.WriteByte('{')

	for i, level := range levels {
		if i > 0 {
			buf.WriteByte(',')
		}

		if level < minSlotLevel || level > maxSlotLevel {
			return nil, fmt.Errorf("slot level %d: must be %d-%d", level, minSlotLevel, maxSlotLevel)
		}

		fmt.Fprintf(&buf, "%q:", strconv.Itoa(level))

		encoded, err := json.Marshal(s[level])
		if err != nil {
			return nil, fmt.Errorf("slot level %d: %w", level, err)
		}

		buf.Write(encoded)
	}

	buf.WriteByte('}')

	return buf.Bytes(), nil
}
