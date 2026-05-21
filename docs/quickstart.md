---
description: "Get Lynq running on Minikube in under 5 minutes. Automated scripts set up a complete local environment with MySQL, sample data, and live node provisioning."
---

# Quick Start

::: tip Time to working
~5 minutes. Automated scripts handle cluster, cert-manager, MySQL, and operator setup.
:::

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Minikube | v1.28.0+ | `brew install minikube` (macOS) or [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/) |
| kubectl | v1.28.0+ | `brew install kubectl` or [kubernetes.io](https://kubernetes.io/docs/tasks/tools/) |
| Docker | Latest | Docker Desktop (macOS) or Docker Engine (Linux) |

System requirements: 1+ core, 1+ GB RAM, 5+ GB disk.

## Setup (5 Steps)

### Step 1: Prepare Minikube Cluster

<QuickstartStep
  :step="1"
  title="Prepare Minikube Cluster"
  duration="~2 min"
  command="./scripts/setup-minikube.sh"
  focus="Bootstraps the base cluster with cert-manager and Lynq CRDs."
  :creates='[
    "Minikube control plane + kubeconfig",
    "cert-manager v1.13.2",
    "Namespaces: lynq-system, lynq-test",
    "Lynq CRDs"
  ]'
  :checklist='[
    "kubectl get nodes shows Ready",
    "kubectl get pods -n cert-manager",
    "kubectl get crds | grep lynq"
  ]'
  next-hint="Once the cluster objects are ready, continue with Step 2 to deploy the controller."
  next-target-id="qs-step-2"
/>

### Step 2: Deploy Lynq

<QuickstartStep
  :step="2"
  title="Deploy Lynq Operator"
  duration="~2 min"
  command="./scripts/deploy-to-minikube.sh"
  focus="Builds the controller image and deploys it into lynq-system."
  :prerequisites='[
    "Step 1 complete: Minikube + cert-manager running",
    "kubectl context points to Minikube"
  ]'
  :creates='[
    "Lynq controller-manager Deployment/Service",
    "Webhook configuration and TLS Secret (via cert-manager)",
    "Metrics and leader-election resources"
  ]'
  :checklist='[
    "kubectl get pods -n lynq-system to confirm controller Ready",
    "kubectl logs -n lynq-system -l control-plane=controller-manager --tail=20",
    "kubectl get validatingwebhookconfiguration | grep lynq"
  ]'
  next-hint="With the controller online you can move to Step 3 and deploy the database."
  next-target-id="qs-step-3"
/>

### Step 3: Deploy MySQL Test Database

<QuickstartStep
  :step="3"
  title="Seed MySQL Test Database"
  duration="~1 min"
  command="./scripts/deploy-mysql.sh"
  focus="Installs a MySQL 8.0 instance with sample node rows inside lynq-test."
  :prerequisites='[
    "Step 2 complete: Lynq operator is running",
    "Namespace lynq-test exists (created during Step 1)"
  ]'
  :creates='[
    "mysql Deployment/Service",
    "nodes database and node_configs table",
    "Three sample node rows",
    "Read-only user node_reader and Secret"
  ]'
  :checklist="[
    'kubectl get pods -n lynq-test | grep mysql',
    'kubectl exec -it deployment/mysql -n lynq-test -- mysql -e &quot;SHOW DATABASES;&quot;',
    'kubectl get secret mysql-credentials -n lynq-test'
  ]"
  next-hint="With the datasource online you can create the LynqHub in Step 4."
  next-target-id="qs-step-4"
/>

### Step 4: Create LynqHub

<QuickstartStep
  :step="4"
  title="Create LynqHub"
  duration="~30 sec"
  command="./scripts/deploy-lynqhub.sh"
  focus="Creates the LynqHub CR that syncs MySQL rows every 30 seconds."
  :prerequisites="[
    'Step 3 complete: MySQL endpoint is Ready',
    'mysql-credentials Secret exists'
  ]"
  :creates="[
    'LynqHub CR (test-hub)',
    'Column and extraValue mappings',
    'Recurring sync loop'
  ]"
  :checklist="[
    'kubectl get lynqhub test-hub -n lynq-system -o yaml | grep syncInterval',
    'kubectl get lynqhub test-hub -o jsonpath=&quot;{.status.desiredNodes}&quot; 2>/dev/null || true',
    'Tail operator logs for hub sync messages'
  ]"
  next-hint="Once the hub is feeding node specs you can define the LynqForm in Step 5."
  next-target-id="qs-step-5"
/>

### Step 5: Apply LynqForm

<QuickstartStep
  :step="5"
  title="Apply LynqForm"
  duration="~30 sec"
  command="./scripts/deploy-lynqform.sh"
  focus="Defines the blueprint (Deployment, Service, etc.) for each active node."
  :prerequisites='[
    "Step 4 complete: test-hub reports Ready",
    "Permissions to create templates in the same namespace as the hub"
  ]'
  :creates='[
    "LynqForm CR (test-template)",
    "Deployment/Service definitions",
    "Hub ↔ Template linkage"
  ]'
  :checklist='[
    "kubectl get lynqform test-template -n lynq-system",
    "kubectl get lynqnodes",
    "kubectl get deployments,services -n lynq-test -l lynq.sh/node"
  ]'
  next-hint="All steps are done. Adding a database row now provisions resources automatically."
  next-target-id="qs-success"
/>

<div id="qs-success"></div>

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

<details>
<summary>Cleanup</summary>

```bash
# Remove everything except the cluster
kubectl delete lynqnodes --all -n lynq-system
kubectl delete lynqform test-template -n lynq-system
kubectl delete lynqhub test-hub -n lynq-system

# Full teardown
./scripts/cleanup-minikube.sh
```

Scripts support env var overrides — run `./scripts/<name>.sh --help` for options.
</details>

## See Also

- [Installation](installation.md) — deploy to a production cluster
- [Datasources](datasource.md) — connect to your own MySQL database
- [Templates](templates.md) — template syntax and 200+ functions
- [Policies](policies.md) — control resource creation, deletion, and conflict behavior
- [Use Cases](advanced-use-cases.md) — common patterns and worked examples
