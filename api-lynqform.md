---
url: 'https://lynq.sh/api-lynqform.md'
description: >-
  LynqForm CRD API reference — hubId, resource arrays, TResource spec fields,
  dependency ordering, creationPolicy, and deletionPolicy.
---

# LynqForm API Reference

**Kind:** `LynqForm`\
**API Version:** `operator.lynq.sh/v1`\
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
  namespaces: []                     # Cluster-scoped; tracked via labels (no ownerReference)
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

* `true` (default): Skip this resource and emit a `DependencySkipped` event.
* `false`: Apply this resource regardless, emit `DependencyFailedButProceeding`.

Note: a dependency that is still starting up (not yet ready) silently **blocks** dependents — no skip event is emitted and `skipOnDependencyFailure` does not apply until the dependency transitions to failed.

## Rollout Configuration (v1.1.16+)

Controls how many LynqNodes are updated simultaneously when the form template changes.

```yaml
rollout:
  maxSkew: 5                   # update at most 5 nodes at a time
  progressDeadlineSeconds: 600 # report a node update that is not Ready within 10 minutes
```

**`maxSkew` behavior:**

* `0` (default): No limit — all nodes update simultaneously
* `1`: Serial — one node at a time
* `N`: Sliding window — up to N nodes updating in parallel

Each LynqForm's rollout is independent; multiple forms pointing at the same hub do not interfere.

**`progressDeadlineSeconds` behavior** (default `600`, range `60`–`3600`):

The deadline times **LynqForm rollouts**: updates that move a node to a new form generation (and
newly created nodes). The rollout status reports `phase: Failed` when an updated node (one
already on the form's current generation) either:

* is **failed** — it has failed or conflicted resources, or is `Degraded` (template, variables or
  dependency errors), or
* has **not become Ready since it was updated to that generation** and its update started more
  than `progressDeadlineSeconds` ago.

An update that only changes the node's data from the hub (row values such as `hostOrUrl`,
`activate` or extra mappings, under the same form generation) is not a LynqForm rollout and is
not timed by this deadline. If it never becomes Ready, the LynqNode reports it as failed once its
resources exceed their `timeoutSeconds`, and the rollout turns `Failed` at that point.

`status.rollout.message` lists the nodes (up to 5, then a count) and the reason for each, a
`RolloutFailed` warning event is emitted once when the phase turns Failed, and
`lynqform_rollout_phase` reports `2`. The phase leaves `Failed` on its own — with a
`RolloutRecovered` event — once those nodes become Ready or are fixed by a new LynqForm change.

The deadline only measures **an update that never completed**. A node that became Ready after its
update and later drops out of it (node drain, lost pod, `kubectl rollout restart`) is recovering,
not stalled, and never fails the rollout by deadline, however long ago the update was. It is not
rolling out either: the rollout's counts and phase follow the completion record below, so a
`Complete` rollout stays `Complete` (with the same `completionTime`) while such a node recovers,
and the message names it — `All 126 nodes updated (1 recovering)`. `updatingNodes` and
`lynqform_rollout_updating_nodes` count only nodes whose update has not completed. A recovering
node that turns failed still turns the rollout `Failed` at once.

`Failed` exposes the stall; it does not skip the node. A failed or stuck node keeps its `maxSkew`
slot, so with `maxSkew: 1` no further node is updated until it recovers. A recovering node keeps
its slot too, so a rollout can read `InProgress` with `updatingNodes: 0`: it is waiting for the
recovering nodes named in the message. Fix the node or revert
the LynqForm change to move the rollout on.

Completion is recorded on the node itself: once the operator sees an updated node Ready for its
new spec, it sets the annotation `lynq.sh/rollout-completed-generation` to the form generation
(one write per node per rollout; the next update to a new form generation resets it to `""` in the
same write that updates the node, and a data-only update leaves it as it is). The verdict therefore survives an operator restart or leader change unchanged — a
completed node is not timed after a restart, and a node whose update never completed is timed from
its original update, not from the restart. Nodes created by an operator version without this
annotation count as completed; only their failures stall the rollout. After a rollback to such a
version and a re-upgrade, a node it moved to a newer form generation still carries the older
generation's marker, so that rollout counts as not completed and is timed.

Completion is observed at each hub sync (`syncInterval`). A node that becomes Ready and regresses
again within one sync of its update, before the operator sees it Ready, counts as not completed.

## Status

```yaml
status:
  observedGeneration: int64
  totalNodes: int32            # Total LynqNodes using this form
  readyNodes: int32            # LynqNodes with Ready=True

  rollout:                     # Only populated when maxSkew > 0
    phase: string              # Idle | InProgress | Failed | Complete (Failed: see progressDeadlineSeconds)
    targetGeneration: int64
    totalNodes: int32
    updatedNodes: int32        # Updated to target generation
    updatingNodes: int32       # Updated, update not completed yet (not counting recovering nodes)
    readyUpdatedNodes: int32   # Updated AND ready for the updated spec now (observedGeneration caught up)
                               # updatedNodes - updatingNodes - readyUpdatedNodes = recovering nodes:
                               # completed the update, not Ready now. Complete counts them as done.
    startTime: timestamp
    completionTime: timestamp  # When the rollout completed, or when it turned Failed
    message: string            # On Failed: the stalled nodes and why; otherwise counts, incl. recovering

  conditions:
  - type: Valid
    status: "True" | "False"
    reason: ValidationSucceeded | ValidationFailed
    message: string
  - type: Applied
    status: "True" | "False"
    reason: string
    message: string
```

## Validation

The admission webhook enforces:

* `spec.hubId` must reference an existing LynqHub in the same namespace
* Each `TResource.id` must be unique within the form
* `dependIds` must reference IDs that exist within the same form
* `dependIds` must not form cycles
* `nameTemplate` and `labelsTemplate`/`annotationsTemplate` must be valid Go templates

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

* [Templates](templates.md) — template syntax, functions, and variable reference
* [Policies](policies.md) — policy behavior and recommended combinations
* [Dependencies](dependencies.md) — dependency ordering and failure handling
* [LynqHub API](api-lynqhub.md) — datasource and sync configuration
* [LynqNode API](api-lynqnode.md) — instance status and lifecycle
* [API index](api.md) — common types and kubectl reference
