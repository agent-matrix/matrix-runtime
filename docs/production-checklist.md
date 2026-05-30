# Production checklist

Work through this before exposing a Matrix Runtime beyond local development.
The runtime self-reports most of these at `GET /v1/ready` (and the console shows
a banner) — use it as a live check.

## Security
- [ ] Set `MATRIX_RUNTIME_API_TOKEN` (operator token). Without it, non-session
      callers are rejected in production modes and `/v1/ready` warns.
- [ ] Set `MATRIXCLOUD_SECRET_KEY` (32-byte hex/base64) so encrypted BYO
      credentials survive a fresh data dir. Keep it in a secret store.
- [ ] Keep `MATRIX_SHELL_ENABLED=false` unless you specifically need MatrixShell
      (it runs commands in a local sandbox).
- [ ] Terminate TLS at your ingress / load balancer (Cloudflare, nginx, …).
- [ ] Review the command allow/deny lists under **Policies** in the console.

## Data
- [ ] Use PostgreSQL (e.g. Neon) for multi-user / HA: set
      `MATRIXCLOUD_DATABASE_URL`. SQLite is single-node only.
- [ ] Confirm the schema isolation (`MATRIXCLOUD_DB_SCHEMA`, default
      `matrixcloud`) when sharing a database with other apps.
- [ ] Back up the database (and the data dir if using SQLite).

## Reliability & limits
- [ ] Set sensible `MATRIX_RUNTIME_MAX_CONCURRENT_JOBS` and
      `MATRIX_RUNTIME_MAX_TTL_SECONDS` for your hardware.
- [ ] Confirm retention is on: `MATRIX_RUNTIME_JOB_RETENTION_HOURS` (24),
      `MATRIX_RUNTIME_LOG_RETENTION_HOURS` (72),
      `MATRIX_RUNTIME_CLEANUP_INTERVAL_MINUTES` (15).
- [ ] Set `MATRIX_RUNTIME_RATE_LIMIT_RPM` (default 120) appropriately.
- [ ] Wire health/readiness probes: liveness `GET /v1/health`, readiness
      `GET /v1/ready` (the Helm chart already does this).
- [ ] Provision persistent storage for the data dir (Helm `persistence.enabled=true`).

## Email (optional)
- [ ] Set `RESEND_API_KEY` + `MATRIXCLOUD_EMAIL_FROM` for password-reset and
      verification emails; otherwise the sender runs in log-only mode.
- [ ] Set `MATRIXCLOUD_APP_URL` so email links point at your console.

## Observability
- [ ] Scrape `GET /v1/version` and `GET /v1/ready` from your monitoring.
- [ ] Ship container logs; watch for the startup `WARNING:` lines.
- [ ] Review the **Audit** page (`GET /v1/cloud/audit`) for sensitive actions.

## Verify
```bash
curl -fsS $URL/v1/health
curl -fsS $URL/v1/version
curl -s  $URL/v1/ready | jq           # ready=true, no high-severity warnings
make smoke                            # end-to-end smoke against a running instance
```
