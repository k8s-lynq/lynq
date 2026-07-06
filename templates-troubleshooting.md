---
url: 'https://lynq.sh/templates-troubleshooting.md'
description: >-
  Debug Lynq templates: preview rendering, interpret error messages, and safely
  evolve templates over time with orphan cleanup and re-adoption.
---

# Template Debugging & Migration

How to debug template rendering issues, interpret common errors, and evolve templates safely over time.

::: tip Related pages

* [Templates Overview](templates.md) — Quick start
* [Template Syntax Reference](templates-syntax.md) — Variables and functions
* [Type Conversion](templates-typed-values.md) — `int`, `float`, `bool`
  :::

***

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

***

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

***

## Common Errors

**`function "unknownFunc" not defined`**

* Using a function that doesn't exist. Check [Sprig docs](https://masterminds.github.io/sprig/) or [custom functions](templates-syntax.md#built-in-custom-functions).

***

**`map has no entry for key "missingVar"`**

* Referencing a variable that isn't defined. Add a `default` or check `extraValueMappings`.

::: v-pre

```yaml
# Fix: provide a default
value: "{{ .optionalField | default \"default-value\" }}"
```

:::

***

**`yaml: line N: mapping values are not allowed in this context`**

* Missing quotes around a template expression.

::: v-pre

```yaml
# Wrong
value: {{ .uid }}

# Correct
value: "{{ .uid }}"
```

:::

***

**`spec.replicas in body must be of type integer`**

* Kubernetes received a string where it expected an integer.

::: v-pre

```yaml
# Wrong
replicas: "{{ .maxReplicas }}"       # renders as string "3"

# Correct
replicas: "{{ .maxReplicas | int }}" # renders as integer 3
```

:::

***

**`cannot unmarshal string into Go struct field ... of type bool`**

* Boolean field received a string value.

::: v-pre

```yaml
automountServiceAccountToken: "{{ .autoMount | bool }}"
readOnlyRootFilesystem: "{{ .readOnly | bool }}"
```

:::

***

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

***

## See Also

* [Resource Lifecycle](api-lifecycle.md) — Template evolution: adding, modifying, removing resources, and re-adopting orphans.
* [Policies](policies.md) — `deletionPolicy`, `creationPolicy`, `conflictPolicy`
* [Template Syntax Reference](templates-syntax.md) — Variables and functions
* [Type Conversion](templates-typed-values.md) — `int`, `float`, `bool`
