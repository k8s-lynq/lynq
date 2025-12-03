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
- `status.failed` = count of failed LynqNodes across all templates
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
- **Timed-out dependencies**: Resources not ready after `timeoutSeconds` (default: 300s).
  - Transitions to **failed** state
  - Emits `ReadinessTimeout` event
  - Triggers `skipOnDependencyFailure` logic for dependents
- This distinction ensures Deployments starting up don't cause misleading "DependencySkipped" events

**LynqNode Status Fields for Dependency Tracking**:
- `status.skippedResources`: Count of resources skipped due to dependency failures
- `status.skippedResourceIds`: List of resource IDs that were skipped
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
3. Filter rows where `activate=true`
4. Calculate desired LynqNode set (all template-row combinations)
   - For each template and each row, create key: `{template-name}-{uid}`
   - Desired count = `len(templates) * len(activeRows)`
5. Create missing LynqNodes (naming: `{uid}-{template-name}`), update existing, delete excess
6. Update `status.{referencingTemplates, desired, ready, failed}`

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
3. **Drift Detection** ✅ (Implemented & Optimized):
   - **Event-driven**: `Owns()` watches on 11 resource types + `Watches()` on Namespaces
     - ServiceAccounts, Deployments, StatefulSets, DaemonSets
     - Services, ConfigMaps, Secrets, PersistentVolumeClaims
     - Jobs, CronJobs, Ingresses
     - **Namespaces**: Tracked via labels (cannot use ownerReferences)
   - **Watch Predicates**: Only trigger on Generation/Annotation changes (not status-only updates)
     - Reduces unnecessary reconciliation overhead
     - Filters out noisy status updates from watched resources
   - **Fast Requeue**: 30-second periodic requeue (changed from 5 minutes)
     - Ensures child resource status changes are reflected quickly in LynqNode status
     - Balances responsiveness with cluster load
   - **Auto-correction**: Automatically reverts manual changes to node resources
   - **Location**: `internal/controller/lynqnode_controller.go` (SetupWithManager, line ~930)
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
- `lynqnode_degraded_status`: Binary indicator (1=degraded, 0=healthy) with reason label
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

- Retry with exponential backoff
- If cumulative failures exceed threshold, mark LynqNode as Degraded
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
2. `LynqNode` is Ready IFF all resources are Ready
3. SSA fieldManager MUST be `lynq`
4. Dependency cycles MUST be rejected
5. Naming MUST respect 63-char K8s limit

### Common Pitfalls

- Don't forget to auto-extract `.host` from `.hostOrUrl`
- Template rendering errors should never panic
- Always validate `dependIds` before topological sort
- SSA requires `fieldManager` and correct content-type
- `waitForReady=true` blocks the reconciliation pipeline

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
