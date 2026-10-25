# Lynq Operator - Development Guidelines

---

## ⚠️ **CRITICAL: Project Renaming in Progress**

**This project is being renamed from "tenant-operator" to "lynq".**

**MANDATORY ACTION:** If you discover any naming inconsistencies with the rules below, you MUST immediately correct them.

### Required Name Conversions

| Old Name | New Name |
|----------|----------|
| `tenant-operator` | `lynq` |
| `tenant-operator-system` | `lynq-system` |
| `https://raw.githubusercontent.com/kubernetes-tenants/tenant-operator` | `https://raw.githubusercontent.com/k8s-lynq/lynq` |
| `ghcr.io/k8s-lynq/lynq-operator` | `ghcr.io/k8s-lynq/lynq` |
| `https://github.com/kubernetes-tenants/tenant-operator` | `https://github.com/k8s-lynq/lynq` |
| `Tenant Operator` | `Lynq Operator` |
| `kubernetes-tenants.org` | `lynq.sh` |
| `ghcr.io/kubernetes-tenants/tenant-operator` | `ghcr.io/k8s-lynq/lynq` |
| `https://kubernetes-tenants.github.io/tenant-operator/` | `https://k8s-lynq.github.io/lynq/` |
| `*.kubernetes-tenants.org` (API groups) | `*.lynq.sh` |
| `https://docs.kubernetes-tenants.org/` | `https://lynq.sh/` |

### CRD Name Conversions

| Old CRD Name | New CRD Name |
|--------------|--------------|
| `TenantRegistry` | `LynqHub` |
| `TenantTemplate` | `LynqForm` |
| `Tenant` | `LynqNode` |

### Controller Name Conversions

| Old Controller Name | New Controller Name |
|---------------------|---------------------|
| Registry Controller | LynqHub Controller |
| Template Controller | LynqForm Controller |
| Tenant Controller | LynqNode Controller |

**Note:** Generic technical terms like "template-registry linkage" (describing the relationship between resources) do NOT need conversion.

---

This document provides essential context and guidelines for Claude when working on the Lynq Operator project.

## Project Overview

**Lynq Operator** is a Kubernetes operator that automates database-driven resource provisioning using a template-based approach. It reads node data from external datasources (initially MySQL) and creates/synchronizes Kubernetes resources declaratively using Server-Side Apply (SSA).

### Core Objectives

1. **Database-driven Auto-provisioning**: Read node lists from external datasources and create/sync Kubernetes resources using templates
2. **K8s-native Operations**: 3 CRDs (LynqHub, LynqForm, LynqNode) with SSA-centric declarative synchronization
3. **Strong Consistency**: Ensure `desired count = (referencing templates * active rows)`, supporting multiple templates per hub
4. **Policy-based Lifecycle**: Creation/deletion/conflict policies, dependency graph-based apply ordering, failure isolation

---

## Architecture Overview

### Three-Controller Design

```
LynqHub Controller -> Reads external DB -> Emits LynqNode CRs
LynqForm Controller -> Ensures template-registry linkage
LynqNode Controller -> Reconciles each LynqNode -> SSA applies resources
```

### Key Components

- **LynqHub**: Defines external datasource (MySQL), sync interval, and value mappings
- **LynqForm**: Blueprint for resources to create per node (Deployments, Services, Ingresses, etc.)
- **LynqNode**: Instance representing a single node row with status tracking
- **SSA Engine**: Server-Side Apply with fieldManager: `lynq`

---

## CRD Responsibilities

### LynqHub

**Purpose**: Periodically read rows from external datasource and determine the active node set, then create/sync LynqNode CRs for each template-row combination.

**Multi-Template Support** ✅:
- **One hub can be referenced by multiple LynqForms**
- For each active row, a separate LynqNode CR is created for each referencing template
- LynqNode naming: `{uid}-{template-name}` format
- Example: Hub "mysql-prod" with 3 rows and 2 templates creates 6 LynqNode CRs

**Key Points**:
- Syncs at `spec.source.syncInterval`
- Only rows where `activate` field is truthy are considered active
- `status.referencingTemplates` = count of LynqForms referencing this hub
- `status.desired` = `referencingTemplates * activeRows` (total LynqNode CRs that should exist)
- `status.ready` = count of Ready LynqNodes across all templates
- `status.failed` = count of failed LynqNodes across all templates: failed or conflicted
  resources, or `Degraded=True`. A node that is rolling out or recovering lost pods is in
  neither `ready` nor `failed` (see "One LynqNode classification" in Common Pitfalls)
- **Garbage Collection** ✅: Automatic LynqNode CR cleanup when:
  - Database row is deleted
  - Row's `activate` field becomes false
  - Template no longer references this hub
  - Detailed events emitted: `LynqNodeDeleting`, `LynqNodeDeleted`, `LynqNodeDeletionFailed`
  - LynqNode finalizers ensure proper resource cleanup before deletion

**Required Value Mappings**:
- `uid`: Node identifier (required)
- `activate`: Activation flag (required)

**Deprecated Value Mappings** (since v1.1.11, removed in v1.3.0):
- `hostOrUrl`: Node URL/host (deprecated, use `extraValueMappings` + `toHost()` function instead)
  - Previously auto-extracted `.host` variable
  - Migration: Use `extraValueMappings` to map column to custom variable, then use `toHost()` in templates

**Extra Value Mappings**: Arbitrary column-to-variable mappings for templates (recommended approach)

### LynqForm

**Purpose**: Defines the resource blueprint for a specific Hub. Each active node row uses this template to generate Kubernetes resources.

**Key Points**:
- References a `hubId`
- Contains arrays of resource types: `serviceAccounts`, `deployments`, `statefulSets`, `daemonSets`, `services`, `ingresses`, `configMaps`, `secrets`, `persistentVolumeClaims`, `jobs`, `cronJobs`, `podDisruptionBudgets`, `networkPolicies`, `horizontalPodAutoscalers`, `manifests` (raw)
- Each resource follows `TResource` structure (see api/v1/common_types.go:58)
- `spec.rollout.maxSkew` throttles how many LynqNodes are updating at once; a node that is not
  Ready (failed or recovering included) keeps its slot. `spec.rollout.progressDeadlineSeconds`
  reports a stall: `status.rollout.phase=Failed` when an updated node is failed, or has not
  become Ready since its update for longer than the deadline. Only a LynqForm rollout (an update
  that changes the node's template generation) is timed; a data-only update is caught by the
  LynqNode turning failed. The node keeps its slot — the rollout stops and says so, it does not
  skip the node
- `status.rollout` counts and phase follow the completion marker, not current readiness: an
  updated node that completed its LynqForm rollout and regressed since is *recovering* — it keeps
  the rollout `Complete` (same `completionTime`) and is named only in the message. maxSkew still
  counts it as updating (see the progressDeadlineSeconds pitfall)
- All `*Template` fields support Go `text/template` + sprig functions

**Template Variables Available**:
- Required: `.uid`, `.activate`
- Deprecated (v1.1.11, removed in v1.3.0): `.hostOrUrl`, `.host`
- Extra: Any keys from `extraValueMappings` (recommended for all custom fields including URLs/hosts)
- Context: `.hubId`, `.templateRef`, etc.

**Custom Template Functions** (All Implemented ✅):
- Standard: All sprig functions (200+ from Sprig library)
- Custom (Implemented):
  - `toHost(url)` ✅ - Extract host from URL
  - `trunc63(s)` ✅ - Truncate to 63 chars (K8s name limit)
  - `sha1sum(s)` ✅ - SHA1 hash (Priority 1 implementation)
  - `fromJson(s)` ✅ - Parse JSON string to object (Priority 1 implementation)
  - Plus all sprig functions: `default`, `b64enc`, `b64dec`, `toJson`, `sha256sum`, etc.

### LynqNode

**Purpose**: Represents a single node instance. The operator creates/syncs all Kubernetes resources for this node.

**Key Points**:
- Created automatically by LynqHub controller
- Contains resolved resource arrays (templates already evaluated)
- Status tracks `readyResources`, `desiredResources`, `failedResources`
- Status tracks `appliedResources` for orphan detection (format: `kind/namespace/name@id`)
- `Ready` condition requires ALL resources to be ready
- Users typically don't edit LynqNode specs directly (managed by operator)
- Supports dynamic template evolution with automatic orphan cleanup

---

## Common Types (api/v1/common_types.go)

### TResource Structure

Every resource in a template is a `TResource` with:

```go
type TResource struct {
    ID                      string              // Unique within template (for dependencies)
    Spec                    runtime.RawExtension // K8s resource spec
    DependIds               []string            // IDs of resources to wait for
    SkipOnDependencyFailure *bool               // Skip this resource if dependency fails (default: true)
    CreationPolicy          CreationPolicy      // Once | WhenNeeded (default)
    DeletionPolicy          DeletionPolicy      // Delete (default) | Retain
    ConflictPolicy          ConflictPolicy      // Stuck (default) | Force
    NameTemplate            string              // Go template for name
    TargetNamespace         string              // Target namespace for cross-namespace resources
    LabelsTemplate          map[string]string   // Template-enabled labels
    AnnotationsTemplate     map[string]string   // Template-enabled annotations
    WaitForReady            *bool               // Default: true
    TimeoutSeconds          int32               // Default: 300, max: 3600
    PatchStrategy           PatchStrategy       // apply (default) | merge | replace
}
```

### Policies

**CreationPolicy**:
- `Once`: Create only once, never reapply even if spec changes (for init Jobs, security resources)
- `WhenNeeded` (default): Reapply when spec changes or state requires it

**DeletionPolicy**:
- `Delete` (default): Remove resource when LynqNode is deleted
- `Retain`: Remove ownerReference but leave resource in cluster

**ConflictPolicy**:
- `Stuck` (default): Stop reconciliation if resource exists with different owner, mark LynqNode as Degraded
- `Force`: Use SSA with `force=true` to take ownership, fail gracefully if unsuccessful

**PatchStrategy** ✅ (All Implemented):
- `apply` (default): SSA (Server-Side Apply) with conflict management
- `merge` ✅: Strategic merge patch (Priority 2 implementation)
- `replace` ✅: Full replacement via Update (Priority 2 implementation)
  - Handles create-if-not-exists
  - Preserves resourceVersion for conflict-free updates

**SkipOnDependencyFailure** ✅ (Implemented):
- `true` (default): Skip creating this resource if any of its dependencies failed
  - Emits `DependencySkipped` event with details about which dependency failed
  - Resource is tracked in `status.skippedResources` and `status.skippedResourceIds`
  - Dependent resources are also marked as failed to cascade the skip behavior
- `false`: Create this resource even if dependencies failed
  - Useful for cleanup resources, logging, or notification resources that should run regardless
  - Emits `DependencyFailedButProceeding` warning event
  - Example use case: ConfigMap for debugging that captures state even when main deployment fails

**Dependency Blocking vs Failure** (Important distinction):
- **Failed dependencies**: Resources that encountered apply errors OR timed out. Triggers `skipOnDependencyFailure` logic.
- **Not-ready dependencies** (within timeout): Resources still progressing (e.g., Deployment starting pods).
  - Blocks dependents silently (no event, no skip count)
  - Does NOT trigger `skipOnDependencyFailure` logic
  - Dependents will be applied on next reconcile when dependency becomes ready
- **Timed-out dependencies**: Resources that never became Ready within `timeoutSeconds`
  (default: 300s) of Lynq applying them.
  - Transitions to **failed** state
  - Emits `ReadinessTimeout` event
  - Triggers `skipOnDependencyFailure` logic for dependents
- **Regressed dependencies**: Resources that became Ready after Lynq last started a rollout of
  them and later dropped out of it (lost pod, node drain, external `kubectl rollout restart`).
  - Treated as not-ready, never timed out — the workload controller is already recovering them
  - The only failure verdict is the workload controller's own: a Deployment whose
    `Progressing` condition is `False/ProgressDeadlineExceeded` is failed
  - Does NOT block dependents: it already satisfied their ordering under the current apply, and
    blocking would freeze their spec updates and drift correction for the whole recovery
  - An apply of ours that changes the resource's live `.spec` (including a force-reapply that
    reverts drift) starts a new rollout: the resource is not-ready again, blocks dependents,
    and is timed from that apply. So does one whose pre-apply read was a stale cache that the
    write's generation outran by two or more, since the cache may have hidden a `.spec` change
  - An apply of ours that does not change it (a metadata-only template change: labels,
    annotations) starts no rollout and leaves a regression a regression, even though it
    re-stamps `lynq.sh/apply-start-time` and, on a Deployment, moves `metadata.generation`
- This distinction ensures Deployments starting up or recovering pods don't cause misleading
  "DependencySkipped" events

**LynqNode Status Fields for Dependency Tracking**:
- `status.skippedResources`: Count of resources skipped due to dependency failures
- `status.skippedResourceIds`: List of resource IDs skipped by the latest reconcile (cleared when
  nothing is skipped)
- Visible in `kubectl get lynqnodes` via the "Skipped" column

### Cross-Namespace Resource Support ✅

**Feature**: Resources can be created in different namespaces from the LynqNode CR using `targetNamespace` field.

**Key Implementation Details**:
- **Same-Namespace Resources**: Use traditional `ownerReferences` for automatic garbage collection
- **Cross-Namespace Resources**: Use label-based tracking instead (since ownerReferences don't work across namespaces)
- **Tracking Labels**:
  - `lynq.sh/node`: LynqNode CR name
  - `lynq.sh/node-namespace`: LynqNode CR namespace
- **Automatic Detection**: Operator automatically detects cross-namespace resources and applies appropriate tracking method
- **Namespace Resources**: Always use label-based tracking (cannot have ownerReferences)

**Usage**:
```yaml
spec:
  targetNamespace: "{{ .uid }}-namespace"  # Templates supported
```

**Reconciliation Behavior**:
- **Creation**: Resources created in specified namespace with tracking labels
- **Updates**: Same-namespace and cross-namespace resources both tracked for drift
- **Deletion**: Label-based cleanup for cross-namespace resources
  - `DeletionPolicy: Delete` - removes resource from target namespace
  - `DeletionPolicy: Retain` - removes tracking labels but keeps resource

**Watch Predicates**:
- Dual tracking: Both `Owns()` (for same-namespace) and `Watches()` (for label-based) are configured
- Cross-namespace resource changes trigger LynqNode reconciliation via label selectors
- Smart predicates reduce unnecessary reconciliations (only on Generation/Annotation changes)

**RBAC Requirements**:
- Operator requires cluster-wide permissions for resource types
- Default RBAC rules support cross-namespace resource provisioning
- Resources can be created in any namespace when `targetNamespace` is specified

**Example Use Cases**:
1. **Multi-Namespace Isolation**: Create node resources across multiple namespaces for better isolation
2. **Shared Infrastructure**: Deploy node-specific resources into shared infrastructure namespaces
3. **Dynamic Namespace Creation**: Create namespace per node, then populate it with resources
4. **Organizational Boundaries**: Align resource placement with organizational namespace structure

---

## Reconciliation Logic

### LynqHub Controller Flow

1. Get all LynqForms that reference this hub
2. Query datasource at `syncInterval`
   - The datasource is held across syncs by `datasource.Pool`, keyed by hub. It is rebuilt
     only when a connection setting changes (host, port, user, password, database, pool
     sizing) and released when the hub is deleted or disappears.
   - Each query carries a deadline from `queryTimeoutFor` — see Common Pitfalls for why
     removing it would let an unreachable database go unnoticed.
3. Filter rows where `activate=true`
4. Calculate desired LynqNode set (all template-row combinations)
   - For each template and each row, create key: `{template-name}-{uid}`
   - Desired count = `len(templates) * len(activeRows)`
5. Create missing LynqNodes (naming: `{uid}-{template-name}`), update existing, delete excess
   - Gated by `maxSkew` when the LynqForm configures a rollout. The "currently updating"
     count is read through `APIReader`, not the cache, and memoized per template per
     reconcile; `updatedInThisIteration` covers updates made later in the same pass. A
     failed read denies admission rather than admitting blindly.
   - Only consulted when a node actually needs creating or updating, so a steady-state sync
     performs no uncached reads at all.
   - A node that needs no update but completed its current one gets its
     `lynq.sh/rollout-completed-generation` recorded (once per LynqForm rollout; see the
     progressDeadlineSeconds pitfall).
6. Update `status.{referencingTemplates, desired, ready, failed}`
   - `ready`/`failed` come from `classifyNode`, the same classification the maxSkew count and
     the LynqForm rollout status use.
   - `updateStatus` skips the write when the status is unchanged. It must stay that way: a
     no-op write here is a LynqHub watch event, which is another reconcile.

### LynqNode Controller Flow ✅

1. **Handle Deletion with Finalizer** ✅ (Implemented):
   - Check if `DeletionTimestamp` is set
   - If finalizer present: Run `cleanupLynqNodeResources()`
     - Respect `DeletionPolicy` per resource:
       - `Delete`: Remove resource from cluster
       - `Retain`: Remove ownerReference, keep resource
   - Remove finalizer after cleanup
   - Return (allow Kubernetes to delete LynqNode CR)

2. **Add Finalizer if Missing** ✅ (Implemented):
   - Check if finalizer `lynqnode.operator.lynq.sh/finalizer` exists
   - Add finalizer if missing
   - Update LynqNode CR
   - Requeue for reconciliation

3. **Build Template Variables**:
   - Extract variables from LynqNode annotations
   - Merge with hub data

4. **Resolve Dependencies**:
   - Build DAG from `dependIds`
   - Detect cycles (fail fast if found)

5. **Topological Sort**:
   - Determine apply order based on dependency graph

6. **Orphan Resource Cleanup** ✅ (Implemented):
   - Detect resources that were previously applied but removed from template
   - Compare `status.AppliedResources` (previous) with current desired resources
   - Resource key format: `kind/namespace/name@id` (e.g., `Deployment/default/myapp@app-deployment`)
   - **DeletionPolicy Preservation**: Each resource stores its DeletionPolicy in annotation `lynq.sh/deletion-policy` at creation time
     - Critical: Orphaned resources no longer exist in template, so annotation is the only source of truth
     - Enables correct cleanup behavior even after template changes
   - For each orphaned resource:
     - Read `DeletionPolicy` from resource annotation (falls back to `Delete` if missing)
     - Respect `DeletionPolicy`:
       - `Delete`: Remove resource from cluster (automatic via ownerReference)
       - `Retain`: Remove tracking labels, add orphan labels, keep resource (no ownerReference)
     - Log deletion/retention event with reason "RemovedFromTemplate"
   - **Orphan Markers** (for retained resources):
     - Label: `lynq.sh/orphaned: "true"` (for selector queries)
     - Annotation: `lynq.sh/orphaned-at: "<RFC3339 timestamp>"`
     - Annotation: `lynq.sh/orphaned-reason: "RemovedFromTemplate" | "LynqNodeDeleted"`
   - **Re-adoption**: When a previously orphaned resource is re-added to template:
     - Operator automatically removes all orphan markers during apply
     - Resource transitions cleanly back to managed state
     - No manual cleanup required
   - Runs before applying new resources to prevent conflicts
   - Enables dynamic template evolution without manual cleanup
   - Easy identification of retained orphan resources via label selectors

7. **For Each Resource in Order**:
   - **Check Dependency Failures** ✅ (Implemented):
     - If any dependency failed/timed out, check `skipOnDependencyFailure` flag
     - `true` (default): Skip this resource, emit `DependencySkipped` event, track in status
     - `false`: Proceed with creation despite failed dependency, emit `DependencyFailedButProceeding` event
   - **Check CreationPolicy** ✅:
     - `Once`: Skip if already created (check annotation `lynq.sh/created-once`)
     - `WhenNeeded` (default): Proceed with apply
   - Evaluate `nameTemplate` (namespace is automatically set to LynqNode CR's namespace)
   - Render resource `spec` with template variables
   - **Apply ConflictPolicy**:
     - `Stuck`: Check ownership, fail if conflict
     - `Force`: SSA with `force=true`
   - **Apply using Selected PatchStrategy** ✅:
     - `apply` (default): Server-Side Apply
     - `merge`: Strategic Merge Patch
     - `replace`: Get → Update with resourceVersion
   - If `waitForReady=true`: Wait for resource Ready condition (with timeout)
   - Track success/failure with metrics

8. **Update LynqNode Status**:
   - Aggregate resource states
   - Update `readyResources`, `failedResources`, `desiredResources`
   - Update `skippedResources`, `skippedResourceIds` for dependency-skipped resources
   - Update `appliedResources` with successfully applied resource keys
   - Set `Ready` condition

9. **Requeue for Fast Status Reflection** ✅ (Implemented - Optimized):
   - Return with `RequeueAfter: 30 * time.Second` (changed from 5 minutes)
   - Ensures rapid detection of child resource status changes
   - Combined with event-driven watches for immediate reaction to changes

**Location**: `internal/controller/lynqnode_controller.go`

### Synchronization Rules

1. **Desired Set Calculation**: Only `activate=true` rows from Hub, multiplied by referencing templates
   - Key: `{template-name}-{uid}` for each template-row combination
   - Total desired = `referencingTemplates * activeRows`
2. **Creation/Deletion**:
   - `desired \ current` -> Create new LynqNodes (naming: `{uid}-{template-name}`)
   - `current \ desired` -> Delete LynqNodes (respect `deletionPolicy`)
3. **Drift Detection** ✅ (two channels):
   - **Watch-driven (immediate)**: `Owns()` watches on 11 resource types + `Watches()` on
     Namespaces fire when a child resource's `metadata.generation` changes.
     - ServiceAccounts, Deployments, StatefulSets, DaemonSets
     - Services, ConfigMaps, Secrets, PersistentVolumeClaims
     - Jobs, CronJobs, Ingresses
     - **Namespaces**: Tracked via labels (cannot use ownerReferences)
   - **Watch Predicates**: Only trigger on Generation/Annotation changes (not status-only
     updates). Reduces unnecessary reconciliation overhead. The watch correctly fires on
     external edits that change spec; `shouldSkipApply` then sees `applied-hash` either
     stripped or stale and re-applies. If the external editor preserved `applied-hash`,
     drift correction defers to the second channel.
   - **Periodic force-reapply (~10 min, configurable)**: every `ForceReapplyInterval`,
     the LynqNode controller bypasses `shouldSkipApply` and re-applies every child
     resource unconditionally. Tracked via `LynqNode.Status.LastFullReconcileAt`. This is
     the drift-correction backstop for external mutations that preserve the
     `applied-hash` annotation. **Nil-handling**: nil `LastFullReconcileAt` is treated as
     "stamp-now, defer first force by one full interval" to prevent a re-apply storm on
     controller restart.
   - **Fast Requeue**: 30-second periodic requeue for status reflection (independent
     of the 10-minute force-reapply cycle).
   - **Location**: `internal/controller/lynqnode_controller.go` (`reconcileSpec`,
     `SetupWithManager`); `internal/apply/ssa.go` (`shouldSkipApply`).
4. **Naming/Scope**: Use `nameTemplate` (63-char limit via `trunc63`). All resources are created in the same namespace as the LynqNode CR.
5. **Ordering**: Topological sort by `dependIds`, `waitForReady` enforces readiness gates

---

## Resource Readiness Rules

**When to mark a resource as Ready**:

- **Deployment**: `status.observedGeneration == metadata.generation` AND `availableReplicas >= spec.replicas`
- **StatefulSet**: `readyReplicas == spec.replicas`
- **Service**: Immediate (or `waitForReady=false` recommended)
- **Ingress**: `status.loadBalancer.ingress` exists OR controller-specific Ready condition
- **Job**: `status.succeeded >= 1`
- **ServiceAccount**: Immediate after creation
- **ConfigMap/Secret**: Immediate after creation
- **Custom Resources**: `status.conditions[type=Ready].status=True` or custom health checks

**Timeout**: Each resource has `timeoutSeconds` (default 300s), exceeding marks resource as failed.

---

## Template System

### Template Language

- **Engine**: Go `text/template` + sprig library
- **Syntax**: `{{ .variable }}`, `{{ function arg }}`, `{{ if .condition }}...{{ end }}`

### Available Variables

**Always available**:
- `.uid`: Node unique identifier
- `.hostOrUrl`: Original URL/host from hub
- `.host`: Auto-extracted host (from `.hostOrUrl`)
- `.activate`: Activation status
- `.hubId`: Hub name (LynqHub name)
- `.templateRef`: Template name

**From extraValueMappings**: Any custom mappings (e.g., `.deployImage`, `.planId`)

### Template Functions

**Sprig functions**: `default`, `trim`, `upper`, `lower`, `b64enc`, `b64dec`, `sha256sum`, etc.

**Custom functions**:
- `toHost(url)`: Extract host from URL
- `trunc63(s)`: Truncate to 63 chars (K8s name limit)
- `sha1sum(s)`: SHA1 hash
- `fromJson(s)`, `toJson(obj)`: JSON serialization

### Template Examples

```yaml
nameTemplate: "{{ .uid }}-api"
nameTemplate: "{{ .uid | trunc63 }}"
nameTemplate: "{{ .uid }}-{{ .planId | default \"basic\" }}"

# In Deployment spec:
image: "{{ default \"nginx:stable\" .deployImage }}"
value: "{{ .host }}"
value: "{{ .uid }}"
```

---

## Security & RBAC

### Credentials

- **SecretRef pattern**: All sensitive data (passwords, tokens) use `SecretRef`
- Example: `spec.source.mysql.passwordRef.{name, key}`
- Never hardcode credentials in CRDs

### OwnerReferences and Resource Tracking

**Two tracking mechanisms based on DeletionPolicy and namespace:**

1. **OwnerReference-based tracking** (automatic garbage collection):
   - Used for: Same-namespace resources with `DeletionPolicy=Delete` (default)
   - Behavior: Kubernetes garbage collector automatically deletes resources when LynqNode is deleted
   - Location: `ownerReferences` field in resource metadata

2. **Label-based tracking** (manual lifecycle management):
   - Used for:
     - Cross-namespace resources (ownerReferences don't work across namespaces)
     - Namespace resources (cannot have ownerReferences)
     - **Resources with `DeletionPolicy=Retain`** (prevents automatic deletion)
   - Tracking labels:
     - `lynq.sh/node`: LynqNode CR name
     - `lynq.sh/node-namespace`: LynqNode CR namespace
   - Behavior: Resources persist after LynqNode deletion, operator manages lifecycle via finalizer

**DeletionPolicy=Retain Implementation** ✅:
- **Creation**: Resources created with label-based tracking only (NO ownerReference)
- **Reason**: Prevents Kubernetes garbage collector from auto-deleting when LynqNode is removed
- **Deletion**: Finalizer removes tracking labels and adds orphan labels, resource persists in cluster
- **Orphan Markers**:
  - Label: `lynq.sh/orphaned: "true"` (for selector queries)
  - Annotation: `lynq.sh/orphaned-at: "<RFC3339 timestamp>"`
  - Annotation: `lynq.sh/orphaned-reason: "LynqNodeDeleted" | "RemovedFromTemplate"`

**Critical Design Decision**: DeletionPolicy must be evaluated at resource creation time, not deletion time. Setting ownerReference initially and trying to remove it during deletion is too late - Kubernetes garbage collector has already marked the resource for deletion.

### RBAC Requirements

**For operator ServiceAccount**:
- CRDs: `lynqhubs`, `lynqforms`, `lynqnodes` (all verbs)
- Workload resources: Required native resources (Deployments, Services, etc.) - namespace-scoped permissions
- `events`, `leases` for leader election
- `secrets` (read-only) for hub credentials

**Principle**: Least privilege, namespace-scoped permissions. All node resources are created in the same namespace as the LynqNode CR.

---

## Observability

### Events

Emit events for:
- Conflict detected (resource exists with different owner)
- Force apply attempted
- Resource timeout
- Template rendering error
- LynqNode Ready transition
- Dependency cycle detected
- **DependencySkipped** ✅: Resource skipped because a dependency failed (includes failed dependency ID)
- **DependencyFailedButProceeding** ✅: Resource created despite dependency failure (when `skipOnDependencyFailure=false`)

### Metrics (Prometheus)

```
lynqnode_reconcile_duration_seconds{result}
lynqnode_resources_ready{lynqnode, namespace}
lynqnode_resources_desired{lynqnode, namespace}
lynqnode_resources_failed{lynqnode, namespace}
lynqnode_condition_status{lynqnode, namespace, type}  # 0=False, 1=True, 2=Unknown
lynqnode_conflicts_total{lynqnode, namespace, resource_kind, conflict_policy}
lynqnode_resources_conflicted{lynqnode, namespace}
lynqnode_degraded_status{lynqnode, namespace, reason}  # 0=not degraded, 1=degraded
hub_desired{hub, namespace}
hub_ready{hub, namespace}
hub_failed{hub, namespace}
apply_attempts_total{kind, result, conflict_policy}
```

**Key Metrics for Monitoring Conflicts and Failures:**

- `lynqnode_conflicts_total`: Counter tracking total conflicts encountered
- `lynqnode_resources_conflicted`: Current number of resources in conflict state
- `lynqnode_degraded_status`: Binary indicator (1=degraded, 0=healthy) with reason label. Only
  the current reason is 1; every other known reason is reset to 0 on each update, so a reason
  change never leaves a stale series firing
- `lynqnode_condition_status`: Status of node conditions (Ready, Degraded, etc.)

**Alert Rules:** Available at `config/prometheus/alerts.yaml` with comprehensive rules for:
- Critical: Failed resources, degraded nodes, not ready nodes
- Warning: Conflicts, high conflict rate, resource mismatches
- Info: New conflicts detected

### Logging

- Template rendering input snapshots (mask sensitive data)
- SSA diff summaries
- Reconciliation start/end with duration
- Error details with context

---

## Validation & Webhooks

### ValidatingWebhook

- `LynqForm.spec.hubId` must reference existing LynqHub
- `valueMappings` must include required keys: `uid`, `hostOrUrl`, `activate`
- `TResource.id` must be unique within template
- `dependIds` must not form cycles
- Templates must be valid Go templates

### DefaultingWebhook

Set defaults:
- `creationPolicy=WhenNeeded`
- `deletionPolicy=Delete`
- `conflictPolicy=Stuck`
- `waitForReady=true`
- `timeoutSeconds=300`
- `patchStrategy=apply`

### OpenAPI Schema

All CRDs have comprehensive OpenAPI v3 schemas with:
- Required fields marked
- Enums for policy types
- Patterns for intervals (e.g., `^\d+(s|m|h)$`)
- Min/max constraints

---

## Failure Handling

### Template Rendering Failure

- Mark LynqNode as Degraded
- Emit event with error details (missing variable, type error)
- Do not apply any resources

### Conflict (Stuck Policy)

- Stop reconciliation for that resource
- Emit event: "Resource {name} exists with different owner"
- Mark LynqNode as Degraded
- Provide hints: Check namespace uniqueness, review naming templates

### Ready Timeout

- A resource that never becomes Ready within `timeoutSeconds` of being applied is failed, which
  marks the LynqNode as Degraded
- A previously Ready resource that regresses is not-ready only (see Dependency Blocking vs Failure)
- Emit event with resource status details

### Dependency Cycle

- Detect during DAG construction
- Mark LynqNode as Degraded immediately
- Emit event: "Dependency cycle detected: A -> B -> A"

---

## Performance & Scalability

### Controller Design

- Separate workqueues for LynqHub/LynqForm/LynqNode controllers
- Rate-limited retries with exponential backoff
- Concurrent reconciliation flags:
  - `--hub-concurrency=N` (default: 3)
  - `--form-concurrency=N` (default: 5)
  - `--node-concurrency=N` (default: 10)
- Leader election flags (`cmd/manager_options.go`), defaults are controller-runtime's:
  - `--leader-elect-lease-duration` (15s), `--leader-elect-renew-deadline` (10s),
    `--leader-elect-retry-period` (2s); validated at startup
    (`lease-duration > renew-deadline > 1.2 × retry-period`)
  - `--leader-elect-release-on-cancel` (default: false)
- `--kube-api-warnings=deduplicate|all` (default: deduplicate) — see Common Pitfalls

### Reconciliation Optimization ✅

- **Fast Status Reflection**: 30-second requeue interval (optimized from 5 minutes)
  - Child resource status changes reflected in LynqNode status within 30 seconds
  - Balances responsiveness with cluster resource usage
- **Smart Watch Predicates**: Only reconcile on meaningful changes
  - Generation changes (spec updates)
  - Annotation changes
  - Filters out status-only updates to reduce reconciliation overhead
- **Namespace Tracking**: Label-based tracking for Namespaces
  - Labels: `lynq.sh/node`, `lynq.sh/node-namespace`
  - Enables immediate reconciliation when Namespaces are modified
- **Event-Driven Architecture**: Immediate reconciliation on watched resource changes

### Large-scale Optimization

- SSA batching for bulk applies
- Resource-type worker pools
- Optional sharding: `--shard=N/M` or namespace partitioning
- Cache frequently accessed resources (registries, templates)

---

## Development Guidelines

### Code Structure

```
api/v1/               # CRD types (already includes common_types.go)
internal/controller/  # Controller implementations
pkg/template/         # Template rendering engine
pkg/apply/            # SSA apply engine
pkg/health/           # Resource readiness checks
pkg/datasource/       # External datasource integrations
```

### Testing Strategy

- Unit tests: Template rendering, policy logic, dependency graph
- Integration tests: Controller reconciliation against real API server
- E2E tests: Full workflow with MySQL datasource

#### E2E Test Execution Guidelines

**IMPORTANT**: Always prefer focused test execution over running the full test suite to maximize development efficiency.

**Running Focused Tests (RECOMMENDED)**:
```bash
# Run specific test by description (substring match)
make test-e2e-focus FOCUS="should convert string values to proper integer types"

# Run tests matching a pattern
make test-e2e-focus FOCUS="Template Functions"

# Run multiple related tests
make test-e2e-focus FOCUS="metrics update"
```

**Running Full Test Suite (USE SPARINGLY)**:
```bash
# Only run when necessary (e.g., pre-commit, CI validation)
# Takes 30-45 minutes to complete
make test-e2e
```

**CI Groups**: `.github/workflows/test-e2e.yml` splits the suite into matrix groups by
focusing on top-level `Describe` names, and each group runs under a 30-minute `go test`
timeout. A new top-level `Describe` that matches no group's focus falls into `other`. When a
group's runtime approaches the timeout, move the slow scenarios to their own top-level
`Describe`, add a group that focuses on it, and add the same name to `other`'s skip list.
For example, `Readiness Regression` runs on its own because its gated-restart scenarios hold a
Deployment not ready for longer than one timeout plus one periodic requeue, and `Rollout Progress
Deadline` because each scenario waits out a 60s `progressDeadlineSeconds` plus a LynqForm requeue.

**Critical Rules**:
1. **Always use 60-minute timeout minimum**: E2E tests build Docker images, create Kind clusters, install cert-manager, and deploy the operator. This takes significant time.
   ```bash
   # Correct timeout setting in Bash tool
   timeout: 3600000  # 60 minutes in milliseconds
   ```

2. **NEVER use `grep` or `tail` to filter test output**: These commands buffer output and hide real-time progress, making it impossible to debug hanging tests. Always view full output or use Ginkgo's built-in filtering.
   ```bash
   # ❌ WRONG - hides progress and makes debugging impossible
   make test-e2e-focus FOCUS="..." 2>&1 | tail -100

   # ✅ CORRECT - shows full output with real-time progress
   make test-e2e-focus FOCUS="..."
   ```

3. **Clean up failed clusters**: If tests fail due to cluster issues (network timeouts, resource conflicts, cert-manager problems):
   ```bash
   # Delete the test cluster and retry
   kind delete cluster --name lynq-test-e2e

   # Then re-run your test
   make test-e2e-focus FOCUS="your test description"
   ```

4. **Understanding FOCUS parameter**:
   - Uses Ginkgo's focus filter (substring match on test descriptions)
   - Case-sensitive
   - Matches against the full test hierarchy: `Describe > Context > It`
   - Examples:
     - `FOCUS="Template Functions"` - runs all tests in "Template Functions" describe block
     - `FOCUS="should convert string"` - runs specific test containing this phrase
     - `FOCUS="Typed template functions"` - runs tests in that context

5. **Test debugging workflow**:
   ```bash
   # Step 1: Run the specific failing test
   make test-e2e-focus FOCUS="exact test description from failure"

   # Step 2: If cluster issues occur, clean up
   kind delete cluster --name lynq-test-e2e

   # Step 3: Re-run after cleanup
   make test-e2e-focus FOCUS="exact test description from failure"

   # Step 4: Check controller logs if test still fails
   kubectl logs -n lynq-system deployment/lynq-controller-manager --tail=100

   # Step 5: Inspect test resources
   kubectl get lynqnodes,lynqforms,lynqhubs -A
   kubectl describe lynqnode <node-name> -n <namespace>
   ```

6. **Performance expectations**:
   - Full test suite (`make test-e2e`): 30-45 minutes
   - Single focused test: 3-5 minutes (includes cluster setup)
   - Subsequent focused tests (cluster exists): 1-2 minutes

**Common Test Failures and Solutions**:

| Issue | Solution |
|-------|----------|
| "Kind cluster already exists" | Tests will reuse existing cluster (faster) |
| "cert-manager webhook timeout" | Cluster networking issue - run `kind delete cluster --name lynq-test-e2e` |
| "deployment not found" | Controller panic or rendering error - check logs |
| "timed out waiting for deployment" | Deployment may be scaled to 0 or image pull failed |
| "panic: cannot deep copy int" | Type conversion issue in templates - ensure int values are converted to int64 |

### Important Invariants

1. `LynqHub.status.desired` MUST equal active row count
2. `LynqNode` is Ready IFF all resources are Ready. It is Degraded IFF something needs an
   operator: failed or conflicted resources, or template/variables/dependency errors. Not-ready
   alone is `Ready=False` (`NotAllResourcesReady`) with `Degraded=False`.
3. SSA fieldManager MUST be `lynq`
4. Dependency cycles MUST be rejected
5. Naming MUST respect 63-char K8s limit
6. A reconcile that changes nothing MUST NOT write. Any write is a watch event, and a
   watch event is another reconcile.
7. The rollout-skew decision MUST be made on API-server state, never on the informer cache.

### Common Pitfalls

- Don't forget to auto-extract `.host` from `.hostOrUrl`
- Template rendering errors should never panic
- Always validate `dependIds` before topological sort
- SSA requires `fieldManager` and correct content-type
- `waitForReady=true` blocks the reconciliation pipeline
- **Readiness timeout measures "Lynq's change never became Ready" — nothing else.**
  - It is measured from `lynq.sh/apply-start-time`, NOT `creationTimestamp`. Using
    `creationTimestamp` causes pre-existing resources (months old) to immediately exceed the
    timeout after apply, marking them as FAILED instantly. This is the root cause of the maxSkew deadlock.
  - `lynq.sh/apply-start-time` is stamped on the child resource by the Applier when a changed spec
    is applied and persists across reconcile loops so the timeout clock does not reset every 30
    seconds. `applyStartTime` (local variable) is only a fallback for the very first reconcile
    before the annotation has been written.
  - Because the annotation only moves when Lynq applies a changed desired object, it is months old on any long-running
    resource. Measuring a later blip (lost pod, drain, external rollout restart) against it fails
    the resource the instant it is seen not ready — a false failure that skipped dependents,
    degraded the node and paged. `readinessTracker` (in-memory, keyed by LynqNode UID + resource
    ID) therefore records what this process has observed:
    - Observed Ready since Lynq last started a rollout of it, then not ready → a regression:
      never timed out (only `ProgressDeadlineExceeded` fails it), and it does not block
      dependents.
    - No such history (fresh apply, or a controller restart) → the clock starts at the later of
      apply-start-time and the first not-ready observation, so a restart grants one full
      `timeoutSeconds` of grace: detection may be delayed, never faked.
  - apply-start-time alone cannot tell "Lynq started a rollout" from "nothing changed": a
    force-reapply that reverts external drift preserves it, because the desired hash did not
    change. `ApplyResource` therefore reports `StartedRollout` — the pre-apply read's `.spec`
    against the write response's, no extra API call — and an apply that started one resets the
    resource's history (`startRollout`): earlier Ready observations stop counting and the clock
    starts no earlier than that apply. A no-op apply leaves the history alone. The tracker
    records the generation that write left the resource at, and a Ready read older than it is
    a stale cache read and does not count.
  - The pre-apply read comes from the informer cache, so the `.spec` comparison alone can miss
    a rollout: a force-reapply reverting drift the cache has not seen yet compares the old
    cached spec to an identical response. One write moves `metadata.generation` by at most one,
    so `StartedRollout` is also true when the response's generation is two or more past the
    cached read's (both non-zero) — the cache missed a write, which is assumed to be a rollout.
    The rule may only err toward re-timing (e.g. an unseen external annotation edit plus a
    metadata-only apply); missing a real rollout would exempt it from `timeoutSeconds` for
    good. Do not "fix" it with an uncached read or a resourceVersion precondition: that is an
    extra API read or a conflict-retry on every apply.
  - `metadata.generation` is NOT the rollout signal. The Deployment registry strategy also
    bumps it when `metadata.annotations` change (they are copied onto the ReplicaSet), and
    every apply that changes the desired hash re-stamps `lynq.sh/applied-hash` and
    `lynq.sh/apply-start-time`, so a metadata-only template change moves a Deployment's
    generation without starting a rollout. Treating that as a rollout re-timed an in-progress
    external `kubectl rollout restart` from the metadata apply. Kinds with no `.spec` on either
    side fall back to comparing generation. A generation jump of two or more (the stale-cache
    rule above) is the one case where generation alone counts.
  - The converse also holds, so for a kind that gives that signal (`apply.TracksRollouts`: it
    has a `.spec` or a non-zero generation) the regression verdict deliberately ignores
    apply-start-time: only `startRollout` (or having no history) ends a Ready history.
    Comparing stamps re-timed the same in-progress restart from a metadata-only apply and
    failed it within `timeoutSeconds` — a false ReadinessTimeout and Degraded alert. A kind
    with neither gives no rollout signal, so a changed stamp still ends its Ready history there.
  - The tracker is in-memory on purpose. Persisting readiness history would need a write per
    readiness transition, breaking invariant 6 and the exactly-one-write-per-apply contract. It is
    released when the LynqNode is cleaned up or found NotFound, and when a LynqNode of the same
    name shows up with a new UID (a delete/recreate no reconcile observed).
- **One LynqNode classification — `classifyNode` (`internal/controller/node_health.go`).**
  - ready = `Ready=True`; failed = `status.failedResources > 0` OR `Conflicted=True` OR
    `Degraded=True` (failure wins over a stale `Ready=True`); everything else — rolling out,
    recovering a regression, not reconciled yet (no Ready condition) — is progressing.
  - LynqHub `status.ready`/`status.failed` (and `hub_ready`/`hub_failed`), the maxSkew
    "currently updating" count, and the LynqForm rollout status all consume it. Counting
    `Ready!=True` as failed made every spot drain raise the hub's failure alerts while the
    rollout counted the same node as merely updating.
  - Progressing nodes have no CRD field or metric of their own: they are
    `hub_desired - hub_ready - hub_failed`, together with nodes not created yet.
- **progressDeadlineSeconds measures "the LynqForm rollout never completed" — nothing else.**
  - A LynqForm rollout is an update that changes the node's `lynq.sh/template-generation` (or
    creates the node). A data-only update — `hostOrUrl`/`activate`/`extra` changed under the same
    generation — is not one: it moves `rollout-update-start` (the maxSkew safety margin reads it)
    but leaves the completion marker as it was. Its failures are the LynqNode's to report: a
    readiness timeout or apply error makes the node failed, and `stallReason` stalls on failed at
    once, marker or not. This matches maxSkew, which also counts per template generation.
  - `lynq.sh/rollout-update-start` only moves when the hub updates the node, so it is as old as
    the update on a node that completed it and regressed much later (spot drain, external
    `kubectl rollout restart`). Timing that regression against it reports Phase=Failed the
    instant the node is seen not ready — the rollout twin of the readiness-timeout trap above.
  - Completion is therefore durable: `lynq.sh/rollout-completed-generation` on the LynqNode. The
    hub sets it to `""` in the same write that creates the node or moves it to a new template
    generation (judged on the uncached read inside that write), and to the template generation
    once a sync sees the node Ready for its current spec (`reachedReadyForCurrentSpec`:
    `Ready=True` AND `status.observedGeneration == metadata.generation`; a Ready condition from
    before the LynqNode controller observed the update is neither progress nor completion).
    `stallReason` times only a node whose marker is pending, from `rollout-update-start`;
    completed nodes are stalled only by `classifyNode == failed`. A restart or leader change reads
    the same annotations, so it neither fakes nor delays a verdict.
  - The LynqForm rollout status reads the same marker (`calculateRolloutStats`): an updated node
    is ready-updated (`reachedReadyForCurrentSpec`), else *recovering* if `updateCompleted`, else
    updating. Phase is `Complete` once ready-updated + recovering = total, so a lost pod no longer
    flips a finished rollout to `InProgress` and restamps its `completionTime`. Recovering has no
    CRD field: it is `updatedNodes - updatingNodes - readyUpdatedNodes`, and the message names it
    only when non-zero (`All N nodes updated (K recovering)`), so the status is rewritten only
    when that count changes. `lynqform_rollout_updating_nodes` excludes it and
    `lynqform_rollout_progress` counts it as done. The hub's maxSkew count deliberately does not
    follow this: a recovering node keeps its slot.
  - The completion write is one per node per update, never on a no-op sync
    (`needsRolloutCompletionMark` is false once written), and only after re-reading the node
    uncached and confirming it still carries the observed update and is not being deleted
    (`recordRolloutCompletion`). It is metadata-only: the LynqNode generation does not move, so
    the hub's `GenerationChangedPredicate` drops the event. It is not free, though. The LynqNode
    controller has no predicate and always runs a spec reconcile, so the event costs one full
    LynqNode reconcile: a live GET per child resource (the applier reads unstructured objects,
    which bypass the cache), the `applied-hash` comparison and the readiness checks — with no
    child write and no status write, since nothing changed. The LynqForm controller also takes
    the event and re-runs its cached LynqNode LIST and O(N) classification. For a 378-node
    rollout with R child resources per node, that is 378 hub GETs + 378 hub writes, 378 LynqNode
    reconciles with about 378×R live GETs, and at most 378 LynqForm LIST passes (fewer, as the
    workqueue coalesces them), spread over the rollout. The marker is not a template variable —
    keep it out of `buildTemplateVariablesFromAnnotations` and `computeRenderCacheKey`, or the
    reconcile it triggers would also re-render and re-apply.
  - A node without the key predates it and counts as completed, so an upgrade cannot fail a
    rollout over a node that regressed before it; the hub does not backfill it.
  - For the same reason, a not-Ready node without the key counts as recovering, not updating, in
    the rollout status — including one caught mid-rollout by the upgrade that introduced the key.
  - A version without the marker (a rollback) neither resets nor records it. A generation change
    it makes leaves a marker for an older generation, which is pending, so the rollout is timed
    after the re-upgrade; a data-only update it makes is not a rollout, as above.
  - Residual: a data-only update re-stamps `rollout-update-start`, so a pending rollout that
    keeps receiving data updates is timed from the latest one. Its failures still stall it.
  - Residual: completion is observed per hub sync. A node that turns Ready and regresses within
    one `syncInterval` of its update looks never-completed and is timed from its update stamp.
  - The Failed message lists sorted node names (capped) and no elapsed times, so an unchanged
    stall renders identically and does not rewrite the LynqForm status. A failed LynqNode list
    is a reconcile error, never zero stats: publishing "no nodes" would flip a Failed rollout to
    Idle and back, with two writes and two events. Likewise, stats computed for one LynqForm
    generation are not written if the form has moved on by the status write
    (`errFormChangedDuringStatus`): the reconcile requeues instead of reporting the previous
    rollout's stall against the new one.
- **Skip-decision is annotation-driven only — there is no in-memory cache.**
  - `shouldSkipApply` reads `lynq.sh/applied-hash` from the live resource. If it matches the
    desired hash, the apply is elided. There is no fast-path / slow-path divergence and no
    in-memory state to invalidate on restart. Restart behavior is identical to warm behavior.
  - This design (single annotation-driven skip path) was adopted to eliminate the recurring
    side-channel-write bug class (commits `d099b63`, `7b629e4`, `b458eda`). The contract is
    "every successful apply issues EXACTLY ONE API write call" — any follow-up MergePatch
    re-introduces the race the `7b629e4` fix established. The unit test
    `TestRegression_ExactlyOneApiWritePerReconcile` is the structural guard.
- **External drift correction has two channels with documented windows.**
  - **Watch-driven (immediate)**: when an external edit bumps `metadata.generation` AND
    strips or alters `lynq.sh/applied-hash`, the Owns watch fires and `shouldSkipApply`
    re-applies on the next reconcile (~30s requeue).
  - **Periodic force-reapply (~10 min)**: when an external edit preserves `applied-hash`
    (e.g. `kubectl edit` that doesn't touch annotations), the annotation-only skip path
    matches and elides the apply. Drift is then corrected on the next periodic
    force-reapply cycle gated by `LynqNode.Status.LastFullReconcileAt` and
    `ForceReapplyInterval`. This window is intentional: it matches Crossplane's periodic
    resync model and trades drift-detection latency for structural elimination of the
    post-apply write race.
  - **Nil-handling on `LastFullReconcileAt`**: the controller treats `nil` as "stamp-now,
    defer first force by one full interval". Eagerly forcing on nil would cause a re-apply
    storm on every controller restart and could regress the BUG 2 restart canary.
- **Why we don't track `metadata.generation` in an annotation.**
  - Stamping live generation back into an annotation requires a follow-up MergePatch after
    the apply. That second API call races against the K8s status subresource, finalizer
    controllers, and admission webhooks — the exact race that commit `b458eda` reverted.
    The cost of accepting the ~10min drift-correction window is far smaller than the
    cost of every regression that the dual-write pattern produces.
- **patchStrategy:replace and the infinite-loop trap.**
  - Each `client.Update()` bumps the Deployment's `metadata.generation` because the
    Lynq-rendered spec omits API-server-defaulted fields (spec.strategy,
    progressDeadlineSeconds, imagePullPolicy, etc.). If `shouldSkipApply` treated every
    benign RV bump as drift, the watch on Deployments would fire, the controller would
    reconcile, RV would be "stale" again → infinite reconcile loop. `observedGeneration`
    permanently lags `generation`, `isDeploymentReady` stays false, and the LynqNode never
    becomes Ready. The annotation-driven skip path correctly avoids this: as long as the
    `applied-hash` annotation on the live resource matches the desired hash, the apply
    is elided regardless of RV.
- `conflictPolicy:Force` has no effect when `patchStrategy:replace` is used. Force only applies to SSA.
- **Any slice published into LynqNode status MUST be sorted before it is published.**
  - `status.appliedResources` is assembled by ranging over a map, and Go randomizes map
    iteration. `status.skippedResourceIds` inherits its order from `TopologicalSort`, which
    also iterates a map. Both must be given a deterministic order — `sort.Strings` for the
    former, sorted IDs within each level for the latter.
  - Why it matters: the API server only elides an update when the serialized object is
    byte-identical. A reordered slice is a real etcd write → `resourceVersion` bump →
    LynqNode watch event → another reconcile → another reordering. That closed a
    self-sustaining full-reconcile loop that pinned the manager at ~2.7x its CPU request on
    an idle, fully-Ready cluster, rewriting a majority of LynqNodes every minute.
  - Churn tracked `1 - 1/k!` in the number of resources almost exactly (k=1: none, k=3:
    ~53%, k=7: ~97%), which is how the mechanism was confirmed.
  - Guarded by `TestRegression_SteadyStateReconcileDoesNotWriteStatus` and
    `TestRegression_AppliedResourcesIsSorted`.
- **"A field was published" is not "a field changed".**
  - Reconcilers republish their whole status every pass, so `StatusManager.applyUpdate`
    must compare each field against the stored value before marking the update dirty.
    Treating a published field as changed issues `Status().Update()` on every reconcile,
    which feeds the same loop as above.
  - Guarded by `TestManager_NoWriteWhenValuesUnchanged` (and its inverse,
    `TestManager_WritesWhenValueActuallyChanges`, so the comparison cannot swallow real
    transitions).
- **"Published empty" is not "not published".**
  - `StatusUpdate` uses nil for a list field nobody published, but publishers build an empty
    list as nil (`applyResources` returns no skipped IDs as nil). `StatusUpdate.Apply`
    therefore records any list carried by an event as non-nil (`publishedList`), so an empty
    list clears the stored one. This covers `skippedResourceIds` and `appliedResources`, the
    only list fields in LynqNode status. Reading nil as "unpublished" left the
    `skippedResourceIds` of a resolved skip on Ready nodes next to `skippedResources: 0`.
  - An empty list equals a nil stored value under `slices.Equal`, so this never adds a write.
  - Guarded by `TestManager_ClearsPublishedEmptyLists`,
    `TestStatusUpdate_ApplyMarksEmptyListsPublished` and
    `TestRegression_StaleSkippedResourceIdsAreClearedOnce`.
- **Rollout throttling must not read LynqNodes from the informer cache.**
  - The hub is re-enqueued from several independent sources. Only `Owns(&LynqNode{})`
    guarantees a fresh node view, because an informer updates its store *before*
    dispatching to handlers. The `For(&LynqHub{})` watch (fed by our own `updateStatus`
    writes), the `Watches(&LynqForm{})` watch, timed requeues, and any already-queued
    event carry no such ordering with the LynqNode cache.
  - On any of those, a cached list can predate an update we just made: the node still
    carries the old `template-generation`, `countUpdatingNodes` skips it, the count comes
    out zero, and a second node is admitted past `maxSkew`.
  - `rolloutSkewCounter` therefore reads through `APIReader`. `updatedInThisIteration`
    covers updates made later in the same pass; the uncached read covers updates the cache
    has not caught up with. Together they are correct for a single active reconciler, which
    is what leader election provides — two simultaneously-active leaders are NOT fenced.
  - The same applies to the slot-*release* side: `isNodeResourcesActuallyReady` must also
    read uncached, or it sees the pre-update, still-Ready Deployment and frees a slot that
    is still occupied.
  - Suppressing the hub-status trigger does NOT fix this — the other triggers produce the
    same unsafe reconcile.
  - Guarded by `internal/controller/maxskew_stale_cache_test.go`, which drives a split
    client whose cache is frozen behind its `APIReader` so the race is deterministic rather
    than timing-dependent.
- **The hub holds one datasource per LynqHub across syncs; the query deadline is what bounds
  outage detection.**
  - Rebuilding the datasource every sync made the adapter's pool settings meaningless and
    paid a DNS lookup, TCP handshake, TLS negotiation and auth round trip per sync — which
    failed intermittently in production. `datasource.Pool` keeps it alive, keyed by hub and
    fingerprinted over every connection setting, so a rotated password or moved endpoint
    still rebuilds on the next sync.
  - That removed the per-sync `Ping`, which was the only ceiling on noticing an unreachable
    database. Nothing else bounds it: the reconcile context has no deadline and the DSN sets
    no dial/read/write timeouts, so a *blackholed* connection (packets dropped rather than
    the socket closed) would block until the OS TCP timeout while the hub kept reporting
    Ready. `queryTimeoutFor` supplies the replacement ceiling, derived from `syncInterval`
    and clamped to `[10s, 2m]`. Do not remove it without providing another bound.
  - The pool is deliberately NOT a `manager.Runnable`: controller-runtime routes a Runnable
    that does not implement `LeaderElectionRunnable` into the same group as the controllers,
    so shutdown could close a datasource an in-flight reconcile is using. `main` closes it
    after `mgr.Start` returns. Implementing `NeedLeaderElection() == false` is worse — the
    "Others" group stops *before* the controllers.
  - Pool defaults (3 open / 2 idle) describe actual usage: a hub runs one query at a time,
    and `database/sql` opens connections lazily, so a hub holds exactly one connection.
- **API server warnings are deduplicated on the rest.Config, not per client.**
  - A deprecated API version in a LynqForm (e.g. `KafkaTopic` v1beta2) makes the API server
    attach a warning to every apply and read of that resource. controller-runtime's default
    handler logs each one: 3 manifests × 126 nodes filled the manager log with 22k identical
    lines in 28 minutes, and kubelet log rotation lost the lines around an incident.
  - `main` sets `WarningHandlerWithContext` on the config passed to `ctrl.NewManager`
    (`withKubeAPIWarningHandler`). Every client the manager builds — manager client,
    `APIReader`, informers, event recorder, leader election — copies the config, so they share
    one `KubeAPIWarningLogger` and dedup is process-wide. A client built from a config that did
    not come from there (e.g. a new `client.New` with its own config) would fall back to
    controller-runtime's non-deduplicating handler.
  - Never suppress warnings entirely: the first occurrence is how an operator learns a template
    needs migrating. Dedup memory is one entry per distinct message. Built-in deprecation and
    field warnings carry no object name, but admission webhook warnings may, which bounds the
    growth by the number of managed objects.
- **Leader lease renewal shares a flow with all of the manager's traffic.**
  - The lease client's HTTP timeout is `--leader-elect-renew-deadline`, and its requests come
    from the operator ServiceAccount, so APF puts them in `service-accounts`/`workload-low`
    together with every reconcile GET and apply. A burst of LynqNode reconciles (each does a live
    GET per child resource) or a slow API server can starve a renewal; after renew-deadline the
    manager exits with `leader election lost`. Longer timings are the knob, not more replicas.
  - `--leader-elect-release-on-cancel` is safe because the manager releases the lease only after
    stopping the controllers, StatusManager and caches, and `main` does nothing on the API server
    after `mgr.Start` returns (it only closes `datasource.Pool`). Keep it that way: any API write
    added after `mgr.Start` would run while a new leader may already be reconciling.
  - StatusManager stops alongside the controllers, not after them: it flushes what it has
    batched when it stops, and a status publish from a reconcile that finishes later is dropped
    (the new leader recomputes it). Safe, but not a guarantee that every status update lands.
  - The release is the last step of a shutdown that may wait up to the manager's 30s graceful
    timeout for in-flight reconciles, while the shipped manifests give the pod
    `terminationGracePeriodSeconds: 10` (chart value `terminationGracePeriodSeconds`). A kill
    before the release silently drops it and the standby waits out the lease, as without the
    flag. Pair `--leader-elect-release-on-cancel` with a grace period of 40s or more.

---

## Example Workflow

### Single Template Scenario
1. User creates `LynqHub` pointing to MySQL in a specific namespace (e.g., `default`)
2. User creates `LynqForm` (e.g., "web-app") referencing the hub in the same namespace
3. LynqHub controller syncs every `syncInterval`:
   - Finds 1 template referencing this hub
   - Queries MySQL for active rows (`activate=true`) - e.g., 3 rows (uid: a, b, c)
   - Creates 3 `LynqNode` CRs: `a-web-app`, `b-web-app`, `c-web-app`
   - Updates `status.referencingTemplates=1`, `status.desired=3`

### Multi-Template Scenario (✅ NEW)
1. User creates `LynqHub` pointing to MySQL
2. User creates multiple `LynqForm` CRs referencing the same hub:
   - "web-app" template (web tier resources)
   - "worker" template (background job resources)
3. LynqHub controller syncs every `syncInterval`:
   - Finds 2 templates referencing this hub
   - Queries MySQL for 3 active rows (uid: a, b, c)
   - Creates 6 `LynqNode` CRs (2 templates × 3 rows):
     - `a-web-app`, `b-web-app`, `c-web-app`
     - `a-worker`, `b-worker`, `c-worker`
   - Updates `status.referencingTemplates=2`, `status.desired=6`

### Reconciliation Process
4. For each `LynqNode`:
   - Template controller ensures linkage
   - LynqNode controller reconciles:
     - Renders templates with row data
     - Builds dependency graph
     - Applies all resources in the same namespace as the LynqNode CR
     - Waits for readiness
     - Updates status
5. Users observe:
   - `LynqHub.status.{referencingTemplates, desired, ready, failed}`
   - `LynqNode.status.conditions[Ready]`
   - Events and metrics
   - All node resources in the same namespace

---

## References

- **K8s SSA**: https://kubernetes.io/docs/reference/using-api/server-side-apply/
- **Sprig Functions**: https://masterminds.github.io/sprig/
- **Kubebuilder**: https://book.kubebuilder.io/
- **Controller Runtime**: https://pkg.go.dev/sigs.k8s.io/controller-runtime
