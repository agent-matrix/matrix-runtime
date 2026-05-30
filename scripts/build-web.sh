#!/usr/bin/env bash
# Build the enterprise console bundle (web/src/app.jsx -> web/static/app.js).
#
# Uses esbuild to transpile JSX and bundle into a single minified file. React and
# ReactDOM are provided at runtime by the vendored UMD builds in web/static/vendor,
# so the console needs no CDN access — important for air-gapped / on-prem installs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ESBUILD_VERSION="${ESBUILD_VERSION:-0.21.5}"

echo "==> building console bundle with esbuild@${ESBUILD_VERSION}"
npx --yes "esbuild@${ESBUILD_VERSION}" "$ROOT/web/src/app.jsx" \
  --loader:.jsx=jsx \
  --jsx=transform \
  --bundle \
  --minify \
  --target=es2019 \
  --outfile="$ROOT/web/static/app.js"

echo "==> wrote $ROOT/web/static/app.js ($(wc -c <"$ROOT/web/static/app.js") bytes)"
