---
description: "Lynq is a Kubernetes operator that provisions and manages resources from database records — no YAML per customer, no CI/CD trigger per change."
---

# What is Lynq?

Lynq is a Kubernetes operator that synchronizes your database with your cluster. When a row is active, the corresponding Kubernetes resources exist. When the row is deactivated or deleted, the resources are cleaned up.

```
Database row (active)  →  LynqNode  →  Deployment + Service + Ingress + ...
Database row (inactive) →  resources deleted
```

No manual `kubectl apply`. No CI/CD pipeline per change. No drift.

## The Core Idea

Lynq implements **Infrastructure as Data** — a pattern where your database is the source of truth for infrastructure state, rather than Git or configuration files.

| Approach | Source of truth | How you change state |
|----------|----------------|---------------------|
| IaC (Terraform, Pulumi) | Code in Git | Edit code, run apply |
| GitOps (Flux, Argo CD) | YAML in Git | Commit YAML, wait for sync |
| **IaD (Lynq)** | **Database rows** | **INSERT / UPDATE / DELETE** |

This is most useful when you already have a database that describes what should exist — a customer table, a device registry, a project list. Lynq closes the loop between that data and the cluster.

## Three CRDs

Lynq adds three custom resources to your cluster:

**LynqHub** — connects to a datasource and syncs rows at a configured interval:
```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
spec:
  source:
    type: mysql
    mysql:
      host: mysql.default.svc.cluster.local
      database: saas_db
      table: customers
      # ...
  valueMappings:
    uid: customer_id     # which column is the unique ID
    activate: is_active  # which column controls provisioning
```

**LynqForm** — defines the Kubernetes resources to create for each active row:
```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
spec:
  hubId: customer-hub
  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      spec:
        # standard Deployment spec, with Go template variables
```

**LynqNode** — one per active row, created automatically. Tracks reconciliation status and the lifecycle of all managed resources.

## What Gets Provisioned

A single LynqForm can define any combination of:

Deployments · StatefulSets · DaemonSets · Services · Ingresses · ConfigMaps · Secrets · PersistentVolumeClaims · Jobs · CronJobs · HorizontalPodAutoscalers · PodDisruptionBudgets · NetworkPolicies · Namespaces · ServiceAccounts · arbitrary manifests

Resources can be placed in the same namespace as the LynqNode or in any other namespace via `targetNamespace`.

## Production Capabilities

- **Policies** — `creationPolicy`, `deletionPolicy`, `conflictPolicy` give fine-grained control over resource lifecycle
- **Dependencies** — `dependIds` + `waitForReady` enforce resource ordering within a node
- **Rollout control** — `maxSkew` limits how many nodes update simultaneously when a template changes
- **Orphan tracking** — resources removed from a template are either deleted or retained with audit markers
- **Observability** — 15 Prometheus metrics, structured logs, Kubernetes events per reconciliation
- **Webhooks** — validation and defaulting webhooks prevent invalid CRDs from reaching the cluster

## When to Use Lynq

Lynq is a good fit when:
- You have a database table describing things that need corresponding Kubernetes resources
- The resource structure is repetitive across many rows (same template, different variables)
- You want provisioning to happen automatically when data changes

Common use cases: multi-node SaaS platforms, edge device fleets, ephemeral dev environments, per-customer database provisioning, feature-flag–driven deployments.

## Datasource Support

- **MySQL** — fully supported (v1.x)
- **PostgreSQL** — planned for v1.2

## See Also

- [Quick Start](quickstart.md) — running on Minikube in 5 minutes
- [Infrastructure as Data](recordops.md) — the paradigm in depth
- [How It Works](how-it-works.md) — controller internals and reconciliation flow
- [API Reference](api.md) — full CRD schema
