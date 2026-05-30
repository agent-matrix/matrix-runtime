#!/usr/bin/env bash
# Smoke-test a running matrix-runtime instance.
set -euo pipefail

BASE="${MATRIX_RUNTIME_BASE:-http://localhost:8080}"
START_CMD="${START_CMD:-npx -y @modelcontextprotocol/server-filesystem /tmp}"

say() { printf "\n==> %s\n" "$1"; }

say "Health"
curl -fsS "$BASE/v1/health"; echo

say "Capabilities"
curl -fsS "$BASE/v1/capabilities"; echo

say "Create mcp.test job"
JOB=$(curl -fsS -X POST "$BASE/v1/jobs" \
  -H "Content-Type: application/json" \
  -d "{\"type\":\"mcp.test\",\"ttl_seconds\":120,\"payload\":{\"runtime\":\"node\",\"transport\":\"stdio\",\"start_command\":\"$START_CMD\"}}")
echo "$JOB"
JOB_ID=$(printf '%s' "$JOB" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')

if [[ -z "$JOB_ID" ]]; then
  echo "failed to create job" >&2; exit 1
fi

say "Stream events for up to 8s (live lifecycle)"
timeout 8 curl -sN "$BASE/v1/jobs/$JOB_ID/events" || true

say "Wait for the sandbox to become ready (up to 90s)"
for _ in $(seq 1 90); do
  SNAP=$(curl -fsS "$BASE/v1/jobs/$JOB_ID")
  STATUS=$(printf '%s' "$SNAP" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
  case "$STATUS" in
    error) echo "job errored: $SNAP" >&2; exit 1 ;;
  esac
  # The mcp.test result is populated with the tool list once ready.
  if printf '%s' "$SNAP" | grep -q '"tools"'; then
    echo "  sandbox ready (tools listed)"
    break
  fi
  sleep 1
done

say "Job status"
curl -fsS "$BASE/v1/jobs/$JOB_ID"; echo

say "Cancel job"
curl -fsS -X DELETE "$BASE/v1/jobs/$JOB_ID"; echo

say "model.inspect job"
curl -fsS -X POST "$BASE/v1/jobs" \
  -H "Content-Type: application/json" \
  -d '{"type":"model.inspect","payload":{"model":"hf:Qwen/Qwen2.5-7B-Instruct","revision":"main"}}'; echo

echo
echo "Smoke test complete."
