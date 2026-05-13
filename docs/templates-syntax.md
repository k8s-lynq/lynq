---
description: "Template syntax reference for Lynq: available variables, custom functions (toHost, trunc63, sha1sum, fromJson), Sprig library, and advanced techniques."
---

# Template Syntax Reference

Complete reference for template variables, custom functions, the Sprig library, and advanced techniques.

::: tip Related pages
- [Templates Overview](templates.md) — Quick start and best practices
- [Type Conversion](templates-typed-values.md) — `int`, `float`, `bool` for Kubernetes fields
- [Debugging & Migration](templates-troubleshooting.md) — Errors and template evolution
:::

## Available Variables

::: v-pre

### Required Variables

These are always available from the template context:

```yaml
.uid         # Node unique identifier (from uid mapping)
.activate    # Activation status (from activate mapping)
```

### Context Variables

Automatically provided:

```yaml
.hubId        # LynqHub name
.templateRef  # LynqForm name
```

### Custom Variables

From `extraValueMappings` in LynqHub:

```yaml
spec:
  extraValueMappings:
    planId: subscription_plan
    region: deployment_region
    dbHost: database_host
```

Access in templates:

```yaml
.planId   # Maps to subscription_plan column
.region   # Maps to deployment_region column
.dbHost   # Maps to database_host column
```

:::

### Deprecated Variables

::: danger DEPRECATED
The following variables are **deprecated since v1.1.11** and will be **removed in v1.3.0**:

```yaml
.hostOrUrl   # Original URL/host from hub (from hostOrUrl mapping)
.host        # Auto-extracted host from .hostOrUrl
```

**Migration:** Use `extraValueMappings` + the `toHost()` function instead. See [Debugging & Migration](templates-troubleshooting.md#migration-from-hostorurlhost).
:::

---

## Built-in Custom Functions

### `toHost(url)`

Extract hostname from URL:

::: v-pre

```yaml
# Input: https://acme.example.com:8080/path
# Output: acme.example.com
env:
- name: HOST
  value: "{{ .domainUrl | toHost }}"  # Use with extraValueMappings
```

:::

### `trunc63(s)`

Truncate to 63 characters (Kubernetes name limit):

::: v-pre

```yaml
nameTemplate: "{{ printf \"%s-%s-deployment\" .uid .region | trunc63 }}"
```

:::

### `sha1sum(s)`

Generate SHA1 hash — useful for short, stable, unique resource names:

::: v-pre

```yaml
nameTemplate: "app-{{ .uid | sha1sum | trunc 8 }}"
```

:::

### `fromJson(s)`

Parse JSON string from a database column:

::: v-pre

```yaml
# Database column: config = '{"apiKey":"sk-abc","endpoint":"https://api.example.com"}'
env:
- name: API_KEY
  value: "{{ (.config | fromJson).apiKey }}"
- name: ENDPOINT
  value: "{{ (.config | fromJson).endpoint }}"
```

:::

---

## Sprig Functions (200+)

Full documentation: <https://masterminds.github.io/sprig/>

### String Functions

::: v-pre

```yaml
nameTemplate: "{{ .uid | upper }}"
nameTemplate: "{{ .uid | lower }}"
value: "{{ .name | trim }}"
value: "{{ .uid | replace \".\" \"-\" }}"
value: "{{ .name | quote }}"
```

:::

### Encoding Functions

::: v-pre

```yaml
value: "{{ .secret | b64enc }}"
value: "{{ .encoded | b64dec }}"
value: "{{ .param | urlquery }}"
value: "{{ .data | sha256sum }}"
```

:::

### Default Values

::: v-pre

```yaml
image: "{{ .deployImage | default \"nginx:stable\" }}"
port: "{{ .appPort | default \"8080\" }}"
region: "{{ .region | default \"us-east-1\" }}"
```

:::

### Conditionals

Prefer `ternary` for binary switches — it stays on one line and reads left-to-right:

::: v-pre

```yaml
# ternary vTrue vFalse condition
env:
- name: DEBUG
  value: "{{ ternary \"true\" \"false\" (eq .planId \"enterprise\") }}"
replicas: {{ ternary 5 2 (eq .planId "enterprise") | int }}
```

Use `if/else` when constructing strings from multiple variables or when the condition is compound:

```yaml
- name: HOST
  value: "{{ if and .customDomain (eq .domainVerified \"true\") }}{{ .customDomain }}{{ else }}{{ .uid }}.default.svc{{ end }}"
```

:::

### Lists and Iteration

::: v-pre

```yaml
annotations:
  tags: "{{ list \"app\" .uid .region | join \",\" }}"
```

:::

### Math Functions

::: v-pre

```yaml
value: "{{ add .basePort 1000 }}"
value: "{{ mul .cpuLimit 2 }}"
value: "{{ max .minReplicas 3 }}"
```

:::

---

## Template Rendering Process

### 1. Variable Collection

The hub controller collects variables from the database row and merges them with context:

```
uid       = "acme-corp"
activate  = true
planId    = "enterprise"           # from extraValueMappings
domainUrl = "https://acme.ex.com"  # from extraValueMappings
hubId     = "customer-hub"         # context
templateRef = "customer-web-app"   # context
```

### 2. Template Evaluation

For each resource in the LynqForm:

1. Render `nameTemplate` → resource name
2. Render `labelsTemplate` → labels map
3. Render `annotationsTemplate` → annotations map
4. Render `spec` → recursively render all string values

### 3. Resource Application

The rendered resource is applied to Kubernetes using Server-Side Apply with `fieldManager: lynq`.

---

## Advanced Template Techniques

::: v-pre

### Local Variables

Define a local variable to avoid repeating complex expressions:

```yaml
nameTemplate: |
  {{- $base := printf "%s-%s" .uid (.region | default "default") -}}
  {{ $base | trunc63 }}-web
```

### Multi-Tier with Shared Variable

```yaml
deployments:
  - id: web
    nameTemplate: |
      {{- $base := printf "%s-%s" .uid (.region | default "default") -}}
      {{ $base | trunc63 }}-web
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: "{{ .webReplicas | default \"2\" | int }}"
        template:
          spec:
            containers:
              - name: web
                image: "{{ .webImage | default \"nginx:stable\" }}"
                env:
                  {{- $apiHost := printf "%s-api-svc.%s.svc.cluster.local" .uid .namespace }}
                  - name: API_ENDPOINT
                    value: "http://{{ $apiHost }}"
  - id: api
    nameTemplate: |
      {{- $base := printf "%s-%s" .uid (.region | default "default") -}}
      {{ $base | trunc63 }}-api
    dependIds: [web]
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: "{{ .apiReplicas | default \"3\" | int }}"
        template:
          spec:
            containers:
              - name: api
                image: "{{ .apiImage | default \"api:latest\" }}"
                env:
                  {{- $dbHost := printf "%s-db.%s.svc.cluster.local" .uid .namespace }}
                  - name: DATABASE_URL
                    value: "postgres://{{ $dbHost }}:5432/{{ .uid }}"
```

### Range Over Lists

```yaml
data:
  endpoints.txt: |
    {{- range $i, $region := list "us-east-1" "us-west-2" "eu-west-1" }}
    {{ $region }}.example.com
    {{- end }}
```

### Complex JSON Parsing

```yaml
# Database column: config = '{"db":{"host":"localhost","port":5432}}'
env:
- name: DB_HOST
  value: "{{ ((.config | fromJson).db).host }}"
- name: DB_PORT
  value: "{{ ((.config | fromJson).db).port }}"
```

:::

## See Also

- [Templates Overview](templates.md) — Quick start examples and best practices
- [Type Conversion](templates-typed-values.md) — `int`, `float`, `bool` for numeric/boolean Kubernetes fields
- [Debugging & Migration](templates-troubleshooting.md) — Common errors, debugging tools, and template evolution
- [API Reference](api.md) — LynqForm CRD schema
