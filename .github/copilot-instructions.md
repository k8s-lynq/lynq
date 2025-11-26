# Lynq Operator - Copilot Instructions

## Project Overview

Lynq Operator is a **Kubernetes operator** that automates database-driven resource provisioning. It reads data from external datasources (MySQL) and dynamically creates Kubernetes resources using declarative templates with Server-Side Apply (SSA).

## Architecture (Three-Controller Design)

```
LynqHub Controller → Reads external DB → Creates LynqNode CRs
LynqForm Controller → Links templates to hubs
LynqNode Controller → Applies K8s resources via SSA
```

### CRDs (`api/v1/`)
| CRD | Type Definition | Sample YAML |
|-----|-----------------|-------------|
| **LynqHub** | `api/v1/lynqhub_types.go` | `config/samples/lynqnodes_v1_lynqhub.yaml` |
| **LynqForm** | `api/v1/lynqform_types.go` | `config/samples/lynqnodes_v1_lynqform.yaml` |
| **LynqNode** | `api/v1/lynqnode_types.go` | `config/samples/lynqnodes_v1_lynqnode.yaml` |

### Core Packages (`internal/`)
| Package | Purpose | Key Files |
|---------|---------|-----------|
| `controller/` | Reconcilers with finalizers | `lynqhub_controller.go`, `lynqform_controller.go`, `lynqnode_controller.go` |
| `apply/` | SSA engine (fieldManager: `lynq-operator`) | `ssa.go` |
| `template/` | Go templates + Sprig + custom funcs | `engine.go` |
| `graph/` | DAG dependency resolution | `dependency.go` |
| `datasource/` | MySQL adapter (pluggable) | `mysql.go`, `interface.go` |
| `readiness/` | Resource health checks | `checker.go` |
| `metrics/` | Prometheus metrics | `metrics.go` |

## Developer Workflow

```bash
make install && make run    # Install CRDs and run locally
make test                   # Unit tests (envtest-based)
make test-e2e               # E2E tests (requires Kind)
make lint && make fmt       # Lint and format
make generate manifests     # After CRD changes
```

**Local dev scripts**: `scripts/setup-minikube.sh`, `scripts/setup-kind.sh`

## Key Conventions

### TResource Structure
📄 **Definition**: `api/v1/common_types.go`

| Policy | Options | Default |
|--------|---------|---------|
| CreationPolicy | `Once`, `WhenNeeded` | `WhenNeeded` |
| DeletionPolicy | `Delete`, `Retain` | `Delete` |
| ConflictPolicy | `Stuck`, `Force` | `Stuck` |
| PatchStrategy | `apply`, `merge`, `replace` | `apply` |

### Template System
📄 **Engine**: `internal/template/engine.go`

**Variables**: `.uid`, `.activate`, `.hubId`, `.templateRef`, + `extraValueMappings`
**Custom functions**: `toHost()`, `trunc63()`, `sha1sum()`, `fromJson()` + all Sprig functions

### Resource Tracking
📄 **Implementation**: `internal/apply/ssa.go`

- **Same-namespace**: `ownerReferences` (auto GC)
- **Cross-namespace/Retain**: Labels `lynq.sh/node`, `lynq.sh/node-namespace`
- **Orphan markers**: Label `lynq.sh/orphaned` + annotations

### Naming Constraints
K8s names max 63 chars → `{{ .uid | trunc63 }}`

## Critical Patterns

### Reconciliation Flow
📄 **Location**: `internal/controller/lynqnode_controller.go`

1. Handle finalizer (`lynqnode.operator.lynq.sh/finalizer`)
2. Build template variables from annotations
3. Topological sort by `dependIds` → `internal/graph/dependency.go`
4. Orphan cleanup (resources removed from template)
5. Apply each resource respecting policies
6. Update status → `internal/status/manager.go`
7. Requeue after 30s

### Watch Configuration
📄 **SetupWithManager**: `internal/controller/lynqnode_controller.go` (line ~930)

- `Owns()`: 11 resource types (Deployments, Services, etc.)
- `Watches()`: Namespaces with label-based predicates

## Testing

| Type | Location | Framework |
|------|----------|-----------|
| Unit tests | `*_test.go` | envtest |
| Behavior tests | `*_behavior_test.go` | envtest |
| E2E tests | `test/e2e/` | Ginkgo + Kind |
| SQL mocks | `internal/datasource/*_test.go` | `go-sqlmock` |

## Documentation & Samples

- **Full docs**: `docs/` (VitePress site at lynq.sh)
- **Helm chart**: `chart/`
- **Sample CRs**: `config/samples/`
- **Prometheus alerts**: `config/prometheus/alerts.yaml`

## ⚠️ Renaming Convention

Project renamed from `tenant-operator` to `lynq`. Convert old names:
- `TenantRegistry` → `LynqHub`, `TenantTemplate` → `LynqForm`, `Tenant` → `LynqNode`
- `kubernetes-tenants.org` → `lynq.sh`
- Image/repo: `k8s-lynq/lynq`
