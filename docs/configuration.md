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
  - --leader-elect-lease-duration=15s          # standby waits this long after the last renewal (default: 15s)
  - --leader-elect-renew-deadline=10s          # leader gives up and exits if it cannot renew within this (default: 10s)
  - --leader-elect-retry-period=2s             # interval between acquire/renew attempts (default: 2s)
  - --leader-elect-release-on-cancel=false     # release the lease on graceful shutdown (default: false)

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
  - --kube-api-warnings=deduplicate            # deduplicate | all (API server warnings, see below)
```

**Leader election tuning:**

The defaults are controller-runtime's. The leader must renew the lease within `--leader-elect-renew-deadline`, which is also the HTTP timeout of each renewal request; if it cannot, it gives up leadership and the process exits (`leader election lost`), and the standby takes over once `--leader-elect-lease-duration` has passed since the last renewal. The values must satisfy `lease-duration > renew-deadline > 1.2 × retry-period`; the manager refuses to start otherwise.

Lease renewals are ordinary API requests from the operator's ServiceAccount. Under API Priority and Fairness they share a flow with every other request the operator makes, so a slow API server or a burst of reconcile traffic delays them too. If the manager restarts with `failed to renew lease ... Client.Timeout exceeded` while the API server is healthy, lengthen the timings rather than adding replicas, for example:

```yaml
  - --leader-elect-lease-duration=30s
  - --leader-elect-renew-deadline=20s
  - --leader-elect-retry-period=5s
```

The trade-off is failover time: after a crash (no graceful shutdown), the standby waits up to `lease-duration` before it takes over.

`--leader-elect-release-on-cancel` removes that wait on a graceful shutdown (rolling update, node drain, spot interruption notice). The manager releases the lease only after it has stopped the controllers and the status manager and stopped the caches, so the standby never runs alongside a reconcile of the old leader. The status manager flushes the updates it has batched when it stops; a status update from a reconcile that finishes after that is dropped, and the new leader recomputes it. If a reconcile is still running when the manager's 30s graceful-shutdown timeout expires, the lease is released anyway and the process exits immediately after.

The release happens at the end of that shutdown, so it only takes effect if the pod is still alive then. The kubelet kills the container `terminationGracePeriodSeconds` after SIGTERM, and the shipped manifests set it to 10s (Helm value `terminationGracePeriodSeconds`, `config/manager/manager.yaml` for kustomize): a shutdown that waits on a long reconcile — a hub query can take up to 2m — is killed before it releases the lease, and the standby waits out `lease-duration` as it does without the flag. Nothing unsafe happens, but the flag silently has no effect. To get it reliably, set `terminationGracePeriodSeconds` to at least the 30s shutdown timeout plus a margin, for example:

```yaml
# Helm values
terminationGracePeriodSeconds: 40
manager:
  leaderElectionTuning:
    leaseDuration: "30s"
    renewDeadline: "20s"
    retryPeriod: "5s"
    releaseOnCancel: true
```

In the chart, set all three timings together and with a unit (`"30s"`): a timing left empty keeps its default, so `renewDeadline: "20s"` alone fails `lease-duration > renew-deadline` and the manager refuses to start.

**API server warnings:**

The API server returns a warning header for requests such as using a deprecated API version (for example a `kafka.strimzi.io/v1beta2` `KafkaTopic` in a LynqForm). By default (`--kube-api-warnings=deduplicate`) each distinct warning message is logged once per process, at info level, with the context of the first request that hit it (usually a reconcile; a warning first seen by an informer or leader election carries no reconcile fields). `--kube-api-warnings=all` logs every occurrence, which for a deprecated version in a LynqForm means one line per apply of that resource on every node. Warnings cannot be turned off: the first occurrence is how you learn that a template needs migrating. A restart logs each warning once again.

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
