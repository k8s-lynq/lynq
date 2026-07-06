---
url: 'https://lynq.sh/quickstart.md'
description: >-
  Get Lynq running on Minikube in under 5 minutes. Automated scripts set up a
  complete local environment with MySQL, sample data, and live node
  provisioning.
---

# Quick Start

::: tip Time to working
\~5 minutes. Automated scripts handle cluster, cert-manager, MySQL, and operator setup.
:::

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Minikube | v1.28.0+ | `brew install minikube` (macOS) or [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/) |
| kubectl | v1.28.0+ | `brew install kubectl` or [kubernetes.io](https://kubernetes.io/docs/tasks/tools/) |
| Docker | Latest | Docker Desktop (macOS) or Docker Engine (Linux) |

System requirements: 1+ core, 1+ GB RAM, 5+ GB disk.

All setup commands below run from the repository root:

```bash
git clone https://github.com/k8s-lynq/lynq && cd lynq
```

## Setup (5 Steps)

### Step 1: Prepare Minikube Cluster

Run (~2 min): `./scripts/setup-minikube.sh`

Bootstraps the base cluster with cert-manager and Lynq CRDs. Creates: Minikube control plane + kubeconfig, cert-manager v1.13.2, namespaces `lynq-system` / `lynq-test`, Lynq CRDs.

Verify:

```bash
kubectl get nodes                  # shows Ready
kubectl get pods -n cert-manager
kubectl get crds | grep lynq
```

### Step 2: Deploy Lynq

Run (~2 min): `./scripts/deploy-to-minikube.sh`

Requires Step 1 complete (Minikube + cert-manager running, kubectl context pointing to Minikube). Builds the controller image and deploys it into `lynq-system`. Creates: Lynq controller-manager Deployment/Service, webhook configuration and TLS Secret (via cert-manager), metrics and leader-election resources.

Verify:

```bash
kubectl get pods -n lynq-system    # controller Ready
kubectl logs -n lynq-system -l control-plane=controller-manager --tail=20
kubectl get validatingwebhookconfiguration | grep lynq
```

### Step 3: Deploy MySQL Test Database

Run (~1 min): `./scripts/deploy-mysql.sh`

Requires Step 2 complete (Lynq operator running; namespace `lynq-test` exists from Step 1). Installs a MySQL 8.0 instance with sample node rows inside `lynq-test`. Creates: mysql Deployment/Service, nodes database and node\_configs table, three sample node rows, read-only user `node_reader` and its Secret.

Verify:

```bash
kubectl get pods -n lynq-test | grep mysql
kubectl exec -it deployment/mysql -n lynq-test -- mysql -e "SHOW DATABASES;"
kubectl get secret mysql-credentials -n lynq-test
```

### Step 4: Create LynqHub

Run (~30 sec): `./scripts/deploy-lynqhub.sh`

Requires Step 3 complete (MySQL endpoint Ready; `mysql-credentials` Secret exists). Creates the LynqHub CR that syncs MySQL rows every 30 seconds: LynqHub CR `test-hub`, column and extraValue mappings, recurring sync loop.

Verify:

```bash
kubectl get lynqhub test-hub -n lynq-system -o yaml | grep syncInterval
kubectl get lynqhub test-hub -o jsonpath="{.status.desiredNodes}" 2>/dev/null || true
kubectl logs -n lynq-system -l control-plane=controller-manager --tail=20   # hub sync messages
```

### Step 5: Apply LynqForm

Run (~30 sec): `./scripts/deploy-lynqform.sh`

Requires Step 4 complete (`test-hub` reports Ready; permissions to create templates in the same namespace as the hub). Defines the blueprint (Deployment, Service, etc.) for each active node. Creates: LynqForm CR `test-template`, Deployment/Service definitions, hub-template linkage.

Verify:

```bash
kubectl get lynqform test-template -n lynq-system
kubectl get lynqnodes
kubectl get deployments,services -n lynq-test -l lynq.sh/node
```

All steps are done. Adding a database row now provisions resources automatically.

## Verify It Works

```bash
# 1. LynqNode CRs — one per active row
kubectl get lynqnodes -n lynq-system
# NAME                          READY   DESIRED   FAILED   AGE
# acme-corp-test-template       2/2     2         0        2m
# beta-inc-test-template        2/2     2         0        2m

# 2. Resources created for each active node
kubectl get deployments,services -n lynq-test -l lynq.sh/node

# 3. Live lifecycle: insert a row → resource appears within 30s
# The deploy-mysql.sh script seeds the `tenant_registry.tenants` table
# (default; override via MYSQL_DATABASE).
kubectl exec -it deployment/mysql -n lynq-test -- \
  mysql -u root -p"$(kubectl get secret mysql-credentials -n lynq-test -o jsonpath='{.data.password}' | base64 -d)" \
  -e "INSERT INTO tenant_registry.tenants (uid, host_or_url, activate, deploy_image, plan_id) VALUES ('delta-co', 'https://delta.example.com', TRUE, 'nginx:stable', 'starter');"

sleep 35
kubectl get lynqnode delta-co-test-template -n lynq-system

# 4. Deactivate → resources clean up within 30s
kubectl exec -it deployment/mysql -n lynq-test -- \
  mysql -u root -p"$(kubectl get secret mysql-credentials -n lynq-test -o jsonpath='{.data.password}' | base64 -d)" \
  -e "UPDATE tenant_registry.tenants SET activate = FALSE WHERE uid = 'delta-co';"

sleep 35
kubectl get lynqnode delta-co-test-template -n lynq-system  # Not found
```

## Troubleshooting

**Operator not starting** — cert-manager must be Ready before Lynq starts:

```bash
kubectl get pods -n cert-manager
kubectl logs -n lynq-system -l control-plane=controller-manager
```

**Nodes not created** — check the hub sync and confirm `activate = TRUE` in the database:

```bash
kubectl get lynqhub test-hub -n lynq-system -o yaml
kubectl exec -it deployment/mysql -n lynq-test -- \
  mysql -u node_reader -p"$(kubectl get secret mysql-credentials -n lynq-test -o jsonpath='{.data.password}' | base64 -d)" \
  -e "SELECT uid, activate FROM tenant_registry.tenants;"
```

**Resources missing** — inspect the LynqNode status for failures:

```bash
kubectl describe lynqnode <name> -n lynq-system
```

For detailed diagnostics, see [Troubleshooting](troubleshooting.md).

## Cleanup

```bash
# Remove everything except the cluster
kubectl delete lynqnodes --all -n lynq-system
kubectl delete lynqform test-template -n lynq-system
kubectl delete lynqhub test-hub -n lynq-system

# Full teardown
./scripts/cleanup-minikube.sh
```

Scripts support env var overrides — run `./scripts/<name>.sh --help` for options.

## See Also

* [Installation](installation.md) — deploy to a production cluster
* [Datasources](datasource.md) — connect to your own MySQL database
* [Templates](templates.md) — template syntax and 200+ functions
* [Policies](policies.md) — control resource creation, deletion, and conflict behavior
* [Use Cases](advanced-use-cases.md) — common patterns and worked examples
