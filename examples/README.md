# Examples

Ready-to-use payloads for the Matrix Runtime API. Set `API=http://localhost:8080`
(and a bearer token if your runtime requires one).

## Jobs

Run an MCP server in a sandbox and probe its tools:

```bash
curl -fsS -X POST "$API/v1/jobs" \
  -H "Content-Type: application/json" \
  --data @examples/jobs/mcp-test-filesystem.json
```

Inspect a Hugging Face model (metadata only — no download):

```bash
curl -fsS -X POST "$API/v1/jobs" \
  -H "Content-Type: application/json" \
  --data @examples/jobs/model-inspect-qwen.json
```

Stream a job's events:

```bash
curl -N "$API/v1/jobs/<job_id>/events"
```

## Catalog entries

`examples/catalog/*.json` are reference manifests for MCP servers and a model
profile. The model profile maps to the importer used by the console
(`POST /v1/model-profiles`); the MCP entries describe the `start_command` used
by `mcp.test` jobs.
