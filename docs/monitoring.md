---
description: "Set up Prometheus metrics, Grafana dashboards, and structured logging for Lynq. Includes metrics catalog, events reference, and alerting overview."
---

# Monitoring & Observability

Lynq exposes 26 Prometheus metrics at `:8443/metrics`, emits structured Kubernetes events per reconciliation, and ships a pre-built Grafana dashboard.

> **What changed in the phase model**: `lynqnode_resources_failed` is now stricter — only Lynq-attributed failures (rollout timeout, ProgressDeadlineExceeded, apply error, Job Failed). Steady-state pod-level disruption (node drain, HPA scale-up, eviction) goes to the new `lynqnode_resources_degraded` metric and Lynq does NOT attribute it as failure. See [Resource Phases](resource-phases.md) for the classification rules.

## Setup

### Prometheus

**Enable ServiceMonitor** (requires Prometheus Operator): uncomment the prometheus section in `config/default/kustomization.yaml`:

```yaml
- ../prometheus   # uncomment this line
```

Then redeploy:

```bash
kubectl apply -k config/default
```

**Manual scrape config:**

```yaml
# prometheus.yml
scrape_configs:
- job_name: lynq
  kubernetes_sd_configs:
  - role: pod
    namespaces:
      names: [lynq-system]
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_label_control_plane]
    action: keep
    regex: controller-manager
  - source_labels: [__meta_kubernetes_pod_container_port_name]
    action: keep
    regex: https
```

**Test the endpoint:**

```bash
kubectl port-forward -n lynq-system deployment/lynq-controller-manager 8443:8443
curl -k https://localhost:8443/metrics | grep lynqnode_
```

**Troubleshoot no metrics data:**

```bash
# Check flag is set
kubectl get deployment -n lynq-system lynq-controller-manager -o yaml | grep metrics-bind-address
# Should show: --metrics-bind-address=:8443 (not "0")

# Check service exists
kubectl get svc -n lynq-system lynq-controller-manager-metrics-service

# Check ServiceMonitor (if using prometheus-operator)
kubectl get servicemonitor -n lynq-system
```

### Grafana Dashboard

Import `config/monitoring/grafana-dashboard.json` via **Dashboards → Import** in the Grafana UI. Select your Prometheus datasource. The dashboard ships 21 panels grouped into five categories:

- **Top-line stat panels**: Total Desired / Ready / Failed Nodes, Total Conflicted Resources
- **Reconciliation health**: Reconciliation Duration percentiles, Reconciliation Rate, Error Rate, LynqNode Ready Status, Degraded Nodes
- **Resource & conflict breakdowns**: Resource Counts by Node and Total, Conflicted Resources by Node and Total, Conflicts Rate by Node
- **Hub & runtime internals**: Hub Health, Apply Rate by Kind, Work Queue Depth
- **Resource Phases** (added with the phase model): Resource Phase Distribution (stacked area), Currently Degraded Resources (table), Workload Disruption Rate (Available→Degraded), Rollout Duration P95 (by kind). See [Resource Phases](resource-phases.md).

## Metrics Catalog

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `lynqnode_reconcile_duration_seconds` | Histogram | `result` | LynqNode reconciliation latency |
| `lynqnode_resources_desired` | Gauge | `lynqnode`, `namespace` | Desired resource count per node |
| `lynqnode_resources_ready` | Gauge | `lynqnode`, `namespace` | Ready resource count per node |
| `lynqnode_resources_failed` | Gauge | `lynqnode`, `namespace` | Failed resource count per node |
| `lynqnode_resources_conflicted` | Gauge | `lynqnode`, `namespace` | Resources in conflict state |
| `lynqnode_conflicts_total` | Counter | `lynqnode`, `namespace`, `resource_kind`, `conflict_policy` | Total conflicts detected |
| `lynqnode_condition_status` | Gauge | `lynqnode`, `namespace`, `type` | Condition status (0=False, 1=True, 2=Unknown) |
| `lynqnode_degraded_status` | Gauge | `lynqnode`, `namespace`, `reason` | Degraded status (0=healthy, 1=degraded) |
| `hub_desired` | Gauge | `hub`, `namespace` | Desired LynqNode count for a hub |
| `hub_ready` | Gauge | `hub`, `namespace` | Ready LynqNode count |
| `hub_failed` | Gauge | `hub`, `namespace` | Failed LynqNode count |
| `apply_attempts_total` | Counter | `kind`, `result`, `conflict_policy` | Resource apply attempts |
| `lynqform_rollout_updating_nodes` | Gauge | `form`, `namespace` | Nodes currently being updated (v1.1.16+) |
| `lynqform_rollout_phase` | Gauge | `form`, `namespace` | Rollout phase: 0=Idle, 1=InProgress, 2=Failed, 3=Complete (v1.1.16+) |
| `lynqform_rollout_progress` | Gauge | `form`, `namespace` | Rollout progress percentage (v1.1.16+) |
| `lynqnode_resources_degraded` | Gauge | `lynqnode`, `namespace` | Resources in Degraded phase (steady-state K8s-converged disruption, NOT a Lynq failure). See [Resource Phases](resource-phases.md). |
| `lynqnode_resources_progressing` | Gauge | `lynqnode`, `namespace` | Resources currently rolling out |
| `lynqnode_resources_pending` | Gauge | `lynqnode`, `namespace` | Resources awaiting controller observation |
| `lynqnode_resource_phase` | Gauge | `lynqnode`, `namespace`, `resource_id`, `kind`, `phase` | Per-resource phase stateset (value=1 for active phase, 0 for the other four) |
| `lynqnode_resource_replicas_desired` | Gauge | `lynqnode`, `namespace`, `resource_id`, `kind` | Per-resource `spec.replicas` / `desiredNumberScheduled` (workloads) |
| `lynqnode_resource_replicas_available` | Gauge | (same) | Per-resource `availableReplicas` / `numberAvailable` |
| `lynqnode_resource_replicas_ready` | Gauge | (same) | Per-resource `readyReplicas` / `numberReady` |
| `lynqnode_resource_replicas_updated` | Gauge | (same) | Per-resource `updatedReplicas` / `updatedNumberScheduled` |
| `lynqnode_resource_degraded_since_seconds` | Gauge | `lynqnode`, `namespace`, `resource_id`, `kind` | Seconds since the resource entered Degraded phase (0 when not Degraded) |
| `lynqnode_resource_rollout_duration_seconds` | Histogram | `kind`, `result` (`complete`/`timeout`/`aborted`) | Rollout duration from apply-start-time to first Available, observed once per generation |
| `lynqnode_resource_phase_transitions_total` | Counter | `kind`, `from`, `to` | Phase transitions (powers PromQL SLO recipes like Available→Degraded rate) |

For PromQL queries using these metrics, see [Prometheus Query Examples](prometheus-queries.md).

## Events

Kubernetes events are emitted for key lifecycle transitions.

```bash
# Events for a specific node
kubectl describe lynqnode <name>

# All LynqNode events
kubectl get events -A --field-selector involvedObject.kind=LynqNode --sort-by='.lastTimestamp'
```

| Event | Type | Meaning |
|-------|------|---------|
| `TemplateApplied` | Normal | Resources applied successfully |
| `LynqNodeDeleting` | Normal | Node deletion started |
| `LynqNodeDeleted` | Normal | Node deletion completed |
| `RolloutComplete` | Normal | Resource transitioned `Progressing`/`Pending` → `Available` (emitted once per generation, also observes the `lynqnode_resource_rollout_duration_seconds` histogram). See [Resource Phases](resource-phases.md). |
| `WorkloadRecovered` | Normal | Resource transitioned `Degraded` → `Available` (K8s converged the steady-state disruption). |
| `TemplateRenderError` | Warning | Template syntax or variable error |
| `ApplyFailed` | Warning | Resource apply failed (RBAC, quota, etc.) |
| `ResourceConflict` | Warning | SSA field-manager conflict detected |
| `ForceApply` | Warning | Ownership taken with `conflictPolicy: Force` |
| `DependencySkipped` | Warning | Resource skipped — dependency failed |
| `ReadinessTimeout` | Warning | Resource did not become ready in time (narrowed to `Progressing`→`Failed` via rollout timeout; never fires for steady-state Degraded). |
| `WorkloadDegraded` | Warning | Resource transitioned `Available` → `Degraded` — pod-level disruption (node drain, eviction, HPA, etc.) after rollout had completed. **Kubernetes is converging; this is NOT a Lynq-attributed failure.** |
| `RolloutAborted` | Warning | Resource transitioned `Progressing` → `Failed` for a non-timeout reason (e.g., `ProgressDeadlineExceeded`, apply error). Distinct from `ReadinessTimeout`. |
| `DependencyError` | Warning | Dependency cycle detected |
| `LynqNodeDeletionFailed` | Warning | Node cleanup failed |

## Logging

Configure log level via the `--zap-log-level` flag: `debug`, `info` (default), `error`.

All logs are structured JSON:

```json
{"level":"info","ts":"2025-01-15T10:30:00Z","msg":"Reconciliation completed","lynqnode":"acme-web","ready":10,"failed":0}
```

```bash
# Follow all logs
kubectl logs -n lynq-system deployment/lynq-controller-manager -f

# Errors only
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep '"level":"error"'

# Specific node
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "acme-web"
```

## Alerting

Alert rules are in `config/prometheus/alerts.yaml`. Deploy with:

```bash
kubectl apply -f config/prometheus/alerts.yaml
```

| Severity | Alerts |
|----------|--------|
| Critical | `LynqNodeDegraded`, `LynqNodeResourcesFailed`, `LynqNodeNotReady`, `LynqNodeStatusUnknown`, `HubManyNodesFailure` |
| Warning | `LynqNodeResourcesMismatch`, `LynqNodeResourcesConflicted`, `LynqNodeHighConflictRate`, `HubNodesFailure`, `HubDesiredCountMismatch`, `LynqNodeReconciliationErrors`, `LynqNodeReconciliationSlow`, `HighApplyFailureRate` |
| Info | `LynqNodeNewConflictsDetected` |

For per-alert diagnosis and resolution steps, see [Alert Runbooks](alert-runbooks.md).

## See Also

- [Prometheus Query Examples](prometheus-queries.md) — PromQL cookbook (50+ queries)
- [Alert Runbooks](alert-runbooks.md) — per-alert diagnosis and resolution
- [Troubleshooting](troubleshooting.md) — symptom-based diagnosis
- [Performance](performance.md) — scaling and optimization
