---
description: "Control resource lifecycle with Lynq's four policy types: CreationPolicy, DeletionPolicy, ConflictPolicy, and PatchStrategy."
---

# Policies

Four policy types control how Lynq creates, updates, and deletes managed resources. Each is set per-resource in the LynqForm spec.

## Overview

| Policy | Controls | Default | Options |
|--------|----------|---------|---------|
| `creationPolicy` | When resources are created/re-applied | `WhenNeeded` | `Once`, `WhenNeeded` |
| `deletionPolicy` | What happens when LynqNode is deleted | `Delete` | `Delete`, `Retain` |
| `conflictPolicy` | Ownership conflict handling | `Stuck` | `Stuck`, `Force` |
| `patchStrategy` | How resources are updated | `apply` | `apply`, `merge`, `replace` |

All four default to the safest option. Set only the policies that need to differ from the default.

::: tip Field-level control
To ignore specific fields during reconciliation (e.g., HPA-managed `replicas`) while still using `WhenNeeded`, see [Field-Level Ignore Control](field-ignore.md).
:::

## CreationPolicy

Controls when a resource is created or re-applied.

### `WhenNeeded` (Default)

Resource is created on first reconcile and updated whenever the spec changes.

```yaml
deployments:
  - id: app
    creationPolicy: WhenNeeded  # default
    nameTemplate: "{{ .uid }}-app"
    spec: ...
```

Use when resources should stay synchronized with the template (standard for Deployments, Services, ConfigMaps).

### `Once`

Resource is created once and never updated by Lynq, even if the template changes. Re-created if manually deleted.

```yaml
jobs:
  - id: init-job
    creationPolicy: Once
    nameTemplate: "{{ .uid }}-init"
    spec: ...
```

Lynq adds annotation `lynq.sh/created-once: "true"` to track creation. Use for init Jobs, database migrations, bootstrap scripts — anything that must run exactly once.

## DeletionPolicy

Controls what happens to resources when the LynqNode CR is deleted.

### `Delete` (Default)

Resource is removed when the LynqNode is deleted. Kubernetes uses `ownerReference` for automatic garbage collection.

```yaml
deployments:
  - id: app
    deletionPolicy: Delete  # default
```

Use for stateless resources (Deployments, Services, ConfigMaps) that have no value after the node is gone.

### `Retain`

Resource **stays in the cluster** when the LynqNode is deleted. Label-based tracking is used instead of `ownerReference` — this is what prevents the Kubernetes GC from auto-deleting the resource.

```yaml
persistentVolumeClaims:
  - id: data-pvc
    deletionPolicy: Retain
    nameTemplate: "{{ .uid }}-data"
    spec: ...
```

On deletion, Lynq adds orphan markers:
```yaml
metadata:
  labels:
    lynq.sh/orphaned: "true"
  annotations:
    lynq.sh/orphaned-at: "2025-01-15T10:30:00Z"
    lynq.sh/orphaned-reason: "LynqNodeDeleted"   # or "RemovedFromTemplate"
```

Find retained resources:
```bash
kubectl get all -A -l lynq.sh/orphaned=true
```

Use for PVCs, audit logs, database instances — resources where data must survive node deletion.

::: warning DeletionPolicy is evaluated at creation time
`ownerReference` is set (or not set) when the resource is first created. If you later change a resource from `Delete` to `Retain`, trigger a reconcile to re-track it using labels. See [Policy Operations](policies-operations.md) for migration steps.
:::

### Orphan Re-adoption

If a retained resource is re-added to the LynqForm template, Lynq removes all orphan markers and resumes management. No manual cleanup needed.

## ConflictPolicy

Controls what happens when a resource already exists with a different SSA field manager.

### `Stuck` (Default)

Reconciliation stops on conflict. LynqNode is marked `Degraded`. A `ResourceConflict` event is emitted.

```yaml
services:
  - id: svc
    conflictPolicy: Stuck  # default
```

Use when safety matters more than availability, or when resources might be managed by other controllers.

### `Force`

Lynq takes ownership using SSA with `force=true`. Emits a `ForceApply` warning event.

```yaml
deployments:
  - id: app
    conflictPolicy: Force
```

Use when Lynq should be the sole source of truth, or when migrating from another management system. This **can overwrite** fields owned by other controllers.

## PatchStrategy

Controls how resources are updated.

### `apply` (Default — Server-Side Apply)

Declarative, field-manager-aware updates. Preserves fields owned by other controllers (e.g., HPA-managed `replicas`).

```yaml
deployments:
  - id: app
    patchStrategy: apply  # default; field manager: "lynq"
```

### `merge` (Strategic Merge Patch)

Merges changes with the existing resource. Preserves unspecified fields. Less precise conflict detection than SSA.

Use for partial updates or compatibility with legacy systems.

### `replace` (Full Replacement)

Completely replaces the resource. Removes any fields not in the template.

Use only when exact resource state is required and no other controller manages the resource.

## Defaults

```yaml
resources:
  - id: example
    creationPolicy: WhenNeeded  # default
    deletionPolicy: Delete      # default
    conflictPolicy: Stuck       # default
    patchStrategy: apply        # default
```

## Recommended Combinations

| Resource Type | creationPolicy | deletionPolicy | conflictPolicy | patchStrategy |
|---------------|----------------|----------------|----------------|---------------|
| Deployment | WhenNeeded | Delete | Stuck | apply |
| Service | WhenNeeded | Delete | Stuck | apply |
| ConfigMap | WhenNeeded | Delete | Stuck | apply |
| Ingress | WhenNeeded | Delete | Stuck | apply |
| Secret | WhenNeeded | Delete | Force | apply |
| PVC | Once | Retain | Stuck | apply |
| Init Job | Once | Delete | Force | replace |
| Namespace | WhenNeeded | Retain | Force | apply |

Key reasoning:
- **PVC**: `Once` because PVC spec is immutable; `Retain` because storage holds data.
- **Init Job**: `Once` to run exactly once; `replace` because Job specs are immutable.
- **Namespace**: `Retain` because deleting a namespace cascades to all its contents.
- **Secret**: `Force` because secrets are often pre-created by external systems (Vault, External Secrets Operator).

## Events

```bash
# Watch policy-related events
kubectl describe lynqnode <name> -n lynq-system
```

Key events:
- `ResourceConflict` — conflict detected (Stuck policy)
- `ForceApply` — forced ownership transfer
- `DependencySkipped` — resource skipped due to failed dependency
- `LynqNodeDeleting` / `LynqNodeDeleted` — deletion lifecycle

## Metrics

```promql
# Apply attempts by policy
apply_attempts_total{kind="Deployment", conflict_policy="Stuck"}

# Current conflicts
lynqnode_conflicts_total{lynqnode="acme-web", conflict_policy="Stuck"}

# Degraded nodes
lynqnode_degraded_status{reason="ConflictDetected"}
```

See [Monitoring](monitoring.md) for the full metrics reference.

## See Also

- [Policy Operations](policies-operations.md) — conflict resolution, cascade deletion protection, and policy migration runbooks
- [Policy Examples](policies-examples.md) — worked examples with diagrams
- [Field-Level Ignore](field-ignore.md) — ignore specific fields during reconciliation
- [Dependencies](dependencies.md) — `skipOnDependencyFailure` and dependency failure behavior
