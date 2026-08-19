---
id: ARD-0009
title: Five test layers, with the round-trip corpus load-bearing
updated: 2026-08-19
status:
  - kind: proposed
  - version: v0.2
supersedes:
  - ARD-0001 decision 9 — model unit tests plus golden View() snapshots, teatest rejected
related:
  - docs/psd/0001_character-management-tui.md
  - docs/adr/0002-character-document-store.md
  - docs/adr/0003-generated-types-as-representation.md
  - docs/adr/0004-play-log-schema-change.md
  - docs/adr/0005-derivation-boundary.md
  - docs/adr/0006-undo-snapshots-restore.md
  - docs/adr/0008-cli-shape-and-ambient-resolution.md
  - docs/adr/0011-open-schemas-through-v1alpha.md
---

# ARD 0009 — Five test layers, with the round-trip corpus load-bearing

## Status

**Proposed.** Reverses ARD 0001 decision 9 on its central choice — that record rejected `teatest` and
PSD-0001 §10 adopts it — and keeps that record's two best ideas about how to make terminal snapshots
stable.

**Revisions.**

- **v0** — initial draft.
- **v0.1** — review pass. The known-good fixture layer 3 is built on does not exist in this repository
  (§6), and layer 5 needs its comparison defined before it can be written, because the obvious one is
  unattainable and the next-obvious one hides exactly the loss it is there to catch (§7).
- **v0.2** — review pass. The record specified five layers and called itself four, and its sections
  skipped a number; both are corrected, and §6–§8 are the old §7–§9. Layer 4 named `internal/store`,
  which ARD 0002 v0.2 decided against building (§4). And ARD 0011 arrived after v0.1, making the corpus
  the sole defence of round-trip fidelity and handing this record a fixture it did not have (§5).

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
- **ARD 0011 leaves the schema open and puts preservation on the save path**, which turns a class of
  document — one carrying keys this build has no field for — into something only a load-and-save test
  can say anything about.

The surface also grew: nine section screens, two wizards, a History screen, and three layout tiers each
of which changes what a screen renders.

## Decision

**Five layers, and the round-trip corpus is the one that matters most.**

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
- **Purity.** `Recompute` over a document leaves that document byte-identical; `Apply` is the only
  function that changes one. ARD 0005 §2 makes the split load-bearing precisely because the failure is
  invisible — a mutating `Recompute` produces the same banner as a pure one, and ARD 0007's debounce
  then writes the difference to disk.

### 4. Store: the port contract against both adapters, then what only SQLite has

v0.1 put this layer on `internal/store`. ARD 0002 v0.2 decided that package is not built — the existing
stack is extended instead — so the layer moves to where the behaviour now lives, and gains a shape the
repository already has:

- **`internal/test/repotest` is the layer.** The rules a `CharacterRepository` owes its callers — upsert
  semantics, the compare-and-set on version, ARD 0002 §1's unloaded-items rule, soft delete, revisions —
  are scenarios both adapters run. `sqlite` runs the whole contract; `inmem` runs what ARD 0010 §2 leaves
  it obliged to implement. Two backends held to one set of answers is the only thing keeping them from
  drifting, and ARD 0002 §9 removed the second composition root that used to help.
- **`internal/infra/sqlite` keeps its own suite** for what is true of the technology rather than the
  port: WAL sidecar behaviour, the pragma set, `VACUUM INTO`, and the migration runner. It runs the
  contract against **both** an in-memory DSN and a temp file, because locking behaviour genuinely differs
  between the two — the split CLAUDE.md already describes, extended to the character repository.

### 5. The round-trip corpus: load → mutate → save → validate

This is the layer that replaces a guarantee, so it gets named fixtures rather than a description. The
structurally distinct cases: multiclass with two slot pools, a lineage-granted innate spell, a cursed
attuned item, an Exhaustion-leveled condition, a `1_1_1` background allocation, a soft-deleted row, a
character with enough revisions to trigger pruning, and — per ARD 0011 §4 — one carrying a key no
generated struct declares.

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
| a schema-legal key no generated struct declares survives load → mutate → save | the corpus is the only defence left; **this fails today** | ARD 0011 |
| `restore --at` rewinds those same unmodelled keys, while `play_log` stays current | the one place restore must *not* use the preservation merge | ARD 0006 |
| restore writes a `before restore to rN` revision, and writes none when the current state is already the newest revision | the guarantee that a restore is answerable by another restore | ARD 0006 |
| every snapshotting write path supplies a non-empty `summary` — import, wizard, recompute, both restore steps | a blank History row is how the required field gets argued away | ARD 0006 |

### 6. The known-good fixture has to be written; it does not exist

PSD-0001 §10 specifies layer 3 as "table-driven against the schema report's worked example ('Vesk
Ambermarch') as a known-good fixture", and both PSD-0001 and PSD-0002 carry
`docs/psd/share/2024-character-schema-report.md` in their `related` lists as the schema report.

That file is not a schema report. It is a copy of PSD-0002, the GM Session Tool spec, front matter and
all — added by `e3ec7ac doc(docs/psd): publish 0002 GM Session Tool` — and its own front matter points at
`docs/psd/share/dnd-2024-character-schema-report.md`, which this repository has never contained. The name
"Vesk Ambermarch" appears nowhere in it. So the worked example that layer 3, the derive contract, and
several citations across PSD-0001 (§5.3's optionality rule, §7.8's "Part 4", §9.4's "Part 2", §7.10's
stance on currency normalization) all rest on is not available to anyone implementing this.

Two consequences for the test strategy, and one for someone else:

- **The fixture is authored, not extracted.** A schema-valid 2024 character with hand-computed outputs,
  living in `testdata/`, reviewed once as arithmetic and then trusted. It is a fixture either way; the
  difference is that nobody can pretend it was validated against a report they cannot read.
- **`internal/test/testdata/character.v1alpha.json` is not it.** That document exists, is named
  "Bruenor", and carries eleven of the root sections — it is the daemon's minimum-viable valid sheet, not
  a worked example with derived outputs to check against. Layer 3 needs a second, richer fixture; the
  multiclass and spellcasting cases in §5 want the same one.
- **Recovering or writing the actual schema report is outside this record**, but PSD-0001 cites it as
  authority for decisions this series has now built on, and someone should know that the citation
  currently resolves to the wrong document.

### 7. Round-trip comparison is semantic JSON equality, not bytes

ARD 0003 §5 says the corpus compares "byte-for-byte where the schema permits", and that qualifier hides a
problem: byte comparison is unattainable here. Go's `encoding/json` sorts map keys on output, 31 of the
generated declarations are open maps (ARD 0003 v0.2's corrected count), and ARD 0003's whole design
re-encodes from a decoded tree — so key order and whitespace differ from the input by construction, on
every document, always.

The comparison is therefore **semantic JSON equality** — decode both sides to `interface{}` and compare —
which happens to be exactly the right sensitivity:

| difference | byte compare | semantic compare |
| --- | --- | --- |
| key order, whitespace | fails (noise) | passes (correct) |
| a dropped nested key (ARD 0003 finding 1) | fails | **fails** |
| an injected default (ARD 0003 finding 2) | fails | **fails** |
| `1.0` becoming `1` | fails | fails, if numbers are compared as decoded |

The last row is why the decode uses `UseNumber` on both sides: comparing through `float64` would let a
number change shape unnoticed, and `internal/jsonmerge` already made this choice for the same reason.

So `assert.JSONEq`-style comparison is the tool, and the two losses ARD 0003 measured are both things it
catches. Byte-level fidelity is not a property this design has, and the corpus should not pretend to test
for one.

One consequence of ARD 0011 lands here rather than in §5's table. The unmodelled-key fixture is compared
the same way, and semantic equality is what makes it meaningful: the key's *value* has to come back
unchanged, not merely be present, and `UseNumber` is what stops a preserved number from changing shape on
the way through a merge it was never decoded by.

### 8. Two CI gates

`go generate` cleanliness — regenerate, `git diff --exit-code` — so a schema edit cannot land without
regenerated types (ARD 0003). And `dmosh character validate --all` against a seeded fixture database.

The second gate needs one caveat, because ARD 0007 makes it easy to misread: a store the TUI has been
editing may legitimately contain invalid documents, so this gate is meaningful **only** against a
database the test seeds itself. Pointed at a real store it would report the user's mid-edit sheet as a
build failure. The seeding belongs in the test, not in a checked-in `.db` file.

## Consequences

- **The round-trip corpus is only as good as its fixtures**, and it is now the sole defence of a
  property ARD 0001 held structurally — ARD 0011 withdrew the other one. A schema section with no
  fixture has no protection.
- **One fixture in §5 is expected to fail on the day it is written**, and that is the point of writing
  it: preservation of unmodelled keys is an obligation ARD 0011 §2 hands the save path, not a property
  the stack has. A red test is the honest record of that until the merge lands.
- **`teatest` will break at some point**, being experimental, and the accepted answer is to fix the test
  files rather than to have avoided the dependency.
- **Goldens are review artifacts**, so they have to stay small enough that a human reads the diff. That
  constrains layer 2 more than layer 1.
- **Layer 3 cannot be written until someone authors its fixture** (§6), and that is now the first task in
  Phase 1's testing work rather than an assumed input.
- **A citation in both PSDs resolves to the wrong document**, which this record can note but not fix.
- **Every record in this series ended up owing this one a test**, which is the honest cost of a design
  whose guarantees are mostly behavioural.
