---
description: "Diagnose Lynq issues by symptom: TLS errors, hub sync failures, nodes not provisioning, resource lifecycle problems, and performance."
---

# Troubleshooting

Organized by symptom. Find the section that matches what you observe, then follow the diagnosis steps.

## Quick Diagnosis

```
What do you see?
│
├─ Operator pod not starting / crashing
│  └─ → TLS / cert-manager
│
├─ Operator running, but no LynqNodes created
│  ├─ Hub status empty or desired=0
│  │  └─ → Hub sync failures
│  └─ Hub looks right, no nodes anyway
│     └─ → Hub sync failures (activate column)
│
├─ LynqNodes exist, resources not created
│  ├─ TemplateRenderError event
│  │  └─ → Template errors
│  ├─ ResourceConflict event
│  │  └─ → Conflict resolution
│  └─ DependencySkipped event
│     └─ → Dependency failures
│
├─ Resources exist but not ready
│  ├─ Degraded count > 0 but Failed count = 0
│  │  └─ → Resource degraded (steady-state)
│  └─ Failed count > 0
│     └─ → Readiness / degraded nodes
│
├─ LynqNode stuck Terminating
│  └─ → Stuck finalizer
│
└─ Slow provisioning / OOMKilled
   └─ → Performance
```

## TLS / cert-manager

**Symptom:** Operator pod crashes with:
```
open /tmp/k8s-webhook-server/serving-certs/tls.crt: no such file or directory
```

cert-manager is required. Lynq's admission webhooks need TLS certificates that cert-manager provisions automatically.

**Check cert-manager status:**

```bash
kubectl get pods -n cert-manager
kubectl get certificate -n lynq-system
kubectl describe certificate -n lynq-system
```

**Fix A — cert-manager not installed:**

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=300s -n cert-manager \
  deployment/cert-manager \
  deployment/cert-manager-webhook \
  deployment/cert-manager-cainjector
```

**Fix B — cert-manager installed but certificate not ready:**

```bash
kubectl logs -n cert-manager -l app=cert-manager | tail -30
kubectl rollout restart -n lynq-system deployment/lynq-controller-manager
```

## Hub Sync Failures

### No LynqNodes created at all

**Check hub status:**

```bash
kubectl get lynqhub <name> -n lynq-system -o yaml
```

A healthy hub shows:
```yaml
status:
  conditions:
  - type: Ready
    status: "True"
    reason: SyncSuccess
    message: "Successfully synced 5 nodes from database"
  desired: 10          # activeRows × referencingForms
  ready: 10
  referencingTemplates: 2
  lastSyncTime: "2024-01-15T10:30:00Z"
```

A failing hub shows:
```yaml
status:
  conditions:
  - type: Ready
    status: "False"
    reason: DatabaseConnectionFailed
    message: "Failed to connect to database: connection refused"
  desired: 0
  lastSyncTime: "2024-01-15T10:25:00Z"  # stale
```

**Check operator logs:**

```bash
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i "hub\|sync\|query\|error"
```

### Connection refused

```bash
# Test from inside the cluster
kubectl run mysql-test --rm -it --image=mysql:8 -- \
  mysql -h mysql.default.svc.cluster.local -u node_reader -p

# Verify the Secret
kubectl get secret mysql-credentials -n lynq-system \
  -o jsonpath='{.data.password}' | base64 -d

# Check network reachability from operator
kubectl exec -n lynq-system deployment/lynq-controller-manager -- \
  nc -zv <mysql-host> 3306
```

### Active rows exist but desired=0

```bash
# Check what Lynq sees in the activate column
kubectl exec -it deployment/mysql -n <ns> -- \
  mysql -u node_reader -p -e "SELECT node_id, is_active FROM node_configs LIMIT 10"
```

Accepted truthy values for `activate`: `1`, `true`, `TRUE`, `True`, `yes`, `YES`, `Yes`. Any other value — including string `"active"` — is treated as inactive. Use a MySQL VIEW to normalize. See [Datasource Views](datasource-views.md).

### Wrong desired count (multi-form)

```bash
kubectl get lynqhub <name> -o jsonpath='{.status}'
# "referencingTemplates" should match number of forms pointing at this hub

kubectl get lynqforms -o jsonpath='{.items[*].spec.hubId}'
# All forms should match the hub name exactly
```

## Template Errors

**Symptom:** `TemplateRenderError` event, no resources created.

```bash
kubectl describe lynqnode <name>
# Events section — look for TemplateRenderError
```

**Common causes:**

| Log message | Cause | Fix |
|-------------|-------|-----|
| `map has no entry for key "planId"` | Template uses `.planId` but it's not in `extraValueMappings` | Add `planId: <column>` to `extraValueMappings` |
| `failed to parse template` | Invalid Go template syntax | Check for unmatched `{{}}` or missing `end` |
| `function "myFunc" not defined` | Unknown template function | Use a [supported function](templates-syntax.md) |

**Verify variables reach the node:**

```bash
kubectl get lynqnode <name> -o jsonpath='{.metadata.annotations}' | jq
# Look for lynq.sh/uid and any custom variable annotations
```

## Conflict Resolution

**Symptom:** LynqNode shows `DEGRADED=true`; `ResourceConflict` event.

```bash
kubectl describe lynqnode <name>
# "Deployment default/acme-app managed by 'helm', not 'lynq'"
```

**Find the owner:**

```bash
kubectl get deployment <resource-name> -o yaml | grep -A10 managedFields
```

**Resolution options:**

| Option | When to use | Action |
|--------|-------------|--------|
| Delete the resource | It should be Lynq-managed going forward | `kubectl delete deployment <name>` |
| Switch to `Force` | Lynq should take ownership | Edit LynqForm `conflictPolicy: Force` |
| Rename the resource | Keep both resources | Change `nameTemplate` to a unique value |
| Remove from LynqForm | Keep the existing resource, stop managing | Remove the block from the form |

After choosing a resolution, trigger reconciliation:

```bash
kubectl annotate lynqnode <name> -n lynq-system \
  lynq.sh/force-reconcile=$(date +%s) --overwrite
```

See [Policy Operations](policies-operations.md) for step-by-step conflict runbooks.

## Dependency Failures

**Symptom:** `DependencySkipped` event; some resources not created.

```bash
kubectl describe lynqnode <name>
kubectl get lynqnode <name> -o jsonpath='{.status.skippedResources}'
```

**Two distinct states:**

- **Blocked (no event):** A dependency is still starting up. Lynq silently waits — this is normal. Check the dependency's own status:
  ```bash
  kubectl get deployment <dep-name> -o jsonpath='{.status.conditions[?(@.type=="Available")].status}'
  ```

- **Skipped (DependencySkipped event):** A dependency actually failed (apply error or timeout). Fix the failing dependency first, or set `skipOnDependencyFailure: false` on the dependent resource to let it proceed regardless.

## Resource Degraded (Steady-State)

**Symptom:** `kubectl get lynqnode` shows `Degraded > 0` but `Failed == 0`; LynqNode `Ready=True`; `Degraded=True` with reason `ResourcesDegraded` or `ResourceFailuresAndDegraded`.

This is **NOT a Lynq failure**. The workload's rollout completed for the current generation, but availability has since dropped — typically a node drain, HPA scale-up, pod eviction, image GC on a cold node, or kubelet restart. Kubernetes is converging the workload; Lynq is just surfacing the observation.

```bash
# Identify the degraded resources
kubectl get lynqnode <name> -o jsonpath='{.status.degradedResourceIds}'

# Inspect the specific resources' phase + reason
kubectl get lynqnode <name> -o jsonpath=\
'{range .status.resourcePhases[?(@.phase=="Degraded")]}{.id}{"\t"}{.kind}{"\t"}{.reason}{"\n"}{end}'

# For a degraded Deployment, drill into pods
kubectl get pods -l <deployment-selector> -o wide
kubectl describe pod <pending-or-evicted-pod>

# For a degraded DaemonSet, check node-level state
kubectl get nodes
kubectl describe node <affected-node>
```

**When to act:**
- `LynqNodeWorkloadDegraded` (Warning, fires at 15 min): investigate the workload; usually self-resolves.
- `LynqNodeWorkloadSeverelyDegraded` (Critical, fires at 30 min on a single resource): Kubernetes has not converged — likely a kubelet, scheduler, or node-level issue. Escalate.
- `LynqNodeWorkloadFlapping` (Warning): pods churning Available↔Degraded — check PodDisruptionBudgets, HPA tuning, node health.

If the workload SHOULD be considered Failed after sustained degradation, that's outside Lynq's responsibility — wire the Prometheus alert to your incident system. See [Resource Phases](resource-phases.md) for the design rationale.

## Readiness / Degraded Nodes (Lynq-attributed failure)

**Symptom:** `readyResources < desiredResources`; `Degraded=True` with reason `ResourcesNotReady` or `ResourceFailures`; `Failed > 0`.

```bash
kubectl get lynqnode <name> -o jsonpath='{.status.readyResources}/{.status.desiredResources}'
kubectl get events --field-selector involvedObject.name=<name> --sort-by='.lastTimestamp'
```

**Check the specific resource:**

```bash
# Deployments
kubectl get deployment <name> -o jsonpath='{.spec.replicas} desired, {.status.availableReplicas} available'
kubectl logs deployment/<name> --tail=30

# Jobs
kubectl get job <name> -o jsonpath='{.status.succeeded}'

# General pod status
kubectl get pods -l lynq.sh/node=<lynqnode-name>
kubectl describe pod <pod-name>
```

**Increase timeout if the resource is slow to start:**

```yaml
deployments:
  - id: app
    timeoutSeconds: 600  # default is 300
```

**The `Degraded` condition clears automatically** once all resources reach ready state. Status updates every 30 seconds (plus immediately on child resource changes).

### Symptom: `Resource not ready after timeout` with `elapsed` much larger than the resource has been actually unhealthy

```
Resource not ready after timeout, marking as failed
  id=core-api  name=acme-api-123  elapsed=38m18s  timeout=1m0s
```

The readiness timeout is measured from `lynq.sh/apply-start-time` on the child resource (not `creationTimestamp`). This annotation is stamped at the last actual apply and **preserved across reconciles when the spec is unchanged** (so a rolling update accumulates elapsed time correctly).

The trade-off: if a resource has been stable for hours/days and then becomes unhealthy due to an external event (pod eviction, node replacement, downstream dependency failure), `elapsed` reflects the time since the last spec change — not the time since the resource became unhealthy. With a short `timeoutSeconds`, the resource is marked FAILED on the very next reconcile after going unhealthy.

**Diagnose:**

```bash
# Compare apply-start-time vs. when the resource actually went unhealthy
kubectl get deploy <name> -o jsonpath='{.metadata.annotations.lynq\.sh/apply-start-time}{"\n"}'
kubectl get deploy <name> -o jsonpath='{range .status.conditions[*]}{.type}={.status} ({.reason}) lastTransitionTime={.lastTransitionTime}{"\n"}{end}'

# Are pods actually unhealthy now?
kubectl get pods -l app=<name> -o wide
kubectl describe pod <unhealthy-pod>
```

If pods are restarting (CrashLoopBackOff, liveness-probe failure, image pull failure, OOMKilled), fix the underlying issue. The Lynq timeout is a faithful signal that *something is wrong now*, not a Lynq bug.

**If the resource is large or rolling slowly**, raise `timeoutSeconds`:

```yaml
deployments:
  - id: app
    timeoutSeconds: 600  # default is 300
```

### Symptom: Burst of reconcile activity after every controller restart

On restart, every LynqNode the controller observes has `status.lastFullReconcileAt == nil` (from its first observation in this process). The controller treats nil as **"stamp `now` as the baseline; defer the first force-reapply by one full `ForceReapplyInterval`"** — explicitly so it does NOT force-reapply every node at once.

You'll briefly see one stamp-only status update per LynqNode during the warm-up window (status writes only — no child-resource writes). This is expected and harmless.

### Symptom: `kubectl edit` to a child resource doesn't auto-revert immediately

If your edit changes `metadata.generation` or alters `lynq.sh/applied-hash`, the next reconcile (within ~30s) re-applies the desired spec.

If your edit preserved `lynq.sh/applied-hash` (e.g. modified a field that Lynq doesn't manage), the annotation-based skip path matches and elides the apply. Drift is corrected on the next periodic force-reapply (`ForceReapplyInterval`, default 10 minutes). To trigger correction sooner, also strip `lynq.sh/applied-hash`:

```bash
kubectl annotate <kind>/<name> -n <ns> lynq.sh/applied-hash- --overwrite
```

## Stuck Finalizer

**Symptom:** LynqNode stuck in `Terminating`.

```bash
kubectl get lynqnode <name> -o jsonpath='{.metadata.finalizers}'
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Failed to delete"
```

Lynq's finalizer (`lynqnode.operator.lynq.sh/finalizer`) runs resource cleanup before the CR is removed. If the operator is crashing or the cleanup is failing, investigate the logs first.

**Force-remove the finalizer only as a last resort** (may leave orphaned resources):

```bash
kubectl patch lynqnode <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
```

## Orphaned Resources

**Symptom:** Resources remain after removing them from a LynqForm template.

This is **expected behavior** when `deletionPolicy: Retain` is set. Retained resources receive orphan markers and stay in the cluster intentionally.

```bash
# Find all retained resources
kubectl get all -A -l lynq.sh/orphaned=true

# Check if a specific resource is orphaned by design
kubectl get deployment <name> -o jsonpath='{.metadata.labels.lynq\.sh/orphaned}'
```

If a resource should have been deleted but wasn't, check the LynqForm's `deletionPolicy`:

```bash
kubectl get lynqform <form-name> -o yaml | grep -A2 deletionPolicy
# Should be Delete, not Retain
```

Force reconciliation to pick up the change:

```bash
kubectl annotate lynqnode <name> -n lynq-system \
  lynq.sh/force-reconcile=$(date +%s) --overwrite
```

## Performance

### Slow provisioning

```bash
# Check reconciliation duration in logs
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation completed" | tail -20

# Disable readiness waits for fast resources (Services, ConfigMaps)
# waitForReady: false
```

Increase concurrency for large deployments:

```yaml
args:
- --node-concurrency=20
- --form-concurrency=10
- --hub-concurrency=5
```

### OOMKilled / high CPU

```bash
kubectl top pod -n lynq-system
kubectl logs -n lynq-system deployment/lynq-controller-manager --previous
```

Increase resource limits in `config/manager/manager.yaml`:

```yaml
resources:
  limits:
    cpu: 2000m
    memory: 2Gi
```

Or reduce concurrency to limit parallel work:

```yaml
args:
- --node-concurrency=5
- --form-concurrency=3
```

## Log Patterns Reference

```bash
# All errors
kubectl logs -n lynq-system deployment/lynq-controller-manager --tail=100 | grep -i error

# Failed reconciliations
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation failed"

# Template errors
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -E "(map has no entry|failed to render)"

# Conflict errors
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i conflict

# Dependency issues
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -E "(DependencySkipped|cycle detected)"

# Follow live
kubectl logs -n lynq-system deployment/lynq-controller-manager -f --timestamps
```

| Log pattern | Meaning | Fix |
|-------------|---------|-----|
| `map has no entry for key "X"` | Template uses undefined variable | Add `X` to `extraValueMappings` |
| `dependency cycle detected: A -> B -> A` | Circular dependency | Refactor to break cycle |
| `conflict: field manager lynq conflicts` | Another controller owns this field | Use `conflictPolicy: Force` or delete resource |
| `timed out waiting for resource` | Resource didn't become ready in time | Check pod logs, increase `timeoutSeconds` |
| `DependencySkipped` | Dependency failed; resource skipped | Fix the failing dependency |
| `failed to parse template` | Go template syntax error | Check for unmatched `{{}}`, missing `end` |

## Getting Help

1. `kubectl describe lynqnode <name>` — events are the fastest first signal
2. `kubectl get lynqhub <name> -o yaml` — check `status.conditions`
3. Operator logs (see patterns above)
4. [GitHub Issues](https://github.com/k8s-lynq/lynq/issues) — include operator version, K8s version, CRD YAML, and logs
