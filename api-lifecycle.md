---
url: 'https://lynq.sh/api-lifecycle.md'
description: >-
  Lynq resource lifecycle internals: tracking labels, orphan markers,
  finalizers, cross-namespace resources, and watch predicates.
---

# Resource Lifecycle

How Lynq tracks, cleans up, and re-adopts resources — covering labels, annotations, finalizers, cross-namespace support, and reconciliation triggers.

::: tip Related

* [API Reference](api.md) — CRD schema (fields, types, defaults)
* [Policies Guide](policies.md) — `deletionPolicy`, `creationPolicy`, `conflictPolicy`
  :::

***

## Tracking Labels and Annotations

### Labels Set on LynqNode CRs

The hub controller sets these labels when creating LynqNode CRs:

| Label | Value | Purpose |
|-------|-------|---------|
| `lynq.sh/hub` | Hub name | Which LynqHub owns this node |
| `lynq.sh/uid` | Row UID | Which database row this node represents |

### Labels Set on Managed Resources

The operator sets these on resources it creates, depending on the tracking mechanism used:

| Label | Value | When set |
|-------|-------|---------|
| `lynq.sh/node` | LynqNode name | Cross-namespace resources; Retain resources; Namespaces |
| `lynq.sh/node-namespace` | LynqNode namespace | Same — paired with `lynq.sh/node` |

Same-namespace resources with `deletionPolicy: Delete` (the default) use Kubernetes `ownerReferences` instead of these labels. The garbage collector then handles cleanup automatically.

### Annotations Set on Managed Resources

| Annotation | Value | Purpose |
|-----------|-------|---------|
| `lynq.sh/deletion-policy` | `"Delete"` or `"Retain"` | Stored at creation time; used during orphan cleanup |
| `lynq.sh/created-once` | `"true"` | Set when `creationPolicy: Once` resource is created; prevents re-application |

### Annotations Set on LynqNode CRs

The LynqHub controller stamps these annotations on every LynqNode CR it creates. The LynqNode controller reads them when rendering templates.

| Annotation | Value | Purpose |
|-----------|-------|---------|
| `lynq.sh/hubId` | LynqHub name | Source hub for this node |
| `lynq.sh/activate` | `"true"`/`"false"` | Activation column value (string form) |
| `lynq.sh/hostOrUrl` | URL/host column | Deprecated `.hostOrUrl` template variable (removed in v1.3.0) |
| `lynq.sh/extra` | JSON-encoded object | All `extraValueMappings` keys/values packed into a single annotation |
| `lynq.sh/template-generation` | int (from `LynqForm.metadata.generation`) | Used by rollout tracking to detect template changes |
| `lynq.sh/rollout-update-start` | RFC3339 timestamp | Stamped each time the hub re-renders the node spec |

(The `.uid` template variable comes from the LynqNode's `lynq.sh/uid` **label**, not an annotation.)

***

## Orphan Markers

When a resource is removed from a LynqForm but its `deletionPolicy` is `Retain`, the operator:

1. Removes tracking labels (`lynq.sh/node`, `lynq.sh/node-namespace`)
2. Adds orphan markers

| Field | Key | Value |
|-------|-----|-------|
| Label | `lynq.sh/orphaned` | `"true"` — for label selector queries |
| Annotation | `lynq.sh/orphaned-at` | RFC3339 timestamp |
| Annotation | `lynq.sh/orphaned-reason` | `"RemovedFromTemplate"` or `"LynqNodeDeleted"` |

**Why split label/annotation?**
Kubernetes label values must be short and RFC 1123 compliant, so the label carries only `"true"` for selector use. Annotations hold the richer metadata (timestamp, reason).

### Finding Orphaned Resources

```bash
# All orphaned resources across all namespaces
kubectl get all -A -l lynq.sh/orphaned=true

# Filter by reason
kubectl get all -A -l lynq.sh/orphaned=true \
  -o jsonpath='{range .items[?(@.metadata.annotations.lynq\.sh/orphaned-reason=="RemovedFromTemplate")]}{.kind}/{.metadata.name}{"\n"}{end}'

# Orphaned resources from a specific node
kubectl get all -A -l lynq.sh/orphaned=true,lynq.sh/node=my-node
```

### Orphan Marker Lifecycle

1. Resource removed from LynqForm → orphan markers added
2. Resource re-added to LynqForm → orphan markers automatically removed on next reconcile
3. LynqNode deleted (with `deletionPolicy: Retain`) → orphan markers added with reason `LynqNodeDeleted`

**Before (orphaned):**

```yaml
metadata:
  labels:
    lynq.sh/orphaned: "true"
  annotations:
    lynq.sh/orphaned-at: "2025-01-15T10:30:00Z"
    lynq.sh/orphaned-reason: "RemovedFromTemplate"
    lynq.sh/deletion-policy: "Retain"
  # No ownerReferences — never set for Retain resources
```

**After re-adoption:**

```yaml
metadata:
  labels:
    lynq.sh/node: "acme-customer-web-app"
    lynq.sh/node-namespace: "lynq-system"
    # lynq.sh/orphaned removed
  annotations:
    lynq.sh/deletion-policy: "Retain"
    # lynq.sh/orphaned-at removed
    # lynq.sh/orphaned-reason removed
```

```bash
# Verify re-adoption
kubectl get deployment acme-worker \
  -o jsonpath='{.metadata.labels.lynq\.sh/orphaned}'
# (empty output = re-adopted)
```

***

## Finalizers

Lynq uses finalizers to ensure proper resource cleanup before Kubernetes deletes objects.

| Resource | Finalizer |
|----------|-----------|
| LynqNode | `lynqnode.operator.lynq.sh/finalizer` |
| LynqHub | `lynq.sh/hub-finalizer` |

**LynqNode finalizer flow:**

1. User deletes LynqNode CR (or operator marks it for deletion)
2. Kubernetes sets `deletionTimestamp` but does NOT delete yet (finalizer present)
3. LynqNode controller runs `cleanupLynqNodeResources()`:
   * For each managed resource, checks `lynq.sh/deletion-policy` annotation
   * `Delete`: removes resource from cluster
   * `Retain`: removes tracking labels, adds orphan markers, leaves resource in place
4. Controller removes the finalizer
5. Kubernetes garbage collects the LynqNode CR

**LynqHub finalizer flow:**

1. Hub is deleted → finalizer prevents immediate GC
2. Hub controller deletes all LynqNode CRs owned by this hub
3. Each LynqNode runs its own finalizer cleanup (above)
4. Hub finalizer is removed

***

## Cross-Namespace Resources

Normally, resources are created in the same namespace as their LynqNode CR. Setting `targetNamespace` on a TResource places it in a different namespace.

```yaml
# In LynqForm
deployments:
  - id: app
    targetNamespace: "{{ .uid }}-ns"   # different namespace
    nameTemplate: "{{ .uid }}-app"
    spec:
      # ...
```

**Tracking difference:** Kubernetes `ownerReferences` only work within a single namespace. For cross-namespace resources (and for Namespace resources, which are cluster-scoped), the operator uses label-based tracking (`lynq.sh/node` + `lynq.sh/node-namespace`) instead.

**Reconciliation trigger:** The operator watches labeled resources across all namespaces. Any change to a resource bearing `lynq.sh/node` triggers reconciliation of the owning LynqNode.

**Deletion:** On LynqNode deletion, the operator queries for all resources with `lynq.sh/node=<name>` across all namespaces and deletes or retains them per their `lynq.sh/deletion-policy` annotation.

***

## Watch Predicates

The LynqNode controller uses two watch mechanisms:

| Mechanism | Scope | Trigger condition |
|-----------|-------|-------------------|
| `Owns()` | Same namespace | Any generation or annotation change |
| `Watches()` (label selector) | All namespaces | Generation or annotation change on labeled resources |

**Smart filtering:** Both watches use a predicate that fires on (a) `metadata.generation` changes, (b) annotation changes outside the `lynq.sh/*` namespace, and (c) targeted status fields needed for real-time readiness propagation — `availableReplicas` / `readyReplicas` / conditions on Deployments, StatefulSets, DaemonSets, Jobs, the load-balancer status on Ingresses, and `currentReplicas` / `desiredReplicas` on HPAs. Updates that don't touch any of those (pure `resourceVersion` bumps, internal `lynq.sh/*` annotation rewrites, status fields on resource types not enumerated above) are filtered.

**Requeue interval:** In addition to event-driven reconciliation, LynqNodes are requeued every 30 seconds to pick up status changes in child resources (e.g., a Deployment becoming ready).

***

## appliedResources Format

`LynqNode.status.appliedResources` tracks all successfully applied resources for orphan detection.

Format per entry: `Kind/Namespace/Name@id`

Examples:

```
Deployment/default/acme-app@deploy-main
Service/default/acme-svc@svc-main
Namespace/acme-ns@ns-main
```

During reconciliation, the controller compares this list against the current template. Entries present in the status but absent from the template are treated as orphans.

## Template Evolution

Lynq handles template changes at runtime. Resources are added, updated, or removed automatically on the next reconcile.

### Adding Resources

New resources are created for all existing LynqNodes using this template:

::: v-pre

```yaml
# Add a new service — created for all existing nodes on next sync
services:
  - id: api-service
    nameTemplate: "{{ .uid }}-api"
    spec:
      apiVersion: v1
      kind: Service
      # ...
```

:::

### Modifying Resources

Resources are updated according to their `patchStrategy` (default: SSA — only managed fields are changed).

### Removing Resources

When a resource is removed from the template, Lynq checks its `deletionPolicy`:

* `Delete` (default): resource is deleted from the cluster
* `Retain`: resource is kept with orphan markers added

::: v-pre

```yaml
# Before: worker and cache exist alongside web
deployments:
  - id: web
    deletionPolicy: Delete
  - id: worker
    deletionPolicy: Retain
  - id: cache
    deletionPolicy: Delete

# After: only web in template
deployments:
  - id: web
    deletionPolicy: Delete
```

:::

**Result:**

* `worker` — retained with orphan labels
* `cache` — deleted from cluster
* `web` — continues managed normally

### Re-adopting Orphaned Resources

When you re-add a previously removed resource back to the template, Lynq automatically removes orphan markers and restores management.

::: v-pre

```bash
# Confirm re-adoption
kubectl get deployment acme-worker -o jsonpath='{.metadata.labels.lynq\.sh/orphaned}'
# (empty output = successfully re-adopted)
```

:::

### Best Practices for Template Changes

* Test in non-production first to validate resource cleanup behavior.
* Use `deletionPolicy: Retain` for stateful resources (PVCs, databases) before removing them from templates.
* Use `creationPolicy: Once` for init resources (one-time Jobs, seed ConfigMaps).

::: v-pre

```bash
# Check current tracked resources before changing a template
kubectl get lynqnode <name> -o jsonpath='{.status.appliedResources}' | jq .
```

:::

## See Also

* [API Reference](api.md) — CRD schema, field types, validation rules
* [Policies](policies.md) — `deletionPolicy`, `creationPolicy`, `conflictPolicy`
* [Templates](templates.md) — Using labels and annotations in templates
* [Templates Troubleshooting](templates-troubleshooting.md) — Debug template errors and rendering issues
