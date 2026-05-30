# Screenshots

Real console screenshots live in `docs/assets/screenshots/` and are used in the
[README](../README.md) showcase. They are captured at 1440×900 @2× (crisp
2880×1800 PNGs), dark theme.

| File | View |
|------|------|
| `login.png`    | Sign in / sign up (premium auth screen, forgot-password) |
| `overview.png` | Live runtime health, capabilities, jobs + production-readiness banner |
| `runtimes.png` | Workspace runtimes — a joined Hugging Face Space + the local control node |
| `models.png`   | Model gateway — import / resolve / attach profiles |
| `settings.png` | Workspace, detected runtimes, storage usage, BYO provider keys |

## Regenerate them

Capture is done with a **transient** Playwright + Chromium install (never
committed). With the runtime running locally:

```bash
# 1) Run the console (separate terminal) and create a demo account/token
make run &                      # serves on :8080 (or set --port)
TOK=$(curl -fsS -X POST localhost:8080/v1/auth/signup -H 'Content-Type: application/json' \
  -d '{"name":"Maya Chen","email":"maya@acme.io","password":"screenshots1","workspace":"Acme AI"}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'); echo "$TOK" > /tmp/mc-token.txt

# 2) Transient browser (installed outside the repo, e.g. /tmp)
cd /tmp && npm i playwright@1.47 && npx playwright install chromium

# 3) Capture (script injects the session token and clicks the SPA nav)
BASE=http://localhost:8080 node scripts/screenshots.mjs   # or the inline shoot.mjs

# Playwright lives only in node_modules (git-ignored) — nothing enters the repo.
```

`scripts/screenshots.mjs` drives the capture; it signs in by injecting the
session token into `localStorage`, then screenshots Overview, Runtimes, Models,
and Settings. For a demo **GIF**, record the *Add a runtime → Duplicate Space*
flow and the *MatrixShell* terminal — they tell the product story fastest.
