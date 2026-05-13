---
description: "Debug Lynq templates: preview rendering, interpret error messages, and safely evolve templates over time with orphan cleanup and re-adoption."
---

# Template Debugging & Migration

How to debug template rendering issues, interpret common errors, and evolve templates safely over time.

::: tip Related pages
- [Templates Overview](templates.md) — Quick start
- [Template Syntax Reference](templates-syntax.md) — Variables and functions
- [Type Conversion](templates-typed-values.md) — `int`, `float`, `bool`
:::

---

## Debugging Templates

### Method 1: Check Rendered Values in an Existing LynqNode

The LynqNode CR stores the fully-rendered spec — templates are already evaluated:

::: v-pre

```bash
# View the rendered deployment spec
kubectl get lynqnode acme-corp-customer-web-app -n lynq-system \
    -o jsonpath='{.spec.deployments[0]}' | jq .

# Check a specific field
kubectl get lynqnode acme-corp-customer-web-app -n lynq-system \
    -o jsonpath='{.spec.deployments[0].spec.replicas}'
# Output: 3  (integer, not "3")
```

:::

### Method 2: Watch Operator Logs

::: v-pre

```bash
kubectl logs -n lynq-system deployment/lynq-controller-manager -f \
    | grep -E "(render|template|error)"

# Successful rendering
# INFO  controller.lynqnode  Rendered template successfully  {"lynqnode": "acme-corp-customer-web-app", "resources": 3}

# Template error
# ERROR controller.lynqnode  Template rendering failed  {"error": "function \"unknownFunc\" not defined"}
```

:::

### Method 3: Check LynqNode Events

::: v-pre

```bash
kubectl describe lynqnode acme-corp-customer-web-app -n lynq-system
# Events:
#   Normal  TemplateRendered  5m  Successfully rendered 3 resources
#   Normal  ResourceApplied   5m  Applied Deployment/acme-corp-web
#   Normal  Ready             5m  All resources are ready
```

:::

### Method 4: Simulate Rendering Locally

For quick iteration before deploying:

```go
// test_template.go
package main

import (
    "os"
    "text/template"
    "github.com/Masterminds/sprig/v3"
)

func main() {
    tmpl := `{{ .uid }}-{{ .planType | default "basic" }}`
    t := template.Must(template.New("test").Funcs(sprig.TxtFuncMap()).Parse(tmpl))
    t.Execute(os.Stdout, map[string]string{
        "uid":      "acme-corp",
        "planType": "enterprise",
    })
    // Output: acme-corp-enterprise
}
```

---

## Debugging Decision Tree

```
Template not working?
│
├─ YAML parse error?
│  └─ Are template expressions quoted? → "{{ .value }}"
│
├─ Function not defined?
│  └─ Check Sprig docs or custom functions (toHost, sha1sum, etc.)
│
├─ Variable not found?
│  └─ Is it in valueMappings or extraValueMappings?
│
├─ Type error (string vs int/bool)?
│  └─ Use int/float/bool for Kubernetes fields → see Type Conversion
│
├─ Resource not created?
│  └─ Check LynqNode events and operator logs
│
└─ Resource in wrong state?
   └─ Are dependencies satisfied? Are policies correct?
```

---

## Common Errors

**`function "unknownFunc" not defined`**

- Using a function that doesn't exist. Check [Sprig docs](https://masterminds.github.io/sprig/) or [custom functions](templates-syntax.md#built-in-custom-functions).

---

**`map has no entry for key "missingVar"`**

- Referencing a variable that isn't defined. Add a `default` or check `extraValueMappings`.

::: v-pre

```yaml
# Fix: provide a default
value: "{{ .optionalField | default \"default-value\" }}"
```

:::

---

**`yaml: line N: mapping values are not allowed in this context`**

- Missing quotes around a template expression.

::: v-pre

```yaml
# Wrong
value: {{ .uid }}

# Correct
value: "{{ .uid }}"
```

:::

---

**`spec.replicas in body must be of type integer`**

- Kubernetes received a string where it expected an integer.

::: v-pre

```yaml
# Wrong
replicas: "{{ .maxReplicas }}"       # renders as string "3"

# Correct
replicas: "{{ .maxReplicas | int }}" # renders as integer 3
```

:::

---

**`cannot unmarshal string into Go struct field ... of type bool`**

- Boolean field received a string value.

::: v-pre

```yaml
automountServiceAccountToken: "{{ .autoMount | bool }}"
readOnlyRootFilesystem: "{{ .readOnly | bool }}"
```

:::

---

## Migration from `.hostOrUrl`/`.host`

::: danger Removed in v1.3.0
`.hostOrUrl` and `.host` variables are deprecated since v1.1.11 and **removed in v1.3.0**.
:::

**Before (deprecated):**

::: v-pre

```yaml
# In LynqHub
spec:
  valueMappings:
    uid: node_id
    hostOrUrl: domain_url   # deprecated
    activate: is_active

# In LynqForm
env:
  - name: HOST
    value: "{{ .host }}"    # deprecated
```

:::

**After (v1.1.11+):**

::: v-pre

```yaml
# In LynqHub
spec:
  valueMappings:
    uid: node_id
    activate: is_active
  extraValueMappings:
    domainUrl: domain_url   # map column to custom variable

# In LynqForm
env:
  - name: HOST
    value: "{{ .domainUrl | toHost }}"  # use toHost() function
```

:::

---

## Template Evolution

Lynq handles template changes at runtime. Resources are added, updated, or removed automatically on the next reconcile.

### Adding Resources

New resources are created for all existing LynqNodes using this template:

::: v-pre

```yaml
# Add a new service — created for all existing nodes on next sync
services:
  - id: api-service
    nameTemplate: "{{ .uid }}-api"
    spec:
      apiVersion: v1
      kind: Service
      # ...
```

:::

### Modifying Resources

Resources are updated according to their `patchStrategy` (default: SSA — only managed fields are changed).

### Removing Resources

When a resource is removed from the template, Lynq checks its `deletionPolicy`:

- `Delete` (default): resource is deleted from the cluster
- `Retain`: resource is kept with orphan markers added

::: v-pre

```yaml
# Before: 3 deployments
deployments:
  - id: web
    deletionPolicy: Delete
  - id: worker
    deletionPolicy: Retain
  - id: cache
    deletionPolicy: Delete

# After: only web remains
deployments:
  - id: web
    deletionPolicy: Delete
```

:::

**Result:**
- `worker` — retained with orphan labels (no ownerReference was set initially for Retain resources)
- `cache` — deleted from cluster
- `web` — continues to be managed normally

**Orphan markers added to retained resources:**

```yaml
metadata:
  labels:
    lynq.sh/orphaned: "true"
  annotations:
    lynq.sh/orphaned-at: "2025-01-15T10:30:00Z"
    lynq.sh/orphaned-reason: "RemovedFromTemplate"
    lynq.sh/deletion-policy: "Retain"
```

### Re-adopting Orphaned Resources

When you re-add a previously removed resource back to the template, Lynq automatically:
1. Detects the orphaned resource
2. Removes all orphan markers
3. Restores tracking labels and management

::: v-pre

```bash
# Confirm orphan markers are removed after re-adoption
kubectl get deployment acme-worker -o jsonpath='{.metadata.labels.lynq\.sh/orphaned}'
# (empty = successfully re-adopted)
```

:::

### Finding Orphaned Resources

::: v-pre

```bash
# List all orphaned resources across all namespaces
kubectl get all -A -l lynq.sh/orphaned=true
```

:::

### Best Practices for Template Changes

1. **Test in non-production first** — validate changes in dev/staging
2. **Use `deletionPolicy: Retain` for stateful resources** — PVCs, databases
3. **Use `creationPolicy: Once` for init resources** — one-time Jobs, seed ConfigMaps
4. **Monitor reconciliation** — watch operator logs during template updates

::: v-pre

```bash
# Check current tracked resources before changing a template
kubectl get lynqnode <name> -o jsonpath='{.status.appliedResources}' | jq .

# Watch reconciliation in real time
kubectl logs -n lynq-system deployment/lynq-controller-manager -f
```

:::

## See Also

- [Policies Guide](policies.md) — `deletionPolicy`, `creationPolicy`, `conflictPolicy`
- [Template Syntax Reference](templates-syntax.md) — Variables and functions
- [Type Conversion](templates-typed-values.md) — `int`, `float`, `bool`
