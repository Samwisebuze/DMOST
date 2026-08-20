package character

// Pattern B — a method on a generated type, in a hand-written file in the same
// package. No extension keyword, no substitute type.
//
// vitals.schema.json's conditionInstance carries
//
//	"allOf": [{ "if":   { "properties": { "name": { "const": "Exhaustion" } }, ... },
//	           "then": { "required": ["level"], ... },
//	           "else": { "properties": { "level": { "type": "null" } } } }]
//
// go-jsonschema drops if/then/else entirely — and drops it *silently*: it emits
// a perfectly plausible ConditionInstance with `Level *int` and an
// UnmarshalJSON that checks instance_id and name and says nothing about the
// conditional. Nothing errors at generate time. The gap is only visible if you
// go looking for it.
//
// The important thing about this case is that it is NOT the same problem as
// SlotsByLevel, even though the project spec lists them side by side as "two
// constructs beyond the generator".
//
//   - slotPool.slots is a *typing* gap. The generated type cannot represent the
//     data, so Go code downstream is worse off. Only Go can fix that.
//   - conditionInstance is a *validation* gap. The generated type represents
//     the data fine; it just doesn't enforce one rule. The compiled schema
//     (see schema.go) enforces that rule already, exactly, from the same source
//     file — and it is the authority at load and save time.
//
// So this method is a convenience for callers holding an already-decoded value
// mid-edit, not a second source of truth. It deliberately restates only what
// the schema says, and the test asserts the two agree. If they ever disagree,
// the schema wins and this is the bug.

import "fmt"

// Validate reports whether the Exhaustion/level pairing holds. The compiled
// schema is the authority; see the note above.
func (c ConditionInstance) Validate() error {
	if c.Name == ConditionInstanceNameExhaustion {
		if c.Level == nil {
			return fmt.Errorf("condition %q: level is required for Exhaustion", c.Name)
		}

		return nil
	}

	if c.Level != nil {
		return fmt.Errorf("condition %q: level must be null for anything but Exhaustion", c.Name)
	}

	return nil
}
