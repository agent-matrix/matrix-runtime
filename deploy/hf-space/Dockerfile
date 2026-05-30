# MatrixCloud demo — Hugging Face Docker Space
#
# This builds matrix-runtime from source and serves the API + embedded console
# on port 8080 (matches `app_port` in README.md). Drop this file (and README.md)
# into a new HF Space repo, or duplicate the published Space.

# ---- build ----
FROM golang:1.24-bookworm AS build
ARG REF=main
RUN git clone --depth 1 --branch "${REF}" https://github.com/agent-matrix/matrix-runtime /src
WORKDIR /src
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/matrix-runtime ./cmd/matrix-runtime

# ---- runtime ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git nodejs npm python3 python3-pip pipx \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/matrix-runtime /usr/local/bin/matrix-runtime

# HF Spaces give you a writable /tmp; durable data should live in Postgres
# (Neon) — set MATRIXCLOUD_DATABASE_URL as a Space secret. The local data dir is
# only model/MCP scratch, so /tmp is fine and avoids storage-permission issues.
ENV MATRIX_RUNTIME_MODE=cloud-worker \
    MATRIX_RUNTIME_PORT=8080 \
    MATRIX_RUNTIME_DATA_DIR=/tmp/matrixcloud \
    MATRIX_SHELL_ENABLED=false
EXPOSE 8080
ENTRYPOINT ["matrix-runtime"]
