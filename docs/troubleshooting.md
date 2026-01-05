# Troubleshooting Guide

Common issues and solutions for Lynq.

[[toc]]

## Quick Diagnostic Decision Tree

Use this flowchart to quickly identify your issue:

```
LynqNode Not Working?
│
├─ Resources not creating at all?
│  ├─ Check: kubectl describe lynqnode <name>
│  │  ├─ TemplateRenderError event? → [Section 2: Template Issues]
│  │  ├─ ResourceConflict event? → [Section 2C: Resource Conflict]
│  │  ├─ DependencySkipped event? → [Section 2D: Dependencies]
│  │  └─ No events? → Check operator logs [Section 1]
│  │
│  └─ Check: kubectl get lynqhub <name>
│     ├─ Status empty? → [Section 3: Database Connection]
│     └─ desired=0? → [Section 9: Hub Not Syncing]
│
├─ Resources exist but not Ready?
│  ├─ readyResources < desiredResources? → [Section 12: ResourcesNotReady]
│  ├─ Pods in CrashLoopBackOff? → Check pod logs
│  └─ Pods pending? → Check resource quotas/node capacity
│
├─ LynqNode stuck Terminating?
│  └─ [Section 8: Finalizer Stuck]
│
└─ Performance issues?
   ├─ Slow provisioning? → [Section 6: Slow Provisioning]
   └─ OOMKilled? → [Section 7: Memory/CPU Issues]
```

## General Debugging

### Check Operator Status

```bash
# Check if operator is running
kubectl get pods -n lynq-system

# View operator logs
kubectl logs -n lynq-system deployment/lynq-controller-manager -f

# Check operator events
kubectl get events -n lynq-system --sort-by='.lastTimestamp'
```

### Check CRD Status

```bash
# List all LynqNode CRs
kubectl get lynqnodes --all-namespaces

# Describe a specific LynqNode
kubectl describe lynqnode <lynqnode-name>

# Get LynqNode status
kubectl get lynqnode <lynqnode-name> -o jsonpath='{.status}'
```

## Common Issues

### 1. Webhook TLS Certificate Errors

**Error:**
```
open /tmp/k8s-webhook-server/serving-certs/tls.crt: no such file or directory
```

**Cause:** Webhook TLS certificates not found. cert-manager is **REQUIRED** for all installations.

::: danger cert-manager Required
cert-manager v1.13.0+ is **REQUIRED** for ALL installations including local development. Webhooks provide validation and defaulting at admission time.
:::

**Diagnosis:**

```bash
# Check if cert-manager is installed
kubectl get pods -n cert-manager

# Check if Certificate resource exists
kubectl get certificate -n lynq-system

# Check Certificate details
kubectl describe certificate -n lynq-system

# Check if secret was created
kubectl get secret -n lynq-system | grep webhook-server-cert
```

**Solutions:**

**A. Install cert-manager** (if not installed):
```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=Available --timeout=300s -n cert-manager \
  deployment/cert-manager \
  deployment/cert-manager-webhook \
  deployment/cert-manager-cainjector

# Verify cert-manager is running
kubectl get pods -n cert-manager
```

**B. Restart operator** (after cert-manager is ready):
```bash
kubectl rollout restart -n lynq-system deployment/lynq-controller-manager

# Watch rollout status
kubectl rollout status -n lynq-system deployment/lynq-controller-manager
```

**C. Check Certificate issuance**:
```bash
# Check if Certificate is Ready
kubectl get certificate -n lynq-system

# If not ready, check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager

# Check if Issuer exists
kubectl get issuer -n lynq-system
```

### 2. LynqNode Not Creating Resources

**Symptoms:**
- LynqNode CR exists
- Status shows `desiredResources > 0`
- But `readyResources = 0`

**Diagnosis:**
```bash
# Check LynqNode status
kubectl get lynqnode <name> -o yaml

# Check events
kubectl describe lynqnode <name>

# Check operator logs
kubectl logs node-name>
```

**Common Causes:**

**A. Template Rendering Error**
```bash
# Look for: "Failed to render resource"
kubectl describe lynqnode <name> | grep -A5 "TemplateRenderError"
```

**What Success vs Failure Looks Like:**

::: code-group
```bash [✅ Success - Operator Logs]
2024-01-15T10:30:45.123Z INFO  controller.lynqnode  Starting reconciliation  {"name": "acme-web", "namespace": "production"}
2024-01-15T10:30:45.125Z INFO  controller.lynqnode  Template variables resolved  {"uid": "acme", "host": "acme.example.com"}
2024-01-15T10:30:45.130Z INFO  controller.lynqnode  Resource rendered successfully  {"kind": "Deployment", "name": "acme-api"}
2024-01-15T10:30:45.135Z INFO  controller.lynqnode  Resource rendered successfully  {"kind": "Service", "name": "acme-api"}
2024-01-15T10:30:45.200Z INFO  controller.lynqnode  SSA apply successful  {"kind": "Deployment", "name": "acme-api"}
2024-01-15T10:30:45.250Z INFO  controller.lynqnode  Reconciliation completed  {"name": "acme-web", "duration": "127ms", "ready": 2, "desired": 2}
```

```bash [❌ Failure - Template Error]
2024-01-15T10:30:45.123Z INFO  controller.lynqnode  Starting reconciliation  {"name": "acme-web", "namespace": "production"}
2024-01-15T10:30:45.125Z ERROR controller.lynqnode  Failed to render resource  {"kind": "Deployment", "error": "template: deployment:15:12: executing \"deployment\" at <.planId>: map has no entry for key \"planId\""}
2024-01-15T10:30:45.126Z INFO  controller.lynqnode  Reconciliation failed  {"name": "acme-web", "error": "template rendering error"}
```

```bash [❌ Failure - kubectl Events]
$ kubectl describe lynqnode acme-web
...
Events:
  Type     Reason               Age   From                    Message
  ----     ------               ----  ----                    -------
  Warning  TemplateRenderError  10s   lynqnode-controller     Failed to render Deployment: template: deployment:15:12: map has no entry for key "planId"
  Warning  ReconciliationFailed 10s   lynqnode-controller     Template rendering failed, skipping all resources
```
:::

Solution: Fix template syntax in LynqForm

**B. Missing Variable**
```bash
# Look for: "map has no entry for key"
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "missing"
```

Solution: Add missing variable to `extraValueMappings`

**C. Resource Conflict**
```bash
# Look for: "ResourceConflict"
kubectl describe lynqnode <name> | grep "ResourceConflict"
```

Solution: Delete conflicting resource or use `conflictPolicy: Force`

**D. Dependency Blocking vs Skipping (v1.1.14+)**

If some resources aren't being created, check if they're waiting for dependencies:

```bash
# Check for DependencySkipped events (actual failure)
kubectl describe lynqnode <name> | grep "DependencySkipped"

# Check skippedResources in status
kubectl get lynqnode <name> -o jsonpath='{.status.skippedResources}'
```

**Blocked (No Event):** If there's no `DependencySkipped` event, the dependency is simply still starting up. Lynq silently blocks dependent resources until dependencies become ready. This is normal behavior.

```bash
# Check dependency status
kubectl get deployment <dependency-name> -o jsonpath='{.status.conditions[?(@.type=="Available")].status}'
```

**Skipped (DependencySkipped Event):** If you see a `DependencySkipped` event, the dependency actually **failed** (apply error or timeout).

Solutions:
- For **blocked** (not ready): Wait for dependency to become ready, or check why it's not progressing
- For **skipped** (failed): Fix the failed dependency, or set `skipOnDependencyFailure: false` if the resource should be created anyway

See [Dependency Management Guide](dependencies.md#understanding-blocking-vs-failed-dependencies) for more details.

### 3. Database Connection Failures

**Error:**
```
Failed to query database: dial tcp: connect: connection refused
```

**What Success vs Failure Looks Like:**

::: code-group
```bash [✅ Success - Hub Syncing]
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i "hub\|mysql\|database"

2024-01-15T10:30:00.100Z INFO  controller.lynqhub  Starting sync  {"hub": "customer-hub", "namespace": "production"}
2024-01-15T10:30:00.150Z INFO  controller.lynqhub  Database connected  {"host": "mysql.default.svc.cluster.local", "database": "tenants"}
2024-01-15T10:30:00.200Z INFO  controller.lynqhub  Query executed  {"rows": 5, "query": "SELECT id AS uid..."}
2024-01-15T10:30:00.250Z INFO  controller.lynqhub  Active nodes found  {"count": 5, "templates": 2}
2024-01-15T10:30:00.300Z INFO  controller.lynqhub  LynqNodes reconciled  {"created": 0, "updated": 10, "deleted": 0}
2024-01-15T10:30:00.350Z INFO  controller.lynqhub  Sync completed  {"hub": "customer-hub", "desired": 10, "ready": 10}
```

```bash [❌ Failure - Connection Refused]
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i "hub\|mysql\|error"

2024-01-15T10:30:00.100Z INFO  controller.lynqhub  Starting sync  {"hub": "customer-hub", "namespace": "production"}
2024-01-15T10:30:05.100Z ERROR controller.lynqhub  Failed to connect to database  {"host": "mysql.wrong-namespace.svc.cluster.local", "error": "dial tcp 10.96.0.1:3306: connect: connection refused"}
2024-01-15T10:30:05.101Z ERROR controller.lynqhub  Sync failed  {"hub": "customer-hub", "error": "database connection failed"}
```

```bash [❌ Failure - Authentication Error]
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i "hub\|mysql\|error"

2024-01-15T10:30:00.100Z INFO  controller.lynqhub  Starting sync  {"hub": "customer-hub"}
2024-01-15T10:30:00.200Z ERROR controller.lynqhub  Database authentication failed  {"error": "Error 1045 (28000): Access denied for user 'lynq'@'10.244.0.15' (using password: YES)"}
```

```bash [❌ Failure - Query Error]
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i "hub\|query\|error"

2024-01-15T10:30:00.100Z INFO  controller.lynqhub  Starting sync  {"hub": "customer-hub"}
2024-01-15T10:30:00.150Z INFO  controller.lynqhub  Database connected
2024-01-15T10:30:00.200Z ERROR controller.lynqhub  Query execution failed  {"error": "Error 1054 (42S22): Unknown column 'tenant_id' in 'field list'", "query": "SELECT tenant_id AS uid..."}
```
:::

**Hub Status Comparison:**

::: code-group
```yaml [✅ Healthy Hub Status]
$ kubectl get lynqhub customer-hub -o yaml
status:
  conditions:
  - lastTransitionTime: "2024-01-15T10:30:00Z"
    message: "Successfully synced 5 nodes from database"
    reason: SyncSuccess
    status: "True"
    type: Ready
  desired: 10          # 5 rows × 2 templates
  ready: 10
  failed: 0
  referencingTemplates: 2
  lastSyncTime: "2024-01-15T10:30:00Z"
```

```yaml [❌ Failed Hub Status]
$ kubectl get lynqhub customer-hub -o yaml
status:
  conditions:
  - lastTransitionTime: "2024-01-15T10:30:05Z"
    message: "Failed to connect to database: connection refused"
    reason: DatabaseConnectionFailed
    status: "False"
    type: Ready
  desired: 0           # No data from DB
  ready: 0
  failed: 0
  referencingTemplates: 2
  lastSyncTime: "2024-01-15T10:25:00Z"  # Stale - last successful sync
```
:::

**Diagnosis:**
```bash
# Check secret exists
kubectl get secret <mysql-secret> -o yaml

# Check Hub status
kubectl get lynqhub <name> -o yaml

# Test database connection from a pod
kubectl run -it --rm mysql-test --image=mysql:8 --restart=Never -- \
  mysql -h <host> -u <user> -p<password> -e "SELECT 1"
```

**Solutions:**

A. Verify credentials:
```bash
kubectl get secret <mysql-secret> -o jsonpath='{.data.password}' | base64 -d
```

B. Check network connectivity:
```bash
kubectl exec -n lynq-system deployment/lynq-controller-manager -- \
  nc -zv <mysql-host> 3306
```

C. Verify LynqHub configuration:
```yaml
spec:
  source:
    mysql:
      host: mysql.default.svc.cluster.local  # Correct FQDN
      port: 3306
      database: nodes
```

### 4. LynqNode Status Not Updating

**Symptoms:**
- Resources are ready in cluster
- LynqNode status shows `readyResources = 0`

**Causes:**
- Reconciliation not triggered
- Readiness check failing

**Solutions:**

A. Force reconciliation:
```bash
# Add annotation to trigger reconciliation
kubectl annotate lynqnode <name> force-sync="$(date +%s)" --overwrite
```

B. Check readiness logic:
```bash
# For Deployments
kubectl get deployment <name> -o jsonpath='{.status}'

# Check if replicas match
kubectl get deployment <name> -o jsonpath='{.spec.replicas} {.status.availableReplicas}'
```

C. Wait longer (resources take time to become ready):
- Deployments: 30s - 2min
- Jobs: Variable
- Ingresses: 10s - 1min

### 5. Template Variables Not Substituting

::: v-pre
**Symptoms:**
- Template shows `{{ .uid }}` literally in resources
- Variables not replaced
:::

**Cause:** Templates not rendered correctly

**Diagnosis:**
```bash
# Check rendered LynqNode spec
kubectl get lynqnode <name> -o jsonpath='{.spec.deployments[0].nameTemplate}'
```

**Solution:**
- Ensure Hub has correct `valueMappings`
- Check database column names match mappings
- Verify node row has non-empty values

### 6. Slow LynqNode Provisioning

**Symptoms:**
- LynqNodes taking > 5 minutes to provision
- High operator CPU usage

**Diagnosis:**
```bash
# Check reconciliation times
kubectl logs -n lynq-system deployment/lynq-controller-manager | \
  grep "Reconciliation completed" | tail -20

# Check resource counts
kubectl get lynqnodes -o json | jq '.items[] | {name: .metadata.name, desired: .status.desiredResources}'
```

**Solutions:**

A. Disable readiness waits:
```yaml
waitForReady: false
```

B. Increase concurrency:
```yaml
args:
- --node-concurrency=20      # Increase LynqNode reconciliation concurrency
- --form-concurrency=10    # Increase Template reconciliation concurrency
- --hub-concurrency=5     # Increase Hub reconciliation concurrency
```

C. Optimize templates (see [Performance Guide](performance.md))

### 7. Memory/CPU Issues

**Symptoms:**
- Operator pod OOMKilled
- High CPU usage

**Diagnosis:**
```bash
# Check resource usage
kubectl top pod -n lynq-system

# Check for memory leaks
kubectl logs -n lynq-system deployment/lynq-controller-manager --previous
```

**Solutions:**

A. Increase resource limits:
```yaml
resources:
  limits:
    cpu: 2000m
    memory: 2Gi
```

B. Reduce concurrency:
```yaml
args:
- --node-concurrency=5       # Reduce LynqNode reconciliation concurrency
- --form-concurrency=3     # Reduce Template reconciliation concurrency
- --hub-concurrency=1     # Reduce Hub reconciliation concurrency
```

C. Increase requeue interval:
```yaml
args:
- --requeue-interval=1m
```

### 8. Finalizer Stuck

**Symptoms:**
- LynqNode CR stuck in `Terminating` state
- Can't delete LynqNode

**Diagnosis:**
```bash
# Check finalizers
kubectl get lynqnode <name> -o jsonpath='{.metadata.finalizers}'

# Check deletion timestamp
kubectl get lynqnode <name> -o jsonpath='{.metadata.deletionTimestamp}'
```

**Solutions:**

A. Check operator logs for deletion errors:
```bash
kubectl logs -n lynq-system deployment/lynq-controller-manager | \
  grep "Failed to delete"
```

B. Force remove finalizer (last resort):
```bash
kubectl patch lynqnode <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
```

**Warning:** This may leave orphaned resources!

### 9. Hub Not Syncing

**Symptoms:**
- Database has active rows
- No LynqNode CRs created

**Diagnosis:**
```bash
# Check Hub status
kubectl get lynqhub <name> -o yaml

# Check operator logs
kubectl logs -n lynq-system deployment/lynq-controller-manager | \
  grep "Hub"
```

**Common Causes:**

A. Incorrect `valueMappings`:
```yaml
# Must match database columns exactly
valueMappings:
  uid: node_id          # Column must exist
  hostOrUrl: node_url   # Column must exist
  activate: is_active     # Column must exist
```

B. No active rows:
```sql
-- Check for active nodes
SELECT COUNT(*) FROM nodes WHERE is_active = TRUE;
```

C. Database query error:
```bash
# Check logs for SQL errors
kubectl logs -n lynq-system deployment/lynq-controller-manager | \
  grep "Failed to query"
```

### 10. Multi-Form Issues

**Symptoms:**
- Expected 2× nodes, only seeing 1×
- Wrong desired count

**Diagnosis:**
```bash
# Check Hub status
kubectl get lynqhub <name> -o jsonpath='{.status}'

# Should show:
# referencingTemplates: 2
# desired: <forms> × <rows>

# Check forms reference same hub
kubectl get lynqforms -o jsonpath='{.items[*].spec.hubId}'
```

**Solution:**
Ensure all forms correctly reference the hub:
```yaml
spec:
  hubId: my-hub  # Must match exactly
```

### 11. Orphaned Resources Not Cleaning Up

**Symptoms:**
- Resources removed from LynqForm still exist in cluster
- `appliedResources` status not updating
- Unexpected resources with node labels/ownerReferences

**Diagnosis:**

```bash
# Check current applied resources
kubectl get lynqnode <name> -o jsonpath='{.status.appliedResources}'

# Should show: ["Deployment/default/app@deploy-1", "Service/default/app@svc-1"]

# List resources with node labels
kubectl get all -l lynq.sh/node=<lynqnode-name>

# Find orphaned resources (retained with DeletionPolicy=Retain)
kubectl get all -A -l lynq.sh/orphaned=true

# Find orphaned resources from this node
kubectl get all -A -l lynq.sh/orphaned=true,lynq.sh/node=<lynqnode-name>

# Check resource DeletionPolicy
kubectl get lynqform <name> -o yaml | grep -A2 deletionPolicy
```

**Common Causes:**

1. **DeletionPolicy=Retain**: Resource was intentionally retained and marked with orphan labels
2. **Status not syncing**: AppliedResources field not updated
3. **Manual resource modification**: OwnerReference or labels removed manually
4. **Operator version**: Upgrade from version without orphan cleanup

::: tip Expected Behavior
Resources with `DeletionPolicy=Retain` are **intentionally kept** in the cluster and marked with orphan labels for easy identification. This is not a bug - it's the designed behavior!
:::

**Solutions:**

**A. Verify DeletionPolicy:**
```yaml
# Check template definition
deployments:
  - id: old-deployment
    deletionPolicy: Delete  # Should be Delete, not Retain
```

**B. Force reconciliation:**
```bash
# Trigger reconciliation by updating an annotation
kubectl annotate lynqnode <name> force-sync="$(date +%s)" --overwrite

# Watch logs
kubectl logs -n lynq-system deployment/lynq-controller-manager -f
```

**C. Manual cleanup (if needed):**
```bash
# Delete orphaned resource manually
kubectl delete deployment <orphaned-resource>

# Or remove owner reference if you want to keep it
kubectl patch deployment <name> --type=json -p='[{"op": "remove", "path": "/metadata/ownerReferences"}]'
```

**D. Check status update:**
```bash
# Verify appliedResources is being updated
kubectl get lynqnode <name> -o jsonpath='{.status.appliedResources}' | jq

# Should reflect current template resources only
```

**Prevention:**

1. Use `deletionPolicy: Delete` for resources that should be cleaned up
2. Monitor `appliedResources` status field regularly
3. Test template changes in non-production first
4. Review orphan cleanup behavior in [Policies Guide](policies.md#orphan-resource-cleanup)

### 12. LynqNode Showing Degraded with ResourcesNotReady

::: tip New in v1.1.4
The `ResourcesNotReady` degraded condition provides granular visibility into resources that haven't reached ready state yet.
:::

**Symptoms:**
- LynqNode condition `Degraded=True` with reason `ResourcesNotReady`
- Not all resources showing as ready even though they exist
- `readyResources < desiredResources` in LynqNode status
- Ready condition shows `status=False` with reason `NotAllResourcesReady`

**Diagnosis:**

```bash
# Check LynqNode status
kubectl get lynqnode <name> -o jsonpath='{.status}' | jq

# Should show:
# "conditions": [
#   {"type": "Ready", "status": "False", "reason": "NotAllResourcesReady"},
#   {"type": "Degraded", "status": "True", "reason": "ResourcesNotReady"}
# ]

# Check resource readiness
kubectl get lynqnode <name> -o jsonpath='{.status.readyResources} / {.status.desiredResources}'

# Identify which resources are not ready
kubectl describe lynqnode <name>

# Check recent events
kubectl get events --field-selector involvedObject.name=<lynqnode-name> --sort-by='.lastTimestamp'
```

**Common Causes:**

1. **Resources still starting up**: Normal during initial provisioning
2. **Resource readiness checks failing**: Container failing health checks
3. **Dependency not satisfied**: Waiting for dependent resources
4. **Timeout exceeded**: Resource taking longer than `timeoutSeconds`
5. **Image pull errors**: Container images not available
6. **Insufficient resources**: Not enough CPU/memory in cluster

**Solutions:**

**A. Check resource status:**
```bash
# For Deployments
kubectl get deployment <name> -o jsonpath='{.status}' | jq

# Check if replicas match
kubectl get deployment <name> -o jsonpath='{.spec.replicas} desired, {.status.availableReplicas} available'

# For StatefulSets
kubectl get statefulset <name> -o jsonpath='{.status.readyReplicas}/{.spec.replicas} ready'

# For Jobs
kubectl get job <name> -o jsonpath='{.status.succeeded}'

# For Services (should be immediate)
kubectl get service <name>

# Check pod status
kubectl get pods -l app=<name>
kubectl describe pod <pod-name>
```

**B. Check resource logs:**
```bash
# Deployment pods
kubectl logs deployment/<name> --tail=50

# Job logs
kubectl logs job/<name>

# Check for errors
kubectl logs deployment/<name> --previous  # If pod crashed
```

**C. Check readiness probes:**
```bash
# See if readiness probes are configured correctly
kubectl get deployment <name> -o jsonpath='{.spec.template.spec.containers[*].readinessProbe}' | jq
```

**D. Adjust timeouts if needed:**
```yaml
# In LynqForm
deployments:
  - id: app
    timeoutSeconds: 600  # Increase from default 300s
    waitForReady: true   # Ensure readiness checks are enabled
    spec:
      template:
        spec:
          containers:
          - name: app
            readinessProbe:
              httpGet:
                path: /health
                port: 8080
              initialDelaySeconds: 10  # Allow time to start
              periodSeconds: 5
```

**E. Monitor reconciliation:**
```bash
# Watch LynqNode status updates (30-second interval in v1.1.4)
watch -n 5 'kubectl get lynqnode <name> -o jsonpath="{.status.readyResources}/{.status.desiredResources} ready"'

# Watch pod status
watch kubectl get pods -l lynq.sh/node=<name>
```

**Expected Behavior:**

::: tip v1.1.4+ Fast Status Updates
- **Status updates every 30 seconds** (down from 5 minutes in earlier versions)
- **Event-driven**: Immediate reconciliation when child resources change
- Resources typically become ready within 1-2 minutes (depending on resource type)
- Degraded condition clears automatically when all resources reach ready state
:::

**When to Investigate Further:**

- Resources stuck "not ready" for > 5 minutes
- `readyResources` count not increasing over time
- Events show repeated failures or errors
- Logs indicate application-level issues

**Prevention:**

1. Set realistic `timeoutSeconds` values for resource types (Deployments: 300s, Jobs: 600s)
2. Ensure resource specifications have correct readiness probes
3. Test templates in non-production environments first
4. Monitor `lynqnode_resources_ready` and `lynqnode_degraded_status` metrics
5. Use `kubectl wait` for pre-flight checks:
   ```bash
   kubectl wait --for=condition=Ready lynqnode/<name> --timeout=300s
   ```

## Debugging Workflows

### Debug Template Rendering

1. Create test LynqNode manually:
```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqNode
metadata:
  name: test-node
  annotations:
    lynq.sh/uid: "test-123"
    lynq.sh/host: "test.example.com"
spec:
  # ... copy from template
```

2. Check rendered resources:
```bash
kubectl get lynqnode -o yaml
```

3. Check operator logs:
```bash
kubectl logs -n lynq-system deployment/lynq-controller-manager -f
```

### Debug Database Connection

1. Create test pod:
```bash
kubectl run -it --rm mysql-test --image=mysql:8 --restart=Never -- bash
```

2. Inside pod:
```bash
mysql -h <host> -u <user> -p<password> <database> -e "SELECT * FROM nodes LIMIT 5"
```

### Debug Reconciliation

1. Enable debug logging:
```yaml
# config/manager/manager.yaml
args:
- --zap-log-level=debug
```

2. Watch reconciliation:
```bash
kubectl logs -n lynq-system deployment/lynq-controller-manager -f | \
  grep "Reconciling"
```

### Reconciliation Log Analysis: Success vs Failure Patterns

Understanding reconciliation logs helps quickly identify issues:

**Complete Healthy Reconciliation Cycle:**

```bash
$ kubectl logs -n lynq-system deployment/lynq-controller-manager --tail=50

# ✅ Phase 1: Start
2024-01-15T10:30:00.000Z INFO  controller.lynqnode  Reconciliation started  {"node": "acme-web", "namespace": "production"}

# ✅ Phase 2: Finalizer Check
2024-01-15T10:30:00.010Z DEBUG controller.lynqnode  Finalizer present  {"node": "acme-web"}

# ✅ Phase 3: Variable Resolution
2024-01-15T10:30:00.020Z DEBUG controller.lynqnode  Variables resolved  {"uid": "acme", "host": "acme.example.com", "planId": "enterprise"}

# ✅ Phase 4: Dependency Graph
2024-01-15T10:30:00.030Z DEBUG controller.lynqnode  Dependency graph built  {"nodes": 4, "edges": 3, "cycles": false}
2024-01-15T10:30:00.031Z DEBUG controller.lynqnode  Apply order determined  {"order": ["secret", "configmap", "deployment", "service"]}

# ✅ Phase 5: Orphan Cleanup
2024-01-15T10:30:00.040Z DEBUG controller.lynqnode  Checking for orphaned resources  {"previous": 4, "current": 4}
2024-01-15T10:30:00.041Z DEBUG controller.lynqnode  No orphaned resources found

# ✅ Phase 6: Resource Apply (each resource)
2024-01-15T10:30:00.100Z INFO  controller.lynqnode  Applying resource  {"kind": "Secret", "name": "acme-creds", "id": "creds"}
2024-01-15T10:30:00.150Z INFO  controller.lynqnode  Resource applied  {"kind": "Secret", "name": "acme-creds", "result": "configured"}

2024-01-15T10:30:00.200Z INFO  controller.lynqnode  Applying resource  {"kind": "ConfigMap", "name": "acme-config", "id": "config"}
2024-01-15T10:30:00.250Z INFO  controller.lynqnode  Resource applied  {"kind": "ConfigMap", "name": "acme-config", "result": "unchanged"}

2024-01-15T10:30:00.300Z INFO  controller.lynqnode  Applying resource  {"kind": "Deployment", "name": "acme-api", "id": "app"}
2024-01-15T10:30:00.400Z INFO  controller.lynqnode  Waiting for readiness  {"kind": "Deployment", "name": "acme-api", "timeout": "300s"}
2024-01-15T10:30:15.000Z INFO  controller.lynqnode  Resource ready  {"kind": "Deployment", "name": "acme-api", "waited": "14.6s"}

2024-01-15T10:30:15.100Z INFO  controller.lynqnode  Applying resource  {"kind": "Service", "name": "acme-api", "id": "svc"}
2024-01-15T10:30:15.150Z INFO  controller.lynqnode  Resource applied  {"kind": "Service", "name": "acme-api", "result": "configured"}

# ✅ Phase 7: Status Update
2024-01-15T10:30:15.200Z INFO  controller.lynqnode  Status updated  {"ready": 4, "desired": 4, "failed": 0, "skipped": 0}

# ✅ Phase 8: Complete
2024-01-15T10:30:15.250Z INFO  controller.lynqnode  Reconciliation completed  {"node": "acme-web", "duration": "15.25s", "result": "success"}
```

**Common Failure Patterns:**

| Log Pattern | Meaning | Solution |
|-------------|---------|----------|
| `map has no entry for key "X"` | Template uses undefined variable | Add `X` to `extraValueMappings` |
| `dependency cycle detected: A -> B -> A` | Circular dependency | Refactor to break cycle |
| `conflict: field manager lynq conflicts` | Another controller owns resource | Use `conflictPolicy: Force` or delete resource |
| `timed out waiting for resource` | Resource didn't become ready | Check pod logs, increase `timeoutSeconds` |
| `DependencySkipped` | Dependency failed, resource skipped | Fix the dependency first |
| `resource already exists with different owner` | OwnerReference conflict | Delete existing resource or use `Force` |
| `failed to parse template` | Go template syntax error | Check for unmatched `{{}}`, missing `end` |

**Quick Log Grep Commands:**

```bash
# Find all errors in last 100 lines
kubectl logs -n lynq-system deployment/lynq-controller-manager --tail=100 | grep -i error

# Find specific node reconciliation
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "acme-web"

# Find all failed reconciliations
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation failed"

# Find template errors
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -E "(map has no entry|template.*error|failed to render)"

# Find dependency issues
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -E "(DependencySkipped|cycle detected|blocked)"

# Find SSA conflicts
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -i conflict

# Watch live with timestamps
kubectl logs -n lynq-system deployment/lynq-controller-manager -f --timestamps
```

## Getting Help

1. Check operator logs
2. Check LynqNode events: `kubectl describe lynqnode <name>`
3. Check Hub status: `kubectl get lynqhub <name> -o yaml`
4. Review [Performance Guide](performance.md)
5. Open issue: https://github.com/k8s-lynq/lynq/issues

Include in bug reports:
- Operator version
- Kubernetes version
- Operator logs
- LynqNode/Hub/Template YAML
- Steps to reproduce
