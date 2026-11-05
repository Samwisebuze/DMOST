#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
for file in pkg/http/web/*.js pkg/http/web/*.mjs; do
  node --check "$file"
done
node --test pkg/http/web/*.test.mjs
if rg -n 'https?://(cdn\.|unpkg\.com|jsdelivr\.net)' pkg/http/web; then
  echo 'Runtime CDN reference found' >&2
  exit 1
fi
