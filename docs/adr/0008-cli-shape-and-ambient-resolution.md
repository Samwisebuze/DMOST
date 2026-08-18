---
id: ARD-0008
title: One binary, cobra subtrees, and ambient target resolution
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0
related:
  - docs/psd/0001_character-management-tui.md
  - docs/psd/0002_gm-managment-tui.md
  - docs/adr/0001-character-sheet-tui.md
---

# ARD 0008 — One binary, cobra subtrees, and ambient target resolution

## Status

**Proposed.** Extends ARD 0001 decision 1 rather than reversing it: the binary is still `dmosh` and
`main` is still thin. What is new is that `dmosh` is a command tree with two ambient inputs.

**Revisions.** v0 — initial draft.

## Context

ARD 0001 decided the name and the module shape, and stopped there — it describes a TUI you launch. The
PSDs describe something larger in two directions.

**Across subsystems.** PSD-0001 §4 covers `dmosh character ...` and says other toolkit features "add
their own subcommand trees under the same root". PSD-0002 is that other feature. So `dmosh` is one
binary hosting several subsystems, and the first decision is whether that is really one program.

**Across invocation styles.** PSD-0001's command set is fifteen commands, of which two (`new`, `open`)
take over the terminal and thirteen print to stdout and exit so the tool can be driven from a Makefile,
a hook, or CI. A TUI and a scriptable CLI have different argument conventions, and this is both.

Then there is the part that needs the most care. PSD-0001 D10 and D18 make *both* the database and the
target character resolvable from the environment, so that an `.envrc` per player folder drives every
command with no arguments at all — `dmosh character open`, `dmosh character show`, `dmosh character
validate`. That is the intended ergonomic, and it is also how a user destroys a sheet they were not
looking at.

Worth recording what was rejected: PSD-0001 v0.2 resolved `./dmosh.db` first, so the same command showed
different characters depending on the working directory.

## Decision

**One `dmosh` binary, cobra command trees per subsystem, and two precedence ladders — one for the
database, one for the character — with the destructive commands carved out of the second.**

### 1. One binary, one subtree per subsystem

`dmosh character ...` now, `dmosh gm ...` next. Shared concerns — `--db` resolution, logging,
terminal-capability detection — live at the root and are inherited, which is the argument for one binary
rather than two: the alternative has both tools re-implementing D10 and disagreeing about it.

### 2. `main` stays thin, and the root command is constructible for tests

`cmd/dmosh/main.go` does flags, logging, composition root, lifecycle — the shape `cmd/dmostd/main.go`
already establishes and ARD 0001 decision 1 adopted. Under cobra the property that shape exists for is
preserved by a `NewRootCmd` that takes its dependencies, its argument slice, and its streams, so a test
drives the whole CLI in-process without a subprocess and without a terminal. `main` is then a call to
it plus an exit code.

### 3. Database resolution: `--db` > `DMOSH_DB` > XDG data dir

`$XDG_DATA_HOME/dmosh/characters.db`, falling back to `~/.local/share/dmosh/characters.db`. **There is
no current-directory search**, and its removal is the point: per-campaign stores are still easy with a
`.envrc` setting `DMOSH_DB`, which gives the old ergonomics without making the answer depend on where
you happened to be standing.

The resolved path is printed in `list`'s header and shown in the TUI's status line, so the active store
is never a mystery.

### 4. Character resolution: argument > `DMOSH_CHARACTER` > the single live character

If none resolves, the command errors and lists the available names rather than guessing. `open` and the
bare `dmosh character` invocation instead fall through to the interactive picker, because they have a UI
to fall back into.

This mirrors §3 deliberately, and the two compose into the `.envrc`-per-player-folder pattern that is
the whole reason either exists.

### 5. Three rules make the indirection safe

- **Lifecycle commands are exempt.** `delete`, `undelete`, `purge`, and `restore` always require an
  explicit `<name-or-id>` and ignore `DMOSH_CHARACTER` entirely. An environment variable you may have
  forgotten is set must never be able to destroy or rewrite a sheet you were not looking at. `history`
  is read-only and honours the variable.
- **An ambiguous name is an error, never a guess.** A `<name-or-id>` matches `characters.id` first, then
  `character_name`; a name matching more than one live character exits non-zero listing the candidate
  ids. Resolving by name is something the user may not be able to see happening, which is what makes
  guessing unacceptable here specifically.
- **A soft-deleted target fails loudly** and names `undelete`, rather than falling through to the
  single-live-character rule.

### 6. Ambient resolution is always disclosed

Every command that resolved its target from the environment says so on stderr — `using
DMOSH_CHARACTER=Vesk Ambermarch` — and the TUI's status line shows the resolved character beside the
resolved database. Stderr rather than stdout so the disclosure does not corrupt a piped `--format json`.

### 7. Exit codes are part of the contract

| code | meaning |
| --- | --- |
| 0 | schema-valid, and stored derived fields match a fresh `Recompute` |
| 1 | schema-invalid — JSON Pointer paths on stderr |
| 2 | schema-valid but derived fields have drifted — diff printed; `recompute` fixes it |

`validate` is the command these are specified for, and they are what make it usable as a CI gate.

### 8. `purge` is CLI-only and requires `--force`

There is no in-TUI path to a permanent delete at all. Soft delete is reachable from the picker behind an
inline `y`/`n` whose prompt names `undelete`; the irreversible operation requires leaving the TUI,
naming the character, and passing a flag.

## Consequences

- **The `.envrc` ergonomic is a guess about how people organise campaigns**, and §12 question 8
  instruments it — counting how often a command resolves its target from the environment rather than an
  argument. It informs more than this tool, since PSD-0002 leans on the same shape.
- **The lifecycle carve-out is an asymmetry users will notice.** `dmosh character show` works with no
  arguments in a configured folder and `dmosh character delete` does not. That is the intended
  friction.
- **Two ambient inputs mean two ways to be confused**, and the mitigation is entirely disclosure (§6).
  If that proves insufficient the answer is a `dmosh character whoami`-style command, not more implicit
  resolution.
- **Cobra becomes a root-module dependency in earnest.** It is already present as an indirect one via
  the codegen tool, so this promotes rather than introduces it.
