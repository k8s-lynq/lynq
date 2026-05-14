---
description: "What Lynq is, why database-driven infrastructure matters, and when to use it — the complete concept in one page."
---

# Introduction to Lynq

Lynq is a Kubernetes operator that turns database rows into Kubernetes resources. When a row is active, the corresponding infrastructure exists. When the row is deactivated or deleted, the resources are cleaned up.

```
Database row (active)    →  Deployment + Service + Ingress + ...
Database row (inactive)  →  resources deleted
```

No manual `kubectl apply`. No CI/CD pipeline per change. No drift.

## The Core Idea

Most infrastructure automation treats Git as the source of truth: write YAML, commit, wait for sync. Lynq takes a different approach — your **database** is the source of truth.

| Approach | Source of truth | How you change state |
|----------|----------------|---------------------|
| IaC (Terraform, Pulumi) | Code in Git | Edit code, run apply |
| GitOps (Flux, Argo CD) | YAML in Git | Commit YAML, wait for sync |
| **Lynq (Infrastructure as Data)** | **Database rows** | **INSERT / UPDATE / DELETE** |

This is most useful when you already have a database that describes what should exist — a customer table, a device registry, a project list. Lynq closes the loop between that data and your cluster.

## Three CRDs

Lynq adds three custom resources to your cluster:

- **LynqHub** — connects to your database and syncs rows at a configured interval. Defines which columns map to which template variables.
- **LynqForm** — the resource blueprint. Defines what Kubernetes resources to create for each active row, using Go templates with 200+ functions.
- **LynqNode** — one per active row per form, created automatically. Tracks reconciliation status and the lifecycle of all managed resources.

The relationship: one Hub can be referenced by multiple Forms. Each active row × each Form = one LynqNode. A hub with 3 active rows and 2 forms creates 6 LynqNodes.

## What Gets Provisioned

A single LynqForm can define any combination of:

Deployments · StatefulSets · DaemonSets · Services · Ingresses · ConfigMaps · Secrets · PersistentVolumeClaims · Jobs · CronJobs · HorizontalPodAutoscalers · PodDisruptionBudgets · NetworkPolicies · Namespaces · ServiceAccounts · arbitrary manifests

Resources can target the LynqNode's namespace or any other namespace via `targetNamespace`.

## Infrastructure as Data in Practice

Operational changes become SQL statements:

```sql
-- Scale a node
UPDATE customers SET replicas = 10 WHERE id = 'acme-corp';

-- Enable a feature (infrastructure-level)
UPDATE nodes SET feature_ai_assistant = TRUE WHERE node_id = 'acme-corp';

-- Traffic switch (blue-green)
UPDATE deployments SET active_color = 'green' WHERE node_id = 'acme-corp';
```

The cluster syncs within the hub's `syncInterval` (default: 1 minute). No new tooling, no context switching — operations your application already knows how to do.

::: tip Combine IaD with GitOps
Use GitOps for cluster-level infrastructure (CRDs, operators, system config) and Lynq for application-level nodes (per-customer stacks, per-device configs, ephemeral environments). They complement each other.
:::

## When to Use Lynq

::: tip Good fit when:
- You have a database table where each row corresponds to a Kubernetes resource set
- The resource structure is repetitive across rows (same template, different variables)
- Provisioning should happen automatically when data changes
- You provision more than a handful of times per day
:::

::: warning Better to use IaC if:
- Infrastructure changes require manual approval for every update
- Changes are infrequent (once a month or less)
- Deep cloud provider integrations are the primary need (Terraform is better)
- Infrastructure doesn't map cleanly to database records
:::

## Datasource Support

- **MySQL** — fully supported (v1.x)
- **PostgreSQL** — planned for v1.2

## See Also

- [Quick Start](quickstart.md) — working environment in ~5 minutes on Minikube
- [Architecture](architecture.md) — three-controller design, reconciliation flow, SSA engine
- [Datasources](datasource.md) — connecting to MySQL, value mappings, database views
- [Use Cases](advanced-use-cases.md) — preview environments, sandboxes, feature flags, blue-green
