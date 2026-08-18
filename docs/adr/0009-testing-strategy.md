---
id: ARD-0009
title: Four test layers, with the round-trip corpus load-bearing
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0
supersedes:
  - ARD-0001 decision 9 — model unit tests plus golden View() snapshots, teatest rejected
related:
  - docs/psd/0001_character-management-tui.md
  - docs/adr/0003-generated-types-as-representation.md
  - docs/adr/0005-derivation-boundary.md
---

# ARD 0009 — Four test layers, with the round-trip corpus load-bearing

## Status

**Proposed.** Reverses ARD 0001 decision 9 on its central choice — that record rejected `teatest` and
PSD-0001 §10 adopts it — and keeps that record's two best ideas about how to make terminal snapshots
stable.

**Revisions.** v0 — initial draft.

## Context

ARD 0001 decision 9 chose two layers: state-transition tests over a `Model` fed synthetic messages, and
golden `View()` snapshots rendered at a fixed width with Lip Gloss forced to an unstyled color profile.
It rejected Charm's `teatest` in one sentence — "an experimental dependency" — and said to revisit if
something was found that neither layer could catch.

Several such things have been found, and none of them is a rendering problem.

- **ARD 0003 gave up a structural guarantee for a tested one.** Whole-document round-trip fidelity used
  to be guaranteed by never decoding. It is now guaranteed by a corpus, or not at all.
- **ARD 0007 lets the store hold invalid documents**, so the load path has states that only exist
  because of a decision made elsewhere.
- **ARD 0006's snapshot completeness depends on a SQL aggregation** rather than on anything a model
  test can see.
- **ARD 0005 is a boundary rather than an implementation**, and a boundary is only real if something
  fails when it moves.

The surface also grew: nine section screens, two wizards, a History screen, and three layout tiers each
of which changes what a screen renders.

## Decision

**Four layers, and the round-trip corpus is the one that matters most.**

### 1. Screens: `teatest`, and the experimental dependency is accepted

`charmbracelet/x/exp/teatest` drives each screen's `Update` with scripted key sequences and asserts on
rendered output.

ARD 0001's objection was one word, "experimental", and the answer it missed is that **a test dependency
does not ship**. `teatest` appears only in `_test.go` files; it is not in the binary a user runs, and
under module-graph pruning it is not in anything that imports `pkg/`. The cost of it breaking is a
morning's work on test files, which is an ordinary cost, not a risk to the product.

What it buys is the thing model tests cannot: an assertion that a key sequence a person would actually
type produces the screen they expect. The state machine and the rendering are the same machine when the
state is "which pane has focus".

### 2. Layout: golden snapshots at 80, 120, 160 — keeping ARD 0001's discipline

Driven by `tea.WindowSizeMsg` at each tier boundary, plus the below-80 refusal path.

Two rules from ARD 0001 decision 9 carry over unchanged, and they are what make layer 1 stable too:
render at a **fixed width set by the test** rather than by the ambient terminal, and force Lip Gloss to
an **unstyled color profile** so escape sequences stay out of the fixtures and the diffs stay readable.
A snapshot that changes when the runner's `TERM` changes is worse than no snapshot.

The split between layers 1 and 2 is deliberate: **`teatest` owns behaviour, goldens own layout.** A
golden that churns on every behaviour change is noise, so goldens snapshot one screen at one width in a
known state, never a whole session.

### 3. Derive: table-driven, plus the guard tests that make ARD 0005 real

A known-good character fixture with stored inputs and stored outputs, asserting that `Recompute`
reproduces the outputs. This doubles as a schema-fidelity regression test.

Three sets of cases beyond the happy path:

- **Boundary guards.** `Recompute` leaves `proficiency_bonus` untouched at a level where the 2024 table
  would change it; the same for carrying capacity and slot maxima. These exist to fail when a
  well-meaning contributor adds a lookup, per ARD 0005.
- **Override propagation.** `abilities.*.override_score` reaches every downstream modifier, save, skill
  and spell DC; `combat.armor_class.override` reaches nothing. ARD 0005 §6 explains why getting these
  backwards is otherwise invisible.
- **Evaluation order.** A mutation to an ability score in one pass produces a correct passive score, not
  a stale one.

### 4. Store: in-memory for speed, file-backed for the things that differ

`internal/store` tests run against an in-memory DSN, with a separate small suite against a temp file for
what is genuinely file-specific: WAL sidecar behaviour, the pragma set, and `VACUUM INTO`. This is the
same split `internal/infra/sqlite` already makes and for the same reason — locking behaviour genuinely
differs between the two.

### 5. The round-trip corpus: load → mutate → save → validate

This is the layer that replaces a guarantee, so it gets named fixtures rather than a description. The
structurally distinct cases: multiclass with two slot pools, a lineage-granted innate spell, a cursed
attuned item, an Exhaustion-leveled condition, a `1_1_1` background allocation, a soft-deleted row, and a
character with enough revisions to trigger pruning.

Plus the regressions that are invisible when they break, each traceable to the record that created them:

| what | why it needs its own test | record |
| --- | --- | --- |
| a populated `play_log` survives load → mutate → save with every `resolutions[]` element intact | silent loss, not visible failure | ARD 0004 |
| an unknown `resolutions[].kind` survives unaltered | PSD-0002's vocabulary is explicitly unstable | ARD 0004 |
| `restore --at` leaves `play_log` untouched while replacing everything else | the double-award guard; invisible until a player is paid twice | ARD 0004 |
| opening a character issues no reads against `item_instances` | the regression that turns lazy into eager | ARD 0002 |
| a snapshot taken with only a few items loaded still captures all of them | the specific way lazy + shared revision could break | ARD 0006 |
| restore reinstates document and item set atomically, including items added after the snapshot | partial restore | ARD 0006 |
| `export` → `import` round-trips items; a bare-document import shows the missing-item pane | not a crash, not an empty pane | ARD 0002 |
| pruning keeps the **union** of most-recent-50 and under-30-days | the intersection is the natural mistake | ARD 0006 |
| `purge` cascades to `character_revisions` and `item_instances` | fails silently if the `foreign_keys` pragma regresses | ARD 0002 |
| the undo stack's bound, and that undo restores derived values rather than recomputing them | a half-restored state recomputes wrong | ARD 0006 |
| every lifecycle command still fails without an explicit argument **while `DMOSH_CHARACTER` is set** | quietly reintroduces the footgun the carve-out exists to prevent | ARD 0008 |
| each rung of both precedence ladders in isolation, plus ambiguous-name and soft-deleted-target errors | ambient state is easy to get wrong by accident | ARD 0008 |

### 6. Two CI gates

`go generate` cleanliness — regenerate, `git diff --exit-code` — so a schema edit cannot land without
regenerated types (ARD 0003). And `dmosh character validate --all` against a seeded fixture database.

## Consequences

- **The round-trip corpus is only as good as its fixtures**, and it is now the sole defence of a
  property ARD 0001 held structurally. A schema section with no fixture has no protection.
- **`teatest` will break at some point**, being experimental, and the accepted answer is to fix the test
  files rather than to have avoided the dependency.
- **Goldens are review artifacts**, so they have to stay small enough that a human reads the diff. That
  constrains layer 2 more than layer 1.
- **Every record in this series ended up owing this one a test**, which is the honest cost of a design
  whose guarantees are mostly behavioural.
