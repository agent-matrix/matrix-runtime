# MatrixCloud — Enterprise Console

The runtime ships a premium, enterprise console (the **MatrixCloud** control
surface) served directly from the binary. Start the runtime and open it:

```bash
make build
./bin/matrix-runtime --mode local-dev
# open http://localhost:8080/
```

The console is embedded via `go:embed` — no separate web server, no CDN at
runtime. React and ReactDOM are vendored locally, so it works fully air-gapped.

## Sign in (multitenant accounts)

The console is gated by a login/signup screen backed by a real **SQLite** user
store (`<data-dir>/matrixcloud.db`). The first sign-up creates a workspace
(tenant) and an Owner user; subsequent users sign in with email + password.
Sessions are bearer tokens persisted in the browser and validated by the
runtime. See [auth.md](auth.md).

## MatrixShell — the in-cloud CLI

Open **MatrixShell** from the top bar: an AI-assisted operator terminal. Type a
command, or describe what you want in plain English — the request becomes a
command with a **risk level** that you confirm before it runs, guarded by a hard
safety denylist (mirrors the standalone
[MatrixShell](https://github.com/agent-matrix/MatrixShell) CLI). The `matrix`
subcommands are wired to this runtime:

| Command                 | Wiring                                   |
|-------------------------|------------------------------------------|
| `matrix status`         | live `/v1/health` + `/v1/capabilities`   |
| `matrix ps`             | live `/v1/jobs`                          |
| `matrix capabilities`   | live `/v1/capabilities`                  |
| `matrix inspect <model>`| live `model.inspect` job                 |

## Guided wizards

- **New Sandbox** — pick a sandbox-enabled MCP server and a runtime, then launch
  a real session.
- **Attach Model** — pick a provider/model; Hugging Face models resolve live via
  `model.inspect`.

## Models — generic importer

The **Models** area keeps four tabs that mirror the model lifecycle, and enforces
the rule that *importing ≠ downloading ≠ attaching ≠ ready*:

| Tab | Answers |
|---|---|
| Available Models | What can I use right now? |
| Connected Providers | Where can models come from? |
| Model Profiles | What models does MatrixCloud know about? |
| Runtime Cache | What is physically downloaded inside a runtime? |

- **Add Model** — manual/custom setup (OpenAI-compatible, vLLM, Ollama, custom
  manifest, external endpoint).
- **Import Model** — a generic, multi-source importer (3 steps: **Source →
  Preview → Attach**) for **Hugging Face, GitHub, GitLab, Amazon S3, Cloudflare
  R2, Ollama, Custom URL**, each with its own form and a private-credential toggle.

The whole flow is backed by **real, persistent state** (no simulation):

- Hugging Face search is **live** via `GET /v1/model-sources/huggingface/search`
  (server-side proxy, no CORS), with direct-HF and offline-sample fallbacks.
- **Import** persists a **Model Profile** (`POST /v1/model-profiles`, status
  `profile_only`) in SQLite (`model_profiles`).
- **Attach & Install** creates a **runtime installation** row
  (`model_runtime_installations`) and a real **`model.attach` job**
  (`POST /v1/model-profiles/{id}/attach`). The job runs server-side and streams
  the genuine lifecycle over SSE — `checking_runtime → checking_disk →
  checking_gpu → fetching_metadata` (real `model.inspect`) `→ downloading`
  (server-driven progress, persisted each step) `→ verifying → creating_profile
  → attached → ready` — writing a serving-profile file to the cache.
- The **Model Profiles** and **Runtime Cache** tabs render live from
  `GET /v1/model-profiles` and `GET /v1/model-installations`, polling while a
  download is in flight; the Attach step's **Refresh** reveals newly-online
  runtimes without closing the modal.

> Weights are staged (the runtime pulls real bytes when GPUs/weights are
> present); every step, progress value, profile and installation is real,
> persisted and streamed.

## Everything is real

Every view renders data from this runtime's own `/v1` API — there are no demo
clusters, fabricated throughput, fake install counts or canned audit rows.

| View            | Source                                                                 |
|-----------------|------------------------------------------------------------------------|
| Overview        | `/v1/health`, `/v1/capabilities`, `/v1/runtimes`, `/v1/jobs` (real runtimes table + real jobs-by-status). |
| Runtimes        | `GET /v1/runtimes` — the real runtime(s) on this control surface.       |
| Catalog         | `GET /v1/catalog` — curated index with real start commands.             |
| Sandboxes       | `POST /v1/sandbox/sessions`, SSE lifecycle, `tools/list`, `tools/call`. |
| Models          | `model-sources/*`, `model-profiles`, `model-installations` + `model.attach` SSE progress. |
| MatrixShell     | `GET/POST /v1/matrixshell/{status,install,exec}` — real Python sandbox. |
| Jobs            | `GET /v1/jobs` + SSE event timeline.                                    |
| Logs            | Live SSE tail of the newest job.                                        |
| Policies        | `GET /v1/policies` — the runtime's enforced config + command lists.     |
| Audit           | `GET /v1/jobs` — real job history.                                      |
| Settings        | `GET /v1/auth/me` + detected runtimes.                                  |

When the runtime is unreachable, views show an empty state and the sidebar marks
the control plane offline (no fabricated fallback data).

## Try the live sandbox

1. Open **Catalog**, find **Filesystem MCP Server** (badged `Sandbox · live`).
2. Click **Test Sandbox → Start Sandbox**.
3. Watch the real lifecycle stream in (`validate → sandbox → mcp_start →
   mcp_initialize → tools_list → ready`), then the runtime's actual `tools/list`.
4. Pick a tool (e.g. `list_directory`) and **Run Tool** — the call executes on
   the live MCP server and returns real output. The session auto-expires after
   its TTL.

## Authentication

When `MATRIX_RUNTIME_API_TOKEN` is set, the `/v1` API requires a bearer token
(the static console assets stay public). Supply the token to the console by
setting `window.MATRIX_API_TOKEN` (e.g. via a small inline script or a reverse
proxy that injects it).

## Building the bundle

The console source lives in `web/src/app.jsx`. The committed
`web/static/app.js` is the pre-built bundle, so `go build` works without a
JS toolchain. To rebuild after editing the source:

```bash
make web        # runs scripts/build-web.sh (esbuild: JSX -> web/static/app.js)
make build      # re-embed and compile the binary
```

## Layout

```
web/
├── embed.go                 # go:embed of static assets
├── src/app.jsx              # console source (React, single module)
└── static/
    ├── index.html           # app shell (loads vendored React + app.js)
    ├── app.js               # built bundle (committed)
    ├── cloud.css            # design system (phosphor-green enterprise theme)
    └── vendor/              # React + ReactDOM UMD (no runtime CDN)
```
