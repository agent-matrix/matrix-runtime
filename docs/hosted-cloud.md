# MatrixCloud — Hosted Control Plane (`cloud.matrixhub.io`)

This document describes how MatrixCloud is operated as a hosted, multi‑tenant
product: users create a free account at **`cloud.matrixhub.io`**, get their own
isolated workspace, and run their **own sandboxes** — including a **duplicated
Hugging Face Space** — that are **managed centrally** from a single control
plane. Users plug in **their own Hugging Face account/token** to run HF LLMs
inside their runtimes.

It also covers the data model, the Neon (PostgreSQL) + Resend wiring, the free
plan/metering, the security model, and a go‑to‑market plan for the Hugging Face
community.

---

## 1. The big picture

```
                         cloud.matrixhub.io  (Vercel — static console, optional)
                                   │
                                   ▼
   Browser ──────────────► api.matrixhub.io  (Oracle box, Cloudflare‑proxied)
   (signup/login)                 │            = matrix-runtime control plane
                                  │              + embedded console (go:embed)
                                  │              + Neon Postgres (schema: matrixcloud)
                                  │              + Resend (transactional email)
                                  │
        ┌─────────────────────────┼──────────────────────────────┐
        ▼                         ▼                                ▼
  User A's HF Space        User B's HF Space                User C self‑hosted
  (duplicated runtime)     (duplicated runtime)             (docker / k8s runtime)
        │                         │                                │
        └─── register + heartbeat (join token → runtime token) ────┘
             outbound HTTPS only; the control plane never dials in
```

Two deployment shapes for the console:

- **All‑in‑one (recommended to start):** the Go binary on the Oracle box serves
  both the API *and* the embedded React console at `api.matrixhub.io`. Point
  `cloud.matrixhub.io` (CNAME/redirect) at it, or serve the console from Vercel
  and have it call `https://api.matrixhub.io`.
- **Split:** Vercel serves the static console at `cloud.matrixhub.io`; it calls
  the API at `api.matrixhub.io`. Set `MATRIXCLOUD_APP_URL=https://cloud.matrixhub.io`
  so email links point at the console.

---

## 2. DNS wiring

Given the existing records for `matrixhub.io`:

| Host                    | Type  | Target                  | Role                                   |
|-------------------------|-------|-------------------------|----------------------------------------|
| `matrixhub.io`, `www`   | A/CNAME | Vercel                | Marketing site                         |
| `admin.matrixhub.io`    | CNAME | Vercel                  | Admin app (shares the Neon instance)   |
| `api.matrixhub.io`      | A     | `129.213.165.60` (Oracle, Cloudflare‑proxied) | **matrix-runtime control plane** |
| **`cloud.matrixhub.io`**| CNAME | Vercel **or** `api.matrixhub.io` | **MatrixCloud console** |

Email auth (already present): `send` MX → Amazon SES; `resend._domainkey` TXT →
Resend DKIM. Keep the Resend DKIM record so transactional mail from
`@matrixhub.io` is signed and lands in the inbox.

> The Neon instance is **shared** with `admin.matrixhub.io`. MatrixCloud must
> not collide with those tables — see §4.

---

## 3. The sharing model: "Duplicate → join the control plane"

The viral mechanic is a **one‑click Hugging Face Space** that any user can
duplicate into their own HF account. Their copy becomes *their* sandbox, but the
**management** (catalog, runtimes list, model attach, usage) stays in the
owner's central control plane.

Flow:

1. **User signs up** at `cloud.matrixhub.io` → gets a workspace.
2. In the console they click **"Add a runtime → Hugging Face Space"**. The
   control plane **mints a single‑use join token** (`mxrt_…`, see
   `POST /v1/cloud/join-tokens`).
3. The user **Duplicates the MatrixCloud Space** on Hugging Face and pastes the
   join token (and, optionally, their HF token) into the Space's secrets:
   - `MATRIX_CLOUD_URL=https://api.matrixhub.io`
   - `MATRIX_RUNTIME_JOIN_TOKEN=mxrt_…`
   - (optional) `HF_TOKEN=hf_…`
4. On boot, the Space's `matrix-runtime` **registers outbound**
   (`POST /v1/cloud/runtimes/register`) using the join token and receives a
   long‑lived **runtime token**. It then **heartbeats**
   (`POST /v1/cloud/runtimes/heartbeat`) every ~30s.
5. The control plane shows the Space as an **online runtime** in the user's
   workspace and can dispatch jobs / show status — **without ever dialing into
   the Space** (HF Spaces aren't reachable inbound for control; everything is
   outbound HTTPS, Cloudflare‑friendly).

This is the same join model as `matrix-runtime join` for self‑hosted/k8s
runtimes — HF Spaces are just `kind="hf-space"`.

### Bring‑your‑own Hugging Face LLMs

Each workspace stores its **own** provider credentials, AES‑256‑GCM encrypted at
rest (`provider_credentials`, see `POST /v1/cloud/providers`). A free user pastes
their HF token once; MatrixCloud uses it **server‑side** to call HF Inference
(or routes it to the user's runtime) so the token never ships to the browser and
only a `••••1234` hint is ever returned. Supported providers are open‑ended:
`huggingface`, `openai`‑compatible, `ollama`, `vllm`, `anthropic`, ….

---

## 4. Data model & isolation (Neon / PostgreSQL)

The same store powers SQLite (local/dev, single binary) and PostgreSQL/Neon
(hosted). The driver is selected at runtime:

- `store.Open(path)` → SQLite (`modernc.org/sqlite`, pure‑Go, static binary).
- `store.OpenPostgres(dsn, schema, secretDir)` → PostgreSQL (`pgx/v5/stdlib`).

**Schema isolation.** Because Neon is shared with `admin.matrixhub.io`,
MatrixCloud creates and resolves **all** of its objects inside a dedicated
Postgres schema (default **`matrixcloud`**):

- `OpenPostgres` appends `search_path=matrixcloud` to the DSN and runs
  `CREATE SCHEMA IF NOT EXISTS "matrixcloud"` before migrating.
- All DDL uses `CREATE TABLE IF NOT EXISTS` and is **purely additive** — it
  never drops or alters tables it didn't create, so existing admin tables are
  untouched even if a name (e.g. `users`) coincides: a `matrixcloud.users` is a
  different relation from `public.users`.
- `?` placeholders are rebound to `$N` for Postgres automatically; DDL types are
  chosen to be valid on both engines (`TEXT`, `INTEGER`).

Tables (all in the `matrixcloud` schema on Neon):

| Table                       | Purpose                                                        |
|-----------------------------|----------------------------------------------------------------|
| `workspaces`                | Tenants. One per signup (the owner's workspace).               |
| `users`                     | Members of a workspace; PBKDF2‑hashed passwords.               |
| `sessions`                  | Bearer session tokens (30‑day TTL).                            |
| `model_profiles`            | Known models (HF, URL, …) — "profile only" until attached.     |
| `model_runtime_installations` | Physical attach/install of a profile to a runtime.           |
| `runtimes`                  | Registered execution planes incl. HF Spaces (`kind`).          |
| `runtime_join_tokens`       | Single/limited‑use tokens to onboard a runtime.                |
| `provider_credentials`      | BYO provider tokens, AES‑256‑GCM encrypted.                    |
| `usage_events`              | Append‑only metering for free‑plan limits & analytics.         |
| `password_resets`           | Single‑use, time‑bounded reset tokens (hashed).                |
| `email_verifications`       | Single‑use email‑verification tokens (hashed).                 |

### Configuration

Secrets are **only** read from the environment — never committed:

```bash
# Hosted (Neon). The first non-empty of these wins.
export MATRIXCLOUD_DATABASE_URL='postgresql://USER:PASS@HOST/neondb?sslmode=require&channel_binding=require'
#   aliases also accepted: MATRIX_RUNTIME_DB_URL, DATABASE_URL
export MATRIXCLOUD_DB_SCHEMA=matrixcloud        # default; isolates from admin.*

# At-rest encryption key for provider_credentials (32 bytes, hex or base64).
export MATRIXCLOUD_SECRET_KEY=$(openssl rand -hex 32)

# Transactional email (Resend).
export RESEND_API_KEY=re_xxxxxxxxx
export MATRIXCLOUD_EMAIL_FROM='MatrixCloud <noreply@matrixhub.io>'
export MATRIXCLOUD_APP_URL=https://cloud.matrixhub.io
```

When `MATRIXCLOUD_DATABASE_URL` is unset, the binary falls back to a local
SQLite file — so local dev and HF‑Space sandboxes need zero Postgres.

> **Neon networking note.** Neon's pooled endpoint listens on TCP `5432` with
> TLS (`sslmode=require`). The control‑plane host (Oracle box) must allow
> outbound `5432`. Some restricted CI/sandbox networks block `5432`; in those
> the binary simply runs on SQLite.

---

## 5. Email (Resend)

`internal/email` posts to `https://api.resend.com/emails` with
`Authorization: Bearer $RESEND_API_KEY`. Two transactional templates:

- **Welcome / verify** on signup (`SendWelcome`) — best‑effort, never blocks
  account creation.
- **Password reset** (`SendPasswordReset`) — single‑use, 1‑hour token.

Endpoints:

| Method & path             | Auth            | Purpose                                  |
|---------------------------|-----------------|------------------------------------------|
| `POST /v1/auth/signup`    | none            | Create account (+ welcome email)         |
| `POST /v1/auth/login`     | none            | Start session                            |
| `POST /v1/auth/forgot`    | none            | Email a reset link (always 200)          |
| `POST /v1/auth/reset`     | reset token     | Set a new password, revoke sessions      |
| `POST /v1/auth/verify`    | verify token    | Confirm email address                    |

With **no** `RESEND_API_KEY`, the sender runs in **log‑only** mode (logs the
message, returns success) so dev/test never need an account. `/v1/auth/forgot`
always returns 200 with a generic message so it can't be used to enumerate which
emails are registered.

---

## 6. Hosted control‑plane API

All workspace‑scoped endpoints authenticate with the **session bearer token**.
Runtime onboarding endpoints use their **own** token (join token → runtime
token) and are exempt from the operator API‑token gate.

| Method & path                          | Auth          | Purpose                               |
|----------------------------------------|---------------|---------------------------------------|
| `GET  /v1/cloud/runtimes`              | session       | List the workspace's runtimes         |
| `POST /v1/cloud/runtimes/register`     | join token    | A runtime/Space registers (outbound)  |
| `POST /v1/cloud/runtimes/heartbeat`    | runtime token | Liveness + capability updates         |
| `GET  /v1/cloud/join-tokens`           | session       | List active join tokens               |
| `POST /v1/cloud/join-tokens`           | session       | Mint a join token (secret shown once) |
| `GET  /v1/cloud/providers`             | session       | List BYO creds (hints only)           |
| `POST /v1/cloud/providers`             | session       | Store a BYO provider token (encrypted)|
| `GET  /v1/cloud/usage`                 | session       | 30‑day usage by kind                  |

Runtimes not seen within ~90s are reported `offline`.

---

## 7. Free plan & metering

`usage_events` is append‑only; `UsageSince` sums per‑kind quantities for a
window. Suggested free‑tier limits (enforced at the handler/job layer):

| Kind               | Free limit (per month) | Notes                              |
|--------------------|------------------------|------------------------------------|
| `sandbox.start`    | generous               | Short‑lived MCP sandboxes          |
| `model.inspect`    | generous               | Metadata only                      |
| `model.attach`     | a handful              | Heavier; pulls weights             |
| `llm.tokens`       | **BYO** (user's HF)    | Costs are on the user's HF account |
| `matrixshell.exec` | generous               | Sandboxed shell commands           |

Because LLM inference runs on the **user's own** HF token, the expensive part is
paid by the user — MatrixCloud's hosting cost stays low, which makes a genuinely
free tier sustainable.

---

## 8. Security model

- **Passwords:** PBKDF2‑HMAC‑SHA256 (120k iterations), stdlib only.
- **Sessions:** 32‑byte random bearer tokens; 30‑day TTL; revoked on password
  reset.
- **Join/runtime/reset/verify tokens:** high‑entropy randoms, stored **hashed**
  (SHA‑256); join tokens are single/limited‑use and time‑bounded.
- **BYO secrets:** AES‑256‑GCM at rest (`MATRIXCLOUD_SECRET_KEY`); only a
  `••••1234` hint is ever returned to the browser; plaintext is used
  server‑side only.
- **Tenant isolation:** every query is workspace‑scoped; on Neon everything is
  additionally namespaced into the `matrixcloud` schema.
- **Network:** runtimes connect **outbound** only — the control plane never
  needs inbound access to a user's HF Space or private cluster. Cloudflare proxy
  in front of `api.matrixhub.io`.
- **No secrets in the repo:** all credentials come from env vars.

---

## 9. Go‑to‑market: getting the Hugging Face community's attention

1. **A flagship Space, "Duplicate me".** Publish `matrixcloud` as a polished HF
   Space with a prominent **Duplicate this Space** button and a 60‑second
   "your Space is now a managed sandbox" demo GIF. Duplication is HF's native
   viral loop.
2. **BYO‑token, zero‑cost narrative.** Lead with "use *your* Hugging Face models
   and token — we never see your weights, you keep your quota." This resonates
   with the community's self‑hosting ethos.
3. **One‑click model attach.** From any HF model page, "Open in MatrixCloud"
   resolves → imports a profile → attaches to a runtime with live SSE logs.
   Make the catalog seed with popular open models (Qwen, Llama, Mistral, …).
4. **Templates & Spaces collection.** Ship an HF *Collection* of ready Spaces
   (chat, RAG, agent, tool‑server) that all register into one control plane.
5. **Open source + Apache‑2.0.** The runtime is OSS; invite PRs for new runtime
   kinds and providers. Credibility on HF comes from being inspectable.
6. **Community content.** A blog/Space write‑up: "Turn any HF Space into a
   centrally‑managed sandbox in 1 click," cross‑posted to the HF forums and
   `huggingface` Discord, plus a short YouTube walkthrough.
7. **Leaderboard / showcase.** A public gallery of community runtimes and the
   models they serve (opt‑in) to create social proof and FOMO.
8. **Generous, honest free tier.** Because inference is BYO, the free tier can be
   real — the strongest acquisition tool of all.

---

## 10. Operating checklist

```bash
# On the Oracle box (api.matrixhub.io):
export MATRIXCLOUD_DATABASE_URL='postgresql://…neon…?sslmode=require&channel_binding=require'
export MATRIXCLOUD_DB_SCHEMA=matrixcloud
export MATRIXCLOUD_SECRET_KEY=$(openssl rand -hex 32)   # persist this!
export RESEND_API_KEY=re_xxxxxxxxx
export MATRIXCLOUD_EMAIL_FROM='MatrixCloud <noreply@matrixhub.io>'
export MATRIXCLOUD_APP_URL=https://cloud.matrixhub.io
export PORT=8080                                        # or honor PaaS $PORT
matrix-runtime --mode cloud-worker
# → creates/uses the `matrixcloud` schema on Neon, serves API + console.
```

On boot you should see `user store ready: postgres (schema "matrixcloud")`.
