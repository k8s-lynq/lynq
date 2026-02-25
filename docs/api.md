---
description: "Complete API reference for Lynq CRDs — LynqHub, LynqForm, and LynqNode. Includes spec fields, status fields, and configuration examples."
---

# API Reference

Complete API reference for Lynq CRDs.

[[toc]]

## LynqHub

Defines external data source and sync configuration.

::: info Resource metadata
- **Kind:** `LynqHub`
- **API Version:** `operator.lynq.sh/v1`
:::

### Spec

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-hub
spec:
  # Data source configuration
  source:
    type: mysql                      # mysql (postgresql planned for v1.2)
    mysql:
      host: string                   # Database host (required)
      port: int                      # Database port (default: 3306)
      username: string               # Database username (required)
      passwordRef:                   # Password secret reference (required)
        name: string                 # Secret name
        key: string                  # Secret key
      database: string               # Database name (required)
      table: string                  # Table name (required)
    syncInterval: duration           # Sync interval (required, e.g., "1m")
  
  # Required column mappings
  valueMappings:
    uid: string                      # Node ID column (required)
    # hostOrUrl: string              # DEPRECATED v1.1.11+ (optional, removed in v1.3.0)
    activate: string                 # Activation flag column (required)
  
  # Optional column mappings
  extraValueMappings:
    key: value                       # Additional column mappings (optional)
```

### Status

```yaml
status:
  observedGeneration: int64          # Last observed generation
  referencingTemplates: int32        # Number of templates referencing this hub
  desired: int32                     # Desired LynqNode CRs (templates × rows)
  ready: int32                       # Ready LynqNode CRs
  failed: int32                      # Failed LynqNode CRs
  conditions:                        # Status conditions
  - type: Ready
    status: "True"
    reason: SyncSucceeded
    message: "Successfully synced N nodes"
    lastTransitionTime: timestamp
```

## LynqForm

Defines resource blueprint for nodes.

::: info Resource metadata
- **Kind:** `LynqForm`
- **API Version:** `operator.lynq.sh/v1`
:::

### Spec

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: my-template
spec:
  hubId: string                 # LynqHub name (required)

  # Rollout configuration (optional, new in v1.1.16)
  rollout:
    maxSkew: int32                   # Max simultaneous updates (0=unlimited, default: 0)
    progressDeadlineSeconds: int32   # Update timeout in seconds (default: 600)

  # Resource arrays
  serviceAccounts: []TResource
  deployments: []TResource
  statefulSets: []TResource
  daemonSets: []TResource
  services: []TResource
  configMaps: []TResource
  secrets: []TResource
  persistentVolumeClaims: []TResource
  jobs: []TResource
  cronJobs: []TResource
  ingresses: []TResource
  podDisruptionBudgets: []TResource  # PodDisruptionBudget resources
  networkPolicies: []TResource       # NetworkPolicy resources
  horizontalPodAutoscalers: []TResource  # HorizontalPodAutoscaler resources
  manifests: []TResource             # Raw unstructured resources
```

### Rollout Configuration

::: tip New in v1.1.16
The `rollout` configuration enables gradual LynqNode updates when templates change.
:::

```yaml
spec:
  rollout:
    maxSkew: 5                       # Update up to 5 nodes simultaneously
    progressDeadlineSeconds: 600     # 10 minute timeout per node
```

**maxSkew Behavior:**
- `0` (default): Unlimited - all nodes update simultaneously (existing behavior)
- `1`: Serial rollout - one node at a time
- `N`: Parallel rollout with sliding window - up to N nodes updating at once

**Template-Isolated Strategy:**
- Each LynqForm applies maxSkew independently
- Multiple LynqForms referencing the same LynqHub don't interfere with each other
- Example: Form A (maxSkew=5) + Form B (maxSkew=5) = up to 10 total nodes updating

### TResource Structure

```yaml
id: string                           # Unique resource ID (required)
nameTemplate: string                 # Go template for resource name (required)
labelsTemplate:                      # Template-enabled labels (optional)
  key: value
annotationsTemplate:                 # Template-enabled annotations (optional)
  key: value
dependIds: []string                  # Dependency IDs (optional)
skipOnDependencyFailure: bool        # Skip if dependency fails (default: true)
creationPolicy: string               # Once | WhenNeeded (default: WhenNeeded)
deletionPolicy: string               # Delete | Retain (default: Delete)
conflictPolicy: string               # Stuck | Force (default: Stuck)
patchStrategy: string                # apply | merge | replace (default: apply)
waitForReady: bool                   # Wait for resource ready (default: true)
timeoutSeconds: int32                # Readiness timeout (default: 300, max: 3600)
spec: object                         # Kubernetes resource spec (required)
```

### Status

```yaml
status:
  observedGeneration: int64
  validationErrors: []string         # Template validation errors
  totalNodes: int32                  # Total nodes using this template
  readyNodes: int32                  # Ready nodes
  rollout:                           # Rollout status (only when maxSkew > 0)
    phase: string                    # Idle | InProgress | Failed | Complete
    targetGeneration: int64          # Target template generation
    totalNodes: int32                # Total nodes for this template
    updatedNodes: int32              # Nodes updated to target generation
    updatingNodes: int32             # Currently updating (not Ready yet)
    readyUpdatedNodes: int32         # Updated AND Ready
    startTime: timestamp             # Rollout start time
    completionTime: timestamp        # Rollout completion time
    message: string                  # Status message
  conditions:
  - type: Valid
    status: "True"
    reason: ValidationSucceeded
```

#### Rollout Status

::: tip New in v1.1.16
The `rollout` status tracks gradual rollout progress when `maxSkew` is configured.
:::

**Phase Values:**
- `Idle`: No rollout in progress (all nodes at current generation)
- `InProgress`: Rollout actively updating nodes
- `Failed`: Rollout failed (progress deadline exceeded)
- `Complete`: All nodes updated and ready

**Tracking Fields:**
- `updatedNodes`: Nodes that have been updated to the target generation
- `updatingNodes`: Nodes currently being updated (updated but not Ready yet)
- `readyUpdatedNodes`: Nodes that are both updated AND Ready

## LynqNode

Represents a single node instance.

::: info Resource metadata
- **Kind:** `LynqNode`
- **API Version:** `operator.lynq.sh/v1`
:::

::: warning Managed resource
LynqNode objects are typically managed by the operator and rarely created manually.
:::

### Spec

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqNode
metadata:
  name: acme-prod-template
  annotations:
    # Template variables (set by Hub controller)
    lynq.sh/uid: "acme-corp"
    # lynq.sh/host: "acme.example.com"                # DEPRECATED v1.1.11+
    # lynq.sh/hostOrUrl: "https://acme.example.com"   # DEPRECATED v1.1.11+
    lynq.sh/activate: "true"
    # Extra variables from extraValueMappings (recommended for custom fields)
    lynq.sh/planId: "enterprise"
    lynq.sh/nodeUrl: "https://acme.example.com"       # Use extraValueMappings instead
spec:
  hubId: string                 # Registry name
  templateRef: string                # Template name

  # Rendered resources (already evaluated)
  deployments: []TResource
  # ... (same structure as LynqForm)
```

### Status

```yaml
status:
  observedGeneration: int64
  desiredResources: int32            # Total resources
  readyResources: int32              # Ready resources
  failedResources: int32             # Failed resources
  skippedResources: int32            # Resources skipped due to dependency failures
  skippedResourceIds: []string       # IDs of skipped resources
  appliedResources: []string         # Tracked resource keys for orphan detection
                                     # Format: "kind/namespace/name@id"
                                     # Example: ["Deployment/default/app@deploy-1", "Service/default/app@svc-1"]
  conditions:
  - type: Ready
    status: "True"
    reason: Reconciled
    message: "Successfully reconciled all resources"
    lastTransitionTime: timestamp
  - type: Progressing
    status: "False"
    reason: ReconcileComplete
  - type: Conflicted
    status: "False"
    reason: NoConflicts
  - type: Degraded
    status: "False"
    reason: Healthy
    message: "All resources are healthy"
```

#### Condition Types

**Ready Condition**

Indicates whether the node is fully reconciled and all resources are ready.

**Status Values:**
- `True`: All resources successfully reconciled and ready
- `False`: Not all resources are ready or some have failed

**Possible Reasons** (when `status=False`):
- `ResourcesFailedAndConflicted`: Both failed and conflicted resources exist (highest priority)
- `ResourcesConflicted`: One or more resources in conflict state
- `ResourcesFailed`: One or more resources failed during reconciliation
- `NotAllResourcesReady`: Resources exist but haven't reached ready state yet

::: tip New in v1.1.4
The Ready condition now provides granular reasons to help quickly identify the root cause of failures. Conflict-related reasons are prioritized for better visibility.
:::

**Progressing Condition**

Indicates whether reconciliation is currently in progress.

**Status Values:**
- `True`: Reconciliation is actively applying changes
- `False`: Reconciliation completed

**Degraded Condition**

::: tip New in v1.1.4
The Degraded condition provides visibility into node health issues separate from the Ready condition.
:::

Indicates when a node is not functioning optimally, even if reconciliation has completed.

**Status Values:**
- `True`: Node has health issues
- `False`: Node is healthy

**Possible Reasons** (when `status=True`):
- `ResourceFailuresAndConflicts`: Node has both failed and conflicted resources
- `ResourceFailures`: Node has failed resources
- `ResourceConflicts`: Node has conflicted resources
- `ResourcesNotReady`: Not all resources have reached ready state (new in v1.1.4)

**Conflicted Condition**

Indicates whether any resources have ownership conflicts.

**Status Values:**
- `True`: One or more resources are in conflict
- `False`: No conflicts detected
```

## Field Types

### Duration

String with unit suffix:
- `s`: seconds
- `m`: minutes
- `h`: hours

Examples: `30s`, `1m`, `2h`

### CreationPolicy

- `Once`: Create once, never update
- `WhenNeeded`: Create and update as needed (default)

### DeletionPolicy

- `Delete`: Delete resource on LynqNode deletion (default) - uses ownerReference for automatic cleanup
- `Retain`: Keep resource on deletion - uses label-based tracking only (NO ownerReference set at creation)

### ConflictPolicy

- `Stuck`: Stop on ownership conflict (default)
- `Force`: Take ownership forcefully

### PatchStrategy

- `apply`: Server-Side Apply (default)
- `merge`: Strategic Merge Patch
- `replace`: Full replacement

## Annotations

### Node Annotations (auto-generated)

```yaml
# Template variables
lynq.sh/uid: string
# lynq.sh/host: string                # DEPRECATED v1.1.11+ (removed in v1.3.0)
# lynq.sh/hostOrUrl: string           # DEPRECATED v1.1.11+ (removed in v1.3.0)
lynq.sh/activate: string

# Extra variables from extraValueMappings (recommended approach)
lynq.sh/<key>: value

# CreationPolicy tracking
lynq.sh/created-once: "true"
```

### Resource Tracking Labels

**Label-based tracking** is used instead of ownerReferences for:
- Cross-namespace resources (ownerReferences don't work across namespaces)
- Namespace resources (cannot have ownerReferences)
- **DeletionPolicy=Retain resources** (to prevent automatic garbage collection)

```yaml
# Tracking labels (set at resource creation)
lynq.sh/node: node-name
lynq.sh/node-namespace: node-namespace

# Orphan label (added when resource becomes orphaned - for selector queries)
lynq.sh/orphaned: "true"
```

**Orphan Markers:**

When resources are retained (not deleted) due to `DeletionPolicy=Retain`, the operator adds:

- **Label** `orphaned: "true"` - For easy filtering with label selectors
- **Annotation** `orphaned-at` - RFC3339 timestamp when the resource became orphaned
- **Annotation** `orphaned-reason` - Reason for becoming orphaned:
  - `RemovedFromTemplate`: Resource was removed from LynqForm
  - `LynqNodeDeleted`: LynqNode CR was deleted

**Why split label/annotation?**
- **Label**: Simple value for selector queries (Kubernetes label values must be RFC 1123 compliant)
- **Annotations**: Detailed metadata like timestamps (no value restrictions)

**Orphan Marker Lifecycle:**

1. **Resource removed from template** → Orphan markers added (label + annotations)
2. **Resource re-added to template** → Orphan markers automatically removed during apply
3. **No manual cleanup needed** → Operator manages the full lifecycle

This enables safe template evolution: you can freely add/remove resources from templates, and previously orphaned resources will be cleanly re-adopted if you add them back.

**Finding orphaned resources:**

```bash
# Find all orphaned resources
kubectl get all -A -l lynq.sh/orphaned=true

# Find orphaned resources by reason (using annotation)
kubectl get all -A -l lynq.sh/orphaned=true -o jsonpath='{range .items[?(@.metadata.annotations.k8s-lynq\.org/orphaned-reason=="RemovedFromTemplate")]}{.kind}/{.metadata.name}{"\n"}{end}'

# Find orphaned resources from a specific node (label still available)
kubectl get all -A -l lynq.sh/orphaned=true,lynq.sh/node=my-node
```

**Re-adoption YAML Comparison:**

When a previously orphaned resource is re-added to the template, Lynq automatically removes orphan markers:

::: code-group
```yaml [Before: Orphaned Resource]
# Resource after being removed from template (orphaned)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: acme-api
  namespace: production
  labels:
    lynq.sh/node: acme-web
    lynq.sh/node-namespace: production
    lynq.sh/orphaned: "true"           # ← Orphan label
  annotations:
    lynq.sh/orphaned-at: "2024-01-15T10:30:00Z"        # ← Orphan timestamp
    lynq.sh/orphaned-reason: "RemovedFromTemplate"    # ← Orphan reason
    lynq.sh/deletion-policy: "Retain"
spec:
  # ...
```

```yaml [After: Re-adopted Resource]
# Same resource after being re-added to template (re-adopted)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: acme-api
  namespace: production
  labels:
    lynq.sh/node: acme-web
    lynq.sh/node-namespace: production
    # ✅ lynq.sh/orphaned label REMOVED
  annotations:
    lynq.sh/deletion-policy: "Retain"
    # ✅ lynq.sh/orphaned-at REMOVED
    # ✅ lynq.sh/orphaned-reason REMOVED
  ownerReferences:
    - apiVersion: operator.lynq.sh/v1
      kind: LynqNode
      name: acme-web
      uid: xxx-xxx-xxx
spec:
  # ...
```
:::

**Verify re-adoption:**

```bash
# Check if resource is no longer orphaned
kubectl get deployment acme-api -n production -o jsonpath='{.metadata.labels.lynq\.sh/orphaned}'
# Output: (empty - label removed)

# Check ownerReference is set (for same-namespace resources with Delete policy)
kubectl get deployment acme-api -n production -o jsonpath='{.metadata.ownerReferences[0].name}'
# Output: acme-web
```

### Resource Annotations

**DeletionPolicy Annotation:**

The operator automatically adds a `deletion-policy` annotation to all created resources:

```yaml
metadata:
  annotations:
    lynq.sh/deletion-policy: "Retain"  # or "Delete"
```

**Purpose:**
- **Critical for orphan cleanup**: When resources are removed from templates, they no longer exist in the template spec
- The annotation is the **only source of truth** for determining the correct cleanup behavior
- Without this annotation, all orphaned resources would default to `Delete` policy

**Behavior:**
- Set automatically during resource creation by `renderResource` function
- Read during orphan cleanup in `deleteOrphanedResource` function
- Falls back to `Delete` if annotation is missing (defensive default)

**Example query:**

```bash
# Find all Retain resources
kubectl get all -A -o jsonpath='{range .items[?(@.metadata.annotations.k8s-lynq\.org/deletion-policy=="Retain")]}{.kind}/{.metadata.name}{"\n"}{end}'
```

## Examples

See [Templates Guide](templates.md) and [Quick Start Guide](quickstart.md) for complete examples.

## Validation Rules

### LynqHub

- `spec.valueMappings` must include: `uid`, `activate`
- `spec.valueMappings.hostOrUrl` is deprecated since v1.1.11 (optional, will be removed in v1.3.0)
- Use `spec.extraValueMappings` with `toHost()` template function instead of `hostOrUrl`
- `spec.source.syncInterval` must match pattern: `^\d+(s|m|h)$`
- `spec.source.mysql.host` required when `type=mysql`

### LynqForm

- `spec.hubId` must reference existing LynqHub
- Each `TResource.id` must be unique within template
- `dependIds` must not form cycles
- Templates must be valid Go templates

### LynqNode

- Typically validated by operator, not manually created
- All referenced resources must exist

## Quick Reference: kubectl Commands

Common kubectl commands for working with Lynq resources:

### Listing Resources

```bash
# List all LynqHubs
kubectl get lynqhubs -A

# List all LynqForms
kubectl get lynqforms -A

# List all LynqNodes with status columns
kubectl get lynqnodes -A

# Get detailed status
kubectl get lynqnodes -A -o wide
```

### Viewing Status

```bash
# Check Hub sync status
kubectl get lynqhub my-hub -o jsonpath='{.status}'

# Check node readiness
kubectl get lynqnode acme-web -o jsonpath='{.status.readyResources}/{.status.desiredResources}'

# List failed nodes
kubectl get lynqnodes -A --field-selector 'status.failedResources!=0'

# Get all node conditions
kubectl get lynqnode acme-web -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}){"\n"}{end}'
```

### Querying by Labels

```bash
# Find all resources managed by a node
kubectl get all -l lynq.sh/node=acme-web

# Find orphaned resources
kubectl get all -A -l lynq.sh/orphaned=true

# Find resources from a specific hub
kubectl get lynqnodes -l lynq.sh/hub=customer-hub
```

### Debugging

```bash
# Describe a node (shows events)
kubectl describe lynqnode acme-web

# Check applied resources list
kubectl get lynqnode acme-web -o jsonpath='{.status.appliedResources}'

# Check skipped resources (due to dependency failures)
kubectl get lynqnode acme-web -o jsonpath='{.status.skippedResourceIds}'

# Force reconciliation
kubectl annotate lynqnode acme-web force-sync="$(date +%s)" --overwrite
```

### Rollout Status (v1.1.16+)

```bash
# Check rollout phase
kubectl get lynqform my-template -o jsonpath='{.status.rollout.phase}'

# Watch rollout progress
watch kubectl get lynqform my-template -o jsonpath='{.status.rollout.updatedNodes}/{.status.rollout.totalNodes}'

# Get detailed rollout status
kubectl get lynqform my-template -o jsonpath='{.status.rollout}'
```

### Output Formats

```bash
# YAML output for debugging
kubectl get lynqnode acme-web -o yaml

# JSON output for scripting
kubectl get lynqnodes -o json | jq '.items[] | {name: .metadata.name, ready: .status.readyResources}'

# Custom columns
kubectl get lynqnodes -o custom-columns='NAME:.metadata.name,READY:.status.readyResources,DESIRED:.status.desiredResources,FAILED:.status.failedResources'
```

## See Also

- [Template Guide](templates.md) - Template syntax and functions
- [Policies Guide](policies.md) - Policy options
- [Dependencies Guide](dependencies.md) - Resource ordering
