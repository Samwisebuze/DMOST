#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/dmost-go-build-cache}"
./scripts/check-web.sh
unformatted="$(gofmt -l $(rg --files -g '*.go' -g '!docs/psd/**' -g '!.claude/**'))"
if [[ -n "$unformatted" ]]; then
  echo "Unformatted Go files: $unformatted" >&2
  exit 1
fi
for module in . cmd/dmostd cmd/dmosh; do
  echo "== $module =="
  (cd "$module" && go vet ./... && go test -count=1 ./... && go build ./...)
done
