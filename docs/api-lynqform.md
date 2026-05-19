---
description: "LynqForm CRD API reference — hubId, resource arrays, TResource spec fields, dependency ordering, creationPolicy, and deletionPolicy."
---

# LynqForm API Reference

**Kind:** `LynqForm`  
**API Version:** `operator.lynq.sh/v1`  
**Group:** `operator.lynq.sh`

LynqForm is the resource blueprint. It references a LynqHub and defines which Kubernetes resources to create for each active row, using Go templates. One LynqForm can reference one hub; one hub can be referenced by many forms.

→ [Templates guide](templates.md) · [Policies reference](policies.md) · [API index](api.md)

## Spec

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: my-form
  namespace: lynq-system
spec:
  hubId: string                      # LynqHub name (required)

  rollout:                           # Optional — controls update rollout (v1.1.16+)
    maxSkew: 0                       # Max simultaneous node updates (0 = unlimited)
    progressDeadlineSeconds: 600     # Per-node update timeout in seconds

  # Resource arrays — each entry follows the TResource structure (see below)
  serviceAccounts: []
  deployments: []
  statefulSets: []
  daemonSets: []
  services: []
  configMaps: []
  secrets: []
  persistentVolumeClaims: []
  jobs: []
  cronJobs: []
  ingresses: []
  podDisruptionBudgets: []
  networkPolicies: []
  horizontalPodAutoscalers: []
  manifests: []                      # Raw unstructured resources (CRDs, custom kinds)
```

## TResource Structure

Every entry in any resource array is a `TResource`:

```yaml
id: string                           # Unique ID within this form (required)
nameTemplate: string                 # Go template for metadata.name (required)
spec: object                         # Kubernetes resource spec (required)

# Optional — templates
labelsTemplate:                      # Template-enabled labels applied to the resource
  key: "{{ .uid }}-value"
annotationsTemplate:                 # Template-enabled annotations
  key: value
targetNamespace: string              # Target namespace; defaults to LynqNode's namespace

# Optional — dependencies
dependIds: []string                  # IDs of resources that must be applied first
skipOnDependencyFailure: true        # Skip this resource if any dependency failed (default: true)

# Optional — lifecycle policies (all have defaults)
creationPolicy: WhenNeeded           # WhenNeeded | Once
deletionPolicy: Delete               # Delete | Retain
conflictPolicy: Stuck                # Stuck | Force  (SSA-only — no effect on merge/replace)
patchStrategy: apply                 # apply | merge | replace

# Optional — readiness
waitForReady: true                   # Wait for resource ready condition (default: true)
timeoutSeconds: 300                  # Readiness timeout in seconds (default: 300, max: 3600)
                                     # Measured from lynq.sh/apply-start-time annotation
                                     # (preserved across reconciles when spec is unchanged)
```

### Policy defaults

All policies default to the safest option. Only specify what differs:

| Policy | Default | Options | Notes |
|--------|---------|---------|-------|
| `creationPolicy` | `WhenNeeded` | `WhenNeeded`, `Once` | |
| `deletionPolicy` | `Delete` | `Delete`, `Retain` | |
| `conflictPolicy` | `Stuck` | `Stuck`, `Force` | `Force` is SSA-only; ignored by `merge` / `replace` |
| `patchStrategy` | `apply` | `apply`, `merge`, `replace` | `apply` (SSA) is the only strategy that preserves fields owned by other controllers |

→ [Policies reference](policies.md) for detailed behavior.

### `nameTemplate`

Go template string that produces `metadata.name`. Must produce a valid Kubernetes name (lowercase, alphanumeric, `-`), maximum 63 characters.

```yaml
nameTemplate: "{{ .uid }}-app"
nameTemplate: "{{ .uid | trunc63 }}"
nameTemplate: "{{ .uid }}-{{ .planId | default \"basic\" }}"
```

### `dependIds`

Array of `id` values that must be applied (and ready, if `waitForReady: true`) before this resource. Lynq builds a DAG from all `dependIds`, performs a topological sort, and applies in order. Cycles are rejected at admission time.

```yaml
- id: configmap
  nameTemplate: "{{ .uid }}-config"
  spec: ...

- id: deployment
  dependIds: [configmap]    # applied only after configmap is ready
  nameTemplate: "{{ .uid }}-app"
  spec: ...
```

### `skipOnDependencyFailure`

Controls what happens when a dependency **fails** (apply error or timeout — not just "not ready yet").

- `true` (default): Skip this resource and emit a `DependencySkipped` event.
- `false`: Apply this resource regardless, emit `DependencyFailedButProceeding`.

Note: a dependency that is still starting up (not yet ready) silently **blocks** dependents — no skip event is emitted and `skipOnDependencyFailure` does not apply until the dependency transitions to failed.

## Rollout Configuration (v1.1.16+)

Controls how many LynqNodes are updated simultaneously when the form template changes.

```yaml
rollout:
  maxSkew: 5                   # update at most 5 nodes at a time
  progressDeadlineSeconds: 600 # 10-minute timeout per node
```

**`maxSkew` behavior:**
- `0` (default): No limit — all nodes update simultaneously
- `1`: Serial — one node at a time
- `N`: Sliding window — up to N nodes updating in parallel

Each LynqForm's rollout is independent; multiple forms pointing at the same hub do not interfere.

## Status

```yaml
status:
  observedGeneration: int64
  totalNodes: int32            # Total LynqNodes using this form
  readyNodes: int32            # LynqNodes with Ready=True

  rollout:                     # Only populated when maxSkew > 0
    phase: string              # Idle | InProgress | Failed | Complete
    targetGeneration: int64
    totalNodes: int32
    updatedNodes: int32        # Updated to target generation
    updatingNodes: int32       # Updated but not yet ready
    readyUpdatedNodes: int32   # Updated AND ready
    startTime: timestamp
    completionTime: timestamp
    message: string

  conditions:
  - type: Valid
    status: "True" | "False"
    reason: ValidationSucceeded | ValidationFailed
    message: string
```

## Validation

The admission webhook enforces:
- `spec.hubId` must reference an existing LynqHub in the same namespace
- Each `TResource.id` must be unique within the form
- `dependIds` must reference IDs that exist within the same form
- `dependIds` must not form cycles
- `nameTemplate` and `labelsTemplate`/`annotationsTemplate` must be valid Go templates

## Example

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: web-app
  namespace: lynq-system
spec:
  hubId: production-nodes

  configMaps:
  - id: config
    nameTemplate: "{{ .uid }}-config"
    spec:
      apiVersion: v1
      kind: ConfigMap
      data:
        PLAN: "{{ .planId }}"
        REGION: "{{ .region }}"

  deployments:
  - id: app
    dependIds: [config]
    nameTemplate: "{{ .uid }}-app"
    creationPolicy: WhenNeeded
    deletionPolicy: Delete
    waitForReady: true
    timeoutSeconds: 300
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: "{{ .uid }}"
        template:
          metadata:
            labels:
              app: "{{ .uid }}"
          spec:
            containers:
            - name: app
              image: myapp:latest
              envFrom:
              - configMapRef:
                  name: "{{ .uid }}-config"

  services:
  - id: svc
    nameTemplate: "{{ .uid }}-svc"
    waitForReady: false
    spec:
      apiVersion: v1
      kind: Service
      spec:
        selector:
          app: "{{ .uid }}"
        ports:
        - port: 80
          targetPort: 8080
```

## See Also

- [Templates](templates.md) — template syntax, functions, and variable reference
- [Policies](policies.md) — policy behavior and recommended combinations
- [Dependencies](dependencies.md) — dependency ordering and failure handling
- [LynqHub API](api-lynqhub.md) — datasource and sync configuration
- [LynqNode API](api-lynqnode.md) — instance status and lifecycle
- [API index](api.md) — common types and kubectl reference
