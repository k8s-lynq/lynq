---
description: "Monitor Lynq with Prometheus metrics, Grafana dashboards, and Kubernetes events. Covers alert rules, custom queries, and observability best practices."
---

# Monitoring & Observability Guide

Comprehensive guide for monitoring Lynq with Prometheus, Grafana, and Kubernetes events.

[[toc]]

## Getting Started

### Accessing Metrics

::: info Endpoint
Lynq exposes Prometheus metrics at `:8443/metrics` over HTTPS.
:::

**Port-forward for local testing:**

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n lynq-system \
  deployment/lynq-controller-manager 8443:8443

# Access metrics (requires valid TLS client or use --insecure)
curl -k https://localhost:8443/metrics
```

**Check if metrics are enabled:**

```bash
# Check if metrics port is exposed
kubectl get svc -n lynq-system lynq-controller-manager-metrics-service

# Check if ServiceMonitor is deployed (requires prometheus-operator)
kubectl get servicemonitor -n lynq-system
```

### Enabling ServiceMonitor

If using Prometheus Operator, enable ServiceMonitor by uncommenting in `config/default/kustomization.yaml`:

```yaml
# Line 27: Uncomment this
- ../prometheus
```

Then redeploy:

```bash
kubectl apply -k config/default
```

::: tip Verify scrape job
After redeploying, confirm that a `ServiceMonitor` named `lynq-controller-manager` appears and that Prometheus discovers the target.
:::

## Metrics Overview

Lynq exposes 15 custom Prometheus metrics organized into five categories:

### Metrics Summary

| Metric | Type | Description | Key Labels |
|--------|------|-------------|------------|
| **Controller Metrics** |
| `lynqnode_reconcile_duration_seconds` | Histogram | LynqNode reconciliation duration | `result` |
| **Resource Metrics** |
| `lynqnode_resources_desired` | Gauge | Desired resource count per node | `lynqnode`, `namespace` |
| `lynqnode_resources_ready` | Gauge | Ready resource count per node | `lynqnode`, `namespace` |
| `lynqnode_resources_failed` | Gauge | Failed resource count per node | `lynqnode`, `namespace` |
| **Hub Metrics** |
| `hub_desired` | Gauge | Desired LynqNode CRs for a hub | `hub`, `namespace` |
| `hub_ready` | Gauge | Ready LynqNode CRs for a hub | `hub`, `namespace` |
| `hub_failed` | Gauge | Failed LynqNode CRs for a hub | `hub`, `namespace` |
| **Apply Metrics** |
| `apply_attempts_total` | Counter | Resource apply attempts | `kind`, `result`, `conflict_policy` |
| **Status Metrics** |
| `lynqnode_condition_status` | Gauge | LynqNode condition status (0=False, 1=True, 2=Unknown) | `lynqnode`, `namespace`, `type` |
| `lynqnode_conflicts_total` | Counter | Total resource conflicts | `lynqnode`, `namespace`, `resource_kind`, `conflict_policy` |
| `lynqnode_resources_conflicted` | Gauge | Current resources in conflict state | `lynqnode`, `namespace` |
| `lynqnode_degraded_status` | Gauge | LynqNode degraded status (0=Not degraded, 1=Degraded) | `lynqnode`, `namespace`, `reason` |
| **Rollout Metrics (v1.1.16+)** |
| `lynqform_rollout_updating_nodes` | Gauge | Nodes currently being updated | `form`, `namespace` |
| `lynqform_rollout_phase` | Gauge | Rollout phase (0=Idle, 1=InProgress, 2=Failed, 3=Complete) | `form`, `namespace` |
| `lynqform_rollout_progress` | Gauge | Rollout progress percentage | `form`, `namespace` |

::: tip Detailed Queries
For comprehensive PromQL query examples, see [Prometheus Query Examples](prometheus-queries.md).
:::

### Quick Start Queries

**LynqNode Health:**
```promql
# Ready nodes
lynqnode_condition_status{type="Ready"} == 1

# Degraded nodes
lynqnode_degraded_status == 1

# Resource readiness percentage
(lynqnode_resources_ready / lynqnode_resources_desired) * 100
```

**Performance:**
```promql
# P95 reconciliation latency
histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# Reconciliation rate
rate(lynqnode_reconcile_duration_seconds_count[5m])

# Error rate
rate(lynqnode_reconcile_duration_seconds{result="error"}[5m])
```

**Hub Health:**
```promql
# Hub health percentage
(hub_ready / hub_desired) * 100

# Total desired nodes
sum(hub_desired)
```

**Conflicts:**
```promql
# Current conflicts
sum(lynqnode_resources_conflicted)

# Conflict rate
rate(lynqnode_conflicts_total[5m])
```

**Rollout (v1.1.16+):**
```promql
# Forms currently rolling out
lynqform_rollout_phase == 1

# Rollout progress by form
lynqform_rollout_progress

# Nodes currently updating
sum(lynqform_rollout_updating_nodes)

# Stalled rollouts (InProgress for too long)
lynqform_rollout_phase == 1 and time() - lynqform_rollout_start_time > 3600
```

::: tip Complete Query Reference
See [Prometheus Query Examples](prometheus-queries.md) for 50+ production-ready queries organized by use case.
:::

### Smart Reconciliation Metrics (v1.1.4+)

::: tip New in v1.1.4
v1.1.4 introduces enhanced status tracking with a 30-second requeue interval for fast status reflection.
:::

**Key Changes:**
- **Fast Status Updates**: Child resource status changes reflected in LynqNode status within 30 seconds (down from 5 minutes)
- **Event-Driven**: Immediate reconciliation on watched resource changes
- **Smart Predicates**: Only reconcile on Generation/Annotation changes, not status-only updates

**Impact on Metrics:**

The 30-second requeue interval means you'll see:
- **Higher reconciliation frequency**: ~2 reconciles per minute per node
- **Lower latency**: Status changes propagate faster
- **Optimized overhead**: Smart predicates filter unnecessary reconciliations

**Monitoring Reconciliation Patterns:**

```promql
# Reconciliation frequency (should show ~2 per minute per node in v1.1.4+)
rate(lynqnode_reconcile_duration_seconds_count[5m])

# P50 latency (should remain low despite faster requeue)
histogram_quantile(0.50, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))

# P95 latency (watch for spikes > 30s)
histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))
```

**Best Practices:**
1. **Capacity Planning**: Monitor reconciliation rate for horizontal scaling decisions
2. **Latency Tracking**: P95 latency should stay under 10s for healthy systems
3. **Event-Driven Behavior**: Most reconciliations should be triggered by resource changes, not periodic requeues
4. **Watch Predicates**: Verify that status-only updates don't trigger full reconciliations

### Controller-Runtime Metrics

Standard controller-runtime metrics:

```promql
# Work queue depth
workqueue_depth{name="lynqnode"}

# Work queue add rate
rate(workqueue_adds_total{name="lynqnode"}[5m])

# Work queue latency
workqueue_queue_duration_seconds{name="lynqnode"}
```

## Metrics Collection

### Prometheus ServiceMonitor

To enable ServiceMonitor, uncomment the prometheus section in `config/default/kustomization.yaml`:

```yaml
# Uncomment this line:
#- ../prometheus
```

The ServiceMonitor configuration is available in `config/prometheus/monitor.yaml`.

**Note:** For production, use cert-manager for metrics TLS by enabling the cert patch in `config/default/kustomization.yaml`.

### Manual Scrape Configuration

```yaml
# prometheus.yml
scrape_configs:
- job_name: 'lynq'
  kubernetes_sd_configs:
  - role: pod
    namespaces:
      names:
      - lynq-system
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_label_control_plane]
    action: keep
    regex: controller-manager
  - source_labels: [__meta_kubernetes_pod_container_port_name]
    action: keep
    regex: https
```

## Logging

### Log Levels

Configure via `--zap-log-level`:

```yaml
args:
- --zap-log-level=info  # Options: debug, info, error
```

**Levels:**
- `debug`: Verbose logging (template values, API calls)
- `info`: Standard logging (reconciliation events)
- `error`: Errors only

### Structured Logging

All logs are structured JSON:

```json
{
  "level": "info",
  "ts": "2025-01-15T10:30:00.000Z",
  "msg": "Reconciliation completed",
  "lynqnode": "acme-prod-template",
  "ready": 10,
  "failed": 0,
  "changed": 2
}
```

### Key Log Messages

#### Reconciliation Events

```
"msg": "Reconciliation completed"
"msg": "Reconciliation completed with changes"
"msg": "Failed to reconcile node"
```

#### Resource Events

```
"msg": "Failed to render resource"
"msg": "Failed to apply resource"
"msg": "Resource not ready within timeout"
```

#### Hub Events

```
"msg": "Deleting LynqNode (no longer in desired set)"
"msg": "Successfully deleted LynqNode"
```

### Querying Logs

```bash
# All logs
kubectl logs -n lynq-system deployment/lynq-controller-manager

# Follow logs
kubectl logs -n lynq-system deployment/lynq-controller-manager -f

# Errors only
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep '"level":"error"'

# Specific node
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep 'acme-prod'

# Reconciliation events
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Reconciliation completed"
```

## Events

Kubernetes events are emitted for key operations.

### Viewing Events

```bash
# All LynqNode events
kubectl get events --all-namespaces --field-selector involvedObject.kind=LynqNode

# Specific LynqNode
kubectl describe lynqnode <name>

# Recent events
kubectl get events --sort-by='.lastTimestamp'
```

### Event Types

#### Normal Events

- `TemplateApplied`: Template successfully applied
- `TemplateAppliedComplete`: All resources applied
- `LynqNodeDeleting`: LynqNode deletion started
- `LynqNodeDeleted`: LynqNode deletion completed

#### Warning Events

- `TemplateRenderError`: Template rendering failed
- `ApplyFailed`: Resource apply failed
- `ResourceConflict`: Ownership conflict detected
- `ReadinessTimeout`: Resource not ready within timeout
- `DependencyError`: Dependency cycle detected
- `LynqNodeDeletionFailed`: Node deletion failed

### Event Examples

```bash
# Success
TemplateAppliedComplete: Applied 10 resources (10 ready, 0 failed, 2 changed)

# Conflict
ResourceConflict: Resource conflict detected for default/acme-app (Kind: Deployment, Policy: Stuck).
Another controller or user may be managing this resource.

# Deletion
LynqNodeDeleting: Deleting Node 'acme-prod-template' (template: prod-template, uid: acme) -
no longer in active dataset. This could be due to: row deletion, activate=false, or template change.
```

## Dashboards

### Grafana Dashboard

A comprehensive Grafana dashboard is available at: `config/monitoring/grafana-dashboard.json`

**How to import:**

1. Open Grafana UI
2. Go to Dashboards → Import
3. Upload `config/monitoring/grafana-dashboard.json`
4. Select your Prometheus datasource

**Dashboard includes 10 panels:**
1. **Reconciliation Duration (Percentiles)** - P50, P95, P99 latency
2. **Reconciliation Rate** - Success vs Error rate
3. **Error Rate** - Gauge showing current error percentage
4. **Total Desired LynqNodes** - Sum across all registries
5. **Total Ready LynqNodes** - Healthy node count
6. **Total Failed LynqNodes** - Failed node count
7. **Resource Counts by LynqNode** - Stacked area chart per node
8. **Hub Health** - Table showing health percentage per hub
9. **Apply Rate by Kind** - Apply attempts by resource type
10. **Work Queue Depth** - Controller queue depths

## Alerting

### Prometheus Alert Rules

A comprehensive set of Prometheus alert rules is available at: **`config/prometheus/alerts.yaml`**

**To deploy the alerts:**

```bash
# Apply the PrometheusRule resource
kubectl apply -f config/prometheus/alerts.yaml

# Or use kustomize
kubectl apply -k config/prometheus
```

### Alert Categories

The alert configuration includes three severity levels:

| Severity | Alerts | Description |
|----------|--------|-------------|
| **Critical** | 5 alerts | Immediate action required - production impact |
| **Warning** | 8 alerts | Investigation needed - potential issues |
| **Info** | 1 alert | Informational - awareness only |

**Critical Alerts:**
- `LynqNodeDegraded` - LynqNode in degraded state
- `LynqNodeResourcesFailed` - LynqNode has failed resources
- `LynqNodeNotReady` - LynqNode not ready for extended period
- `LynqNodeStatusUnknown` - LynqNode condition status unknown
- `HubManyNodesFailure` - Many nodes failing in a hub

**Warning Alerts:**
- `LynqNodeResourcesMismatch` - Ready count doesn't match desired
- `LynqNodeResourcesConflicted` - Resources in conflict state
- `LynqNodeHighConflictRate` - High rate of conflicts
- `HubNodesFailure` - Some nodes failing
- `HubSyncIssues` - Hub sync problems
- `LynqNodeReconciliationErrors` - High error rate
- `LynqNodeReconciliationSlow` - Slow reconciliation performance
- `HighApplyFailureRate` - High apply failure rate

**Info Alerts:**
- `LynqNodeNewConflictsDetected` - New conflicts detected

::: tip Alert Configuration
For complete alert definitions with thresholds and runbook links, see `config/prometheus/alerts.yaml`.
:::

### Sample Alert Rules

**Critical:**
```yaml
# LynqNode has failed resources
- alert: LynqNodeResourcesFailed
  expr: lynqnode_resources_failed > 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "LynqNode {{ $labels.lynqnode }} has {{ $value }} failed resource(s)"
    runbook_url: "https://lynq.sh/runbooks/node-resources-failed"
```

**Warning:**
```yaml
# Resources in conflict
- alert: LynqNodeResourcesConflicted
  expr: lynqnode_resources_conflicted > 0
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "LynqNode {{ $labels.lynqnode }} has resources in conflict"
    runbook_url: "https://lynq.sh/runbooks/node-conflicts"
```

### Alert Routing (AlertManager)

Configure AlertManager to route alerts based on severity:

```yaml
# alertmanager.yml
route:
  group_by: ['alertname', 'lynqnode', 'namespace']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'default'
  routes:
  # Critical alerts to PagerDuty
  - match:
      severity: critical
    receiver: 'pagerduty'

  # Warning alerts to Slack
  - match:
      severity: warning
    receiver: 'slack'

  # Info alerts to email
  - match:
      severity: info
    receiver: 'email'

receivers:
- name: 'pagerduty'
  pagerduty_configs:
  - service_key: '<pagerduty-key>'

- name: 'slack'
  slack_configs:
  - api_url: '<slack-webhook>'
    channel: '#lynq-alerts'
```

## Best Practices

### 1. Monitor Key Metrics

Essential metrics to track:
- Reconciliation duration (P95)
- Error rate
- Resource ready/failed counts
- Hub desired vs ready

### 2. Set Up Alerts

Minimum recommended alerts:
- Operator down
- High error rate (> 10%)
- Slow reconciliation (P95 > 30s)
- Resources failed (> 0 for 5min)

### 3. Retain Logs

Recommended log retention:
- **Debug logs:** 1-3 days
- **Info logs:** 7-14 days
- **Error logs:** 30+ days

### 4. Dashboard Review

Weekly review:
- Reconciliation performance trends
- Error patterns
- Resource health
- Capacity planning

### 5. Event Monitoring

Monitor events for:
- Conflicts (investigate ownership)
- Timeouts (adjust readiness settings)
- Template errors (fix templates)

## Troubleshooting Metrics

### Metrics Not Available

**Problem:** `curl https://localhost:8443/metrics` returns connection refused.

**Solution:**

1. **Check if metrics port is configured:**
   ```bash
   kubectl get deployment -n lynq-system lynq-controller-manager -o yaml | grep metrics-bind-address
   ```

   Should see: `--metrics-bind-address=:8443`

2. **Check if port is exposed:**
   ```bash
   kubectl get deployment -n lynq-system lynq-controller-manager -o yaml | grep -A 5 "ports:"
   ```

   Should see containerPort 8443.

3. **Check if service exists:**
   ```bash
   kubectl get svc -n lynq-system lynq-controller-manager-metrics-service
   ```

4. **Check operator logs:**
   ```bash
   kubectl logs -n lynq-system deployment/lynq-controller-manager | grep metrics
   ```

### No Metrics Data

**Problem:** Metrics endpoint works but returns no custom metrics.

**Solution:**

1. **Verify metrics are registered:**
   ```bash
   curl -k https://localhost:8443/metrics | grep lynqnode_
   ```

   Should see: `lynqnode_reconcile_duration_seconds`, `lynqnode_resources_ready`, etc.

2. **Trigger reconciliation:**
   ```bash
   # Apply a test resource
   kubectl apply -f config/samples/operator_v1_lynqhub.yaml

   # Wait 30s and check metrics again
   curl -k https://localhost:8443/metrics | grep lynqnode_reconcile_duration_seconds_count
   ```

3. **Check if controllers are running:**
   ```bash
   kubectl logs -n lynq-system deployment/lynq-controller-manager | grep "Starting Controller"
   ```

### ServiceMonitor Not Working

**Problem:** Prometheus not scraping metrics.

**Solution:**

1. **Check if Prometheus Operator is installed:**
   ```bash
   kubectl get crd servicemonitors.monitoring.coreos.com
   ```

2. **Check if ServiceMonitor is created:**
   ```bash
   kubectl get servicemonitor -n lynq-system
   ```

3. **Check ServiceMonitor labels match Prometheus selector:**
   ```bash
   kubectl get servicemonitor -n lynq-system lynq-controller-manager-metrics-monitor -o yaml
   ```

4. **Check Prometheus logs:**
   ```bash
   kubectl logs -n monitoring prometheus-xyz
   ```

### TLS Certificate Errors

**Problem:** `x509: certificate signed by unknown authority`

**Solution:**

For development, use `--insecure` or `-k`:
```bash
curl -k https://localhost:8443/metrics
```

For production, use cert-manager by enabling the cert patch in `config/default/kustomization.yaml`:
```yaml
# Uncomment this line:
#- path: cert_metrics_manager_patch.yaml
```

## See Also

- **[Prometheus Query Examples](prometheus-queries.md)** - 50+ ready-to-use PromQL queries
- **`config/prometheus/alerts.yaml`** - Complete alert rule definitions
- **`config/monitoring/grafana-dashboard.json`** - Grafana dashboard
- [Performance Guide](performance.md) - Performance tuning
- [Troubleshooting Guide](troubleshooting.md) - Common issues
