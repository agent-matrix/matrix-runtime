#!/usr/bin/env bash
# Install matrix-runtime locally, via Docker, or to Kubernetes.
set -euo pipefail

MODE="local"
RUNTIME_MODE="${MATRIX_RUNTIME_MODE:-local-dev}"
NAMESPACE="${NAMESPACE:-matrix-runtime}"

usage() {
  cat <<EOF
Usage: $0 --mode <docker|kubernetes|local> [options]

  --mode docker        Build and run via docker compose
  --mode kubernetes    Apply raw manifests in deploy/k8s
  --mode local         Build the binary and run it (default)

Environment:
  MATRIX_RUNTIME_MODE  Runtime mode (default: local-dev)
  NAMESPACE            Kubernetes namespace (default: matrix-runtime)
EOF
}

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
    echo "==> Starting matrix-runtime via docker compose"
    docker compose -f "$ROOT/deploy/docker-compose/docker-compose.yml" up --build -d
    ;;
  kubernetes)
    echo "==> Applying Kubernetes manifests to namespace $NAMESPACE"
    kubectl apply -f "$ROOT/deploy/k8s/namespace.yaml"
    kubectl apply -f "$ROOT/deploy/k8s/configmap.yaml"
    [[ -f "$ROOT/deploy/k8s/secret.yaml" ]] && kubectl apply -f "$ROOT/deploy/k8s/secret.yaml" || \
      echo "    (skipping secret; copy secret.example.yaml to secret.yaml and edit it)"
    kubectl apply -f "$ROOT/deploy/k8s/pvc.yaml"
    kubectl apply -f "$ROOT/deploy/k8s/deployment.yaml"
    kubectl apply -f "$ROOT/deploy/k8s/service.yaml"
    ;;
  local)
    echo "==> Building matrix-runtime binary"
    (cd "$ROOT" && go build -o bin/matrix-runtime ./cmd/matrix-runtime)
    echo "==> Starting matrix-runtime (mode=$RUNTIME_MODE)"
    exec "$ROOT/bin/matrix-runtime" --mode "$RUNTIME_MODE"
    ;;
  *)
    echo "unknown mode: $MODE" >&2; usage; exit 1 ;;
esac
