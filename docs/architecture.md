# Architecture

Matrix Runtime is the **execution plane** for Matrix Cloud. The control plane
(matrix-hub) decides *what* to run; the runtime decides *how* and actually runs
it.

```
   matrix-hub (control plane)
        │  creates jobs / sandbox sessions
        ▼
   matrix-runtime (execution plane)
        │  runs MCP servers, inspects models, (future) agents & tools
        │  streams logs / events back
        ▼
   matrix-llm (model gateway)  ← used by agent.run for LLM calls
```

## Components

- **API server** (`api/`) — HTTP on port 8080. Health, capabilities, jobs and
  the sandbox compatibility aliases. Server-Sent Events for live job streams.
- **Job manager** (`internal/jobs/`) — typed, TTL-bounded units of work with a
  concurrency limit, an in-memory store and per-type handlers.
- **MCP client** (`internal/mcp/`) — newline-delimited JSON-RPC 2.0 over the
  child process's stdio: `initialize`, `notifications/initialized`,
  `tools/list`, `tools/call`.
- **Security** (`internal/security/`) — start-command allow-list and validation,
  TTL/limit clamping, raw-secret rejection.
- **Hugging Face** (`internal/hf/`) — model id resolution, metadata fetch, and a
  staged downloader.
- **Models** (`internal/models/`) — runtime detection and the model.inspect
  logic; Ollama/vLLM/SGLang preload are stubbed.
- **Cache** (`internal/cache/`) — on-disk layout for models, scratch and logs.
- **Control plane** (`internal/controlplane/`) — outbound hybrid-cloud client
  and the (designed, stubbed) control-channel tunnel.

## Job lifecycle

1. `POST /v1/jobs` validates type + TTL, creates a job (`queued`) and returns a
   `job_id` plus an `events_url`.
2. A goroutine acquires a concurrency slot, sets the job `running` and invokes
   the type handler with a TTL-bounded context.
3. The handler emits step events (`validate`, `sandbox`, `mcp_start`,
   `mcp_initialize`, `tools_list`, `ready`, …) on the job's event bus.
4. On completion the manager records the terminal status — `complete`,
   `error`, `expired` (TTL), or `cancelled` (DELETE) — emits a final event and
   closes the bus.

## Sandboxes

A sandbox session is a thin alias over an `mcp.test` job. The MCP server is kept
alive for the TTL; `tools/list` results are cached and `tools/call` is proxied
to the live process. `DELETE` cancels the underlying job.

## Job types

| Type            | MVP status                                            |
|-----------------|-------------------------------------------------------|
| `mcp.test`      | Implemented (10-minute sandbox)                       |
| `model.inspect` | Implemented (Hugging Face metadata)                   |
| `model.attach`  | Implemented (install profile onto a runtime; SSE progress persisted to `model_runtime_installations`) |
| `model.pull`    | Stages cache + metadata; weights deferred             |
| `mcp.run`       | Defined, stubbed                                      |
| `model.preload` | Defined, stubbed                                      |
| `agent.run`     | Defined, stubbed                                      |
| `tool.run`      | Defined, stubbed                                      |
