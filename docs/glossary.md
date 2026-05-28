---
description: "Definitions for Lynq-specific terms: CRDs, labels, annotations, policy types, template functions, and core Kubernetes concepts used throughout the docs."
---

# Glossary

Quick-reference definitions. For deeper explanations, follow the linked pages.

## Index

[A](#a) · [C](#c) · [D](#d) · [E](#e) · [F](#f) · [H](#h) · [L](#l) · [M](#m) · [N](#n) · [O](#o) · [P](#p) · [R](#r) · [S](#s) · [T](#t) · [V](#v) · [W](#w)

---

## A

**activate** — The database column mapped to control whether a row provisions resources. Accepted truthy values: `1`, `true`, `TRUE`, `True`, `yes`, `YES`, `Yes`. Everything else (including `NULL`) is inactive. → [Datasource](datasource.md)

**appliedResources** — A `status` field on LynqNode listing every resource currently managed, in `kind/namespace/name@id` format. Used for orphan detection: resources in `appliedResources` but not in the current template become orphans.

**Available (phase)** — One of five [resource phases](resource-phases.md). A workload is Available when its rollout is complete for the current generation AND fully healthy by native Kubernetes semantics (all replicas serving traffic). Counts toward `readyResources`.

---

## C

**cascade deletion** — Automatic deletion of child resources when a parent is deleted. Deleting a LynqHub removes all its LynqNode CRs; each LynqNode's finalizer then cleans up managed resources per their `deletionPolicy`. → [Policy Operations](policies-operations.md)

**cert-manager** — Required Kubernetes add-on (v1.13.0+) that provisions TLS certificates for Lynq's admission webhooks. → [Installation](installation.md)

**ConflictPolicy** — Per-resource policy controlling behavior when SSA detects a field owner conflict. `Stuck` (default) stops reconciliation and marks the node Degraded; `Force` takes ownership with `force=true`. → [Policies](policies.md)

**CreationPolicy** — Per-resource policy controlling update behavior. `WhenNeeded` (default) re-applies only when the rendered spec changes (and during periodic drift-correction); `Once` creates the resource once and never updates it again. → [Policies](policies.md)

**CRD (Custom Resource Definition)** — Kubernetes extension that adds new resource types. Lynq installs three: `lynqhubs.operator.lynq.sh`, `lynqforms.operator.lynq.sh`, `lynqnodes.operator.lynq.sh`.

---

## D

**DAG (Directed Acyclic Graph)** — The dependency graph built from each resource's `dependIds`. Cycles are rejected at admission time. Lynq performs a topological sort to determine apply order.

**dependIds** — Array field on `TResource`. Lists IDs of resources that must be applied (and ready, if `waitForReady: true`) before this resource is processed. → [Dependencies](dependencies.md)

**DeletionPolicy** — Per-resource policy controlling lifecycle on LynqNode deletion. `Delete` (default) uses `ownerReference` for automatic GC; `Retain` uses label-based tracking and leaves the resource in the cluster. Evaluated at creation time — not deletion time. → [Policies](policies.md)

**Degraded (phase)** — One of five [resource phases](resource-phases.md). A workload reaches Degraded when its rollout completed for the current generation but availability has since dropped (node drain, HPA scale-up, pod eviction, image GC). Kubernetes is converging the workload; **Lynq does NOT mark this as Failed.** Counts toward `readyResources` (the workload is still serving traffic) and `degradedResources`.

**drift detection** — The process of detecting and correcting manual changes to managed resources. Lynq uses event-driven watches (immediate) plus a 30-second periodic requeue for eventual consistency.

---

## E

**extraValueMappings** — Optional LynqHub field for mapping additional database columns to template variables. `extraValueMappings: planId: subscription_plan` makes `.planId` available in all templates. → [Datasource](datasource.md)

---

## F

**fieldManager** — SSA identifier marking which controller owns which fields. Lynq uses `lynq-operator` as its field manager. Other controllers retain ownership of their own fields.

**finalizer** — Kubernetes mechanism preventing resource deletion until cleanup completes. Lynq adds `lynqnode.operator.lynq.sh/finalizer` to every LynqNode CR to ensure managed resources are cleaned up before the CR is removed.

**Form Builder** — A GUI tool in the Lynq dashboard for building LynqForm specs visually. → [Form Builder](template-builder.md)

---

## H

**hub** — Short for LynqHub. See [LynqHub](#lynqhub).

---

## I

**Infrastructure as Data (IaD)** — The paradigm where database rows are the source of truth for infrastructure state. INSERT/UPDATE/DELETE rows → Kubernetes resources appear/change/disappear. → [Introduction](introduction.md)

---

## L

**LynqForm** — CRD defining the Kubernetes resource blueprint for one set of active rows. Each form references a LynqHub and defines which resources (Deployments, Services, etc.) to create per active row, using Go templates. → [API Reference](api-lynqform.md)

**LynqHub** — CRD defining the database connection (MySQL), sync interval, and column mappings. The hub queries the database and creates/deletes LynqNode CRs to match the active row set. → [API Reference](api-lynqhub.md)

**LynqNode** — CRD representing one active row × one LynqForm combination. Created automatically by the LynqHub controller. Tracks the status of all managed resources and drives reconciliation. → [API Reference](api-lynqnode.md)

**`lynq.sh/node`** — Label on cross-namespace resources and namespace resources. Value is the LynqNode CR name. Used for tracking when `ownerReference` can't be used.

**`lynq.sh/node-namespace`** — Label on cross-namespace resources. Value is the LynqNode namespace.

**`lynq.sh/orphaned`** — Label set to `"true"` on retained resources after the LynqNode is deleted or the resource is removed from the template. Find orphans: `kubectl get all -A -l lynq.sh/orphaned=true`.

---

## M

**multi-form** — Configuration where one LynqHub is referenced by multiple LynqForms. Each active row × each form = one LynqNode. A hub with 5 active rows and 3 forms creates 15 LynqNodes.

---

## N

**nameTemplate** — Go template string that generates `metadata.name` for a resource. Must produce a valid Kubernetes name (lowercase, alphanumeric, `-`) up to 63 characters. Use `trunc63` to enforce the limit.

---

## O

**orphan** — A resource previously managed by Lynq that is no longer in the template (or whose LynqNode was deleted). Resources with `DeletionPolicy: Retain` become orphans rather than being deleted.

**orphan markers** — Labels and annotations added to retained resources:
- `lynq.sh/orphaned: "true"`
- `lynq.sh/orphaned-at: (RFC3339 timestamp)`
- `lynq.sh/orphaned-reason: "RemovedFromTemplate"` or `"LynqNodeDeleted"`

**ownerReference** — Kubernetes metadata establishing a parent-child GC relationship. Resources with `DeletionPolicy: Delete` have an `ownerReference` pointing to their LynqNode. Resources with `DeletionPolicy: Retain` do not.

---

## P

**PatchStrategy** — Per-resource policy controlling how updates are applied. `apply` (default, SSA), `merge` (strategic merge patch), `replace` (full replacement). → [Policies](policies.md)

**Phase (resource phase)** — One of five classifications Lynq assigns to each child resource every reconcile: `Pending`, `Progressing`, `Available`, `Degraded`, `Failed`. Derived purely from native Kubernetes status (no annotations written). Source of truth in `status.resourcePhases`. → [Resource Phases](resource-phases.md)

**Progressing (phase)** — Rollout-in-progress phase: the controller has observed the latest generation but rollout criteria aren't met yet. Subject to Lynq's rollout timeout — escalates to `Failed` when `timeoutSeconds` elapses. → [Resource Phases](resource-phases.md)

---

## R

**RecordOps** — Lynq's term for the practice of using database record operations (INSERT/UPDATE/DELETE) as the primary mechanism for infrastructure change. See [Infrastructure as Data](#i).

**reconciliation** — The control loop where the operator compares desired state (templates + DB rows) with actual cluster state and applies changes to converge them. Triggered by DB sync, CRD changes, child resource changes, and a 30-second periodic requeue.

---

## S

**Server-Side Apply (SSA)** — Kubernetes API mechanism for declarative, field-manager-aware updates. Lynq uses SSA as the default apply method. Each controller owns only the fields it sets; other controllers own their fields independently.

**skipOnDependencyFailure** — Boolean field on `TResource` (default: `true`). When `true`, a resource is skipped if any of its dependencies failed. When `false`, it's applied regardless.

**syncInterval** — LynqHub field setting the database poll frequency. Format: Go duration (`30s`, `1m`, `5m`). Default: `30s`. → [Datasource](datasource.md)

---

## T

::: v-pre
**`toHost`** — Custom template function. `{{ .nodeUrl | toHost }}` extracts the hostname from a URL string.
:::

**topological sort** — Algorithm used to determine apply order from the dependency graph. Resources with no dependencies are applied first; dependents follow.

**TResource** — The base structure for every resource entry in a LynqForm. Holds `id`, `spec`, `nameTemplate`, `dependIds`, the four policies, and readiness settings. → [API Reference](api-lynqform.md)

**`trunc63`** — Custom template function. Truncates a string to 63 characters (the Kubernetes name length limit). Use in `nameTemplate` when UIDs may be long.

---

## V

**valueMappings** — Required LynqHub field mapping database column names to the two required template variables: `uid` and `activate`. → [Datasource](datasource.md)

---

## W

**waitForReady** — Boolean field on `TResource` (default: `true`). When `true`, Lynq waits for the resource's ready condition before applying dependent resources. → [Dependencies](dependencies.md)

---

## See Also

- [Introduction](introduction.md) — what Lynq is and when to use it
- [Architecture](architecture.md) — three-controller design
- [Policies](policies.md) — CreationPolicy, DeletionPolicy, ConflictPolicy, PatchStrategy
- [Templates](templates.md) — template syntax and available functions
