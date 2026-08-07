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

## Architecture

Layered / ports-and-adapters, with dependencies pointing inward toward `pkg/domain`:

```
cmd/dmostd  →  pkg/http  →  pkg/app  →  pkg/domain  ←  pkg/inmem
                    ↘         ↙
                     pkg/dto (v1alpha wire types + mappers)
```

- **`pkg/domain`** — pure domain. `User` has *all fields unexported* with read-only getters; it can only be constructed via `NewUser` (validating) or `UserFactory.Rehydrate` (non-validating, for adapters reconstructing persisted/decoded state). `UserRepository` is the port. Value objects (`UserID`, `Email`) live in `vo.go`; sentinel errors wrap `ErrInvalid` so callers use `errors.Is`.
- **`pkg/app`** — use-case layer. `app.App` is a struct of service interfaces (`service.go` declares them, `services/` implements them). `app.New()` wires the default in-memory stack; `cmd/dmostd` builds its own `app.App` explicitly.
- **`pkg/dto/v1alpha`** — versioned wire contract, deliberately separate from domain shapes (e.g. v1alpha's flat `name` splits into first/last; v1alpha's `username` maps to domain `handle`). All translation lives in `pkg/dto/v1alpha/mapper` (`package mapper`), which imports the DTOs aliased as `dto` to avoid shadowing `encoding/json`. Domain types must never grow JSON tags. Versioned media types are declared here (`application/vnd.dmost.user.v1alpha+json`).

  These types are JSON-specific despite the format-neutral path: they carry `json:` tags, and their shapes encode JSON-contract decisions (`name` as one `"First Last"` string, `created_at` as an RFC3339 string). A second format does **not** reuse them — gRPC would use generated `.pb.go` types, and XML would want its own shapes on an independent version line. If a second format ever lands, add a format axis then (`pkg/dto/json/v1alpha`, `pkg/dto/xml/v1alpha`); don't add `xml:` tags to these structs.
- **`pkg/http`** — gorilla/mux server *and* a matching `Client` in the same file as each resource's handlers (`user.go` holds `registerUserRoutes`, `CreateHandler`, `ListHandler`, and the client's `Create`/`ListAll`). Keep that pairing: end-to-end tests in `cmd/dmostd` drive the real server through this client.
- **`pkg/http/problem`** — RFC 7807 `application/problem+json` builder. All error responses go through it (`problem.New().Of(code).WriteTo(w)`); use `Wrap` to expose the cause as `reason` in the body and `WrapSilent` to keep it server-side only.
- **`pkg/inmem`** — in-memory `UserRepository` adapter; the only storage backend today.

`Server.serveHTTP` wraps the router to do two things mux cannot: honor a `_method` form override on POSTs, and strip a `.json` URL suffix while setting the JSON `Accept`/`Content-Type` headers (so `curl /users.json` works).

`Server.SetApp` copies into the pre-allocated `*app.App` the handlers captured at construction — handlers hold the pointer, so the app must be set through this method rather than reassigned.

## Testing patterns

- `cmd/dmostd/*_test.go` are end-to-end: `MustRunMain` starts the real server on `:0` (random port) so tests run in parallel, and drives it via `http.Client`.
- `pkg/http/user_test.go` uses `httptest` with a `FakeUserService` satisfying `app.UserService` — prefer fakes over mocks here.
- `github.com/stretchr/testify` (`require` for fatal preconditions, `assert` for checks) is the assertion library.

## Conventions

- Commits follow Conventional Commits (`feat(cmd/dmostd): ...`, `chore: ...`).
- `/cmd` = executables, `/pkg` = reusable libraries, `/public` = web applications (currently empty).
- Adding a wire-format change means a *new* `v1alpha`-style package, not edits to shipped DTOs.
