---
id: ARD-0004
title: play_log enters character.schema.json as Phase 0 pre-work
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0
related:
  - docs/psd/0001_character-management-tui.md
  - docs/psd/0002_gm-managment-tui.md
  - docs/jsonschema/character/v1alpha/character.schema.json
---

# ARD 0004 — `play_log` enters `character.schema.json` as Phase 0 pre-work

## Status

**Proposed.** This is the first record that changes the character document format. It supersedes
nothing; it precedes everything, and PSD-0001 §11 schedules it as Phase 0 for the reasons in §2 below.

**Revisions.** v0 — initial draft.

## Context

`character.schema.json` has survived every revision of PSD-0001 unchanged. PSD-0001 v0.4.7 ends that,
and the reason comes from outside it: PSD-0002's GM session tool proposes changes to a character as
*patch packets*, and its safety model is idempotency — applying the same packet twice must not award
the same XP twice. PSD-0002 §6.3 is explicit that there is no separate `applied_patches` array; the
character's own log is the only record, so the two cannot disagree. It names "the character log",
assigns ownership of the change to PSD-0001, and never names the field.

Three properties of the existing contract turn this from a field addition into a sequencing decision.

- **The root is closed.** `character.schema.json` line 8 is `"additionalProperties": false`. A
  document carrying an unrecognized root field is *invalid*, so there is no version of this where the
  tool tolerates `play_log` now and formalizes it later — and `import` would reject the very documents
  the GM tool produces.
- **The Go types are a build artifact of the schema** (ARD 0003). A root field added after Phase 1
  means regenerating the tree, re-running every screen's model against it, and revisiting the
  round-trip corpus, for a field that could have been there from the first commit at no cost.
- **CLAUDE.md says wire-format changes get a new package, not edits to shipped DTOs.** Taken
  literally, adding a property to the v1alpha schema is exactly the edit that rule forbids.

## Decision

**Add `play_log` to `character.schema.json` as an optional root array, bump `schema_version` to
`1.1.0`, and land it before Phase 1 begins. It stays inside `v1alpha`; no new DTO package is created.**

### 1. `play_log`, not `character_log`

One new root property and one `$defs` entry — the full fragment is PSD-0001 §6.1, and it is the
authority on the shape:

```json
"play_log": { "type": "array", "items": { "$ref": "#/$defs/playLogEntry" } }
```

An entry is `entry_id`, `seq`, `at`, `kind`, optional campaign/session/tool-version provenance, and a
`patch` object holding `patch_id` and a `resolutions[]` array. PSD-0002 uses prose, not a field name,
so the name is a free choice: `character_log` inside a character document is redundant, while
`build_log` / `play_log` reads as the opposition it is — decisions made *building* a character, versus
things that happened *to* it in play.

### 2. Two typing decisions inside the fragment are load-bearing

- **`resolutions[].kind` is a bare string, not an enum.** PSD-0002 §10.7 makes no stability promise
  about its op vocabulary and requires receivers to skip unknown kinds with a warning rather than
  fail. An enum here would make recording *that skip* impossible — the document could not describe the
  op it had just declined — which defeats the re-offer mechanism the idempotency design rests on.
- **Entry-level `kind` *is* an enum** (`patch_applied`, `award`, `note`), because these are this
  tool's own entry types rather than a foreign vocabulary. Only `patch_applied` is needed by PSD-0002;
  the other two are reserved and nothing in v1 writes them.

### 3. It stays in `v1alpha`, and CLAUDE.md's rule does not bite

The rule protects *stability*, and CLAUDE.md's own account of the `internal/dto` ↔ `pkg/dto` split
says where that protection starts: pre-release versions live under `internal/` precisely so an
unstable contract cannot become an outside caller's dependency, and a version graduates by moving to
`pkg/dto`. `pkg/dto` holds no versions yet. `v1alpha` has no external consumers — enforced by the
import-path prefix, not by convention — so there is nothing an additive change can break.

Two version numbers are involved and they are not the same thing:

| version | versions what | this change |
| --- | --- | --- |
| `v1alpha` (package path) | the wire contract's compatibility promise | unchanged |
| `schema_version` (in the document) | the document format | `1.0.0` → `1.1.0` |

A *breaking* schema change would still need a new package. This one is additive and the property is
optional, so every existing document stays valid.

### 4. v1 reads it, validates it, carries it, and renders nothing

No screen in PSD-0001 appends a play-log entry. The tool generates types for it, validates it,
round-trips it without loss, includes it in snapshots and in `export`'s envelope, and shows it
nowhere. That is the honest scope: this is groundwork for PSD-0002, and building a UI for an
always-empty array would be worse than building none. Its likely eventual home is a third mode on the
History screen, which ARD 0006 and PSD-0001 §7.13 leave unstructured.

### 5. `restore` preserves the play log — the one exemption in the document

ARD 0006's `restore --at` reinstates a character document wholesale. If that included `play_log`,
restoring to a revision from before a patch was applied would erase the record of applying it, and the
next `patch apply` would re-offer ops the player already took — the exact double-award the idempotency
design exists to prevent.

So `play_log` is preserved across restore rather than replaced, and it is the only field with that
exemption. The asymmetry with `build_log` is the point: build decisions *are* sheet state, so rewinding
a sheet rewinds them, whereas the play log records things that happened in the world, and rewinding a
character sheet does not un-happen a session. Restore's confirmation names the exemption.

## Consequences

- **Phase 0 exists because of this record**, and PSD-0001's phasing is built around it: schema change,
  `schema_version` bump, regenerated types, no behaviour and no screens.
- **§6's mismatch banner fires once** for every document written before the bump. That is the intended
  behaviour, not a defect, and it is the first thing a Phase 1 user will see.
- **The GM tool's idempotency now has a place to live**, and PSD-0002's Phase 1 dependency on this
  record is discharged by a schema change rather than by a feature.
- **v1 carries a field it cannot show.** Anyone reading the generated types will find `PlayLog` with no
  screen behind it, and the schema is the only place explaining why.
- **`build_log` and `play_log` now differ in how `restore` treats them**, which is a rule that lives in
  the store rather than in the schema. Nothing in `character.schema.json` marks the exemption, so the
  only defence is ARD 0009's test that restore leaves the array untouched.
