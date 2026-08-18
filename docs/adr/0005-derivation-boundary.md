---
id: ARD-0005
title: Derived values are arithmetic over inputs, never rules tables
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0.1
related:
  - docs/psd/0001_character-management-tui.md
  - docs/jsonschema/character/v1alpha/abilities.schema.json
---

# ARD 0005 — Derived values are arithmetic over inputs, never rules tables

## Status

**Proposed.** PSD-0001 calls this "the largest bet in the document" (§12 question 2) and designs the
instrumentation to find out whether it was right. This record states the boundary; it does not claim
the bet is won.

**Revisions.**

- **v0** — initial draft.
- **v0.1** — review pass. v0 listed the computed set but not the order it is computed in, which is
  underspecified for chained fields and genuinely ambiguous for overrides (§6). Also relocates the one
  rules-shaped default PSD-0001 admits to, which v0's "closed set" claim could not survive (§7).

## Context

`character.schema.json` stores inputs and computed values side by side. An ability score is
`base_score`, `background_increase`, `feat_increases[]`, `other_increases[]`, an optional
`override_score` — *and* `total_score`, `modifier`, `save_bonus_total`, all four of which are
`required`. `combat.armor_class` holds both a named `breakdown[]` and a `total`. `currency` holds
counts and a `derived` block with a conversion table.

So the document is deliberately denormalized, and something has to keep the computed half true. That
raises the question this record answers, which is not *whether* to compute but **how far computation
goes**. Two answers were available:

- **A character builder.** Proficiency bonus follows from total level. Spell slots follow from class
  and level. Carrying capacity is STR × 15. Speed follows from species. Each of these is a table, and
  once one is in, the argument for the next is identical.
- **A validating character sheet.** The tool knows the *shape* of a 2024 character and does the
  arithmetic reliably. It does not know that a Wood Elf has 35 ft of speed or that a level 5 Ranger
  has three 1st-level slots.

The pull toward the first is strong and constant, because each individual table looks small. Three
things argue against it beyond effort: a table the tool gets wrong is wrong in a way the player cannot
see and may not be able to correct; rules content is a licensing question the toolkit has not answered
outside SRD-derived catalog data; and a tool that computes from tables must then model every feature
that modifies them, which is unbounded.

## Decision

**`internal/character/derive` owns every computed field. The boundary is arithmetic over
user-supplied inputs, with no rules tables. `Recompute(*Document) []Change` is pure, and the list of
computed fields is a closed contract — anything not on it is an input.**

### 1. The computed set, exhaustively

- Ability total = `base_score` + `background_increase` + Σ`feat_increases` + Σ`other_increases`, or
  `override_score` when set
- Ability modifier = `floor((total_score - 10) / 2)`
- Save total = ability modifier + (PB if `save_proficient`) + `save_misc_bonus`
- Skill total = ability modifier + (PB × 1 if proficient, × 2 if expertise) + `misc_bonus`
- Passive scores = 10 + the corresponding skill total
- Initiative = DEX modifier + misc bonus
- AC total = sum of `armor_class.breakdown` terms, or `override`
- Spell save DC = 8 + PB + source ability modifier; spell attack bonus = PB + source ability modifier
- `prepared_current_count` = entries where `is_prepared` and `counts_against_prepared_maximum`
- Inventory per-entry weight, container roll-ups honouring `contents_are_weightless_to_carrier`,
  carried total
- Coin weight = total coins ÷ 50; currency `derived` totals via the document's own conversion table
- Attunement slots used = entries where `is_attuned`
- Encumbrance tier, only when the variant is enabled and thresholds are supplied

**Inputs, never computed:** proficiency bonus, HP maximum, hit dice totals, carrying capacity and
push/drag/lift, every AC breakdown term, every speed, spell slot maxima per level per pool, prepared
maximum, cantrips known, resource maxima, attunement slot maximum, spellcasting ability per source.

That the list is closed is what makes it a boundary. A contributor adding a lookup is not making a
judgement call; they are changing this record.

### 2. One function, called from one place

`Recompute` is pure — document in, `[]Change` out — and the parent model calls it after every
mutation (PSD-0001 §5.1), so derived-state bugs have exactly one home. Child screens emit typed
messages and never write derived fields themselves.

The `[]Change` return is not decoration: it is the same value `validate` needs to report drift and the
TUI needs to show its on-open banner. One function produces the diff; three callers decide what to do
with it.

### 3. It does not live in `pkg/domain`

`pkg/domain/character` cannot see inside a sheet by design — `Character` is an aggregate and a
`json.RawMessage`, and `NewCharacter`'s doc comment explains why. Derivation needs field-level
knowledge of a specific schema version, which is the one thing that package is built not to have. So
derive sits with the generated types (ARD 0003), under the same versioned lifetime as the schema it
encodes.

### 4. Write-back is restricted to three paths (D7)

`Recompute` runs in memory everywhere. Only three paths persist its output: the TUI on open (surfacing
the diff as a banner rather than applying it silently), the TUI on any subsequent edit, and explicit
`dmosh character recompute`.

`show` and `export` recompute for display only. `validate` compares stored against recomputed and
reports drift as **exit code 2** without touching the row. Read commands therefore stay safe in a
pipeline, `dmosh character show` against a database the TUI has open cannot write to it, and CI
stays reproducible.

The banner-not-silent-fix rule on open matters more than it looks: a document whose stored `total_score`
disagrees with its inputs is evidence of something, and quietly correcting it destroys the evidence.

### 5. Overrides are visible, and clearing them is an affordance

Where the schema offers an override (`abilities.*.override_score`, `combat.armor_class.override`), the
screen renders the computed value struck through and the override prominent, with a visible way to
clear it and return to computed. Never a silent "the number changed because you cleared something
three fields away".

### 6. Evaluation order and override semantics are part of the contract

Derived fields chain: ability modifier feeds save total, skill total, and spell save DC; skill total
feeds passive scores; per-entry weight feeds container roll-ups feeds carried total feeds encumbrance
tier. `Recompute` is therefore not a set of independent formulas but a single pass in dependency order,
and the order is:

1. ability totals → ability modifiers
2. saves, skills, initiative, spell save DC and attack bonus
3. passive scores
4. inventory per-entry weights → container roll-ups → carried total → attunement used → encumbrance
5. coin weight and currency `derived` totals

One pass, no fixed-point iteration, because nothing in the closed set is cyclic. A future computed
field that would create a cycle is a change to this record.

**The two overrides in the schema behave differently, and v0 read as though they were the same.**

| override | scope |
| --- | --- |
| `abilities.*.override_score` | replaces the total *and propagates* — every modifier, save, skill, passive score and spell DC downstream uses the overridden value |
| `combat.armor_class.override` | replaces `armor_class.total` and is **terminal** — nothing in the closed set reads AC |

That asymmetry is a property of the schema rather than a choice made here, but it has to be written
down: an implementer who made `override_score` terminal would produce a sheet whose STR was 20 and
whose Athletics bonus still said 14, and no test in the exhaustive list would necessarily catch it.
The Vesk Ambermarch fixture (ARD 0009) needs an override case for exactly this reason.

### 7. The one rules-shaped default lives outside `derive`

PSD-0001 §7.7 admits one exception to the no-rules stance: hit dice recovery on a long rest defaults to
half the pool total rounded down, editable per pool. It calls this "the one rules-shaped default in the
tool", and it is editable "precisely because it is a rule the tool shouldn't be certain about".

It does not go in `derive`. Rest is an *action* — it moves `current` toward `maximum` for pools whose
trigger matches, and proposes a hit-dice figure the user can change — and actions belong to the screen
that offers them (§7.7's rest panel). Putting it in `derive` would mean the closed set in §1 has an
exception in it, and a closed set with one exception is how the second exception gets argued for.

The distinction that keeps this honest: `derive` answers "what do these inputs add up to", and its
output is a function of the document alone. Rest answers "what should these values become now", and its
output depends on an event. Only the first is idempotent, and only the first can be compared against
stored state to produce `validate`'s exit code 2.

## Consequences

- **The tool will feel like it isn't pulling its weight, to some users, in a way that is measurable.**
  PSD-0001 §12 question 2 instruments exactly this: edit counts on the fields the tool refuses to
  compute, plus override usage. Frequent proficiency-bonus edits away from level-ups is the cheapest
  possible argument for deriving it, and the one table most likely to earn an exception.
- **Moving the boundary costs one function.** D2 is a boundary, not an architecture. That is the
  property that makes this record safe to be wrong about.
- **Guard tests are load-bearing, not incidental.** A test asserting `Recompute` leaves
  `proficiency_bonus` untouched at a level where the 2024 table would change it is the only thing
  standing between this decision and a well-meaning contributor. ARD 0009 carries them.
- **Every input the tool refuses to compute is an input the user can get wrong**, and the tool will
  faithfully propagate it. The mitigation is Phase 2b's read-only reference lines (§7.3, §7.8), which
  put the ladder on screen without applying it.
- **Proficiency bonus is the sharpest instance of that**, and worth naming separately: it is an input,
  and it appears in six of the formulas in §1. A PB left at 2 after reaching level 5 makes every save,
  every proficient skill, every passive score, and every spell save DC quietly wrong together, and
  `validate` reports nothing because a stale input is not drift. This is the specific failure §12
  question 2 is instrumented to detect, and the argument that would move the boundary.
- **`validate` exit code 2 covers computed fields only.** Drift means "stored disagrees with
  recomputed". An input that disagrees with the rules has no detector by construction.
