---
description: "Define resource ordering with dependency graphs in Lynq. Learn about DAG-based topological sorting, cycle detection, and the interactive dependency visualizer."
---

# Dependency Management Guide

Resource ordering and dependency graphs in Lynq.

[[toc]]

## Overview

Lynq uses a DAG (Directed Acyclic Graph) to order resource creation and ensure dependencies are satisfied before applying resources.

## Dependency Visualizer

The Lynq includes an interactive dependency graph visualizer tool that helps you:

- **Visualize Dependencies**: See the complete dependency graph of your LynqForm
- **Detect Cycles**: Automatically identify circular dependencies that would cause failures
- **Understand Execution Order**: View numbered badges showing the order resources will be applied
- **Test Your Templates**: Paste your YAML and analyze dependencies before deployment

::: tip Interactive Tool Available
Visit the **[🔍 Dependency Visualizer](./dependency-visualizer.md)** page to analyze your LynqForm dependencies interactively. Load preset examples or paste your own YAML to visualize the dependency graph in real-time.
:::

### How to Use the Visualizer

**Step 1: Navigate to the Visualizer**

Open the [Dependency Visualizer](/dependency-visualizer) page in the documentation.

**Step 2: Load or Paste Your LynqForm**

```yaml
# Option A: Use preset examples from the dropdown
# Option B: Paste your own LynqForm YAML

apiVersion: lynq.sh/v1
kind: LynqForm
metadata:
  name: my-app
spec:
  hubId: customer-hub
  secrets:
    - id: db-creds
  deployments:
    - id: db
      dependIds: ["db-creds"]
    - id: app
      dependIds: ["db"]
  services:
    - id: app-svc
      dependIds: ["app"]
```

**Step 3: Analyze the Graph**

The visualizer will show:

```
+-------------+
| db-creds    |  ← Order: 1 (no dependencies)
+-------------+
      |
      v
+-------------+
| db          |  ← Order: 2 (after db-creds)
+-------------+
      |
      v
+-------------+
| app         |  ← Order: 3 (after db)
+-------------+
      |
      v
+-------------+
| app-svc     |  ← Order: 4 (after app)
+-------------+
```

**Step 4: Identify Issues**

- **Red nodes/edges**: Cycle detected - must be fixed before deployment
- **Yellow warning**: Missing dependency reference
- **Numbers on nodes**: Execution order

**Step 5: Export or Copy**

Copy the corrected YAML back to your LynqForm manifest.

## Defining Dependencies

Use the `dependIds` field to specify dependencies:

::: info Syntax
Set `dependIds` to an array of resource IDs. The controller ensures all referenced resources finish reconciling before applying the dependent resource.
:::

```yaml
deployments:
  - id: app
    dependIds: ["secret"]  # Wait for secret first
    nameTemplate: "{{ .uid }}-app"
    spec:
      # ... deployment spec
```

## Dependency Resolution

### Topological Sort

Resources are applied in dependency order:

```
secret (no dependencies)
  ↓
app (depends on: secret)
  ↓
svc (depends on: app)
```

### Cycle Detection

Circular dependencies are rejected:

<DependencyAnimationCycle />

::: warning Why it fails
Dependency resolution uses a DAG. Any cycle blocks reconciliation and surfaces as `DependencyError`.
:::

```yaml
# ❌ This will fail
- id: a
  dependIds: ["b"]
- id: b
  dependIds: ["a"]
```

Error: `DependencyError: Dependency cycle detected: a -> b -> a`

#### Refactoring Circular Dependencies

**Real-world example:** App and DB have circular config dependency

::: v-pre

```yaml
# ❌ BEFORE: Circular dependency
deployments:
  - id: db
    dependIds: ["app-config"]  # DB needs app config for connection pooling
    spec:
      containers:
        - name: postgres
          env:
            - name: MAX_CONNECTIONS
              valueFrom:
                configMapKeyRef:
                  name: "{{ .uid }}-app-config"
                  key: max_connections

configMaps:
  - id: app-config
    dependIds: ["db"]  # Config needs DB host info (circular!)
    spec:
      data:
        database_host: "{{ .uid }}-db-svc"
        max_connections: "100"
```

**Error message:**
```bash
$ kubectl describe lynqnode acme-customer-web-app
Events:
  Type     Reason           Age   Message
  ----     ------           ----  -------
  Warning  DependencyError  5s    Dependency cycle detected: db -> app-config -> db
```

**Solution:** Break the cycle by removing unnecessary dependency

```yaml
# ✅ AFTER: Acyclic graph
configMaps:
  - id: app-config
    # Removed: dependIds: ["db"]  ← Config doesn't actually need DB to exist first!
    spec:
      data:
        database_host: "{{ .uid }}-db-svc"  # This is just a name, not requiring DB to exist
        max_connections: "100"

deployments:
  - id: db
    dependIds: ["app-config"]  # DB still waits for config
    spec:
      containers:
        - name: postgres
          env:
            - name: MAX_CONNECTIONS
              valueFrom:
                configMapKeyRef:
                  name: "{{ .uid }}-app-config"
                  key: max_connections
```

**Why this works:** The ConfigMap contains the DB service name as a string (`{{ .uid }}-db-svc`), which doesn't require the DB to actually exist. The name is predictable from the template.

:::

**Visualization after fix:**
```
+-------------+
| app-config  |  ← Order: 1
+-------------+
      |
      v
+-------------+
| db          |  ← Order: 2
+-------------+
```

## Common Patterns

### Pattern 1: Secrets Before Apps

```yaml
secrets:
  - id: api-secret
    nameTemplate: "{{ .uid }}-secret"
    # No dependencies

deployments:
  - id: app
    dependIds: ["api-secret"]  # Wait for secret
```

### Pattern 2: ConfigMap Before Deployment

```yaml
configMaps:
  - id: app-config
    nameTemplate: "{{ .uid }}-config"

deployments:
  - id: app
    dependIds: ["app-config"]  # Wait for configmap
```

### Pattern 3: App Before Service

```yaml
deployments:
  - id: app
    # No dependencies

services:
  - id: svc
    dependIds: ["app"]  # Wait for deployment first
```

### Pattern 4: PVC Before StatefulSet

```yaml
persistentVolumeClaims:
  - id: data-pvc
    # No dependencies

statefulSets:
  - id: stateful-app
    dependIds: ["data-pvc"]  # Wait for PVC
```

## Dependency Failure Behavior

### `skipOnDependencyFailure` (Default: true)

Controls whether a resource should be skipped when any of its dependencies **fail** (apply error, timeout).

::: info New in v1.1.14
The `skipOnDependencyFailure` flag provides fine-grained control over dependency failure handling.
:::

```yaml
deployments:
  - id: app
    dependIds: ["db"]
    skipOnDependencyFailure: true  # Default: skip if db fails
```

**Behavior:**
- `true` (default): Resource is **skipped** if any dependency fails (apply error or timeout)
- `false`: Resource is **still created** even if dependencies fail

### Understanding Blocking vs Failed Dependencies

::: tip Important Distinction (v1.1.14+)
Lynq distinguishes between dependencies that are **not ready yet** (still progressing) and those that have **failed** (encountered an error).
:::

| Dependency State | Dependent Resource | DependencySkipped Event | skippedCount |
|------------------|-------------------|------------------------|--------------|
| **Failed** (apply error) | Skipped or created (based on `skipOnDependencyFailure`) | ✅ Yes (if skipped) | +1 (if skipped) |
| **Timed out** (exceeds `timeoutSeconds`) | Skipped or created (based on `skipOnDependencyFailure`) | ✅ Yes (if skipped) | +1 (if skipped) |
| **Not ready yet** (within timeout) | Blocked (waits for next reconcile) | ❌ No | 0 |

**Why this matters:**

When a Deployment is starting up (pulling images, scheduling pods), it's "not ready yet" but **not failed**. Dependent resources:
- Are **blocked** from creation until the Deployment becomes ready
- Do **NOT** receive a `DependencySkipped` event (which would be misleading)
- Are **NOT** counted in `skippedResources` status

This prevents confusing alerts like "DependencySkipped: app skipped because db failed" when `db` is simply still starting up.

**Example timeline:**

```
T=0s:  Deployment created, status: Progressing
       ConfigMap blocked (dependency not ready yet)
       NO DependencySkipped event

T=30s: Reconcile runs, Deployment still Progressing
       ConfigMap still blocked
       NO DependencySkipped event

T=45s: Deployment becomes Ready
       Reconcile runs, ConfigMap created
       SUCCESS - no skip events emitted
```

Compare this to a **failed** dependency:

```
T=0s:  Deployment created with invalid image
T=20s: Deployment fails (ImagePullBackOff, timeout exceeded)
       DependencySkipped event emitted for ConfigMap
       skippedResources: 1
```

### Concrete kubectl Output: Blocked vs Failed

**Scenario: DB Deployment → App Deployment → Service chain**

#### State 1: Blocked (Waiting for dependency to become ready)

```bash
$ kubectl describe lynqnode acme-customer-web-app -n lynq-system

Status:
  Conditions:
    - Type: Ready
      Status: "False"
      Reason: WaitingForDependencies
      Message: "Waiting for 1 resource(s) to become ready"
  Desired Resources: 3
  Ready Resources:   1    # Only db-creds is ready
  Failed Resources:  0    # Nothing failed!
  Skipped Resources: 0    # Nothing skipped!

Events:
  Type    Reason           Age   Message
  ----    ------           ----  -------
  Normal  Reconciling      30s   Starting reconciliation
  Normal  ResourceApplied  28s   Applied Secret/acme-db-creds
  Normal  ResourceApplied  25s   Applied Deployment/acme-db
  # Note: NO events for app or svc - they're silently blocked

# Check the blocking dependency
$ kubectl get deployment acme-db -o jsonpath='{.status.conditions[*].type}'
Progressing Available

$ kubectl get deployment acme-db -o jsonpath='{.status.availableReplicas}'
0  # ← Still starting up, not yet available
```

**Key indicator:** No `DependencySkipped` events, `skippedResources: 0`

#### State 2: Failed (Dependency encountered an error)

```bash
$ kubectl describe lynqnode acme-customer-web-app -n lynq-system

Status:
  Conditions:
    - Type: Ready
      Status: "False"
      Reason: DependencyFailed
    - Type: Degraded
      Status: "True"
      Reason: ResourceFailed
      Message: "Deployment acme-db failed: ImagePullBackOff"
  Desired Resources: 3
  Ready Resources:   1
  Failed Resources:  1     # DB failed!
  Skipped Resources: 2     # App and Svc were skipped
  Skipped Resource Ids:
    - "app"
    - "svc"

Events:
  Type     Reason             Age   Message
  ----     ------             ----  -------
  Normal   Reconciling        2m    Starting reconciliation
  Normal   ResourceApplied    2m    Applied Secret/acme-db-creds
  Warning  ResourceFailed     90s   Deployment acme-db failed: ImagePullBackOff
  Warning  ReadinessTimeout   60s   Resource db not ready within 60s, marking as failed
  Warning  DependencySkipped  60s   Resource 'app' skipped because dependency 'db' failed
  Warning  DependencySkipped  60s   Resource 'svc' skipped because dependency 'app' skipped

# Check the failed dependency
$ kubectl get deployment acme-db
NAME      READY   UP-TO-DATE   AVAILABLE   AGE
acme-db   0/1     1            0           2m

$ kubectl get pods -l app=acme-db
NAME                      READY   STATUS             RESTARTS   AGE
acme-db-5f8b7c9d4-xyz12   0/1     ImagePullBackOff   0          2m
```

**Key indicator:** `DependencySkipped` events present, `skippedResources > 0`

#### Quick Diagnosis Table

| What You See | State | What to Do |
|--------------|-------|------------|
| `skippedResources: 0`, no events for dependent | **Blocked** | Wait - dependency is still starting |
| `DependencySkipped` event, `skippedResources > 0` | **Failed** | Fix the failed dependency first |
| `Ready: True` for all resources | **Success** | Everything is working |

### When to Skip (Default Behavior)

The default `skipOnDependencyFailure: true` is recommended for most resources:

```yaml
secrets:
  - id: db-creds
    # ... credentials for database

deployments:
  - id: db
    dependIds: ["db-creds"]
    waitForReady: true
    # skipOnDependencyFailure: true (default)

  - id: app
    dependIds: ["db"]
    # skipOnDependencyFailure: true (default)
    # App will be SKIPPED if db fails or is not ready
```

**Use when:**
- Resource would fail anyway without its dependency
- You want to prevent cascading failures
- Resource cannot function without its dependency

### When NOT to Skip

Set `skipOnDependencyFailure: false` for resources that should be created regardless of dependency status:

```yaml
deployments:
  - id: main-app
    dependIds: ["config"]
    waitForReady: true

  - id: cleanup-job
    dependIds: ["main-app"]
    skipOnDependencyFailure: false  # Create even if main-app fails
    spec:
      apiVersion: batch/v1
      kind: Job
      spec:
        template:
          spec:
            containers:
            - name: cleanup
              image: busybox
              command: ["sh", "-c", "echo 'Performing cleanup...'"]
            restartPolicy: OnFailure
```

**Use when:**
- Cleanup or fallback resources that must always run
- Monitoring/alerting resources
- Resources that can partially function without dependency
- Debug or diagnostic resources

### Status Tracking

When resources are skipped, the LynqNode status tracks them:

```yaml
status:
  desiredResources: 5
  readyResources: 2
  failedResources: 1
  skippedResources: 2            # Resources skipped due to dependency failures
  skippedResourceIds:            # Which specific resources were skipped
    - "app"
    - "svc"
```

**Events emitted:**
- `DependencySkipped`: When a resource is skipped due to dependency failure
- `DependencyFailedButProceeding`: When `skipOnDependencyFailure: false` and resource is created despite failure

### Examples

**Example 1: Database → App → Service Chain**

```yaml
deployments:
  - id: db
    waitForReady: true

  - id: app
    dependIds: ["db"]
    # If db fails, app is skipped (default behavior)

services:
  - id: svc
    dependIds: ["app"]
    # If app is skipped, svc is also skipped
```

**Example 2: Cleanup Job Always Runs**

```yaml
deployments:
  - id: main-app
    waitForReady: true

jobs:
  - id: cleanup-job
    dependIds: ["main-app"]
    skipOnDependencyFailure: false  # Always create
    creationPolicy: Once
```

**Example 3: Monitoring Independent of App**

```yaml
deployments:
  - id: app
    dependIds: ["config"]

  - id: metrics-exporter
    dependIds: ["app"]
    skipOnDependencyFailure: false  # Monitor even if app fails
```

## Readiness Gates

Use `waitForReady` to wait for resource readiness:

<DependencyAnimationWaitForReady />

::: tip Combine readiness and dependencies
`dependIds` only guarantees creation order. Enable `waitForReady` to ensure *ready* status before dependent workloads roll out.
:::

```yaml
deployments:
  - id: db
    waitForReady: true
    timeoutSeconds: 300

deployments:
  - id: app
    dependIds: ["db"]  # Wait for db to exist AND be ready
    waitForReady: true
```

## Best Practices

### 1. Shallow Dependencies

Prefer shallow dependency trees:

**Good (depth: 2):**
```
secret
  ├─ app
  │   └─ svc
  └─ db
      └─ db-svc
```

**Bad (depth: 5):**
```
secret → config → pvc → db → db-svc → app
```

### 2. Parallel Execution

Independent resources execute in parallel:

<DependencyAnimationParallel />

```yaml
deployments:
  - id: app-a
    dependIds: ["secret"]  # Both execute in parallel

  - id: app-b
    dependIds: ["secret"]  # Both execute in parallel
```

### 3. Minimal Dependencies

Only specify necessary dependencies:

**Good:**
```yaml
- id: app
  dependIds: ["secret"]  # Only essential dependency
```

**Bad:**
```yaml
- id: app
  dependIds: ["secret", "unrelated-resource"]  # Unnecessary wait
```

## Debugging Dependencies

### Common Errors

::: danger Cycle detected
```
DependencyError: Dependency cycle detected: a -> b -> c -> a
```
**Fix:** Remove or refactor at least one edge so the graph becomes acyclic.
:::

::: warning Missing dependency
```
DependencyError: Resource 'app' depends on 'missing-id' which does not exist
```
**Fix:** Ensure every `dependIds` entry references a defined resource ID.
:::

::: warning Readiness timeout
```
ReadinessTimeout: Resource db not ready within 300s
```
**Fix:** Increase `timeoutSeconds` or set `waitForReady: false` when readiness should not block dependent resources.
:::

::: info Dependency skipped (expected behavior)
```
DependencySkipped: Resource 'app' skipped because dependency 'db' failed.
```
**Note:** This event is emitted **only when a dependency actually fails** (apply error or timeout), not when it's simply not ready yet. This is expected when `skipOnDependencyFailure: true` (default). Set to `false` if the resource should be created anyway.
:::

::: tip Dependency blocked (silent waiting)
If your dependent resource isn't being created but there's **no DependencySkipped event**, the dependency is likely still starting up. Lynq silently blocks dependent resources until dependencies become ready. Check:
```bash
# Check if dependency is still progressing
kubectl get deployment <dep-name> -o jsonpath='{.status.conditions}'
```
The dependent resource will be created on the next reconcile after the dependency becomes ready.
:::

::: info Dependency failed but proceeding
```
DependencyFailedButProceeding: Creating resource 'cleanup' despite dependency 'app' failure (skipOnDependencyFailure: false)
```
**Note:** Resource is being created as configured. Ensure it can handle missing dependency.
:::

## See Also

- [🔍 Dependency Visualizer](dependency-visualizer.md) - Interactive tool for analyzing dependencies
- [Template Guide](templates.md)
- [Policies Guide](policies.md)
- [Troubleshooting Guide](troubleshooting.md)
