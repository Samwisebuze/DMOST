---
id: ARD-0002
title: Align the character stack with PSD-0001, breaking it where needed
updated: 2026-08-19
status:
  - kind: proposed
  - version: v0.4
narrows:
  - ARD-0001 decision 7 — merge patches remain the wire's write shape, not the TUI's
adopts:
  - ARD-0001 appendix — CharacterRepository.List, widened past what it proposed
related:
  - docs/psd/0001_character-management-tui.md
  - internal/infra/sqlite/migrations/000001_create_characters_table.up.sql
  - pkg/app/service.go
---

# ARD 0002 — Align the character stack with PSD-0001, breaking it where needed

## Status

**Proposed.** No code exists for the TUI yet, and the code that does exist changes under this record.
It reaffirms ARD 0001 decision 3 — `internal/infra/sqlite` behind the repository port — and narrows
decision 7 rather than reversing it.

**Revisions.**

- **v0** — initial draft. Had the TUI own a private `internal/store`, bypassing `pkg/domain`,
  `pkg/app` and `internal/infra/sqlite`, and withdrew ARD 0001's proposed `List` port method.
- **v0.1** — review pass on that design: row columns as projections, `STRICT`, and a guard against two
  `characters` tables meeting in one file.
- **v0.2** — **reframed, reversing v0's central decision.** v0 left the repository with two persistence
  stacks and the existing one serving only `dmostd`. `dmostd` is speculative — no users, no deployed
  database, an in-memory repository that loses everything on exit — so the stack is the thing worth
  keeping and `dmostd`'s current shape is not. This record now proposes the breaking changes that align
  `pkg/domain/character`, `pkg/app` and `internal/infra/sqlite` with PSD-0001. v0.1's projection
  finding survives inverted (§8); its two-tables guard is moot and dropped.
- **v0.3** — §9 narrowed by ARD 0010, which freezes `dmostd`. The daemon stays wired to `inmem` but
  stops obliging it to implement the whole widened port; `repotest`'s full contract runs against
  `sqlite`. Also corrects the port's method count — seven rows, nine methods.
- **v0.4** — realigned to ARD 0006 v0.3. `ReplaceCharacter` carries a required `Summary`, because the
  TUI is not the only caller that snapshots, and `restore` is two writes rather than one now that it
  captures the state it is about to replace (§2, §3).

## Context

PSD-0001 §5.3 and §9 describe an `internal/store` package that "owns all persistence; no other package
issues SQL", loading with `SELECT document FROM characters WHERE id = ? AND deleted_at IS NULL` and
saving with a whole-document `UPDATE` under a trailing revision predicate. Nothing in that path passes
through `pkg/domain`, `pkg/app`, `internal/dto/v1alpha/mapper`, or `internal/infra/sqlite`.

Read as a specification of *storage behaviour* it is right, and this record adopts nearly all of it. Read
as a specification of *package structure* it would fork the repository, because the behaviour it asks for
is a superset of what the existing stack does rather than a different thing:

| PSD-0001 wants | the stack has | gap |
| --- | --- | --- |
| a JSON document stored verbatim, opaque to the layer above | exactly that — `characters.data TEXT NOT NULL CHECK (json_valid(data))` | none |
| optimistic concurrency on a revision | `Save` is a compare-and-set on `common.Aggregate.Version`; a stale write is `common.ErrConflict` | naming only |
| enumeration for a picker, sorted by name, without decoding every row | `CharacterRepository` is `Save` and `Find` | a port method and a projection |
| soft delete, `undelete`, `purge` | nothing | port methods and a column |
| snapshots written in the save transaction | nothing | a table and a save option |
| item instances sharing the character's revision | nothing | an entity and an aggregate boundary |
| a whole-document write that skips schema validation (autosave) | `Update` always gates on the schema | a use case |

Seven rows, five of which are additive. That is a change list, not an incompatibility.

**What decides it is that nothing has shipped.** `cmd/dmostd` wires `inmem`, so every character the
daemon has ever stored died with the process; `internal/infra/sqlite` is wired into nothing at all; the
only clients of the HTTP API are its own end-to-end tests. There is no deployed database, no external
consumer of `v1alpha` (the `internal/` prefix guarantees it), and no user whose data a breaking change
could damage. Breaking changes to this stack are as cheap right now as they will ever be, and the cost
of a second stack — two persistence layers, two migration runners, two answers to "how is a character
stored" — is permanent.

`pkg/app`'s own doc comment already anticipates the pressure the TUI applies. On `UserService.Create`:

> That is deliberate **for now**: the mapper is where request validation already lives […] It does mean
> this layer is pinned to v1alpha. When a second version lands, the fix is a version-neutral command
> struct per use case, with each dto version mapping into it — not a second Create method.

A second *consumer* arrives before a second version does, and it applies the same force: a TUI holding a
decoded document should not have to construct a `v1alpha.UpdateCharacterRequest` to save it.

## Decision

**`internal/store` is not built. `pkg/domain/character`, `pkg/app` and `internal/infra/sqlite` are
extended to meet PSD-0001's requirements, breaking their current shape where that is what it takes.**

```
cmd/dmosh  →  pkg/tui  →  pkg/app (command structs)  →  pkg/domain/character
                                        ↑                        ↑
cmd/dmostd →  pkg/http  →  internal/dto/v1alpha/mapper    internal/infra/{inmem,sqlite}
```

### 1. The aggregate boundary becomes character-plus-items

D20 makes a character and its item instances one conflict domain under one revision. That is an
aggregate boundary, so it is stated in the domain rather than implied by a transaction.

`ItemInstance` becomes an entity in `pkg/domain/character` — an identity and an opaque document, the same
shape `Character` already has — and `Character` gains an item set. `Save` persists the whole aggregate;
one revision covers all of it; `common.ErrConflict` means the same thing it means today.

**Lazy loading is expressed in the aggregate, not worked around it.** §7.9 loads an item on first
selection, so a `Character` returned by `Find` has its items *unloaded*, which is a third state distinct
from "loaded and empty". A `Save` of a Character whose items are unloaded leaves the stored items
untouched; a `Save` of one whose items are loaded replaces them. That rule is a property of the port, so
it belongs in `internal/test/repotest` where `inmem` and `sqlite` are both held to it.

This is the sharpest cost in the record, and it is worth naming as a cost: an aggregate whose save
semantics depend on whether part of it was loaded is a subtlety that will catch someone.

### 2. The port grows from two methods to nine (seven rows below)

| method | why |
| --- | --- |
| `Save(ctx, *Character, ...SaveOption)` | as today, plus `WithSnapshot(summary)` so the revision row is written in the same transaction (ARD 0006) |
| `Find(ctx, CharacterID)` | as today; items unloaded |
| `FindItems(ctx, CharacterID, ItemInstanceID)` | the lazy fetch behind §7.9's detail pane |
| `List(ctx, ListOptions)` | enumeration for the picker and `dmosh character list`; `ListOptions` carries `IncludeDeleted` and the ordering |
| `SoftDelete` / `Undelete` | D8, the `deleted_at` transition |
| `Purge(ctx, CharacterID)` | the only real `DELETE`, and the only one that cascades |
| `Revisions(ctx, CharacterID)` and `FindRevision(ctx, CharacterID, selector)` | ARD 0006's history and the snapshot a restore reads |

**`restore` is orchestration in `pkg/app`, not a port method.** It snapshots the current document, reads
the target revision, splices `play_log` from the current document per ARD 0004 §5, and calls `Save` —
which writes forward as a new revision and carries the snapshot's item set. Two writes (ARD 0006 §4),
each complete on its own, so no transaction spans two port calls; and the splice stays above the
adapter, because preserving one JSON field is schema knowledge and §8 is the only place this record lets
that into a storage layer.

### 3. `pkg/app` takes version-neutral commands

The service interfaces stop taking `v1alpha` request types and take command structs owned by `pkg/app`.
`internal/dto/v1alpha/mapper` maps a request into a command; `pkg/tui` builds one directly.

This is the change `pkg/app`'s doc comment names, arriving for the reason it predicted. It is also the
largest breaking change here: every method on `CharacterService` and `UserService` changes signature, and
`pkg/http`, `cmd/dmostd`, `pkg/http/user_test.go`'s `FakeUserService`, and the mapper all change with
them.

Two write use cases rather than one flag, because ARD 0007 needs them to differ in kind:

| use case | schema gate | snapshot |
| --- | --- | --- |
| `SaveCharacterDraft` — the 2s debounced autosave | none; the document must decode, nothing more (ARD 0007 §7) | no |
| `ReplaceCharacter` — `Ctrl+S`, session end, `import`, wizard completion, `restore`, `recompute` | full, with JSON Pointer paths | yes, always |

`ReplaceCharacter`'s command carries a **required** `Summary`, since snapshotting is not optional on it
and five call sites reach it — only one of which is the parent model that can compose one from
mutations. ARD 0006 §8 has the strings and the reason a nullable field would not survive contact.

`Patch` survives unchanged in purpose: it is the wire's partial-write shape, and ARD 0001 decision 7 is
narrowed rather than superseded — merge patches stay for HTTP, and the TUI writes whole documents because
it holds the whole document already.

### 4. Validation becomes the compiled schema, and `validateSheet` is replaced

`mapper.validateSheet` decodes into the generated type and discards the result. Per ARD 0003 that decode
is no longer the gate: the gate is `santhosh-tekuri/jsonschema` compiled from the vendored schema, which
is the only thing that produces the JSON Pointer paths PSD-0001 §6 promises and ARD 0007 §4 renders.

The consequence reaches the wire: `pkg/http/problem`'s `reason` for an invalid sheet becomes a list of
pointers instead of a Go type error, which is strictly better and is still a breaking change to what a
client sees.

### 5. `version` stays authoritative; the document's copies are written at the boundary

**This inverts v0.1 §7.** That revision had the document win, because in v0's design the row's columns
were the only other copy and nothing owned them. Here `common.Aggregate.Version` owns the revision — set
inside the adapter's critical section through `CharacterFactory.NextVersion`, exactly as
`inmem.Save` already does it — so the aggregate wins.

`_id`, `created_at`, `updated_at` and `doc_revision` inside the sheet become projections written at the
mapper boundary from the aggregate, on the way out. There is no generated column for `doc_revision`: the
existing `version` column is authoritative and already indexed by the primary key's rowid. `deleted_at`
stays a column with no document counterpart, deliberately — soft deletion is a fact about this store's
row, and a deleted character exported and imported elsewhere is not deleted there.

This keeps `mapper.serverOwnedSheetKeys` honest: those fields are refused from clients precisely because
something else owns them, and now something does.

### 6. Migrations are additive, and SQLite's rules make that possible

`000001_create_characters_table.up.sql` is not edited — CLAUDE.md's rule stands, and the table it creates
is already the right shape for a document store. What follows are new numbered pairs:

- `deleted_at TEXT` — `ALTER TABLE ADD COLUMN`, fine on a `STRICT` table.
- the listing projections — `character_name`, `total_level`, `ruleset_rev`, `campaign_id` — as
  **`VIRTUAL`** generated columns over `json_extract(data, …)`, each with an index.
- `character_revisions` (with ARD 0006 §7's `restored_from`) and `item_instances`, both with
  `ON DELETE CASCADE` and both `STRICT`.

`VIRTUAL` rather than `STORED` is forced and worth knowing: **SQLite's `ALTER TABLE ADD COLUMN` cannot
add a `STORED` generated column**, so `STORED` would mean a twelve-step table rebuild — new table, copy,
drop, rename — to gain a column whose value is already indexed. Virtual generated columns are indexable,
and an index over one stores the extracted value, so the query plan for
`ORDER BY character_name` reads the index rather than re-extracting per row. The startup budget in §2 is
met either way.

PSD-0001 names the column `document`; it is `data` here. The name is not a requirement, and renaming a
shipped column to match a spec's prose is churn.

### 7. `foreign_keys`, `WAL` and `busy_timeout` are already the adapter's rules

`internal/infra/sqlite` already folds per-connection settings into the DSN rather than issuing an `Exec`
— because an `Exec` after `sql.Open` configures one pooled connection and silently misses the rest — and
already documents `ForeignKeys` as the constraint that must be on. D11 asks for exactly that set. What
changes is the composition root's configuration, not the adapter: a file DSN under ARD 0008's XDG path,
`WAL`, and a non-zero `busy_timeout`.

`foreign_keys` stops being hygiene and becomes load-bearing the moment §6's cascades exist: without it
they silently do nothing and `purge` orphans rows.

### 8. The one crack in the opacity rule: `CharacterSummary`

ARD 0001's appendix argued that `List` must return full aggregates, because a projection "would require
the repository to know the sheet's shape, which is the one thing `pkg/domain/character` is built not to
know" — and said that if enumeration ever became slow enough to matter, the move would be a summary type
populated by `json_extract`, "a decision worth its own record, because it puts schema knowledge into a
storage adapter for the first time".

**This is that record, and the decision is to do it now rather than when it hurts.** PSD-0001 §2 budgets
under 100ms to first paint against hundreds of rows, and explicitly rules out re-parsing every row's
document to populate a picker. Returning full aggregates and decoding them in Go is the thing that
budget forbids.

So `List` returns `[]CharacterSummary` — identity, version, timestamps, deleted state, plus
`character_name` and `total_level`. It is read-only, never a write path, and it is the single place where
this domain admits what is inside a sheet. `sqlite` populates it from §6's generated columns; `inmem`
decodes those two fields; `repotest` holds both to the same answers.

The cost is exactly the one ARD 0001 named: `pkg/domain/character` now has a type that changes if the
schema's `identity` or `progression` sections change. Two fields, one direction, one documented
justification — bounded, but real, and this record is where anyone widening it should have to argue.

### 9. `dmostd` stays on `inmem` — and, under ARD 0010, stops paying for the port

`dmosh` wires `sqlite`, `dmostd` keeps `inmem`. That is ARD 0001 decision 2's reasoning unchanged: two
composition roots wiring different backends keep both implementations exercised above the level of the
contract tests.

**v0.3 narrows what follows from that.** v0.2 concluded that `inmem` therefore implements all of the
port, soft delete, revisions and the lazy item semantics of §1 included, and called that the right work
because CLAUDE.md treats `inmem` as the reference implementation of the versioning rule rather than a
test double. ARD 0010 freezes `dmostd`, which removes the premise: the daemon reaches two of the nine
methods this record adds, so the remaining seven were never going to be kept honest by a second
composition root — only by `repotest`, which is the level the argument claimed to reach past.

What survives is the narrower and truer half. `inmem` must satisfy the interface or nothing compiles,
and it keeps full semantics for `Save` and `Find` — which is where the versioning rule lives, so its
status as the reference implementation is intact. Beyond those it implements what something actually
calls, and says so explicitly where it does not. ARD 0010 §2 has the rule and its cost.

## Consequences

- **One persistence stack, one aggregate, two applications.** The alternative — v0's design — is now on
  record as considered and rejected, which is what the series is for.
- **A wide breaking change, landing across:** `pkg/domain/character` (aggregate, port, `Rehydrate`
  signature, new entity, `CharacterSummary`), `pkg/app` (command structs, two new use cases),
  `pkg/app/services`, `internal/dto/v1alpha/mapper` (request→command, compiled-schema validation),
  `internal/infra/inmem`, `internal/infra/sqlite`, `internal/test` and `internal/test/repotest`,
  `pkg/http`, and `cmd/dmostd`. Everything the daemon exposes keeps working; almost every signature
  behind it changes.
- **`pkg/app` stops being pinned to `v1alpha`**, which is a benefit taken as a side effect rather than
  the goal, and it costs every call site.
- **`pkg/domain/character` is no longer entirely opaque** (§8). Two fields, in one read-only type.
- **An aggregate with a partially-loaded child set is a subtle object** (§1), and the port contract is
  the only thing that will keep the two adapters agreeing about it.
- **PSD-0001 §5.3 and §9 are now inaccurate as written** — there is no `internal/store`, the column is
  `data` not `document`, and the revision predicate is the aggregate's `version`. The spec should be
  revised to match; its *behaviour* is adopted essentially whole, which is why this is a documentation
  fix rather than a disagreement.
- **`usage_counters` (§12) has no aggregate and does not get one.** It is `dmosh`-local, never leaves the
  machine, and the composition root can hand the TUI a small counter store from `internal/infra/sqlite`
  without a domain port. `catalog_entries` is Phase 2b, needs a real aggregate, and is its own record.
- **`GET /characters` is still not proposed.** The port can enumerate now, so the door is open, but a
  wire surface for listing is a `v1alpha` addition with its own cost and its own decision.
