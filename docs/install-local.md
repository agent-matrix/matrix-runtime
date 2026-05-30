# Install: local

Run matrix-runtime directly on a workstation.

## Prerequisites

- Go 1.24+
- Node.js + npm (for `npx`-based MCP servers)
- Python 3 + pipx/uvx (for Python MCP servers)

## Build and run

```bash
make build
./bin/matrix-runtime --mode local-dev
```

Or via the install script:

```bash
./scripts/install.sh --mode local
```

The data directory defaults to `~/.matrix/runtime/data` in `local-dev`.

## Verify

```bash
curl http://localhost:8080/v1/health
curl http://localhost:8080/v1/capabilities
./scripts/smoke-test.sh
```
