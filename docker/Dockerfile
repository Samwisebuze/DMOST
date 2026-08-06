# syntax=docker/dockerfile:1.26.0
#
# Multi-stage build for cmd/dmostd.
#
#   base ──┬── test ──── builder ──── release  (scratch, binary only)
#          └── debug-build ────────── debug    (alpine + delve on :2345)
#
# Build from the REPOSITORY ROOT — the root module holds pkg/..., and the nested
# cmd/dmostd module reaches it through `replace ... => ../..`, so both must be
# in the context:
#
#   docker build -f docker/Dockerfile -t dmostd:dev .
#   docker build -f docker/Dockerfile --target debug -t dmostd:debug .
#
# Requires BuildKit (the `docker-buildx` package) for the cache mounts below.


# ---------------------------------------------------------------------------
# base — toolchain + downloaded module dependencies, shared by every stage.
# ---------------------------------------------------------------------------
# Keep this tag in step with the `go` directive in both go.mod files.
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
COPY go.mod go.sum ./
COPY cmd/dmostd/go.mod cmd/dmostd/go.sum cmd/dmostd/

# One download per module — there is no workspace to resolve both in one pass.
# cmd/dmostd's `replace` points at /src, whose go.mod is already in place above,
# so the root module resolves from disk rather than being fetched.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download \
 && cd cmd/dmostd && go mod download

COPY . .


# ---------------------------------------------------------------------------
# test — vet + unit/e2e tests. `builder` depends on this stage, so a failure
# here fails the whole build and no image is produced.
# ---------------------------------------------------------------------------
FROM base AS test

# Once per module. cmd/dmostd is nested, so no pattern rooted at /src reaches it
# — neither ./... nor github.com/samwisebuze/dmost/... — and a single invocation
# would skip the e2e tests while still exiting 0.
#
# -count=1 disables Go's test result cache, which the GOCACHE mount would
# otherwise carry across builds and turn into a no-op.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go vet ./... \
 && go test -count=1 ./... \
 && cd cmd/dmostd \
 && go vet ./... \
 && go test -count=1 ./... \
 && touch /tests.ok


# ---------------------------------------------------------------------------
# builder — the release binary.
# ---------------------------------------------------------------------------
FROM base AS builder

# Load-bearing: this is the only edge from the release image back to `test`.
# Without it BuildKit prunes the test stage out of the graph and a default
# build would never run the tests.
COPY --from=test /tests.ok /tests.ok

# Built from inside cmd/dmostd: it is its own module, so the package does not
# resolve from /src.
#
# -trimpath removes absolute build paths; -s -w drops the symbol table and
# DWARF, which is most of the binary's size.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/dmostd \
 && go build -trimpath -ldflags="-s -w" -o /out/dmostd .


# ---------------------------------------------------------------------------
# debug-build — delve, plus an unoptimized build that is actually debuggable.
# ---------------------------------------------------------------------------
FROM base AS debug-build

# Delve v1.27.1 supports Go 1.25-1.27, so it matches the 1.26.3 toolchain.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.27.1

# Deliberately NOT -trimpath and NOT -ldflags="-s -w": delve needs the DWARF
# and the original source paths. -N disables optimization, -l disables inlining,
# so stepping and variable inspection behave the way the source reads.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/dmostd \
 && go build -gcflags="all=-N -l" -o /out/dmostd .


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

# The binary is its own health client — see runHealthcheck in cmd/dmostd. This
# has to be the exec form: the shell form runs through /bin/sh, which does not
# exist here. --start-period covers process startup, during which a failing
# probe does not count against --retries.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/dmostd", "-healthcheck"]

ENTRYPOINT ["/dmostd"]
