#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/dmost-go-build-cache}"
state_dir="${DMOST_CERT_DIR:-$PWD/.dmost-local}"
mkdir -p "$state_dir"
log_file="$state_dir/demo.log"
binary="$state_dir/dmostd-demo"
(cd cmd/dmostd && go build -o "$binary" .)
args=(--local-https --cert-dir "$state_dir" --listen "${DMOST_LISTEN:-:8443}" --bootstrap-listen "${DMOST_BOOTSTRAP_LISTEN:-:8081}")
if [[ -n "${DMOST_ADVERTISE:-}" ]]; then args+=(--advertise "$DMOST_ADVERTISE"); fi
"$binary" "${args[@]}" >"$log_file" 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true' EXIT INT TERM
for ((i=0; i<100; i++)); do
  if ! kill -0 "$pid" 2>/dev/null; then cat "$log_file" >&2; exit 1; fi
  address="$(sed -n 's/^Open on your device: //p' "$log_file" | head -1)"
  browser_address="$(sed -n 's/^Open on this computer: //p' "$log_file" | head -1)"
  https_port="$(sed -n 's/^HTTPS port: //p' "$log_file" | head -1)"
  code="$(sed -n 's/^Join code: //p' "$log_file" | head -1)"
  fingerprint="$(sed -n 's/^CA fingerprint: //p' "$log_file" | head -1)"
  if [[ -n "$address" && -n "$browser_address" && -n "$https_port" && -n "$code" && -n "$fingerprint" ]]; then break; fi
  sleep .1
done
if [[ -z "${address:-}" || -z "${browser_address:-}" || -z "${https_port:-}" || -z "${code:-}" || -z "${fingerprint:-}" ]]; then cat "$log_file" >&2; exit 1; fi
printf 'Open this one address on your device: %s\nJoin code: %s\n' "$address" "$code"
printf 'Open on this computer: %s\n' "$browser_address"
printf 'CA fingerprint: %s\n' "$fingerprint"
if command -v systemctl >/dev/null && systemctl is-active --quiet ufw; then
  printf 'UFW is active. The device address may be blocked by the host firewall; no firewall settings were changed.\n'
fi
printf 'Press Ctrl+C to end the table. Host log: %s\n' "$log_file"
wait "$pid"
