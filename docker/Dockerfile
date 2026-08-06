# syntax=docker/dockerfile:1.26.0
#
# Multi-stage build for cmd/dmostd.
#
#   base ──┬── test ──── builder ──── release  (scratch, binary only)
#          └── debug-build ────────── debug    (alpine + delve on :2345)
#
# Build from the REPOSITORY ROOT — this is a go.work workspace with no root
# module, so the context must contain go.work and all six module directories:
#
#   docker build -f docker/Dockerfile -t dmostd:dev .
#   docker build -f docker/Dockerfile --target debug -t dmostd:debug .
#
# Requires BuildKit (the `docker-buildx` package) for the cache mounts below.


# ---------------------------------------------------------------------------
# base — toolchain + downloaded module dependencies, shared by every stage.
# ---------------------------------------------------------------------------
# Keep this tag in step with the `go` directive in go.work and the go.mod files.
FROM golang:1.26.3-alpine AS base

# CGO_ENABLED=0 is what makes the binary static enough for a scratch image;
# the default (CGO_ENABLED=1) produces a dynamically-linked ELF that cannot run
# there. -buildvcs=false because .git is excluded from the build context.
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-buildvcs=false

WORKDIR /src

# Copy only the module graph first so the (slow) dependency download stays
# cached when application source changes.
COPY go.work go.work.sum ./
COPY cmd/dmostd/go.mod cmd/dmostd/go.sum cmd/dmostd/
COPY pkg/app/go.mod    pkg/app/go.sum    pkg/app/
COPY pkg/domain/go.mod pkg/domain/go.sum pkg/domain/
COPY pkg/dto/go.mod    pkg/dto/go.sum    pkg/dto/
COPY pkg/http/go.mod   pkg/http/go.sum   pkg/http/
COPY pkg/inmem/go.mod  pkg/inmem/go.sum  pkg/inmem/

# Run at the workspace root, which resolves all six modules in one pass.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .


# ---------------------------------------------------------------------------
# test — vet + unit/e2e tests. `builder` depends on this stage, so a failure
# here fails the whole build and no image is produced.
# ---------------------------------------------------------------------------
FROM base AS test

# Import-path patterns, not ./... — the latter fails at a go.work root with
# "directory prefix . does not contain modules listed in go.work".
#
# -count=1 disables Go's test result cache, which the GOCACHE mount would
# otherwise carry across builds and turn into a no-op.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go vet github.com/samwisebuze/dmost/... \
 && go test -count=1 github.com/samwisebuze/dmost/... \
 && touch /tests.ok


# ---------------------------------------------------------------------------
# builder — the release binary.
# ---------------------------------------------------------------------------
FROM base AS builder

# Load-bearing: this is the only edge from the release image back to `test`.
# Without it BuildKit prunes the test stage out of the graph and a default
# build would never run the tests.
COPY --from=test /tests.ok /tests.ok

# -trimpath removes absolute build paths; -s -w drops the symbol table and
# DWARF, which is most of the binary's size.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/dmostd \
      github.com/samwisebuze/dmost/cmd/dmostd


# ---------------------------------------------------------------------------
# debug-build — delve, plus an unoptimized build that is actually debuggable.
# ---------------------------------------------------------------------------
FROM base AS debug-build

# Delve v1.27.1 supports Go 1.25-1.27, so it matches the 1.26.3 toolchain.
# GOWORK=off keeps `go install pkg@version` from being evaluated against the
# workspace in the current directory.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOWORK=off go install github.com/go-delve/delve/cmd/dlv@v1.27.1

# Deliberately NOT -trimpath and NOT -ldflags="-s -w": delve needs the DWARF
# and the original source paths. -N disables optimization, -l disables inlining,
# so stepping and variable inspection behave the way the source reads.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -gcflags="all=-N -l" -o /out/dmostd \
      github.com/samwisebuze/dmost/cmd/dmostd


# ---------------------------------------------------------------------------
# debug — headless delve on :2345 wrapping the daemon on :8080.
#
# Alpine rather than scratch: a debug image wants a shell, and both binaries
# are static (CGO_ENABLED=0) so musl is irrelevant to them.
#
# Needs ptrace, which the default container profiles block:
#   docker run --rm -p 8080:8080 -p 2345:2345 \
#     --cap-add=SYS_PTRACE --security-opt apparmor=unconfined dmostd:debug
# ---------------------------------------------------------------------------
FROM alpine:3.22 AS debug

COPY --from=debug-build /go/bin/dlv    /usr/local/bin/dlv
COPY --from=debug-build /out/dmostd    /usr/local/bin/dmostd
# The binary's DWARF references /src/..., so ship the source for stepping.
COPY --from=debug-build /src           /src

WORKDIR /src
EXPOSE 8080 2345

# Flag order matters: dlv's usage is `dlv exec <binary> [flags]`, and --continue
# is a flag of the exec subcommand rather than a root flag, so everything has to
# follow the binary path.
#
# --listen=:2345 rather than the default 127.0.0.1:0, so the port is reachable
# from outside the container. --continue starts the daemon immediately instead
# of blocking until a client attaches; drop it if you need to break on something
# during startup. --accept-multiclient is what allows detaching and reattaching.
ENTRYPOINT ["dlv", "exec", "/usr/local/bin/dmostd", \
            "--headless", "--listen=:2345", "--api-version=2", \
            "--accept-multiclient", "--continue"]


# ---------------------------------------------------------------------------
# release — scratch, containing the application binary and nothing else.
#
# MUST be the last stage in this file: `docker build` with no --target builds
# the final stage, and the production image is the sensible default. Keeping it
# here is also what pulls `test` into the default build's dependency graph.
# ---------------------------------------------------------------------------
FROM scratch AS release

COPY --from=builder /out/dmostd /dmostd

# A numeric UID needs no /etc/passwd, so running unprivileged costs no extra
# files. :8080 is above 1024, so an unprivileged user can bind it.
USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/dmostd"]
