# matrixcloud — Python client &amp; CLI

Official Python SDK and command-line interface for **Matrix Cloud / Matrix
Runtime**. Talk to the `/v1` API: authenticate, run jobs, inspect models, and
drive MCP sandboxes.

## Install (with [uv](https://docs.astral.sh/uv/) — fast)

```bash
cd clients/python
uv sync --extra dev      # creates .venv and installs the package + dev tools
uv run mxc status        # or: source .venv/bin/activate && mxc status
```

From the repo root you can also run `make venv` (uv) and `make py-test`.

Plain pip works too:

```bash
python -m venv .venv && . .venv/bin/activate
pip install -e '.[dev]'
```

## CLI

```bash
mxc signup                       # create a workspace + owner
mxc login                        # sign in (token stored in ~/.config/matrixcloud)
mxc status                       # runtime health + capabilities
mxc jobs                         # list jobs
mxc inspect hf:Qwen/Qwen2.5-7B-Instruct
mxc sandbox start mcp_server:filesystem \
    --cmd 'npx -y @modelcontextprotocol/server-filesystem /tmp'
mxc sandbox tools <session_id>
mxc sandbox call  <session_id> list_directory --args '{"path":"/tmp"}'
mxc sandbox stop  <session_id>
```

Point at a remote runtime with `--url` or `MATRIXCLOUD_URL`; use
`MATRIXCLOUD_TOKEN` to pass a session token in CI.

## Library

```python
from matrixcloud import MatrixCloud

with MatrixCloud("http://localhost:8080") as mc:
    mc.login("you@acme.io", "secret123")
    print(mc.capabilities()["capabilities"])
    meta = mc.inspect_model("hf:Qwen/Qwen2.5-7B-Instruct")
    print(meta["recommended_runtime"], meta["estimated_parameters"])

    s = mc.create_sandbox("mcp_server:filesystem",
                          "npx -y @modelcontextprotocol/server-filesystem /tmp")
    for ev in mc.stream_sandbox_events(s["session_id"]):
        print(ev["step"], ev["message"])
        if ev["step"] == "ready":
            break
    print([t["name"] for t in mc.sandbox_tools(s["session_id"])])
    mc.delete_sandbox(s["session_id"])
```

Licensed under Apache-2.0.
