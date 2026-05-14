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
│  └─ → Readiness / degraded nodes
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

## Readiness / Degraded Nodes

**Symptom:** `readyResources < desiredResources`; `Degraded=True` with reason `ResourcesNotReady`.

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
