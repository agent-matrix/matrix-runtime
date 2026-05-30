# MatrixHub Helm Chart

MatrixHub is an open-source, self-hosted AI model registry engineered for large-scale enterprise inference. It serves as a drop-in private replacement for Hugging Face, purpose-built to accelerate vLLM and SGLang workloads.
## Introduction

This chart bootstraps a [MatrixHub](https://github.com/matrixhub-ai/matrixhub) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+

## Installing the Chart

To install the chart with the release name `matrixhub`:

```bash
helm install matrixhub ./deploy/charts/matrixhub
```

The command deploys MatrixHub on the Kubernetes cluster in the default configuration.

## Uninstalling the Chart

To uninstall/delete the `matrixhub` deployment:

```bash
helm delete matrixhub
```

## Configuration

The following table lists the configurable parameters of the MatrixHub chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `apiserver.replicaCount` | Number of replicas | `1` |
| `apiserver.labels` | Deployment labels | `{app: matrixhub-apiserver}` |
| `apiserver.podAnnotations` | Pod annotations | `{}` |
| `apiserver.podLabels` | Pod labels | `{}` |
| `apiserver.image.registry` | Image registry | `ghcr.io` |
| `apiserver.image.repository` | Image repository | `matrixhub-ai/matrixhub` |
| `apiserver.image.tag` | Image tag | `""` (defaults to appVersion) |
| `apiserver.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `apiserver.image.pullSecrets` | Image pull secrets | `[]` |
| `apiserver.service.type` | Kubernetes service type | `ClusterIP` |
| `apiserver.service.port` | Service port | `9527` |
| `apiserver.service.nodePort` | NodePort (when type=NodePort) | `30001` |
| `apiserver.service.annotations` | Service annotations | `{}` |
| `apiserver.resources.limits.cpu` | CPU limit | `500m` |
| `apiserver.resources.limits.memory` | Memory limit | `512Mi` |
| `apiserver.resources.requests.cpu` | CPU request | `50m` |
| `apiserver.resources.requests.memory` | Memory request | `128Mi` |
| `apiserver.debug` | Debug mode | `false` |
| `apiserver.logLevel` | Log level (debug/info/warn/error) | `warn` |
| `apiserver.port` | API server port | `9527` |
| `apiserver.database.driver` | Database driver (mysql/postgres) | `mysql` |
| `apiserver.database.accessType` | Database access type | `readwrite` |
| `apiserver.database.maxOpenConns` | Max open connections | `100` |
| `apiserver.database.maxIdleConns` | Max idle connections | `10` |
| `apiserver.database.connMaxLifetimeSeconds` | Connection max lifetime | `3600` |
| `apiserver.database.connMaxIdleSeconds` | Connection max idle time | `1800` |
| `apiserver.database.migrate` | Enable database migration | `true` |
| `apiserver.database.migrationPath` | Migration path | `/etc/matrixhub/migrations` |
| `apiserver.database.dsn` | Custom database DSN | `""` |
| `mysql.registry` | MySQL image registry | `docker.io` |
| `mysql.repository` | MySQL image repository | `library/mysql` |
| `mysql.tag` | MySQL image tag | `5.7.39` |
| `mysql.pullPolicy` | MySQL pull policy | `IfNotPresent` |
| `mysql.persistence.size` | PVC size | `8Gi` |
| `mysql.persistence.storageClass` | Storage class | `""` (default) |
| `mysql.pullSecrets` | MySQL pull secrets | `[]` |
| `mysql.rootPassword` | MySQL root password | `password` |
| `global.imagePullSecrets` | Global image pull secrets | `[]` |
| `global.imageRegistry` | Global image registry override | `""` |
| `global.storage.apiserver.builtIn` | Use built-in MySQL | `true` |
| `global.busybox.image.repository` | Busybox image repository | `busybox` |
| `global.busybox.image.tag` | Busybox image tag | `latest` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```bash
helm install matrixhub ./deploy/charts/matrixhub --set apiserver.image.tag=latest
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example:

```bash
helm install matrixhub ./deploy/charts/matrixhub -f values.yaml
```

## Storage

By default, the chart deploys a built-in MySQL database with persistent storage. To use an external database, set:

```yaml
global:
  storage:
    apiserver:
      builtIn: false

apiserver:
  database:
    dsn: "your-custom-dsn-string"
```

## Exposing the Service

### ClusterIP (default)

The service is exposed as ClusterIP and accessible from within the cluster:

```bash
export POD_NAME=$(kubectl get pods --namespace matrixhub -l app=matrixhub-apiserver -o jsonpath="{.items[0].metadata.name}")
kubectl port-forward $POD_NAME 9527:9527 --namespace matrixhub
```

### NodePort

To expose the service via NodePort:

```bash
helm install matrixhub ./deploy/charts/matrixhub --set apiserver.service.type=NodePort
```