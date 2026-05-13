---
description: "Interactive visualizer for inspecting resource dependency graphs within a LynqForm."
layout: doc
aside: false
---

# Dependency Graph Visualizer

Interactive tool to analyze and visualize LynqForm dependencies. Paste your YAML to see the execution order and detect cycles.

<DependencyGraphVisualizer />

## How to Use

1. **Load an Example**: Click one of the preset buttons above to load a sample LynqForm
2. **Edit YAML**: Modify the YAML in the left editor, or paste your own LynqForm
3. **Analyze**: Click the "🔍 Analyze Dependencies" button to visualize the dependency graph
4. **Explore**:
   - View the execution order with numbered badges
   - Hover over nodes to highlight them in both graph and table
   - Check for dependency cycles (shown in red)
   - See which resources can execute in parallel

## What it Shows

- **Nodes**: Each resource in your form (Deployment, Service, Secret, etc.)
- **Arrows**: Dependencies between resources (A → B means "A depends on B")
- **Numbers**: Execution order (1 = first, 2 = second, etc.)
- **Parallel Execution**: Resources with the same number can run in parallel
- **Cycles**: Invalid circular dependencies shown in red with error message

## Example Scenarios

### Simple App
Basic 3-tier structure: Secret → Deployment → Service

### Multi-Tier Stack
Complex 6-level architecture with namespace, database, application, and ingress

### Parallel Resources
Demonstrates resources that can be created simultaneously

### Cycle Detection
Shows what happens when you have circular dependencies (invalid configuration)

## Tips

- Resources at the same execution level can be applied in parallel
- Use `dependIds` to control the order of resource creation
- Avoid creating dependency cycles - they will cause reconciliation to fail
- Keep dependency chains shallow for better performance

## Related Documentation

- [Dependencies Guide](./dependencies) - Complete guide on defining dependencies
- [Templates Guide](./templates) - Learn about LynqForm structure
- [Policies Guide](./policies) - Resource lifecycle policies

---

::: tip Need Help?
If you encounter issues with your form, check the [Troubleshooting Guide](./troubleshooting) or review the error message shown by the visualizer.
:::
