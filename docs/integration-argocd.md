# Argo CD Integration Guide

Deliver GitOps-driven node environments by mapping each **LynqNode** to an **Argo CD Application**.

[[toc]]

## Overview

Lynq can render Argo CD `Application` manifests for every active node row. Each LynqNode becomes the canonical source of truth for a corresponding Argo CD Application, enabling GitOps workflows, progressive delivery, and automated cleanup.

```mermaid
flowchart LR
    Registry["LynqHub<br/>DB rows"]
    Template["LynqForm<br/>Argo CD manifest"]
    Node["LynqNode CR"]
    ArgoApp["Argo CD Application"]
    ArgoCD["Argo CD Controller"]
    Targets["Target Cluster / Namespace"]

    Hub --> Template --> Node --> ArgoApp --> ArgoCD --> Targets

    classDef control fill:#e3f2fd,stroke:#64b5f6,stroke-width:2px;
    classDef argocd fill:#fff3e0,stroke:#ffb74d,stroke-width:2px;
    class Hub,Template,Node control;
    class ArgoApp,ArgoCD argocd;
```

### Core Benefits

- **1:1 Mapping** – Every LynqNode owns exactly one Argo CD Application (`LynqNode` ↔️ `Application`).
- **Automatic Sync** – Application source paths follow node metadata (UID, plan, region).
- **Declarative Cleanup** – When a LynqNode is deleted (or deactivated), the Argo CD Application and downstream workloads are removed.
- **GitOps Alignment** – Teams keep delivery pipelines in Git, while Lynq handles orchestration and lifecycle.

## Prerequisites

- Argo CD installed (v2.8+ recommended) and accessible from the node namespace.
- ServiceAccount and RBAC granting Lynq permission to create Argo CD `Application` objects in the Argo CD namespace (often `argocd`).
- Lynq chart deployed with namespace permissions covering the Argo CD API group.
- Git repository that hosts node application configuration.

## Baseline Template (1 LynqNode ➝ 1 Application)

The following template renders an Argo CD Application per LynqNode. Each `Application` points to a unique Git path derived from node metadata.

::: v-pre
```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: argocd-app-template
spec:
  hubId: saas-registry

  manifests:
    - id: argocd-app
      nameTemplate: "{{ printf \"%s-app\" (.uid | trunc63) }}"
      spec:
        apiVersion: argoproj.io/v1alpha1
        kind: Application
        metadata:
          namespace: argocd
          labels:
            node.lynq.sh/uid: "{{ .uid }}"
            node.lynq.sh/region: "{{ .region | default \"global\" }}"
        spec:
          project: nodes
          source:
            repoURL: https://github.com/your-org/node-configs.git
            targetRevision: main
            path: "nodes/{{ .uid }}"
          destination:
            server: https://kubernetes.default.svc
            namespace: "{{ .uid }}-workspace"
          syncPolicy:
            automated:
              prune: true
              selfHeal: true
            syncOptions:
              - CreateNamespace=true
              - ApplyOutOfSyncOnly=true
```
:::

### Flow

```mermaid
sequenceDiagram
    participant DB as MySQL (Node Data)
    participant Registry as LynqHub Controller
    participant Node as LynqNode CR
    participant Operator as LynqNode Controller
    participant Argo as Argo CD Controller

    Hub->>DB: SELECT active nodes
    DB-->>Hub: Node rows
    Hub->>Node: Create/Update LynqNode CR
    Operator->>Node: Render Argo CD manifest
    Operator->>Argo: Apply Application (SSA)
    Argo->>Argo: Sync Git repo
    Argo->>Cluster: Deploy node workloads
```

## Advanced Sync Patterns

::: v-pre
| Pattern | Description | LynqForm Template Hints |
| --- | --- | --- |
| **Environment Branching** | Target different Git branches per region or plan (`targetRevision: "{{ ternary \"main\" \"staging\" (eq .planId \"enterprise\") }}"`). | Use extra value mappings for `planId`, `region`. |
| **Dynamic Paths** | Compose repo paths from UID segments (`path: "nodes/{{ .region }}/{{ .uid }}"`). | Use Sprig `splitList`, `join`, `default`. |
| **App-of-Apps** | Point each node to an `Application` that references node-specific sub-apps. | Render Application with `path: nodes/{{ .uid }}/apps`. |
| **Multi-Cluster Delivery** | Route nodes to dedicated clusters using Argo CD credentials (`destination.server`). | Map datasource columns to `clusterServer`, `clusterName`. |
| **Progressive Rollouts** | Annotate Applications for Argo Rollouts or Progressive Sync plugins. | Add `metadata.annotations` via templates. |
:::

## Additional Use Cases

### 1. AppSet Fan-Out per Node Plan

- Combine Lynq with Argo CD ApplicationSet.
- Lynq renders a control-plane Application that references an ApplicationSet generator.
- Generator reads node metadata (via ConfigMap/Secret) to produce feature-specific Applications per plan tier.

### 2. Multi-Cluster Nodes with Cluster Secrets

- Add `extraValueMappings` for cluster credentials.
- LynqForm template creates:
  1. An Argo CD `ClusterSecret` (with kubeconfig) in the Argo CD namespace.
  2. An `Application` targeting that cluster secret.
- Enables dedicated clusters per enterprise node.

### 3. Canary and Blue/Green Releases

- Render two Applications per node (`node-app-canary`, `node-app-stable`) with different `targetRevision`.
- Use `creationPolicy: Once` on the stable Application and `WhenNeeded` on the canary for rapid rollback.
- Combine with Argo Rollouts by templating `analysis` and promotion hooks.

```mermaid
flowchart TD
    NodeA["LynqNode (Enterprise)"]
    Canary["Application<br/>targetRevision=canary"]
    Stable["Application<br/>targetRevision=stable"]
    Rollouts["Argo Rollouts / Analysis"]
    Namespace["LynqNode Namespace"]

    NodeA --> Canary --> Rollouts --> Namespace
    NodeA --> Stable --> Namespace
```

## Verification Commands

After deploying, verify the integration works correctly:

```bash
# 1. Check LynqNodes created Argo CD Applications
kubectl get applications -n argocd -l node.lynq.sh/uid

# Example output:
# NAME              SYNC STATUS   HEALTH STATUS   AGE
# acme-corp-app     Synced        Healthy         5m
# beta-inc-app      Synced        Healthy         5m

# 2. Verify Application points to correct Git path
kubectl get application acme-corp-app -n argocd -o jsonpath='{.spec.source.path}'
# Expected: nodes/acme-corp

# 3. Check sync status
kubectl get application acme-corp-app -n argocd -o jsonpath='{.status.sync.status}'
# Expected: Synced

# 4. Check health status
kubectl get application acme-corp-app -n argocd -o jsonpath='{.status.health.status}'
# Expected: Healthy

# 5. View sync history
argocd app history acme-corp-app

# 6. Force sync (if needed)
argocd app sync acme-corp-app

# 7. Check downstream workloads
kubectl get all -n acme-corp-workspace

# 8. View Application events
kubectl describe application acme-corp-app -n argocd | tail -20
```

**Monitor Both Systems:**

```bash
# Combined health check
echo "=== LynqNode Status ===" && \
kubectl get lynqnode acme-corp-web-app -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' && \
echo "" && \
echo "=== Argo CD Application Status ===" && \
kubectl get application acme-corp-app -n argocd -o jsonpath='{.status.sync.status}/{.status.health.status}'

# Expected: True (LynqNode Ready) + Synced/Healthy (Argo CD)
```

## Operational Tips

- Label Applications with node metadata for quick filtering (`node.lynq.sh/uid`).
- Grant the operator service account access to `argoproj.io` API group via ClusterRole.
- Monitor Argo CD sync status alongside LynqNode status; both must be healthy for end-to-end readiness.
- Use the `Retain` deletion policy when you need to keep Applications for post-mortem analysis.

## What to Read Next

- [Templates Guide](templates.md) – Advanced templating and function usage.
- [Policies Guide](policies.md) – Control resource lifecycle (Retain vs. Delete).
- [Monitoring Guide](monitoring.md) – Capture Argo CD and Lynq metrics together.
