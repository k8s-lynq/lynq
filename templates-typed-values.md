---
url: 'https://lynq.sh/templates-typed-values.md'
description: >-
  Type conversion in Lynq templates: use int, float, and bool functions to
  produce correctly-typed values for Kubernetes resource fields.
---

# Template Type Conversion

How to produce correctly-typed integers, floats, and booleans in Lynq templates.

::: tip Related pages

* [Templates Overview](templates.md) — Quick start
* [Template Syntax Reference](templates-syntax.md) — Variables and functions
  :::

## The YAML Quoting Problem

When writing LynqForm CRDs in YAML, template expressions **must** be quoted because the YAML parser requires it:

```yaml
# INVALID YAML — parser error
containerPort: {{ .appPort }}

# Valid YAML — must quote template expressions
containerPort: "{{ .appPort }}"
```

**The problem:** Quotes make the YAML valid, but the rendered output remains a string, causing Kubernetes API validation to fail for numeric and boolean fields:

```yaml
# After rendering without type functions
containerPort: "8080"  # Kubernetes rejects — expected integer
replicas: "3"          # Kubernetes rejects — expected integer
enabled: "true"        # Kubernetes rejects — expected boolean

# What Kubernetes expects
containerPort: 8080    # integer
replicas: 3            # integer
enabled: true          # boolean
```

***

## Type Conversion Functions

### `int(value)`

Convert to integer:

::: v-pre

```yaml
replicas: "{{ .maxReplicas | int }}"                   # "3" → 3
containerPort: "{{ .appPort | int }}"                  # "8080" → 8080
replicas: "{{ .replicas | default \"2\" | int }}"      # chain with default
value: "{{ .cpuCount | int }}"                         # 2.8 → 2 (truncates)
```

:::

**Conversion rules:**

* String `"123"` → `123`
* Float `2.8` → `2` (truncates)
* Already int → returns as-is
* Invalid input → `0` (graceful fallback)

### `float(value)`

Convert to float64:

::: v-pre

```yaml
resources:
  limits:
    cpu: "{{ .cpuLimit | float }}"              # "1.5" → 1.5
targetCPUUtilization: "{{ .threshold | float }}" # "75.5" → 75.5
```

:::

**Conversion rules:**

* String `"1.5"` → `1.5`
* Int `2` → `2.0`
* Already float → returns as-is
* Invalid input → `0.0`

### `bool(value)`

Convert to boolean:

::: v-pre

```yaml
enabled: "{{ .featureEnabled | bool }}"                     # "true" → true
readOnly: "{{ .isReadOnly | bool }}"                        # "false" → false
automountServiceAccountToken: "{{ .autoMount | bool }}"    # 1 → true, 0 → false
```

:::

**Truthy values** → `true`:

* Strings: `"true"`, `"True"`, `"TRUE"`, `"1"`, `"yes"`, `"Yes"`, `"YES"`, `"y"`, `"Y"`
* Numbers: any non-zero integer (`1`, `42`, `-5`)
* Boolean: `true`

**Falsy values** → `false`:

* Strings: `"false"`, `"False"`, `"FALSE"`, `"0"`, `"no"`, `"No"`, `"NO"`, `"n"`, `"N"`, `""` (empty)
* Numbers: `0`
* Boolean: `false`

***

## When to Use Type Conversion

::: tip Use type conversion for

* **Integers**: `replicas`, `containerPort`, `targetPort`, `minReplicas`, `maxReplicas`
* **Floats**: `cpu` resource limits/requests, `targetCPUUtilizationPercentage`
* **Booleans**: `automountServiceAccountToken`, `readOnlyRootFilesystem`, `privileged`

:::

::: info Don't use type conversion for

* Environment variable values — always strings in containers
* Labels and annotations — always strings
* Command arguments — always strings
* ConfigMap/Secret data values — always strings
* Image tags — always strings even if numeric-looking (`"1.2.3"`)

:::

***

## Complete Example

::: v-pre

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: typed-app
spec:
  hubId: production-db
  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-api"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: "{{ .maxReplicas | default \"2\" | int }}"
          template:
            spec:
              automountServiceAccountToken: "{{ .mountToken | bool }}"
              containers:
                - name: app
                  image: "{{ .image }}"
                  ports:
                    - containerPort: "{{ .appPort | int }}"
                      protocol: TCP
                  env:
                    # String fields — no conversion needed
                    - name: APP_ENV
                      value: "{{ .environment }}"
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    # Integer env var — keep as string (env values are always strings)
                    - name: MAX_CONNECTIONS
                      value: "{{ .maxConns }}"
                  resources:
                    limits:
                      cpu: "{{ .cpuLimit | float }}"
                      memory: "{{ .memoryLimit }}Mi"
                    requests:
                      cpu: "{{ .cpuRequest | float }}"
                      memory: "{{ .memoryRequest }}Mi"
```

:::

***

## How It Works

The `int`/`float`/`bool` functions use **type markers** internally to survive the template rendering boundary:

1. **Template function wraps the result:**
   ```go
   int("42") → "__LYNQ_TYPE_INT__42"  // internal representation
   ```

2. **Go template engine processes normally** — the marker is treated as a string and survives evaluation.

3. **Controller detects and restores the type:**
   ```go
   renderUnstructured() detects marker → converts to native Go int → 42
   ```

4. **Kubernetes receives a correctly-typed value:**
   ```yaml
   containerPort: 42  # pure integer, no quotes
   ```

::: details Why not use Sprig's `atoi`?
Go's `text/template` engine always returns rendered results as **strings**, regardless of what type a function returns internally. Sprig's `atoi` returns an integer during template execution, but the final output is still a string.

The type marker approach is the only way to preserve type information across the template rendering boundary.
:::

## See Also

* [Template Syntax Reference](templates-syntax.md) — All variables and functions
* [Debugging & Migration](templates-troubleshooting.md) — Type-related error messages and fixes
