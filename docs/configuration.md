# Configuration

Configuration is layered: **mode-aware defaults → environment variables →
command-line flags**.

## Modes

`MATRIX_RUNTIME_MODE` or `--mode`: `cloud-worker`, `customer-agent`, `hf-space`,
`local-dev`.

## Environment variables

| Variable                             | Default                                   | Meaning                                  |
|--------------------------------------|-------------------------------------------|------------------------------------------|
| `MATRIX_RUNTIME_MODE`                | `local-dev`                               | Runtime mode                             |
| `MATRIX_RUNTIME_PORT` (or `PORT`)    | `8080`                                    | HTTP port. Falls back to `$PORT` (Cloud Run/Render/Railway/Koyeb/Heroku); auto-picks the next free port if the chosen one is busy. |
| `MATRIX_RUNTIME_DATA_DIR`            | `/var/lib/matrix-runtime` (local: `~/.matrix/runtime/data`) | Data root              |
| `MATRIX_RUNTIME_MAX_TTL_SECONDS`     | `600`                                     | Max job/sandbox TTL                      |
| `MATRIX_RUNTIME_MAX_CONCURRENT_JOBS` | `1` (hf-space), `5` (customer/cloud), `2` (local) | Concurrency limit                |
| `MATRIX_RUNTIME_ID`                  | derived from mode                         | Stable runtime id                        |
| `MATRIX_RUNTIME_WORKSPACE`           | —                                         | Workspace name                           |
| `MATRIX_RUNTIME_API_TOKEN`           | —                                         | If set, API requires this bearer token   |
| `MATRIX_CLOUD_URL`                   | `https://cloud.matrixhub.io`              | Control-plane URL                        |
| `MATRIX_RUNTIME_JOIN_TOKEN`          | —                                         | Hybrid-cloud join token                  |
| `HF_TOKEN`                           | —                                         | Hugging Face token (gated/private)       |
| `MATRIX_RUNTIME_HF_CACHE_DIR`        | `<data>/models/huggingface`               | HF model cache directory                 |
| `MATRIX_RUNTIME_DB_PATH`             | `<data>/matrixcloud.db`                   | SQLite multitenant user database         |

## Flags

```
matrix-runtime --mode customer-agent --port 8080
matrix-runtime join --cloud-url https://cloud.matrixhub.io --token mxrt_xxxxx [--runtime-id ID] [--workspace NAME]
```

## Internal limits (defaults)

| Limit                     | Default     |
|---------------------------|-------------|
| `install_timeout_seconds` | `120`       |
| `startup_timeout_seconds` | `45`        |
| `rpc_timeout_seconds`     | `20`        |
| `max_log_bytes`           | `1048576`   |

## Cache layout

```
<data>/
├── models/huggingface/<ns>--<name>/{metadata.json,lock.json,snapshots/}
├── mcp/<job_id>/        # per-sandbox scratch (deleted on completion)
├── agents/
├── jobs/<job_id>/
└── logs/
```
