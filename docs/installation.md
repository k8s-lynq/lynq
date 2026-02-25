---
description: "Install Lynq on Kubernetes using Helm, Kustomize, or from source. Covers prerequisites, cert-manager setup, and multi-platform support."
---

# Installation Guide

This guide covers various installation methods for Lynq.

[[toc]]

::: tip Trying it locally?
Use the [Quick Start with Minikube](quickstart.md) guide for an automated setup tailored to first-time users.
:::

## Prerequisites

### Required

| Component | Minimum version | Notes |
| --- | --- | --- |
| Kubernetes cluster | v1.11.3+ | API compatibility tested with recent releases |
| `kubectl` | Matches cluster | Must target the cluster where you deploy |
| **cert-manager** | **v1.13.0+** | **REQUIRED for all installations** (production, development, local) |

::: danger cert-manager is REQUIRED
**cert-manager v1.13.0+** is **REQUIRED for ALL installations** (production, development, and local environments). It provisions webhook TLS certificates, handles automatic renewal, and injects CA bundles into webhook configurations.

**Webhooks are no longer optional.** They provide essential validation and defaulting at admission time.

Install before deploying Lynq:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```
:::

### Optional

- **MySQL database** for node data source (PostgreSQL support planned for v1.2)

## Kubernetes Compatibility

### Supported Versions

The operator relies only on GA/stable Kubernetes APIs and controller-runtime patterns, making it compatible across the supported upstream version skew.

**Validated versions** (end-to-end tested and production-verified):

| Kubernetes Version | Lynq Version | Status |
|--------------------|--------------|--------|
| v1.34              | v1.1.16+     | ✅ Validated |
| v1.33              | v1.1.16+     | ✅ Validated |
| v1.32              | v1.1.16+     | ✅ Validated |
| v1.31              | v1.1.16+     | ✅ Validated |
| v1.30              | ≤ v1.1.15    | ✅ Validated |
| v1.29              | ≤ v1.1.15    | ✅ Validated |
| v1.28              | ≤ v1.1.15    | ✅ Validated |
| Other GA releases  | -            | ⚠️ Expected to work |

::: info Version Support Policy
- **Kubernetes v1.31+**: Actively tested and validated with Lynq v1.1.16 and later releases
- **Kubernetes v1.28–v1.30**: Validated up to Lynq v1.1.15. These versions are expected to work with newer Lynq releases but are no longer part of the active E2E test matrix
:::

::: tip Compatibility Philosophy
The operator is designed to work across Kubernetes version skew. Earlier or newer versions are expected to function, but validate in a staging environment before rolling out broadly.
:::

## Installation Methods

### Method 1: Install with Helm (Recommended)

**cert-manager is REQUIRED** for all installations.

```bash
# Step 1: Install cert-manager (REQUIRED)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Step 2: Wait for cert-manager to be ready
kubectl wait --for=condition=Available --timeout=300s -n cert-manager \
  deployment/cert-manager \
  deployment/cert-manager-webhook \
  deployment/cert-manager-cainjector

# Step 3: Add Helm repository
helm repo add lynq https://k8s-lynq.github.io/lynq
helm repo update

# Step 4: Install Lynq
helm install lynq lynq/lynq \
  --namespace lynq-system \
  --create-namespace
```

See the [Helm Chart README](https://github.com/k8s-lynq/lynq/blob/main/chart/README.md) for detailed configuration options.

---

### Method 2: Install with Kustomize

**cert-manager is REQUIRED** for webhook TLS certificate management.

```bash
# Step 1: Install cert-manager (if not already installed)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Step 2: Wait for cert-manager to be ready
kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager
kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager-webhook

# Step 3: Install Lynq
# cert-manager will automatically issue and manage webhook TLS certificates
kubectl apply -k https://github.com/k8s-lynq/lynq/config/default
```

::: info What cert-manager handles
- Issues TLS certificates for the webhook server
- Renews certificates before expiration
- Injects the CA bundle into webhook configurations
- Provides battle-tested certificate automation for Kubernetes clusters
:::

### Method 3: Install from Source

```bash
# Clone repository
git clone https://github.com/k8s-lynq/lynq.git
cd lynq

# Install CRDs
make install

# Install cert-manager first if not already installed
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Deploy operator
make deploy IMG=ghcr.io/k8s-lynq/lynq:latest
```

::: warning Remember TLS
Even when deploying from source, install cert-manager before applying the operator manifests, otherwise webhooks will fail to start.
:::

### Method 4: Local Development with Minikube

For local development, use Minikube with automated setup scripts. **cert-manager is automatically installed** by the setup script.

See [Local Development with Minikube](local-development-minikube.md) for detailed instructions.

```bash
# Quick setup (cert-manager included)
./scripts/setup-minikube.sh      # Create cluster with cert-manager
./scripts/deploy-to-minikube.sh  # Build and deploy operator
```

::: tip cert-manager in Local Development
The setup script automatically installs cert-manager. You don't need to install it manually for local development when using the provided scripts.
:::

## Verification

Check that the operator is running:

```bash
# Check operator deployment
kubectl get deployment -n lynq-system lynq-controller-manager

# Check operator logs
kubectl logs -n lynq-system deployment/lynq-controller-manager -f

# Verify CRDs are installed
kubectl get crd | grep operator.lynq.sh
```

Expected output:
```
lynqhubs.operator.lynq.sh    2025-01-15T10:00:00Z
lynqnodes.operator.lynq.sh             2025-01-15T10:00:00Z
lynqforms.operator.lynq.sh     2025-01-15T10:00:00Z
```

::: tip Troubleshooting
If the deployment is not ready, inspect `kubectl describe deployment/lynq-controller-manager` for webhook, RBAC, or image issues.
:::

## Configuration Options

### Webhook TLS Configuration

Webhook TLS is managed automatically by cert-manager. The default configuration includes:

```yaml
# config/default/kustomization.yaml
# Webhook patches are enabled by default
patches:
- path: manager_webhook_patch.yaml
- path: webhookcainjection_patch.yaml
```

::: info cert-manager responsibilities
- Issue TLS certificates for the webhook server
- Inject CA bundles into webhook configurations
- Renew certificates before expiration
:::

### Resource Limits

Adjust operator resource limits based on your cluster size:

```yaml
# config/manager/manager.yaml
resources:
  limits:
    cpu: 500m      # Increase for large clusters
    memory: 512Mi  # Increase for many nodes
  requests:
    cpu: 100m
    memory: 128Mi
```

### Concurrency Settings

Configure concurrent reconciliation workers:

```yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --node-concurrency=10        # Concurrent LynqNode reconciliations (default: 10)
        - --form-concurrency=5       # Concurrent Template reconciliations (default: 5)
        - --hub-concurrency=3       # Concurrent Hub syncs (default: 3)
        - --leader-elect                 # Enable leader election
```

## Multi-Platform Support

The operator supports multiple architectures:

- `linux/amd64` (Intel/AMD 64-bit)
- `linux/arm64` (ARM 64-bit, Apple Silicon)

Container images are automatically pulled for your platform.

## Namespace Isolation

By default, the operator is installed in `lynq-system` namespace:

```bash
# Check operator namespace
kubectl get all -n lynq-system

# View RBAC
kubectl get clusterrole | grep lynq
kubectl get clusterrolebinding | grep lynq
```

## Upgrading

### Upgrade CRDs First

```bash
# Upgrade CRDs (safe, preserves existing data)
make install

# Or with kubectl
kubectl apply -f config/crd/bases/
```

### Upgrade Operator

```bash
# Update operator deployment
kubectl set image -n lynq-system \
  deployment/lynq-controller-manager \
  manager=ghcr.io/k8s-lynq/lynq:v1.1.0

# Or use make
make deploy IMG=ghcr.io/k8s-lynq/lynq:v1.1.0
```

### Rolling Back

```bash
# Rollback to previous version
kubectl rollout undo -n lynq-system \
  deployment/lynq-controller-manager

# Check rollout status
kubectl rollout status -n lynq-system \
  deployment/lynq-controller-manager
```

## Uninstallation

```bash
# Delete operator deployment
kubectl delete -k config/default

# Or with make
make undeploy

# Delete CRDs (WARNING: This deletes all LynqNode data!)
make uninstall

# Or with kubectl
kubectl delete crd lynqhubs.operator.lynq.sh
kubectl delete crd lynqforms.operator.lynq.sh
kubectl delete crd lynqnodes.operator.lynq.sh
```

**Warning:** Deleting CRDs will delete all LynqHub, LynqForm, and LynqNode resources. Ensure you have backups if needed.

## Troubleshooting Installation

### Webhook TLS Errors

**Error:** `open /tmp/k8s-webhook-server/serving-certs/tls.crt: no such file or directory`

**Solution:** Install cert-manager to automatically manage webhook TLS certificates.

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager

# Restart operator to pick up certificates
kubectl rollout restart -n lynq-system deployment/lynq-controller-manager
```

### CRD Already Exists

**Error:** `Error from server (AlreadyExists): customresourcedefinitions.apiextensions.k8s.io "lynqnodes.operator.lynq.sh" already exists`

**Solution:** This is normal during upgrades. CRD updates are applied automatically.

### Image Pull Errors

**Error:** `Failed to pull image "ghcr.io/k8s-lynq/lynq:latest"`

**Solution:** Ensure your cluster can access GitHub Container Registry (ghcr.io). Check network policies and image pull secrets if needed.

### Permission Denied

**Error:** `Error from server (Forbidden): User "system:serviceaccount:lynq-system:lynq-controller-manager" cannot create resource`

**Solution:** Ensure RBAC resources are installed:
```bash
kubectl apply -f config/rbac/
```

## Next Steps

- [Create your first LynqHub](quickstart.md#step-4-deploy-lynqhub)
- [Learn about Templates](templates.md)
- [Configure Monitoring](monitoring.md)
- [Set up Security](security.md)
