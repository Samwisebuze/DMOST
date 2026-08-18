---
id: ARD-0008
title: One binary, cobra subtrees, and ambient target resolution
updated: 2026-08-18
status:
  - kind: proposed
  - version: v0.1
supersedes:
  - ARD-0001 decision 3 — the database's location under os.UserConfigDir()
related:
  - docs/psd/0001_character-management-tui.md
  - docs/psd/0002_gm-managment-tui.md
  - docs/adr/0001-character-sheet-tui.md
---

# ARD 0008 — One binary, cobra subtrees, and ambient target resolution

## Status

**Proposed.** Extends ARD 0001 decision 1 rather than reversing it: the binary is still `dmosh` and
`main` is still thin. What is new is that `dmosh` is a command tree with two ambient inputs.

**Revisions.**

- **v0** — initial draft.
- **v0.1** — review pass. §7's exit codes turn out not to be a contract yet: they collide with cobra's
  usage errors and say nothing about `--all` (§9). Also records why the store's location moved off
  ARD 0001's `os.UserConfigDir()`, and answers the apparent inconsistency between §5's carve-out and the
  TUI's own delete and restore (§10).

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

### 8. XDG rather than `os.UserConfigDir()`, because a character is data

ARD 0001 decision 3 put the database under `os.UserConfigDir()`. This record moves it, for two reasons
that v0 left implicit.

A character sheet is not configuration. It is the user's work, and the directory conventions separate
those deliberately — a config directory is the sort of thing a user might reasonably delete to reset an
application. Go's standard library has `os.UserConfigDir`, `os.UserCacheDir`, and `os.UserHomeDir`, and
**no `os.UserDataDir`**, which is exactly why D10 spells the XDG path out by hand rather than calling
something.

XDG is honoured on every platform rather than deferring to platform-native locations, which is a
deliberate choice for a terminal tool: someone running `dmosh` over SSH inside tmux expects
`~/.local/share`, and `~/Library/Application Support/dmosh/characters.db` is a path that is awkward to
type and surprising to find. The cost is that `dmosh` is not a well-behaved macOS or Windows citizen in
this one respect.

### 9. Exit codes: usage errors are moved out of the way, and `--all` reports the worst outcome

§7's table cannot be a contract as written. Cobra exits 1 on an unknown flag or a missing argument, so a
script cannot distinguish "this document is schema-invalid" from "you typed the command wrong" — and 1
is the code a caller is most likely to branch on. §7 also says nothing about `validate --all`, which is
the form the CI gate actually uses.

Two additions:

| code | meaning |
| --- | --- |
| 64 | usage error — unknown flag, missing or unresolvable argument, ambiguous name |
| 70 | internal error — the store could not be opened, a migration failed |

64 and 70 are the `sysexits.h` values for `EX_USAGE` and `EX_SOFTWARE`, borrowed rather than invented so
that anyone who has written a shell script recognizes them. Cobra's default exit is overridden at the
root so a usage error never lands on 1.

And `--all` **returns the worst outcome by severity, which is not the numeric maximum**: invalid
outranks drifted, so it exits 1 if any document is invalid, else 2 if any has drifted, else 0. Stating
that is not pedantry — the obvious implementation takes the maximum code, and that would report a store
containing one unparseable sheet and forty drifted ones as merely drifted.

### 10. The carve-out is about visibility, not about the command

§5 exempts `delete`, `undelete`, `purge`, and `restore` from ambient resolution, and yet the TUI can soft
delete from the picker (§8.4) and restore from History mode (§7.13). That reads as an inconsistency and
is not one, because the rule is not "these operations are dangerous" — it is **"a destructive operation
must name a target the user can see"**.

In the TUI the target is selected, on screen, and behind an inline `y`/`n` whose prompt names the
recovery path. On the command line with `DMOSH_CHARACTER` set, the target is a string in a file the user
may have written months ago in a different folder. Same operation, different amount of evidence in front
of the person authorizing it.

`purge` is the one operation that stays CLI-only regardless, per §11, because there is no recovery path
to name.

### 11. `purge` is CLI-only and requires `--force`

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
- **Exit code 1 now means exactly one thing**, at the cost of two codes most users will never see and a
  cobra root that has to override its default error handling.
- **`dmosh` follows XDG on platforms where XDG is not the convention** (§8), which will read as a bug to
  someone who expects a macOS application to keep its data under `~/Library`.
- **Cobra becomes a root-module dependency in earnest.** It is already present as an indirect one via
  the codegen tool, so this promotes rather than introduces it.
