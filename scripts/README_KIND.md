# Kind Cluster Scripts for E2E Testing

This directory contains scripts for managing kind (Kubernetes IN Docker) clusters for E2E testing.

## Scripts

### setup-kind.sh

Creates and configures a kind cluster for E2E testing with:
- Kubernetes cluster with configurable version
- cert-manager installation
- Proper webhook configuration
- Port mappings for ingress

**Usage:**
```bash
# Use default settings
./scripts/setup-kind.sh

# Customize settings with environment variables
KIND_CLUSTER=my-test-cluster \
KIND_K8S_VERSION=v1.29.0 \
CERT_MANAGER_VERSION=v1.16.3 \
./scripts/setup-kind.sh
```

**Environment Variables:**
- `KIND_CLUSTER`: Cluster name (default: `lynq-test-e2e`)
- `KIND_K8S_VERSION`: Kubernetes version (default: `v1.28.3`)
- `CERT_MANAGER_VERSION`: cert-manager version (default: `v1.16.3`)

### cleanup-kind.sh

Completely removes the kind cluster and all associated resources:
- Deletes test namespaces
- Removes Lynq CRDs
- Destroys the kind cluster
- Cleans up kubectl context

**Usage:**
```bash
# Use default cluster name
./scripts/cleanup-kind.sh

# Specify cluster name
KIND_CLUSTER=my-test-cluster ./scripts/cleanup-kind.sh
```

## E2E Test Integration

The `make test-e2e` target automatically:
1. Creates a kind cluster (if it doesn't exist)
2. Builds and loads the operator image
3. Runs the E2E tests
4. Cleans up the cluster after tests complete

**Run E2E tests:**
```bash
make test-e2e
```

**Manual cluster management:**
```bash
# Create cluster
./scripts/setup-kind.sh

# Run tests without cleanup
KIND_CLUSTER=lynq-test-e2e go test ./test/e2e/ -v -ginkgo.v

# Clean up manually
./scripts/cleanup-kind.sh
```

## Comparison with Minikube Scripts

| Feature | Kind | Minikube |
|---------|------|----------|
| Speed | Faster (containers) | Slower (VMs) |
| Resource Usage | Lower | Higher |
| CI/CD | Better suited | Less suited |
| Local Development | Good | Better for complex scenarios |
| Cleanup | Instant | Slower |

The kind scripts are optimized for CI/CD and automated testing, while minikube scripts (`setup-minikube.sh`, `cleanup-minikube.sh`) are better for local development and manual testing.
