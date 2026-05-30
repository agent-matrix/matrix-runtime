# Install: on-prem

For the MVP, the on-prem story **is** the hybrid model: customers run
`matrix-runtime` in their own infrastructure (Kubernetes or Docker) while
MatrixHub Cloud remains the SaaS control plane. See
[install-hybrid.md](install-hybrid.md) and [install-kubernetes.md](install-kubernetes.md).

A fully air-gapped Matrix Cloud — control plane, catalog, model registry and
gateway all on-prem — is **out of scope** for this first runtime. Those pieces
(private model registry, Git/LFS server, dataset registry, enterprise RBAC,
air-gapped bundles) belong to a future `matrix-registry` / enterprise offering.

## What stays inside customer infra today

- MCP server execution and short-lived sandboxes
- Secrets and secret references
- Internal API access from agents/tools (when implemented)
- Hugging Face model access and the local model cache

## What remains SaaS today

- The control plane (matrix-hub): catalog, workspaces, jobs, policies
- The model gateway (matrix-llm)

## Bare-metal / VM install (systemd)

For a host install without containers, use the production `make install`:

```bash
sudo make install INSTALL_SYSTEMD=1
sudo $EDITOR /etc/matrix-runtime/matrix-runtime.env
sudo systemctl daemon-reload
sudo systemctl enable --now matrix-runtime
systemctl status matrix-runtime
journalctl -u matrix-runtime -f
```

This installs a stripped, static, version-stamped binary to
`/usr/local/bin/matrix-runtime`, creates a non-login `matrix` service user and
the `/var/lib/matrix-runtime` data directory, and drops a hardened systemd unit
(`NoNewPrivileges`, `ProtectSystem=strict`, `PrivateTmp`, restricted namespaces,
`ReadWritePaths` limited to the data dir). Override locations with `PREFIX`,
`DATA_DIR`, `SERVICE_USER`, `ENV_FILE`, or stage into a package root with
`DESTDIR`.

## Hardening checklist

- Set `MATRIX_RUNTIME_API_TOKEN` to require a bearer token on the API.
- Run with a read-only root filesystem where possible and a non-root user
  (the chart and Dockerfile already use uid 10001).
- Restrict egress to the Hugging Face Hub and MatrixHub Cloud only.
- Keep `MATRIX_RUNTIME_MAX_TTL_SECONDS` and `MAX_CONCURRENT_JOBS` conservative.
