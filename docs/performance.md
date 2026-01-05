# Performance Tuning Guide

Practical optimization strategies for scaling Lynq to thousands of nodes.

[[toc]]

## Understanding Performance

Lynq uses three reconciliation layers:

1. **Event-Driven (Immediate)**: Reacts to resource changes instantly
2. **Periodic (30 seconds)**: Fast status updates and drift detection
3. **Database Sync (Configurable)**: Syncs node data at defined intervals

This architecture ensures:
- ✅ Immediate drift correction
- ✅ Fast status reflection (30s)
- ✅ Configurable database sync frequency

## Configuration Tuning

### 1. Database Sync Interval

Adjust how frequently the operator checks your database:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-hub
spec:
  source:
    syncInterval: 1m  # Default: 1 minute
```

**Recommendations:**
- **High-frequency changes**: `30s` - Faster node provisioning, higher DB load
- **Normal usage**: `1m` (default) - Balanced performance
- **Stable nodes**: `5m` - Lower DB load, slower updates

### 2. Resource Wait Timeouts

Control how long to wait for resources to become ready:

```yaml
deployments:
  - id: app
    waitForReady: true
    timeoutSeconds: 300  # Default: 5 minutes (max: 3600)
```

**Recommendations:**
- **Fast services**: `60s` - Quick deployments (< 1 min)
- **Normal apps**: `300s` (default) - Standard deployments
- **Heavy apps**: `600s` - Database migrations, complex initialization
- **Skip waiting**: Set `waitForReady: false` for non-critical resources

### 3. Creation Policy Optimization

Reduce unnecessary reconciliations:

```yaml
configMaps:
  - id: init-config
    creationPolicy: Once  # Create once, never reapply
```

**Use Cases:**
- `Once`: Init scripts, immutable configs, security resources
- `WhenNeeded` (default): Normal resources that may need updates

## Template Optimization

### 1. Keep Templates Simple

**✅ Good - Efficient template:**
```yaml
nameTemplate: "{{ .uid }}-app"
```

**❌ Bad - Complex template:**
```yaml
nameTemplate: "{{ .uid }}-{{ .region }}-{{ .planId }}-{{ now | date \"20060102\" }}"
# Avoid: timestamps, random values, complex logic
```

**Tips:**
- Keep templates simple and predictable
- Avoid `now`, `randAlphaNum`, or other non-deterministic functions
- Use consistent naming patterns
- Cache-friendly templates improve performance

### 2. Dependency Graph Optimization

**✅ Good - Shallow dependency tree:**
```yaml
resources:
  - id: namespace      # No dependencies
  - id: deployment     # Depends on: namespace
  - id: service        # Depends on: deployment
# Depth: 3 - Resources can be created in parallel groups
```

**❌ Bad - Deep dependency tree:**
```yaml
resources:
  - id: a              # No dependencies
  - id: b              # Depends on: a
  - id: c              # Depends on: b
  - id: d              # Depends on: c
  - id: e              # Depends on: d
# Depth: 5 - Fully sequential, slow
```

**Impact:**
- Shallow trees enable parallel execution
- Deep trees force sequential execution
- Each level adds wait time

### 3. Minimize Resource Count

**Example:** Create 5 essential resources per node instead of 15

```yaml
# Essential only
spec:
  namespaces: [1]
  deployments: [1]
  services: [1]
  configMaps: [1]
  ingresses: [1]
# Total: 5 resources
```

**Impact:**
- Fewer resources = Faster reconciliation
- Less API server load
- Lower memory usage

## Scaling Considerations

### Resource Limits

Adjust operator resource limits based on node count:

```yaml
# values.yaml for Helm
resources:
  limits:
    cpu: 2000m      # For 1000+ nodes
    memory: 2Gi     # For 1000+ nodes
  requests:
    cpu: 500m       # Minimum for stable operation
    memory: 512Mi   # Minimum for stable operation
```

**Guidelines:**
- **< 100 nodes**: Default limits (500m CPU, 512Mi RAM)
- **100-500 nodes**: 1 CPU, 1Gi RAM
- **500-1000 nodes**: 2 CPU, 2Gi RAM
- **1000+ nodes**: Consider horizontal scaling (coming in v1.3)

### Database Optimization

1. **Add indexes** to node table:
```sql
CREATE INDEX idx_is_active ON node_configs(is_active);
CREATE INDEX idx_node_id ON node_configs(node_id);
```

2. **Use read replicas** for high-frequency syncs

3. **Connection pooling**: Operator uses persistent connections

## Monitoring Performance

### Key Metrics with Thresholds

Monitor these Prometheus metrics with specific thresholds:

| Metric | Normal | Warning | Critical | Action |
|--------|--------|---------|----------|--------|
| Reconciliation P95 | < 5s | 5-15s | > 15s | Simplify templates, reduce dependencies |
| Reconciliation P99 | < 15s | 15-30s | > 30s | Check for blocking resources |
| Node Ready Rate | > 98% | 95-98% | < 95% | Check failed nodes, resource issues |
| Error Rate | < 1% | 1-5% | > 5% | Investigate operator logs |
| Skipped Resources | 0 | 1-5 | > 5 | Fix dependency failures |
| Conflict Count | 0 | 1-3 | > 3 | Review resource ownership |
| CPU Usage | < 50% | 50-80% | > 80% | Increase limits or reduce concurrency |
| Memory Usage | < 70% | 70-90% | > 90% | Increase limits or restart operator |
| Hub Sync Duration | < 1s | 1-5s | > 5s | Optimize DB query, add indexes |

**Prometheus Queries:**

```promql
# Reconciliation duration P95 (target: < 5s)
histogram_quantile(0.95,
  sum(rate(lynqnode_reconcile_duration_seconds_bucket[5m])) by (le)
)

# Reconciliation duration P99 (target: < 15s)
histogram_quantile(0.99,
  sum(rate(lynqnode_reconcile_duration_seconds_bucket[5m])) by (le)
)

# Node readiness rate (target: > 98%)
sum(lynqnode_resources_ready) / sum(lynqnode_resources_desired) * 100

# Error rate (target: < 1%)
sum(rate(lynqnode_reconcile_duration_seconds_count{result="error"}[5m]))
/ sum(rate(lynqnode_reconcile_duration_seconds_count[5m])) * 100

# Conflict count by node
sum by (lynqnode) (lynqnode_resources_conflicted)

# Degraded nodes
count(lynqnode_degraded_status == 1)
```

### Sample Prometheus Alert Rules

```yaml
# config/prometheus/alerts.yaml
groups:
  - name: lynq-performance
    rules:
      - alert: LynqSlowReconciliation
        expr: histogram_quantile(0.95, sum(rate(lynqnode_reconcile_duration_seconds_bucket[5m])) by (le)) > 15
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Lynq reconciliation is slow"
          description: "P95 reconciliation time is {{ $value | humanizeDuration }}"

      - alert: LynqHighErrorRate
        expr: |
          sum(rate(lynqnode_reconcile_duration_seconds_count{result="error"}[5m]))
          / sum(rate(lynqnode_reconcile_duration_seconds_count[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Lynq error rate above 5%"
          description: "Current error rate: {{ $value | humanizePercentage }}"

      - alert: LynqLowReadyRate
        expr: sum(lynqnode_resources_ready) / sum(lynqnode_resources_desired) < 0.95
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Lynq ready rate below 95%"
          description: "Only {{ $value | humanizePercentage }} resources are ready"

      - alert: LynqHighMemory
        expr: |
          container_memory_usage_bytes{container="manager", namespace="lynq-system"}
          / container_spec_memory_limit_bytes{container="manager", namespace="lynq-system"} > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Lynq operator memory above 90%"
          description: "Memory usage: {{ $value | humanizePercentage }}"
```

See [Monitoring Guide](monitoring.md) for complete metrics reference.

## Bottleneck Identification Priority

When performance degrades, identify bottlenecks in this order:

```
Performance Issue?
│
├─ Step 1: Check Reconciliation Duration
│  $ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation completed" | tail -20
│  │
│  ├─ > 15s? → Check dependency depth, waitForReady timeouts
│  └─ < 5s? → Continue to Step 2
│
├─ Step 2: Check Hub Sync Duration
│  $ kubectl get lynqhub -o jsonpath='{range .items[*]}{.metadata.name}: {.status.lastSyncDuration}{"\n"}{end}'
│  │
│  ├─ > 5s? → Optimize DB query, add indexes
│  └─ < 1s? → Continue to Step 3
│
├─ Step 3: Check Resource Usage
│  $ kubectl top pods -n lynq-system
│  │
│  ├─ CPU > 80%? → Increase limits or reduce concurrency
│  └─ Memory > 90%? → Increase limits or restart operator
│
└─ Step 4: Check Error Rate
   $ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep -c ERROR
   │
   ├─ High error count? → Check specific error messages
   └─ Low errors? → Performance may be within normal range
```

### Real-World Before/After Optimization

**Scenario:** 500 nodes with 8 resources each = 4,000 resources total

**Before Optimization:**
```bash
# Status: Slow reconciliation, high CPU
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation completed" | tail -5
2024-01-15T10:30:45.123Z INFO  Reconciliation completed  {"node": "node-001", "duration": "45.2s"}
2024-01-15T10:31:30.456Z INFO  Reconciliation completed  {"node": "node-002", "duration": "43.8s"}
2024-01-15T10:32:15.789Z INFO  Reconciliation completed  {"node": "node-003", "duration": "47.1s"}

$ kubectl top pods -n lynq-system
NAME                                     CPU(cores)   MEMORY(bytes)
lynq-controller-manager-xxx              1850m        1.8Gi

# Metrics
lynqnode_reconcile_duration_seconds{quantile="0.95"} = 45.0
lynqnode_resources_ready / lynqnode_resources_desired = 0.72  # 72% ready
```

**Optimization Applied:**

```yaml
# 1. Reduced dependency depth from 5 to 3 levels
# Before: secret → configmap → deployment → service → ingress (depth: 5)
# After: secret, configmap (parallel) → deployment → service, ingress (parallel)

# 2. Set waitForReady: false for non-critical resources
configMaps:
  - id: config
    waitForReady: false  # ConfigMaps don't need readiness checks

# 3. Used creationPolicy: Once for init resources
secrets:
  - id: init-secret
    creationPolicy: Once  # Skip re-applying on each reconcile

# 4. Increased concurrency
args:
  - --node-concurrency=20  # Up from 10
```

**After Optimization:**
```bash
# Status: Fast reconciliation, normal CPU
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation completed" | tail -5
2024-01-15T11:30:02.123Z INFO  Reconciliation completed  {"node": "node-001", "duration": "3.2s"}
2024-01-15T11:30:05.456Z INFO  Reconciliation completed  {"node": "node-002", "duration": "2.8s"}
2024-01-15T11:30:08.789Z INFO  Reconciliation completed  {"node": "node-003", "duration": "3.5s"}

$ kubectl top pods -n lynq-system
NAME                                     CPU(cores)   MEMORY(bytes)
lynq-controller-manager-xxx              450m         890Mi

# Metrics
lynqnode_reconcile_duration_seconds{quantile="0.95"} = 4.2  # 90% improvement!
lynqnode_resources_ready / lynqnode_resources_desired = 0.99  # 99% ready
```

**Improvement Summary:**

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Reconciliation P95 | 45s | 4.2s | **90% faster** |
| Ready Rate | 72% | 99% | **+27%** |
| CPU Usage | 1850m | 450m | **75% reduction** |
| Memory Usage | 1.8Gi | 890Mi | **50% reduction** |
| Time to Ready (all) | ~25min | ~3min | **88% faster** |

## Troubleshooting Slow Performance

### Symptom: Slow Node Creation

**Check:**
1. Database query performance
2. `waitForReady` timeouts
3. Dependency chain depth

**Solution:**
```bash
# Check reconciliation times
kubectl logs -n lynq-system -l control-plane=controller-manager | grep "Reconciliation completed"

# Reduce sync interval if database is slow
kubectl patch lynqhub my-hub --type=merge -p '{"spec":{"source":{"syncInterval":"2m"}}}'
```

### Symptom: High CPU Usage

**Check:**
1. Reconciliation frequency
2. Template complexity
3. Total node count

**Solution:**
```bash
# Check CPU usage
kubectl top pods -n lynq-system

# Increase resource limits
kubectl edit deployment -n lynq-system lynq-controller-manager
```

### Symptom: Memory Growth

**Possible causes:**
1. Too many cached resources
2. Large template outputs
3. Memory leak (file an issue)

**Solution:**
```bash
# Restart operator to clear cache
kubectl rollout restart deployment -n lynq-system lynq-controller-manager

# Monitor memory over time
kubectl top pods -n lynq-system --watch
```

## Best Practices Summary

1. **✅ Start with defaults** - Only optimize if you see issues
2. **✅ Keep templates simple** - Avoid complex logic and non-deterministic functions
3. **✅ Use shallow dependency trees** - Enable parallel resource creation
4. **✅ Set appropriate timeouts** - Balance speed vs reliability
5. **✅ Monitor key metrics** - Watch reconciliation duration and error rates
6. **✅ Index your database** - Improve sync query performance
7. **✅ Use `CreationPolicy: Once`** - For immutable resources

## See Also

- [Monitoring Guide](monitoring.md) - Complete metrics reference and dashboards
- [Prometheus Queries](prometheus-queries.md) - Ready-to-use queries
- [Configuration Guide](configuration.md) - All operator settings
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
