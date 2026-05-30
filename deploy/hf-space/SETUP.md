# Deploying the MatrixCloud demo on Hugging Face

Two files in `deploy/hf-space/` (`Dockerfile`, `README.md`) are everything a
Hugging Face **Docker Space** needs.

## 1. Create the Space
- **Owner / name:** `ruslanmv / matrixcloud`
- **Short description:** "Self-hostable execution plane for AI — MCP sandboxes, models, MatrixShell."
- **License:** Apache-2.0
- **SDK:** **Docker** → template **Blank**
- **Hardware:** **Free** (CPU basic). No GPU needed — model *inspection* is
  metadata only, and LLM inference is bring-your-own (the user's HF token).
- **Storage bucket:** **not required** for the demo. Use Postgres (Neon) for
  durable accounts instead (see secrets below). Mounting a bucket at `/data` is
  only useful if you want the model cache to persist — if you do, also set
  `MATRIX_RUNTIME_DATA_DIR=/data`.
- **Visibility:** **Public** (so people can try it).
- **Dev mode / Protected:** not needed (PRO-only).

## 2. Add the files
Push `Dockerfile` and `README.md` from this folder to the new Space's git repo
(rename `deploy/hf-space/README.md` → `README.md` at the Space root; its YAML
front-matter sets `sdk: docker` and `app_port: 8080`).

## 3. Set Space secrets/variables
Secrets:
```
MATRIXCLOUD_DATABASE_URL = postgresql://…neon…?sslmode=require&channel_binding=require
MATRIXCLOUD_SECRET_KEY   = <openssl rand -hex 32>
RESEND_API_KEY           = re_…           # optional (emails)
```
Variables:
```
MATRIXCLOUD_DB_SCHEMA  = matrixcloud
MATRIXCLOUD_APP_URL    = https://ruslanmv-matrixcloud.hf.space
MATRIXCLOUD_EMAIL_FROM = MatrixCloud <noreply@matrixhub.io>   # if using Resend
```
Leave `MATRIX_RUNTIME_API_TOKEN` **unset** — the multitenant console
authenticates per-user with sessions; users just sign up. (Setting it would
gate operator-level API calls but isn't needed for the demo.)

Keep `MATRIX_SHELL_ENABLED=false` on a public Space (it runs sandboxed commands;
enable only if you want to demo it).

## 4. Test
Open `https://ruslanmv-matrixcloud.hf.space`, sign up, and try:
- **Models** → import + inspect `Qwen/Qwen2.5-7B-Instruct`
- **Settings** → plug in your Hugging Face token (BYO provider)
- **Catalog / Sandboxes** → launch a filesystem MCP sandbox
- `…/docs` and `…/v1/version` to confirm the API

> If you don't set `MATRIXCLOUD_DATABASE_URL`, the demo still works but accounts
> live in ephemeral `/tmp` and reset whenever the Space rebuilds/sleeps.
