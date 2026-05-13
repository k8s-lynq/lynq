---
description: "Interactive Form Builder for composing LynqForm resource templates visually."
layout: doc
aside: false
---

# Form Builder

Build LynqForm YAML configurations interactively. Add resources, set dependency ordering, and export a ready-to-apply manifest.

<TemplateBuilder />

## How to Use

### Building from Scratch

1. **Set Hub ID**: Enter your LynqHub name
2. **Add Resources**: Click "+ Add Resource" to create new resources
   - Select resource type (Deployment, Service, ConfigMap, etc.)
   - Set unique ID and name template
   - Configure dependencies
3. **Preview YAML**: Switch to Preview tab to see generated YAML
4. **Export**: Copy or download your form

### Importing Existing YAML

1. **Switch to Editor Tab**: Click "Editor" in the right panel
2. **Paste YAML**: Paste your existing LynqForm
3. **Import**: Click "⬅️ Import to UI" button
4. **Edit**: Modify resources using the form UI
5. **Export**: Generate updated YAML

## Features

### 📝 Form-Based Resource Creation

- Visual interface for all resource types
- Automatic dependency management
- Template variable hints
- Real-time YAML preview

### 🔄 Bidirectional Sync

- **UI → YAML**: Build visually, export YAML
- **YAML → UI**: Import existing forms, edit visually

### 🎯 Template Variables

::: v-pre
Available variables for use in templates:

- `{{ .uid }}` - Node unique identifier
- `{{ .host }}` - Extracted host from URL
- `{{ .hostOrUrl }}` - Original URL/host value
- `{{ .activate }}` - Activation status

Usage example:
```
{{ .uid }}-app
{{ .host }}
```
:::

### ⚙️ Resource Configuration

For each resource, configure:

- **ID**: Unique identifier for dependencies
- **Name Template**: Go template for resource name
- **Dependencies**: Select which resources must exist first
- **Policies**: Wait for ready, creation/deletion policies

## Supported Resource Types

| Type | Description |
|------|-------------|
| **Namespace** | Kubernetes namespace |
| **Deployment** | Pod deployment controller |
| **StatefulSet** | Stateful application controller |
| **DaemonSet** | Node-level daemon controller |
| **Service** | Network service |
| **Ingress** | HTTP/HTTPS routing |
| **ConfigMap** | Configuration data |
| **Secret** | Sensitive data |
| **PersistentVolumeClaim** | Storage volume |
| **Job** | One-time task |
| **CronJob** | Scheduled task |
| **HorizontalPodAutoscaler** | Auto-scaling configuration |
| **Manifest** | Raw YAML for custom resources |

## Tips

### Best Practices

::: v-pre
1. **Use Clear IDs**: Choose descriptive, unique identifiers
2. **Template Names**: Use `{{ .uid }}` prefix for uniqueness
3. **Dependencies**: Only add necessary dependencies
4. **Test First**: Use [Dependency Visualizer](./dependency-visualizer.md) to check for cycles
:::

### Common Patterns

**Secret → Deployment → Service:**
```
1. Add Secret (id: app-secret)
2. Add Deployment (id: app, depends on: app-secret)
3. Add Service (id: app-svc, depends on: app)
```

**Namespace First:**
```
1. Add Namespace (id: node-ns)
2. Add all other resources depending on node-ns
```

## Next Steps

- [Dependencies Guide](./dependencies.md) - Learn about dependency management
- [🔍 Dependency Visualizer](./dependency-visualizer.md) - Visualize your form's dependency graph
- [Templates Guide](./templates.md) - Complete template documentation
- [Quick Start](./quickstart.md) - Deploy your first form

---

::: tip Need Help?
If you encounter issues, check the [Troubleshooting Guide](./troubleshooting.md) or refer to the [API Reference](./api.md).
:::
