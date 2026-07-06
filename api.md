---
url: 'https://lynq.sh/api.md'
description: >-
  Lynq Kubernetes CRD reference index — LynqHub, LynqForm, LynqNode overview,
  common types, shared labels/annotations, and kubectl commands.
---

# API Reference

Lynq adds three Custom Resource Definitions to your cluster.

| CRD | API Group | Purpose |
|-----|-----------|---------|
| [LynqHub](api-lynqhub.md) | `operator.lynq.sh/v1` | Database connection + sync configuration |
| [LynqForm](api-lynqform.md) | `operator.lynq.sh/v1` | Resource blueprint (what to create per active row) |
| [LynqNode](api-lynqnode.md) | `operator.lynq.sh/v1` | Instance for one row × one form; tracks reconciliation status |

**Naming convention:** LynqNode CRs follow `{uid}-{form-name}`. A hub with 3 active rows and 2 forms creates 6 LynqNodes: `acme-web-app`, `acme-worker`, `beta-web-app`, `beta-worker`, `corp-web-app`, `corp-worker`.

## Common Types

### Duration

String with unit suffix. Pattern: `^\d+(s|m|h)$`.

Examples: `30s`, `1m`, `2h`

### CreationPolicy

| Value | Behavior |
|-------|----------|
| `WhenNeeded` (default) | Create, then re-apply only when the rendered spec changes (annotation-based skip) or during periodic drift-correction |
| `Once` | Create once; never update even if spec changes |

### DeletionPolicy

| Value | Behavior |
|-------|----------|
| `Delete` (default) | Set `ownerReference` → Kubernetes GC removes resource when LynqNode is deleted |
| `Retain` | Label-based tracking only (no `ownerReference`) → resource stays after LynqNode deletion |

DeletionPolicy is evaluated at **creation time**. The tracking mechanism (ownerReference vs labels) is set when the resource is first created.

### ConflictPolicy

| Value | Behavior |
|-------|----------|
| `Stuck` (default) | Stop reconciliation on SSA field-manager conflict; mark LynqNode Degraded |
| `Force` | Apply with `force=true`; take ownership from other field managers |

### PatchStrategy

| Value | Behavior |
|-------|----------|
| `apply` (default) | Server-Side Apply (SSA) — field-manager-aware, preserves other controllers' fields |
| `merge` | Strategic merge patch — preserves unspecified fields |
| `replace` | Full replacement — removes fields not in the template |

## Labels Reference

### On LynqNode CRs (set by hub controller)

| Label | Value |
|-------|-------|
| `lynq.sh/hub` | Hub name |
| `lynq.sh/uid` | Row UID |

### On managed resources (set by LynqNode controller)

| Label | Set when | Value |
|-------|----------|-------|
| `lynq.sh/node` | Cross-namespace, Retain policy, or Namespace resources | LynqNode name |
| `lynq.sh/node-namespace` | Paired with `lynq.sh/node` | LynqNode namespace |
| `lynq.sh/orphaned` | Resource removed from template or LynqNode deleted (Retain) | `"true"` |

### Annotations on managed resources

| Annotation | Value | Purpose |
|-----------|-------|---------|
| `lynq.sh/deletion-policy` | `"Delete"` or `"Retain"` | Stored at creation; used during orphan cleanup |
| `lynq.sh/created-once` | `"true"` | Marks resources created with `creationPolicy: Once` |
| `lynq.sh/orphaned-at` | RFC3339 timestamp | When orphaned |
| `lynq.sh/orphaned-reason` | `"RemovedFromTemplate"` or `"LynqNodeDeleted"` | Why orphaned |

## kubectl Quick Reference

### Listing resources

```bash
kubectl get lynqhubs -n lynq-system
kubectl get lynqforms -n lynq-system
kubectl get lynqnodes -n lynq-system

# All namespaces
kubectl get lynqnodes -A

# Wide output (shows more columns)
kubectl get lynqnodes -A -o wide
```

### Checking status

```bash
# Hub sync status
kubectl get lynqhub <name> -o jsonpath='{.status}'

# Node readiness
kubectl get lynqnode <name> -o jsonpath='{.status.readyResources}/{.status.desiredResources}'

# Node conditions
kubectl get lynqnode <name> -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}){"\n"}{end}'

# Applied resources list
kubectl get lynqnode <name> -o jsonpath='{.status.appliedResources}'
```

### Querying by labels

```bash
# Resources managed by a specific node
kubectl get all -l lynq.sh/node=<lynqnode-name>

# Orphaned resources cluster-wide
kubectl get all -A -l lynq.sh/orphaned=true

# Nodes from a specific hub
kubectl get lynqnodes -l lynq.sh/hub=<hub-name>
```

### Operations

```bash
# Force reconciliation
kubectl annotate lynqnode <name> lynq.sh/force-reconcile=$(date +%s) --overwrite

# Describe (shows events)
kubectl describe lynqnode <name>

# Custom columns output
kubectl get lynqnodes -o custom-columns='NAME:.metadata.name,READY:.status.readyResources,DESIRED:.status.desiredResources,FAILED:.status.failedResources'

# JSON for scripting
kubectl get lynqnodes -o json | jq '.items[] | {name: .metadata.name, ready: .status.readyResources}'
```

## See Also

* [LynqHub API](api-lynqhub.md) — full hub spec and status reference
* [LynqForm API](api-lynqform.md) — full form spec, TResource structure, rollout
* [LynqNode API](api-lynqnode.md) — full node status, conditions, lifecycle
* [Resource Lifecycle](api-lifecycle.md) — orphan markers, finalizers, cross-namespace tracking
* [Policies](policies.md) — policy behavior in detail
* [Templates](templates.md) — template syntax and available variables
