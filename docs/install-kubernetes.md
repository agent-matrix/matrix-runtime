# Install: Kubernetes

Two options: the Helm chart (recommended) or raw manifests.

## Helm

```bash
helm install matrix-runtime ./deploy/helm/matrix-runtime \
  --namespace matrix-runtime \
  --create-namespace
```

Render without installing to inspect the output:

```bash
helm template matrix-runtime ./deploy/helm/matrix-runtime
```

Profiles are provided as values files:

```bash
helm install matrix-runtime ./deploy/helm/matrix-runtime \
  -n matrix-runtime --create-namespace \
  -f deploy/helm/matrix-runtime/values-hybrid.yaml
```

- `values-hybrid.yaml`  — customer-agent dialing out to MatrixHub Cloud
- `values-hf-space.yaml` — single short-lived sandbox
- `values-local.yaml`    — in-cluster local dev (NodePort)

## Raw manifests

```bash
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/configmap.yaml
cp deploy/k8s/secret.example.yaml deploy/k8s/secret.yaml   # edit it first
kubectl apply -f deploy/k8s/secret.yaml
kubectl apply -f deploy/k8s/pvc.yaml
kubectl apply -f deploy/k8s/deployment.yaml
kubectl apply -f deploy/k8s/service.yaml
```

Or use `./scripts/install.sh --mode kubernetes`.

## Verify

```bash
kubectl -n matrix-runtime port-forward svc/matrix-runtime 8080:8080
curl http://localhost:8080/v1/health
```
