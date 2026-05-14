---
description: "Right-size the Lynq controller pod with real benchmark data — CPU 0–600m and memory 10–140MB across 5–220 LynqNodes — and understand why resources scale the way they do."
---

# Controller Resource Sizing

Right-size the Lynq controller pod. This page explains *why* Lynq uses resources the way it does, then gives you the numbers to set `requests` and `limits` confidently.

::: tip Time to working
5 minutes to pick the right tier and update your Helm values or manager YAML.
:::

## Observed Usage (Real Data)

| Environment | LynqNodes | CPU (observed) | Memory (observed) |
|-------------|-----------|---------------|-------------------|
| Small | 5 | **0 – 5 m** | **10 – 60 MB** |
| Medium | 220 | **500 – 600 m** | **100 – 140 MB** |

CPU is bursty — it spikes during reconciliation and drops to near-zero between cycles. Memory grows steadily with node count and stays roughly flat within a stable cluster.

## Why Lynq Uses Resources This Way

Understanding the two resource drivers makes sizing intuitive.

### CPU — driven by reconciliation workload

Lynq's CPU usage is almost entirely **reconciliation work**, not background polling. The controller sits idle until something triggers a reconcile, then it processes work concurrently up to the `--node-concurrency` limit.

Each LynqNode reconciliation involves:
1. **Template rendering** — evaluate Go templates for each managed resource
2. **Dependency graph traversal** — topological sort of `dependIds`
3. **Kubernetes API calls** — one Server-Side Apply (SSA) call per resource
4. **Status update** — patch the LynqNode status and conditions

With 220 nodes, default `--node-concurrency=10`, and a 30-second periodic requeue, the operator runs roughly **7 reconciliations per second** at steady state. That lands at 500–600 m CPU — close to one full core.

**Why CPU is spiky:** during a hub sync (every `syncInterval`, e.g. 1 minute), all out-of-date nodes are enqueued at once. CPU jumps to the peak value briefly, then falls back to near-idle as the queue drains.

### Memory — driven by the controller cache

The controller-runtime framework keeps an **in-memory cache** of every Kubernetes object it watches. This cache enables instant reads without hitting the API server on every reconcile. It's the dominant memory consumer.

What lives in the cache:

| Cache contents | 5 nodes estimate | 220 nodes estimate |
|----------------|-----------------|-------------------|
| LynqNode CRs | ~75 KB | ~3.3 MB |
| Managed resources (5 avg per node) | ~0.1 MB | ~55–110 MB |
| LynqHub + LynqForm CRs | ~50 KB | ~50 KB |
| Go runtime + controller overhead | ~20–30 MB | ~20–30 MB |
| **Total** | **~20–30 MB** | **~80–140 MB** |

Each cached Kubernetes object is a deserialized Go struct (roughly 50–100 KB per resource object). With 220 nodes × ~5 resources each = ~1,100 objects in cache, the math lands in the observed 100–140 MB range.

**Memory is stable, not spiky:** once the cache is warm it stays roughly flat. You won't see memory grow over time in a stable cluster — growth only happens when node count increases.

## Sizing Recommendations

Set `requests` to match observed usage at your expected node count. Set `limits` to 2–3× requests to absorb burst without throttling — especially for CPU.

| Node count | CPU request | CPU limit | Memory request | Memory limit |
|-----------|------------|-----------|----------------|--------------|
| < 50 | `50m` | `200m` | `64Mi` | `128Mi` |
| 50 – 200 | `200m` | `500m` | `128Mi` | `256Mi` |
| 200 – 500 | `500m` | `1000m` | `256Mi` | `512Mi` |
| 500 – 1000 | `1000m` | `2000m` | `512Mi` | `1Gi` |
| 1000+ | `2000m` | `4000m` | `1Gi` | `2Gi` |

::: warning Never set memory limit below memory request
If the pod is OOMKilled it restarts, losing its cache. On restart it must re-sync everything — causing a CPU spike and temporary reconciliation lag. Leave at least 2× headroom above observed memory.
:::

### Applying the limits

**Helm:**

```yaml
# values.yaml
resources:
  requests:
    cpu: 500m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi
```

**Kustomize / direct manifest:**

```yaml
# config/manager/manager.yaml
containers:
  - name: manager
    resources:
      requests:
        cpu: 500m
        memory: 256Mi
      limits:
        cpu: 1000m
        memory: 512Mi
```

## Scaling Projection

Use this rough model to project usage before you deploy at scale:

```
CPU (steady state)  ≈ node_count × 2.5 m
Memory (warm cache) ≈ 30 MB + (node_count × 0.5 MB)
```

| Projected nodes | CPU (steady) | Memory (warm) |
|----------------|-------------|--------------|
| 50 | ~125 m | ~55 MB |
| 220 | ~550 m | ~140 MB |
| 500 | ~1250 m | ~280 MB |
| 1000 | ~2500 m | ~530 MB |

These are steady-state estimates. Add 50–100% headroom for burst and reconciliation spikes.

## What Goes Wrong with Under-sized Resources

### CPU throttling

If the CPU limit is too low, the kernel rate-limits the container. Reconciliations slow down — you'll see longer reconcile durations in `lynqnode_reconcile_duration_seconds` without any error. Everything still works, just slower.

**Signal:** `container_cpu_cfs_throttled_seconds_total` increases. Reconciliation P95 > 15 s.

**Fix:** raise the CPU limit, or increase `--node-concurrency` to spread work differently.

### Memory OOMKill

If the memory limit is too close to the working set, the pod is killed and restarted when the cache reaches the limit. After restart, the controller re-builds the cache from scratch — a cold-start reconciliation wave that spikes CPU temporarily.

**Signal:** pod restarts increasing (`kube_pod_container_status_restarts_total`). `OOMKilled` in `kubectl describe pod`.

**Fix:** raise the memory limit. Never set it below `30 MB + (node_count × 0.6 MB)`.

## How to Verify Current Usage

```bash
# Live resource usage
kubectl top pod -n lynq-system

# See limits and requests
kubectl get pod -n lynq-system -l control-plane=controller-manager \
  -o jsonpath='{range .items[*]}{.spec.containers[*].resources}{"\n"}{end}'

# CPU throttling rate (>5% is worth investigating)
kubectl exec -n lynq-system deployment/lynq-controller-manager -- \
  cat /sys/fs/cgroup/cpu/cpu.stat 2>/dev/null || \
  kubectl top pod -n lynq-system --containers
```

Prometheus query for memory headroom:

```promql
# Memory usage as % of limit (alert if > 80%)
container_memory_working_set_bytes{container="manager", namespace="lynq-system"}
/ container_spec_memory_limit_bytes{container="manager", namespace="lynq-system"} * 100
```

## Concurrency and Resource Trade-off

Higher `--node-concurrency` increases CPU but decreases reconciliation latency. Lower concurrency reduces CPU but slows down mass reconciliations.

| `--node-concurrency` | CPU at 220 nodes | Time to reconcile 220 nodes |
|---------------------|-----------------|----------------------------|
| 5 | ~300 m | ~60–90 s |
| 10 (default) | ~550 m | ~30–45 s |
| 20 | ~900 m | ~15–25 s |

If you see high CPU but don't need fast mass reconciliation (e.g., your nodes change infrequently), reduce concurrency first before reducing limits.

## See Also

- [Performance Tuning](performance.md) — sync intervals, dependency depth, template optimization
- [Configuration](configuration.md) — `--node-concurrency`, `--hub-concurrency`, `--form-concurrency` flags
- [Monitoring](monitoring.md) — Prometheus metrics for reconcile duration and resource usage
- [Prometheus Queries](prometheus-queries.md) — ready-to-use PromQL for sizing decisions
