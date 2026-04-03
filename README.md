# kafka-lag-scaler

"A Kubernetes controller written in Go that implements the reconciliation pattern to autoscale workloads based on real-time Kafka consumer lag."

## How it works

`kafka-lag-scaler` stores desired state as a `KafkaScaler` custom resource in the Kubernetes API server, which persists that state in etcd and emits watch events when resources change. The controller-runtime reconciler consumes those events and runs an Observe → Diff → Act → Requeue loop: it observes the `KafkaScaler`, reads Kafka consumer lag through the Kafka Admin API, diffs desired replicas against current deployment state, and patches the target `Deployment` when the lag policy requires a change. Status is written back to the `KafkaScaler` status subresource so the resource itself becomes the distributed control plane record for the controller's latest observation. The system is intentionally eventually consistent: changes propagate through watch events and periodic requeues rather than synchronous request/response hooks.

## Architecture diagram

```text
                +-----------------------------+
                | KafkaScaler CR              |
                | spec + status               |
                +-------------+---------------+
                              |
                              v
                +-----------------------------+
                | Kubernetes API Server       |
                | persisted in etcd           |
                +-------------+---------------+
                              |
                       watch / get / update
                              |
                              v
                +-----------------------------+
                | kafka-lag-scaler Operator   |
                | reconcile loop              |
                +------+------+---------------+
                       |      |
           get lag     |      | patch replicas
                       |      |
                       v      v
        +-----------------+  +----------------------+
        | Kafka Admin API |  | Target Deployment    |
        | end/commit offs |  | spec.replicas        |
        +-----------------+  +----------------------+
                       \
                        \ update status
                         \
                          v
                +-----------------------------+
                | KafkaScaler Status          |
                | currentLag / replicas / cond|
                +-----------------------------+
```

## Key distributed systems concepts implemented

- Reconciliation loop pattern
- etcd as distributed state store plus watch events
- Eventual consistency via `RequeueAfter`, not webhooks
- Leader election with a Kubernetes `Lease` for active-passive HA
- Optimistic concurrency with `Patch` + `MergeFrom`, not `Update`
- Status/spec separation where users write spec and the controller writes status

## Project structure

```text
.
├── api/
│   └── v1alpha1/
│       ├── groupversion_info.go            # API group/version registration and AddToScheme
│       ├── kafkascaler_types.go            # KafkaScaler spec/status types and CRD markers
│       └── zz_generated.deepcopy.go        # Runtime deepcopy implementations for API objects
├── internal/
│   └── controller/
│       ├── kafka_client.go                 # franz-go Kafka admin client for computing consumer lag
│       ├── kafkascaler_controller.go       # Reconciler, finalizers, status updates, deployment patching
│       ├── kafkascaler_controller_test.go  # envtest integration cases for scale up, cooldown, and scale down
│       ├── scaler.go                       # Pure scaling policy functions
│       ├── scaler_test.go                  # Unit tests for replica math and cooldown logic
│       └── suite_test.go                   # envtest suite bootstrap, manager startup, shared mock lag fetcher
├── config/
│   ├── crd/
│   │   └── bases/
│   │       └── autoscaling.kafkascaler.io_kafkascalers.yaml # CRD manifest used by envtest and packaging
│   └── samples/
│       └── kafkascaler_sample.yaml        # Demo custom resource for quickstart and live walkthroughs
├── helm/
│   └── kafka-lag-scaler/
│       ├── Chart.yaml                     # Helm chart metadata
│       ├── crds/
│       │   └── kafkascalers.yaml          # Native Helm CRD installation payload
│       ├── templates/
│       │   ├── _helpers.tpl               # Naming and label helpers
│       │   ├── clusterrole.yaml           # Cluster-scoped RBAC for CRD, deployments, events, and leases
│       │   ├── clusterrolebinding.yaml    # Binds operator RBAC to the service account
│       │   ├── deployment.yaml            # Operator deployment with leader election and health probes
│       │   └── serviceaccount.yaml        # Service account used by the controller
│       └── values.yaml                    # Install-time overrides for image, HA, and resources
├── go.mod                                 # Module definition and direct dependencies
└── go.sum                                 # Dependency checksums
```

## Quickstart

### Prerequisites

- Go 1.22+
- `kubectl` plus `minikube` or `kind`
- Helm 3
- A running Kafka cluster

### Install the operator

```bash
helm install kafka-lag-scaler ./helm/kafka-lag-scaler \
  --namespace kafka-lag-scaler-system \
  --create-namespace
```

### Apply a KafkaScaler

```bash
kubectl apply -f config/samples/kafkascaler_sample.yaml
```

### Watch it work

```bash
kubectl get kafkascalers -w
```

Expected output:

```text
NAME                     CURRENTLAG   CURRENTREPLICAS   LASTSCALETIME
orders-consumer-scaler   5000         5                 2026-04-03T14:25:11Z
orders-consumer-scaler   0            2                 2026-04-03T14:29:42Z
```

## Running tests

1. Unit tests (pure functions, no cluster):

```bash
go test ./internal/controller -run "TestCalculateReplicas|TestShouldScale" -v
```

2. Integration tests (envtest, no cluster needed):

```bash
KUBEBUILDER_ASSETS="$(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest use -p path 1.29.x)" \
go test ./internal/controller -run TestController -v
```
