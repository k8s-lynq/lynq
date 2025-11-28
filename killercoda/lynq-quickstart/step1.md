# Step 1: Install Prerequisites

First, let's verify our Kubernetes cluster and install **cert-manager**, which is required for Lynq's webhook certificates.

## Verify Kubernetes Cluster

Check that your cluster is ready:

```bash
kubectl get nodes
```{{exec}}

You should see a node with `Ready` status.

## Install cert-manager

Lynq requires cert-manager for managing webhook TLS certificates:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```{{exec}}

Wait for cert-manager pods to be ready:

```bash
kubectl wait --for=condition=Available --timeout=300s -n cert-manager \
  deployment/cert-manager \
  deployment/cert-manager-webhook \
  deployment/cert-manager-cainjector
```{{exec}}

Verify cert-manager is running:

```bash
kubectl get pods -n cert-manager
```{{exec}}

## Create Namespaces

Create the namespaces we'll use:

```bash
kubectl create namespace lynq-system
kubectl create namespace lynq-demo
```{{exec}}

✅ **Checkpoint**: You should have:
- A running Kubernetes cluster
- cert-manager pods running in `cert-manager` namespace
- `lynq-system` and `lynq-demo` namespaces created

Click **Continue** to deploy the Lynq operator.
