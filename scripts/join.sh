#!/usr/bin/env bash
# Write the hybrid-cloud join configuration to ~/.matrix/runtime/config.yaml.
set -euo pipefail

CLOUD_URL=""
TOKEN=""
RUNTIME_ID=""
WORKSPACE=""

usage() {
  cat <<EOF
Usage: $0 --cloud-url <url> --token <mxrt_token> [--runtime-id <id>] [--workspace <name>]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cloud-url) CLOUD_URL="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --runtime-id) RUNTIME_ID="$2"; shift 2 ;;
    --workspace) WORKSPACE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "$CLOUD_URL" || -z "$TOKEN" ]]; then
  echo "error: --cloud-url and --token are required" >&2
  usage
  exit 1
fi

CONF_DIR="$HOME/.matrix/runtime"
CONF_FILE="$CONF_DIR/config.yaml"
mkdir -p "$CONF_DIR"

{
  echo "cloud_url: $CLOUD_URL"
  echo "join_token: $TOKEN"
  [[ -n "$RUNTIME_ID" ]] && echo "runtime_id: $RUNTIME_ID"
  [[ -n "$WORKSPACE" ]] && echo "workspace: $WORKSPACE"
} > "$CONF_FILE"
chmod 600 "$CONF_FILE"

echo "Wrote join configuration to $CONF_FILE"
