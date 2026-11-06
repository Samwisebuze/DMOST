#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/dmost-go-build-cache}"
tmp="$(mktemp -d)"
trap 'kill "${pid:-}" 2>/dev/null || true; wait "${pid:-}" 2>/dev/null || true; rm -rf "$tmp"' EXIT
(cd cmd/dmostd && go build -o "$tmp/dmostd" .)
"$tmp/dmostd" --local-https --advertise 127.0.0.1 --listen 127.0.0.1:0 --bootstrap-listen 127.0.0.1:0 --cert-dir "$tmp/certs" >"$tmp/host.log" 2>&1 &
pid=$!
for ((i=0; i<100; i++)); do
  address="$(sed -n 's/^Open on your device: //p' "$tmp/host.log" | head -1)"
  if [[ -n "$address" ]]; then break; fi
  if ! kill -0 "$pid" 2>/dev/null; then cat "$tmp/host.log" >&2; exit 1; fi
  sleep .1
done
if [[ -z "${address:-}" ]]; then cat "$tmp/host.log" >&2; exit 1; fi
[[ "$(rg -c '^Open on your device: ' "$tmp/host.log")" == 1 ]]
curl --noproxy '*' --fail --silent "$address" >"$tmp/bootstrap.html"
curl --noproxy '*' --fail --silent "${address}dmost-ca.cer" >"$tmp/ca.cer"
app="$(sed -n 's/.*href="\(https:[^"]*\)".*/\1/p' "$tmp/bootstrap.html" | head -1)"
if [[ -z "$app" ]]; then echo 'HTTPS app link missing' >&2; exit 1; fi
curl --noproxy '*' --fail --silent --cacert "$tmp/certs/ca.pem" "$app" >"$tmp/app.html"
rg -q 'DMOST Local Table' "$tmp/app.html"
echo 'Local bootstrap and trusted HTTPS app: pass'
