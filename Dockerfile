# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Version metadata (passed by CI/Make) is stamped into the binary for /v1/version.
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w \
      -X github.com/agent-matrix/matrix-runtime/internal/config.Version=${VERSION} \
      -X github.com/agent-matrix/matrix-runtime/internal/config.Commit=${COMMIT} \
      -X github.com/agent-matrix/matrix-runtime/internal/config.Date=${BUILD_DATE}" \
    -o /out/matrix-runtime ./cmd/matrix-runtime

# ---- runtime stage ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git nodejs npm python3 python3-pip pipx \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -u 10001 matrix
WORKDIR /app
COPY --from=builder /out/matrix-runtime /usr/local/bin/matrix-runtime
RUN mkdir -p /var/lib/matrix-runtime && chown -R matrix:matrix /var/lib/matrix-runtime

USER matrix
ENV MATRIX_RUNTIME_DATA_DIR=/var/lib/matrix-runtime \
    MATRIX_RUNTIME_PORT=8080
VOLUME ["/var/lib/matrix-runtime"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS "http://localhost:${MATRIX_RUNTIME_PORT:-8080}/v1/health" || exit 1
ENTRYPOINT ["matrix-runtime"]
