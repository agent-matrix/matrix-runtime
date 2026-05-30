# Refactor from matrixhub-ai

`legacy/matrixhub-ai/` is **imported reference code only**. It is a separate Go
module (its own `go.mod`), so it is excluded from this module's build and tests.
**Do not extend it directly.** Useful ideas were refactored into clean modules
under `internal/`.

## What was imported

The full matrixhub-ai backend (Go API server, storage, registry/sync system,
model metadata, auth/governance), its React UI and its Docusaurus site, placed
verbatim under `legacy/matrixhub-ai/` for reference.

## What was extracted (as ideas, re-implemented cleanly)

| Concept (legacy)                                              | Where it lives now                          |
|--------------------------------------------------------------|---------------------------------------------|
| Hugging Face model resolver / repo-id parsing                | `internal/hf/resolver.go`                   |
| Hugging Face metadata fetch & parsing (`registrydiscovery/hf`) | `internal/hf/metadata.go`                 |
| Model metadata model (pipeline, library, license, type)      | `internal/hf/types.go`, `internal/models/`  |
| Hugging Face downloader (resolve/snapshot paths)             | `internal/hf/downloader.go`                 |
| Simple local model cache + lock                              | `internal/cache/`                           |
| Sync/job executor pattern (`internal/jobserver`)             | `internal/jobs/manager.go`                  |
| Job logs / status / cancel behaviour                         | `internal/jobs/` + `internal/logs/`         |
| Token/project permission ideas                               | `internal/security/secrets.go`, API auth    |

The implementations are **new and stdlib-only**; they borrow structure and
naming conventions, not code, from the legacy tree.

## What was intentionally NOT extracted (MVP)

- Full Hugging Face-compatible web UI
- Full model registry product
- Git server / SSH Git server / Git LFS server
- Dataset registry
- Multi-region sync
- Enterprise RBAC UI
- Docusaurus docs site
- Full private model registry
- S3/NFS model storage
- Air-gapped registry
- Malware scanning

## Where future registry features may go

These enterprise/registry capabilities belong to a future `matrix-registry`
(or enterprise Matrix Cloud) rather than the execution-plane runtime. Matrix
Runtime stays focused on **executing** MCP servers, agents, tools, sandboxes and
optional model jobs.
