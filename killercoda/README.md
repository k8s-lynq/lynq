# Killercoda Scenarios for Lynq

This directory contains interactive tutorial scenarios for [Killercoda](https://killercoda.com/).

## Available Scenarios

### [lynq-quickstart](./lynq-quickstart/)

**Database-Driven Kubernetes Automation in 10 Minutes**

A hands-on tutorial that walks users through:
1. Installing Lynq Operator with cert-manager
2. Setting up a MySQL database with sample tenant data
3. Creating a LynqHub to sync database rows
4. Defining a LynqForm template
5. Testing the full tenant lifecycle (add, deactivate, reactivate, update)

**Duration**: ~10 minutes
**Environment**: Kubernetes (kubeadm 1-node)

## Publishing to Killercoda

### Prerequisites

1. Create an account on [Killercoda](https://killercoda.com/)
2. Fork/clone this repository

### Setup

1. Go to [Killercoda Creator](https://killercoda.com/creator/)
2. Connect your GitHub repository
3. Select the `killercoda` directory as the scenarios root
4. Publish the scenarios

### Scenario Structure

Each scenario follows this structure:

```
scenario-name/
├── index.json      # Scenario configuration
├── intro.md        # Introduction page
├── step1.md        # Step 1
├── step2.md        # Step 2
├── ...
└── finish.md       # Completion page
```

### `index.json` Configuration

```json
{
  "title": "Scenario Title",
  "description": "Short description",
  "details": {
    "intro": { "text": "intro.md" },
    "steps": [
      { "title": "Step Title", "text": "step1.md" }
    ],
    "finish": { "text": "finish.md" }
  },
  "backend": {
    "imageid": "kubernetes-kubeadm-1node"
  }
}
```

### Available Backend Images

- `kubernetes-kubeadm-1node` - Single node Kubernetes cluster
- `kubernetes-kubeadm-2nodes` - Two node Kubernetes cluster
- `ubuntu` - Plain Ubuntu environment

## Contributing

When creating new scenarios:

1. Create a new directory under `killercoda/`
2. Add `index.json` with scenario metadata
3. Add markdown files for each step
4. Use `{{exec}}` suffix for executable code blocks
5. Test locally using Killercoda's preview feature

### Markdown Syntax for Executable Commands

```markdown
```bash
kubectl get pods
```{{exec}}
```

This renders as a clickable command that users can execute.

## Links

- [Killercoda Documentation](https://killercoda.com/docs)
- [Lynq Documentation](https://lynq.sh/)
- [Lynq GitHub Repository](https://github.com/k8s-lynq/lynq)
