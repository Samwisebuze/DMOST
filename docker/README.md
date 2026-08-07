# Docker build for `cmd/dmostd`

Multi-stage build producing two images: a `scratch` release image containing only the
application binary, and an Alpine debug image running the daemon under a headless
[Delve](https://github.com/go-delve/delve) server.

```
base ──┬── test ──── builder ─────────────── release  (scratch, binary only)
       └── debug-build ─────────────────────  debug   (alpine + dlv on :2345)
```

## Prerequisites

BuildKit is required — the Dockerfile uses `RUN --mount=type=cache`. Docker's legacy
builder cannot parse it.

```bash
sudo apt-get install -y docker-buildx    # Ubuntu; provides the buildx CLI plugin
docker buildx version
```

## Build

Always build **from the repository root**. The root module holds `pkg/...` and
`cmd/dmostd` is a nested module resolving it through `replace ... => ../..`, so the
context must include both the root `go.mod` and `cmd/dmostd/go.mod`.

```bash
# Release image. Runs `go vet` + `go test` on the way through; a failure here
# fails the build and produces no image.
docker build -f docker/Dockerfile -t dmostd:dev .

# Debug image.
docker build -f docker/Dockerfile --target debug -t dmostd:debug .

# Just the tests, no release image.
docker build -f docker/Dockerfile --target test .
```

Note that `--target builder` does **not** skip the tests — `builder` depends on `test` by
design. To compile without running them (when iterating on the build itself), target
`debug-build`, which produces a binary off `base` directly:

```bash
docker build -f docker/Dockerfile --target debug-build .
```

`release` is deliberately the **last** stage in the Dockerfile: `docker build` with no
`--target` builds the final stage, so the production image is the default. Moving it
would also drop `test` out of the default build's dependency graph, since the
`COPY --from=test /tests.ok` line in `builder` is the only edge that keeps it there.

## Run

```bash
docker run --rm -p 8080:8080 dmostd:dev

curl -s localhost:8080/healthz
curl -s localhost:8080/users.json
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' \
     -d '{"name":"Ada Lovelace","username":"ada","email":"ada@example.com"}'
```

## Compose

`compose/docker-compose.yml` wraps both images as services. Requires Compose v2
(`docker compose`); v1's legacy builder cannot parse the Dockerfile's cache mounts.

```bash
docker compose -f docker/compose/docker-compose.yml up --build        # dmostd on :8080
docker compose -f docker/compose/docker-compose.yml down
```

`context: ../..` is resolved relative to the compose file rather than the working
directory, so the command above works from anywhere and BuildKit still gets the
repository root that the build requires. `--build` runs the tests, for the same reason
a plain `docker build` does.

`DMOSTD_PORT` moves the published port; the container side stays `:8080`, which is
hard-coded in `http.NewServer()`.

```bash
DMOSTD_PORT=9090 docker compose -f docker/compose/docker-compose.yml up --build
```

The `debug` service sits behind a profile, so `up` does not start it and it never
contends with `dmostd` for `:8080`. It carries the `SYS_PTRACE` and AppArmor settings
described below, and publishes Delve on `DLV_PORT` (default `2345`):

```bash
docker compose -f docker/compose/docker-compose.yml --profile debug up --build debug
```

The release service runs `read_only` with `cap_drop: ALL` and `no-new-privileges` — the
image is a single static binary running as UID 65532, so none of that costs it anything.
Its `healthcheck` re-executes the binary as its own probe — see
[Notes](#notes-and-limitations) — and adds `start_interval`, so a fresh container reports
`healthy` in about a second rather than waiting out the 30s interval.

## Debug

The debug image needs `ptrace`, which the default container profiles block:

```bash
docker run --rm -p 8080:8080 -p 2345:2345 \
  --cap-add=SYS_PTRACE --security-opt apparmor=unconfined \
  dmostd:debug
```

Add `--security-opt seccomp=unconfined` if Delve still cannot attach.

The daemon starts immediately and serves on `:8080` (the entrypoint passes `--continue`);
attach whenever you like, and `--accept-multiclient` lets you detach and reattach.

```bash
dlv connect localhost:2345
```

From a VS Code `launch.json`:

```json
{
  "name": "Attach to dmostd (docker)",
  "type": "go",
  "request": "attach",
  "mode": "remote",
  "port": 2345,
  "host": "127.0.0.1",
  "remotePath": "/src",
  "cwd": "${workspaceFolder}"
}
```

The debug binary is built with `-gcflags="all=-N -l"` and keeps its DWARF, and `/src`
holds the sources the binary was compiled from, so source-level stepping and variable
inspection both work.

One gotcha when picking a breakpoint: `CreateHandler` and `ListHandler` are
*constructors* that return an `http.HandlerFunc`. They run once at route registration,
so a breakpoint on the function itself never fires per-request. Break on a line inside
the returned closure instead:

```
(dlv) break /src/pkg/http/user.go:70
(dlv) continue
```

## Pinned versions

| Component | Pin | Notes |
| --- | --- | --- |
| Dockerfile frontend | `docker/dockerfile:1.26.0` | |
| Build / test stages | `golang:1.26.3-alpine` | keep in step with the `go` directive in both `go.mod` files |
| Debug runtime | `alpine:3.22` | |
| Delve | `v1.27.1` | supports Go 1.25–1.27 |
| Release runtime | `scratch` | |

## Notes and limitations

- **`CGO_ENABLED=0` is what makes `scratch` viable.** The default (`CGO_ENABLED=1`)
  produces a dynamically-linked binary that cannot run there.
- **The release image really is one file.** Verify by unpacking the image's single layer:
  ```bash
  d=$(mktemp -d) && docker save dmostd:dev | tar -x -C "$d"
  for b in "$d"/blobs/sha256/*; do tar -tvf "$b" 2>/dev/null; done
  # -rwxr-xr-x 0/0  7913634  dmostd
  ```
  Use this rather than `docker export`: exporting a *container* additionally shows
  `/dev`, `/etc/hosts`, `/proc` and friends, which the runtime injects and which are not
  part of the image.
- **The `HEALTHCHECK` re-executes the binary.** Docker health checks run *inside* the
  container, and `scratch` has no shell, `curl` or `wget` — so `dmostd` is its own
  client: `/dmostd -healthcheck` `GET`s `/healthz` on `127.0.0.1:8080` and exits 0 or 1
  (`cmd/dmostd/healthcheck.go`). It has to be declared in **exec** form,
  `CMD ["/dmostd", "-healthcheck"]`; the shell form runs through a `/bin/sh` that is not
  in the image.
  ```bash
  docker inspect --format '{{.State.Health.Status}}' <container>
  ```
  Note that `/healthz` is a liveness check only — it reports that the process can answer
  requests, and inspects no services, so it will not go unhealthy over a sick dependency.
  That is what makes it safe as a restart signal.
- **No CA certificates.** Harmless today — `pkg/http/server.go` only reaches for
  `autocert` when `Server.Domain` is set, and nothing sets it. Enabling TLS would require
  adding `ca-certificates` and a writable autocert cache, which `scratch` cannot provide.
- **No runtime configuration.** The listen address is hard-coded to `:8080` in
  `http.NewServer()`, and `cmd/dmostd/main.go` still carries `//TODO: Load config from env`.
- **`.dockerignore` must keep `*_test.go`** — the `test` stage runs them.
- **No single pattern covers both modules.** `cmd/dmostd` is nested, so from `/src`
  neither `./...` nor `github.com/samwisebuze/dmost/...` reaches it — a lone invocation
  would skip the e2e tests and still exit 0. The `test` stage therefore runs `go vet` and
  `go test` once at the root and again inside `cmd/dmostd`, and the build stages `cd`
  into `cmd/dmostd` before `go build .`.
