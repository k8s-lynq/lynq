---
description: "Production-ready Prometheus query examples for Lynq: health, performance, conflicts, rollouts, and hub metrics."
---

# Prometheus Query Examples

Ready-to-use PromQL queries for Lynq — node health, reconciliation performance, conflict detection, rollout progress, and hub-level aggregates.

All metrics are exposed at `:8443/metrics`. See [Monitoring](monitoring.md) for setup and the full metrics catalog.

## Node Health

### Check Ready Nodes

```promql
# All ready nodes
lynqnode_condition_status{type="Ready"} == 1

# Count ready nodes
count(lynqnode_condition_status{type="Ready"} == 1)

# Percentage of ready nodes
count(lynqnode_condition_status{type="Ready"} == 1) / count(lynqnode_condition_status{type="Ready"}) * 100
```

### Check Not Ready Nodes

```promql
# All not ready nodes
lynqnode_condition_status{type="Ready"} != 1

# Count not ready nodes
count(lynqnode_condition_status{type="Ready"} != 1)

# List not ready nodes with details
lynqnode_condition_status{type="Ready"} != 1
```

### Check Degraded Nodes

```promql
# All degraded nodes
lynqnode_degraded_status == 1

# Count degraded nodes
count(lynqnode_degraded_status == 1)

# Degraded nodes by reason
sum(lynqnode_degraded_status) by (reason)

# Top 10 degraded nodes
topk(10, lynqnode_degraded_status)

# Degraded nodes with resources not ready (v1.1.4+)
lynqnode_degraded_status{reason="ResourcesNotReady"} == 1

# Count by specific degraded reason
sum(lynqnode_degraded_status{reason="ResourcesNotReady"})

# Nodes with resource failures only
lynqnode_degraded_status{reason="ResourceFailures"} == 1

# Nodes with conflicts only
lynqnode_degraded_status{reason="ResourceConflicts"} == 1

# Nodes with both failures and conflicts
lynqnode_degraded_status{reason="ResourceFailuresAndConflicts"} == 1
```

### Resource Health by Node

```promql
# Ready resources per node
lynqnode_resources_ready

# Failed resources per node
lynqnode_resources_failed

# Resource readiness percentage per node
(lynqnode_resources_ready / lynqnode_resources_desired) * 100

# Nodes with 100% resources ready
(lynqnode_resources_ready / lynqnode_resources_desired) == 1
```

## Conflict Monitoring

### Current Conflicts

```promql
# Total resources currently in conflict
sum(lynqnode_resources_conflicted)

# Nodes with conflicts
lynqnode_resources_conflicted > 0

# Top 10 nodes with most conflicts
topk(10, lynqnode_resources_conflicted)

# Conflict percentage per node
(lynqnode_resources_conflicted / lynqnode_resources_desired) * 100
```

### Conflict Rate

```promql
# Conflict rate (conflicts per second)
rate(lynqnode_conflicts_total[5m])

# Conflict rate per node
sum(rate(lynqnode_conflicts_total[5m])) by (lynqnode)

# Conflict rate by resource kind
sum(rate(lynqnode_conflicts_total[5m])) by (resource_kind)

# Conflict rate by policy
sum(rate(lynqnode_conflicts_total[5m])) by (conflict_policy)
```

### Historical Conflicts

```promql
# Total conflicts in last hour
increase(lynqnode_conflicts_total[1h])

# Total conflicts in last 24 hours
increase(lynqnode_conflicts_total[24h])

# Conflicts over time (5m windows)
sum(increase(lynqnode_conflicts_total[5m])) by (lynqnode)
```

### Conflict Policy Analysis

```promql
# Conflicts by policy type
sum(lynqnode_conflicts_total) by (conflict_policy)

# Force policy usage rate
rate(lynqnode_conflicts_total{conflict_policy="Force"}[5m])

# Stuck policy conflicts
rate(lynqnode_conflicts_total{conflict_policy="Stuck"}[5m])
```

## Failure Detection

### Failed Resources

```promql
# Total failed resources
sum(lynqnode_resources_failed)

# Nodes with failed resources
lynqnode_resources_failed > 0

# Top 10 nodes with most failures
topk(10, lynqnode_resources_failed)

# Failure rate per node
(lynqnode_resources_failed / lynqnode_resources_desired) * 100
```

### Failure Trends

```promql
# Failed resources over time
lynqnode_resources_failed

# Increase in failures (last 1h)
increase(lynqnode_resources_failed[1h])

# Average failures per node
avg(lynqnode_resources_failed)
```

### Critical Failures

```promql
# Nodes with >50% resources failed
(lynqnode_resources_failed / lynqnode_resources_desired) > 0.5

# Nodes with >5 failed resources
lynqnode_resources_failed > 5

# Nodes that are both degraded and have failures
lynqnode_degraded_status == 1 and lynqnode_resources_failed > 0
```

## Performance Monitoring

### Reconciliation Duration

```promql
# P50 reconciliation duration
histogram_quantile(0.50, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# P95 reconciliation duration
histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# P99 reconciliation duration
histogram_quantile(0.99, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# Max reconciliation duration
histogram_quantile(1.0, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))
```

### Reconciliation Rate

```promql
# Total reconciliation rate
rate(lynqnode_reconcile_duration_seconds_count[5m])

# Success rate
rate(lynqnode_reconcile_duration_seconds_count{result="success"}[5m])

# Error rate
rate(lynqnode_reconcile_duration_seconds_count{result="error"}[5m])

# Success percentage
(rate(lynqnode_reconcile_duration_seconds_count{result="success"}[5m]) / rate(lynqnode_reconcile_duration_seconds_count[5m])) * 100
```

### Apply Performance

```promql
# Apply rate by result
sum(rate(apply_attempts_total[5m])) by (result)

# Apply rate by resource kind
sum(rate(apply_attempts_total[5m])) by (kind)

# Apply success rate
rate(apply_attempts_total{result="success"}[5m]) / rate(apply_attempts_total[5m])

# Failed applies by kind
sum(rate(apply_attempts_total{result="error"}[5m])) by (kind)
```

## Hub Health

### Hub Status

```promql
# Desired nodes per hub
hub_desired

# Ready nodes per hub
hub_ready

# Failed nodes per hub
hub_failed

# Hub health percentage
(hub_ready / hub_desired) * 100
```

### Hub Capacity

```promql
# Total desired nodes across all registries
sum(hub_desired)

# Total ready nodes across all registries
sum(hub_ready)

# Total failed nodes across all registries
sum(hub_failed)

# Overall health percentage
(sum(hub_ready) / sum(hub_desired)) * 100
```

### Hub Trends

```promql
# Hub health over time
(hub_ready / hub_desired) * 100

# Registries with >90% health
(hub_ready / hub_desired) > 0.9

# Unhealthy registries (<80% ready)
(hub_ready / hub_desired) < 0.8
```

## Capacity Planning

### Resource Counts

```promql
# Total desired resources across all nodes
sum(lynqnode_resources_desired)

# Total ready resources
sum(lynqnode_resources_ready)

# Total failed resources
sum(lynqnode_resources_failed)

# Total conflicted resources
sum(lynqnode_resources_conflicted)
```

### Growth Trends

```promql
# Desired node growth rate
rate(hub_desired[24h])

# Resource growth per node
rate(lynqnode_resources_desired[24h])

# Average resources per node
avg(lynqnode_resources_desired)
```

### Load Distribution

```promql
# Top 10 nodes by resource count
topk(10, lynqnode_resources_desired)

# Bottom 10 nodes by resource count
bottomk(10, lynqnode_resources_desired)

# Nodes with >100 resources
lynqnode_resources_desired > 100

# Distribution of resources per node
histogram_quantile(0.50, lynqnode_resources_desired)
```

## Combined Queries

### Overall Health Dashboard

```promql
# Total nodes
count(lynqnode_condition_status{type="Ready"})

# Ready percentage
count(lynqnode_condition_status{type="Ready"} == 1) / count(lynqnode_condition_status{type="Ready"}) * 100

# Total resources
sum(lynqnode_resources_desired)

# Ready resources percentage
sum(lynqnode_resources_ready) / sum(lynqnode_resources_desired) * 100

# Active conflicts
sum(lynqnode_resources_conflicted)

# Total failures
sum(lynqnode_resources_failed)
```

### Problem Detection

```promql
# Nodes with issues (not ready OR degraded OR conflicts OR failures)
(lynqnode_condition_status{type="Ready"} != 1)
or (lynqnode_degraded_status == 1)
or (lynqnode_resources_conflicted > 0)
or (lynqnode_resources_failed > 0)

# Count problematic nodes
count(
  (lynqnode_condition_status{type="Ready"} != 1)
  or (lynqnode_degraded_status == 1)
  or (lynqnode_resources_conflicted > 0)
  or (lynqnode_resources_failed > 0)
)
```

### Performance Summary

```promql
# P95 latency, error rate, and throughput
{
  p95_latency: histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m])),
  error_rate: rate(lynqnode_reconcile_duration_seconds_count{result="error"}[5m]),
  throughput: rate(lynqnode_reconcile_duration_seconds_count[5m])
}
```

## Alert Conditions

These queries are used in the alert rules (`config/prometheus/alerts.yaml`):

### Critical Conditions

```promql
# Node has failed resources
lynqnode_resources_failed > 0

# Node is degraded
lynqnode_degraded_status > 0

# Node not ready
lynqnode_condition_status{type="Ready"} != 1

# Hub has many failures
hub_failed > 5 or (hub_failed / hub_desired > 0.5 and hub_desired > 0)
```

### Warning Conditions

```promql
# Resources in conflict
lynqnode_resources_conflicted > 0

# High conflict rate
rate(lynqnode_conflicts_total[5m]) > 0.1

# Resources mismatch
lynqnode_resources_ready != lynqnode_resources_desired and lynqnode_resources_desired > 0

# Slow reconciliation
histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m])) > 30

# Resources not ready (v1.1.4+)
lynqnode_degraded_status{reason="ResourcesNotReady"} > 0

# Nodes with both failures and conflicts (v1.1.4+)
lynqnode_degraded_status{reason="ResourceFailuresAndConflicts"} > 0
```

## v1.1.4 Enhanced Status Queries

::: tip New in v1.1.4
v1.1.4 introduces more granular degraded condition reasons and smart reconciliation with 30-second requeue interval.
:::

### New Degraded Reasons

```promql
# All nodes degraded due to resources not ready
lynqnode_degraded_status{reason="ResourcesNotReady"} == 1

# Nodes with resource failures only
lynqnode_degraded_status{reason="ResourceFailures"} == 1

# Nodes with conflicts only
lynqnode_degraded_status{reason="ResourceConflicts"} == 1

# Nodes with both failures and conflicts
lynqnode_degraded_status{reason="ResourceFailuresAndConflicts"} == 1

# Count nodes by degraded reason
sum(lynqnode_degraded_status == 1) by (reason)
```

### Ready Condition Granularity

Check why nodes are not ready:

```promql
# All not-ready nodes with detailed reason
lynqnode_condition_status{type="Ready"} != 1

# Degraded nodes with Ready=False
lynqnode_condition_status{type="Ready"} != 1 and lynqnode_condition_status{type="Degraded"} == 1

# Query node annotations for reason details (requires external tooling)
# Reasons: ResourcesFailedAndConflicted, ResourcesConflicted,
#          ResourcesFailed, NotAllResourcesReady
```

### Smart Reconciliation Monitoring

Monitor the 30-second requeue behavior:

```promql
# Reconciliation frequency (should show ~2 per minute per node in v1.1.4+)
rate(lynqnode_reconcile_duration_seconds_count[5m])

# P50 latency (should remain low due to fast requeue)
histogram_quantile(0.50, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# P95 latency (watch for spikes > 30s)
histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# Detect reconciliation bottlenecks
histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m])) > 10

# Status-only reconciliations (fast path)
rate(lynqnode_reconcile_duration_seconds_count{result="status_only"}[5m])
```

### Readiness Tracking

Track how quickly resources become ready:

```promql
# Percentage of resources ready
(sum(lynqnode_resources_ready) / sum(lynqnode_resources_desired)) * 100

# Nodes with incomplete readiness
lynqnode_resources_ready < lynqnode_resources_desired

# Average time to readiness (approximation via reconciliation duration)
avg(rate(lynqnode_reconcile_duration_seconds_sum{result="success"}[5m]) /
    rate(lynqnode_reconcile_duration_seconds_count{result="success"}[5m]))
```

## Tips for Using These Queries

1. **Adjust Time Windows**: Change `[5m]`, `[1h]`, `[24h]` based on your needs
2. **Filter by Namespace**: Add `{namespace="default"}` to filter
3. **Filter by Node**: Add `{lynqnode="my-node"}` to focus on specific nodes
4. **Combine Queries**: Use `and`, `or`, `unless` for complex conditions
5. **Aggregation**: Use `sum`, `avg`, `max`, `min` for aggregations
6. **Top/Bottom N**: Use `topk(N, ...)` or `bottomk(N, ...)`

### Example: Filter by Namespace and Time

```promql
# Failed resources in default namespace, last 1 hour
lynqnode_resources_failed{namespace="default"}[1h]

# Conflicts for specific node in last 5 minutes
rate(lynqnode_conflicts_total{lynqnode="acme-prod-template", namespace="default"}[5m])
```

## See Also

- [Monitoring Guide](monitoring.md) - Complete monitoring documentation
- [Alert Rules](../config/prometheus/alerts.yaml) - Prometheus alert rules
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
