# kafka-lag-scaler

Kubernetes operator that scales a target Deployment based on Kafka consumer-group lag instead of CPU.

## Overview

This project introduces a `KafkaScaler` custom resource. You provide the topic, consumer group, target deployment, and scaling policy (`minReplicas`, `maxReplicas`, `lagPerReplica`, cooldown). The controller periodically reads Kafka lag, calculates desired replicas, patches the target Deployment, and updates `KafkaScaler` status so you can observe current lag and scaling decisions.

## Prerequisites

- Go 1.22+
- Docker
- `kubectl`
- A Kubernetes cluster (`minikube` or `kind`)
- Helm 3
- A Kafka-compatible broker reachable from the cluster (Kafka or Redpanda)

## Run tests

Unit tests (pure policy logic):

```bash
go test ./internal/controller -run "TestCalculateReplicas|TestShouldScale" -v
```

Integration tests (envtest API server + etcd):

```bash
KUBEBUILDER_ASSETS="$(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest use -p path 1.29.x)" \
go test ./internal/controller -run TestController -v
```

## Running in a real cluster

At a high level, the runtime flow is:

1. Build the operator image from this repo (`Dockerfile`) and make it available to your cluster.
2. Install the operator chart from `helm/kafka-lag-scaler`.
3. Deploy the target workload you want to scale (`config/samples/orders-worker.yaml`).
4. Install a Kafka-compatible broker in the cluster (Kafka or Redpanda) and note its internal service DNS/port.
5. Apply `KafkaScaler` (`config/samples/kafkascaler_sample.yaml`) with `kafkaBrokers` set to your actual broker endpoint.
6. Produce backlog in the configured topic/consumer group.
7. Watch `KafkaScaler` status and target Deployment replicas update.

## Install the operator (Helm)

```bash
helm install kafka-lag-scaler ./helm/kafka-lag-scaler \
  --namespace kafka-lag-scaler-system \
  --create-namespace
```

## Apply sample resources

Target deployment:

```bash
kubectl apply -f config/samples/orders-worker.yaml
```

KafkaScaler resource:

```bash
kubectl apply -f config/samples/kafkascaler_sample.yaml
```

## Observe behavior

```bash
kubectl get kafkascalers -w
kubectl get deploy orders-worker -w
kubectl logs -n kafka-lag-scaler-system deploy/kafka-lag-scaler -f
```

## Notes

- The sample `kafkaBrokers` value is tuned for the local Redpanda test setup we used in development.
- In real environments, replace it with your actual Kafka bootstrap service addresses.
