---
description: "Step-by-step runbooks for each Lynq Prometheus alert. Covers diagnosis, root cause analysis, and resolution for every alert rule."
---

# Alert Runbooks

Diagnosis and resolution steps for every Lynq Prometheus alert. Each runbook identifies the root cause, lists confirmation commands, and provides a remediation path.



## Overview

Runbooks for all Lynq alerts, organized by severity. Each runbook includes diagnosis commands, resolution steps, and a verification check to confirm the alert has cleared.

Alert names match `config/prometheus/alerts.yaml`.

| Severity | Alerts |
|----------|--------|
| Critical | `LynqNodeDegraded`, `LynqNodeResourcesFailed`, `LynqNodeNotReady`, `LynqNodeStatusUnknown`, `HubManyNodesFailure` |
| Warning | `LynqNodeResourcesMismatch`, `LynqNodeResourcesConflicted`, `LynqNodeHighConflictRate`, `HubNodesFailure`, `HubDesiredCountMismatch`, `LynqNodeReconciliationErrors`, `LynqNodeReconciliationSlow`, `HighApplyFailureRate` |
| Info | `LynqNodeNewConflictsDetected` |

---

## Critical Alerts

### LynqNodeDegraded

**Alert Name:** `LynqNodeDegraded`
**Severity:** Critical
**Threshold:** `lynqnode_degraded_status > 0` for 5+ minutes

#### Description

LynqNode CR has entered a degraded state, indicating the operator cannot successfully reconcile the node's resources. This is a critical condition preventing normal node operation.

#### Symptoms

- LynqNode's `Ready` condition is `False`
- `status.degraded` shows specific degradation reason
- Resources may be partially applied or stuck
- Events show template, conflict, or dependency errors

#### Possible Causes

1. **Template Rendering Errors** - Missing variables, syntax errors, type conversion failures
2. **Dependency Cycles** - Circular dependencies between resources
3. **Resource Conflicts** - Resources exist with different owners (`ConflictPolicy: Stuck`)
4. **External Dependencies** - Missing ConfigMaps, Secrets, or Namespaces
5. **RBAC Issues** - Operator lacks permissions

#### Diagnosis

```bash
# Check node status
kubectl get lynqnode <lynqnode-name> -n <namespace> -o yaml

# Review events
kubectl describe lynqnode <lynqnode-name> -n <namespace>

# Check operator logs
kubectl logs -n lynq-system -l control-plane=controller-manager --tail=100

# Validate template variables
kubectl get lynqnode <lynqnode-name> -n <namespace> -o jsonpath='{.metadata.annotations}'
```

#### Resolution

**For Template Errors:**
```bash
# Check LynqHub extraValueMappings
kubectl get lynqhub <hub-name> -o yaml

# Verify LynqForm syntax
kubectl get lynqform <template-name> -o yaml
```

**For Dependency Cycles:**
```bash
# Review dependIds configuration
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.spec.*.dependIds}'

# Remove circular dependencies in template
kubectl edit lynqform <template-name>
```

**For Resource Conflicts:**
```bash
# Check conflicting resources
kubectl get <resource-type> <resource-name> -o yaml | grep -A 10 metadata

# Option 1: Delete conflicting resource
kubectl delete <resource-type> <resource-name>

# Option 2: Change ConflictPolicy to Force
kubectl edit lynqform <template-name>
```

#### Verification

```bash
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.conditions[?(@.type=="Degraded")].status}'
# Should return: False
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.readyResources}/{.status.desiredResources}'
# Should return equal values
```

---

### LynqNodeResourcesFailed

**Alert Name:** `LynqNodeResourcesFailed`
**Severity:** Critical
**Threshold:** `lynqnode_resources_failed > 0` for 5+ minutes

#### Description

Node has one or more resources that failed to apply or became unhealthy, indicating critical provisioning failure.

#### Symptoms

- `status.failedResources > 0`
- Specific resources not created or in error state
- Events show apply failures or timeout errors

#### Diagnosis

```bash
# Check failed resources count
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.failedResources}'

# List applied resources
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.appliedResources}'

# Check events for failures
kubectl get events --field-selector involvedObject.kind=LynqNode,involvedObject.name=<lynqnode-name>
```

#### Resolution

**For Apply Failures:**
```bash
# Check RBAC permissions
kubectl auth can-i create <resource-type> --as=system:serviceaccount:lynq-system:lynq

# Review resource spec in template
kubectl get lynqform <template-name> -o yaml
```

**For Readiness Timeouts:**
```bash
# Increase timeout
kubectl edit lynqform <template-name>
# Set: timeoutSeconds: 600

# Or disable readiness wait
# Set: waitForReady: false
```

#### Verification

```bash
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.failedResources}'
# Should return: 0
```

---

### LynqNodeNotReady

**Alert Name:** `LynqNodeNotReady`
**Severity:** Critical
**Threshold:** `lynqnode_condition_status{type="Ready"} == 0` for 15+ minutes

#### Description

Node has not reached Ready state for an extended period, indicating persistent provisioning issues.

#### Symptoms

- `Ready` condition is `False` for 15+ minutes
- Resources may be pending, creating, or failing health checks
- LynqNode status shows ongoing reconciliation

#### Diagnosis

```bash
# Check Ready condition
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'

# Check resource readiness
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.readyResources}/{.status.desiredResources}'

# Identify slow resources
kubectl get all -l lynq.sh/lynqnode=<lynqnode-name>
```

#### Resolution

```bash
# Check if resources are progressing
kubectl describe <resource-type> <resource-name>

# Review readiness probes
kubectl get pod <pod-name> -o yaml | grep -A 10 readinessProbe

# Check dependencies
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.spec.*.dependIds}'
```

#### Verification

```bash
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# Should return: True
```

---

### LynqNodeStatusUnknown

**Alert Name:** `LynqNodeStatusUnknown`
**Severity:** Critical
**Threshold:** `lynqnode_condition_status{type="Ready"} == 2` for 10+ minutes

#### Description

LynqNode status is Unknown, indicating potential controller or API server communication issues.

#### Symptoms

- `Ready` condition status is `Unknown`
- Status updates not propagating
- Controller may be unreachable or crashed

#### Diagnosis

```bash
# Check controller pods
kubectl get pods -n lynq-system

# Check controller logs
kubectl logs -n lynq-system -l control-plane=controller-manager --tail=100

# Check API server connectivity
kubectl get --raw /healthz
```

#### Resolution

```bash
# Restart controller if unhealthy
kubectl rollout restart deployment -n lynq-system lynq-controller-manager

# Check for resource pressure
kubectl top pods -n lynq-system

# Review recent changes
kubectl rollout history deployment -n lynq-system lynq-controller-manager
```

#### Verification

```bash
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# Should return: True (not Unknown)
kubectl get pods -n lynq-system
# All pods should be Running/Ready
```

---

### HubManyNodesFailure

**Alert Name:** `HubManyNodesFailure`
**Severity:** Critical
**Threshold:** `hub_failed > 5` or `hub_failed / hub_desired > 0.5` for 5+ minutes

#### Description

Hub has widespread node failures (>5 lynqnodes or >50% failure rate), indicating systemic issue affecting multiple lynqnodes.

#### Symptoms

- High number of failed lynqnodes in hub
- Multiple lynqnodes showing similar errors
- Pattern of failures across all lynqnodes

#### Diagnosis

```bash
# Check hub status
kubectl get lynqhub <hub-name> -o yaml

# List failed nodes
kubectl get lynqnodes -l operator.lynq.sh/hub=<hub-name> \
  --field-selector status.phase=Failed

# Check database connectivity
kubectl logs -n lynq-system -l control-plane=controller-manager | grep "database\|mysql"
```

#### Resolution

**For Database Issues:**
```bash
# Verify database connectivity
kubectl get secret <db-secret> -o yaml

# Check database availability
kubectl run mysql-test --rm -it --image=mysql:8 -- \
  mysql -h <db-host> -u <db-user> -p<db-password> -e "SELECT 1"

# Review hub sync interval
kubectl get lynqhub <hub-name> -o jsonpath='{.spec.source.syncInterval}'
```

**For Template Issues:**
```bash
# Check template validity
kubectl get lynqform -l operator.lynq.sh/hub=<hub-name>

# Review template syntax
kubectl get lynqform <template-name> -o yaml

# Validate template rendering
kubectl describe lynqnode <any-failed-node>
```

#### Verification

```bash
kubectl get lynqhub <hub-name> -o jsonpath='{.status.failed}'
# Should return: 0
kubectl get lynqhub <hub-name> -o jsonpath='{.status.ready}/{.status.desired}'
# Should return equal values
```

---

## Warning Alerts

### LynqNodeResourcesMismatch

**Alert Name:** `LynqNodeResourcesMismatch`
**Severity:** Warning
**Threshold:** `lynqnode_resources_ready != lynqnode_resources_desired` (no failures) for 15+ minutes

#### Description

LynqNode's ready resource count doesn't match desired count, but no failures are detected. Reconciliation may be stuck or slow.

#### Diagnosis

```bash
# Check resource counts
kubectl get lynqnode <lynqnode-name> -o jsonpath='Ready: {.status.readyResources}, Desired: {.status.desiredResources}, Failed: {.status.failedResources}'

# Check if resources are progressing
kubectl get all -l lynq.sh/lynqnode=<lynqnode-name>
```

#### Resolution

```bash
# Check for pending resources
kubectl get events --field-selector involvedObject.name=<resource-name>

# Verify dependencies are satisfied
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.spec.*.dependIds}'

# Force reconciliation
kubectl annotate lynqnode <lynqnode-name> operator.lynq.sh/reconcile="$(date +%s)" --overwrite
```

---

### LynqNodeResourcesConflicted

**Alert Name:** `LynqNodeResourcesConflicted`
**Severity:** Warning
**Threshold:** `lynqnode_resources_conflicted > 0` for 10+ minutes

#### Description

Node has resources in conflict state, usually indicating ownership conflicts with existing resources.

#### Diagnosis

```bash
# Check conflicted resources
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.conflictedResources}'

# Check conflict count
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status.resourcesConflicted}'

# Review conflict events
kubectl describe lynqnode <lynqnode-name> | grep Conflict
```

#### Resolution

```bash
# Identify conflicting resources
kubectl get events --field-selector reason=ResourceConflict

# Option 1: Delete conflicting resources
kubectl delete <resource-type> <resource-name>

# Option 2: Use unique naming
kubectl edit lynqform <template-name>
# Update nameTemplate: "{{ .uid }}-{{ .planId }}-app"

# Option 3: Change to Force policy
kubectl edit lynqform <template-name>
# Set: conflictPolicy: Force
```

---

### LynqNodeHighConflictRate

**Alert Name:** `LynqNodeHighConflictRate`
**Severity:** Warning
**Threshold:** `rate(lynqnode_conflicts_total[5m]) > 0.1` for 10+ minutes

#### Description

High rate of conflicts detected, indicating recurring ownership or naming issues.

#### Diagnosis

```bash
# Check conflict rate
kubectl get --raw /metrics | grep lynqnode_conflicts_total

# Identify conflict patterns
kubectl logs -n lynq-system -l control-plane=controller-manager | grep -i conflict
```

#### Resolution

```bash
# Review naming templates
kubectl get lynqform <template-name> -o yaml | grep nameTemplate

# Ensure unique names per node
# Use: nameTemplate: "{{ .uid }}-{{ sha1sum .host | trunc 8 }}-app"

# Consider Force policy if appropriate
kubectl patch lynqform <template-name> --type=merge -p '{"spec":{"deployments":[{"conflictPolicy":"Force"}]}}'
```

---

### HubNodesFailure

**Alert Name:** `HubNodesFailure`
**Severity:** Warning
**Threshold:** `0 < hub_failed <= 5` for 10+ minutes

#### Description

Hub has some failed nodes (1-5), indicating isolated provisioning issues.

#### Diagnosis

```bash
# List failed nodes
kubectl get lynqnodes -l operator.lynq.sh/hub=<hub-name> \
  | grep -v "True"

# Check specific node
kubectl describe lynqnode <failed-lynqnode-name>
```

#### Resolution

```bash
# Investigate individual node failures
kubectl logs -n lynq-system -l control-plane=controller-manager | grep <failed-lynqnode-name>

# Check node-specific data
kubectl get lynqnode <failed-lynqnode-name> -o yaml

# Verify database row
# (Connect to database and check node_id row)
```

---

### HubDesiredCountMismatch

**Alert Name:** `HubDesiredCountMismatch`
**Severity:** Warning
**Threshold:** `hub_ready != hub_desired` (no failures) for 20+ minutes

#### Description

Hub's ready node count doesn't match desired count, but no failures detected. Sync may be delayed.

#### Diagnosis

```bash
# Check hub status
kubectl get lynqhub <hub-name> -o jsonpath='Desired: {.status.desired}, Ready: {.status.ready}, Failed: {.status.failed}'

# List all lynqnodes
kubectl get lynqnodes -l operator.lynq.sh/hub=<hub-name>

# Check sync interval
kubectl get lynqhub <hub-name> -o jsonpath='{.spec.source.syncInterval}'
```

#### Resolution

```bash
# Force hub sync
kubectl annotate lynqhub <hub-name> operator.lynq.sh/sync="$(date +%s)" --overwrite

# Check database for new rows
# Verify activate=true for expected nodes

# Review hub controller logs
kubectl logs -n lynq-system -l control-plane=controller-manager | grep "registry.*<hub-name>"
```

---

### LynqNodeReconciliationErrors

**Alert Name:** `LynqNodeReconciliationErrors`
**Severity:** Warning
**Threshold:** Error rate `> 10%` for 10+ minutes

#### Description

High error rate in node reconciliations, indicating controller issues, API problems, or resource contention.

#### Diagnosis

```bash
# Check error rate
kubectl get --raw /metrics | grep 'lynqnode_reconcile_duration_seconds_count{result="error"}'

# Review controller logs for errors
kubectl logs -n lynq-system -l control-plane=controller-manager --tail=200 | grep -i error

# Check API server health
kubectl get --raw /healthz
kubectl get --raw /readyz
```

#### Resolution

```bash
# Check controller resource usage
kubectl top pods -n lynq-system

# Increase controller resources if needed
kubectl edit deployment -n lynq-system lynq-controller-manager

# Review concurrent reconciliation settings
kubectl get deployment -n lynq-system lynq-controller-manager -o yaml | grep concurrent
```

---

### LynqNodeReconciliationSlow

**Alert Name:** `LynqNodeReconciliationSlow`
**Severity:** Warning
**Threshold:** P95 duration `> 30s` for 15+ minutes

#### Description

Slow reconciliation detected (P95 > 30s), indicating performance issues, resource contention, or complex configurations.

#### Diagnosis

```bash
# Check reconciliation duration
kubectl get --raw /metrics | grep lynqnode_reconcile_duration_seconds

# Identify slow nodes
kubectl get lynqnodes --sort-by='.status.lastReconcileTime'

# Check for large templates
kubectl get lynqforms -o json | jq '.items[] | {name: .metadata.name, resources: (.spec | [.deployments, .services, .configMaps] | flatten | length)}'
```

#### Resolution

```bash
# Optimize template complexity
# - Reduce resource count per node
# - Use efficient dependency chains
# - Avoid unnecessary waitForReady

# Increase concurrency
kubectl patch deployment -n lynq-system lynq-controller-manager \
  --type=json -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--node-concurrency=20"}]'

# Consider sharding by namespace
# Deploy multiple operators with namespace filters
```

---

### HighApplyFailureRate

**Alert Name:** `HighApplyFailureRate`
**Severity:** Warning
**Threshold:** Apply failure rate `> 20%` for 10+ minutes

#### Description

High failure rate for resource applies, indicating template issues or RBAC permission problems.

#### Diagnosis

```bash
# Check apply metrics
kubectl get --raw /metrics | grep apply_attempts_total

# Identify failing resource types
kubectl logs -n lynq-system -l control-plane=controller-manager | grep "Failed to apply"

# Check RBAC for resource types
kubectl auth can-i create deployment --as=system:serviceaccount:lynq-system:lynq
```

#### Resolution

```bash
# Verify RBAC permissions
kubectl describe clusterrole lynq-role

# Add missing permissions
kubectl edit clusterrole lynq-role

# Validate resource templates
kubectl get lynqform <template-name> -o yaml

# Check for API deprecations
kubectl api-resources | grep <resource-kind>
```

---

## Info Alerts

### LynqNodeNewConflictsDetected

**Alert Name:** `LynqNodeNewConflictsDetected`
**Severity:** Info
**Threshold:** `increase(lynqnode_conflicts_total[5m]) > 0` for 2+ minutes

#### Description

New conflicts detected in the last 5 minutes. Informational alert for conflict awareness.

#### Diagnosis

```bash
# Check recent conflicts
kubectl get events --sort-by='.lastTimestamp' | grep Conflict | head -20

# View conflict details
kubectl describe lynqnode <lynqnode-name> | grep -A 5 Conflict
```

#### Resolution

If conflicts persist, escalate to LynqNodeResourcesConflicted or LynqNodeHighConflictRate resolution procedures.

## LynqNodeWorkloadDegraded (Warning)

**Symptom:** `lynqnode_resources_degraded > 0` for 15+ minutes.

A workload's rollout completed for the current generation, but availability has since dropped due to causes outside Lynq's rollout: node drain, HPA scale-up, pod eviction, image GC on a fresh node, or kubelet restart. **Kubernetes is converging the workload — this is NOT a Lynq failure.** The alert exists so operators can verify K8s actually converges.

### Diagnosis

```bash
# Which resource(s) are degraded?
kubectl get lynqnode <name> -o jsonpath='{.status.degradedResourceIds}'

# Per-resource detail with reason
kubectl get lynqnode <name> -o jsonpath=\
'{range .status.resourcePhases[?(@.phase=="Degraded")]}{.id}{"\t"}{.kind}{"\t"}{.reason}{"\n"}{end}'

# For a Deployment, check pod state
kubectl get pods -l app=<label> -o wide
kubectl describe pod <pending-pod>

# Node-level check
kubectl get nodes
kubectl describe node <node-with-pending-pod>
```

### Likely causes

- **Node drain** in progress (`kubectl drain` or cluster autoscaler scale-down)
- **HPA** just scaled up faster than pods could become Available
- **Image pull** on a cold node (esp. large images)
- **Kubelet restart** evicted pods briefly
- **PodDisruptionBudget** preventing rescheduling

### Resolution

Usually self-resolves — wait. Escalate to `LynqNodeWorkloadSeverelyDegraded` (30+ min) if it does not.

## LynqNodeWorkloadSeverelyDegraded (Critical)

**Symptom:** `lynqnode_resource_degraded_since_seconds > 1800` (any resource degraded > 30 min).

Kubernetes has not converged the workload. This is a node-level or scheduler-level problem.

### Diagnosis

```bash
# Identify the resource
kubectl get lynqnode <name> -o yaml | yq '.status.resourcePhases[] | select(.phase=="Degraded")'

# Pod-level check
kubectl get pods -l <selector> --field-selector=status.phase!=Running
kubectl describe pod <not-running-pod>
kubectl get events --field-selector involvedObject.name=<pod-name>
```

### Likely causes

- Persistent image pull failure
- Insufficient node resources (CPU/memory/PIDs)
- Missing PV / PVC binding failure
- Kubelet on a node is broken
- Scheduler can't satisfy nodeSelector / affinity / taints

### Resolution

Treat as a node/cluster incident, not a Lynq issue. Fix the underlying constraint and Lynq will pick up the recovery via watch.

## LynqNodeWorkloadFlapping (Warning)

**Symptom:** `rate(lynqnode_resource_phase_transitions_total{from="Available",to="Degraded"}[15m]) > 0.1` for 15+ minutes.

Pods are churning Available↔Degraded faster than 0.1/sec for a sustained period. Investigate workload thrash.

### Diagnosis

```bash
# Which kinds are flapping?
sum by (kind) (rate(lynqnode_resource_phase_transitions_total{from="Available",to="Degraded"}[15m]))

# Check recent events for restart loops
kubectl get events --sort-by='.lastTimestamp' -A | grep -E '(Killing|OOMKilled|CrashLoop)'
```

### Likely causes

- OOMKilled pods (readiness/liveness loop)
- Aggressive HPA tuning (rapid scale up/down)
- Faulty PodDisruptionBudget allowing too many simultaneous evictions
- Flaky readiness probe

## LynqNodeRolloutSlow (Warning)

**Symptom:** `histogram_quantile(0.95, ...lynqnode_resource_rollout_duration_seconds_bucket{result="complete"}...) > 300` for 30+ minutes — P95 rollouts exceed 5 minutes.

### Diagnosis

```bash
# Which kinds have the slow rollouts?
histogram_quantile(0.95,
  sum by (le, kind) (rate(lynqnode_resource_rollout_duration_seconds_bucket{result="complete"}[1h])))

# Recent rollouts (with their duration)
sum(rate(lynqnode_resource_rollout_duration_seconds_count{result="complete"}[1h]))
```

### Likely causes

- Slow readiness probe (`initialDelaySeconds` + checks taking too long)
- Slow image pull
- Insufficient resource requests preventing scheduling
- Slow startup probes / pre-stop hooks

## See Also

- [Resource Phases](resource-phases.md) — the 5-phase model these alerts are based on
- [Monitoring](monitoring.md) — metrics catalog, events reference, alert overview
- [Troubleshooting](troubleshooting.md) — symptom-based diagnosis (non-alert issues)
- [Performance](performance.md) — scaling and tuning
- [Prometheus alerts config](https://github.com/k8s-lynq/lynq/blob/main/config/prometheus/alerts.yaml)
