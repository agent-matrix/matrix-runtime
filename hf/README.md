---
title: MatrixCloud
emoji: 🟢
colorFrom: green
colorTo: indigo
sdk: docker
app_port: 7860
pinned: false
license: apache-2.0
short_description: "Self-hostable execution plane for AI: MCP, models, agents."
---

# MatrixCloud

The self-hostable **execution plane** for MatrixCloud: run MCP server sandboxes,
inspect Hugging Face models, use the MatrixShell operator terminal, and bring
your own HF token to run HF LLMs — all from one console.

This Space runs the single static `matrix-runtime` binary (API + embedded React
console). The container launcher is `app/main.py`; the binary is built from the
[`agent-matrix/matrix-runtime`](https://github.com/agent-matrix/matrix-runtime)
repository at build time.

## Configure (Space → Settings → Variables and secrets)

**Secrets**
- `MATRIXCLOUD_DATABASE_URL` — PostgreSQL/Neon DSN (durable accounts across
  rebuilds). Without it, data lives in ephemeral `/tmp` and resets on restart.
- `MATRIXCLOUD_SECRET_KEY` — `openssl rand -hex 32` (stable encryption key).
- `RESEND_API_KEY` — optional; enables welcome/password-reset emails.

**Variables**
- `MATRIXCLOUD_DB_SCHEMA=matrixcloud`
- `MATRIXCLOUD_APP_URL=https://<owner>-matrixcloud.hf.space`

Open the Space URL, create an account, and you're in.
