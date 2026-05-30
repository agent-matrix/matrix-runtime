#!/usr/bin/env bash
# Deploy the MatrixCloud demo to a Hugging Face Docker Space — self-contained.
#
# This ships the full committed source to the Space and builds matrix-runtime
# there (local build context), so it works whether or not the GitHub repo is
# public. The embedded console + all features come from the current commit.
#
# Usage:
#   HF_TOKEN=hf_xxx ./scripts/deploy-hf-space.sh
#
# Optional env:
#   HF_USERNAME   account/org that owns the Space   (default: ruslanmv)
#   SPACE_NAME    Space name                         (default: matrixcloud)
#
# The token is read only from the environment and used in-memory — never written
# to a file or committed. Pushing publishes a PUBLIC Space. Rotate the token
# afterwards if it was shared anywhere.
set -euo pipefail

HF_USERNAME="${HF_USERNAME:-ruslanmv}"
SPACE_NAME="${SPACE_NAME:-matrixcloud}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ -z "${HF_TOKEN:-}" ]; then
  echo "❌ Set HF_TOKEN (a Hugging Face *write* token): HF_TOKEN=hf_xxx $0" >&2
  exit 1
fi
for f in hf/README.md hf/requirements.txt hf/app/main.py; do
  test -f "$f" || { echo "❌ missing $f" >&2; exit 1; }
done

echo "👤 Verifying token..."
who="$(curl -fsS --max-time 30 -H "Authorization: Bearer ${HF_TOKEN}" \
        https://huggingface.co/api/whoami-v2 | sed -n 's/.*"name":"\([^"]*\)".*/\1/p' | head -1 || true)"
echo "   authenticated as: ${who:-unknown}"

echo "🚀 Ensuring Space ${HF_USERNAME}/${SPACE_NAME} exists (Docker SDK)..."
code="$(curl -s -o /dev/null -w '%{http_code}' -X POST \
  -H "Authorization: Bearer ${HF_TOKEN}" -H 'Content-Type: application/json' \
  -d "{\"type\":\"space\",\"name\":\"${SPACE_NAME}\",\"sdk\":\"docker\",\"private\":false}" \
  https://huggingface.co/api/repos/create || true)"
case "$code" in
  200|201) echo "   created." ;;
  409)     echo "   already exists — will force-update." ;;
  *)       echo "   create returned HTTP ${code} (continuing; push will surface real errors)." ;;
esac

DEPLOY_DIR="$(mktemp -d)"
trap 'rm -rf "${DEPLOY_DIR}"' EXIT
echo "📦 Exporting committed source → ${DEPLOY_DIR} ..."
git archive --format=tar HEAD | tar -x -C "${DEPLOY_DIR}"
# Trim things the Space build doesn't need (docs/ holds PNGs that HF would
# require via Git LFS — and the Space never serves them).
rm -rf "${DEPLOY_DIR}/legacy" "${DEPLOY_DIR}/bin" "${DEPLOY_DIR}/coverage.out" \
       "${DEPLOY_DIR}/.github" "${DEPLOY_DIR}/docs"

# HF Space config: README front-matter + python launcher + a self-contained,
# local-context Dockerfile (overrides the repo's root Dockerfile in the tree).
cp hf/README.md "${DEPLOY_DIR}/README.md"
cp hf/requirements.txt "${DEPLOY_DIR}/requirements.txt"
mkdir -p "${DEPLOY_DIR}/app"
cp hf/app/main.py "${DEPLOY_DIR}/app/main.py"

cat > "${DEPLOY_DIR}/Dockerfile" <<'DOCKER'
# Self-contained Space build: compiles matrix-runtime from the pushed source.
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/matrix-runtime ./cmd/matrix-runtime

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git nodejs npm python3 python3-pip pipx \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/matrix-runtime /usr/local/bin/matrix-runtime
COPY requirements.txt /app/requirements.txt
RUN pip3 install --no-cache-dir --break-system-packages -r /app/requirements.txt
COPY app /app/app
ENV MATRIX_RUNTIME_MODE=cloud-worker \
    MATRIX_RUNTIME_PORT=7860 \
    MATRIX_RUNTIME_DATA_DIR=/tmp/matrixcloud \
    MATRIX_SHELL_ENABLED=false \
    HF_HOME=/tmp/hf
EXPOSE 7860
ENTRYPOINT ["python3", "/app/app/main.py"]
DOCKER

cd "${DEPLOY_DIR}"
git init -q -b main
git config user.name  "matrixcloud-deploy"
git config user.email "deploy@matrixhub.io"
git config commit.gpgsign false
git config tag.gpgsign false
git add -A
git -c commit.gpgsign=false commit -q -m "Deploy MatrixCloud ($(cd "$ROOT" && git rev-parse --short HEAD))"

echo "🔁 Pushing → HF Space main (force)..."
git push --force -q "https://${HF_USERNAME}:${HF_TOKEN}@huggingface.co/spaces/${HF_USERNAME}/${SPACE_NAME}" main

echo
echo "✅ Deployed. The Space is building now:"
echo "   https://huggingface.co/spaces/${HF_USERNAME}/${SPACE_NAME}"
echo "   app URL (after build): https://${HF_USERNAME}-${SPACE_NAME}.hf.space"
echo
echo "Recommended Space secrets (Settings → Variables and secrets) for durable accounts:"
echo "   MATRIXCLOUD_DATABASE_URL=postgresql://…neon…?sslmode=require&channel_binding=require"
echo "   MATRIXCLOUD_SECRET_KEY=\$(openssl rand -hex 32)"
echo "   MATRIXCLOUD_APP_URL=https://${HF_USERNAME}-${SPACE_NAME}.hf.space"
