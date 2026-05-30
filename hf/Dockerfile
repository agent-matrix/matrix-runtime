# MatrixCloud — Hugging Face Docker Space
#
# Self-contained: builds matrix-runtime from the source shipped in this Space
# repo (the deploy script / workflow push the full repo here). It does NOT clone
# GitHub, so it always builds exactly the committed code — no branch/cache drift.
# Serves the API + embedded console on port 7860 (HF's default app port); the
# launcher app/main.py preps the environment and execs the binary.

# ---- build ----
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/matrix-runtime ./cmd/matrix-runtime

# ---- runtime ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git nodejs npm python3 python3-pip pipx \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/matrix-runtime /usr/local/bin/matrix-runtime
COPY requirements.txt /app/requirements.txt
RUN pip3 install --no-cache-dir --break-system-packages -r /app/requirements.txt
COPY app /app/app

# HF Spaces give you a writable /tmp; durable data should live in Postgres
# (set MATRIXCLOUD_DATABASE_URL). Listening port matches app_port in README.md.
ENV MATRIX_RUNTIME_MODE=cloud-worker \
    MATRIX_RUNTIME_PORT=7860 \
    MATRIX_RUNTIME_DATA_DIR=/tmp/matrixcloud \
    MATRIX_SHELL_ENABLED=false \
    HF_HOME=/tmp/hf
EXPOSE 7860
ENTRYPOINT ["python3", "/app/app/main.py"]
