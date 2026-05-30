# Security

The MVP goal is **safe-enough** execution of verified MCP servers and
short-lived sandboxes — not perfect isolation. Treat the runtime as a
trust-boundary component and deploy it accordingly (network egress controls,
resource limits, non-root containers).

## Command validation

Start commands are validated by `internal/security/commands.go`.

**Allowed program prefixes:** `npx`, `uvx`, `pipx`, `python`, `python3`, `node`.

**Rejected:**

- Shell chaining and control: `&&`, `||`, `;`, `|`
- Redirects: `>`, `>>`, `<`
- Backgrounding: `&`
- Command substitution / backticks: `` ` ``, `$( … )`
- Blocked tokens: `sudo`, `docker`, `apt`, `apt-get`, `apk`, `yum`, `dnf`,
  `systemctl`, `mkfs`, `mount`, `curl`, `wget`, `sh`, `bash`, `zsh`, `eval`

Because shell metacharacters are rejected, the command is tokenised with a
simple quote-aware splitter — no shell, expansion or globbing is involved.

## Runtime limits

| Limit                  | Default     |
|------------------------|-------------|
| max TTL                | 600s        |
| install timeout        | 120s        |
| startup timeout        | 45s         |
| RPC timeout            | 20s         |
| max log bytes          | 1 MiB       |
| max concurrent jobs    | 1 (hf-space) / 5 (customer-agent) |

Processes are launched with `exec.CommandContext` bound to a TTL-scoped context,
in a per-job temporary directory that is deleted on completion. The child
process is never exposed directly to the API caller.

## Secrets

For the MVP:

- **`mcp.test` must never receive raw user secrets.** Env entries whose key
  looks sensitive (`*token*`, `*password*`, `*api_key*`, …) are rejected unless
  the value is a **secret reference** (`${secret:name}` or `secret://…`).
- Secret references are reserved for future `mcp.run` / `agent.run`.

## API authentication

Set `MATRIX_RUNTIME_API_TOKEN` to require an `Authorization: Bearer <token>`
header on all endpoints except `/v1/health`. When unset (typical for
`local-dev`), the API is open.

## Hybrid posture

In `customer-agent` mode the runtime connects **outbound** to MatrixHub Cloud;
no inbound exposure is required. Secrets, internal APIs, MCP servers and model
access stay inside customer infrastructure.
