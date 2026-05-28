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

  # Phase model rollback (emergency only)
  - --legacy-readiness-strict=false            # default false (phase model on);
                                               # set true to revert to pre-phase-model
                                               # strict readiness behavior. See below.
```

### `--legacy-readiness-strict`

Emergency rollback flag for the phase model. When `true`:

- The readiness check reverts to strict equality (every replica must be Available); any partial unavailability past `timeoutSeconds` becomes `Failed` with a `ReadinessTimeout` event.
- The new `Degraded` phase is never observed; `WorkloadDegraded`/`WorkloadRecovered`/`RolloutComplete` events are never emitted.
- The new metric series remain registered but the gauges stay at 0 (existing dashboards keep working without showing the new signals).
- LynqNode.status fields (`degradedResources`, `progressingResources`, `pendingResources`, `resourcePhases`, `degradedResourceIds`) stay at zero / empty.

Intended for production rollback only. The flag is slated for removal after one release cycle if no users opt in. See the [Resource Phases](resource-phases.md) concept page for the design rationale.

**Concurrency guidance:**

| Cluster scale | hub | form | node |
|---------------|-----|------|------|
| Small (<50 nodes) | 3 | 5 | 10 |
| Medium (50–500) | 5 | 8 | 20 |
| Large (500+) | 8 | 10 | 30 |

For more tuning options, see [Performance](performance.md).

## Resource Limits

The shipped manifests and chart use conservative defaults:

```yaml
resources:
  requests:
    cpu: 10m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 128Mi
```

Increase memory before scaling beyond a few hundred LynqNodes — the controller-runtime cache grows with total managed object count. For observed benchmarks, a sizing table, and a CPU/memory model, see [Resource Sizing](resource-sizing.md).

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
