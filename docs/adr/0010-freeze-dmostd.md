---
id: ARD-0010
title: Freeze dmostd until LAN server work begins
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0
narrows:
  - ARD-0002 decision 9 — the daemon stays in the tree, but stops being a reason to do work
related:
  - docs/adr/0002-character-document-store.md
  - docs/psd/0001_character-management-tui.md
  - docs/psd/0002_gm-managment-tui.md
---

# ARD 0010 — Freeze `dmostd` until LAN server work begins

## Status

**Proposed.** Settles a question ARD 0002 raised and left open. It does not reverse ARD 0002 §9 so
much as remove its premise: the daemon is kept, and stops being an argument for anything.

**Revisions.**

- **v0** — initial draft.

## Context

ARD 0002's Context argues that `dmostd` is speculative in order to justify breaking the stack beneath
it — no users, no deployed database, an in-memory repository that loses everything on exit, so "the
stack is the thing worth keeping and `dmostd`'s current shape is not". Its §9 then keeps the daemon
wired and grows `internal/infra/inmem` to match the widened port, reasoning that two composition roots
keep both backends honest.

Those two positions pull opposite ways, and the second one is expensive. The port grows from two
methods to nine (`Save`, `Find`, `FindItems`, `List`, `SoftDelete`, `Undelete`, `Purge`, `Revisions`,
`FindRevision`). Seven of them exist because the TUI needs them; `dmostd` exercises two. Under §9 the
in-memory adapter owes full semantics for all nine anyway — soft-delete rules, a purge cascade,
revision history, snapshot-on-save, and the loaded/unloaded item tri-state — and `repotest` owes a
scenario for each, against a backend no user reaches.

The honest framing is not *delete or keep*. `dmostd` is the shape a LAN server would take, and both
PSDs point at one eventually: PSD-0002 describes a GM running a session for players who are not
sitting at the GM's keyboard. The daemon is a bet on that, placed early. The question is whether the
bet is *maintained* while nothing depends on it, or *dormant* until something does.

## Decision

**Freeze `dmostd`.** It stays in the tree, it stays building, and it stops constraining the design.
Concretely:

### 1. Frozen means "compiles and passes", not "works on"

The two-invocation commands in CLAUDE.md stay exactly as they are, and both halves stay green:

```bash
go build ./... && (cd cmd/dmostd && go build ./...)
go test  ./... && (cd cmd/dmostd && go test ./...)
```

A change that breaks the daemon's build or its existing tests is still a broken change. What the
freeze removes is the obligation to *extend* it: no new routes, no new DTO versions, no new handlers,
and no widening of the HTTP write path. Its end-to-end tests keep running because they are cheap and
they catch accidental breakage; they are not a reason to add anything.

### 2. The daemon stops paying for port growth

When the port widens for the TUI, `internal/infra/inmem` must still satisfy the interface or nothing
compiles — that is unavoidable and not up for decision. What is up for decision is how much *behaviour*
it owes behind each method, and the answer is: only what something actually calls.

- `Save` and `Find` keep full semantics. `inmem` remains the reference implementation of the
  versioning rule, as CLAUDE.md describes it — that rule lives in the methods the daemon and
  `app.New()` exercise, so it survives the freeze intact.
- The seven TUI-driven methods are implemented in `inmem` only where the TUI's own in-memory wiring
  needs them. Where they are not, they return a sentinel meaning *this backend does not implement
  this*, rather than a plausible-looking wrong answer.
- `internal/test/repotest`'s full `CharacterRepository` contract runs against `internal/infra/sqlite`.
  `inmem` runs the subset it claims to implement. A backend that opts out of a scenario says so in
  one place, rather than each scenario growing a conditional.

This is the part of ARD 0002 §9 that is withdrawn. Its argument — that two composition roots keep both
backends honest — is not wrong in principle, but it does not describe what would happen here: the
daemon reaches two of nine methods, so the other seven were only ever going to be kept honest by
`repotest`, which is the level the argument claimed to reach past.

### 3. A wire-visible change is no longer a breaking change

ARD 0002 §4 changes what an HTTP client sees in a `problem+json` body — `reason` becomes JSON Pointer
paths from the compiled schema. Under the freeze that is not a compatibility event, because there is no
client: `v1alpha` is under `internal/`, so the import-path prefix makes an external consumer impossible,
and no deployment exists. The change lands; nothing is owed a migration.

### 4. The thaw is a decision, not a drift

The freeze ends when LAN server work begins, and it ends deliberately: this record is superseded, the
gap between `inmem` and the port is closed on purpose, and whatever the daemon then needs — real
persistence, authentication, a stable published DTO version under `pkg/dto` — is decided then, against
requirements that exist. Nothing thaws by accident because someone added a handler.

## Consequences

- **Phase 0 shrinks, and that is the point.** PSD-0001 §11's stack strand no longer has to carry
  `inmem` to nine full port implementations plus the matching `repotest` scenarios. The work that
  remains is work the TUI needs.
- **`inmem` and `sqlite` will drift, and the contract no longer hides it.** Today both run the same
  scenarios, so they cannot diverge silently. After this they can, in exactly the methods `inmem` opts
  out of. The mitigation is that the opt-out is explicit and enumerable — a sentinel and a list, not an
  absence — so the thaw has a work list rather than an archaeology problem.
- **A frozen binary bit-rots quietly.** It compiles, so nothing complains, while the code around it
  moves — `pkg/app` command structs, the compiled-schema validator, the widened port. The build and
  test gates catch breakage but not staleness, and the longer the freeze runs the more of ARD 0002's
  "cheaply, because nothing has shipped" advantage is spent on a component nobody is watching.
- **`pkg/http`, `internal/dto/v1alpha` and `cmd/dmostd` become a trap for a future contributor** —
  live, tested, plausible-looking code that is not where the product is. That is a documentation
  obligation this record creates: CLAUDE.md's architecture section should say the daemon is frozen and
  point here, or the next person to read the tree will reasonably conclude the HTTP path is the main
  event. It currently reads as though it is.
- **The LAN bet is preserved at its current cost and no more.** Deleting the daemon would be cheaper
  today and more expensive the day PSD-0002 needs it; maintaining it is the reverse. Freezing takes
  neither side, which is the right trade only if the thaw condition is real. If LAN work is still
  unstarted a year from now, the honest follow-up record is deletion, not a longer freeze.
