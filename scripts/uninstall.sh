#!/usr/bin/env bash
# Remove a matrix-runtime install (local/docker/kubernetes).
set -euo pipefail

MODE="local"
NAMESPACE="${NAMESPACE:-matrix-runtime}"

usage() { echo "Usage: $0 --mode <docker|kubernetes|local>"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 1 ;;
  esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

case "$MODE" in
  docker)
    docker compose -f "$ROOT/deploy/docker-compose/docker-compose.yml" down -v
    ;;
  kubernetes)
    kubectl delete -n "$NAMESPACE" deployment,service,configmap,secret,pvc -l app=matrix-runtime --ignore-not-found
    kubectl delete namespace "$NAMESPACE" --ignore-not-found
    ;;
  local)
    pkill -f "bin/matrix-runtime" || true
    rm -rf "$ROOT/bin"
    echo "Removed local binary and stopped any running process."
    ;;
  *)
    echo "unknown mode: $MODE" >&2; usage; exit 1 ;;
esac
