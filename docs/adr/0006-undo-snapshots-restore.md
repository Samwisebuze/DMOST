---
id: ARD-0006
title: Undo is in-session; history is snapshots that write forward
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0.2
related:
  - docs/psd/0001_character-management-tui.md
  - docs/adr/0002-character-document-store.md
  - docs/adr/0004-play-log-schema-change.md
---

# ARD 0006 — Undo is in-session; history is snapshots that write forward

## Status

**Proposed.** Depends on ARD 0002, which puts the character and its item instances in one conflict
domain under one revision — the property that makes restore expressible as a single transaction.

**Revisions.**

- **v0** — initial draft.
- **v0.1** — review pass. Three gaps, all in the space between §2's trigger and §5's pruning: what
  actually fires a snapshot is more frequent than "validated save" suggests (§6), a restored revision
  is indistinguishable from an ordinary one in the table (§7), and nothing said where the `summary`
  text comes from (§8).
- **v0.2** — realigned to ARD 0002 v0.2. Nothing about the mechanism changes; what changes is who owns
  each half of it. Snapshots are a `Save` option on the repository port, and `restore` is orchestration
  in `pkg/app` rather than a store operation — which is what keeps the `play_log` splice out of a
  storage adapter.

## Context

PSD-0001 v0.1 kept each character in a JSON file, and leaned on that for durability: the character
survives the tool being wrong about something, because you can open it in an editor. Moving to a row in
a database weakens that, and ARD 0002 takes the move anyway.

Two things have to answer for it. Ordinary mis-edits need to be reversible while the user is still
sitting there, and a sheet needs a past that outlives the session — the thing a directory of JSON files
never had unless the user thought to put it in git.

There is an existing structure that looks like it should serve: `build_log`. It does not. Only two
surfaces write it (the creation wizard and the level-up wizard, per §8.3), the §7.5 escape hatch
deliberately writes nothing, and a log mixing "this was a level-up decision" with "I fixed a typo"
would be worse than one recording only the former. So the build log is **not a replayable event log**,
and nothing may be built on the assumption that it is.

## Decision

**Two independent mechanisms, neither built on `build_log`: a bounded in-session undo stack of whole
documents, and a `character_revisions` table written inside the save transaction, restored by writing
forward.**

### 1. In-session undo stores whole documents, not inverse operations

A bounded stack — 50 frames — of complete document states, pushed by the parent model on every
mutation, with `u` to undo and `Ctrl+R` to redo. Discarded on exit. Disabled in read-only mode.

Whole documents rather than inverse operations because a character document is a few tens of kilobytes
while the mutation set is broad and still growing: nine section screens' worth of typed messages, each
of which would need an inverse written and tested. Fifty copies of the document is cheaper than fifty
inverses, and it cannot be subtly wrong.

Undo restores derived values along with everything else; it does not restore inputs and then recompute
(ARD 0005). Recomputing from a half-restored state is the specific bug PSD-0001 §10 calls out for its
own test.

`u` rather than `Ctrl+Z`, because `Ctrl+Z` is `SIGTSTP`: capturing it means disabling the tty's suspend
character and breaking job control for users who expect it.

### 2. Snapshots are written by validated saves, in the same transaction

A validated save passes `WithSnapshot(summary)` to `CharacterRepository.Save` (ARD 0002 §2), and the
adapter inserts the `character_revisions` row inside the transaction that carries the `UPDATE`. Debounced autosaves do not snapshot — they fire every few seconds and would swamp the table
(ARD 0007 owns the autosave rule).

Same transaction, not a follow-up write, because a snapshot that can be absent for a save that
succeeded is not history, it is a suggestion.

### 3. A snapshot captures the whole aggregate, assembled in SQL

The obvious worry about ARD 0002's shared revision is that a snapshot needs every item instance while
the editor has lazily loaded only the few the user opened. It does not, because the aggregation happens
in the database:

```sql
INSERT INTO character_revisions (character_id, doc_revision, document, items, saved_at, summary)
SELECT :id, :rev, :document,
       (SELECT json_group_array(json(document)) FROM item_instances WHERE owner_character_id = :id),
       :saved_at, :summary;
```

`json_group_array` collects the item documents without the editor ever holding them, so lazy loading
(§7.9, §9.4) and complete snapshots compose rather than trade off. This is the subtlest thing in the
design and the reason the aggregation is not done in Go.

### 4. Restore writes forward, and replaces document and items atomically

`restore --at <timestamp|revision>` is composed in `pkg/app` from two port calls: read the revision, then
`Save` the character carrying the snapshot's document and its item set, which advances the version and
replaces the stored items in one transaction (ARD 0002 §1, §2).

Composed rather than a single port method because the restored document is not quite the snapshot's: the
current `play_log` is spliced back into it first, per ARD 0004 §5. That splice is knowledge of one field
inside the sheet, and it belongs above the adapter — ARD 0002 §8 keeps exactly one such crack open, and
this is not it.

Writing forward rather than rewinding the counter is what makes restore itself undoable: the restored
state is a new revision, the revision it came from is still there, and a restore made in error is
answered by another restore. It also keeps the revision sequence monotonic, which the aggregate's
compare-and-set depends on.

There is no partial-restore state and no restore-induced dangling reference: items added after the
snapshot disappear, items present at the snapshot come back, and the document's
`inventory.entries[].item_instance_id` references resolve because both sides moved together.

**`play_log` is the one exemption** — preserved rather than replaced, per ARD 0004 §5, because
rewinding a sheet does not un-happen a session. Restore's confirmation names it.

### 5. Pruning keeps the union, not the intersection

On insert: keep the most recent 50 revisions per character, **and** keep anything under 30 days old.
A character saved heavily in one session keeps that session's detail; one saved occasionally over a
year keeps a year of history. At ~30 KB a document, 50 revisions is ~1.5 MB per character, and `purge`
reclaims it.

Union rather than intersection is the whole rule, and it is easy to implement backwards. The numbers
are tunable defaults; the union is not.

### 6. The trigger is explicit save and session end — not every navigation-away

PSD-0001 §8.5 gates validation on `Ctrl+S`, on navigation away from the editor, and on `export`; §8.6
then says every *validated* save snapshots. Composed literally, that means a snapshot every time the
user leaves a section screen — which in an evening of editing is dozens, and pruning's most-recent-50
would then churn inside a single session. §5's claim that "one saved occasionally over a year keeps a
year of history" would be false for anyone who actually uses the tool.

So the two are separated: **validation runs on navigation-away, snapshots do not.** A snapshot is
written on explicit `Ctrl+S` and on session end, and an insert whose document is byte-identical to the
newest existing revision is skipped rather than stored.

That makes a revision mean something a user can recognize — "I decided this was a good state" or "I
finished a session" — instead of "I pressed Escape". It also makes §7.13's Revisions mode legible,
since a list where forty of fifty entries are screen exits is not a history anyone reads.

### 7. A restored revision says what it was restored from

§4 writes a restore forward as a new revision, which means the table ends up holding two rows with
identical documents and different numbers, and nothing distinguishing them. History mode would render
`r42` and `r39` as unrelated saves, and `r42`'s `summary` would either be empty or a lie.

`character_revisions` therefore carries one more column, `restored_from INTEGER`, null on an ordinary
save and set to the source revision on a restore. History renders it (`r42  restored from r39`), and
the confirmation prompt that names the `play_log` exemption can name the source too. Nothing has
shipped, so this is a change to migration 1 rather than a migration of its own.

### 8. `summary` comes from the mutation, not from a diff

`character_revisions.summary` is a one-line description of what changed, and §7.13's Revisions mode is
built entirely out of it:

```
r41  2026-08-10 14:02  Spellcasting — expended a 1st-level slot
```

Nothing in PSD-0001 says who writes that string. The tempting answer is a document diff, and it is the
wrong one: a diff can see that `slots_used` went from 1 to 2, but not that the user cast a spell, and
the strings above are all statements of intent rather than of delta.

So the summary is composed from the typed mutation messages the parent model already applies (§5.1) —
the screen that emitted them and the action they represent — accumulated since the last snapshot and
rendered as "section — most significant action, plus N more". The model is the only place that knows
intent, and it already sees every mutation in one place, which is the same property ARD 0005 relies on.

## Consequences

- **The store is now the durability story, and it is a better one than files were.** `history` plus
  `restore --at` is strictly more than a directory of JSON files had. ARD 0002's `export` envelope and
  the readable `document` column cover the rest of what the file design offered.
- **Restorable revisions are a sparse subset of the revision sequence**, and §6 makes them sparser
  still: autosave and navigation-away both advance `doc_revision` without snapshotting. History shows
  gaps, and they are honest.
- **The parent model owes the store a summary on every save**, which is a coupling §8 accepts
  deliberately: the store cannot compose that string, and no other component sees every mutation.
- **Snapshot storage grows with editing, not with time**, and the only reclamation is `purge`, which
  is also the only path that deletes a character.
- **Whether any of this is used is an open question with an instrument.** §12 question 5 counts
  `restore` invocations and undo depth. If nobody restores across a campaign, pruning can be far more
  aggressive and §7.13 collapses to one mode.
