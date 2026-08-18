# ADR 0001 — Character sheet TUI

## Status

**Accepted.** No code exists for this yet; everything below describes a decision, not a state of
the tree. The one change it asks of existing packages is called out under
[A new port method](#a-new-port-method-characterrepositorylist).

## Context

The character sheet stack is finished on the server side and unusable by a person.

`pkg/domain/character` holds the aggregate. `docs/jsonschema/character/v1alpha` defines a
twenty-odd-section sheet across fourteen schema files. `internal/dto/v1alpha` carries the wire
types, `mapper` translates them, `pkg/app` exposes create, replace, patch and find, `pkg/http`
serves all four, and `internal/infra` implements the repository port twice over. What none of that
adds up to is a way to *use* it: the only executable in the repository is the `dmostd` daemon, and
the only clients of its API are the end-to-end tests in `cmd/dmostd`. Making a character today
means hand-writing a document that satisfies eleven required sections and POSTing it with `curl`.

The gap is a human-facing application, and the project's stated interest — native, local-first
design in Go — points at a terminal UI over local storage rather than at a web front end. `/public`
exists in the directory guide and is empty; nothing here forecloses it, but a TUI is the smaller
and more direct answer to "I want to edit my character."

Four constraints come from the code and shape everything below.

- **The domain cannot see inside a sheet.** `character.Character` has exactly two fields: an
  embedded `common.Aggregate[CharacterID]` and a `json.RawMessage`. There is no `HitPoints`, no
  `AbilityScore`, no `Level`. `NewCharacter`'s doc comment states the reasoning — what counts as a
  well-formed sheet is a wire-contract rule that changes with the schema version, while the
  domain's own rule is the narrower one that survives it: a Character has a sheet, and that sheet
  is a JSON object.
- **The generated schema type is a gate, never a representation.** `mapper.validateSheet` decodes
  a sheet into `internal/dto/v1alpha/character.CharacterSchema` and *throws the result away*. The
  bytes are what gets stored. Re-encoding the decoded struct would silently drop every field the
  generated type has no home for, and both the mapper and `CharacterResponse` carry comments
  saying so. Any client that round-trips a whole sheet through that type is data loss waiting for a
  schema to drift.
- **Concurrency control is already designed.** `Save` is a compare-and-set on
  `common.Aggregate.Version`; a stale write returns `common.ErrConflict`, which deliberately does
  *not* wrap `ErrInvalid`, because losing a race is not a malformed request. `PatchCharacterRequest`
  already carries the expected version.
- **The repository cannot enumerate.** `CharacterRepository` is `Save` and `Find`. Nothing lists.

## Decision

Build **`dmosh`**, a Bubble Tea terminal application that manages character sheets against local
SQLite, with the UI as a library under `pkg/tui` and a thin executable under `cmd/dmosh`.

```
cmd/dmosh (nested module)  →  pkg/tui  →  pkg/app  →  pkg/domain
      │                                                    ↑
      └── composition root ──→ internal/infra/sqlite ───────┘
                                (file-backed, ~/.config/dmost)
```

### 1. A nested module at `cmd/dmosh`, with the UI in `pkg/tui`

`cmd/dmosh` gets its own `go.mod` with `replace github.com/samwisebuze/dmost => ../..`, mirroring
`cmd/dmostd`, so the executable versions independently of the library it consumes. `main.go` stays
thin — flags, logging, composition root, lifecycle — in the `Main` / `NewMain` / `Run(ctx)` /
`Close()` shape `cmd/dmostd/main.go` already establishes, and for the same reason: that shape is
what lets end-to-end tests share the program's own setup code.

Everything that draws or decides anything lives in `pkg/tui`, under the root module. The `/pkg` =
reusable libraries convention wants it there, and the practical payoff is that the UI is testable
without building or launching a binary.

The name is `dmosh` — "DMOST shell" — rather than the bare `dmost`, which stays free for a general
CLI if one is ever wanted. A TUI and a scriptable command-line tool are different programs with
different argument conventions, and naming the TUI `dmost` would make the good name unavailable to
the one that deserves it.

### 2. Embedded, not networked

`dmosh` builds its own `app.App` directly over the domain services. It does not start `dmostd`, does
not speak HTTP, and does not use `pkg/http.Client`.

This is what local-first means here: the application is complete on its own, with no daemon to
babysit, no port to conflict, and no failure mode where the UI is up and the server is not. `app.App`
is a struct of interfaces precisely so a composition root can assemble it, and `cmd/dmostd` already
assembles one by hand rather than calling `app.New()` — this is a second composition root, not a
second wiring mechanism.

The cost is a second place that knows how to wire the application, and the two will drift if nobody
watches them. That is accepted, and it has an upside: `dmostd` wires `inmem` while `dmosh` wires
`sqlite`, so the two roots between them keep both backends honest at the composition level, not just
in the repository contract tests.

Talking to a remote `dmostd` is deliberately out of scope. The client methods already exist and
carry the same signatures the services do, so the door is open — but a `--remote` flag implies a
port abstraction over "local service vs. HTTP client", and that is a decision to make when there is
a reason to make it, not now.

### 3. SQLite, file-backed — `dmosh` is the adapter's first real consumer

The composition root wires `internal/infra/sqlite` against a file under `os.UserConfigDir()`.

`internal/infra/sqlite` is written, documented at length, and run through the port contract against
both a file DSN and an in-memory one — and it is wired into nothing. `cmd/dmostd` still builds
`inmem` repositories, so every character the daemon has ever stored died with the process. An
application whose entire purpose is to keep a character sheet between sessions is the natural first
consumer, and adopting it costs almost nothing: `NewDB()` returns something already runnable, `Open`
runs the embedded migrations unless `AutoMigrate` is cleared, and the schema therefore exists on
first launch without a setup step.

Three settings the root sets explicitly, all documented in `go doc ./internal/infra/sqlite`:

| setting | value | why |
| --- | --- | --- |
| `DSN` | a file path under `os.UserConfigDir()` | the whole point; the default DSN is in-memory |
| `JournalMode` | `WAL` | it is ignored for in-memory databases and worth having for a file |
| `BusyTimeout` | non-zero | zero leaves SQLite's default of not waiting at all |

The in-memory backend stays available for a throwaway session and for tests, but the default is the
file. Note that the pool caps documented for the in-memory DSN — the reserved keepalive connection,
the two-connection ceiling — are an in-memory concern; a file-backed database returns a real
`SQLITE_BUSY` that `retry.go` already handles.

### 4. Bubble Tea and Lip Gloss

The Charm stack, in its Elm shape: a `Model` value, `Update(tea.Msg) (tea.Model, tea.Cmd)`, and
`View() string`.

The decisive property is not the rendering — it is that a `Model` is a plain value with a pure
transition function. A test constructs one, feeds it synthetic messages, and asserts on the result
without a terminal, a goroutine, or a program running anywhere. A widget-and-callback toolkit like
`tview` would write less layout code for a form-heavy screen, but its state lives inside stateful
widgets bound to a running application, and testing it means driving something that owns the
terminal. Given that this application's hard parts are draft state, dirty tracking, and conflict
resolution — all of them state-machine problems — testability of the state machine outranks the
layout convenience.

Blocking calls into `app.App` never happen inside `Update`. They are `tea.Cmd`s that return a
message, which is also what keeps the conflict handling in decision 7 expressible as ordinary state
transitions.

### 5. A section list beside a detail pane

The left column lists the sheet's sections; the right pane renders and edits whichever has focus.

The column maps one-to-one onto the schema files, because those files already are the decomposition
someone chose deliberately: identity and progression, abilities, origin, classes, features, vitals,
combat, spellcasting, inventory, currency, reputation, build log. That correspondence is worth
preserving — it means a new section in the schema has an obvious home in the UI, and it keeps the
navigation legible to anyone who has read `docs/jsonschema/character/v1alpha/README.md`.

A tabbed layout shaped like a physical character sheet would read better at the table, and may
eventually be the right answer for a play mode. It does not scale to twenty-odd sections without
bespoke layout work for each tab, and the first version needs to reach every section before it needs
any one of them to be beautiful.

### 6. Section-scoped view models — **not** the generated `CharacterSchema`

**This is the load-bearing decision.** The TUI defines its own structs, one per section it can edit,
and decodes only the section in focus into one. It never decodes the whole sheet into
`internal/dto/v1alpha/character.CharacterSchema`, and it never re-encodes a whole sheet from a
decoded value.

The reason is the second constraint in the Context, and it is not hypothetical. `validateSheet`
decodes into `CharacterSchema` purely to enforce required fields, enums and patterns, then discards
the result, so that a field the generated type does not know about still reaches storage and still
comes back to the client. A TUI that unmarshalled the sheet, edited a struct field, and marshalled
it back would quietly delete every such field the moment anyone saved — a homebrew annotation, a
key from a newer schema, anything an older generated type has no home for. The system's stated
guarantee would be broken by its own flagship client.

Owning the view models has a second benefit worth naming: the UI's shape is free to differ from the
document's. A screen can present computed values, or group fields that live in different sections,
without either pretending to be part of the wire contract or forcing the schema to accommodate a
display concern.

The cost is real. Every editable section needs a hand-written struct and hand-written decode, and
when the schema gains a field the UI does not learn about it for free — the generated type would
have. That is the trade being made: mechanical work in exchange for never silently dropping a user's
data.

### 7. Writes are merge patches through `CharacterService.Patch`

Saving builds an RFC 7396 JSON Merge Patch naming only the fields that changed, and calls the
existing `CharacterService.Patch`.

**Nothing new is written for merging.** That path already exists end to end —
`mapper.ApplyCharacterPatchRequest` → `internal/jsonmerge.Apply` — and the TUI inherits its rules by
using it rather than reimplementing them:

- **The schema gate runs on the merged result, not on the patch**, which is what catches an edit
  that empties a required section. A patch is frequently legal JSON in isolation while the document
  it produces is not.
- **`serverOwnedSheetKeys` refuses a patch that names `_id`, `doc_type`, `schema_version`,
  `created_at`, `updated_at`, `doc_revision` or `owner_user_id`** at the top level. The TUI does not
  need its own rule about what a user may not edit; it needs to render the error when it trips one.
- **A merge patch touches the keys it names and leaves the rest of the document alone**, which is
  exactly the property decision 6 requires. `jsonmerge` merges decoded maps, not the generated
  struct, so an unknown field is data it walks past rather than a field it has nowhere to put.

One cost, stated plainly because the DTO's own doc comment states it: **`jsonmerge.Apply` re-encodes
from a decoded value.** A patched sheet comes back with object keys sorted and insignificant
whitespace gone, so byte-for-byte fidelity to whatever was originally stored is lost the first time
anything is patched. Unknown *fields* survive — that is the guarantee that matters — and numbers
survive intact because both documents decode with `UseNumber`. Byte-exactness does not. For a sheet
that a TUI is going to rewrite anyway, normalized output is an acceptable price; a client that needs
its exact bytes preserved must use `Create` or the replace path instead, which store them.

Creating a character uses `CharacterService.Create`, which does store the client's bytes. The TUI
composes a new sheet from a minimal template satisfying the eleven required sections and submits it
whole; there is nothing to merge into yet.

### 8. Explicit save, with a conflict prompt

Edits accumulate in an in-memory draft. A dirty indicator shows unsaved work, quitting dirty asks
for confirmation, and an explicit key commits — sending the `Version` the sheet was loaded at on the
`PatchCharacterRequest`.

Autosaving on every accepted field edit was considered and rejected. Sheet editing is exploratory —
someone tries a stat spread, reads it back, changes their mind — and a write per keystroke-committed
field gives that no undo boundary while making every intermediate state durable.

`common.ErrConflict` surfaces as a modal offering two ways out:

| choice | what happens |
| --- | --- |
| **Reload** | discard the draft, re-`Find`, show the stored sheet |
| **Reapply** | re-`Find` to pick up the current version, resubmit the same patch against it |

Reapply is safe to offer precisely *because* the write is a merge patch: it names only the fields
the user touched, so replaying it against a newer sheet preserves whatever changed elsewhere. That
is a narrow, honest form of merge — it does not detect that both sides edited the same field, and
the last writer wins on that field. A real three-way merge with per-field conflict reporting is
deferred; if it is ever wanted it is its own record.

Worth noting where this can trip today: `services.CharacterService` checks the expected version
explicitly before the repository's compare-and-set, so a stale write fails early rather than racing.
Both paths return the same sentinel, and the UI does not distinguish them.

### 9. Model unit tests plus golden `View()` snapshots

Two layers, both on the existing `testify` setup:

- **State transitions** — construct a `Model`, feed it `tea.Msg` values, assert on what comes back.
  This is where draft tracking, dirty state, and every branch of the conflict prompt get tested,
  and none of it needs a terminal.
- **Rendering** — snapshot `View()` into golden files under `testdata/`, so layout and styling
  regressions are visible in a diff.

Golden snapshots of styled terminal output are notoriously unstable, and two things make them
stable here: render at a **fixed width** set by the test rather than by the ambient terminal, and
force Lip Gloss to an **unstyled color profile** so ANSI escape sequences stay out of the fixtures
and the golden files stay readable to a human reviewing the diff. A snapshot that changes when the
test runner's `TERM` changes is worse than no snapshot.

Charm's `teatest` harness would drive a whole program against a simulated terminal, and is the
closer thing to end-to-end. It is an experimental dependency, and the two layers above cover the
logic and the layout between them; revisit if something is found that neither can catch.

## A new port method: `CharacterRepository.List`

The picker screen in decision 5 has nothing to call. `CharacterRepository` is `Save` and `Find`;
there is no way to enumerate characters, so the UI cannot show the user what they have.

This record proposes one addition, and only one:

| where | change |
| --- | --- |
| `pkg/domain/character/character.go` | `List(context.Context) ([]Character, error)` on the port |
| `internal/infra/inmem/character.go` | implement it |
| `internal/infra/sqlite/character.go` | implement it |
| `internal/test/repotest/character.go` | a contract case, so the two cannot drift |
| `pkg/app/service.go` and `services/character.go` | `FindAll`, mirroring `UserService.FindAll` |

The contract case is not optional. Enumeration is a rule of the *port* — that every saved character
comes back, at its stored version — rather than of either technology, which is exactly the
distinction `repotest`'s package comment draws for deciding what belongs there. Ordering is the one
question the contract has to answer explicitly; leaving it unspecified is a legitimate answer, but
it has to be a stated one, since a map-backed implementation and a SQL one will disagree by default.

**`List` returns full aggregates, not a projection.** That looks wasteful — the picker needs a name
and a level, and it will be handed entire sheets — but the sheet is opaque, and
`identity.character_name` is a fact inside the document that only a caller with a view model can
extract. A projection would require the repository to know the sheet's shape, which is the one thing
`pkg/domain/character` is built not to know. If enumeration ever becomes slow enough to matter, the
move is a summary type populated by SQLite's `json_extract` in the adapter, with the in-memory
backend decoding to match — and that is a decision worth its own record, because it puts schema
knowledge into a storage adapter for the first time.

Nothing here proposes an HTTP surface for listing. `CharacterResponse`'s doc comment notes there is
no list counterpart in `v1alpha` because the port had nothing to list; that stays true until someone
wants `GET /characters`, which is a separate decision with a wire-contract cost.

## Consequences

- **Charm dependencies land in the root module**, because `pkg/tui` lives there. `cmd/dmostd` never
  imports `pkg/tui`, so under module-graph pruning its own `go.mod` should not gain them — but that
  is an expectation, not a verified fact, and it must be **confirmed when the code lands** with
  `(cd cmd/dmostd && go mod tidy && go build ./...)`. If it turns out the daemon's graph does grow,
  the fix is to move the UI into `cmd/dmosh/internal/tui` and give up the reusability.
- **`modernc.org/sqlite` and `golang-migrate` enter `cmd/dmosh`'s graph.** Both are CGo-free, so the
  binary stays `CGO_ENABLED=0`-clean and a `scratch` image remains possible if `dmosh` is ever
  containerized — which matters less for a TUI than for a daemon, but costs nothing to preserve.
- **A third module means three invocations, not two.** Every command in CLAUDE.md's "Build & test"
  section grows a `(cd cmd/dmosh && ...)` clause, and the warning that no pattern rooted at the repo
  covers everything now applies one module more strongly. Updating CLAUDE.md is follow-up work for
  whoever lands the module; this record does not edit it.
- **Two composition roots now exist, wiring different backends** — `dmostd` on `inmem`, `dmosh` on
  `sqlite`. They will drift. The mitigation is that they drift *visibly*, in two short files that do
  nothing but wiring.
- **Every editable section costs a hand-written view model**, per decision 6, and schema additions
  do not appear in the UI for free. This is the accepted price of not dropping unknown fields.
- **Patched sheets are normalized**, per decision 7: keys sorted, whitespace gone. Anyone comparing
  stored bytes to what they originally submitted will see a difference after the first save.
- **No auth, no ownership, single user.** `owner_user_id` is server-owned and unpatchable, there is
  no user to attribute a sheet to, and the SQLite database has no `users` table — the User
  repository is in-memory only. `dmosh` operates one local database as one implicit person. Nothing
  here should be read as a design for multi-user editing.
- **Play-state tracking is not addressed.** Current HP, spell slots and conditions are ordinary
  sheet fields under this design, edited like any other. Whether tracking them during a session
  wants a different interaction model — and a different write cadence than decision 8's explicit
  save — is a question for a later record.
