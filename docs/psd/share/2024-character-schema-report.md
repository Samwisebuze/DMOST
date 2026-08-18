---
title: "Product Specification: GM Session Tool (v0)"
component: "dmosh CLI — gm subsystem"
status: "Draft v0.6"
audience: "Engineering (Go/TUI implementers), project contributors"
related_docs:
  - docs/psd/0001_character-management-tui.md
  - docs/jsonschema/character.schema.json
  - docs/psd/share/dnd-2024-character-schema-report.md
---

# Product Specification: GM Session Tool (v0)

## 0. Decision record

| # | Question | Resolution | Where |
|---|---|---|---|
| 1 | Patch op format | Semantic ops naming intent, never JSON Pointers into the character document | §6.3 |
| 2 | `grant_item` payload | Opaque to the GM tool — stored and forwarded, never inspected | §6.3 |
| 3 | Op vocabulary stability | No compatibility guarantee in v0; unknown kinds skip rather than fail | §10.7 |
| 4 | Patch idempotency | Per op, recorded in the character log; re-apply re-offers the unresolved remainder | §6.3 |
| 5 | Ending HP | No HP op — both parties watched the damage happen | §6.3 |
| 6 | Player-tracked resources | No resource op, same reasoning; the vocabulary is nine ops | §6.3 |
| 7 | Initiative | Typed in, never rolled; entered in any order; adversary instances share one count by default | §7.3 |
| 8 | Campaigns per database | One | §8.3 |
| 9 | Stale snapshots | Flagged on the party board and in `party list`, measured in sessions since import | §7.2 |
| 10 | Splitting an adversary | Keeps the parent's count, pre-filled for override; takes the instance's own label | §7.3 |

**v0.5 changes:** decision 6 is new (`adjust_resource` dropped). The rest of this revision is corrective — it closes the gaps an implementation review found: no capture path existed for the award events the tool exists to emit (§5.4, §7.2), participants and events had no ids (§5.2, §5.4), party HP between fights had nowhere to live (§5.1), encounter `status` was rendered but not modelled (§5.2), and patch regeneration could mint fresh ULIDs and double-award (§6.3). Repetition across sections has been cut in favour of this table.

**v0.6 changes:** the last of §12's questions are settled. Initiative entry became an any-order editable mode rather than a sequential pass, because numbers arrive at a table in whatever order people speak (§7.3). Splitting an adversary keeps the parent's count with the field pre-filled for override, and the `tiebreak - 1` offset is gone — the sort's existing insertion-order key already does that work (§5.2, §7.3). Entry and group labels are specified (§5.2). Staleness stays on the session-delta signal already in the document; no second measure was added.

**Naming:** the binary is `dmosh`. This revision renames every invocation, the character store (`dmosh.db`), and the fallback data directory (`$XDG_DATA_HOME/dmosh/`) accordingly. The latter two are defined by the character TUI spec and need the same edit there; the character tool's database resolver should also fall back to a pre-existing `dnd.db` in the same location and say so once, rather than silently creating an empty store beside it.

---

## 1. Purpose and scope

This is the second interface shipped for the Dungeon Master Open Source Toolkit, and the first aimed at the GM. Its job is narrow on purpose: **help a GM run a session at the table**, and in doing so, force us to build the player↔GM data exchange interface for real.

Everything else a GM might want — campaign prep, NPC and location databases, quest trackers, encounter building against a monster catalog — is out of scope for v0. Not because it's unimportant, but because we don't yet know which of those the toolkit should own, and each is a large, opinionated domain commitment. v0 exists to earn the right to make those commitments by putting something usable in front of GMs first.

v0 is also deliberately the *cheapest* way to build the exchange layer. Players and GMs move data by exporting and importing files. There is no server, no discovery, no sockets. But the files are not ad-hoc dumps — they carry a versioned envelope (§6.1) designed from the start to be the message body a future LAN sync server sends over the wire. When the GM-hosted server lands, the transport changes and the payloads do not.

**In scope for v0:**

- Importing player character snapshots into a campaign (player → GM).
- A live party board: read-only vitals, defenses, and passive scores for every party member.
- An encounter runner: initiative order, round/turn advancement, HP and condition tracking, concentration tracking.
- Awarding XP, currency, items, renown, and story features to party members during or after play.
- A minimal, homebrew-first adversary store — enough to run a fight, not a monster catalog.
- A session log that accumulates the consequences of play and exports them as per-character **patch packets** (GM → player) the character tool can apply.
- Non-interactive commands for everything scriptable (§4).

**Out of scope for v0, named so the boundary is explicit:**

- Campaign prep of any kind: NPCs, locations, factions, quests, session notes, hooks, calendars.
- Encounter *building* — difficulty budgets, XP thresholds, party-strength math. v0 runs an encounter the GM has already decided on.
- Any bundled monster/spell/item catalog (§10.2).
- Rules automation: no attack rolls, no damage calculation, no saving-throw resolution, no durations expiring on their own. The GM does the rules; the tool remembers the numbers.
- The LAN sync server, discovery, auth, and conflict resolution.
- Any GM authority over a character document. The player's copy is authoritative, always. The GM holds read-only snapshots and *proposes* changes as patches.

## 2. Non-functional constraints

Inherited wholesale from the character TUI spec §2: single static Go binary, usable at 80×24 with graceful degradation, works over SSH and inside tmux, no true-color or terminal-graphics dependencies, no network calls, no telemetry, sub-100ms startup.

Two constraints specific to this tool:

**The GM binary never opens the character store.** `internal/gmstore` opens `campaign.db` and nothing else. There is no code path from `dmosh gm` to `dmosh.db`, enforced by package boundary rather than convention: the campaign store package has no dependency on the character store package, and the only way character data enters a campaign is `dmosh gm party import` reading a file. This is what makes the eventual client/server split a transport change rather than a rewrite.

**Encounter state must survive a crash mid-fight.** Nothing about a session is reconstructible from memory once the table has moved on. Every mutation during an encounter commits synchronously (§8.3), not debounced the way the character editor's autosave is.

## 3. Technology stack

Identical to the character TUI spec §3 — same libraries, same idioms, shared internal packages where it makes sense.

| Layer | Library | Role |
|---|---|---|
| CLI framework | `spf13/cobra` | `dmosh gm ...` subcommand tree under the same root binary |
| TUI runtime | `charmbracelet/bubbletea` | Elm-architecture loop for the session runner |
| Component kit | `charmbracelet/bubbles` | `list`, `table`, `textinput`, `textarea`, `viewport`, `help`, `key` |
| Structured forms | `charmbracelet/huh` | Campaign init, adversary entry, award forms, patch review |
| Styling | `charmbracelet/lipgloss` | Layout and theming; shared `theme` package with the character TUI |
| Markdown rendering | `charmbracelet/glamour` | Statblock text snapshots, session log rendering |
| Logging | `charmbracelet/log` | File-based structured logging |
| Schema validation | `santhosh-tekuri/jsonschema/v5` | Validates campaign documents, imported snapshots, adversary imports, emitted patches |
| Persistence | `database/sql` + `modernc.org/sqlite` | Embedded document store, cgo-free for the static-binary requirement |
| Demo/docs tooling | `charmbracelet/vhs` | Terminal recordings for docs |

**No new dependencies in v0.** Notably absent and intentionally so: any HTTP or RPC library, any service-discovery library, any crypto/identity library. Those belong to the sync milestone, and pulling them in early invites network assumptions into a tool that shouldn't have them.

Shared `internal/` packages used by both subsystems: `theme`, the schema-validation wrapper, and `exchange` — which owns the envelope and the patch op vocabulary. The GM subsystem depends on `exchange` to *emit* ops, the character subsystem to *apply* them, and neither depends on the other; that package is the entire write-side contract between the two tools. There is deliberately **no shared `derive` package** (see §5.1). Struct generation from JSON Schema follows whatever the character tool settles on in its §12.1 spike.

## 4. Command structure

```
dmosh gm init [--name NAME] [--db PATH] [--force]         # create a campaign document + campaign.db
dmosh gm party import <file>... [--db PATH]               # ingest character snapshot envelopes
dmosh gm party refresh <member> <file>                    # replace one member's snapshot
dmosh gm party list                                       # roster table, with a staleness column
dmosh gm party manifest --out FILE                        # write a party_manifest envelope
dmosh gm adversary add [--from FILE]                      # huh form, or non-interactive import
dmosh gm adversary list
dmosh gm encounter new [--name NAME] [--member M]... [--adversary ID[:N]]...
dmosh gm encounter list
dmosh gm run [<encounter>] [--db PATH]                    # session runner TUI
dmosh gm session start|end [--number N]                   # end generates patches and opens review
dmosh gm patch list [--session N] [--character X]
dmosh gm patch export [--character X] [--session N] --out DIR [--force]
dmosh gm show <thing> [--format text|markdown|json]
dmosh gm validate [--all]
dmosh gm export [--format json] --out FILE                # whole campaign document
dmosh gm feedback --out FILE                              # local usage summary (§11)
```

`<thing>` for `show` is one of `campaign`, `member <id>`, `adversary <id>`, `encounter <id>`, `session <n>`, `patch <id>`.

`validate` with no arguments validates the campaign document. `--all` additionally validates every roster snapshot against the vendored character schema and every pending patch against the op vocabulary.

**Which commands take over the terminal:** `run`, `session end`, bare `dmosh gm`, `adversary add` without `--from`, and `encounter new` without participant flags. Everything else prints and exits — including `adversary add --from FILE` and a fully-flagged `encounter new`, which exist precisely so a campaign can be built from a script or a fixture in CI.

**Exit codes**, matching the character tool's contract: `0` success; `1` validation failure, with failing JSON Pointer paths on stderr; `2` succeeded with warnings (schema skew, stale snapshots, skipped ops).

Matching additions on the player side, specified here because they are half of the exchange interface but implemented in the `character` subsystem:

```
dmosh character export <name> --format snapshot --out FILE
dmosh character patch preview <file>                      # no writes
dmosh character patch apply <file>                        # interactive, per-op review
dmosh character manifest apply <file>                     # sets campaign_id, nothing else
```

`patch apply` is interactive by §6.3's mandate, which makes it the character subsystem's second interactive command after `new`/`open` — a companion edit to that spec's §4.

Bare `dmosh gm` resolves the default campaign database (`--db PATH`, then `./campaign.db`, then `$XDG_DATA_HOME/dmosh/campaign.db`) and opens the session dashboard (§7.1).

## 5. Data model

### 5.1 What the campaign document holds

v0 introduces `campaign.schema.json`, kept deliberately thin: an aggregate root stored as JSON text in a SQLite column, validated on every read and write.

Top-level blocks:

- `campaign` — id, name, ruleset revision (2024 default, matching the character schema's `ruleset.revision`), created/updated timestamps, `doc_revision`, `current_session_number`.
- `roster[]` — one entry per party member (§5.1.1).
- `adversaries[]` — statblock documents (§5.3).
- `encounters[]` — definitions and run state (§5.2).
- `sessions[]` — `session_number`, start/end timestamps, `events[]` (§5.4).
- `outbound_patches[]` — generated patches with `status` (`pending` / `exported` / `acknowledged`), see §6.3.
- `usage` — local counters for §11's feedback summary: `{screen_entries: {dashboard, party_board, runner, adversary_library, session_review}, sessions_run, encounters_run}`. Incremented on screen entry, flushed with the next mutation rather than synchronously — this is the one thing in the document that doesn't need §2's durability. It never leaves the machine.

#### 5.1.1 Roster entries

```
{
  member_id,                 // ULID, the campaign's stable handle for this player
  display_name,
  character_snapshot,        // full character document, verbatim, never edited
  provenance: {
    source_character_id,     // the snapshot's _id
    source_doc_revision,     // the snapshot's doc_revision
    imported_at,             // wall clock
    imported_at_session,     // campaign.current_session_number at import; 0 before session 1
    source_tool_version,
    produced_by,             // from the envelope
    schema_skew              // true if the snapshot's schema_version differs from the vendored one
  },
  import_history: [ { at, at_session, source_doc_revision, source_tool_version } ],
  cached: { ... },           // derived fields lifted from the snapshot at import, with computed_at
  party_state: {             // live between-fight state; see below
    hp_current, hp_temp, conditions[], concentrating_on, updated_at
  }
}
```

`cached` holds what the party board needs without re-walking the snapshot: AC, HP max, passive Perception, save bonuses, speeds, defenses. These are **read out of the snapshot, not computed** — the GM tool does not reimplement the character tool's derive package, and there is no fallback. A derived field absent from a snapshot renders as `—` on the party board with the field named; the fix is a fresh snapshot, not a second implementation of the maths. This is why §3 lists no shared `derive` package.

`party_state` is the live state of a party member *between* encounters, seeded at import from the snapshot's `vitals.hit_points.current` and `vitals.conditions`. The handoff rule: on encounter start, each party participant's `combat_state` initialises from `party_state`; on encounter end, `party_state` is written back from `combat_state`. Party-member HP is therefore tracked continuously across a session, and never written into the snapshot or sent anywhere (§6.3 emits no HP op).

`imported_at_session` exists so staleness is measured in sessions rather than wall-clock days (§7.2, §12.1): a snapshot is stale when `campaign.current_session_number - provenance.imported_at_session >= 2`.

### 5.2 Encounters and live state

An encounter separates **definition** from **run state**, because the same encounter may be run, abandoned, and re-run.

Definition:

```
{
  encounter_id,
  name,
  status,                    // not_started | in_progress | complete | abandoned
  participants: [ { participant_id,        // ULID, minted at creation or when added mid-fight
                    kind,                  // party_member | adversary
                    member_id,             // when kind == party_member
                    adversary_id,          // when kind == adversary
                    instance_label } ]     // "Bandit 3"; display only, not an identifier
}
```

Every participant has a `participant_id` because nothing else identifies one: two instances of a statblock share an `adversary_id`, and `instance_label` is free text a GM will duplicate. `combat_state` is a map keyed by `participant_id`, `initiative_order[].participant_refs[]` holds `participant_id`s, and session events name a `participant_id` as their subject.

Run state:

```
{
  round, turn_index,
  initiative_order: [ { count, tiebreak, label, participant_refs[] } ],
  combat_state: { <participant_id>: { hp_current, hp_temp, conditions[],
                                      concentrating_on, notes, is_active } }
}
```

`conditions[]` uses the same enum as `character.schema.json` — not a parallel list. Adversary `hp_current` initialises from the statblock's `hp_max`, per instance. Dead, fled, and removed participants stay in the record with `is_active: false`.

**Initiative entries can cover more than one participant.** An entry holds a list of refs, so eight bandits on one count are eight refs under a single entry — not eight entries that happen to share a number. Splitting one out (§7.3) is then moving a ref between entries rather than a resort.

**Labels.** Each adversary instance gets an `instance_label` of `"<name> <n>"` at encounter creation — Bandit 1 through Bandit 8. A grouped entry's default `label` is `"<name> ×<count>"` — "Bandit ×8" — which is deliberately not an English plural: naive pluralisation produces "Wolfs" and "Octopuss", and the ×N form also stays correct as instances drop out. A split entry takes the instance's own label, so splitting the third bandit yields "Bandit 3" and leaves "Bandit ×7" behind. Every label is free text the GM can overwrite at any time with `r` — "Fleeing Bandit", "Boss Bandit" — and a renamed entry never has its label regenerated.

`tiebreak` is an optional integer the GM types, DEX modifier by convention. Ordering is `count` descending, `tiebreak` descending, then insertion order ascending. That third key is what makes the order total when a split entry deliberately duplicates its parent's count *and* tiebreak (§7.3), so nothing needs a magic offset to stay deterministic. An entry with no `count` yet sorts last, below every filled entry (§7.3).

**Status transitions.** `not_started` → `in_progress` the first time the GM leaves entry mode with at least one count filled (§7.3) — there is no discrete "setup finished" moment to hang it on, since blanks are legal and entry mode is re-enterable. `in_progress` → `complete` via `E` in the runner, with a `y`/`n` confirm, which also flushes `party_state` per §5.1.1. `Esc` leaves an encounter `in_progress` so it can be resumed. Starting an encounter that is already `in_progress` marks the previous run `abandoned` and clears its run state.

### 5.3 Adversaries: minimal and homebrew-first

`adversary.schema.json` is the smallest thing that lets a GM run a fight, plus an escape hatch. It is `$ref`'d from `campaign.schema.json`'s `adversaries[]` and compiled separately so `adversary add --from FILE` can validate a bare statblock — that file may hold one statblock or an array of them.

- **Required:** `adversary_id`, `name`, `ac`, `hp_max`, `speeds`.
- **Optional, structured:** ability scores, saves, resistances/immunities/vulnerabilities, condition immunities, senses, CR, `initiative_modifier`, `hp_formula`, `actions[]` (name, `to_hit`, `damage`, `save_dc`, short text), `legendary_actions[]`, `reactions[]`.
- **Escape hatch:** `text_snapshot` — Glamour-rendered markdown for everything the structured fields don't capture.

`initiative_modifier` and `hp_formula` are optional and **never used by the tool** — nothing rolls (§1). They are displayed in the detail pane as a reminder for a GM rolling by hand. Requiring them would be friction on the screen §5.3 is trying to keep frictionless.

`content_origin` and `attribution_required` are carried on every adversary, matching the character schema's provenance fields. v0 ships **zero** bundled content; every statblock is authored by the GM or imported from a file they provide (§10.2, §10.3).

### 5.4 Session events

Every mutation during a session appends to `sessions[].events[]`. An event is:

```
{ event_id,        // ULID
  kind,
  at,
  session_number,
  encounter_id,    // null outside an encounter
  round,           // null outside an encounter
  subject,         // participant_id, plus member_id when the subject is a party member
  payload }
```

`subject` carries `member_id` alongside `participant_id` so the patch fold can group by character without walking the encounter.

The log is the source of truth for patch generation. Patches are not assembled by diffing state at session end; they are **folded from the log**, so the GM can inspect exactly why a patch says what it says, and a bug in the fold is a fixable pure function over data we still have.

| Event kind | Folds to | Note |
|---|---|---|
| `xp_awarded` | `award_xp` | |
| `currency_awarded` | `award_currency` | |
| `item_awarded` | `grant_item` | |
| `item_removed` | `remove_item` | |
| `renown_awarded` | `award_renown` | |
| `feature_granted` | `grant_feature` | |
| `note` | `note` | |
| `condition_applied` | `apply_condition` | Only if still active at session end |
| `condition_removed` | — | Cancels its `condition_applied` |
| `damage_taken` | — | Drives the runner and party board; never crosses the boundary (§6.3) |
| `healing_received` | — | " |
| `temp_hp_set` | — | " |
| `concentration_set` | — | GM-side tracking only |
| `concentration_broken` | — | " |
| `participant_removed` | — | " |

`remove_condition` has no folding event: a condition that ends *between* sessions is invisible to the current session's log. In v0 it is **manual-only**, added by the GM in the review screen (§7.5). Revisit if it turns out to be common.

The event log is deliberately much richer than the patches folded from it — the GM needs the play-by-play, the player needs the consequences.

## 6. The exchange interface

This is the section v0 exists to build. Everything above is scaffolding for it.

### 6.1 The envelope

Every file crossing the player/GM boundary is a JSON document with the same envelope:

```json
{
  "envelope_version": "1",
  "kind": "character_patch",
  "payload_schema": "patch-ops.schema.json@1.0.0",
  "message_id": "01JAV8QK3M4N5P6R7S8T9V0W1X",
  "produced_at": "2026-08-17T19:40:00Z",
  "produced_by": { "tool": "dmosh", "version": "0.3.1", "role": "gm" },
  "campaign_id": "camp_44b1",
  "correlation": { "in_reply_to": null, "session_number": 21 },
  "payload": { }
}
```

`kind` is one of `character_snapshot`, `character_patch`, `party_manifest`. `produced_by.role` is `player` or `gm`. `campaign_id` may be null on a snapshot from a character not yet stamped with one. `payload_schema` is always `<schema-file>@<semver>`, matching the character document's own `schema_version` format — `character.schema.json@1.0.0` for a snapshot, `patch-ops.schema.json@1.0.0` for a patch.

The envelope is transport-shaped rather than file-shaped: `message_id`, `produced_at`, `produced_by`, `correlation`, and a discriminating `kind` are what a message needs when the same structure goes over a socket instead of a USB stick. When the LAN server lands, `POST /messages` takes this body unchanged and the file path stays as the offline fallback. Nothing in v0 produces a bare payload without an envelope.

**The envelope is the single source for `campaign_id`, `session_number`, and the payload's schema version.** Payloads do not repeat them. A version mismatch warns and degrades; it does not silently mutate.

### 6.2 Player → GM: `character_snapshot`

Payload is a complete, unmodified `character.schema.json` document, produced by `dmosh character export --format snapshot` and consumed by `dmosh gm party import`.

Import rules:

- Each file in a batch succeeds or fails independently. Failures print to stderr; exit code is 1 if any failed.
- Hard schema failure → rejected, with failing JSON Pointer paths.
- **Schema skew** (the snapshot's `schema_version` differs from the vendored one) → imported with `provenance.schema_skew: true`. The party board marks the row, and any `cached` field the tool couldn't find renders as `—` (§5.1.1). Nothing about a snapshot is ever writable, so "read-only" would have meant nothing here.
- A snapshot whose `_id` already exists in the roster → error naming `party refresh`.
- Two members sharing a `display_name` → both import; `member_id` disambiguates and `party list` shows both.
- An envelope naming a different `campaign_id` → warn and import.
- `gm init` against an existing `campaign.db` → refuse unless `--force`.

`party refresh <member> <file>` replaces one member's snapshot and appends to `import_history[]`. `<member>` resolves as `member_id` first, then a case-insensitive unique `display_name` match; an ambiguous name errors and lists candidates. If the incoming `_id` differs from `provenance.source_character_id`, refuse: *"that snapshot is a different character (`chr_x` vs `chr_y`); use `party import` to add it as a new member."*

Sending whole documents rather than a reduced "GM view" is a v0 simplification with a real cost (the GM sees backstory and gold) and a real benefit (zero new schema). Whether players want redaction is a §11 feedback question.

### 6.3 GM → player: `character_patch`

```json
{
  "patch_id": "01JAV8QK3M4N5P6R7S8T9V0W1X",
  "target_character_id": "chr_7f3a91",
  "observed_doc_revision": 14,
  "summary": "Session 21: 2 encounters, 450 XP, 75 gp, 1 item",
  "ops": [
    { "op_id": "01JAV8QK4A...", "kind": "award_xp",
      "amount": 450,
      "reason": "Session 21 award", "source_event_ids": ["01JAV8Q..."] },

    { "op_id": "01JAV8QK4B...", "kind": "award_currency",
      "coins": { "gp": 75 }, "counterparty": "Bandit camp strongbox",
      "reason": "Strongbox, split 4 ways", "source_event_ids": ["01JAV8Q..."] },

    { "op_id": "01JAV8QK4C...", "kind": "grant_item",
      "name": "Potion of Healing", "quantity": 2, "item_payload": null,
      "reason": "Loot from the bridge fight", "source_event_ids": ["01JAV8Q..."] },

    { "op_id": "01JAV8QK4D...", "kind": "award_renown",
      "group_ref": { "group_id": "grp_ee", "group_name": "Emerald Enclave" },
      "delta": 2, "reason_category": "assigned_mission",
      "reason": "Cleared the blightwood", "source_event_ids": ["01JAV8Q..."] },

    { "op_id": "01JAV8QK4E...", "kind": "note",
      "text": "The Consortium now knows you have the ruby.",
      "source_event_ids": ["01JAV8Q..."] }
  ]
}
```

`observed_doc_revision` is copied from the snapshot's `doc_revision` (stored as `provenance.source_doc_revision`, §5.1.1).

**Ops name intentions, not locations.** No op contains a JSON Pointer, a field name, or any other reference to the character document's shape. `award_xp` says a character earned 450 XP; where XP lives, whether the table uses milestone advancement and should ignore it, and what else must change as a consequence are the *receiver's* business. This buys three things:

- **The character schema can move without breaking the GM tool.** Rename the XP field, restructure inventory, split currency — the character tool updates its own handlers and every existing patch file still applies.
- **The receiver owns its own invariants.** Currency changes append to `/currency/ledger` and recompute `/currency/derived`; a granted item implies an item instance, an inventory entry, and a weight recompute. The character tool's `grant_item` handler does what its own add-item path does, derive pass and log append included. None of it is the patch's problem.
- **The write-side contract stays small enough to publish later.** Nine op kinds, not "our character schema, which you must also implement." v0 makes no stability promise (§10.7); the point is that the contract is the right size and in the right place.

#### The v0 vocabulary

| Kind | Payload | Notes |
|---|---|---|
| `award_xp` | `amount` | Receiver may no-op on milestone-advancement characters |
| `award_currency` | `coins{cp,sp,ep,gp,pp}`, `counterparty` | Signed; never normalized |
| `grant_item` | `name`, `quantity`, `item_payload` | `item_payload` is opaque — see below |
| `remove_item` | `item_ref`, `quantity` | Consumed, stolen, destroyed |
| `apply_condition` | `name`, `level`, `source`, `duration` | `name` uses the character schema's condition enum; `level` for Exhaustion only |
| `remove_condition` | `name` | Manual-only in v0 (§5.4) |
| `award_renown` | `group_ref`, `delta`, `reason_category` | See below |
| `grant_feature` | `name`, `text_snapshot`, `source` | Story boons, charms, blessings |
| `note` | `text` | No mechanical change; lands in the character log only |

Every op also carries `op_id` (ULID), a human-readable `reason`, and `source_event_ids`. An op the GM added by hand in the review screen has `source_event_ids: []` and renders as "manually added".

**Reference shapes.** `item_ref` is `{entry_id, name}` and `group_ref` is `{group_id, group_name}`; the id may be null, the name may not, and at least one must resolve. Ids are copied verbatim from the snapshot the GM holds (`inventory.entries[].entry_id`, `reputation.groups[].group_id`). The receiver prefers the id, falls back to the name, and marks the op **unresolvable** in review if neither matches — never failing the patch, never guessing. Stale ids are expected: the GM's snapshot is by definition older than the document being patched.

**`award_renown` specifics.** If the named group isn't on the character, the receiver offers to create it with `tracking_mode: score` and `renown_score: 0` before applying. A `delta` that would take the score below zero clamps to zero (the 2024 floor) and the review UI says so. `reason_category` is `advanced_interests | assigned_mission | significant_quest | downtime | offense | other`.

**`award_currency` sign.** Negative values are legal and are how a GM records a cost paid at the table; the receiver appends an `expense` ledger entry for a negative total and `income` for a positive one. Mixed-sign coin maps within one op are rejected at validation.

**Free-text fields.** `apply_condition.duration`, `apply_condition.source`, and `grant_feature.source` are display strings, shown verbatim and never parsed — v0 expires nothing on its own (§1).

**`grant_item.item_payload` is opaque.** It is `null`, or an arbitrary JSON object the GM tool stores, forwards, and displays by `name` but never inspects or constructs field-by-field; `patch-ops.schema.json` types it `{"type": ["object","null"]}` and stops. The receiver validates it against its own item schema on apply and rejects that single op with a readable reason if it doesn't fit. Typing it against the character domain's item schema would have reintroduced exactly the coupling semantic ops exist to remove. In practice `null` with a name and quantity is the common case; a populated payload is for a homebrew item the GM has already authored.

**What the vocabulary deliberately omits.** No HP op and no resource op. The rule: **the GM proposes what the GM knows and the player doesn't** — awards, loot, renown, story boons — *plus* state the GM's tool authoritatively tracked during a fight, which is why conditions stay. HP and class-resource spends fail both tests: the player watched them happen and tracks them on their own sheet. Tables that need the carryover use `note` ("you're at 19/34 going into next session"), and how often GMs write that note is §11.4's signal for which op earns its way in next.

**Unknown kinds degrade, they don't fail.** A receiver meeting an op kind it doesn't know skips it with a visible warning — *"1 op skipped: unknown kind `award_downtime`; update your character tool"* — applies the rest, and records it `skipped` with `detail: "unknown kind"`, so re-applying the same file after an upgrade re-offers it.

**The cost, stated plainly.** The GM tool cannot express an arbitrary change. When the vocabulary doesn't cover something, the GM writes a `note` and says it in words — a real limitation, and the best instrument we have for knowing what to add (§11.4). A general escape hatch would have hidden that signal.

#### Patch lifecycle and identity

`session end` generates one patch per character from the event log, mints `patch_id` and every `op_id` **once**, and stores them in `outbound_patches[]` with `status: pending`. Review-screen edits (§7.5) mutate the stored ops in place, preserving every id; adding a manual op after export mints a new `op_id` inside the same `patch_id` and returns the patch to `pending`. `patch export` only serialises what's stored and flips `pending` → `exported`; re-exporting writes a byte-identical file. This is load-bearing — regenerating with fresh ULIDs would let a player apply 450 XP twice, the exact failure the design exists to prevent.

`patch export` writes one file per character, `patch_<campaign>_s<session>_<member_id>.json`, into a directory the GM shares however they like. `--session` defaults to the most recently ended session; an existing file is refused without `--force`. `acknowledged` exists in the schema with no v0 mechanism to set it — acknowledgement is a sync-era concept, and the state machine shouldn't have to change shape later.

#### Three properties make patches safe

**Idempotency, tracked per op.** Applying a patch appends one entry to the **character log** — the non-build log the character subsystem plans for quest rewards, XP, and permanent buffs:

```
{ patch_id, campaign_id, session_number, applied_at, source_tool_version,
  resolutions: [ { op_id, kind, status: applied|skipped|unresolvable, at, detail } ] }
```

There is no `applied_patches` array; the log is the only record, so the two can't disagree. **This is a change to `character.schema.json`** — that document declares `additionalProperties: false` and today carries only `build_log`, whose enum covers build decisions. Adding the log and its `$defs` entry is owned by the character subsystem and is a Phase 1 dependency of this tool (§9).

Re-applying the same file is therefore not a refusal but a **re-offer of the unresolved remainder**: `patch apply` scans the log for the `patch_id`, subtracts every op marked `applied`, and presents what's left — ops skipped last time, plus ops that were unresolvable then and may resolve now. Previously applied ops are never shown again. If nothing remains, the tool says so and exits cleanly. This matters because "I'll deal with the loot later" is the normal case: a player takes their XP and gold at the table and leaves four inventory ops until they're at a desk.

**Advisory, not authoritative, revision.** `observed_doc_revision` records what the GM last saw. It does not gate application. If the player has edited since, the review UI says so and lets them decide. A patch that could be rejected on staleness would make the GM an authority over the player's document.

**Mandatory human review.** `patch apply` is interactive: every op renders as a before → after line with its `reason`, accepted or skipped **per op**. Partial application is a first-class outcome. There is no `--yes` flag in v0 — this is the schema report's "no silent mutation of a live sheet" applied to the exchange layer, and the cheapest answer to every trust question about letting someone else write to your sheet.

### 6.4 GM → players: `party_manifest`

A small payload naming the campaign, its `campaign_id`, ruleset revision, and roster display names. Written by `dmosh gm party manifest --out FILE`, consumed by `dmosh character manifest apply <file>`, which sets `campaign_id` on the character and nothing else. Its v0 job is making future player exports self-address; it also proves the multi-recipient broadcast shape before the server needs it.

## 7. Screen inventory

Five screens. The character tool needed eleven because it models a whole paper sheet; a session runner is one dense screen plus support.

### 7.1 Session dashboard (home)

Campaign name and current session number; a party strip (name, class/level, HP bar from `party_state`, active conditions, stale-snapshot count); encounters with `status`; a footer showing pending patch count. If a session is open and an encounter is `in_progress`, `Enter` resumes it from anywhere — the tool's most common action should be one key from the top.

`w` opens the **award form** (§7.2), reachable here so a GM can award between encounters without leaving the dashboard.

### 7.2 Party board

One expandable row per member. Collapsed: name, AC, HP current/max, passive Perception, active conditions, concentration. Expanded: saves, the skills a GM actually calls for (Perception, Insight, Stealth, Investigation) as passive and modifier, speeds, defenses, senses, languages, and the character's `identity.roleplay_notes[]` if present. Read-only and styled as such, with import time and `doc_revision` in the footer.

**Stale snapshots are flagged, not just dated.** A row stale by §5.1.1's rule carries an inline marker — "imported 3 sessions ago" in the warning style — and `dmosh gm party list` shows the same flag in a column, so a GM can check before play without opening the TUI. The failure mode is quiet: a GM reads an AC that changed two levels ago and rules against a player on it. The nudge never blocks and never auto-refreshes; the GM's copy changes only when a player sends a new snapshot.

Write paths on this screen:

- `d`/`h`/`t` — damage, healing, temp HP against `party_state` outside combat, appending events.
- `c` — apply/remove a condition.
- `w` — the **award form**: a `huh` sequence of award kind (XP / currency / item / renown / feature / note), recipients (multi-select over the roster, defaulting to all), the kind's fields, and a reason. It writes **one event per recipient**, which is what makes §6.3's worked example producible: without it, nothing in the tool emits `xp_awarded`, `currency_awarded`, or `item_awarded` at all.

### 7.3 Encounter runner

The core screen and the one worth prototyping first. Three panes at wide terminals, stacking to one at 80 columns:

- **Initiative pane** (left): the ordered list, current turn highlighted, round counter on top.
- **Detail pane** (right): the selected combatant — statblock with actions and `text_snapshot` for adversaries, the party board's read-only vitals for members.
- **Action bar** (bottom): `d` damage, `h` heal, `t` temp HP, `c` toggle condition (filtered picker over the condition enum, Exhaustion taking a level), `k` concentration set/break, `x` mark removed/dead, `Space` note, `w` award.

Every action appends a session event and commits synchronously. Damage entry is a bare number field with `-`/`+` — this input happens dozens of times per fight, and friction here is the whole product's friction.

**Initiative is typed in, never rolled.** Players roll their own initiative — the same line §1 draws around rules automation. **Instances of the same statblock share one count by default**: eight bandits act together on one entry, which is standard table practice and keeps the pane readable at 80 columns.

**Entry happens in any order.** Encounter start puts the initiative pane into *entry mode* rather than running a wizard: every row is editable, the GM moves between them freely, and a header counts how many are still blank. This is not a keystroke optimisation — it's the only shape that matches a table, where numbers arrive in whatever order people speak up rather than in roster order. "Dana, what did you get?" — "17" — and the GM types it against Dana's row wherever it sits.

Blanks are legal. An encounter can start with rows unfilled; a blank sorts last, renders as `—`, and can be filled at any point, which is what a late arrival or a distracted player actually looks like. Entry mode is left with `Esc` and re-entered with `i`, so correcting a number mid-fight is the same interaction as entering it — there is no separate setup phase to be outside of.

Navigation and edge rules, all of which an implementer would otherwise have to invent:

- `n`/`p` advance and rewind the turn. Rewind matters because tables reorder and correct constantly. `p` at round 1, turn 0 is a no-op with a status-line message.
- `n` skips entries whose participants are all `is_active: false`.
- `i` enters entry mode on the selected row; on a grouped entry it moves the whole group.
- `s` splits the selected instance onto its own entry at the parent's `count` and `tiebreak`, then selects it with the count field focused and pre-filled. `Enter` accepts the parent's number, typing overrides it — a conservative default with a zero-friction override, rather than a choice between the two. No magic offset is needed to keep the order deterministic: §5.2's third sort key is insertion order, and the split entry was inserted later, so it lands after its parent.
- `r` renames the selected entry.
- `a` adds a combatant mid-fight at a typed count; it acts this round only if its count sorts below the current turn's.
- `x` on the *current* participant advances to the next entry.
- `E` ends the encounter with a `y`/`n` confirm, writing `status: complete` and flushing `party_state` (§5.1.1).
- `Esc` exits entry mode when in it, and otherwise navigates back, leaving the encounter `in_progress` for later resumption. Ending an encounter is always deliberate; nothing completes an encounter by walking away from it.

Concentration is a prompt, not automation: when a concentrating combatant takes damage, the tool surfaces "Concentration check: DC 10 or half damage (DC 12)" and waits. It computes the DC because that's arithmetic; it does not roll, resolve, or drop concentration.

### 7.4 Adversary library

A Bubbles `list` of statblocks with a Glamour detail pane, plus a `huh` form for add/edit and import from JSON. Nothing more: no cross-source search, no catalog, no CR filtering worth the name.

### 7.5 Session review & patch queue

Reached by `dmosh gm session end` or from the dashboard. Left pane: the session event log grouped by character. Right pane: the generated patch for the selected character, op by op, each showing its source events.

The GM can edit an op's `reason`, drop an op (a death the party healed back, a loot award that got re-split), or add a manual op — a `huh` form whose first field picks from the §6.3 vocabulary, the rest generated per kind. All edits mutate stored ops in place, preserving ids (§6.3). This is where the GM's judgment lands before anything reaches a player, and it's why the fold has to be inspectable rather than clever.

## 8. Cross-cutting behaviors

### 8.1 Validation

Four schemas compile at startup: `campaign.schema.json`, `adversary.schema.json`, a vendored copy of `character.schema.json` (for imports), and `patch-ops.schema.json` (for emitted patches). Snapshots validate before entering the roster, adversary imports before insertion, patches before export, and the campaign document on every save.

The GM tool validates against the character schema **only on the read path** — a direct consequence of §6.3, and a useful check that the decoupling holds: if it ever needs that schema to write something, the vocabulary is missing an op.

A validation failure on import is a rejection with JSON Pointer detail, not the character tool's read-only-open fallback — an unparseable party member is not something a session degrades into gracefully.

### 8.2 Keybindings

Inherits the character tool's §8.4 conventions: `?` for context help, `Esc` back, `Ctrl+S` save, inline `y`/`n` confirms for destructive actions, per-screen `key.Binding` sets generating the help overlay. The encounter runner is the one place where speed beats consistency and single unmodified letters are spent freely.

### 8.3 Persistence

Same storage shape as the character tool §9: SQLite, one document per row as JSON text, generated columns for anything a list query needs, numbered embedded migrations.

```sql
CREATE TABLE campaign (
    id             TEXT PRIMARY KEY,
    document       TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    doc_revision   INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    campaign_name  TEXT GENERATED ALWAYS AS (json_extract(document, '$.campaign.name')) STORED
);

CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
```

One campaign per database file in v0 — a table holding one row looks odd, but it keeps many-campaigns-per-file open without committing to the UI that would need.

Encounter mutations commit immediately rather than on the character editor's 2-second debounce (§2). WAL mode is on by default, unlike the character tool's §12.6 default, because `gm show`/`validate` running against a database the runner has open is a normal GM workflow.

`dmosh gm export --format json` writes the whole campaign document to a file, so nothing is trapped inside a database.

## 9. Phased delivery

**Phase 1 — the exchange spine.** `gm init`, `party import`, `party list`, the party board and award form (§7.2), character-side `export --format snapshot`, and the patch path end to end: `patch export`, `character patch preview`, `character patch apply`.

Two dependencies gate this phase, both on the character subsystem:

1. **The character log** (§6.3) must exist, since idempotency has nowhere to live without it.
2. **Only five of nine ops have a receiving surface in the character tool's own Phase 1** — `award_xp`, `award_currency`, `apply_condition`, `remove_condition`, `note`. `grant_item`, `remove_item`, and `grant_feature` need its Phase 2 screens; `award_renown` needs reputation, which that spec puts out of v1 scope entirely. The GM side emits all nine correctly; the remaining four land as `unresolvable` until those screens exist, and the re-offer mechanism (§6.3) means they apply later without the GM regenerating anything. Plan Phase 1 acceptance against the five, not the nine.

**Phase 2 — the runner.** Encounter definition, adversary library, the runner screen, session events, real patch folding. This is where the interaction-design risk lives.

**Phase 3 — review polish.** The patch queue's editing affordances, `party_manifest`, campaign export/import.

Sync, prep, and everything in §1's out-of-scope list wait for §11's feedback.

## 10. Choices this document deliberately does not make

Recorded so that "we haven't decided" reads as a decision rather than an oversight:

1. **Whether the toolkit owns campaign prep at all.** A bad prep model is stickier than no prep model.
2. **Content licensing and bundled catalogs.** SRD 5.2 under CC-BY-4.0 makes a bundled adversary catalog legally possible, at the cost of attribution obligations and a content pipeline. Homebrew-first defers it.
3. **Statblock fidelity.** §5.3's minimal shape plus `text_snapshot` is a probe, not a model — we don't yet know whether GMs author statblocks here or keep a PDF open and need only AC and HP.
4. **Whether the GM ever holds authoritative state.** v0's answer is a hard no. The sync milestone might want the GM authoritative for *party-level* documents — treasury, shared loot, party renown — which the schema report flagged as wanting their own aggregate root. v0 creates no party aggregate, so it stays open.
5. **Sync transport, discovery, and identity.** The envelope constrains payloads and nothing else.
6. **Redacted export profiles.** §6.2 sends whole character documents; whether players want to withhold parts is a §11 question.
7. **Any stability guarantee on the op vocabulary.** No compatibility promise, no deprecation policy, no commitment that an op kind means the same thing next month — it should churn as §11.4's feedback arrives, and freezing it now freezes in our guesses. The one forward-compatibility d v0 does implement is unknown-kind skipping, because it costs nothing. Revisit when the op list stops changing between releases, which is itself the signal the domain is understood well enough to publish.

## 11. What v0 is built to find out

There's no telemetry, so feedback is a deliberate artifact. `dmosh gm feedback --out FILE` writes a local, reviewable markdown summary from the campaign document's `usage` block (§5.1) and its own contents — sessions and encounters run, average combatants, screen entries, patches generated and of which op kinds, how many statblocks used only `text_snapshot`, and the text of `note` ops. The GM reads it in full and chooses whether to share it. Nothing leaves the machine on its own.

The README carries the questions v0 is designed to answer:

1. Do GMs run combat in a terminal at all, or is the party board the whole value?
2. Is file-based exchange tolerable in practice, or does the friction show up before we ship sync? Specifically: does the GM ask for fresh snapshots mid-campaign, and do they arrive?
3. Is per-op review the right granularity, or do players want "accept all" after the first session of trust?
4. **Which ops get used, and what's missing?** `note` is the instrument: every time a GM writes a note describing a mechanical change instead of using an op, that's a missing entry in the vocabulary in the GM's own words.
5. Do GMs author statblocks here, or enter a name, AC, and HP and keep a PDF open?
6. Does the read-only-snapshot boundary hold, or is there a category of change GMs expect to make directly to a character?
7. Do players want to redact anything before sending a snapshot?

Question 2 is load-bearing. If file exchange proves fine for a whole campaign, sync drops down the roadmap and Phase 3 becomes prep. If it breaks down in week two, sync is next — and the payload formats are already built and exercised.

## 12. Open questions

One, and it doesn't gate anything.

1. **The staleness threshold.** §5.1.1 flags a snapshot stale at two sessions since import. Sessions is the right unit — wall-clock days is wrong for a group that plays monthly, and `doc_revision` delta would be accurate but is unobservable to the GM without the very snapshot they're trying to prompt for. The `2` is a placeholder: too low and the marker is always on and becomes wallpaper, too high and it stops preventing the bad ruling it exists to prevent. Set it once someone has run a campaign and can say when they actually felt misled. If the session proxy turns out to be noisy, the obvious refinement is counting patches exported since import — the GM caused those changes, so it's a certainty rather than a guess — but that's a second measure for a problem v0 hasn't yet shown it has.