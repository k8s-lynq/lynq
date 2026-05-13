---
description: "How to use field-level ignore control to prevent Lynq from overwriting fields managed by other controllers."
---

# Field-Level Ignore Control

## Overview

The `ignoreFields` feature provides fine-grained control over which fields should be excluded from synchronization. This allows you to manage most resource fields declaratively through templates while letting specific fields be controlled externally (e.g., by HPA, manual scaling, or other operators).

<script setup>
import FieldIgnoreAnimation from './.vitepress/components/FieldIgnoreAnimation.vue'
</script>

<FieldIgnoreAnimation />

## Problem Statement

Standard `CreationPolicy` options are too coarse-grained:

- **`Once`**: Creates resource once, never syncs any fields (all-or-nothing)
- **`WhenNeeded`** (default): Continuously syncs all fields

**Real-world scenario**: You want HPA to dynamically control `spec.replicas`, but still want the operator to manage container images, environment variables, and other configuration.

## Solution: Selective Field Ignoring

The `ignoreFields` array accepts **standard JSONPath expressions** to specify which fields should be excluded from synchronization.

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
spec:
  deployments:
    - id: web-app
      creationPolicy: WhenNeeded  # Default - continues syncing
      ignoreFields:
        - "$.spec.replicas"  # Let HPA control this
        - "$.spec.template.spec.containers[0].resources"  # Allow manual tuning
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: "{{ .uid }}-web"
        spec:
          replicas: 3  # Initial value, then ignored
          template:
            spec:
              containers:
                - name: app
                  image: "nginx:{{ .version }}"  # Continues to sync
```

## How It Works

### Initial Creation

When a resource is **first created**:

1. ✅ All fields (including ignored ones) are applied
2. ✅ Resource created with complete specification

### Subsequent Reconciliations

When the resource **already exists**:

1. 🔍 Operator detects resource exists
2. 🗑️ Removes ignored fields from desired state
3. ✅ Applies only non-ignored fields via SSA
4. ✨ **Result**: Ignored fields preserve cluster values, others sync to template

## Example Scenario

### Step 1: Initial Deployment

**Template**:
```yaml
ignoreFields: ["$.spec.replicas"]
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: app
          image: nginx:1.20
```

**Result**: Deployment created with `replicas: 3`, `image: nginx:1.20`

### Step 2: HPA Scales Up

HPA scales deployment to `replicas: 5`

### Step 3: Template Update

**Template updated**:
```yaml
ignoreFields: ["$.spec.replicas"]
spec:
  replicas: 3  # Template still shows 3
  template:
    spec:
      containers:
        - name: app
          image: nginx:1.21  # Image updated
```

### Step 4: Reconciliation

1. Cluster state: `replicas: 5`, `image: nginx:1.20`
2. Desired state: `replicas: 3`, `image: nginx:1.21`
3. Operator removes `replicas` from desired state
4. Operator applies: `image: nginx:1.21` only

**Final Result**:
- ✅ `replicas: 5` (preserved - HPA in control)
- ✅ `image: nginx:1.21` (synced - operator in control)

## Supported JSONPath Expressions

This feature uses the [ojg/jp](https://github.com/ohler55/ojg) library, providing **complete JSONPath standard support**.

### Basic Path Navigation

```yaml
ignoreFields:
  # Simple field
  - "$.spec.replicas"

  # Nested field
  - "$.spec.template.spec.securityContext"

  # Deeply nested field
  - "$.spec.template.spec.containers[0].image"
```

### Array Element Access

```yaml
ignoreFields:
  # First container's image
  - "$.spec.template.spec.containers[0].image"

  # Second container's resources
  - "$.spec.template.spec.containers[1].resources"

  # Init container resources
  - "$.spec.template.spec.initContainers[0].resources"
```

### Wildcard Support

```yaml
ignoreFields:
  # All containers' images
  - "$.spec.template.spec.containers[*].image"

  # All containers' environment variables
  - "$.spec.template.spec.containers[*].env"

  # All init containers' resources
  - "$.spec.template.spec.initContainers[*].resources"
```

### Map Keys with Special Characters

```yaml
ignoreFields:
  # Annotation with dots and slashes
  - "$.metadata.annotations['app.kubernetes.io/version']"

  # Label with special chars
  - "$.metadata.labels['app.kubernetes.io/managed-by']"

  # ConfigMap data key
  - "$.data['application.properties']"
```

## Common Use Cases

### 1. HPA-Controlled Replicas

Let HPA manage scaling while operator manages configuration:

```yaml
deployments:
  - id: api-server
    ignoreFields:
      - "$.spec.replicas"
    spec:
      replicas: 3  # Initial/default value
      # ... rest of spec
```

### 2. Manual Resource Tuning

Allow manual resource adjustments while syncing code changes:

```yaml
deployments:
  - id: backend
    ignoreFields:
      - "$.spec.template.spec.containers[0].resources"
    spec:
      template:
        spec:
          containers:
            - name: app
              image: "backend:{{ .version }}"  # Syncs
              resources:  # Ignored after creation
                limits:
                  memory: "2Gi"
```

### 3. Multiple Containers

Different ignore policies per container:

```yaml
deployments:
  - id: multi-container-app
    ignoreFields:
      # Main app: ignore resources
      - "$.spec.template.spec.containers[0].resources"
      # Sidecar: ignore image (updated separately)
      - "$.spec.template.spec.containers[1].image"
    spec:
      template:
        spec:
          containers:
            - name: app
              image: "app:latest"
            - name: sidecar
              image: "sidecar:stable"
```

### 4. Bulk Ignoring with Wildcards

Ignore same field across all containers:

```yaml
deployments:
  - id: microservice
    ignoreFields:
      # All containers can be manually scaled
      - "$.spec.template.spec.containers[*].resources.limits"
      # All containers' liveness probes can be tuned
      - "$.spec.template.spec.containers[*].livenessProbe"
```

## Validation

### Admission Webhook

JSONPath expressions are validated at admission time:

```bash
# Valid
ignoreFields: ["$.spec.replicas"]  ✅

# Invalid - missing $ prefix
ignoreFields: ["spec.replicas"]  ❌
Error: invalid JSONPath "spec.replicas"

# Invalid - malformed bracket
ignoreFields: ["$[invalid"]  ❌
Error: invalid JSONPath "$[invalid"
```

### Runtime Behavior

- **Non-existent paths**: Silently skipped (no error)
- **Array out of bounds**: Silently skipped (no error)
- **Type mismatches**: Silently skipped (no error)

This graceful handling ensures reconciliation continues even if ignored fields don't exist.

## Interaction with Policies

### CreationPolicy: Once

When combined with `CreationPolicy: Once`, `ignoreFields` has **no effect**:

```yaml
deployments:
  - id: init-job
    creationPolicy: Once
    ignoreFields: ["$.spec.replicas"]  # No effect - resource never reconciled
```

**Reason**: `Once` policy means resource is created once and never touched again, so field-level control is irrelevant.

### ConflictPolicy

Ignored fields **do not participate** in conflict detection:

```yaml
deployments:
  - id: app
    conflictPolicy: Stuck
    ignoreFields: ["$.spec.replicas"]
```

If another controller modifies `replicas`, no conflict is detected because it's ignored.

### DeletionPolicy

`ignoreFields` does **not affect** deletion behavior:

```yaml
deployments:
  - id: app
    deletionPolicy: Retain
    ignoreFields: ["$.spec.replicas"]
```

When LynqNode is deleted, deletion policy is still respected (resources retained).

## Best Practices

### 1. Document Why Fields Are Ignored

```yaml
deployments:
  - id: api-server
    # Replicas controlled by HPA based on CPU/memory
    ignoreFields: ["$.spec.replicas"]
```

### 2. Use Specific Paths Over Wildcards

```yaml
# ✅ Good - explicit and clear
ignoreFields:
  - "$.spec.template.spec.containers[0].resources"

# ⚠️ Use with caution - affects all containers
ignoreFields:
  - "$.spec.template.spec.containers[*].resources"
```

### 3. Test Ignored Fields in Staging

Before production, verify:
1. Initial creation includes ignored fields ✅
2. Manual changes to ignored fields are preserved ✅
3. Template changes to other fields still sync ✅

### 4. Monitor Drift

While ignored fields won't trigger reconciliation, monitor them separately:

```promql
# Alert if replicas drift significantly from template
abs(
  kube_deployment_spec_replicas -
  tenant_template_spec_replicas
) > 10
```

## Troubleshooting

### Issue: Ignored Field Still Being Overwritten

**Check**:
1. Is `creationPolicy` set to `WhenNeeded`? (default)
2. Is JSONPath expression valid?
3. Does path exactly match the field structure?

```bash
# Verify JSONPath
kubectl get lynqnode -o yaml
# Check ignoreFields syntax
```

### Issue: Template Changes Not Applying

**Possible causes**:
1. Field is accidentally in `ignoreFields`
2. Wrong JSONPath (typo in path)

```yaml
# Wrong
ignoreFields: ["$.spec.template.spec.container[0].image"]
                                          # ^^^ missing 's'

# Correct
ignoreFields: ["$.spec.template.spec.containers[0].image"]
```

### Issue: Validation Error on Admission

**Check JSONPath syntax**:

```bash
# Test JSONPath locally
echo '{"spec":{"replicas":3}}' | jq '.spec.replicas'

# Valid JSONPath must start with $
ignoreFields: ["$.spec.replicas"]  ✅
ignoreFields: ["spec.replicas"]    ❌
```

## Performance Considerations

### Minimal Overhead

- **Parsing**: JSONPath expressions parsed once at filter creation
- **Filtering**: O(n) where n = number of ignored fields (typically < 5)
- **Memory**: Negligible (~100 bytes per expression)

### Large-Scale Deployments

For 1000+ nodes with ignored fields:
- **CPU impact**: < 1% overhead
- **Reconciliation time**: < 10ms additional latency

## Migration Guide

### Adding ignoreFields to Existing Resources

**Step 1**: Identify fields to ignore

```bash
# Check what's being managed
kubectl get deployment myapp -o yaml | grep replicas
```

**Step 2**: Add to template

```yaml
deployments:
  - id: myapp
    ignoreFields: ["$.spec.replicas"]
    # ... existing spec
```

**Step 3**: Apply template

```bash
kubectl apply -f lynqnode-template.yaml
```

**Step 4**: Verify

```bash
# Make a manual change
kubectl scale deployment myapp --replicas=10

# Update template (change something else)
# Apply template

# Verify replicas unchanged
kubectl get deployment myapp -o jsonpath='{.spec.replicas}'
# Should still show 10
```

### Removing ignoreFields

**Warning**: Removing `ignoreFields` will cause operator to overwrite cluster values with template values on next reconciliation.

```yaml
# Before
ignoreFields: ["$.spec.replicas"]  # replicas=10 in cluster
spec:
  replicas: 3  # template value

# After removal
ignoreFields: []  # ← Removed

# Next reconciliation: replicas will be reset to 3
```

## Related Features

- [Policies](./policies.md) - Overall resource management policies
- [Dependencies](./dependencies.md) - Resource creation ordering
- [Templates](./templates.md) - Template variable system
