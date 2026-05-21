---
description: "Runbooks for policy operations: cascade deletion protection, conflict resolution, and migrating between policy types."
---

# Policy Operations

Operational runbooks for common policy scenarios: protecting resources from cascade deletion, resolving conflicts, and migrating policy settings on existing resources.

For the reference (what each policy does), see [Policies](policies.md).

## Protecting Resources from Cascade Deletion

Deleting a LynqHub or LynqForm cascades to all LynqNode CRs, which triggers cleanup for every managed resource. If you need to delete or recreate a Hub/Form without losing the underlying resources, set `deletionPolicy: Retain` **before** making structural changes.

::: warning DeletionPolicy is evaluated at creation time
`ownerReference` is set (or not) when a resource is first created. If you're about to delete a Hub or Form and want to preserve the resources underneath, you must update the policy and trigger a reconcile *first*, then delete.
:::

### Setting Up Retain Before Deletion

**Step 1: Update the LynqForm to use Retain for all resources you want to keep**

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: my-form
spec:
  hubId: my-hub
  deployments:
    - id: app
      deletionPolicy: Retain
      nameTemplate: "{{ .uid }}-app"
      spec: ...
  persistentVolumeClaims:
    - id: data
      deletionPolicy: Retain
      nameTemplate: "{{ .uid }}-data"
      spec: ...
```

**Step 2: Apply the updated form and trigger reconciliation**

```bash
kubectl apply -f updated-lynqform.yaml

# Force reconciliation on all nodes for this form
# (LynqNode has no lynq.sh/form label; filter by spec.templateRef instead.)
kubectl get lynqnodes -n lynq-system -o json | \
  jq -r '.items[] | select(.spec.templateRef == "my-form") | .metadata.name' | \
  xargs -I{} kubectl annotate lynqnode {} -n lynq-system \
    lynq.sh/force-reconcile=$(date +%s) --overwrite
```

**Step 3: Verify resources are no longer using ownerReferences**

```bash
# ownerReferences should be empty for Retain resources
kubectl get deployment acme-app -o jsonpath='{.metadata.ownerReferences}'
```

**Step 4: Now it's safe to delete the Hub or Form**

```bash
kubectl delete lynqhub my-hub
# LynqNode CRs are deleted; resources remain in cluster
```

**Step 5: Find retained resources**

```bash
kubectl get all -A -l lynq.sh/orphaned=true
```

### Instead of Delete: Update In Place

Prefer updating over deleting when possible:

```bash
# ❌ Avoid: triggers cascade deletion
kubectl delete lynqhub my-hub
kubectl apply -f updated-hub.yaml

# ✅ Prefer: in-place update, no cascade
kubectl apply -f updated-hub.yaml
```

## Conflict Resolution

### Step-by-Step: Resolving a Stuck Conflict

**Symptom:** LynqNode shows `DEGRADED=true`

```bash
kubectl get lynqnodes -n lynq-system
NAME                      READY   DESIRED   FAILED   DEGRADED   AGE
acme-web-my-form          2/3     3         0        true       10m
```

**Step 1: Identify the conflicted resource**

```bash
kubectl get lynqnode acme-web-my-form -n lynq-system \
  -o jsonpath='{.status.conditions[?(@.type=="Degraded")].message}'
# Deployment default/acme-app managed by 'helm', not 'lynq'
```

**Step 2: Find who owns the resource**

```bash
kubectl get deployment acme-app -o yaml | grep -A10 managedFields
# Look for: manager: helm (or whatever tool owns it)
```

**Step 3: Choose a resolution strategy**

| Strategy | When to use | Action |
|----------|-------------|--------|
| Delete and let Lynq recreate | Resource should be Lynq-managed | `kubectl delete deployment acme-app` |
| Switch to `Force` policy | Lynq should take ownership | Update LynqForm `conflictPolicy: Force` |
| Rename the resource | Keep both managed independently | Change `nameTemplate` to a unique name |
| Remove from LynqForm | Keep existing, stop Lynq managing it | Remove the resource block from the form |

**Step 4: Trigger reconciliation and verify**

```bash
kubectl annotate lynqnode acme-web-my-form -n lynq-system \
  lynq.sh/force-reconcile=$(date +%s) --overwrite

kubectl get lynqnode acme-web-my-form -n lynq-system
# DEGRADED should now be false
```

### Bulk Conflict Check

```bash
# Find all degraded nodes
kubectl get lynqnodes -A -o json | \
  jq -r '.items[] | select(.status.conditions[]?.type=="Degraded" and .status.conditions[]?.status=="True") | .metadata.name'

# Watch for conflict events
kubectl get events -A --field-selector reason=ResourceConflict
```

## Policy Migration Runbooks

::: warning
Policy changes affect future behavior only — not the current state of existing resources. Follow these steps to safely transition existing managed resources.
:::

### Delete → Retain

**Goal:** Preserve resources that were previously set to auto-delete.

**Step 1: Update the LynqForm**

```yaml
deployments:
  - id: app
    deletionPolicy: Retain  # was: Delete
```

**Step 2: Apply and trigger reconciliation**

```bash
kubectl apply -f updated-lynqform.yaml
kubectl annotate lynqnode <node-name> -n lynq-system \
  lynq.sh/force-reconcile=$(date +%s) --overwrite
```

**Step 3: Verify ownerReference is removed**

```bash
kubectl get deployment <resource-name> -o jsonpath='{.metadata.ownerReferences}'
# Should be empty
```

### Retain → Delete

**Goal:** Enable automatic cleanup for resources that were previously retained.

::: warning
Resources will be deleted automatically when LynqNode is deleted after this change.
:::

**Step 1: Audit what will be affected**

```bash
kubectl get all -l lynq.sh/node=<lynqnode-name> -n <namespace>
```

**Step 2: Update the LynqForm**

```yaml
deployments:
  - id: app
    deletionPolicy: Delete  # was: Retain
```

**Step 3: Apply and trigger reconciliation**

```bash
kubectl apply -f updated-lynqform.yaml
kubectl annotate lynqnode <node-name> -n lynq-system \
  lynq.sh/force-reconcile=$(date +%s) --overwrite
```

**Step 4: Verify ownerReference is restored**

```bash
kubectl get deployment <resource-name> -o jsonpath='{.metadata.ownerReferences[0].name}'
# Should show the LynqNode name
```

### Stuck → Force

**Goal:** Allow Lynq to take ownership of conflicted resources.

**Step 1: Identify currently conflicted resources**

```bash
kubectl get lynqnode <node-name> -n lynq-system \
  -o jsonpath='{.status.conditions[?(@.type=="Degraded")].message}'
```

**Step 2: Update the LynqForm**

```yaml
deployments:
  - id: app
    conflictPolicy: Force  # was: Stuck
```

**Step 3: Apply and watch for ForceApply events**

```bash
kubectl apply -f updated-lynqform.yaml
kubectl get events -n <namespace> --field-selector reason=ForceApply -w
```

**Step 4: Verify ownership transferred**

```bash
kubectl get deployment <resource-name> -o yaml | grep -A5 managedFields
# manager should now be: lynq
```

### Once → WhenNeeded

**Goal:** Allow updates to resources that were created with `Once`.

**Step 1: Remove the Once annotation from existing resources**

```bash
kubectl patch deployment <resource-name> --type=json \
  -p='[{"op":"remove","path":"/metadata/annotations/lynq.sh~1created-once"}]'
```

**Step 2: Update the LynqForm**

```yaml
deployments:
  - id: app
    creationPolicy: WhenNeeded  # was: Once
```

**Step 3: Apply and verify updates are applied**

```bash
kubectl apply -f updated-lynqform.yaml
# Make a template change and confirm the resource is updated
```

### Pre-Migration Checklist

Before any policy migration:

- [ ] Back up current resource state: `kubectl get <resource> -o yaml > backup.yaml`
- [ ] Identify all affected LynqNodes: `kubectl get lynqnodes -l lynq.sh/hub=<hub-name>`
- [ ] Plan for downtime if needed (especially Delete policy changes)
- [ ] Test in a non-production environment first
- [ ] Monitor operator logs during migration: `kubectl logs -n lynq-system -l control-plane=controller-manager -f`

## See Also

- [Policies](policies.md) — policy reference (what each policy does)
- [Policy Examples](policies-examples.md) — worked examples with diagrams
- [Field-Level Ignore](field-ignore.md) — ignore specific fields during reconciliation
