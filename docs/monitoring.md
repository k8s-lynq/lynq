---
description: "Set up Prometheus metrics, Grafana dashboards, and structured logging for Lynq. Includes metrics catalog, events reference, and alerting overview."
---

# Monitoring & Observability

Lynq exposes 15 Prometheus metrics at `:8443/metrics`, emits structured Kubernetes events per reconciliation, and ships a pre-built Grafana dashboard.

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

Import `config/monitoring/grafana-dashboard.json` via **Dashboards → Import** in the Grafana UI. Select your Prometheus datasource. The dashboard ships 17 panels grouped into four categories:

- **Top-line stat panels**: Total Desired / Ready / Failed Nodes, Total Conflicted Resources
- **Reconciliation health**: Reconciliation Duration percentiles, Reconciliation Rate, Error Rate, LynqNode Ready Status, Degraded Nodes
- **Resource & conflict breakdowns**: Resource Counts by Node and Total, Conflicted Resources by Node and Total, Conflicts Rate by Node
- **Hub & runtime internals**: Hub Health, Apply Rate by Kind, Work Queue Depth

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
| `TemplateRenderError` | Warning | Template syntax or variable error |
| `ApplyFailed` | Warning | Resource apply failed (RBAC, quota, etc.) |
| `ResourceConflict` | Warning | SSA field-manager conflict detected |
| `ForceApply` | Warning | Ownership taken with `conflictPolicy: Force` |
| `DependencySkipped` | Warning | Resource skipped — dependency failed |
| `ReadinessTimeout` | Warning | Resource did not become ready in time |
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
