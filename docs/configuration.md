---
description: "Lynq operator configuration reference: controller runtime flags, concurrency knobs (node, hub, form), requeue intervals, and feature gates."
---

# Configuration

This page covers the controller manager's runtime flags. For resource-level configuration (hub connection, form policies, templates), follow the links in each section.

## Controller Flags

Set in `config/manager/manager.yaml` under `spec.template.spec.containers[].args`:

```yaml
args:
  # Leader election
  - --leader-elect=true                        # required in HA deployments

  # Concurrency (tune for cluster scale)
  - --hub-concurrency=3                        # concurrent hub syncs (default: 3)
  - --form-concurrency=5                       # concurrent form reconciliations (default: 5)
  - --node-concurrency=10                      # concurrent node reconciliations (default: 10)

  # Metrics and health
  - --metrics-bind-address=:8443               # Prometheus scrape endpoint (0 = disabled)
  - --health-probe-bind-address=:8081          # liveness/readiness probes
  - --metrics-secure=true                      # serve metrics over HTTPS

  # TLS (cert-manager provisions these automatically)
  - --webhook-cert-path=/tmp/k8s-webhook-server/serving-certs
  - --metrics-cert-path=/tmp/k8s-metrics-server/serving-certs

  # Logging
  - --zap-log-level=info                       # debug | info | error
  - --zap-encoder=json                         # json | console
  - --zap-devel=false                          # true enables development mode
```

**Concurrency guidance:**

| Cluster scale | hub | form | node |
|---------------|-----|------|------|
| Small (<50 nodes) | 3 | 5 | 10 |
| Medium (50–500) | 5 | 8 | 20 |
| Large (500+) | 8 | 10 | 30 |

For more tuning options, see [Performance](performance.md).

## Resource Limits

```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

These defaults suit clusters up to ~200 LynqNodes. For observed benchmarks, a sizing table, and a CPU/memory model, see [Resource Sizing](resource-sizing.md).

## Configuration by Topic

| Topic | Where to configure | Reference |
|-------|--------------------|-----------|
| Database connection, sync interval, column mappings | `LynqHub` spec | [Datasource](datasource.md) |
| Resource blueprints, rollout settings | `LynqForm` spec | [Templates](templates.md) |
| CreationPolicy, DeletionPolicy, ConflictPolicy, PatchStrategy | Per-resource in `LynqForm` | [Policies](policies.md) |
| Prometheus metrics, alerting | Operator flags + Prometheus config | [Monitoring](monitoring.md) |
| RBAC, network policies, credentials | Cluster config + SecretRef | [Security](security.md) |

## See Also

- [Installation](installation.md) — deploying the operator
- [Performance](performance.md) — scaling and optimization
- [Security](security.md) — RBAC and credential management
