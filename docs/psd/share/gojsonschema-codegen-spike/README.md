# Spike: extending generated types from `go-jsonschema`

Reference artifact for **PSD-0001 §6 / D22**. It answers what that spec could not
answer from the outside: *how* do the hand-written exceptions attach to a
generated type tree, which of §6's named constructs actually needs one, and —
since the answer turned out to generalise — which schema keywords the generator
drops on the floor without telling you.

Everything asserted in PSD-0001 §6 about the generator's behaviour was
established here, against `atombender/go-jsonschema` **v0.24.1**. If the spec and
this directory ever disagree, re-run the tests — they are the evidence.

## Running it

```bash
cd docs/psd/share/gojsonschema-codegen-spike
go generate ./...   # required first — the .gen.go files are not committed
go test ./...       # 11 test functions, 29 cases with subtests
```

This is a **third Go module**, deliberately outside the two that CLAUDE.md
describes. It is documentation that happens to compile, not part of the build:
nothing imports it, and a nested module is skipped by `./...` from the repo root,
so neither `go build ./...` nor `go test ./...` there is affected. It is exempt
from the "extend the two-invocation commands" rule for that reason. The module
path is `dmost.spike`, which does not resolve anywhere on purpose.

## The finding

`go-jsonschema` v0.24.1 reads an extension keyword that its README does not
document: **`goJSONSchema`**, honoured on any subschema (declared at
`pkg/schemas/model.go:208`, consumed at four points in
`pkg/generator/schema_generator.go`).

| field | valid position | effect |
| --- | --- | --- |
| `type` | type | emit this Go type name; generate nothing else for the construct |
| `imports` | type or property | add imports to the generated file |
| `nillable` | type | treat the substituted type as nil-able |
| `pointer` | type | force or suppress the pointer |
| `identifier` | property | override the generated Go field name |
| `extraTags` | property | add struct tags |

So the seam goes **in the schema**, which is already the source of truth, rather
than in a Go-side convention about which declarations are safe to hand-edit. The
generator names your type; you write it in a normal file in the same package.

## Three patterns, in increasing order of cost

**A — substitute a type** (`character/slots.go`), for a construct the generator
*cannot represent*. `slotPool.slots` is `patternProperties: {"^[1-9]$": ...}` with
`additionalProperties: false`; unaided, the generator emits
`type SlotPoolSlots map[string]interface{}` — every read needs a type assertion
and every key is accepted, including `"0"`, `"10"` and `"pact"`. With
`"goJSONSchema": {"type": "SlotsByLevel"}` it emits `Slots SlotsByLevel` and no
`SlotPoolSlots` declaration at all.

The load-bearing part: the generated `SlotPool.UnmarshalJSON` still runs its own
required-field checks and then calls `json.Unmarshal(value, &plain)`, so
`encoding/json` dispatches into `SlotsByLevel.UnmarshalJSON`. **The hand-written
rule runs on the generated decode path** without either file importing the other,
and you keep all of the generated validation.

**B — a method on a generated type** (`character/condition.go`). No extension, no
substitute type: Go permits methods on generated types from another file in the
same package. For rules where the generated *shape* is already correct.

**C — `identifier` / `extraTags`** on a property, for a field rename or a `db:`
tag, with no hand-written code at all. Demonstrated on `pool_id` → `PoolID`.

## Why PSD-0001's two constructs were split

Drafts through v0.4.7 listed `slotPool.slots` and `conditionInstance` together as
"the two patterns it can't express" and sent both to one `types_manual.go`. They
are different problems:

- `slotPool.slots` is a **typing** gap. The generated type cannot carry the data,
  so Go code downstream is worse off. Only Go can fix it → Pattern A.
- `conditionInstance` is a **validation** gap. `allOf`/`if`/`then`/`else` is
  dropped *silently* — you get a plausible `Level *int` and an `UnmarshalJSON`
  that checks `instance_id` and `name` and never mentions the rule — but the
  shape is fine. `santhosh-tekuri` compiled from the same file already enforces
  it exactly, and ADR-0003 makes it the authority. A hand-written type here is a
  **restatement**, not a fix.

`TestConditionalRulesNeedNoHandWrittenType` shows both halves: the generated type
decodes `{"name":"Exhaustion"}` with no `level` quite happily, and the compiled
schema rejects it. If a `Validate()` is kept for callers holding a decoded value
mid-edit, it is Pattern B and it is pinned to the schema — that test compares the
two verdicts across four cases so they cannot drift.

That also argues against the name `types_manual.go`: one file for "things the
generator couldn't do" mixes a real typing fix with a redundant validator. One
hand-written file per concept, named for the concept, beside the generated file.

## `additionalProperties` — a third instance of the same class

`additionalprops/` pins what the generator does with each of the six ways an
object can declare `additionalProperties`. The short answer to "does it support
`additionalProperties: true`" is *partly*: booleans parse fine (`true` becomes
`{}`, `false` becomes `{"not": {}}`, `pkg/schemas/model.go:253`), but what is
generated turns on whether the object also declares `properties`.

| schema | generated Go | decode | encode |
| --- | --- | --- | --- |
| no `properties` + `true` | `map[string]interface{}` | ok | ok |
| no `properties` + `{type: string}` | `map[string]string` | ok | ok |
| `properties` + `true` | `AdditionalProperties any` | **always nil** | injects `"AdditionalProperties":null` |
| `properties` + `{type: int}` | `AdditionalProperties map[string]int` | ok | nests under a literal key |
| `properties` + `false` | plain struct | extras dropped, **no error** | ok |
| `properties` + absent | plain struct | extras dropped, **no error** | ok |

Free-form objects are handled correctly. The two rows that carry extras alongside
declared fields do **not** round-trip: `properties` + `true` gets no
`UnmarshalJSON` generated at all, so the field is never populated and encoding
injects a key named after the Go field; the typed variant decodes correctly but
has no matching `MarshalJSON`, so encode nests what decode inlined, and feeding
that back in discards the extras outright.

The row that matters for DMOST is the fifth. Every schema under
`docs/jsonschema/character/v1alpha` uses `additionalProperties: false`, and
**the generated code does not enforce it** — extras are dropped in silence, with
no error, exactly the way `conditionInstance`'s `if`/`then`/`else` is dropped.
Only the compiled schema rejects them. That is the same silent-degradation
finding as the two constructs above, in a third place, which is what makes it a
class rather than a pair of accidents.

These tests pin current behaviour rather than endorse it. If a later version of
the generator fixes the broken rows, they fail — and that is the point: the table
stops being true and PSD-0001 §6 needs revisiting.

## Gotchas found the hard way

- **The extension is ignored at `$ref` sites.** `generateTypeInline` guards the
  extension block with `t.Enum == nil && t.Ref == ""`. Put `goJSONSchema` on the
  `$defs` entry itself, never on the property that `$ref`s it.
- **`--extra-imports` is not about the `imports` field.** Despite the config
  comment ("allows the generator to pull imports from outside the standard
  library"), the flag only appends the *YAML formatter*
  (`pkg/generator/generate.go:57`). `imports` works without it.
- **The keyword is inert to the validator.** JSON Schema ignores unknown
  keywords, so the patched file still compiles under draft 2020-12 and still
  validates real documents — asserted in
  `TestExtensionKeywordIsInertToTheValidator`. The cost is that a Go-specific
  hint now lives in a language-neutral contract file; that is the strongest
  argument for the parallel-types approach D22 replaces, and it is recorded as
  correction #7 in PSD-0001 §0.
- **Regeneration is non-destructive.** `-o` fixes one output path, so the
  `.gen.go` is the only file rewritten and the hand-written files beside it
  survive untouched.
- **Silent degradation is the real hazard.** Neither construct produced a warning
  or a non-zero exit. A CI `go generate && git diff --exit-code` catches *drift*,
  but nothing catches "the generator quietly gave up on this property" — only a
  test like `TestHandWrittenRuleAgreesWithTheCompiledSchema`, which runs the same
  cases past both the hand-written Go and the compiled schema and asserts the
  verdicts match.

## Layout

```
gen.go                        the two //go:generate directives
character/schema/*.schema.json  14 files: the production schemas, byte-identical
                              except the goJSONSchema keywords in
                              spellcasting.schema.json — diff the directory
                              against docs/jsonschema/character/v1alpha to see
                              the entire change
character/character.gen.go    generated (gitignored), DO NOT EDIT
character/slots.go            Pattern A — SlotsByLevel
character/condition.go        Pattern B — method on a generated type
character/schema.go           the compiled schema: the authority both defer to
character/spike_test.go       the 7 claims
baseline/baseline.gen.go      generated (gitignored) straight from
                              docs/jsonschema/character/v1alpha — the control
additionalprops/              the six additionalProperties variants; a
                              self-contained fixture, not a production schema,
                              because the character schemas only ever use `false`
```

Two details about that layout are deliberate. The schemas sit *inside* the
`character` package because `character/schema.go` embeds them and `go:embed`
cannot reach above its own package; the generator is pointed at the same copy so
one set of files serves both the types and the validator. And the control is
generated from the production schemas directly, with no copy, so it cannot drift
from what the repo ships — every difference between the two generated packages is
caused by the extension keywords and nothing else.

## What this spike does *not* settle

§6's **Vendoring** paragraph still describes a single vendored
`character.schema.json`; it is 14 files, which `docs/QUESTIONNAIRE.md` Q2 has open.
This spike does not resolve that question, but it is a working demonstration of
the shape the answer would take: all 14 files embedded via `go:embed` and
registered with an offline loader against their `example.com` `$id` prefix, so
nothing is fetched over the network (`character/schema.go`).
