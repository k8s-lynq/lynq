---
title: "5 Kubernetes Lifecycle Mistakes and How Lynq Policies Prevent Them"
date: 2025-12-18
author: Tim Kang
github: selenehyun
description: Real-world problems that occur when managing Kubernetes resources at scale, and how Lynq's policy system prevents them.
tags:
  - Lynq
  - Kubernetes
  - Resource Lifecycle
  - Policies
sidebar: false
editLink: false
prev: false
next: false
---

# 5 Kubernetes Lifecycle Mistakes and How Lynq Policies Prevent Them

<BlogPostMeta />

Managing Kubernetes resources manually works fine at small scale. But when you're managing hundreds of similar resources - like in a multi-tenant SaaS platform - things break in unexpected ways.

This post covers five real problems that occur when managing resource lifecycles at scale, and how Lynq's policy system prevents each one.

## How Lynq Works (Quick Overview)

Before diving into the problems, here's a 30-second overview of Lynq:

<LynqFlowDiagram />

Lynq reads data from a database (LynqHub), applies templates (LynqForm) to each row, and creates Kubernetes resources (via LynqNode). When a row is added, resources are created. When a row is deleted, resources are cleaned up.

Simple enough. But "how" resources are created, updated, and deleted matters enormously at scale.

## Problem 1: The Init Job That Ran 47 Times

### What Happened

A team set up a database initialization Job in their template:

```yaml
jobs:
  - id: db-init
    nameTemplate: "{{ .uid }}-db-init"
    spec:
      apiVersion: batch/v1
      kind: Job
      spec:
        template:
          spec:
            containers:
            - name: init
              image: postgres:15
              command: ["psql", "-c", "CREATE DATABASE app;"]
            restartPolicy: Never
```

The Job ran successfully on the first deployment. Great!

Then someone updated the template to fix a typo in another resource. The Job ran again. And again. Every template change triggered the Job to run again, attempting to create a database that already existed.

Over a month, that Job ran 47 times across 200+ tenants.

### The Root Cause

By default, Lynq's `creationPolicy` is `WhenNeeded` - resources are reapplied whenever the template changes. This is correct for Deployments that should always reflect the latest spec. But for one-time Jobs, this behavior is catastrophic.

### The Solution: `creationPolicy: Once`

```yaml
jobs:
  - id: db-init
    creationPolicy: Once  # Run exactly once, never again
    nameTemplate: "{{ .uid }}-db-init"
    spec:
      # ...
```

With `Once`, Lynq creates the resource on first reconciliation and marks it with a `lynq.sh/created-once` annotation. Even if the template changes, this resource is never touched again.

<CreationPolicyVisualizer />

### When to Use Each Policy

| Scenario | Policy | Why |
|----------|--------|-----|
| Application Deployment | `WhenNeeded` | Should always match template |
| ConfigMap with settings | `WhenNeeded` | Settings should stay current |
| Database migration Job | `Once` | Should only run once |
| Bootstrap script | `Once` | Initialization is one-time |
| Certificate Secret | `Once` | Don't regenerate existing certs |

## Problem 2: The Customer Data That Vanished

### What Happened

A SaaS platform stored customer uploads in PersistentVolumeClaims. When a customer cancelled their subscription, the database row was deleted, which triggered Lynq to clean up all associated resources - including the PVC with 3 years of customer data.

The customer later renewed their subscription. Their data was gone.

### The Root Cause

By default, Lynq's `deletionPolicy` is `Delete` - when a LynqNode is removed, all associated resources are garbage collected via Kubernetes ownerReferences. This is correct for Deployments and Services, but devastating for data.

### The Solution: `deletionPolicy: Retain`

```yaml
persistentVolumeClaims:
  - id: customer-data
    deletionPolicy: Retain  # Never auto-delete, even if tenant is removed
    nameTemplate: "{{ .uid }}-data"
    spec:
      apiVersion: v1
      kind: PersistentVolumeClaim
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 100Gi
```

With `Retain`, Lynq uses label-based tracking instead of ownerReferences. When the LynqNode is deleted, the resource stays in the cluster with orphan markers:

```yaml
metadata:
  labels:
    lynq.sh/orphaned: "true"
  annotations:
    lynq.sh/orphaned-at: "2025-01-15T10:30:00Z"
    lynq.sh/orphaned-reason: "LynqNodeDeleted"
```

<DeletionPolicyVisualizer />

### Managing Orphaned Resources

Resources with `Retain` won't auto-delete, so you need a cleanup process:

```bash
# Find all orphaned resources
kubectl get pvc -A -l lynq.sh/orphaned=true

# Find resources orphaned more than 90 days ago (manual filtering)
kubectl get pvc -A -l lynq.sh/orphaned=true -o json | \
  jq '.items[] | select(.metadata.annotations["lynq.sh/orphaned-at"] < "2024-10-01")'
```

**Tip:** Set up a scheduled job to review orphaned resources periodically, apply your data retention policy, and delete what's no longer needed.

## Problem 3: The Silent Overwrite War

### What Happened

Team A used Lynq to manage application Deployments. Team B manually created a Deployment with the same name for testing. When Lynq reconciled, it silently took over Team B's Deployment, replacing their test configuration with the template spec.

Team B spent hours debugging why their changes kept disappearing.

### The Root Cause

With Server-Side Apply's default behavior, if you apply a resource, you become the owner of the fields you specified. If another system had different values for those fields, yours win.

### The Solution: `conflictPolicy: Stuck`

```yaml
deployments:
  - id: app
    conflictPolicy: Stuck  # Stop if someone else owns this resource
    nameTemplate: "{{ .uid }}-app"
    spec:
      # ...
```

With `Stuck` (the default), Lynq detects ownership conflicts and stops reconciliation for that resource. The LynqNode is marked as `Degraded`, and an event is emitted:

```
ResourceConflict: Resource conflict detected for default/acme-app
(Kind: Deployment, Policy: Stuck). Another controller owns this resource.
```

<ConflictPolicyVisualizer />

### When to Use `Force`

Sometimes you *want* Lynq to take over. For example, when migrating from another management system:

```yaml
deployments:
  - id: app
    conflictPolicy: Force  # Take ownership even if someone else has it
    nameTemplate: "{{ .uid }}-app"
    spec:
      # ...
```

With `Force`, Lynq uses Server-Side Apply with `force=true` to claim ownership. Use this deliberately, not as a default.

| Scenario | Policy | Why |
|----------|--------|-----|
| Normal operation | `Stuck` | Detect conflicts early |
| Migration from Helm/Kustomize | `Force` | Intentionally taking over |
| Shared resources | `Stuck` | Avoid stepping on others |
| Lynq is sole owner | `Force` | No other controllers involved |

## Problem 4: The HPA That Stopped Working

### What Happened

A team configured HorizontalPodAutoscaler to manage their Deployment replicas. Lynq's template specified `replicas: 3`. Every time Lynq reconciled, it reset replicas back to 3, fighting the HPA.

The HPA would scale to 10 replicas under load. Seconds later, Lynq would scale back to 3. The application oscillated wildly.

### The Root Cause

Server-Side Apply tracks field ownership. When Lynq applies a template with `replicas: 3`, it becomes the owner of that field. When HPA tries to update replicas, there's a conflict.

### The Solution: `patchStrategy` and Field Awareness

**Option 1: Don't specify replicas in the template**

The simplest fix is to not include the conflicting field:

```yaml
deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        # replicas: 3  # Don't specify - let HPA manage it
        selector:
          matchLabels:
            app: myapp
        template:
          # ...
```

**Option 2: Use `creationPolicy: Once`**

If you need an initial replica count but want HPA to manage it afterwards:

```yaml
deployments:
  - id: app
    creationPolicy: Once  # Set initial state, then hands-off
    nameTemplate: "{{ .uid }}-app"
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: 3  # Initial value only
        # ...
```

**Option 3: Use `ignoreFields` (v1.1.4+)**

For fine-grained control, ignore specific fields during reconciliation:

```yaml
deployments:
  - id: app
    ignoreFields:
      - path: "spec.replicas"  # Ignore this field during updates
    nameTemplate: "{{ .uid }}-app"
    spec:
      # ...
```

### PatchStrategy Options

Different strategies handle updates differently:

| Strategy | Behavior | Use Case |
|----------|----------|----------|
| `apply` (default) | Server-Side Apply with field ownership | Most resources |
| `merge` | Strategic merge patch, preserves unspecified fields | Legacy compatibility |
| `replace` | Full replacement, removes unspecified fields | ConfigMaps needing exact state |

## Problem 5: The Template Change That Brought Down Everything

### What Happened

An engineer updated the container image in a LynqForm template that managed 500 tenants. Within seconds, all 500 Deployments started rolling out simultaneously.

The container registry couldn't handle 500 concurrent image pulls. The Kubernetes API server was overwhelmed with update requests. Network bandwidth saturated.

The new image had a subtle bug. By the time anyone noticed, all 500 tenants were affected.

### The Root Cause

When a LynqForm template changes, Lynq reconciles all LynqNodes using that template. Without rate limiting, this happens all at once.

### The Solution: `maxSkew`

```yaml
apiVersion: lynq.sh/v1
kind: LynqHub
metadata:
  name: production-hub
spec:
  rollout:
    maxSkew: 10  # Only 10 nodes updating at any time
  source:
    # ...
```

With `maxSkew: 10`, Lynq updates at most 10 nodes concurrently. It waits for each batch's Pods to reach Ready state before starting the next batch.

<RolloutAnimation />

### Calculating maxSkew

Consider your infrastructure capacity:

| Factor | Consideration |
|--------|---------------|
| Registry bandwidth | How many concurrent pulls can it handle? |
| API server load | How many concurrent updates? |
| Acceptable blast radius | If new version is buggy, how many tenants can be affected before you notice? |
| Rollout time | 500 nodes with maxSkew=10 = 50 batches |

**Rule of thumb:** Start with 5-10% of total nodes. For 500 nodes, `maxSkew: 25-50` gives you time to detect issues before full rollout.

For a deep dive into how we implemented maxSkew and the edge cases we discovered, see [Implementing maxSkew: How to Safely Update Nodes at Scale](/blog/maxskew-implementation-lessons).

## The Five Policies Summary

Each problem has a corresponding policy solution:

| Problem | Default Behavior | Policy | Solution Value |
|---------|------------------|--------|----------------|
| Init Job runs repeatedly | `WhenNeeded` | `creationPolicy: Once` | One-time execution |
| Data deleted with tenant | `Delete` | `deletionPolicy: Retain` | Data preservation |
| Silent resource overwrite | Continue | `conflictPolicy: Stuck` | Conflict detection |
| HPA fights with template | Apply all fields | `ignoreFields` / `patchStrategy` | Field-level control |
| Blast radius on update | All at once | `maxSkew` | Gradual rollout |

## Key Takeaways

1. **Defaults are optimized for correctness, not safety.** `WhenNeeded` and `Delete` make sense for most resources, but can be dangerous for Jobs and data.

2. **Think about lifecycle at design time.** Retrofitting policies after an incident is painful. Ask "what should happen when this is deleted?" for every resource.

3. **Conflicts are features, not bugs.** `Stuck` policy makes conflicts visible. Silent overwrites are far worse than explicit failures.

4. **Rate limit everything at scale.** Any operation that runs against hundreds of resources needs throttling. `maxSkew` prevents self-inflicted outages.

5. **Test deletion scenarios.** Most teams test creation thoroughly but never test what happens when things are removed.

---

For complete policy documentation, see [Policies Guide](/policies). For more detailed examples with diagrams, see [Policy Examples](/policies-examples).

*Have you encountered lifecycle issues we didn't cover? Share your experience on [GitHub Discussions](https://github.com/k8s-lynq/lynq/discussions).*

<BlogPostFooter />
