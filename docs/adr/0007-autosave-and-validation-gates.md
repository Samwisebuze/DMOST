---
id: ARD-0007
title: Autosave is unvalidated; validation gates the boundaries
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0.2
supersedes:
  - ARD-0001 decision 8 — explicit save, with a conflict prompt
related:
  - docs/psd/0001_character-management-tui.md
  - docs/adr/0002-character-document-store.md
  - docs/adr/0006-undo-snapshots-restore.md
---

# ARD 0007 — Autosave is unvalidated; validation gates the boundaries

## Status

**Proposed.** Reverses ARD 0001 decision 8, which considered autosave and rejected it. The argument
that changed the answer is ARD 0006, which did not exist when that record was written.

**Revisions.**

- **v0** — initial draft.
- **v0.2** — realigned to ARD 0002 v0.2. The two write paths this record needs are now two use cases on
  `pkg/app` — `SaveCharacterDraft` and `ReplaceCharacter` — rather than one `UPDATE` with validation
  bolted on or off. That is a better fit than what v0 described: the difference between them is a
  difference in kind, and it now reads as one in the signature.
- **v0.1** — review pass against ARD 0003's load path. §3's "the store may hold an invalid document" is
  too loose: composed with validate-then-decode, it lets the tool write a document it cannot reopen
  (§7). Tightens the invariant and adds the recovery path. Also stops §1 promising a flush it cannot
  deliver on every exit path (§8).

## Context

ARD 0001 decision 8 chose explicit save: edits accumulate in an in-memory draft, a dirty indicator
shows unsaved work, quitting dirty asks for confirmation, and a key commits. It rejected autosave in
terms worth quoting, because they are still true —

> Sheet editing is exploratory — someone tries a stat spread, reads it back, changes their mind — and a
> write per keystroke-committed field gives that no undo boundary while making every intermediate state
> durable.

— and it paired explicit save with a conflict modal offering **Reload** or **Reapply**, where Reapply
resubmitted the same merge patch against a newer sheet.

Three things have changed since.

- **The undo boundary now exists independently.** ARD 0006 gives every mutation an undo frame and 50 of
  them a stack. "No undo boundary" was the strongest half of the objection, and it has been answered by
  a mechanism that does not depend on withholding writes.
- **Reapply is no longer available to the TUI.** ARD 0002 keeps `Patch` as the wire's partial-write
  shape but has the editor write whole documents, since it holds the whole document already — and a whole
  document cannot be replayed against a newer one the way a patch naming only changed fields can. Half of
  ARD 0001's conflict modal is gone regardless of what this record decides.
- **The tool is designed to be used at the table** (§12 question 1). §7.6's `+`/`-` HP adjusters and
  §7.8's one-key cast action are built for speed under time pressure, and the cost of losing five
  minutes of combat bookkeeping to a closed terminal is higher than the cost of durable intermediate
  states.

What has *not* changed is that a mid-edit document is routinely incomplete. Blocking a write on
validation means losing work exactly when the user is typing most.

## Decision

**The document autosaves on a 2s debounce and that write does not validate. Validation gates the
boundaries instead: explicit save, navigation away from the editor, and export.**

### 1. The debounce is the durability mechanism

Two seconds after the last mutation, the document goes through `SaveCharacterDraft` (ARD 0002 §3) — no
schema gate, no snapshot, no confirmation. The most work at risk at any moment is two seconds of it, and
per §8 that is the guarantee, not the flush.

The two write use cases exist so this distinction is visible at the call site rather than carried in a
boolean: `SaveCharacterDraft` is the unvalidated one, `ReplaceCharacter` is the gate in §2.

### 2. Validation runs at five places, and none of them is the debounce

On `open`, on `import`, on wizard completion, on explicit `Ctrl+S`, on navigation away from the editor,
and on `export`. Explicit save is blocked by a failure and the document stays in memory to be fixed; the
failing JSON Pointer paths are shown, which is why ARD 0003 validates before decoding.

### 3. The store may transiently hold a schema-invalid document, and that is a stated property

This is the consequence that makes the decision real rather than a convenience. `characters.document` is
not guaranteed schema-valid at rest. It is guaranteed to be JSON — the shipped migration's
`CHECK (json_valid(data))` on a `STRICT` table says so, and `character.NewCharacter` additionally
requires a JSON *object* — but nothing stronger.

Everything reading the store must therefore be written for that: `open` has the banner-and-read-only
path, `validate` exists precisely to report it, and `show` renders what it finds. A component that
assumes a stored document is valid has a bug, not a bad input.

### 4. Read-only is entered for three different reasons

| reason | behaviour |
| --- | --- |
| validation failed on open | banner naming the failing pointers, read-only until acknowledged |
| `ruleset.revision` is not `"2024"` (D5) | read-only with an explanatory banner; viewable, exportable, validatable |
| `schema_version` mismatch | banner naming the delta; does **not** block opening or editing |

The middle one is a scope decision rather than a safety one: the tool models the 2024 ruleset, and
editing a 2014 sheet with 2024 assumptions would corrupt it slowly. The third deliberately does not
gate, per the schema report's stance against silent mutation of live sheets — and it will fire once for
every document written before ARD 0004's bump.

### 5. The status line carries the state, so a save-time rejection is never a surprise

Resolved database path, a dirty indicator (`●` unsaved, `✓` saved with a relative timestamp), and a
`⚠` whenever the in-memory document is currently failing validation. The user knows before pressing
`Ctrl+S` whether it will be refused.

### 6. Conflict detection exists; conflict resolution does not

`common.ErrConflict` from the port — the aggregate's compare-and-set finding the stored revision has
moved on — raises a banner, and the write is not lost from memory. v1 assumes one intended writer per character and offers no merge, no reload prompt, and no
reapply — ARD 0001's modal is superseded rather than reimplemented.

Detection without resolution is worth having anyway: it is the difference between a lost write and a
reported one, and it costs nothing because it falls out of writing the `UPDATE` correctly.

### 7. The invariant is *decodable always*, valid only at the gates

§3 as v0 stated it is too weak, and composing it with ARD 0003 shows why. That record validates before
decoding so that a bad document yields JSON Pointer paths rather than Go type errors. But the generated
tree enforces things of its own during decode — enum membership, and numeric bounds:

```go
if plain.SlotsUsed != nil && 0 > *plain.SlotsUsed {
	return fmt.Errorf("field %s: must be >= %v", "slots_used", 0)
}
```

An editor holding `slots_used = -1` in memory re-encodes it happily, autosave writes it, and the next
`open` cannot decode it at all. The read-only-with-banner path in §4 does not save the user, because
there is no document in memory to acknowledge a banner about. **A tool that cannot reopen a file it
wrote has no excuse**, and unvalidated autosave is exactly the mechanism that produces one.

So the invariant is split in two:

| property | when it holds | enforced by |
| --- | --- | --- |
| the stored document **decodes** into the generated tree | always | field-level input constraints in the editor |
| the stored document **validates** against the schema | at the gates in §2 only | the compiled schema |

The first is the editor's obligation: a numeric field refuses a value outside the schema's bounds at
input time, and an enum field can only be set to a member. That is the same inline refusal ARD 0003's
hand-written exceptions provide for slot keys and Exhaustion levels, applied as a general rule rather
than to two special cases. It is not a validation gate — it is narrower and cheaper, and it is what
makes autosave safe to leave ungated.

And because "always" enforced by convention is not always, `open` gets a fallback: a decode failure
drops to a recovery path that reports the offending field, offers the newest snapshot (ARD 0006) as a
restore target, and offers the raw JSON for export. Read-only until acknowledged, then editable — the
user must be able to repair the document the tool broke.

### 8. What the exit path can and cannot promise

v0 said quitting flushes synchronously in the `tea.Program` quit path. That is true for `q` and for
`Ctrl+C`, which Bubble Tea delivers as a message. It is not true for `SIGHUP` from a closed terminal,
for `SIGTERM`, or for a killed process, and writing a signal handler for each would mean a write racing
a teardown that is already in progress.

**The debounce is the guarantee; the flush is an optimization.** Two seconds of loss on an abrupt exit
is the stated worst case, and it is the number to argue with if anyone finds it too high. Claiming a
clean flush on every exit path would be a promise the runtime cannot keep.

## Consequences

- **ARD 0001's exploratory-editing objection is accepted and unaddressed at the storage layer.** Every
  intermediate state is durable. The answer to "I changed my mind" is `u`, and the answer to "I changed
  my mind about the last twenty minutes" is `restore` (ARD 0006).
- **`validate` becomes a routine, expected non-zero exit.** A store whose TUI is mid-edit will report
  invalid documents, and that is correct behaviour rather than a failure of the tool.
- **The `⚠` indicator is now load-bearing UI**, not decoration: it is the only thing standing between
  unvalidated autosave and a user surprised at save time.
- **Field-level input constraints become mandatory rather than a nicety** (§7), and they are the one
  piece of validation that cannot be deferred to a gate.
- **`open` needs a path for documents it cannot decode**, which is code that exists solely to survive
  this decision being imperfectly implemented.
- **A conflict is a dead end in v1.** The banner tells the user their write did not land and offers
  nothing to do about it beyond copying values out by hand.
