---
description: "Right-size the Lynq controller pod with real observed data — CPU 0–600m and memory 10–140MB across 5–220 LynqNodes — and understand why resources scale the way they do."
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

**Benchmark environment for the 220-node data point:**
- 1 LynqHub, **3 LynqForms** (Form A: 9 resources/node · Form B: 3 resources/node · Form C: 2 resources/node)
- ~73 active database rows × 3 forms = ~220 LynqNodes
- Total managed Kubernetes objects in cache: ~1,025

CPU is bursty — it spikes during reconciliation and drops to near-zero between cycles. Memory grows with the number of managed objects and stays roughly flat within a stable cluster.

## Why Lynq Uses Resources This Way

Understanding the two resource drivers makes sizing intuitive.

### CPU — driven by reconciliation workload

Lynq's CPU usage is almost entirely **reconciliation work**, not background polling. The controller sits idle until something triggers a reconcile, then it processes work concurrently up to the `--node-concurrency` limit.

Each LynqNode reconciliation involves:
1. **Template rendering** — evaluate Go templates for each managed resource
2. **Dependency graph traversal** — topological sort of `dependIds`
3. **Kubernetes API calls** — one Server-Side Apply (SSA) call **per resource**
4. **Status update** — patch the LynqNode status and conditions

Steps 3 and 4 dominate. A LynqNode that manages 9 resources makes 9 SSA calls per reconcile — roughly 9× more CPU work than a LynqNode with 1 resource. This means **total managed resources across all LynqNodes** is the primary CPU driver, not just the node count.

**Why CPU is spiky:** during a hub sync (every `syncInterval`, e.g. 1 minute), all out-of-date nodes are enqueued at once. CPU jumps to the peak value briefly, then falls back to near-idle as the queue drains.

### Memory — driven by the controller cache

The controller-runtime framework keeps an **in-memory cache** of every Kubernetes object it watches. This cache enables instant reads without hitting the API server on every reconcile. It is the dominant memory consumer, and it scales with **total managed object count**, not just LynqNode count.

What lives in the cache (benchmark environment at 220 nodes, ~1,025 managed objects):

| Cache contents | 5 nodes estimate | 220 nodes (observed) |
|----------------|-----------------|---------------------|
| LynqNode CRs | ~75 KB | ~3.3 MB |
| Managed resources (~1,025 objects) | ~0.1 MB | ~70–110 MB |
| LynqHub + LynqForm CRs | ~50 KB | ~50 KB |
| Go runtime + controller overhead | ~20–30 MB | ~20–30 MB |
| **Total** | **~20–30 MB** | **~100–140 MB** |

Each cached Kubernetes object is a deserialized Go struct (roughly 70–110 KB). 1,025 objects × ~100 KB/object ≈ 100 MB, which matches the observed range.

**Memory is stable, not spiky:** once the cache is warm it stays flat. Growth happens only when total managed object count increases.

## Sizing Table

Set `requests` to match observed usage at your expected scale. Set `limits` to 2–3× requests to absorb burst — especially for CPU.

The table below uses the benchmark resource density (avg ~4.7 managed resources per LynqNode). See [Adjusting for Different Densities](#adjusting-for-different-resource-densities) if your forms define significantly more or fewer resources.

| LynqNodes | CPU request | CPU limit | Memory request | Memory limit |
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

The accurate way to project resource usage is to estimate **total managed objects** — the sum across all LynqForms of (LynqNodes for that form × resources defined in that form).

```
total_managed = Σ (node_count_per_form × resources_per_form)

CPU (steady state)  ≈ total_managed × 0.55 m
Memory (warm cache) ≈ 30 MB + (total_managed × 0.1 MB)
```

**Benchmark verification:**
```
total_managed = (74 × 9) + (73 × 3) + (73 × 2) = 666 + 219 + 146 = 1,031

CPU  ≈ 1,031 × 0.55 = 567 m  → observed 500–600 m ✓
Mem  ≈ 30 + 1,031 × 0.1 = 133 MB → observed 100–140 MB ✓
```

**Projection table** (same 3-form density as benchmark, avg ~4.7 resources/node):

| Active rows | LynqNodes (×3 forms) | Total managed | CPU (steady) | Memory (warm) |
|-------------|---------------------|---------------|-------------|--------------|
| ~1 | 5 | ~23 | ~13 m | ~32 MB |
| ~17 | 50 | ~235 | ~130 m | ~54 MB |
| ~73 | 220 | ~1,025 | ~565 m | ~133 MB |
| ~170 | 500 | ~2,345 | ~1,290 m | ~265 MB |
| ~335 | 1,000 | ~4,690 | ~2,580 m | ~499 MB |

These are steady-state estimates. Add 50–100% headroom for burst and reconciliation spikes.

## Adjusting for Different Resource Densities

If your LynqForms define significantly more or fewer resources per node than the benchmark (~4.7 avg), the `total_managed` formula is the reliable path.

**Example — heavier setup:**
> 2 LynqForms, each with 15 resources per node. 100 active rows = 200 LynqNodes.
> `total_managed = 2 × 100 × 15 = 3,000`
> `CPU ≈ 3,000 × 0.55 = 1,650 m` — over 1.5 cores, not the ~250 m you'd get from a node-count-only estimate.

**Example — lighter setup:**
> 1 LynqForm with 3 resources per node. 500 active rows = 500 LynqNodes.
> `total_managed = 500 × 3 = 1,500`
> `CPU ≈ 1,500 × 0.55 = 825 m` — less than the 500-node row in the table above suggests.

Use `kubectl top pod -n lynq-system` early in your rollout to validate against the model.

## What Goes Wrong with Under-sized Resources

### CPU throttling

If the CPU limit is too low, the kernel rate-limits the container. Reconciliations slow down — you'll see longer reconcile durations in `lynqnode_reconcile_duration_seconds` without any errors. Everything still works, just slower.

**Signal:** `container_cpu_cfs_throttled_seconds_total` increases. Reconciliation P95 > 15 s.

**Fix:** raise the CPU limit, or reduce `--node-concurrency` to lower the burst peak.

### Memory OOMKill

If the memory limit is too close to the working set, the pod is killed and restarted when the cache fills. After restart, the controller re-builds the cache from scratch — a cold-start reconciliation wave that temporarily spikes CPU.

**Signal:** pod restarts increasing (`kube_pod_container_status_restarts_total`). `OOMKilled` in `kubectl describe pod`.

**Fix:** raise the memory limit. A safe floor is:

```
memory limit ≥ 30 MB + (total_managed × 0.15 MB)
```

For the benchmark at ~1,025 managed objects: `30 + 1,025 × 0.15 ≈ 184 MB` — which explains why the 200–500 node tier uses `256Mi` as the request.

## How to Verify Current Usage

```bash
# Live resource usage
kubectl top pod -n lynq-system

# See configured limits and requests
kubectl get pod -n lynq-system -l control-plane=controller-manager \
  -o jsonpath='{range .items[*]}{.spec.containers[*].resources}{"\n"}{end}'

# Count total managed objects as a sizing check
kubectl get lynqnodes -A --no-headers | wc -l   # LynqNode count
```

Prometheus query for memory headroom:

```promql
# Memory usage as % of limit (alert if > 80%)
container_memory_working_set_bytes{container="manager", namespace="lynq-system"}
/ container_spec_memory_limit_bytes{container="manager", namespace="lynq-system"} * 100
```

## Concurrency and Resource Trade-off

Higher `--node-concurrency` increases burst CPU but decreases reconciliation latency. Lower concurrency reduces the burst peak but slows down mass reconciliations.

The table below is an estimate based on the benchmark environment (220 LynqNodes, ~1,025 managed objects). Actual values depend on your resource density and cluster API server latency.

| `--node-concurrency` | Est. CPU at 220 nodes | Est. time to reconcile all 220 nodes |
|---------------------|----------------------|--------------------------------------|
| 5 | ~300 m | ~60–90 s |
| 10 (default) | ~550 m | ~30–45 s |
| 20 | ~900 m | ~15–25 s |

If you see high CPU but nodes change infrequently, reduce concurrency before raising limits.

## See Also

- [Performance Tuning](performance.md) — sync intervals, dependency depth, template optimization
- [Configuration](configuration.md) — `--node-concurrency`, `--hub-concurrency`, `--form-concurrency` flags
- [Monitoring](monitoring.md) — Prometheus metrics for reconcile duration and resource usage
- [Prometheus Queries](prometheus-queries.md) — ready-to-use PromQL for sizing decisions
