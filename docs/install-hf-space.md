# Install: Hugging Face Space

Deploy matrix-runtime as a Hugging Face **Docker Space** to provide 10-minute
MCP sandbox testing.

## Configure the Space

- Space SDK: **Docker**
- The repository `Dockerfile` builds and runs `matrix-runtime`.
- Set the listening port to `8080` (the Dockerfile exposes it).

Set these Space variables/secrets:

```
MATRIX_RUNTIME_MODE=hf-space
MATRIX_RUNTIME_MAX_CONCURRENT_JOBS=1
MATRIX_RUNTIME_MAX_TTL_SECONDS=600
# Optional, for gated/private model inspection:
HF_TOKEN=hf_xxxxx
```

`hf-space` mode defaults to a single concurrent job, which matches a Space's
constrained resources and the short-lived-sandbox use case.

## Use

```bash
curl -X POST https://<your-space>.hf.space/v1/sandbox/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "entity_id": "mcp_server:filesystem",
    "ttl_seconds": 600,
    "runtime": "node",
    "transport": "stdio",
    "start_command": "npx -y @modelcontextprotocol/server-filesystem /tmp"
  }'
```

Stream events, list tools and call tools via the `/v1/sandbox/sessions/...`
endpoints. The sandbox is torn down automatically after the TTL.
