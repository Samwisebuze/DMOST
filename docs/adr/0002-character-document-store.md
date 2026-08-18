---
id: ARD-0002
title: Character storage is a document store the TUI owns
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0.1
supersedes:
  - ARD-0001 decision 3 — SQLite wired through internal/infra/sqlite
  - ARD-0001 decision 7 — writes are merge patches through CharacterService.Patch
  - ARD-0001 appendix — the proposed CharacterRepository.List port method
related:
  - docs/psd/0001_character-management-tui.md
  - internal/infra/sqlite/migrations/000001_create_characters_table.up.sql
---

# ARD 0002 — Character storage is a document store the TUI owns

## Status

**Proposed.** No code exists for this yet. It supersedes three parts of ARD 0001, which was
accepted before PSD-0001 was written; where the two disagree, this record is the newer decision and
ARD 0001's Status carries the pointer.

**Revisions.**

- **v0** — initial draft.
- **v0.1** — review pass against the code and the sibling records. Resolves a dual-authority problem
  v0 inherited from PSD-0001 §9.1: five row columns duplicate fields the document already declares,
  and nothing said which one wins (§7 below). Adopts `STRICT` and states the guard against the two
  `characters` tables meeting in one file, both of which v0 left as unresolved consequences.

## Context

ARD 0001 decided that `dmosh` would reach storage the way `dmostd` does: `pkg/tui` → `pkg/app` →
`pkg/domain/character`, with `internal/infra/sqlite` behind the `CharacterRepository` port and
writes expressed as RFC 7396 merge patches through `CharacterService.Patch`. It also proposed adding
`List` to the port, because a picker screen had nothing to call.

PSD-0001 specifies something different, and not by oversight — §5.3 and §9 describe an
`internal/store` package that "owns all persistence; no other package issues SQL", loading with
`SELECT document FROM characters WHERE id = ? AND deleted_at IS NULL` and saving with
`UPDATE characters SET document = ?, doc_revision = doc_revision + 1, updated_at = ? WHERE id = ? AND doc_revision = ?`.
Nothing in that path passes through `pkg/domain`, `pkg/app`, `internal/dto/v1alpha/mapper`, or
`internal/infra/sqlite`.

The gap is not stylistic. The two designs disagree about what a character *is* at rest.

- **The existing stack stores an opaque sheet under an aggregate.**
  `internal/infra/sqlite/migrations/000001_create_characters_table.up.sql` is four columns — `id`,
  `data`, `created_at`, `version` — and its comment says so outright: "the character aggregate is an
  identity, an opaque sheet, a creation instant, and a version, and this table is those four and
  nothing else." The domain deliberately cannot see inside `data`.
- **PSD-0001 stores a queryable document.** Its `characters` table carries `deleted_at` for soft
  delete, `schema_version`, and four `GENERATED ALWAYS AS (json_extract(document, …)) STORED`
  columns so that `list` sorts by name and level in SQL rather than decoding every row in Go — which
  is what §2's sub-100ms startup budget rests on. Generated columns require the storage layer to know
  the sheet's shape. That is precisely the knowledge `pkg/domain/character` is built not to have.

Three further requirements in PSD-0001 have no expression in the port as it stands: soft delete with
an `undelete`/`purge` pair (D8), a snapshot table written inside the same transaction as the update
(D17, §8.6), and item instances that share the character's revision so that character-plus-items is
one conflict domain (D20, §9.4). Each of those is a multi-statement transaction. `Save(ctx, *Character)`
cannot express one, and widening the port until it can would mean pushing snapshots, soft deletion,
and the item table into `pkg/domain` — a domain that would then know about revisions tables.

So the question this record answers is not "SQLite or not". It is: **does the TUI reach storage
through the existing domain port, or does it own a document store directly?**

## Decision

**`internal/store` owns character persistence for `dmosh`, directly over `database/sql`. It does not
go through `pkg/domain/character`, `pkg/app`, or `internal/infra/sqlite`.**

```
cmd/dmosh  →  internal/tui  →  internal/store  →  modernc.org/sqlite
                     ↘  internal/character (documents + derive, ARD 0003, ARD 0005)
```

### 1. The document is the aggregate root; the row is its home

The `document` column holds the full `character.schema.json` document as JSON *text*, unmodified.
JSON1 functions project a few fields out of it for indexing and ordering. They never decompose it:
there is no `abilities` table, no `inventory_entries` table, and adding one is a decision that would
need its own record.

Generated columns rather than columns the writer populates, because SQLite recomputes them on write
and they therefore cannot drift from the document. `character_name`, `total_level`, `ruleset_rev`,
and `campaign_id` are `STORED` so the partial index over live rows can use them. §7 extends the same
reasoning to the columns PSD-0001 §9.1 left as plain ones.

Every table is `STRICT`, for the reason
`internal/infra/sqlite/migrations/000001_create_characters_table.up.sql` already argues at length:
without it SQLite stores what it is handed regardless of declared type, and `database/sql` sends a Go
`[]byte` as a BLOB that TEXT affinity will not convert — so a store binding `[]byte(doc)` instead of
`string(doc)` would write a blob silently and forever, and every `json_extract` over it would then be
reading JSONB. With `STRICT` the same mistake is an error at the first write.

### 2. The character and its item instances are one aggregate, one revision, one conflict domain

`item_instances` rows carry no revision of their own (D20). The character's `doc_revision` covers the
whole aggregate: editing an item marks the character dirty, and the item write joins the character's
transaction under the character's `doc_revision` predicate.

This is what makes snapshot restore a clean transition rather than a partial one, which ARD 0006
takes up. It also means there is exactly one answer to "did someone else change this while I was
editing", instead of one per row.

### 3. Optimistic concurrency is the trailing `doc_revision` predicate

Every write is `… WHERE id = ? AND doc_revision = ?`. Zero rows affected is a conflict and is
surfaced, never retried and never silently overwritten.

The value compared is the revision the document was *loaded* at, and the new value is written into
the document rather than incremented in the column (§7).

This is the same rule `common.Aggregate.Version` implements, arrived at independently, and the
similarity is worth naming: the mechanism is not being abandoned, only relocated. `dmosh` assumes one
intended writer per character (§8.5); the check exists because it is a side effect of using the
database correctly, not because v1 has a multi-writer story. What v1 does *with* a conflict is
ARD 0007's problem.

### 4. Soft delete lives in the store, and `purge` is what completes it

`deleted_at IS NULL` means live. `delete` is an `UPDATE`, `undelete` clears the column, and `purge`
is the only path that issues a real `DELETE` — therefore the only one that fires the
`ON DELETE CASCADE` on `character_revisions` and `item_instances`. Without `purge`, soft deletion
alone means the database grows without bound and the cascades never run.

`foreign_keys = ON` is not decoration here. SQLite ignores foreign keys by default, so without the
pragma the cascades silently do nothing and `purge` orphans rows. It sits alongside `journal_mode =
WAL` (so `dmosh character show` against a database the TUI has open is a legitimate workflow) and
`busy_timeout = 5000`, per D11.

### 5. Its own numbered migrations, not `golang-migrate`

`internal/store` applies numbered `.sql` files embedded with `go:embed` against its own
`schema_migrations` table.

The root module already depends on `golang-migrate/migrate/v4` for `internal/infra/sqlite`, so this
is a second migration mechanism in one repository and that cost is real. It is taken because
`golang-migrate` brings a driver-registry and a `Close` that closes the `*sql.DB` handed to it — a
hazard CLAUDE.md documents and `internal/infra/sqlite` works around — in exchange for features this
store does not use: no down-migrations in the CLI surface, no remote sources, no version forcing.
A `for` loop over embedded files and one table is the whole requirement.

### 6. `CharacterRepository.List` is withdrawn

ARD 0001 proposed adding `List(context.Context) ([]Character, error)` to the port, with a `repotest`
contract case, `inmem` and `sqlite` implementations, and `FindAll` on the service. **None of that is
needed.** The picker reads `SELECT id, character_name, total_level, updated_at FROM characters WHERE
deleted_at IS NULL ORDER BY character_name` — the query ARD 0001's own appendix said would be the
right move "if enumeration ever becomes slow enough to matter", reached immediately because the
generated columns make it free.

Nothing else in that appendix is withdrawn: it was right that a projection puts schema knowledge into
the storage layer, and this record accepts exactly that, having concluded the storage layer is the
place for it once the storage layer stops being a domain port.

### 7. Row columns are projections of the document; the document is authoritative

PSD-0001 §9.1 gives `characters` four generated columns and five plain ones — `id`, `schema_version`,
`doc_revision`, `created_at`, `updated_at` — and four of those five duplicate a field
`character.schema.json` already declares at its root. It does not say which copy wins, and its
`UPDATE` increments the *column* `doc_revision` while never mentioning `$.doc_revision`. Left there,
the two diverge on the first save, and the divergence escapes: `export` emits the document, so a
handed-off or patched character would carry a revision number the store had moved past — which is
exactly the field PSD-0002's patch flow and `mapper.serverOwnedSheetKeys` both treat as
authoritative bookkeeping.

**The document wins.** `schema_version`, `doc_revision`, `created_at`, and `updated_at` become
`GENERATED ALWAYS AS (json_extract(document, …)) STORED` alongside the other four. The store sets
`$.doc_revision` and `$.updated_at` in the JSON it is about to write, and SQLite projects them; there
is no second place to forget.

Two exceptions, both forced:

| column | why it stays plain |
| --- | --- |
| `id` | SQLite forbids a generated column in a PRIMARY KEY. It is written from `_id` on insert and never updated. |
| `deleted_at` | It has no document counterpart, deliberately. Soft deletion is a fact about this store's row, not about the character — a deleted character exported and imported elsewhere is not deleted there. |

`id` being the one hand-written copy of a document field is the one place drift remains possible, so
it is worth an insert-time assertion rather than trust.

## Consequences

- **The existing character stack is not the flagship application's path to storage.**
  `pkg/domain/character`, `pkg/app/services.CharacterService`, `internal/dto/v1alpha/mapper`,
  `internal/jsonmerge`, `internal/infra/sqlite`, and `pkg/http`'s character handlers keep working and
  keep their tests, but their only consumers are `dmostd` and its end-to-end tests. That is a
  layered architecture with no application on top of it.
- **Two SQLite schemas now both have a `characters` table, with different columns, in different
  files.** `internal/infra/sqlite`'s is `id/data/created_at/version`; `internal/store`'s is the
  document table above. Nothing in either package stops them being pointed at the same file, and if
  that happens each migration runner sees a `characters` table it cannot read and its own
  `schema_migrations` row absent. **`internal/store`'s first migration therefore refuses to run
  against a database holding a `characters` table it did not create**, naming the other tool in the
  error rather than failing on a missing column later. The reverse guard is `internal/infra/sqlite`'s
  to add if it ever wants one; nothing points it at a user's data directory today.
- **Merge-patch write semantics are lost, and with them ARD 0001's reapply-on-conflict offer.**
  A whole-document `UPDATE` cannot be replayed against a newer document the way a patch naming only
  changed fields can.
- **Revision numbers have gaps, and History shows them.** `doc_revision` advances on every save
  including the unvalidated debounced ones (ARD 0007), while `character_revisions` rows are written
  only for validated saves (ARD 0006). So the revisions a user can restore to are a sparse subset of
  the sequence, and §7.13's Revisions mode will list `r41, r38, r33`. That is honest — those are the
  revisions that exist — but it must not be mistaken for a missing-rows bug.
- **`internal/store` must not become a second domain.** The temptation, once it knows
  `$.identity.character_name`, is to keep going. The boundary this record draws is: generated columns
  for listing, `json_group_array` for snapshots, and nothing else. Anything further needs a record.
