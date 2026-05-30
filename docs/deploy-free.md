# Deploy MatrixCloud for free (to test)

MatrixCloud is a **single static binary** (backend API + embedded console +
SQLite). That makes it cheap to run anywhere. This guide lists free options,
from "best/full features" to "shareable hosted demo".

## What works on a free tier?

Free tiers are **CPU-only** with an **ephemeral disk**. Here's what that means:

| Feature | Free tier | Notes |
|---|---|---|
| Console, login, multitenant accounts | ✅ | SQLite resets when the container restarts (ephemeral disk) |
| MCP **sandboxes** (`mcp.test`) | ✅ | needs `node`/`npx`; the Docker image bundles it |
| **MatrixShell** sandbox (install + exec) | ✅ | re-installs the venv after a cold start (ephemeral disk) |
| **Models**: search, import profile, `model.inspect`, attach (SSE progress) | ✅ | metadata + progress are real |
| Serving real model **weights** (GPU) | ❌ | no GPU on free tiers — use a GPU VM / your infra |
| Persistence across restarts | ⚠️ | only on a real VM (Oracle Always Free) or a mounted volume |

> The runtime honors `$PORT` (Cloud Run / Render / Railway / Koyeb / Heroku) and
> `MATRIX_RUNTIME_PORT`, and auto-falls back to the next free port locally.

---

## 1. Local / WSL — recommended for full features

Free, instant, and the only place the **MatrixShell sandbox** runs against your
own machine.

```bash
git clone https://github.com/agent-matrix/matrix-runtime
cd matrix-runtime
make run            # → prints http://localhost:8080
```

Open the URL, sign up, and open **MatrixShell → Install**.

---

## 2. GitHub Codespaces — one-click cloud dev box (free monthly hours)

1. On the repo: **Code ▸ Codespaces ▸ Create codespace**.
2. In the terminal: `make run`.
3. Codespaces auto-forwards **port 8080** — click the popup to open the console.

Great for a quick shared test without installing anything locally.

---

## 3. Hugging Face Spaces (Docker) — shareable hosted demo, free CPU

The runtime has a first-class `hf-space` mode.

1. Create a new **Space** → SDK **Docker** → **Blank**.
2. Add this `Dockerfile` (it just builds from the repo):

   ```dockerfile
   FROM ghcr.io/agent-matrix/matrix-runtime:latest
   ```

   *(or copy this repo's `Dockerfile` and build from source.)*
3. In **Settings ▸ Variables and secrets**, set:

   ```
   MATRIX_RUNTIME_MODE = hf-space
   MATRIX_RUNTIME_PORT = 7860      # HF Spaces serve on 7860
   ```
4. (Optional, for gated models) add a secret `HF_TOKEN`.

The console is live at your Space URL. Spaces sleep after inactivity and have an
ephemeral disk, so accounts/cache reset on wake — fine for a demo.

---

## 4. Google Cloud Run — generous free tier, scales to zero

```bash
# from a clone of this repo, with gcloud configured:
gcloud run deploy matrixcloud \
  --source . \
  --allow-unauthenticated \
  --region us-central1 \
  --set-env-vars MATRIX_RUNTIME_MODE=customer-agent
```

Cloud Run injects `$PORT` (honored automatically). The container filesystem is
ephemeral; mount a volume or accept that SQLite resets between revisions.

---

## 5. Render / Koyeb / Fly.io — container deploy from this repo

- **Render**: New ▸ **Web Service** ▸ connect the repo ▸ runtime **Docker**.
  Render sets `$PORT` automatically. Free instances spin down when idle.
- **Koyeb**: New App ▸ **Dockerfile** ▸ deploy. Free tier, `$PORT` honored.
- **Fly.io**: `fly launch` (detects the `Dockerfile`); set the internal port to
  `8080` in `fly.toml`. Small machines are inexpensive / free-ish.

For all three, optionally set `MATRIX_RUNTIME_API_TOKEN` to protect the `/v1`
API publicly (the console login still gates the UI separately).

---

## 6. Oracle Cloud Always Free — a real, always-on box

For persistence (accounts + cache survive restarts) and no idle-sleep, the
**Oracle Cloud Always Free** ARM VM is the best free option:

```bash
# on the VM (Ubuntu/Debian, Go + node + python installed):
git clone https://github.com/agent-matrix/matrix-runtime && cd matrix-runtime
sudo make install INSTALL_SYSTEMD=1
sudo systemctl enable --now matrix-runtime
# edit /etc/matrix-runtime/matrix-runtime.env to set MATRIX_RUNTIME_API_TOKEN, mode, etc.
```

Put it behind Caddy/nginx + a free Let's Encrypt cert for HTTPS. See
[install-onprem.md](install-onprem.md).

---

## Securing a public test instance

- Set `MATRIX_RUNTIME_API_TOKEN` to require a bearer token on `/v1` (static
  console assets and `/v1/auth/*` stay public so users can still sign in).
- Keep `MATRIX_RUNTIME_MAX_TTL_SECONDS` / `MAX_CONCURRENT_JOBS` conservative.
- Restrict egress to Hugging Face + your control plane where possible.
- See [security.md](security.md).
