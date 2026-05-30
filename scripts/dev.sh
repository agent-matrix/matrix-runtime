#!/usr/bin/env bash
# Run matrix-runtime in local-dev mode with live rebuilds on demand.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export MATRIX_RUNTIME_MODE="${MATRIX_RUNTIME_MODE:-local-dev}"
export MATRIX_RUNTIME_DATA_DIR="${MATRIX_RUNTIME_DATA_DIR:-$ROOT/.devdata}"

echo "==> mode=$MATRIX_RUNTIME_MODE data_dir=$MATRIX_RUNTIME_DATA_DIR"
cd "$ROOT"
exec go run ./cmd/matrix-runtime --mode "$MATRIX_RUNTIME_MODE"
