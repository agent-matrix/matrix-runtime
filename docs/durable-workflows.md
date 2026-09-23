# Durable Agent-Matrix workflows

Matrix Runtime persists the execution state for long-running Agent-Matrix runs.
Matrix OS remains the **global cognitive authority**; Runtime is the durable
execution substrate.

## API

- `POST /v1/workflows`
- `GET /v1/workflows/{run_id}`
- `GET /v1/workflows/{run_id}/events`
- `POST /v1/workflows/{run_id}/transition`
- `POST /v1/workflows/{run_id}/resume`
- `POST /v1/workflows/{run_id}/cancel`
- `POST /v1/workflows/{run_id}/rollback`

Events are append-only, sequenced, replayable, and persisted in SQLite/Postgres.
The workflow state machine supports human checkpoints, pause/resume, verification,
cancellation and rollback.

This boundary is designed for WorkGraphs compiled by Matrix Architect and may be
driven by Matrix OS or the Hive adapter.
