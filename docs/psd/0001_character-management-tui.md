---
id: PSD-0001
title: Character Management TUI
updated: 2026-08-19
status: 
    - kind: draft
    - version: v0.4.8
component: 
    - type: CLI
    - name: dmosh
    - subsystem: character
audience:
  - implementers[GO]
  - implementers[TUI]
  - contributors
related:
  - docs/jsonschema/character/v1alpha/classes.schema.json
  - docs/jsonschema/character/v1alpha/spellcasting.schema.json
  - docs/jsonschema/character/v1alpha/vitals.schema.json
  - docs/psd/share/2024-character-schema-report.md
  - docs/psd/share/gojsonschema-codegen-spike/README.md
---

# Product Specification: Character Management TUI

## 0. Revision history and resolved decisions

**v0.1** — initial spec. Assumed one character per JSON file on disk.

**v0.2** — storage moved to a SQLite-backed document store. Every decision downstream of "how is a character persisted" revised.

**v0.3** — first ambiguity pass. Eight cross-cutting decisions made explicit (D1–D8).

**v0.4** — second ambiguity pass. The nine remaining open questions resolved (D9–D17); §13 is now down to four genuinely-open items, all of them deliberately deferred rather than unexamined.

**v0.4.1** — the binary is named **`dmosh`** (was `dnd`). This also renames the environment variable to `DMOSH_DB` and the default data directory to `$XDG_DATA_HOME/dmosh/`.

**v0.4.2** — `DMOSH_CHARACTER` makes the target character resolvable from the environment (D18), so an `.envrc` per player folder can drive the whole CLI argument-free.

**v0.4.3** — the Phase 3 History screen is specified (D19), closing the last of §13's four open questions from v0.4. Three remain, all deferred by choice.

**v0.4.4** — item instance loading and revisioning decided (D20), superseding D15's deferral.

**v0.4.8** — **D9 amended (D22).** §6's struct-generation block is rewritten against the generator's actual behaviour: the hand-written exceptions attach through its own `goJSONSchema` schema keyword rather than a parallel `types_manual.go`, and only one of the two named constructs needs a hand-written type at all. Extended in place with a third silently-dropped construct — **`additionalProperties` is not enforced by the generated code**, which affects every schema we ship, since all of them close their objects with `false`. Folded into this revision rather than numbered separately: it is the same decision and the same review cycle, in the manner of v0.4.5. No change to the document format.

**v0.4.7** — adds §6.1, the `play_log` schema change, as **Phase 0 pre-work**. This is the first revision to modify `character.schema.json`; see the amended note below.

**v0.4.6** — adds §12, *What v1 is built to find out*, following the GM session tool spec's §11 convention: a local `usage_counters` table, a `dmosh character feedback` command, and eight user-facing questions this version exists to answer. Open questions move to §13.

**v0.4.5** — **D20 amended:** item instances now share the character's `doc_revision` rather than carrying their own. Lazy loading is unchanged. Amended in place rather than issued a new number, since it is the same decision reversing one of its two halves within one review cycle; the reversal and its consequences are recorded in *Corrections and inferences* below. This closes §13's item-snapshot question outright — restore is now complete by construction.

### v0.3 decisions

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D1 | Content catalog | **Deferred to Phase 2.** Phase 1 is manual entry throughout. Scope narrowed by D13. | §7.2, §7.11, §11 |
| D2 | Rules-engine depth | **Pure arithmetic, no rules tables.** PB, slot maxima, prepared maxima, carrying capacity, and AC breakdown terms are user-entered. | §8.1 |
| D3 | Level-up flow | **Dedicated level-up wizard, Phase 2** (§7.12). | §7.5, §7.12 |
| D4 | Item-instance storage | **Sibling `item_instances` table** keyed by `owner_character_id`. | §9.1, §9.3 |
| D5 | 2014 ruleset support | **2024 only.** 2014 documents open read-only with a banner. | §6, §7.11 |
| D6 | Autosave vs. validation | **Autosave is unvalidated.** Validation gates explicit save, navigation-away, and export. | §6, §8.5 |
| D7 | Recompute write-back | **Read commands never write.** Only the TUI and explicit `recompute` persist derived values. | §4, §8.1 |
| D8 | Character deletion | **Soft delete** with a recovery path. Command renamed by D17. | §4, §8.4, §9.1 |

### v0.4 decisions

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D9 | Struct codegen | **`atombender/go-jsonschema` plus hand-written exceptions** for the two patterns it can't express. **Amended by D22** — the attachment mechanism is the generator's own `goJSONSchema` keyword, and only one of the two patterns needs a hand-written type. | §6 |
| D10 | Default database location | **`--db` flag > `DMOSH_DB` env var > XDG data dir.** No current-directory search. | §4, §9 |
| D11 | SQLite journal mode | **WAL, with `busy_timeout` and `foreign_keys` pragmas.** Network filesystems documented as unsupported. | §9.2 |
| D12 | Terminal width | **80-column hard floor, named breakpoints at 80/120/160.** | §2, §7 |
| D13 | Catalog scope | **Spells + equipment + class tables as read-only reference.** Species/backgrounds/feats stay free-text. | §7.2, §7.8, §7.9, §7.12, §11 |
| D14 | Homebrew authoring | **Local reusable catalog.** Custom entries persist to the catalog tables; no sharing mechanism yet. | §7.2, §9.1, §11 |
| D15 | Item loading/revisioning | Deferred at the time — **superseded by D20.** | §9.4 |
| D16 | Phase 2 split | **2a (screens + level-up, manual entry) then 2b (catalog + pickers).** | §11 |
| D17 | Undo | **Both:** an in-session undo stack and a persisted snapshot table with time-travel restore. | §8.6, §9.1 |

### v0.4.2 decision

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D18 | Character target resolution | **`<name-or-id>` argument > `DMOSH_CHARACTER` env var > the single live character.** Lifecycle commands (`delete`, `undelete`, `purge`, `restore`) are exempt and always require an explicit target. | §4, §10 |

### v0.4.3 decision

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D19 | History screen | **One screen, two modes (`build_log` decisions / snapshot revisions), rendered as log lines.** Structure — diffing, filtering, grouping — deliberately deferred until there is a real log to design against. | §7.13, §8.4, §11 |

### v0.4.4 decision

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D20 | Item loading and revisioning | **Lazy load** on first detail-pane selection; **shared `doc_revision`** with the owning character, so character-plus-items is one aggregate and snapshot restore is atomic. Eager loading deferred until something demands it. Supersedes D15; revisioning half amended in v0.4.5. | §5.3, §7.9, §8.6, §9.1, §9.3, §9.4, §10 |

### v0.4.7 decision

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D21 | Play log (schema prerequisite) | **Add `play_log` to `character.schema.json` as Phase 0 pre-work**, for PSD-0002's patch idempotency. Read/validated/round-tripped in v1; written by nothing. Exempt from `restore`. | §6.1, §10, §11 |

### v0.4.8 decision

| # | Decision | Resolution | Sections |
|---|---|---|---|
| D22 | Codegen extension mechanism | **Hand-written types attach via the generator's `goJSONSchema` schema keyword**, not a separate `types_manual.go`. `slotPool.slots` gets a substituted type; `conditionInstance` gets no hand-written type, because the compiled schema already enforces the rule the generator drops. Operating rule: **assume the generated type enforces only required fields, enums, and patterns** — `additionalProperties` is dropped too, on every schema we ship. Amends D9. | §6 |

### Corrections and inferences

Seven of these decisions had consequences that required changes beyond the sections they nominally touch. Recording them explicitly because each is a place where a reader of an earlier draft would otherwise be misled:

1. **D13 narrows D1.** v0.3's §7.2 promised that species and background become catalog-backed pickers in Phase 2. Under D13 they do not — the catalog covers spells, equipment, and class-table reference text only. Species, background, origin feats, and class features remain free-text entry indefinitely. Whether they ever get catalog coverage is now an open question (§13.1).
2. **D17 forces a command rename.** D8 introduced `dmosh character restore` for undeleting; D17's snapshot feature wants `restore --at <point>` for time-travel. These are different operations and shouldn't share a verb. Undelete is now `dmosh character undelete`; `restore` means snapshot restore.
3. **D8 + D11 expose a gap.** Soft delete alone means the database grows forever, and the `ON DELETE CASCADE` on `item_instances` never fires because soft deletion is an `UPDATE`. A `dmosh character purge` command is therefore required to complete D8 — it is the only path that issues a real `DELETE`, and the only one that cascades. This was inferred rather than asked; flag it if you disagree.
4. **D18 carves out the lifecycle commands, and surfaces a latent gap.** The request was to make the target argument optional; exempting `delete`/`undelete`/`purge`/`restore` from that is an added safety judgement, not something asked for — flag it if you'd rather the variable applied uniformly. Separately, D18 forced a rule the spec had never stated: nothing previously said what `<name-or-id>` does when two live characters share a name. Resolving by name is now potentially invisible to the user, so ambiguity is an explicit error (§4).
5. **D20's revisioning half was reversed, and that exposed an `export` gap.** v0.4.4 specified independent per-item revisions; v0.4.5 reverses that to a shared revision so restore is a clean transition. Two knock-ons were not asked about. First, snapshots must now capture the item set, which meant an `items` column on `character_revisions` and an aggregation done in SQL (`json_group_array`) so that capturing every item does not force the editor to abandon lazy loading — that combination is the subtlest thing in this decision and §9.4 shows the statement. Second, treating character-plus-items as one aggregate makes it indefensible for `export --format json` to emit only the character document: §9.3 claims export is the backup-and-handoff story, and an export that silently dropped every magic item would make that claim false. Export is now a `{character, items[]}` envelope. The `doc_revision` column added to `item_instances` in v0.4.4 has been removed again.
6. **D21 carries two judgements PSD-0002 did not make.** That spec names "the character log" and its entry shape but not the field name or its interaction with anything here. I chose `play_log` over `character_log` for the opposition with `build_log` (§6.1), and — more consequentially — **exempted it from `restore`**. Without that exemption, rewinding to a revision predating a patch would erase the record of applying it and let the next `patch apply` re-award the same XP, which is precisely the failure PSD-0002's idempotency design exists to prevent. It is the only field with such an exemption, and it makes D20's "restore is a clean transition" true of sheet state rather than of literally every byte. Both are flaggable.

7. **D22 removes work D9 asked for, and that is the point.** The request behind it was "how do the hand-written exceptions attach"; the answer changed what they are. Two things were not asked about. First, `conditionInstance` loses its hand-written type entirely — v0.4.7 already conceded the compiled schema was "the authority... a convenience check, not a second source of truth", which is an argument for not writing the type at all, and D22 follows it through. Anything kept there is now a method on the generated type (Pattern B) with a test pinning it to the schema. Second, and more consequentially, putting the seam in the schema means **`character.schema.json` and its siblings now carry a Go-specific keyword**. It is inert — JSON Schema ignores unknown keywords, and the files still compile and validate unchanged — but a language-neutral contract document is no longer strictly language-neutral. That is a real cost and the strongest argument for the approach D22 replaces; flag it if you'd rather pay the sync burden of a parallel file to keep the schemas clean. Both are flaggable.

Through v0.4.6 the document format itself was unchanged across every revision. **v0.4.7 ends that**, additively and deliberately: `play_log` (§6.1) is the one field this spec adds, it is required by PSD-0002 rather than by anything here, and it is scheduled as Phase 0 precisely so the change happens once, before any code depends on the old shape. `character.schema.json` remains the source of truth for what a character *is*.

---

## 1. Purpose and scope

This is the first interface shipped for the Dungeon Master Open Source Toolkit: a terminal UI, invoked from a Cobra-based CLI, that lets a single player create, view, and edit one D&D 2024 character to full paper-character-sheet fidelity. It reads and writes documents conforming to `character.schema.json`. It has no network layer of its own; storage is a local SQLite database. The schema already carries the fields a future sync/server tool will need (`owner_user_id`, `campaign_id`, `doc_revision`), and this spec treats those as inert metadata — present in the database, not exposed as an editable screen — so this tool does not have to be rewritten when the server arrives.

**In scope for v1:** everything a player would track on a physical 2024 character sheet plus its scratch space — identity, ability scores and saves, skills, proficiencies and languages, combat stats (AC, initiative, speed, HP, hit dice, death saves), conditions, feats and features, class/subclass and weapon mastery, resources (class pools like Rage or Ki), rest state, spellcasting (all four block types from the schema report: class-prepared, always-prepared, granted/innate, spellbook), inventory and equipped state, currency, and freeform notes/backstory.

**Explicitly out of scope for v1:** reputation/renown tracking (`reputation` block — a DMG optional toolbox rule, not a paper-sheet field, and party/DM-facing more than player-facing), the party treasury reference, and any multi-character or multi-user view. The `build_log` array is populated automatically (§8.3) but not browsable until Phase 3.

**Explicitly out of scope for the whole toolkit, this phase:** the DM's local server and the sync protocol. This tool produces character data — in the local SQLite store, and via `export` in standalone files — that will be syncable later; it does not sync any of it now.

**A note on what this tool is, given D2 and D13.** This is deliberately *a validating character sheet, not a character builder*. It knows the shape of a 2024 character and does the arithmetic reliably; it does not know that Wood Elves have 35 ft speed or that a level 5 Ranger has three 1st-level slots. Phase 2b's catalog reduces typing for spells and equipment and puts class tables on screen as reference, but the tool never computes from them. Implementers should resist the pull toward "just hard-code this one table" — the boundary in §8.1 is the contract, and §10 describes the test that enforces it.

## 2. Non-functional constraints

The tool is a single static Go binary. No network calls, no telemetry.

**Terminal size (D12).** 80×24 is a hard floor: the tool must be fully usable there, and may refuse to launch below it with a clear message. Above the floor, layout uses three named tiers rather than ad-hoc width arithmetic scattered through screens (§7). It must work over SSH and inside tmux/screen, which rules out anything depending on Kitty/iTerm graphics protocols or true-color-only rendering — Lip Gloss's adaptive color profile detection (`termenv`) handles this and should not be overridden.

**Startup.** Under 100ms to first paint against a database holding a realistic character count (tens to low hundreds of rows). This rules out re-parsing every row's document on launch to populate a picker; see §9.1's generated columns.

**Durability.** Character data no longer sits on disk as a directly-editable JSON file. §9.3 covers how the "survives the tool being wrong about something" property is preserved — now materially strengthened by D17's snapshots.

## 3. Technology stack

| Layer | Library | Role |
|---|---|---|
| CLI framework | `spf13/cobra` | Command parsing, subcommands, help generation, shell completion |
| TUI runtime | `charmbracelet/bubbletea` | Elm-architecture event loop driving the interactive session |
| Component kit | `charmbracelet/bubbles` | `list`, `table`, `textinput`, `textarea`, `viewport`, `paginator`, `help`, `key`, `spinner` |
| Structured forms | `charmbracelet/huh` | The creation and level-up wizards, and any screen that is "fill in N fields and confirm" |
| Styling | `charmbracelet/lipgloss` | Layout, breakpoint tiers, theming; also the non-interactive `show` render |
| Markdown rendering | `charmbracelet/glamour` | Backstory/notes panes and `export --format markdown` |
| Logging | `charmbracelet/log` | Structured, leveled logging to a file (never stdout while the TUI owns the screen) |
| Schema validation | `santhosh-tekuri/jsonschema/v5` | Validates documents against the actual `character.schema.json` (§6) |
| Struct generation | `atombender/go-jsonschema` | Build-time codegen of Go types from the schema (D9, D22, §6). Module path is `github.com/atombender/...`; `omissis/go-jsonschema` is the former name |
| Persistence | `database/sql` + `modernc.org/sqlite` | Embedded store (§9). Pure-Go and cgo-free; `mattn/go-sqlite3` was rejected because its cgo dependency breaks the single-static-binary requirement |
| Demo/docs tooling | `charmbracelet/vhs` | Not a runtime dependency; generates terminal-recording GIFs for the README |

One deliberate non-choice: no hand-maintained Go struct tree that drifts from the schema (§6). A related one from v0.1 is retired — that draft argued against a database on the grounds that "a character is one JSON file, matching the aggregate-root framing." The framing doesn't require a filesystem; §9 explains how a SQLite row plays the same role with the same one-character-one-blob shape, while adding transactional writes, indexed listing, and snapshot history the file design didn't have.

## 4. Process and command structure

The binary is `dmosh`. This spec covers `dmosh character ...`; other toolkit features add their own subcommand trees under the same root.

```
# Interactive
dmosh character new [--name NAME] [--template FILE]             # creation wizard (§7.11)
dmosh character open [<name-or-id>]                             # full TUI editor

# Non-interactive
dmosh character list [--include-deleted]
dmosh character show [<name-or-id>] [--format text|markdown|json]
dmosh character validate [<name-or-id>]|--all                   # exit codes below
dmosh character recompute [<name-or-id>]|--all                  # only read-path command that writes (D7)
dmosh character history [<name-or-id>]                          # list saved snapshots
dmosh character import <file> [--format json]
dmosh character export [<name-or-id>] --format markdown|json --out <file>
dmosh character feedback --out <file>                           # local usage summary (§12)

# Lifecycle (D8, D17) — these ALWAYS require an explicit target (D18)
dmosh character delete <name-or-id>                             # soft delete
dmosh character undelete <name-or-id>                           # reverse a soft delete
dmosh character purge <name-or-id> --force                      # permanent DELETE; cascades to items
dmosh character restore <name-or-id> --at <timestamp|revision>  # restore a snapshot
```

**Database resolution (D10),** in precedence order: the `--db PATH` flag, then the `DMOSH_DB` environment variable, then `$XDG_DATA_HOME/dmosh/characters.db` (falling back to `~/.local/share/dmosh/characters.db` when `XDG_DATA_HOME` is unset). **There is no current-directory search.** v0.2 resolved `./dmosh.db` first, which meant the same command showed different characters depending on where it was run; that footgun is now removed. Per-campaign stores are still easy — a `.envrc` setting `DMOSH_DB` per campaign folder gives the old ergonomics without the ambiguity, and the resolved path is printed in `list`'s header and the TUI's status line so the active store is never a mystery.

**Character resolution (D18),** in precedence order: the positional `<name-or-id>` argument, then the `DMOSH_CHARACTER` environment variable, then — only when the store holds exactly one live character — that character. If none of the three resolves, the command errors with the available names rather than guessing; `open` and the bare `dmosh character` invocation instead fall through to the interactive picker, since they have a UI to fall back into.

This mirrors D10 deliberately, and the two compose: an `.envrc` in a player's folder that sets both `DMOSH_DB` and `DMOSH_CHARACTER` makes every command in that directory operate on that player's sheet with no arguments at all — `dmosh character open`, `dmosh character show`, `dmosh character validate`. That is the intended ergonomic.

Three rules make the indirection safe:

- **Lifecycle commands are exempt.** `delete`, `undelete`, `purge`, and `restore` always require an explicit `<name-or-id>` and ignore `DMOSH_CHARACTER` entirely. An environment variable you may have forgotten is set must never be able to destroy or rewrite a sheet you weren't looking at. (`history` is read-only and so honours the variable; it moved out of the lifecycle group in the listing above for that reason.)
- **Ambiguous names are an error, never a guess.** A `<name-or-id>` is matched first against `characters.id`, then against `character_name`. If a name matches more than one live character the command exits non-zero listing the candidate ids, because resolving by name is now something the user may not be able to see happening.
- **Soft-deleted targets fail loudly.** If the resolved character is soft-deleted, the command errors and names `undelete` rather than silently falling through to the single-live-character rule.

Every command that resolved its target from the environment rather than an argument says so on stderr (`using DMOSH_CHARACTER=Vesk Ambermarch`), and the TUI's status line shows the resolved character alongside the resolved database path, so ambient configuration is always visible.

**Interactive vs. scriptable.** `new` and `open` take over the terminal. Everything else prints to stdout and exits, so the tool can be driven from a Makefile, a pre-commit hook, or CI.

**`validate` exit codes** are part of the contract:

| Code | Meaning |
|---|---|
| 0 | Schema-valid, and stored derived fields match a fresh `Recompute` |
| 1 | Schema-invalid — JSON Pointer paths printed to stderr |
| 2 | Schema-valid but derived fields have drifted — diff printed; `recompute` fixes it |

`import` is the explicit, validated on-ramp for an externally-produced document (a hand-edited fixture, another tool's export, a file recovered from a v0.1-era install). `--template FILE` on `new` takes the same format, used to prefill the wizard — which is how a table shares a house-standard starting sheet.

Running `dmosh character` with no subcommand resolves the database, then applies D18's character resolution: it opens `DMOSH_CHARACTER` if set, otherwise the single live character if there is exactly one, otherwise the `list` screen as an interactive picker.

## 5. Application architecture

### 5.1 Model/Update/View

```go
type model struct {
    doc        *character.Document // in-memory decoded schema document
    docID      string              // matches document._id / the characters.id row key
    dirty      bool
    revision   int  // doc_revision as of last successful save; the UPDATE predicate (§5.3)
    readOnly   bool // failed validation on open, or ruleset != 2024 (D5)
    undo       *undoStack // in-session undo/redo (§8.6)
    screen     screenID
    nav        navStack   // back-stack of prior screens
    active     tea.Model  // focused sub-model (a screen or a huh.Form)
    width, height int
    tier       layoutTier // Compact | Comfortable | Wide (§7)
    styles     theme
    err        error      // last non-fatal error, surfaced as a status-line banner
}
```

Each screen (§7) is its own `tea.Model`, composed into the parent the way Bubble Tea's own multi-component examples do. The parent owns navigation and the document; child screens receive a read view of the relevant slice and emit typed `tea.Msg` values (`abilityChangedMsg`, `skillToggledMsg`, `inventoryEntryMovedMsg`, …) rather than mutating the document directly. The parent applies the mutation, pushes an undo frame, runs `Recompute` (§8.1), marks `dirty`, and re-renders. Every mutation therefore passes through one place, which matters because derived fields chain (ability modifiers feed skill bonuses feed passive scores).

### 5.2 Navigation model

Navigation is a stack, not a single enum, so `Esc` always returns to the prior screen rather than a hard-coded parent, and deep entry points need no special-cased return paths. The root is always the Sheet Overview (§7.1).

### 5.3 Data layer

`internal/character` holds Go types mirroring `character.schema.json` 1:1 — same field names in `snake_case` JSON tags, same optionality (a field typed `["string","null"]` is a pointer or nullable wrapper, never a bare string defaulting to `""`, because the schema report is explicit that missing vs. empty vs. zero are meaningfully different states). Generation is covered in §6.

`internal/store` wraps `database/sql` and owns all persistence; no other package issues SQL. Loading is `SELECT document FROM characters WHERE id = ? AND deleted_at IS NULL` then decode-then-validate-then-hydrate. Saving is dehydrate-then-write: `UPDATE characters SET document = ?, doc_revision = doc_revision + 1, updated_at = ? WHERE id = ? AND doc_revision = ?` inside a transaction that also inserts a snapshot row (§8.6) when the save is a validated one. The trailing `doc_revision` predicate is an optimistic-concurrency check (§8.5); zero rows affected is surfaced, never silently overwritten. Item-instance writes join that same transaction and are guarded by that same predicate rather than carrying one of their own (D20, §9.4), so the character row is the single conflict domain for the whole aggregate.

## 6. Schema fidelity and validation

`character.schema.json` is the source of truth for the storage format and the TUI must not fork it.

**Vendoring.** The schema is vendored at `internal/character/schema/character.schema.json`, kept byte-identical to the project doc by a `go generate` check, and compiled once at startup via `jsonschema.Compile`.

**Struct generation (D9, amended by D22).** `atombender/go-jsonschema` generates the bulk of the type tree at build time via `go generate`, so a schema change that removes or renames a field becomes a compile error rather than silent data loss. The module path is `github.com/atombender/go-jsonschema`; earlier drafts of this spec called it `omissis/go-jsonschema` after the original author, and that name still resolves in prose but not in a `go.mod`.

**How the hand-written exceptions attach.** Not as a parallel `types_manual.go` kept in sync by convention — the generator has a hook for this. It reads a **`goJSONSchema`** keyword from any subschema (undocumented in its README; declared at `pkg/schemas/model.go:208`, consumed at four points in `pkg/generator/schema_generator.go`):

| field | valid position | effect |
|---|---|---|
| `type` | type | emit this Go type name and generate nothing else for the construct |
| `imports` | type or property | add imports to the generated file |
| `nillable`, `pointer` | type | control nil-ability of the substituted type |
| `identifier` | property | override the generated Go field name |
| `extraTags` | property | add struct tags |

So the seam lives **in the schema**, which is already the source of truth, rather than in a Go-side convention about which declarations are safe to hand-edit. The generator names your type; you write it in a normal file in the same package. Three patterns, in increasing order of cost:

- **A — substitute a type.** For a construct the generator *cannot represent*. Put `"goJSONSchema": {"type": "SlotsByLevel"}` on the subschema and hand-write `SlotsByLevel` beside the generated file.
- **B — a method on a generated type.** No extension and no substitute type: Go permits methods on generated types from another file in the same package. For rules where the generated *shape* is already correct.
- **C — `identifier` / `extraTags`** on a property, for a field rename or an extra tag, with no hand-written code at all.

The load-bearing property of A is that the generated `UnmarshalJSON` still runs its own required-field checks and then delegates via `json.Unmarshal(value, &plain)`, so `encoding/json` dispatches into the hand-written `UnmarshalJSON`. **The hand-written rule runs on the generated decode path**, with neither file importing the other, and substituting a type costs none of the generated validation.

**The two constructs are not the same problem.** Drafts through v0.4.7 listed them together as "the two patterns it can't express" and sent both to one file. They need different treatment:

- **`slotPool.slots` is a *typing* gap.** It uses `patternProperties` (`^[1-9]$`) with `additionalProperties: false`. Unaided the generator emits `type SlotPoolSlots map[string]interface{}` — every read needs a type assertion and every key is accepted, including `"0"`, `"10"`, and `"pact"`. The generated type cannot carry the data, so only Go can fix it. **Pattern A**, as a `map[int]SlotCount` whose `UnmarshalJSON` enforces the 1–9 key range: the map key is an `int`, so a caller cannot spell a key wrong, and the decoder refuses out-of-range and non-numeric keys.
- **`conditionInstance` is a *validation* gap, and needs no hand-written type.** It uses `allOf`/`if`/`then`/`else` to require `level` for Exhaustion and forbid it otherwise. The generator drops the conditional *silently* — it emits a perfectly plausible `Level *int` and an `UnmarshalJSON` that checks `instance_id` and `name` and never mentions the rule — but the generated shape is correct. `santhosh-tekuri/jsonschema` compiled from the same file already enforces the rule exactly, and is the authority at load and save time. A hand-written type here would be a **restatement of a rule already enforced**, not a fix. If a `Validate()` method is wanted for callers holding an already-decoded value mid-edit, it is **Pattern B** — a method on the generated type, explicitly a convenience, and pinned against the compiled schema by a test that compares the two verdicts case by case so they cannot drift.

This retires the name `types_manual.go`: grouping by "generator limitation" puts a real typing fix and a redundant validator in one file. One hand-written file per concept, named for the concept, beside the generated file.

**`additionalProperties` is a third instance of the same class, and it is the one that touches every schema we ship.** Every file under `docs/jsonschema/character/v1alpha` that closes an object does so with `additionalProperties: false` — the character root, `abilities`, and `slotPool.slots`. **The generated code does not enforce any of them.** Unknown keys are dropped in silence, with no error, exactly the way `conditionInstance`'s conditional is dropped; only the compiled schema rejects them. Two consequences:

- The generated type's value as a *validator* is narrower than it looks. It enforces required fields, enums, and patterns — not object closure. Any statement of what validation the decode path buys us must say so, or a reader will assume a closed object is checked when it is not.
- **`additionalProperties: true` on an object that also declares `properties` needs a hand-written decode/encode pair.** It is the natural way to say "these known fields, plus anything else", and out of the box the generator loses data on it: it emits an `AdditionalProperties interface{}` field, generates no `UnmarshalJSON` at all for that type, and — the field having no `json:` tag — injects a bogus `"AdditionalProperties": null` key into everything it encodes. A typed `additionalProperties` decodes correctly but has no matching `MarshalJSON`, so encoding nests what decoding inlined and a second pass discards the extras entirely. Only a *free-form* object with no `properties` is handled correctly, becoming a plain `map[string]T`.

  This is a cost, not a prohibition — three routes are verified, in increasing order of what they cost you. **The default is Pattern B with no schema change**: hand-write `UnmarshalJSON` and `MarshalJSON` on the generated type, which works *precisely because* the generator emits neither for this case, so the method slots are free and the declared fields stay typed. **Pattern A with a deliberately map-shaped substitute** puts the seam in the schema instead, at the price of typed access to declared fields. And for a *typed* `additionalProperties` the job is half as big: the generated decoder is already correct, so supply `MarshalJSON` alone. Changing the spelling to `additionalProperties: {}` is **not** a route — it is the same schema as `true` and generates the identical dead field.

**Evidence.** All of the above is demonstrated by a runnable spike at `docs/psd/share/gojsonschema-codegen-spike/` — 15 test functions across two packages, generating the character types twice from the same schemas so that the effect of the extension is a test result rather than an assertion, and pinning all six `additionalProperties` variants against both the generated Go and the compiled schema. Its control is generated from `docs/jsonschema/character/v1alpha` directly, so it cannot drift from what the repo ships. If this section and that directory ever disagree, re-run its tests; they are the record.

**Constraints on using the extension.** Each of these was established empirically against v0.24.1 and would otherwise cost an implementer an afternoon:

- **It is ignored at `$ref` sites.** `generateTypeInline` guards the extension block with `t.Enum == nil && t.Ref == ""`. Put `goJSONSchema` on the `$defs` entry itself, never on the property that `$ref`s it.
- **Where you put it changes what you get, and the two constraints pull against each other.** On a *property* the substitution is direct — `Slots SlotsByLevel`, as with `slotPool.slots`. On a *`$defs` entry* the generator emits `type OpenViaExtension OpenBag`: a **defined type**, and a defined type in Go does not inherit the methods of the type it is defined from. A substitute that is a struct carrying custom `MarshalJSON`/`UnmarshalJSON` therefore arrives at the use site **without them**, silently, and the round-trip breaks with nothing to flag it. Since the previous constraint forces a shared `$defs` object to carry the extension at the definition — exactly the position that degrades — a substitute used that way must either be **map-shaped** (`encoding/json` handles maps natively, so there are no methods to lose) or have its methods hung on the generated defined type instead.
- **`--extra-imports` is unrelated to the `imports` field.** Despite its config comment, the flag only appends the YAML formatter (`pkg/generator/generate.go:57`). `imports` works without it.
- **The keyword is inert to the validator.** JSON Schema ignores unknown keywords, so a schema file carrying `goJSONSchema` still compiles under draft 2020-12 and still validates documents unchanged. The real cost is that a Go-specific hint now lives in a language-neutral contract file — the one genuine argument for the parallel-types approach this amends.
- **Regeneration is non-destructive.** `-o` fixes exactly one output path, so the `.gen.go` is the only file rewritten and the hand-written files beside it are untouched by design rather than by luck.

**CI.** A job asserts the generated output is current (regenerate, `git diff --exit-code`) so a schema edit can't land without regenerated types. Note what that does *not* catch: none of the three constructs above produced a warning or a non-zero exit when the generator gave up on it. `git diff --exit-code` catches **drift**; nothing catches **silent degradation**. Only a test that runs the same cases past both the hand-written Go and the compiled schema, and asserts the verdicts agree, closes that gap — and it is required for every construct handled by Pattern A or B.

Three independent silent drops — `patternProperties`, `if`/`then`/`else`, and `additionalProperties` — is enough to stop treating each as a surprise. **Assume the generated type enforces only required fields, enums, and patterns, and treat anything else as unenforced until a test proves otherwise.** That is the operating rule; the spike is where new cases get checked before they are relied on.

**Where validation runs (D6):** on `open`, on `import`, on wizard completion, on explicit save (`Ctrl+S`), on navigation away from the editor, and on `export`. It does **not** run on debounced autosave — mid-edit documents are routinely incomplete, and blocking autosave on them would lose work exactly when the user is typing most.

**Failure handling.** A validation failure on *open* shows a non-fatal banner with the failing JSON Pointer paths and opens the row read-only until acknowledged. A failure on an *explicit save* blocks that write and surfaces the same detail; the document stays in memory to be fixed.

**Ruleset gate (D5).** A document whose `ruleset.revision` is not `"2024"` opens read-only with an explanatory banner. It can still be viewed, exported, and validated. The creation wizard always writes `"2024"`.

**Version drift.** `schema_version` is checked on load. A mismatch does not block opening (per the schema report's stance against silent mutation of live sheets) but produces the same kind of banner, naming the delta.

### 6.1 Prerequisite: the play log — a change to `character.schema.json`

Everything else in this spec is implementable against the schema as it stands. This is the exception, and it is **pre-work: land it before Phase 1 begins**, not during.

**Why it exists.** The GM session tool (PSD-0002) proposes changes to a character as *patch packets*, and its safety model rests on idempotency — applying the same patch twice must not award the same 450 XP twice. That requires the character document to record which patch ops it has already applied. PSD-0002 §6.3 is explicit that there is no separate `applied_patches` array: the log is the only record, so the two cannot disagree. It names this "the character log" and assigns ownership of the change here, as a Phase 1 dependency of that tool.

**Why it must land first.** D9 makes the Go type tree a build artifact of the schema. A root-level field added after Phase 1 means regenerating types, re-running every screen's model against them, and revisiting the round-trip corpus — for a field that could have been there from the first commit at no cost. The schema also declares `additionalProperties: false` at the root, so this cannot be a field the tool tolerates in the meantime and formalises later; until the schema knows about it, a document carrying it is *invalid*, and `import` would reject the very documents the GM tool produces.

**Naming.** The field is `play_log`, sitting alongside the existing `build_log`. PSD-0002 refers to "the character log" in prose but never names the field, so this is a free choice: `character_log` inside a character document is redundant, whereas `build_log` / `play_log` reads as the deliberate opposition it is — decisions made *building* the character, versus things that happened *to* it in play. Flag it if you would rather match the other spec's prose.

**The change.** One new root property and one `$defs` entry:

```json
"play_log": { "type": "array", "items": { "$ref": "#/$defs/playLogEntry" } }
```

```json
"playLogEntry": {
  "type": "object",
  "required": ["entry_id", "seq", "at", "kind"],
  "properties": {
    "entry_id":            { "type": "string" },
    "seq":                 { "type": "integer", "minimum": 0 },
    "at":                  { "type": "string", "format": "date-time" },
    "kind":                { "enum": ["patch_applied", "award", "note"] },
    "campaign_id":         { "type": ["string", "null"] },
    "session_number":      { "type": ["integer", "null"] },
    "source_tool_version": { "type": ["string", "null"] },
    "patch": {
      "type": ["object", "null"],
      "properties": {
        "patch_id": { "type": "string" },
        "resolutions": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["op_id", "kind", "status"],
            "properties": {
              "op_id":  { "type": "string" },
              "kind":   { "type": "string" },
              "status": { "enum": ["applied", "skipped", "unresolvable"] },
              "at":     { "type": "string", "format": "date-time" },
              "detail": { "type": ["string", "null"] }
            }
          }
        }
      }
    },
    "text":    { "type": ["string", "null"] },
    "payload": { "type": ["object", "null"] }
  }
}
```

Two details in that fragment are load-bearing rather than incidental:

- **`resolutions[].kind` is a bare string, not an enum.** PSD-0002 §10.7 makes no stability promise about its op vocabulary and requires receivers to skip unknown kinds with a warning rather than fail. An enum here would make recording *that* skip impossible — the document could not describe the op it had just declined — which would defeat the re-offer mechanism the whole idempotency design depends on.
- **`kind` at entry level *is* an enum,** because these are the character tool's own entry types, not a foreign vocabulary. `patch_applied` is the only one PSD-0002 needs. `award` and `note` are reserved for the native uses that spec anticipates — quest rewards, XP, permanent buffs recorded by the player directly — and nothing in v1 writes them.

**Version impact.** This is additive and `play_log` is optional, so every existing document stays valid. `schema_version` goes `1.0.0` → `1.1.0`; §6's mismatch banner will fire once for documents written before the bump, which is the intended behaviour, not a defect.

**What v1 actually does with it: reads, never writes.** No screen in this spec appends a play-log entry. The tool must generate types for it, validate it, round-trip it without loss, include it in snapshots and in `export`'s envelope, and render nothing. That is the honest scope — this is groundwork for PSD-0002, and pretending otherwise would invite someone to build a UI for an always-empty array. When it does get a surface, the likely home is a third mode on §7.13, which D19 deliberately left unstructured.

**Restore must not rewind the play log.** This is the one interaction that would otherwise be a silent data-integrity bug. D20 makes `restore --at` reinstate the character document wholesale; if that included `play_log`, restoring to a revision from before a patch was applied would erase the record of applying it, and the next `patch apply` would re-offer ops the player has already taken — the exact double-award PSD-0002's idempotency exists to prevent. So **`play_log` is preserved across restore rather than replaced**, and it is the only field with that exemption.

The asymmetry with `build_log` is deliberate and worth stating: build decisions *are* sheet state, so rewinding a sheet should rewind them. The play log records things that happened in the world — a session occurred, a patch was applied — and rewinding your character sheet does not un-happen the session. Restore's confirmation names the exemption.

## 7. Screen inventory

Screens map onto the schema's top-level sections, grouped the way a player thinks about their sheet rather than the way the JSON nests. **Nine section screens (§7.2–§7.10) plus the Sheet Overview, two wizards, and the History screen.** Number keys `1`–`9` map to the nine section screens; the wizards and History are reached contextually and by keybinding respectively (§8.4).

**Layout tiers (D12).** Three named tiers live in the theme package; screens query the tier rather than doing width arithmetic:

| Tier | Columns | Behaviour |
|---|---|---|
| `Compact` | 80–119 | Single column. Detail panes stack below their list and are reachable by `Enter`; the Inventory tree shows a breadcrumb plus a flat list of the current container; the Spellcasting tab strip collapses to a cycling source selector (`[` / `]`). |
| `Comfortable` | 120–159 | Two columns. List and detail side by side; the Inventory tree renders as a real indented tree; Spellcasting tabs render as a strip. |
| `Wide` | 160+ | Three columns. Adds a persistent context rail — active conditions, concentration, and the §7.9 weight/attunement summary — visible from every screen. |

Below 80 columns the tool refuses to launch with a message naming the current and required width.

### 7.1 Sheet Overview (home screen)

A dashboard, not a form: character name, class/level/species/background line, the headline combat numbers (AC, HP current/max/temp, initiative, speed, proficiency bonus, passive perception) as a Lip Gloss stat row, the ability block with modifiers, and a condition/exhaustion strip when any are active. Below that, the nine section screens as navigable entries, each with a one-line summary (e.g. "Spellcasting — Ranger, 2 of 4 first-level slots remaining, concentrating on Hunter's Mark") so the overview doubles as a status board. `Enter` navigates in; `1`–`9` jump from anywhere (§8.4).

### 7.2 Identity & Background

Free-text and short-choice fields: character/player name, pronouns, alignment, age/height/weight, faith, species, background.

**Species, background, origin feat, and their dependent data are free-text throughout v1 (D13).** The catalog does not cover them, so the user types the species name, its sub-choice (lineage/legacy/ancestry), its traits, the background's granted skills and tool proficiency, and the equipment package into the corresponding schema fields as editable lists. `source_ref.is_homebrew` defaults to true, which is the honest representation of "the user typed this in"; the user can clear it and record a book/page instead.

This is the largest remaining manual-entry burden in the tool, and it is a deliberate scope choice rather than an oversight — see §13.1.

Appearance, backstory, and roleplay notes render through Glamour in view mode and drop into a Bubbles `textarea` on edit — the one screen where long prose is the dominant content shape.

### 7.3 Abilities & Saves

Six ability rows, editable at the *input* level only — base score, generation method, background allocation, feat-increase entries — with total, modifier, and save bonus computed read-only beside them (§8.1). The user never types a modifier or an ability total.

**Proficiency bonus is an input here, not a computed value (D2)** — a single integer field in the header beside total character level, schema-bounded 2–6. From Phase 2b a read-only reference line shows the standard PB-by-level ladder next to it (D13), so the user can look it up without leaving the screen; the tool still does not apply it.

The background-allocation strip validates what the schema expresses — the `2_and_1` / `1_1_1` pattern, three points total, no ability raised more than 2 — and refuses an invalid split. It can only check the allocation against the background's three *eligible* abilities when the user has filled in `background.ability_scores_offered`; when populated, the check applies.

### 7.4 Skills & Proficiencies

A Bubbles `table` — one row per skill, columns for governing ability, proficient/expertise state, misc bonus, computed total — plus list panels for armor, weapon, tool, and language proficiencies. Space cycles none → proficient → expertise, matching the paper sheet's two-bubble convention; the total recomputes immediately.

### 7.5 Classes, Features & Feats

One expandable list per class (multiclass supported — the schema's `classes` array is not capped) showing level, subclass, saving-throw proficiencies, and weapon mastery slate. Features and feats render as a filterable list (by origin: class/subclass/species/background/feat) with the entry's `text_snapshot` in a Glamour detail pane.

**Read-mostly for structural data:** class levels, subclass selection, and feat acquisition belong to the level-up wizard (§7.12), because those decisions need `build_log` entries and cross-field consequences. Direct add/edit/remove *is* available as an escape hatch — a DM hands out a one-off feature, a player fixes a typo — but it writes no `build_log` entry, and the help text says so. Feature and feat text is hand-entered in all phases (D13).

From Phase 2b, a read-only class-table reference panel shows the selected class's progression, which is the highest-value use of the class-table catalog: it is exactly the information a player needs to fill in their own maxima.

### 7.6 Combat & Vitals

HP (max/current/temp, with `+`/`-` quick adjusters since damage and healing are the most frequent edit in play), hit dice pools per class, death saves (three success/three failure pips toggled with `s`/`f`, auto-reset on stabilize), Heroic Inspiration as a boolean (not a counter — the 2024 change), speeds, senses, and active conditions with add/remove plus, for Exhaustion, a level stepper.

**AC is entered as its breakdown and summed (D2).** The user adds named terms — "Half Plate 15", "Dex (capped) +1", "Shield +2" — and the tool totals them into `armor_class.total`, storing both per the schema's design. The tool knows nothing of armor categories or DEX caps; the user encodes the cap by entering the capped value. `armor_class.calculation` is a free-text label. An override (§8.2) short-circuits the sum.

Speeds are entered directly. `speeds.notes` is where the user records why current differs from base ("Base 35, reduced 10 by Exhaustion 2"); the tool does not apply condition effects to speed, because that is a rules table.

### 7.7 Resources & Rest

A generic list of `resource` entries (Rage uses, Ki points, Luck Points, per-feature pools — anything with `maximum`/`current`/`recharge`) as gauges with +/- adjustment, grouped by recharge trigger. Maxima are user-entered.

A rest panel with Short Rest and Long Rest actions resets `current` to `maximum` for every resource and slot pool whose trigger matches, restores hit dice, and surfaces `pending_long_rest_choices` (e.g. "swap a prepared spell", "swap a weapon mastery") as a checklist to work through before the rest completes. This is mechanical — moving numbers the user configured — and stays inside D2. Hit dice recovery defaults to half the pool total rounded down, editable per pool; it is the one rules-shaped default in the tool, and it is editable precisely because it is a rule the tool shouldn't be certain about.

### 7.8 Spellcasting

The most structurally complex screen, reflecting Part 4 of the schema report. A tab strip selects among `spellcasting.sources` (a character can have several — e.g. a Ranger with a Wood Elf lineage source), each showing its ability, save DC, attack bonus, preparation rule, and prepared count against maximum. In `Compact` the strip becomes a `[`/`]` cycling selector.

Per D2: **spellcasting ability, prepared maximum, cantrips known, and every pool's per-level slot maxima are user-entered.** Save DC (`8 + PB + ability modifier`) and attack bonus (`PB + ability modifier`) *are* computed, being arithmetic over values already supplied. Prepared *current* count is computed by counting entries. From Phase 2b the class-table reference panel sits beside the maxima fields.

Below the tabs, one spell list spans all sources, filterable by level and preparation origin, each entry badged by `preparation.origin` (prepared / always-prepared / innate-free-cast / in-spellbook-unprepared) since those differ at cast time. Casting is a single action (`c`) that decrements the correct pool by the entry's effective level, prompts for an upcast level where applicable, and — if `duration.concentration` — offers to set the `concentration` block, warning when something else is already held. Free-cast innate spells decrement their own `free_casts.remaining` rather than a pool; the UI must not conflate the two, which is exactly the bug the schema's explicit `origin` field exists to prevent.

**Spells are catalog-picked from Phase 2b (D13)** with a "custom…" escape hatch that falls back to full manual entry and persists the result to the local catalog (D14).

### 7.9 Inventory & Equipment

A container tree — Worn/Carried, then nested bags — each leaf showing quantity, weight, and equipped state. Rendering degrades per tier: a real indented tree at `Comfortable` and above, breadcrumb-plus-flat-list at `Compact`. The tree renders from the character document alone, since `inventory.entries` caches `item_name_cache` and `weight_lb_total`; only the detail pane reads `item_instances`, and it does so lazily on first selection (D20, §9.4). That fetch is a primary-key lookup of one small document and should be imperceptible; the pane shows a brief skeleton rather than a blank if it isn't. An entry whose instance no longer exists renders a "this item is missing" state offering to remove the orphaned entry — reachable via `import` of a document whose items didn't come with it (§9.4).

Move-between-containers is select-then-target (`m` to pick up, navigate, `Enter` to drop), not drag-and-drop, since terminals have no pointer model worth building around. Equip/unequip and attune/end-attunement are single-key toggles respecting the schema's hard constraints — attunement maximum (user-entered, default 3) and hands as a two-slot resource — refusing with an inline reason rather than failing silently.

A summary footer pins carried weight, carrying capacity, and attunement usage at all times; at `Wide` it moves to the persistent context rail. **Carrying capacity is user-entered (D2);** the tool rolls up carried weight through the container tree (honoring `contents_are_weightless_to_carrier`), adds coin weight, and compares. Encumbrance tier appears only when `encumbrance_variant_enabled` is set and thresholds are supplied.

**Base equipment is catalog-picked from Phase 2b (D13)**, with the same custom-entry escape hatch and local-catalog persistence.

### 7.10 Currency

Five denomination counters (cp/sp/ep/gp/pp) as direct-entry fields — never normalized, per the schema report's explicit stance — plus a computed total shown as a clearly-labeled, non-editable "≈ N gp". Conversion ratios come from the document's own `derived.conversion_table_used`, seeded with standard values and editable; the electrum toggle respects `electrum_enabled_at_table`. Coin weight (count ÷ 50) feeds §7.9.

A separate valuables list (gemstones, art objects, trade goods) with name, category, estimated value, appraised/sold flags. A collapsible ledger shows the append-only transaction log, collapsed by default.

### 7.11 Creation wizard (`dmosh character new`)

A sequential `huh.Form` group: name → ability generation method → ability scores → proficiency bonus and starting level → species (+ sub-choice) → background (+ ability allocation) → starting class → starting equipment (package vs. gold) → review.

**No ruleset step (D5)** — the wizard always writes `ruleset.revision: "2024"`.

Species, background, and class steps are free-text in all phases (D13); the equipment step gains a catalog picker in Phase 2b. Cross-field validation the schema can't express — principally the background allocation pattern — lives here, per §7.3.

Completing the wizard produces a schema-valid document with `doc_revision: 0` and an initial `build_log` containing `species_selected`, `background_selected`, and `class_selected`, then hands off into the editor (§7.1).

### 7.12 Level-up wizard (Phase 2a) (D3)

Triggered from the Classes screen. A `huh.Form` sequence: choose which class gains the level → confirm new class level and total character level (with proficiency bonus editable here, since this is when it changes) → HP increase (roll, average, or manual, writing `hp_roll_mode`) → add the hit die to its pool → new features for that class/level → subclass if this is that class's level 3 → ASI-or-feat if this is an ASI level → weapon mastery and prepared-spell adjustments where applicable → review.

The wizard **prompts for rather than derives everything rules-table-shaped (D2)**: it asks "what features do you gain at this level?" rather than knowing. Its value is sequencing, completeness (it won't let you forget the hit die or the subclass), and writing the correct `build_log` entries using the enumerated `decision` values the schema already defines. From Phase 2b the class-table reference panel is shown alongside the feature step, which is where it helps most.

### 7.13 History (Phase 3) (D19)

**One screen, two modes, rendered as log lines.** The two histories this tool accumulates answer different questions — `build_log` answers "what did I decide at level 4", snapshots answer "what did this sheet look like last Tuesday" — but they are both a reverse-chronological list of things that happened, and that is enough shared shape to justify one screen. `Tab` switches mode. (`Tab` rather than the `[`/`]` used in §7.8's Compact tab strip, because this screen is a single full-width list with no panes to cycle focus between, so §8.4's normal meaning for `Tab` is free here.)

Reached with `H` from anywhere; not one of the `1`–`9` numbered sections, since it describes the sheet's past rather than being part of it.

**Decisions mode** reads `build_log`, newest first, one line per entry — sequence, date, character level, the `decision` enum, and a flattened one-line rendering of `payload`:

```
#12  2026-06-02  L4  asi_slot_spent       feat: Piercer
#11  2026-05-11  L3  subclass_selected    Ranger → Hunter
#10  2026-04-01  L1  background_selected  Wayfarer (CON +2, DEX +1)
```

**Revisions mode** reads `character_revisions`, newest first, using the `summary` column §9.1 already stores:

```
r41  2026-08-10 14:02  Spellcasting — expended a 1st-level slot
r40  2026-08-10 13:10  Inventory — consumed Potion of Greater Healing
r39  2026-08-09 22:40  Currency — +75 gp (Emerald Enclave bounty)
```

Both modes render from data that already exists; **this screen requires no schema or table changes.**

**Actions.** Decisions mode is read-only, consistent with §8.3's statement that the build log is not a replayable event log. Revisions mode supports exactly one action — `Enter` restores the selected revision, the TUI equivalent of `dmosh character restore --at`, behind §8.4's inline `y`/`n` confirm. Restoring writes the old document back as a *new* revision (§8.6), so it is itself undoable, which is what makes exposing it here safe.

**Deliberately deferred (D19):** diffing between revisions, filtering or searching either log, grouping decisions by level or session, and any richer visualization than log lines. Snapshots ship in Phase 1 and the `dmosh character history` CLI command exists from then, so users are not without access to this data until Phase 3 — this screen is the comfortable way to read it, not the only way. Structure gets designed once there is a real log to look at.

## 8. Cross-cutting behaviors

### 8.1 Derived-value computation

`internal/character/derive` owns every computed field. **The boundary is arithmetic over user-supplied inputs; no rules tables (D2).** This list is the contract and it is exhaustive — anything not named as computed is an input.

**Computed:**

- Ability total = `base_score` + `background_increase` + Σ`feat_increases` + Σ`other_increases`, or `override_score` when set
- Ability modifier = `floor((total_score - 10) / 2)`
- Save total = ability modifier + (PB if `save_proficient`) + `save_misc_bonus`
- Skill total = ability modifier + (PB × 1 if proficient, × 2 if expertise) + `misc_bonus`
- Passive scores = 10 + the corresponding skill total
- Initiative = DEX modifier + misc bonus
- AC total = sum of `armor_class.breakdown` terms, or `override`
- Spell save DC = 8 + PB + source ability modifier; spell attack bonus = PB + source ability modifier
- `prepared_current_count` = count of entries where `is_prepared` and `counts_against_prepared_maximum`
- Inventory: per-entry weight (quantity × unit weight), container roll-ups honoring `contents_are_weightless_to_carrier`, carried total
- Coin weight = total coin count ÷ 50; currency `derived` totals via the document's stored conversion table
- Attunement slots used = count of entries with `is_attuned`
- Encumbrance tier, only when the variant is enabled and thresholds supplied

**User-entered, never computed:** proficiency bonus, HP maximum, hit dice totals, carrying capacity and push/drag/lift, all AC breakdown terms, all speeds, spell slot maxima per level per pool, prepared maximum, cantrips known, resource maxima, attunement slot maximum, spellcasting ability per source.

`Recompute(*Document) []Change` is pure and is called from the parent model's `Update` (§5.1) after every mutation, so derived-state bugs have exactly one home.

**Write-back policy (D7).** `Recompute` runs in memory everywhere; only three paths persist its output — the TUI on open (surfacing the diff as a banner, per the schema report's no-silent-mutation stance), the TUI on any subsequent edit, and explicit `dmosh character recompute`. `show` and `export` recompute for display only. `validate` compares stored against recomputed and reports drift with exit code 2 without touching the row. Read commands stay safe in pipelines and CI stays reproducible.

### 8.2 Manual overrides

Several fields exist so a player can override a computed value (`abilities.*.override_score`, `combat.armor_class.override`). Wherever an override exists, the screen renders the computed value struck through or grayed and the override prominent, with a visible affordance to clear it and return to computed — never a silent "the number changed because you cleared something three fields away".

### 8.3 Build log

Structural decisions — species/background/subclass selection, feat acquisition, ASI spend, weapon mastery swap, spell prepared/swapped, epic boon, level gained — append a `build_log` entry using the schema's enumerated `decision` values.

**Only two surfaces write to it:** the creation wizard (§7.11) and the level-up wizard (§7.12). The §7.5 escape hatch deliberately does not, because a log mixing "this was a level-up decision" with "I fixed a typo" is worse than one recording only the former. The consequence, stated plainly: **the build log is not a replayable event log and undo is not built on it** (§8.6). In Phase 1, where §7.12 doesn't exist, the log is sparse — acceptable because nothing consumes it until Phase 3.

### 8.4 Keybindings

Global: `?` opens the context-sensitive Bubbles `help` overlay; `Esc`/`q` navigates back one level (never quits from a non-root screen, avoiding accidental data loss); `Ctrl+S` forces a validated save; `1`–`9` jump to sections; `H` opens the History screen (§7.13, Phase 3); `Tab`/`Shift+Tab` cycle focus within a screen; `u` undoes and `Ctrl+R` redoes (§8.6). Every screen registers its own `key.Binding` set so the help overlay is generated, not hand-maintained.

**Why `u` rather than `Ctrl+Z`:** `Ctrl+Z` is `SIGTSTP` in a terminal. Capturing it means disabling the tty's suspend character, which surprises users who expect job control to work. Vim-style `u`/`Ctrl+R` is unambiguous and costs nothing.

Destructive actions (remove inventory entry, clear an override, delete a character from the picker) require an inline `y`/`n` confirmation, not a modal dialog. Deletion is soft (D8) and its prompt names `undelete`, so the user knows it is recoverable. `purge` requires `--force` and is CLI-only — it has no in-TUI path at all.

### 8.5 Autosave and dirty state

The document saves on a 2s debounce after the last mutation, and always on clean navigation away from the editor (`q` from the overview, or `Ctrl+C` with a synchronous flush in the `tea.Program` quit path). **Debounced autosave does not validate (D6)** — it writes as-is so work is never lost. Validation gates `Ctrl+S`, navigation-away, and `export`. The store can therefore transiently hold a schema-invalid document; §6's open-with-warning path handles encountering one and `validate` reports it.

The status line shows the resolved database path, a dirty indicator (`●` unsaved / `✓` saved with relative timestamp), and a `⚠` when the in-memory document is currently failing validation — so save-time rejection is never a surprise.

Each save is the UPDATE-with-revision-predicate from §5.3; `doc_revision` increments on every success. v1 assumes one intended writer per character, but the conflict-detection mechanism exists as a side effect of using the database correctly. Zero rows affected surfaces a conflict banner rather than losing the write; resolving such a conflict is out of scope for v1.

### 8.6 Undo and snapshots (D17)

Two independent mechanisms, deliberately not built on `build_log` (§8.3).

**In-session undo.** A bounded stack of whole-document states, pushed by the parent model on every mutation (§5.1), with `u` to undo and `Ctrl+R` to redo. Bounded by count (50 frames) and discarded on exit — this is "I mistyped that", not history. Whole-document snapshots are used rather than inverse operations because a character document is a few tens of kilobytes and the mutation set is broad; storing 50 copies is cheaper than writing and testing an inverse for every message type. Undo is disabled in read-only mode.

**Persisted snapshots.** Every *validated* save (`Ctrl+S`, navigation-away) inserts a row into `character_revisions` in the same transaction as the `UPDATE`. Debounced autosaves do not snapshot — they fire every few seconds and would swamp the table. Surfaced as `dmosh character history` (listing revision, timestamp, and a one-line summary of what changed) and `dmosh character restore --at <timestamp|revision>`, which writes the old document back as a *new* revision rather than rewinding the counter, so restoring is itself undoable.

**Snapshots cover the whole aggregate (D20).** Because a character and its items share one revision, each snapshot row captures the character document *and* the full set of item instances at that revision, assembled in SQL so lazy loading is unaffected (§9.4). Restore replaces both in a single transaction, so moving to a prior revision is a clean transition rather than a partial one.

**Pruning,** applied on insert: keep the most recent 50 revisions per character, and additionally keep anything under 30 days old. A character saved heavily in one session keeps that session's detail; one saved occasionally over a year keeps a year of history. At ~30 KB per document, 50 revisions is ~1.5 MB per character — acceptable, and `purge` reclaims it. The specific numbers are tunable defaults, not load-bearing.

## 9. Storage layer

### 9.1 Schema

```sql
CREATE TABLE characters (
    id             TEXT PRIMARY KEY,       -- matches document._id
    document       TEXT NOT NULL,          -- full character.schema.json document, as JSON text
    schema_version TEXT NOT NULL,
    doc_revision   INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    deleted_at     TEXT,                   -- soft delete (D8); NULL = live
    -- generated columns lift the fields `list` needs out of the document so it can be
    -- queried and sorted in SQL without deserializing every row's JSON in Go:
    character_name TEXT    GENERATED ALWAYS AS (json_extract(document, '$.identity.character_name')) STORED,
    total_level    INTEGER GENERATED ALWAYS AS (json_extract(document, '$.progression.total_character_level')) STORED,
    ruleset_rev    TEXT    GENERATED ALWAYS AS (json_extract(document, '$.ruleset.revision')) STORED,
    campaign_id    TEXT    GENERATED ALWAYS AS (json_extract(document, '$.campaign_id')) STORED
);
CREATE INDEX idx_characters_name ON characters(character_name) WHERE deleted_at IS NULL;

-- Snapshot history (D17, §8.6)
CREATE TABLE character_revisions (
    character_id  TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    doc_revision  INTEGER NOT NULL,
    document      TEXT NOT NULL,
    items         TEXT NOT NULL DEFAULT '[]',  -- D20: item_instances documents at this revision,
                                               -- as a JSON array. Snapshots capture the whole
                                               -- aggregate so restore is atomic (§9.4).
    saved_at      TEXT NOT NULL,
    summary       TEXT,                    -- one-line description of what changed
    PRIMARY KEY (character_id, doc_revision)
);

-- Phase 2a (D4, D20). No per-item revision: items share the owning character's doc_revision,
-- so the character row is the single conflict domain for the whole aggregate.
CREATE TABLE item_instances (
    id                  TEXT PRIMARY KEY,
    owner_character_id  TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    document            TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX idx_items_owner ON item_instances(owner_character_id);

-- Phase 2b (D13, D14). Holds SRD 5.2.1 seed content AND user homebrew.
CREATE TABLE catalog_entries (
    id           TEXT PRIMARY KEY,
    entry_kind   TEXT NOT NULL,            -- 'spell' | 'item' | 'class_table'
    name         TEXT NOT NULL,
    document     TEXT NOT NULL,            -- the entry, shaped per its kind
    is_homebrew  INTEGER NOT NULL DEFAULT 0,
    srd_licensed INTEGER NOT NULL DEFAULT 0,
    authored_by  TEXT,                     -- maps to source_ref.authored_by_user_id
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_catalog_kind_name ON catalog_entries(entry_kind, name);

-- Local feedback counters for §12. Deliberately NOT in the character document: that
-- document is the portable artifact (exported, patched, one day synced), and usage
-- data has no business travelling with it. Never leaves the machine.
CREATE TABLE usage_counters (
    key         TEXT PRIMARY KEY,   -- e.g. 'screen_entry.spellcasting', 'restore.invoked',
                                    -- 'field_edit.proficiency_bonus', 'width_bucket.compact'
    value       INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL
);

CREATE TABLE schema_migrations (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL
);
```

This keeps the schema report's document-database framing rather than fighting it: the `document` column *is* the aggregate root, and JSON1 functions project a few fields for indexing — never to decompose the document into a normalized relational shape. `dmosh character list` becomes `SELECT id, character_name, total_level, updated_at FROM characters WHERE deleted_at IS NULL ORDER BY character_name`, and the generated columns cannot drift because SQLite recomputes them on write.

**Homebrew (D14).** `catalog_entries` holds seeded SRD content and user-created entries in one table, discriminated by `is_homebrew`. A "custom…" escape hatch in any picker writes a homebrew row, making that entry reusable by every character in the database instead of being retyped. There is no sharing or pack format yet (§13.2).

Migrations are numbered `.sql` files embedded via Go's `embed`, applied in order against `schema_migrations` at startup — independent of `character.schema.json`'s `schema_version`, which versions the document rather than the tables.

### 9.2 Connection setup (D11)

Three pragmas are set on every connection, in this order:

```
PRAGMA journal_mode = WAL;    -- readers never block on the TUI's writer
PRAGMA busy_timeout = 5000;   -- retry rather than error on the rare writer collision
PRAGMA foreign_keys = ON;     -- REQUIRED: off by default in SQLite
```

WAL matters because D7 guarantees read commands never write, which makes `dmosh character show` against a database the TUI has open a legitimate workflow — a status bar, a stream overlay, a script watching HP. Without WAL those readers would intermittently hit `SQLITE_BUSY`.

`foreign_keys = ON` is not optional decoration: SQLite ignores foreign key constraints by default, so without it the `ON DELETE CASCADE` clauses on `character_revisions` and `item_instances` silently do nothing and `purge` would orphan rows.

**Two caveats to document in the README.** WAL creates `-wal` and `-shm` sidecar files beside the database, so backing up or moving a store means copying all three, or better, using `VACUUM INTO` / the SQLite backup API — a plain `cp` of just the `.db` while the TUI is running can capture a torn state. And WAL is unreliable on network filesystems (NFS, SMB); the tool should detect a non-local filesystem at startup where it can and warn, rather than letting a user discover corruption on a shared drive.

### 9.3 What the file-based design gave up, and how that's mitigated

v0.1 leaned on "the character is a plain JSON file, so it survives the tool being wrong about something." Moving the row into a database weakens the `cat`-it-in-an-emergency property. Four things preserve or exceed it:

- The `document` column holds unmodified JSON *text*, so `sqlite3 characters.db "SELECT document FROM characters WHERE character_name='…'"` still returns a readable document.
- `dmosh character export --format json` is first-class, so backup, diffing, and handing a character to someone without this tool never require a SQLite client. **It bundles the item instances alongside the character document** — an envelope of `{character, items[]}` rather than the bare document — because D20 makes those one aggregate, and an "export" that dropped every magic item would make this durability claim false. `import` unpacks the same envelope, and accepts a bare character document too, which is the case that can produce the missing-item state in §7.9.
- Deletion is soft (D8), so "I destroyed my character" is answered by `undelete` where the filesystem's answer was a trash can.
- **Snapshots (D17) exceed what files gave.** A directory of JSON files had no history unless the user thought to put it in git. `dmosh character history` plus `restore --at` is strictly better than the thing it replaced.

### 9.4 Item instances (D4, D20)

Item instances live in their own table keyed by `owner_character_id`, matching the schema report's Part 2, where an item instance is its own document with its own `_id`. The character document's `inventory.entries[]` continues to hold `item_instance_id` plus the denormalized `item_name_cache` and weight.

**Lazy loading (D20).** An item instance is read when its detail pane is first opened, not on character open, and is cached in memory for the rest of the session. Eager loading is deferred until a real need appears — it is a change to one function, and nothing in the design forecloses it.

This works cleanly only because the schema report's redundancy policy already anticipated it. Everything the Inventory screen shows *without* a detail pane open comes from the character document alone: the tree renders from `item_name_cache`, the footer's carried weight sums the cached `weight_lb_total` per entry, and attunement usage counts `entries[].equipped.is_attuned`. No screen has to block on an item read to draw itself, so lazy loading costs a fetch on selection and nothing else. Had the schema normalized those caches away, this decision would have been forced the other way.

**Shared revisioning (D20).** A character and its item instances are **one conflict domain and one revision**. `item_instances` rows carry no revision of their own; the character's `doc_revision` covers the whole aggregate. Editing an item marks the character dirty and its write goes in the same transaction, guarded by the same `doc_revision` predicate (§5.3). The point is §8.6: a snapshot that captures the aggregate makes restore a clean transition to a prior state rather than a partial one.

**Lazy loading and shared revisioning compose without tension**, which is worth showing because it is not obvious. The worry would be that a snapshot needs every item while the editor has only lazily loaded a few. It doesn't — the snapshot is assembled in SQL, so items the editor never read are still captured:

```sql
INSERT INTO character_revisions (character_id, doc_revision, document, items, saved_at, summary)
SELECT :id, :rev, :document,
       (SELECT json_group_array(json(document)) FROM item_instances WHERE owner_character_id = :id),
       :saved_at, :summary;
```

`json_group_array` aggregates the item documents inside the database. The editor never has to hold the full inventory in memory, so lazy loading survives intact and snapshots are still complete.

**Restore is atomic.** `restore --at` replaces the character document *and* the item set — delete the current `item_instances` rows for that character, reinsert from the snapshot's `items` array, write the document, bump the revision — in one transaction. The result is exactly the state at that revision, and because it writes forward as a new revision (§8.6) it stays undoable. There is no partial-restore caveat and no restore-induced dangling reference.

**Dangling references are still possible, from a different direction.** `import` reads a standalone character document whose `inventory.entries[]` may reference `item_instance_id`s that do not exist in this database (see §9.3 on what `export` bundles). The tree still renders such an entry from its cached name and weight; the detail pane shows a "this item is missing" state offering to remove the orphaned entry. It must never be a crash or an empty pane.

## 10. Testing strategy

Screen-level logic uses `charmbracelet/x/exp/teatest`, driving each screen's `Update` with scripted key sequences and asserting on rendered output. Layout tiers are tested by driving `tea.WindowSizeMsg` at 80, 120, and 160 columns and snapshotting each — which also catches the below-80 refusal path.

The derive package (§8.1) is table-driven against the schema report's worked example ("Vesk Ambermarch") as a known-good fixture: computing modifiers, saves, skill totals, and weights from its inputs and asserting they match its stored outputs is both a derive test and a schema-fidelity regression test.

**Because of D2, the derive tests also enforce the no-rules-tables boundary.** A test asserting `Recompute` leaves `proficiency_bonus` untouched at a level where the 2024 table would change it is the cheapest way to catch a well-meaning contributor adding a lookup. Equivalent guard tests cover carrying capacity and slot maxima.

`internal/store` tests run against SQLite's `:memory:` mode — faster setup/teardown, no stray files — with a separate small suite against a temp file for the paths that are file-specific: WAL sidecar behaviour, the pragma set, and `VACUUM INTO`. Round-trip tests (load → mutate via the model → save → validate) run over fixtures seeded at setup, covering the structurally distinct cases: multiclass with two slot pools, a lineage-granted innate spell, a cursed attuned item, an Exhaustion-leveled condition, a `1_1_1` background allocation, a soft-deleted row, and a character with enough revisions to trigger pruning.

Specific behaviours that need their own tests because they are easy to get subtly wrong: the undo stack's bound and its interaction with `Recompute` (undoing must restore derived values, not recompute them from a half-restored state); snapshot pruning keeping the union of "most recent 50" and "under 30 days" rather than the intersection; `purge` cascading to both `character_revisions` and `item_instances`, which fails silently if the `foreign_keys` pragma regresses; and the three `validate` exit codes.

§6.1's `play_log` needs three, all cheap and all guarding against silent loss rather than visible failure: that a document carrying a populated `play_log` round-trips through load → mutate → save with every entry and every `resolutions[]` element intact; that a `resolutions[].kind` the tool has never heard of survives that round-trip unaltered, since PSD-0002's vocabulary is explicitly unstable; and that `restore --at` leaves `play_log` untouched while replacing everything else, which is the double-award guard and the one place a regression would be invisible until a player was awarded twice.

D20 needs four. That opening a character issues no reads against `item_instances` — the regression that would quietly turn lazy into eager. That a snapshot taken while only a few items were lazily loaded still captures *all* of them, which is the specific way the lazy/shared combination could break and the reason the aggregation is done in SQL. That restore reinstates document and item set atomically, including the case where items were added after the snapshot and must disappear on restore. And that `export` → `import` round-trips items, with a bare-document import producing the missing-item pane rather than a crash.

D18's resolution needs its own small suite, since it is the one behaviour driven by ambient state a test can easily get wrong by accident: each rung of the precedence ladder in isolation, the ambiguous-name error, the soft-deleted-target error, and — most importantly — an assertion that every lifecycle command still fails without an explicit argument *while `DMOSH_CHARACTER` is set*. That last case is the regression that would quietly reintroduce the footgun the carve-out exists to prevent.

`dmosh character validate --all` against a seeded fixture database is a CI gate, as is `go generate` cleanliness (§6).

## 11. Phased delivery

**Phase 0 — schema prerequisite.** The `play_log` change to `character.schema.json` (§6.1), the `schema_version` bump to `1.1.0`, and regenerated types. No behaviour, no screens; it exists so that D9's codegen and the round-trip corpus are built against the final root shape rather than migrated to it later.

**Phase 1 — the numeric sheet.** Sheet Overview, Identity & Background, Abilities & Saves, Skills & Proficiencies, Combat & Vitals, Currency, the creation wizard, in-session undo and snapshots (§8.6), and the full non-interactive command set. Manual entry throughout; no catalog. Covers the numeric heart of a paper sheet and is independently useful.

**Phase 2a — structure, still manual (D16).** Classes/Features/Feats (§7.5), the level-up wizard (§7.12), Resources & Rest, Spellcasting, Inventory & Equipment, and the `item_instances` table (lazy-loaded, sharing the character's revision per D20). Needs nothing architecturally new beyond that table — all screens run on manual entry.

**Phase 2b — catalog (D13, D14).** `catalog_entries`, SRD 5.2.1 seeding for spells and base equipment, class tables as read-only reference panels (§7.3, §7.5, §7.8, §7.12), picker retrofit in Spellcasting and Inventory, and homebrew persistence via the custom-entry escape hatch.

Sequencing 2a before 2b is deliberate: the catalog's picker requirements are then informed by screens that already exist, rather than guessed at.

**Phase 3.** The History screen (§7.13 — one screen, two modes, log lines), reputation/renown, import from other tools' export formats. Multi-character and sync-aware features wait for the server component and are a different tool that happens to read the same database.

## 12. What v1 is built to find out

There is no telemetry (§2), so feedback has to be a deliberate artifact. `dmosh character feedback --out FILE` writes a local, reviewable markdown summary from the `usage_counters` table (§9.1) — screens entered, which fields get edited, wizard versus escape-hatch level-ups, undo and restore use, terminal widths seen. The player reads it in full and decides whether to share it. Nothing leaves the machine on its own.

**The counters live in the database, not in the character document.** That document is the portable artifact — exported, patched, one day synced — and usage data has no business travelling with it. Keeping them in a sibling table is also what lets this section exist without touching `character.schema.json`, which has survived every revision of this spec unchanged (§0).

The README carries the questions v1 is designed to answer:

1. **Is this used at the table during play, or between sessions for bookkeeping?** The whole shape of Phase 2 assumes the former: §7.6's `+`/`-` HP adjusters, §7.8's one-key cast action, §7.7's rest panel are all built for speed under time pressure. *Instrument:* each run is classified at exit as combat-dominant (HP, slots, resources, conditions) or sheet-dominant (identity, inventory, proficiencies, currency) and increments one of two counters. *Direction:* if runs are overwhelmingly sheet-dominant, keystroke economy in the runner-ish screens was the wrong investment and the tool should optimise for review, printing, and export instead.

2. **Does D2's no-rules-tables boundary hold, or does the tool feel like it isn't pulling its weight?** This is the largest bet in the document. *Instrument:* edit counts on the fields the tool deliberately refuses to compute — proficiency bonus, slot maxima, prepared maxima, carrying capacity — plus use of the §8.2 override fields. *Direction:* frequent proficiency-bonus edits at moments that aren't level-ups mean players are correcting it after the fact, which is the cheapest possible argument for deriving it from level and the one table most likely to earn an exception. Heavy `armor_class.override` use means the named-breakdown-terms model is wrong rather than the arithmetic.

3. **Does manual entry stop people finishing a character?** D1 deferred the catalog and D13 narrowed it so species, backgrounds, and feats stay free-text indefinitely (§7.2). That is the largest remaining typing burden and it is entirely a guess that players tolerate it. *Instrument:* characters created versus characters still being edited a week later, and how many reach level 2. *Direction:* if abandonment clusters immediately after the creation wizard, Phase 2b is urgent and its scope should widen to cover exactly the fields §13.1 leaves open. If completion rates are fine, the manual-entry burden is theoretical and the catalog can stay narrow.

4. **Do players use the level-up wizard, or bypass it?** §7.12's entire claim is sequencing and completeness, and §7.5's escape hatch makes bypassing easy on purpose. *Instrument:* `build_log` entry count against observed level changes — a level 5 character with two log entries was levelled by escape hatch. *Direction:* if bypass is the norm, the wizard's claim is false, and both the build log and §7.13's Decisions mode are built on sand. That would be worth knowing before Phase 3 builds a screen to browse them.

5. **Does anyone ever restore a snapshot?** D17 and D20 bought a revisions table, an `items` column, and atomic restore. *Instrument:* `restore` invocations, and how deep the in-session undo stack actually gets. *Direction:* if no one restores across a whole campaign, snapshots are dead weight, pruning can be far more aggressive than §8.6's defaults, and §7.13 collapses to one mode. If restores do happen, what a player was doing just before one says what the tool let them break.

6. **Which sections do players actually open?** *Instrument:* screen-entry counters per section. *Direction:* reputation and renown were cut from v1 (§1) on the reasoning that they are DM-facing; players asking for them is the signal to reconsider. Conversely, a section nobody opens is a candidate for folding into the Sheet Overview rather than costing a number key.

7. **What terminal width do people actually run at?** D12 bought three layout tiers and a hard 80-column floor. *Instrument:* width bucket at startup. *Direction:* if nothing is ever below 120, `Compact` was wasted work and future screens can assume two panes; if 80 is common, the tier discipline stays mandatory and narrow layouts need designing first rather than last.

8. **Does the environment-variable ergonomic get used?** D18's `.envrc`-per-player-folder pattern is a guess about how people organise campaigns. *Instrument:* how often a command resolves its target from `DMOSH_CHARACTER` rather than an argument. *Direction:* this one informs more than this tool — the GM subsystem leans on the same shape for `--db`, so evidence here is evidence there before that spec commits further.

**Question 1 is load-bearing.** If the tool turns out to be a between-sessions companion rather than a table companion, Phase 2's interaction-heavy screens are still worth building but stop being the product's centre of gravity, and the roadmap should turn toward export, print, and sharing instead. Question 2 is the one most likely to force reversing a stated decision, and it is deliberately the easiest to act on: D2 is a boundary, not an architecture, and moving it costs one function.

## 13. Open questions

Two remain. Each is deferred by choice, with the reason stated.

1. **Catalog coverage for species, backgrounds, feats, and features (follows D13).** These stay free-text through v1, and they are the largest remaining manual-entry burden (§7.2). Whether a later phase adds them is open; it's a bigger content problem than spells because the entries are more entangled with the wizards and with each other. Worth revisiting once Phase 2b shows how well the catalog pattern works for spells.
2. **Shareable homebrew packs (follows D14).** The local catalog makes homebrew reusable within one database but not transferable between people, which is in tension with the toolkit's collaborative goal. Deferred deliberately to the server work, where sharing semantics (who owns a pack, what happens on conflicting imports, how versions are pinned) will be forced decisions anyway rather than guesses.