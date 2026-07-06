---
description: "Lynq's 5-phase resource state model — how the operator distinguishes rollout-in-progress (Lynq's responsibility) from steady-state pod-level disruption (Kubernetes converges, Lynq doesn't attribute failure). Includes per-kind classification rules, metrics, events, and the legacy-strict rollback flag."
---

# Resource Phases

Each child resource that a LynqNode manages is classified into exactly one of **five phases** every reconcile. The phase model lets Lynq distinguish:

- **Rollout in progress** — Lynq just changed the spec. Lynq IS responsible for verifying convergence. Strict readiness criteria + rollout timeout still apply.
- **Steady-state pod-level disruption** — the spec hasn't changed. Kubernetes is converging the workload (node drain, HPA scale-up, pod eviction, image GC, kubelet restart). Lynq does **NOT** attribute failure.

This page is the canonical reference for the phase classifier, the per-resource status it produces, the metrics it emits, and the events it fires.

## Why phases?

Before phases, the readiness checker used strict equality (`updatedReplicas == replicas && availableReplicas == replicas`) for every kind of pod-based workload. Combined with the rollout timeout (`lynq.sh/apply-start-time`, reset only on spec change), this meant:

- A Deployment that was Ready hours ago and lost one pod to a node drain *today* hit `ReadinessTimeout` → `failedResources++` → LynqNode flipped `Degraded`, even though `Deployment.status.conditions[Available]=True`.
- HPA scale-ups that briefly outpaced pod readiness reported failures.
- Kubelet restarts that briefly evicted pods reported failures.

Kubernetes itself has clear semantics for the rollout-vs-steady-state boundary. The phase model just adopts them.

## The 5 phases

| Phase | Condition | LynqNode treatment |
|-------|-----------|--------------------|
| `Pending` | `observedGeneration < generation`, or `spec.replicas==0`, or controller hasn't observed the spec yet | Blocks dependents silently |
| `Progressing` | observedGeneration matches but rollout criteria not yet met | Blocks dependents silently. Subject to **rollout timeout** (`lynq.sh/apply-start-time` mechanism). |
| `Available` | rollout complete AND fully healthy by native K8s semantics | Counts toward `readyResources` |
| `Degraded` | rollout completed for current generation BUT availability dropped post-rollout | Counts toward `readyResources` AND `degradedResources`. **Never transitions to `Failed`** — no steady-state timeout. |
| `Failed` | rollout timeout exceeded while Progressing, OR `Deployment.conditions[Progressing].reason=ProgressDeadlineExceeded`, OR apply error, OR `Job.conditions[Failed]=True` | Counts toward `failedResources` |

`Available` and `Degraded` both contribute to `readyResources` — a Deployment with 2 of 3 pods serving traffic is still considered Ready for LynqNode aggregation. The `degradedResources` count and the dedicated `Degraded` condition with reason `ResourcesDegraded` surface the steady-state disruption separately.

## State diagram

```mermaid
stateDiagram-v2
    [*] --> Pending: apply
    Pending --> Progressing: observedGeneration matches
    Progressing --> Available: rollout complete + healthy
    Progressing --> Failed: rollout timeout / ProgressDeadlineExceeded
    Available --> Degraded: post-rollout pod loss (drain / eviction / HPA)
    Degraded --> Available: Kubernetes converges
    Available --> Progressing: new spec applied (generation bumps)
    Available --> [*]: removed from template
    Degraded --> [*]: removed from template
    Failed --> Progressing: new spec applied (operator retries)
```

Transitions are observed against `node.status.resourcePhases` (the previous reconcile's phase per resource ID). Restart behavior is consistent: a resource that was already `Degraded` at controller restart does NOT re-emit `WorkloadDegraded` because there is no transition.

## Per-kind classification

The classifier is a pure function of the live child object's status fields plus `elapsedSinceApply` (from `lynq.sh/apply-start-time`). No annotations are written for classification — preserves the "exactly one API write per reconcile" invariant.

### Deployment

```
Pending      observedGeneration < generation, or spec.replicas == 0
Progressing  observedGeneration == generation AND rollout has NOT yet converged
Available    rollout converged AND availableReplicas == spec.replicas
Degraded     rollout converged AND availableReplicas < spec.replicas
             (post-rollout disruption — Kubernetes is converging)
Failed       status.conditions[Progressing].reason == "ProgressDeadlineExceeded"
             OR rollout timeout elapsed while Progressing
```

"Rollout converged for the current generation" is detected from **either** of two K8s-native signals (OR — either is sufficient):

1. `status.conditions[Progressing].reason == "NewReplicaSetAvailable"` — the explicit marker that the new ReplicaSet completed.
2. `status.conditions[Available].status == "True"` — set when `availableReplicas` reaches the `minAvailable` threshold.

Both come from Kubernetes itself. Using only signal (1) misclassified fast 1-replica deployments — K8s sometimes never transitions `Progressing.reason` to `NewReplicaSetAvailable` on quick rollouts, while `Available=True` is set reliably. The OR removes that gap. `updatedReplicas == spec.replicas` is NOT a sufficient signal on its own: an image-pull-failing pod still increments `updatedReplicas` as the pod object is created with the new template, even though it never becomes Available.

### StatefulSet

```
Available    observedGeneration == generation AND updatedReplicas == spec.replicas
             AND currentReplicas == spec.replicas
             AND (updateRevision == "" OR currentRevision == updateRevision)
             AND readyReplicas == spec.replicas
Degraded     (above rollout-complete criteria) AND readyReplicas > 0
             AND readyReplicas < spec.replicas
```

`currentRevision == updateRevision` is the StatefulSet-specific "rollout complete" signal. StatefulSet has no equivalent of Deployment's `Available=True` condition, so the classifier additionally requires `readyReplicas > 0` as proof the generation reached partial health — otherwise a never-Ready StatefulSet (e.g., bad image) would be misclassified as Degraded instead of Progressing→Failed.

### DaemonSet

```
Available    observedGeneration == generation
             AND updatedNumberScheduled == desiredNumberScheduled
             AND numberAvailable == desiredNumberScheduled
Degraded     updatedNumberScheduled == desiredNumberScheduled
             AND numberAvailable > 0
             AND numberAvailable < desiredNumberScheduled  ← e.g., node drain
```

Same regression guard as StatefulSet — `numberAvailable > 0` ensures the rollout reached partial health before classifying transient drops as Degraded.

> **Known limitation (StatefulSet / DaemonSet).** The `readyReplicas > 0` /
> `numberAvailable > 0` heuristic can't distinguish "reached full health once,
> then lost some pods" (genuinely Degraded) from "never fully converged — one
> pod is healthy, the rest permanently fail" (should be Progressing → Failed).
> A parallel StatefulSet or a DaemonSet where a subset of pods permanently
> crash-loop (e.g. a bad image on some nodes) while at least one stays Ready is
> classified **Degraded indefinitely** rather than escalating to Failed.
> Deployments are unaffected — they carry the native
> `Progressing.reason == "NewReplicaSetAvailable"` signal that STS/DS lack. A
> precise fix needs "was fully ready once this generation" history that Lynq
> does not currently persist; until then, alert on
> `lynqnode_resource_degraded_since_seconds` for prolonged STS/DS degradation.

### Everything else

`ConfigMap`, `Secret`, `ServiceAccount`, `CronJob`, `PodDisruptionBudget`, `NetworkPolicy` — classified `Available` immediately upon creation. No phase transitions.

`Service` — `Available` immediately for `ClusterIP`/`NodePort`/`Headless`; `LoadBalancer` services Progress until `status.loadBalancer.ingress` is populated, then `Available`. May time out to `Failed` via Lynq's rollout timeout.

`Job` — `Available` on `Complete=True` or `succeeded > 0`; `Failed` immediately on `Failed=True` condition; `Progressing` otherwise.

`Ingress` — `Available` once `status.loadBalancer.ingress` is populated OR `spec.rules` is set; `Progressing`/`Failed` otherwise.

`PVC` — `Available` on `status.phase == "Bound"`; `Progressing`/`Failed` otherwise.

`HPA` — `Available` on `AbleToScale=True` condition; `Progressing`/`Failed` otherwise.

Custom resources — `Available` when `status.conditions[Ready].status == "True"`, or immediately when `status.conditions` is absent (matches the pre-phase-model fallback). `Progressing`/`Failed` otherwise.

None of the above kinds have a `Degraded` phase — the post-rollout-partial-availability concept only applies to pod-based workloads.

## Per-resource visibility

The per-resource phase array is the source of truth for kubectl jsonpath and custom-columns queries:

```yaml
status:
  resourcePhases:
  - id: app-deployment
    kind: Deployment
    name: acme-prod-web
    phase: Degraded
    reason: "availableReplicas=2/3, observedGeneration matched"
    sinceSeconds: 47
  - id: app-service
    kind: Service
    name: acme-prod-web
    phase: Available
```

Quick recipes:

```bash
# Default view — Degraded and Progressing columns surface immediately
kubectl get lynqnode

# Wide view exposes Failed, Skipped, Pending, Conflicted, DegradedIds
kubectl get lynqnode -o wide

# Per-resource phase via custom-columns
kubectl get lynqnode acme-corp-web-app -o custom-columns=\
'NAME:.metadata.name,RESOURCE:.status.resourcePhases[*].id,PHASE:.status.resourcePhases[*].phase'

# Find currently-Degraded resources across the cluster
kubectl get lynqnodes -A -o jsonpath=\
'{range .items[*]}{range .status.resourcePhases[?(@.phase=="Degraded")]}{.id}{"\t"}{.reason}{"\n"}{end}{end}'
```

### Dependency-blocked and skipped resources

Resources that were **not applied this reconcile** because of a dependency
still get a `resourcePhases` entry — as `Pending`, with a reason prefix that
distinguishes the two cases:

- `blocked: waiting for dependency '<id>' to become ready` — the dependency is
  still `Pending`/`Progressing` (within its timeout). Counted in
  `pendingResources`. Applied automatically once the dependency becomes ready.
- `skipped: dependency '<id>' failed (retries next reconcile)` — the dependency
  `Failed` and this resource has `skipOnDependencyFailure: true` (default).
  Counted in `skippedResources` / `skippedResourceIds` (not in
  `pendingResources`), and a `DependencySkipped` event is emitted.

This makes a node stuck at "2/5 ready, 0 failed" self-explanatory: the missing
resources appear as `Pending` with the exact dependency they are waiting on.

### Diagnosing a rollout from the reason string

Deployment `Progressing`/timeout-`Failed` reasons include both counters —
`updatedReplicas=X/N, availableReplicas=Y/N` — so status alone distinguishes:

- `availableReplicas=0/N` → **first provision** (or total outage): nothing is
  serving traffic yet.
- `availableReplicas>0` with `updatedReplicas<N` → an **update rollout on a
  previously-healthy Deployment**: old-generation pods are still serving while
  the new ReplicaSet converges (or times out).

## LynqNode conditions

| Condition | Status | Reasons |
|-----------|--------|---------|
| `Ready` | True when all resources are `Available` OR `Degraded` AND no failures/conflicts | `Reconciled` (True), `ResourcesFailed` / `ResourcesConflicted` / `ResourcesFailedAndConflicted` / `NotAllResourcesReady` (False) |
| `Progressing` | True during reconciliation | `Reconciling` / `ReconcileComplete` |
| `Conflicted` | True when any resource has ownership conflict | `ResourceConflict` / `NoConflict` |
| `Degraded` | True when at least one resource is Failed, Conflicted, OR in Degraded phase | `ResourceFailures` / `ResourceConflicts` / **`ResourcesDegraded`** (new) / **`ResourceFailuresAndDegraded`** (new) / `ResourceFailuresAndConflicts` / `ResourcesNotReady` / `Healthy` |

The `Degraded` condition with reason `ResourcesDegraded` is the new lower-severity signal: **LynqNode.Ready stays True**, but operators can see "Kubernetes is converging some workload disruption that isn't Lynq's fault."

## Metrics

Aggregate (per LynqNode):

```
lynqnode_resources_ready{lynqnode,namespace}        # Available + Degraded
lynqnode_resources_degraded{lynqnode,namespace}     # NEW
lynqnode_resources_progressing{lynqnode,namespace}  # NEW
lynqnode_resources_pending{lynqnode,namespace}      # NEW
lynqnode_resources_failed{lynqnode,namespace}       # narrowed semantics
```

Per-resource (stateset + replica counters):

```
lynqnode_resource_phase{lynqnode,namespace,resource_id,kind,phase}  # value=1 active, 0 others
lynqnode_resource_replicas_desired{...}
lynqnode_resource_replicas_available{...}
lynqnode_resource_replicas_ready{...}
lynqnode_resource_replicas_updated{...}
lynqnode_resource_degraded_since_seconds{...}       # seconds in Degraded; 0 otherwise
```

Transitions and rollout latency:

```
lynqnode_resource_phase_transitions_total{kind,from,to}   # counter
lynqnode_resource_rollout_duration_seconds{kind,result}    # histogram; result=complete|timeout|aborted
```

See [Prometheus Queries](prometheus-queries.md) for PromQL recipes.

## Events

| Event | Type | When |
|-------|------|------|
| `WorkloadDegraded` | Warning | Available → Degraded. Includes resource id and native status snapshot. |
| `WorkloadRecovered` | Normal | Degraded → Available |
| `RolloutComplete` | Normal | Progressing/Pending → Available. Also records `lynqnode_resource_rollout_duration_seconds{result="complete"}`. |
| `ReadinessTimeout` | Warning | Progressing → Failed (rollout timeout elapsed). Narrowed: never fires during steady-state Degraded. |
| `RolloutAborted` | Warning | Progressing → Failed (non-timeout reason: ProgressDeadlineExceeded, apply error). |

Events are emitted only on real transitions — i.e., when the previous reconcile's phase for this resource differs from the current one. No spam on every reconcile.

## What changes for operators

- `failedResources` is now stricter — no false positives from node drain.
- `degradedResources` is new — primary signal for steady-state partial availability.
- LynqNode stays `Ready=True` during steady-state degradation. Use the `Degraded` condition with reason `ResourcesDegraded` (or the metric) to alert on it.
- `ReadinessTimeout` event no longer fires during steady-state Degraded — only during active rollouts.
- New alerts: `LynqNodeWorkloadDegraded` (Warning, 15+ min), `LynqNodeWorkloadSeverelyDegraded` (Critical, single resource > 30 min), `LynqNodeWorkloadFlapping`, `LynqNodeRolloutSlow`. See [Alert Runbooks](alert-runbooks.md).

### Reconcile behavior under other managers

- **Status-only events take a no-apply path.** A child-resource status change that does NOT change the desired spec — an HPA scale, a pod restart, rollout-progress updates from another manager — is reconciled through the lightweight status path: Lynq re-evaluates phases and updates `status`/metrics but does **not** re-render or re-apply. Spec changes, database-driven variable changes (Hub-rewritten annotations), resource failures, and the periodic ~10-min drift sweep still take the full apply path. This keeps routine third-party churn from triggering unnecessary applies.
- **Ignored-field changes no longer churn.** With `patchStrategy: apply` + [`ignoreFields`](field-ignore.md) (e.g. `$.spec.replicas` for HPA), a change the field's owner makes is excluded from Lynq's desired-spec hash — so it triggers no re-apply and does not reset the readiness-timeout / degraded-since clocks. Lynq still reads the live value for phase classification.

## Rollback

For emergency rollback to pre-phase-model behavior, set the controller flag:

```
--legacy-readiness-strict=true
```

Strict equality returns, `Degraded` phase is never observed, `WorkloadDegraded` events are never emitted, `ReadinessTimeout` fires on any partial availability past `timeoutSeconds`. The new metric series remain registered but the gauges stay at 0. Legacy mode also disables the lightweight status-only reconcile path — every event takes the full reconcile, exactly as the pre-phase-model controller behaved. This flag is slated for removal after one release cycle.

> **Note on stale fields after enabling legacy mode.** The legacy path does not
> populate `status.resourcePhases`, `status.degradedResourceIds`, or the
> per-resource `lynqnode_resource_phase` series, so values written by the phase
> model before the flip linger until they are otherwise overwritten. The
> aggregate `degraded/progressing/pending` counts and prom gauges are cleared on
> the next reconcile. If the stale per-resource fields matter for your tooling,
> delete and recreate the affected LynqNodes after toggling the flag.

## See also

- [Architecture](architecture.md) — where the phase model fits in the reconcile pipeline
- [LynqNode API](api-lynqnode.md) — status field reference
- [Monitoring](monitoring.md) — metric and event catalog
- [Prometheus Queries](prometheus-queries.md) — PromQL recipes for the new metrics
- [Alert Runbooks](alert-runbooks.md) — diagnosis steps for the new alerts
- [Troubleshooting](troubleshooting.md) — diagnose "degraded but not failed" scenarios
