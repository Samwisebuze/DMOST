# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

DMOST ("Dungeon Master's Open Source Toolkit") — a personal project exploring native, local-first application design in Go. Module path prefix: `github.com/samwisebuze/dmost`.

## Build & test

This is a **multi-module Go workspace** (`go.work`) with no module at the repo root. Because of that, `./...` patterns fail from the root:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
```

Use import-path patterns instead, or `cd` into a module first:

```bash
go build github.com/samwisebuze/dmost/...       # build everything
go test  github.com/samwisebuze/dmost/...       # test everything
go vet   github.com/samwisebuze/dmost/...

go test github.com/samwisebuze/dmost/pkg/domain/...        # one module
go test -run TestUsersAPI_Create github.com/samwisebuze/dmost/cmd/dmostd   # one test
(cd pkg/domain && go test ./...)                            # ./... works inside a module
```

Run the daemon: `go run github.com/samwisebuze/dmost/cmd/dmostd` (listens on `:8080`, debug JSON logs to stdout).

New modules must be added to both `go.work` and given a `go.mod`. Note that intra-repo dependencies are currently resolved *only* through the workspace — the individual `go.mod` files do not `require` sibling modules, so anything relying on `GOWORK=off` (including some `go mod tidy` flows) will not resolve.

## Architecture

Layered / ports-and-adapters, with dependencies pointing inward toward `pkg/domain`:

```
cmd/dmostd  →  pkg/http  →  pkg/app  →  pkg/domain  ←  pkg/inmem
                    ↘         ↙
                     pkg/dto (v1alpha wire types + mappers)
```

- **`pkg/domain`** — pure domain. `User` has *all fields unexported* with read-only getters; it can only be constructed via `NewUser` (validating) or `UserFactory.Rehydrate` (non-validating, for adapters reconstructing persisted/decoded state). `UserRepository` is the port. Value objects (`UserID`, `Email`) live in `vo.go`; sentinel errors wrap `ErrInvalid` so callers use `errors.Is`.
- **`pkg/app`** — use-case layer. `app.App` is a struct of service interfaces (`service.go` declares them, `services/` implements them). `app.New()` wires the default in-memory stack; `cmd/dmostd` builds its own `app.App` explicitly.
- **`pkg/dto/v1alpha`** — versioned wire contract, deliberately separate from domain shapes (e.g. v1alpha's flat `name` splits into first/last; v1alpha's `username` maps to domain `handle`). All translation lives in `.../v1alpha/mapper`; domain types must never grow JSON tags. Versioned media types are declared here (`application/vnd.dmost.user.v1alpha+json`).
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
