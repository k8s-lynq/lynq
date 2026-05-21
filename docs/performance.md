---
description: "Tuning guide for Lynq at scale. Covers concurrency flags, requeue intervals, watch predicates, and large node count recommendations."
---

# Performance Tuning Guide

Practical optimization strategies for scaling Lynq to hundreds of nodes.

## Understanding Performance

Lynq uses four reconciliation layers:

1. **Event-Driven (Immediate)**: Reacts to spec or non-`lynq.sh/*` annotation changes on watched child resources via `Owns()` watches, and to LynqNode CR changes.
2. **Periodic (30 seconds)**: Refreshes child-resource status into LynqNode status (so `readyResources` / `failedResources` reflect cluster reality quickly).
3. **Force-Reapply (`ForceReapplyInterval`, default 10 minutes)**: Per-LynqNode periodic resync that bypasses the per-resource skip check and re-applies every child resource unconditionally. This is Lynq's drift-correction backstop.
4. **Database Sync (Configurable)**: Syncs node data at defined intervals (default: 30 seconds).

Drift correction operates on two channels:

- **Immediate (watch-driven)** — external mutations that bump `metadata.generation` or alter the `lynq.sh/applied-hash` annotation are caught on the next reconcile cycle (sub-30s typical).
- **~10 minute (periodic force-reapply)** — external mutations that preserved `applied-hash` are caught on the next `ForceReapplyInterval`-gated cycle. This trades a longer correction window for a structurally race-free apply path (no post-apply MergePatch).

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
    syncInterval: 1m  # Default: 30 seconds
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

Quick reference based on real benchmark data:

- 5 nodes: **0 – 5 m** CPU / 10 – 60 MB RAM
- **300+ nodes: < 100 m CPU / < 120 MB RAM** — annotation-driven skip path eliminates the per-reconcile API-write cost; the vast majority of reconciles are no-op (hash match ⇒ skip)

| Node count | CPU request | CPU limit | Memory request | Memory limit |
|-----------|------------|-----------|----------------|--------------|
| < 50 | `25m` | `100m` | `64Mi` | `128Mi` |
| 50–200 | `50m` | `200m` | `96Mi` | `192Mi` |
| 200–500 | `100m` | `300m` | `128Mi` | `256Mi` |
| 500–1000 | `200m` | `500m` | `256Mi` | `512Mi` |
| 1000+ | `500m` | `1000m` | `512Mi` | `1Gi` |

CPU is bursty: steady-state is near-idle, with brief spikes during periodic force-reapply (every `ForceReapplyInterval`, default 10 min) and template-change events. Memory is stable once the controller-runtime informer cache warms up.

For the explanation of *why* resources scale this way (cache model, reconciliation burst pattern, concurrency trade-offs), see [Resource Sizing](resource-sizing.md).

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

| Metric | Target | Warning | Critical | Action |
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
1. Controller-runtime informer caches scale with total watched-resource count (12 native kinds × cluster-wide scope) — proportional to node count and template breadth
2. Large rendered template outputs held briefly during reconcile
3. Memory leak (file an issue)

Note: Lynq's apply path itself holds **no in-memory per-resource cache** — skip decisions read the `lynq.sh/applied-hash` annotation on each live resource. Restarting the operator does not free any Lynq-specific cache.

**Solution:**
```bash
# Monitor memory over time
kubectl top pods -n lynq-system --watch

# If growth correlates with node count, tune watch scope or shard concurrency
#   --node-concurrency=N  (lower = lower steady-state memory)
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
