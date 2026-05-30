# Install: hybrid cloud

In the hybrid model, **MatrixHub Cloud remains SaaS** (the control plane) while
the customer runs only `matrix-runtime` inside their own infrastructure.

```
MatrixHub Cloud (SaaS)         Customer infrastructure
  control plane        ◀────── matrix-runtime (outbound only)
                                  • execution
                                  • secrets
                                  • internal APIs
                                  • model access
```

The runtime connects **outbound** to MatrixHub Cloud, so enterprise customers do
not need to open inbound firewall holes. Execution, secrets, internal APIs and
model access all stay inside customer infra.

## Join the cloud

```bash
matrix-runtime join \
  --cloud-url https://cloud.matrixhub.io \
  --token mxrt_xxxxx
```

or:

```bash
./scripts/join.sh --cloud-url https://cloud.matrixhub.io --token mxrt_xxxxx
```

This writes `~/.matrix/runtime/config.yaml`:

```yaml
cloud_url: https://cloud.matrixhub.io
join_token: mxrt_xxxxx
runtime_id: rt_acme_prod
workspace: acme
```

## Deploy with Helm

```bash
helm install matrix-runtime ./deploy/helm/matrix-runtime \
  --namespace matrix-runtime \
  --create-namespace \
  --set cloud.url=https://cloud.matrixhub.io \
  --set runtime.joinToken=mxrt_xxxxx
```

## Configuration

| Variable                    | Meaning                          |
|-----------------------------|----------------------------------|
| `MATRIX_CLOUD_URL`          | Control-plane URL                |
| `MATRIX_RUNTIME_JOIN_TOKEN` | Join token (`mxrt_…`)            |
| `MATRIX_RUNTIME_ID`         | Optional stable runtime id       |
| `MATRIX_RUNTIME_WORKSPACE`  | Workspace name                   |

## Control channel

The future control channel — the runtime opening an outbound WebSocket/HTTPS
connection to matrix-hub, which then pushes jobs and receives streamed
logs/results — is designed (`internal/controlplane/tunnel.go`) but stubbed. For
the MVP the direct HTTP API is sufficient.
