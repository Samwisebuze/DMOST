---
id: ARD-0011
title: Character schemas stay open through v1alpha
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0
supersedes:
  - ARD-0003 section 5 — the "durable fix" of additionalProperties false throughout, withdrawn
related:
  - docs/adr/0003-generated-types-as-representation.md
  - docs/adr/0004-play-log-schema-change.md
  - docs/adr/0009-testing-strategy.md
  - docs/jsonschema/character/v1alpha/README.md
  - docs/psd/0001_character-management-tui.md
---

# ARD 0011 — Character schemas stay open through `v1alpha`

## Status

**Proposed.** Withdraws the fix ARD 0003 §5 proposed and keeps the problem it was fixing, reassigning
it from the schema to the representation.

**Revisions.**

- **v0** — initial draft.

## Context

`additionalProperties: false` appears three times across the fourteen files in
`docs/jsonschema/character/v1alpha/`: at `character.schema.json`'s root, once in
`abilities.schema.json`, once in `spellcasting.schema.json`. Every other object is open, so an extra
key inside `combat`, `inventory.entries[]`, `identity` or `currency` is valid today.

ARD 0003 §5 measured what that costs and proposed closing it. Its finding: whether a schema-legal extra
key survives a whole-document round trip depends on nothing but what the generator emitted for the
object it sits in. Of the declarations in `internal/dto/v1alpha/character/character.gen.go`, 31 are
`map[string]interface{}` and keep the key; 73 are structs and drop it. Closing every object converts
that silent drop into a validation error naming the pointer, which is why §5 scheduled it into ARD
0004's single Phase 0 format bump.

Two things are true at once, and the record has to pick which one governs. The format is in alpha and
moving: `play_log` is arriving, PSD-0002's GM tool will want fields nothing has specified yet, and both
PSDs describe a catalog and homebrew work that will add more. And a closed schema turns every one of
those additions into a document-invalidating event for anything written before it, under a validator
ARD 0004 §7 establishes has no multi-version leniency and no plan for any.

## Decision

**`additionalProperties` stays open for the life of `v1alpha`**, throughout the character schema. The
three closed objects open with it, the root included.

An unrecognized field is data the tool does not yet support. Adding a field is additive by construction
and invalidates nothing written before it; a field the tool has no screen for is carried and ignored
until one exists. During alpha the schema's job is to describe what the tool understands, not to
enumerate everything a document may contain.

### 1. This makes preservation an obligation, not a property

The decision's stated intent is that an unsupported field is *inert*. That is not what the stack does
today, and leaving the schema open does not make it so — it only removes the layer that would have
objected. The generated tree has **no** unknown-key mechanism: there is not one `AdditionalProperties`
field anywhere in `character.gen.go`, and the struct-typed objects have no custom unmarshaler that
would retain one. So an unknown key inside any of the 73 struct-typed objects is dropped by
`encoding/json` on decode and gone on the next encode.

Composed with ARD 0007's debounced autosave, the sequence is: open a sheet carrying a field this
version does not know, wait two seconds, and the field is **deleted** — with no user edit, no error,
and a document that stays schema-valid so nothing reports it. Under ARD 0003 §5's closed schema that
same document would have been rejected loudly at load. Withdrawing the tightening therefore does not
make the problem smaller; it removes the only defence the record set had against it, and replaces
"loud rejection" with "silent deletion" unless something else is built.

So the obligation this record creates is explicit: **the save path must preserve keys the in-memory
representation has no field for.** Until it does, "inert until support is added" is aspirational, and
the honest reading of the decision is that the tool quietly eats fields it does not understand.

### 2. The mechanism is a merge, and the repository already has the primitive

The shape that satisfies §1 is that a save *merges* the re-encoded tree into the document it loaded,
rather than replacing it. Keys the editor never modelled are not named by the merge and survive; keys
it owns are overwritten. `internal/jsonmerge` already implements RFC 7396 at the format level, decoding
with `UseNumber` so a number cannot change shape in transit and encoding with `SetEscapeHTML(false)` so
a stored `<` comes back as `<` — both properties this path needs and neither of which a naive
re-marshal has.

**The hazard is real and this record does not settle it.** Merge-patch semantics cannot distinguish
*the editor does not know this key* from *the user cleared this field*: an absent key preserves the old
value rather than deleting it, so a cleared field silently reverts. Resolving that needs the save path
to emit explicit nulls for fields it owns and cleared, or to scope the merge to the keys the compiled
schema declares. That belongs in the record that specifies the write path, alongside ARD 0007's two
write use cases — not here. What this record fixes is that the whole-document *replace* now has a known
data-loss mode and may not be shipped as-is.

### 3. What this does to ARD 0004

It removes a premise and strengthens a rule.

ARD 0004's Context argues that `play_log` had to be a schema change rather than a tolerated extra
because the root is closed, so a document carrying it would be invalid. With the root open that premise
is gone — and it was already the weaker of the two arguments, since nothing in the tree enforces the
closed root today: the only validator is `mapper.validateSheet`, which decodes into the generated type
without `DisallowUnknownFields`, and no JSON Schema validator is a dependency of either module.
ARD 0004's sequencing survives on its second argument, which was always the load-bearing one: the Go
types are a build artifact of the schema, so `play_log` must exist in the schema before the tree the
whole TUI is built on is generated.

ARD 0004 §7's rule — *every subsequent change to `character.schema.json` within `v1alpha` must be
additive and optional* — becomes this record's rule, stated from the other direction, and the
contradiction between it and the widened Phase 0 goes away. ARD 0003 §5's tightening was the one
non-additive change scheduled inside the additive-only regime; withdrawing it makes the `1.1.0` bump
genuinely additive, exactly as ARD 0004 §3 claims.

### 4. What replaces the guarantee

ARD 0003 gave up ARD 0001's structural round-trip guarantee and named two replacements: the round-trip
corpus, and §5's tightening. One of those is now withdrawn, so the corpus is not one defence of two —
it is the only one, and ARD 0009's description of it as load-bearing becomes literal.

That raises its bar. A corpus that exercises only documents this version fully understands cannot see
this failure at all. It needs at least one fixture carrying a key no generated struct has a field for,
asserted to survive load → save unchanged — which will fail until §2 is built, and should.

## Consequences

- **Phase 0 gets smaller and less risky.** It carries `play_log`, the attunement default, the
  `schema_version` bump to `1.1.0` and regeneration — one additive format change. Opening the three
  closed objects rides along in the same bump. Nothing in Phase 0 invalidates a document any more, so
  PSD-0001 D25's "widened" framing narrows back.
- **The tool cannot tell a typo from a future field.** This is the cost of the decision, taken
  deliberately: `"stength": 14` is not an error, it is an unrecognized field, and it will be carried
  forever alongside the `strength` the user later fixes. A closed schema catches that at the pointer;
  an open one cannot, and no amount of preservation machinery helps. Import from a foreign tool is
  where this will bite first and hardest.
- **Documents accumulate fields nothing reads.** Preservation and accretion are the same mechanism seen
  from two ends. There is no reaper, no report of unknown keys, and no way for a user to see what their
  document is carrying — which is tolerable while the format is young and becomes a real gap around the
  time the format stops being young.
- **The decision has a horizon, and it is `v1alpha`.** Openness is a property of an alpha format, not a
  permanent stance. The version that graduates to `pkg/dto` should close its objects — at that point
  the format is a promise, the tool understands everything it declares, and loud rejection is the
  correct behaviour. Whoever writes the graduation record should treat closing as part of the work
  rather than a separate argument to win.
- **Until §2 lands, the stack is strictly worse at this than ARD 0003 §5 would have left it.** Silent
  deletion is a worse failure than loud rejection, and it is what the tree does today. That is the
  ordering constraint this record hands the implementer: the merge on save is not a refinement to do
  later, it is what makes this decision safe.
