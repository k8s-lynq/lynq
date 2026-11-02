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

A not-ready node is not necessarily a failed one. Every LynqNode is in exactly one of three states: **ready** (`Ready=True`), **failed** (failed or conflicted resources, or `Degraded=True`), or **progressing** (rolling out, recovering lost pods, or not reconciled yet). Not-ready covers both failed and progressing nodes.

```promql
# All not ready nodes (failed and progressing)
lynqnode_condition_status{type="Ready"} != 1

# Count not ready nodes
count(lynqnode_condition_status{type="Ready"} != 1)

# Not-ready nodes that need an operator (Degraded=True)
max by (lynqnode, namespace) (lynqnode_condition_status{type="Degraded"}) == 1
and on (lynqnode, namespace) (max by (lynqnode, namespace) (lynqnode_condition_status{type="Ready"}) != 1)

# Not-ready nodes that are only progressing (Degraded=False): no action needed yet
max by (lynqnode, namespace) (lynqnode_condition_status{type="Degraded"}) == 0
and on (lynqnode, namespace) (max by (lynqnode, namespace) (lynqnode_condition_status{type="Ready"}) != 1)
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

# Count by specific degraded reason
sum(lynqnode_degraded_status{reason="ResourceFailures"})

# Not-ready nodes (not-ready alone is Ready=False/NotAllResourcesReady, never Degraded)
lynqnode_condition_status{type="Ready"} == 0

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

# Not-ready resources per node: neither ready nor failed (starting up, recovering after a
# regression such as a lost pod, or skipped because a dependency failed).
# desired = ready + not ready + failed
lynqnode_resources_desired - lynqnode_resources_ready - lynqnode_resources_failed
```

`lynqnode_resources_failed` counts only Lynq's own changes that never became Ready (readiness timeout, `ProgressDeadlineExceeded`) or failed to apply. A resource that was Ready and later regressed is not-ready, not failed.

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

A hub's nodes split into three states: `hub_desired = hub_ready + hub_failed + progressing`. `hub_failed` counts only nodes that need an operator (failed or conflicted resources, or `Degraded=True`). Progressing nodes (rolling out, recovering lost pods, or not created yet, including creation held back by `maxSkew`) have no metric of their own; they are the gap `hub_desired - hub_ready - hub_failed`.

::: warning Older Lynq versions
Lynq v1.1.23-alpha.1 and earlier count every node with `Ready!=True` in `hub_failed`. On those versions the progressing gap is only nodes not created yet, and `hub_failed` rises during every rollout or node drain.
:::

### Hub Status

```promql
# Desired nodes per hub
hub_desired

# Ready nodes per hub
hub_ready

# Failed nodes per hub (need an operator)
hub_failed

# Progressing nodes per hub (neither ready nor failed). clamp_min hides the brief negative
# value a scrape taken between two status updates can produce
clamp_min(hub_desired - hub_ready - hub_failed, 0)

# Ready percentage per hub (drops during rollouts and drains; not a failure signal by itself)
(hub_ready / hub_desired) * 100
```

### Hub Capacity

```promql
# Total desired nodes across all hubs
sum(hub_desired)

# Total ready nodes across all hubs
sum(hub_ready)

# Total failed nodes across all hubs
sum(hub_failed)

# Total progressing nodes across all hubs
clamp_min(sum(hub_desired) - sum(hub_ready) - sum(hub_failed), 0)

# Overall ready percentage
(sum(hub_ready) / sum(hub_desired)) * 100
```

### Hub Trends

```promql
# Hub ready percentage over time
(hub_ready / hub_desired) * 100

# Hubs with any failed node (what HubNodesFailure / HubManyNodesFailure fire on)
hub_failed > 0

# Hubs with progressing nodes but none failed (what HubDesiredCountMismatch fires on after 20m)
(hub_ready != hub_desired) and (hub_desired > 0) and (hub_failed == 0)
```

## Rollouts

LynqForms with `spec.rollout.maxSkew > 0` report rollout metrics; other LynqForms have no rollout series. `lynqform_rollout_phase` is `0=Idle`, `1=InProgress`, `2=Failed`, `3=Complete`. `Failed` means an updated LynqNode failed, or did not become Ready within `spec.rollout.progressDeadlineSeconds`; the node keeps its `maxSkew` slot, so the rollout stops until it is fixed.

```promql
# Stalled rollouts (what LynqFormRolloutFailed fires on)
lynqform_rollout_phase == 2

# Count stalled rollouts
count(lynqform_rollout_phase == 2) or vector(0)

# Rollouts in progress
lynqform_rollout_phase == 1

# Rollout progress percentage (nodes Ready on the new LynqForm generation)
lynqform_rollout_progress

# Nodes currently updating (updated but not Ready yet; bounded by maxSkew)
lynqform_rollout_updating_nodes

# In-progress rollouts that made no progress in the last 30 minutes
(lynqform_rollout_phase == 1) and on (form, namespace) (delta(lynqform_rollout_progress[30m]) == 0)
```

The nodes that stalled a rollout are listed in the LynqForm status:

```bash
kubectl get lynqform <name> -n <namespace> -o jsonpath='{.status.rollout.message}'
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

Not-ready alone is not a problem: rolling-out and recovering nodes are not ready but need no action. Query nodes that need an operator separately from nodes that are only not ready.

```promql
# Nodes that need an operator (degraded OR conflicts OR failures)
max by (lynqnode, namespace) (lynqnode_degraded_status) == 1
or max by (lynqnode, namespace) (lynqnode_resources_conflicted) > 0
or max by (lynqnode, namespace) (lynqnode_resources_failed) > 0

# Count nodes that need an operator
count(
  max by (lynqnode, namespace) (lynqnode_degraded_status) == 1
  or max by (lynqnode, namespace) (lynqnode_resources_conflicted) > 0
  or max by (lynqnode, namespace) (lynqnode_resources_failed) > 0
)

# Nodes that are not ready (needing an operator or only progressing)
lynqnode_condition_status{type="Ready"} != 1

# Stalled rollouts
lynqform_rollout_phase == 2
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

# Node not ready (fires after 15m; includes progressing nodes)
lynqnode_condition_status{type="Ready"} == 0

# Node status unknown (fires after 10m)
lynqnode_condition_status{type="Ready"} == 2

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

# Nodes not ready for 15m+ (what the LynqNodeNotReady alert fires on)
min_over_time(lynqnode_condition_status{type="Ready"}[15m]) == 0

# Nodes with both failures and conflicts (v1.1.4+)
lynqnode_degraded_status{reason="ResourceFailuresAndConflicts"} > 0

# Hub has failed nodes (1-5)
hub_failed > 0 and hub_failed <= 5

# Hub has progressing nodes but none failed (fires after 20m)
(hub_ready != hub_desired) and (hub_desired > 0) and (hub_failed == 0)

# LynqForm rollout stalled
lynqform_rollout_phase == 2
```

## v1.1.4 Enhanced Status Queries

::: tip New in v1.1.4
v1.1.4 introduces more granular degraded condition reasons and smart reconciliation with 30-second requeue interval.
:::

### New Degraded Reasons

`ResourcesNotReady` is no longer emitted: not-ready alone is `Ready=False` (`NotAllResourcesReady`), not Degraded. Query not-ready nodes with `lynqnode_condition_status{type="Ready"} == 0`.

```promql
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
