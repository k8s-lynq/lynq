---
description: "Install Lynq on a Kubernetes cluster using Helm (recommended), Kustomize, or from source. Covers cert-manager setup, verification, and upgrades."
---

# Installation

This guide covers deploying Lynq to a production or staging cluster. For a local Minikube setup, see [Quick Start](quickstart.md).

## Prerequisites

| Component | Minimum version | Notes |
|-----------|----------------|-------|
| Kubernetes | v1.28+ | v1.31+ for latest test coverage |
| kubectl | Matches cluster | |
| cert-manager | **v1.13.0+** | **Required.** Manages webhook TLS automatically. |

### Install cert-manager

cert-manager must be running before Lynq is deployed. It provisions and renews the TLS certificates that Lynq's admission webhooks require.

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for all three cert-manager components to be ready
kubectl wait --for=condition=Available --timeout=300s -n cert-manager \
  deployment/cert-manager \
  deployment/cert-manager-webhook \
  deployment/cert-manager-cainjector
```

If cert-manager is already installed (v1.13.0+), skip this step.

## Install

### Helm (Recommended)

```bash
helm repo add lynq https://k8s-lynq.github.io/lynq
helm repo update

helm install lynq lynq/lynq \
  --namespace lynq-system \
  --create-namespace
```

See the [Helm Chart README](https://github.com/k8s-lynq/lynq/blob/main/chart/README.md) for all values.

### Kustomize

```bash
kubectl apply -k https://github.com/k8s-lynq/lynq/config/default
```

### From Source

```bash
git clone https://github.com/k8s-lynq/lynq.git
cd lynq
make install                                    # install CRDs
make deploy IMG=ghcr.io/k8s-lynq/lynq:latest  # deploy operator
```

## Verify

```bash
kubectl get deployment -n lynq-system lynq-controller-manager
kubectl get crd | grep operator.lynq.sh
```

Expected CRDs:
```
lynqhubs.operator.lynq.sh
lynqforms.operator.lynq.sh
lynqnodes.operator.lynq.sh
```

If the operator pod is crashing, check webhook TLS first:
```bash
kubectl describe pod -n lynq-system -l control-plane=controller-manager
```

## Configuration

### Resource Limits

```yaml
# config/manager/manager.yaml (shipped defaults)
resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi
```

Increase memory before scaling beyond a few hundred LynqNodes. See [Resource Sizing](resource-sizing.md) for benchmarks and a CPU/memory model.

### Concurrency

```yaml
args:
  - --hub-concurrency=3    # concurrent hub syncs (default: 3)
  - --form-concurrency=5   # concurrent form reconciliations (default: 5)
  - --node-concurrency=10  # concurrent node reconciliations (default: 10)
  - --leader-elect
```

### Multi-Architecture

Pre-built images support `linux/amd64` and `linux/arm64`. Docker automatically selects the right image for your nodes.

## Upgrade

Always upgrade CRDs before upgrading the operator:

```bash
# 1. Upgrade CRDs (preserves existing data)
kubectl apply -f config/crd/bases/

# 2. Upgrade operator
helm upgrade lynq lynq/lynq --namespace lynq-system
# or:
kubectl set image -n lynq-system \
  deployment/lynq-controller-manager \
  manager=ghcr.io/k8s-lynq/lynq:v1.1.20
```

To roll back:
```bash
kubectl rollout undo -n lynq-system deployment/lynq-controller-manager
```

## Uninstall

```bash
# Remove operator (keeps CRDs and node data)
helm uninstall lynq --namespace lynq-system
# or: kubectl delete -k config/default

# Remove CRDs — this deletes all LynqHub, LynqForm, and LynqNode resources
make uninstall
# or: kubectl delete crd lynqhubs.operator.lynq.sh lynqforms.operator.lynq.sh lynqnodes.operator.lynq.sh
```

::: warning CRD deletion is destructive
Deleting the CRDs deletes all custom resources. Back up your LynqHub and LynqForm manifests before running `make uninstall`.
:::

## Troubleshooting Installation

**`no such file or directory: tls.crt`** — cert-manager is not ready or not installed:
```bash
kubectl get pods -n cert-manager
# If missing: install cert-manager (see Prerequisites above)
kubectl rollout restart -n lynq-system deployment/lynq-controller-manager
```

**`AlreadyExists` on CRDs** — normal during upgrades. The apply is idempotent.

**`ImagePullBackOff`** — cluster can't reach `ghcr.io`. Check network policies or configure an image pull secret.

**`Forbidden: cannot create resource`** — RBAC not applied:
```bash
kubectl apply -f config/rbac/
```

## See Also

- [Quick Start](quickstart.md) — local Minikube setup with automated scripts
- [Configuration](configuration.md) — all operator flags and settings
- [Security](security.md) — RBAC, credential management, and audit logging
- [Monitoring](monitoring.md) — Prometheus metrics and alerting
