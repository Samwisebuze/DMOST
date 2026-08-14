# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

DMOST ("Dungeon Master's Open Source Toolkit") — a personal project exploring native, local-first application design in Go. Module path prefix: `github.com/samwisebuze/dmost`.

## Build & test

There are **two modules and no `go.work`** — each resolves on its own:

- `github.com/samwisebuze/dmost` at the repo root — everything under `pkg/`.
- `github.com/samwisebuze/dmost/cmd/dmostd`, a **nested** module for the daemon. It `require`s the root module and pins it with `replace github.com/samwisebuze/dmost => ../..`.

**No pattern rooted at the repo covers both.** `cmd/dmostd` is a separate module, so from the root neither `./...` nor `github.com/samwisebuze/dmost/...` matches it — they skip the daemon and its e2e tests and still exit 0. Anything that must cover everything runs twice:

```bash
go build ./... && (cd cmd/dmostd && go build ./...)
go test  ./... && (cd cmd/dmostd && go test ./...)
go vet   ./... && (cd cmd/dmostd && go vet ./...)

go test ./pkg/domain/...                                     # one package tree
(cd cmd/dmostd && go test -run TestUsersAPI_Create ./...)    # one test
```

Run the daemon: `(cd cmd/dmostd && go run .)` — listens on `:8080`, debug JSON logs to stdout. `go run github.com/samwisebuze/dmost/cmd/dmostd` does **not** work from the root.

Dependency changes mean `go mod tidy` in whichever module changed — the two `go.mod`/`go.sum` pairs are independent, and a dep used by both is listed in both.

New packages under `pkg/` need no `go.mod`; they join the root module. Only add a new module (with a `replace` back to the root) for another `cmd/` executable that should version independently — and extend the two-invocation commands above to cover it.

### Reading the API

**Don't take a type or function inventory from this file — it goes stale.** Ask the toolchain:

```bash
go doc ./pkg/domain              # package summary: exported types and their constructors
go doc ./pkg/domain User         # one symbol, with its methods
go doc -all ./pkg/domain         # every doc comment in the package
go doc -u ./pkg/domain User      # include unexported fields and functions
go doc -short ./pkg/http         # one line per symbol
```

This works on **dependencies too** — read a library's real API instead of guessing at it or reaching for the web:

```bash
go doc github.com/gorilla/mux              # or just `go doc mux` — a path suffix resolves
go doc github.com/gorilla/mux Router.PathPrefix
go doc github.com/stretchr/testify/require
go doc net/http.Handler
```

`-u` is load-bearing on our own packages: entities keep every field unexported, so the default view shows only `// Has unexported fields.` The doc comments *are* the design record — `go doc -all ./pkg/domain` is ~160 lines of rationale this file deliberately does not restate.

Two gotchas:

- **The import-path form resolves whatever is in the current module's dependency graph** — its own packages, its requirements, the stdlib. `cmd/dmostd` is the one exception: the requirement points *from* it *to* the root, never back, so from the root `go doc github.com/samwisebuze/dmost/cmd/dmostd` fails with `cannot find package` while `go doc ./cmd/dmostd` resolves. Anything under `pkg/` or `internal/` takes either form from either module. A dependency of only one module (`gorilla/mux` is direct in the root, indirect in `cmd/dmostd`) needs no special handling — both graphs contain it.
- **`package main` needs `-cmd`.** Without it `go doc ./cmd/dmostd` prints *nothing* and still exits 0 — exported symbols are elided for commands.

### gopls

Installed, but at `$(go env GOPATH)/bin/gopls`, which is **not on `PATH`** — anything shelling out to it fails with `ENOENT` until that's fixed:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Reach for it where `go doc` can't help — references, and outlines that include unexported declarations:

```bash
gopls check pkg/http/user.go             # diagnostics for a file
gopls symbols pkg/domain/user.go         # outline, unexported included
gopls references pkg/domain/user.go:40:6 # 1-based line:col, on the identifier
```

**`gopls references` has the same blind spot as `go test ./...`, and it is just as silent.** gopls loads the module rooted at its working directory, so neither vantage point sees the whole repo — and each returns a confident, incomplete answer:

| run from | refs to `domain.User` | misses |
| --- | --- | --- |
| repo root | 86 (`pkg/`, `internal/`) | both hits in `cmd/dmostd/user_test.go` |
| `cmd/dmostd` | 20 (`cmd/`, `pkg/`) | most of the root module's |

Neither is a superset. For a rename or an impact check, run it from both and union — or confirm against `grep -rn`, which is bounded by the filesystem rather than by a module graph.

### Docker

Build **from the repository root** — the context needs both `go.mod`s. BuildKit is required (the Dockerfile uses `RUN --mount=type=cache`). `docker/README.md` has the full account; the parts that bite:

```bash
docker build -f docker/Dockerfile -t dmostd:dev .                    # release: scratch, one binary
docker build -f docker/Dockerfile --target debug -t dmostd:debug .   # alpine + delve on :2345
docker compose -f docker/compose/docker-compose.yml up --build
```

- **The image build runs the test suite.** `builder` does `COPY --from=test /tests.ok`, and that line is the only edge keeping `test` in the default build graph — don't remove it, and keep `release` as the last stage in the file.
- **Bumping Go touches three places in lockstep:** the `go` directive in *both* `go.mod`s and the `golang:` tag in the Dockerfile.
- `.dockerignore` must keep `*_test.go` — the `test` stage needs them.
- `scratch` has no shell, so the health check re-executes the binary (`dmostd -healthcheck`, `cmd/dmostd/healthcheck.go`) and must be declared in exec form.

## Architecture

Layered / ports-and-adapters, with dependencies pointing inward toward `pkg/domain`:

```
cmd/dmostd  →  pkg/http  →  pkg/app  →  pkg/domain  ←  internal/infra/inmem
                    ↘         ↙                     ←  internal/infra/sqlite
              internal/dto (v1alpha wire types + mappers)
```

- **`pkg/domain`** — pure domain. Entities keep *all fields unexported* behind read-only getters and embed `Aggregate[T ID]` (`aggregate.go`), which owns identity, `CreatedAt`, and a `version`. An entity is constructible only via its validating constructor or the non-validating `Rehydrate` on its factory (for adapters reconstructing persisted/decoded state). Value objects live in `vo.go`, repository ports beside the entity.

  Versioning is a **persistence** concern, not a domain rule: mutators leave it alone, and only a repository advances it — through `UserFactory.NextVersion`, called from inside the critical section that has already compare-and-set on `Version` (`inmem.Save` is the reference implementation). A caller writing back a version the store has moved past gets `ErrConflict`.

  Callers match errors with `errors.Is`. Most sentinels wrap `ErrInvalid` — but `ErrNotFound` and `ErrConflict` deliberately do **not**: losing a race is not a malformed request.
- **`pkg/app`** — use-case layer. `app.App` is a struct of service interfaces (`service.go` declares them, `services/` implements them). `app.New()` wires the default in-memory stack; `cmd/dmostd` builds its own `app.App` explicitly.
- **`internal/dto/v1alpha`** — versioned wire contract, deliberately separate from domain shapes (e.g. v1alpha's flat `name` splits into first/last; v1alpha's `username` maps to domain `handle`). All translation lives in `internal/dto/v1alpha/mapper` (`package mapper`), which imports the DTOs aliased as `dto` so call sites read as a direction. Domain types must never grow JSON tags. Versioned media types are declared here (`application/vnd.dmost.user.v1alpha+json`).

  **Stability decides the path.** Pre-release versions (alpha, beta) live under `internal/dto/`; stable, published ones under `pkg/dto/`. `internal/` keeps an unstable contract from becoming an outside caller's dependency, and a version graduates by moving out. `pkg/dto` therefore holds no versions yet — only a `doc.go` stating the promise. Both `doc.go` files carry the full rationale; keep them and this section in agreement.

  This works across the module boundary: `internal` is enforced by **import path prefix**, so the nested `cmd/dmostd` module can import `internal/dto` (its path is under `github.com/samwisebuze/dmost`), and its e2e tests do.

  These types are JSON-specific despite the format-neutral path: they carry `json:` tags, and their shapes encode JSON-contract decisions (`name` as one `"First Last"` string, `created_at` as an RFC3339 string). A second format does **not** reuse them — gRPC would use generated `.pb.go` types, and XML would want its own shapes on an independent version line. If a second format ever lands, add a format axis then (`internal/dto/json/v1alpha`, `internal/dto/xml/v1alpha`); don't add `xml:` tags to these structs.
- **`pkg/http`** — gorilla/mux server *and* a matching `Client` in the same file as each resource's handlers: `user.go` holds the route registration, every handler, and the client methods that call them; `health.go` does the same for `/healthz`. Keep that pairing — end-to-end tests in `cmd/dmostd` drive the real server through this client, so a new handler without a client method leaves them unable to reach it.
- **`pkg/http/problem`** — RFC 7807 `application/problem+json` builder. All error responses go through it (`problem.New().Of(code).WriteTo(w)`); use `Wrap` to expose the cause as `reason` in the body and `WrapSilent` to keep it server-side only.
- **`internal/infra`** — outbound technical adapters: the code answering *how* something is stored or fetched, usually by owning a connection to something outside the process. One subdirectory per technology, each holding the connection *and* the port implementations that need it (a SQL-backed `UserRepository` lives beside the `*DB` it queries). Repo-private for the same reason `internal/dto` is — a storage backend is an implementation detail, not a published contract.

  **Connections are modeled as processes**, in the shape `pkg/http.Server` set: exported plain fields for configuration, a no-argument constructor returning something already runnable on defaults, and `Open() error` / `Close() error`. The constructor allocates and defaults but **does not connect**, so it cannot fail and returns no error; `cmd/dmostd`'s `Main` owns the lifecycle. `internal/infra/doc.go` carries the rationale — keep it and this bullet in agreement.

  **An adapter with no connection still lives here.** `internal/infra/inmem` is the in-memory `UserRepository` — the only storage backend wired into `cmd/dmostd` today — and has nothing to `Open` or `Close`. Placement follows the *role* (it swaps for `sqlite` at the composition root), not the machinery; so a future filesystem- or memory-backed port goes here too rather than under `pkg/`. It is not a test double: its locking, uniqueness scan, and compare-and-set on `Version` are the reference implementation of the versioning rule above.

  `internal/infra/sqlite` is the first connection-owning one. Two things about it generalize: the driver is **`modernc.org/sqlite`** because `docker/Dockerfile` builds with `CGO_ENABLED=0` for the `scratch` image and a cgo binding would break that, and per-connection settings (`foreign_keys`, `busy_timeout`) are folded into the **DSN**, never issued as an `Exec` — an `Exec` after `sql.Open` configures one pooled connection and silently misses the rest. Its default DSN is a *shared-cache* in-memory database with a reserved keepalive connection; bare `":memory:"` gives every pooled connection its own empty database. `go doc -all ./internal/infra/sqlite` has the full account.

  Two consequences of that shared cache worth knowing before touching the package. `Open` caps an **unconfigured** in-memory pool at `MaxOpenConns = 2` — one for the keepalive, one to work with — because the driver answers a shared-cache table-lock conflict by parking the goroutine on `sqlite3_unlock_notify` rather than returning an error, so two contending connections stall instead of failing. And **two, never one**: the keepalive holds a slot for the lifetime of the DB, so a cap of 1 deadlocks every query. `SQLITE_BUSY` on a *file-backed* database is a real returned error, and `retry.go` re-issues it up to `maxRetries` times with 1–8ms jitter.

  **Schema is managed with `golang-migrate`**, from `internal/infra/sqlite/migrations/*.sql` embedded via `go:embed` — the `scratch` image ships only the binary, so a `source/file` driver would find nothing at runtime. The database driver must stay `.../migrate/v4/database/sqlite` (modernc-backed) and never `database/sqlite3` (cgo), for the same `CGO_ENABLED=0` reason as above. `DB.Open` runs migrations unless `DB.AutoMigrate` is cleared. One hazard: golang-migrate's driver implements `Close` by closing the `*sql.DB` you gave it, so `DB.Migrate` deliberately never closes its `*migrate.Migrate` — don't "fix" that. Shipped migrations are never edited; a change is a new numbered pair.

`Server.serveHTTP` wraps the router to do two things mux cannot: honor a `_method` form override on POSTs, and strip a `.json` URL suffix while setting the JSON `Accept`/`Content-Type` headers (so `curl /users.json` works).

`Server.SetApp` copies into the pre-allocated `*app.App` the handlers captured at construction — handlers hold the pointer, so the app must be set through this method rather than reassigned.

## Testing patterns

- `cmd/dmostd/*_test.go` are end-to-end: `MustRunMain` starts the real server on `:0` (random port) so tests run in parallel, and drives it via `http.Client`.
- `pkg/http/user_test.go` uses `httptest` with a `FakeUserService` satisfying `app.UserService` — prefer fakes over mocks here.
- `github.com/stretchr/testify` (`require` for fatal preconditions, `assert` for checks) is the assertion library.
- `internal/test` holds `Must*` constructors that build valid domain objects or fail the test — use them rather than hand-rolling one (`go doc ./internal/test`). The rehydrating helper takes a caller-chosen `UserID` so ordering assertions don't depend on generation order.
- **`internal/test/repotest` holds the rules a repository port owes its callers**, as scenarios any implementation is run through — `inmem` and `sqlite` both call `RunCharacterRepositoryContract`. A rule that is true of the *port* (Save is an upsert, an insert keeps its version, a replacement advances it, a stale write is refused) belongs there so the two backends cannot drift; a rule that is true of one *technology* (how the sheet is physically stored, what a closed pool does) stays in that package's own test file. Only `CharacterRepository` has a contract so far — `inmem/user_test.go` is still standalone.
- `internal/infra/sqlite`'s tests are in-package (`package sqlite`), unlike everything else, so they can reach the pool and assert on stored representation. Repository tests there run the contract against **both** a file-backed and an in-memory DSN, because their locking behavior genuinely differs.

## Conventions

- Commits follow Conventional Commits (`feat(cmd/dmostd): ...`, `chore: ...`).
- `/cmd` = executables, `/pkg` = reusable libraries, `/internal` = repo-private code (test helpers, pre-release DTOs), `/public` = web applications (currently empty).
- Adding a wire-format change means a *new* `v1alpha`-style package, not edits to shipped DTOs.
