---
description: "Deep dive into Lynq's three-controller architecture, CRD design, reconciliation flow, SSA engine, and key design patterns for Infrastructure as Data."
---

# Architecture

This document provides a detailed overview of Lynq's architecture as a RecordOps platform, including system components, reconciliation flow, and key design decisions that enable Infrastructure as Data.

::: tip Infrastructure as Data Architecture
Lynq implements Infrastructure as Data through the RecordOps pattern. Database records control infrastructure state, enabling real-time provisioning without YAML files or CI/CD pipelines.

[Learn more about Infrastructure as Data →](./recordops.md)
:::

## System Overview

```mermaid
flowchart TB
    subgraph External["External Data Source"]
        DB[(MySQL / PostgreSQL*)]
    end

    subgraph Cluster["Kubernetes Cluster"]
        direction TB

        subgraph Controllers["Operator Controllers"]
            RC[LynqHub Controller]
            TC[LynqForm Controller]
            TNC[LynqNode Controller]
        end

        subgraph CRDs["Custom Resources"]
            TR[LynqHub]
            TT[LynqForm]
            T[LynqNode CRs]
        end

        subgraph Engine["Apply Engine"]
            SSA["SSA Apply Engine<br/>(fieldManager: lynq)"]
        end

        subgraph Resources["Kubernetes Resources"]
            DEP[Deployments]
            SVC[Services]
            ING[Ingresses]
            etc[ConfigMaps, Secrets, ...]
        end

        API[(etcd / K8s API Server)]
    end

    DB -->|"syncInterval<br/>(e.g., 1m)"| RC
    RC -->|"Creates/Updates/Deletes<br/>LynqNode CRs"| API
    API -->|"Stores"| TR
    API -->|"Stores"| TT
    API -->|"Stores"| T

    TC -->|"Validates<br/>template-registry linkage"| API
    TNC -->|"Reconciles<br/>each LynqNode"| SSA
    SSA -->|"Server-Side Apply"| API

    API -->|"Creates"| DEP
    API -->|"Creates"| SVC
    API -->|"Creates"| ING
    API -->|"Creates"| etc

    style RC fill:#e3f2fd,stroke:#64b5f6,stroke-width:2px
    style TC fill:#e8f5e9,stroke:#81c784,stroke-width:2px
    style TNC fill:#fff3e0,stroke:#ffb74d,stroke-width:2px
    style SSA fill:#fce4ec,stroke:#f06292,stroke-width:2px
    style DB fill:#f3e5f5,stroke:#ba68c8,stroke-width:2px
```

::: info Database Support

- **MySQL**: Fully supported (v1.0+)
- **PostgreSQL**: Planned for v1.2
  :::

## Architecture at a Glance

Quick reference for the three main components:

| Component          | Purpose                                 | Example                               |
| ------------------ | --------------------------------------- | ------------------------------------- |
| **LynqHub** | Connects to database, syncs node rows | MySQL every 30s → Creates LynqNode CRs  |
| **LynqForm** | Defines resource blueprint              | Deployment + Service per node       |
| **LynqNode**       | Instance of a single node               | `acme-corp-web-app` → 5 K8s resources |

**Infrastructure as Data Workflow**: Database row becomes active → LynqHub controller syncs and multiplies rows by all referencing LynqForms → A LynqNode CR is created per `{row × template}` combination → LynqNode controller renders the LynqForm snapshot for that node and applies it via the SSA engine → Kubernetes resources are created/updated and kept in sync. Your data defines infrastructure, Lynq provisions it.

## Reconciliation Flow

```mermaid
sequenceDiagram
    participant DB as MySQL Database
    participant RC as LynqHub Controller
    participant API as K8s API Server
    participant TC as LynqForm Controller
    participant TNC as LynqNode Controller
    participant SSA as SSA Engine

    Note over DB,SSA: Hub Sync Cycle (e.g., every 1 minute)

    RC->>DB: SELECT * FROM nodes WHERE activate=TRUE
    DB-->>RC: Active node rows

    RC->>API: Create/Update LynqNode CRs (desired set)
    RC->>API: Delete LynqNodes not in desired set

    TC->>API: Validate Template-Registry linkage
    TC->>API: Ensure consistency

    loop For Each LynqNode
        TNC->>API: Get LynqNode spec
        TNC->>TNC: Build dependency graph (dependIds)
        TNC->>TNC: Topological sort resources

        loop For Each Resource (in order)
            TNC->>TNC: Render templates (name, namespace, spec)
            TNC->>SSA: Apply resource with conflict policy
            SSA->>API: Server-Side Apply (force or not)

            alt waitForReady = true
                TNC->>API: Wait for resource Ready condition
                API-->>TNC: Ready (or timeout)
            end
        end

        TNC->>API: Update LynqNode status (ready/failed counts)
    end

    RC->>API: Update Hub status (desired/ready/failed)
```

## Three-Controller Design

The operator uses a three-controller architecture to separate concerns and optimize reconciliation:

### 1. LynqHub Controller

**Purpose**: Syncs database (e.g., 1m interval) → Creates/Updates/Deletes LynqNode CRs

**Responsibilities**:

- Periodically queries external datasource at `spec.source.syncInterval`
- Filters active rows where `activate` field is truthy
- Calculates desired LynqNode set: `referencingTemplates × activeRows`
- Creates missing LynqNode CRs (naming: `{uid}-{template-name}`)
- Updates existing LynqNode CRs with fresh data
- Deletes LynqNode CRs for inactive/removed rows (garbage collection)
- Updates Hub status with counts

**Key Status Fields**:

```yaml
status:
  referencingTemplates: 2 # Number of templates using this hub
  desired: 6 # referencingTemplates × activeRows
  ready: 5 # Ready LynqNodes across all templates
  failed: 1 # Failed LynqNodes across all templates
```

### 2. LynqForm Controller

**Purpose**: Validates template-registry linkage and invariants

**Responsibilities**:

- Validates that `spec.hubId` references an existing LynqHub
- Ensures template syntax is valid (Go text/template)
- Validates resource IDs are unique within template
- Detects dependency cycles in `dependIds`
- Updates template status

### 3. LynqNode Controller

**Purpose**: Renders templates → Resolves dependencies → Applies resources via SSA

**Responsibilities**:

- Builds template variables from LynqNode spec
- Resolves resource dependencies (DAG + topological sort)
- Renders all templates (names, namespaces, specs)
- Applies resources using Server-Side Apply
- Waits for resource readiness (if `waitForReady=true`)
- Updates LynqNode status with resource counts and conditions
- Handles conflicts according to ConflictPolicy
- Manages finalizers for proper cleanup

## CRD Architecture

### LynqHub

Defines external datasource configuration and sync behavior:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-saas-hub
spec:
  source:
    type: mysql
    mysql:
      host: mysql.default.svc.cluster.local
      port: 3306
      database: nodes
      table: node_data
      username: node_reader
      passwordRef:
        name: mysql-secret
        key: password
    syncInterval: 30s
  valueMappings:
    uid: node_id # Required
    hostOrUrl: domain # Required
    activate: is_active # Required
  extraValueMappings:
    planId: subscription_plan
    deployImage: container_image
```

**Multi-Form Support**: One hub can be referenced by multiple LynqForms, creating separate LynqNode CRs for each form-row combination.

**Concrete Naming Example: Hub + 2 Forms + 3 Rows**

Given:
- **Hub**: `customer-hub` with 3 active database rows: `acme`, `beta`, `corp`
- **Forms**: `web-app` and `worker` referencing the hub

**Result**: 6 LynqNode CRs created (3 rows × 2 templates):

| Database UID | Template | LynqNode Name |
|--------------|----------|---------------|
| `acme` | `web-app` | `acme-web-app` |
| `acme` | `worker` | `acme-worker` |
| `beta` | `web-app` | `beta-web-app` |
| `beta` | `worker` | `beta-worker` |
| `corp` | `web-app` | `corp-web-app` |
| `corp` | `worker` | `corp-worker` |

**Hub Status:**
```yaml
status:
  referencingTemplates: 2    # web-app + worker
  desired: 6                 # 3 rows × 2 templates = 6 nodes
  ready: 6
  failed: 0
```

**Naming Convention**: `{uid}-{template-name}`

```bash
# List all LynqNodes for this hub
$ kubectl get lynqnodes -l lynq.sh/hub=customer-hub
NAME              READY   DESIRED   FAILED   AGE
acme-web-app      5/5     5         0        10m
acme-worker       3/3     3         0        10m
beta-web-app      5/5     5         0        10m
beta-worker       3/3     3         0        10m
corp-web-app      5/5     5         0        10m
corp-worker       3/3     3         0        10m
```

**When a Template is Removed:**

If `worker` form is deleted:
- LynqNodes `acme-worker`, `beta-worker`, `corp-worker` are deleted
- Hub status updates: `desired: 3, referencingTemplates: 1`

**When a Database Row is Deactivated:**

If `beta` row's `activate` becomes false:
- LynqNodes `beta-web-app`, `beta-worker` are deleted
- Hub status updates: `desired: 4` (2 rows × 2 templates)

### LynqForm

Blueprint for resources to create per node:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: web-app
spec:
  hubId: my-saas-hub
  deployments:
    - id: app-deployment
      nameTemplate: "{{ .uid }}-app"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          # ... deployment spec
```

**Supported Resource Types**:

- `serviceAccounts`
- `deployments`, `statefulSets`, `daemonSets`
- `services`
- `configMaps`, `secrets`
- `persistentVolumeClaims`
- `jobs`, `cronJobs`
- `ingresses`
- `namespaces`
- `manifests` (raw resources)

### LynqNode

Instance representing a single node:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqNode
metadata:
  name: acme-web-app
spec:
  uid: acme
  templateRef: web-app
  hubId: my-saas-hub
  # ... resolved resource arrays
status:
  desiredResources: 10
  readyResources: 10
  failedResources: 0
  appliedResources:
    - "Deployment/default/acme-app@app-deployment"
    - "Service/default/acme-svc@app-service"
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2024-01-15T10:30:00Z"
```

## Key Design Patterns

### Server-Side Apply (SSA)

All resources are applied using Kubernetes Server-Side Apply with `fieldManager: lynq`. This provides:

- **Conflict-free updates**: Multiple controllers can manage different fields
- **Declarative management**: Operator owns only fields it sets
- **Drift detection**: Automatic detection of manual changes
- **Force mode**: Optional force ownership with `ConflictPolicy: Force`

### Resource Tracking

Two mechanisms based on namespace and deletion policy:

1. **OwnerReference-based** (automatic GC):

   - Same-namespace resources with `DeletionPolicy=Delete`
   - Kubernetes garbage collector handles cleanup

2. **Label-based** (manual lifecycle):
   - Cross-namespace resources
   - Namespace resources
   - Resources with `DeletionPolicy=Retain`
   - Labels: `lynq.sh/node`, `lynq.sh/node-namespace`

### Dependency Management

Resources are applied in order based on `dependIds`:

```yaml
deployments:
  - id: app-deployment
    # ...

services:
  - id: app-service
    dependIds: ["app-deployment"]
    waitForReady: true
    # ...
```

The operator:

1. Builds a Directed Acyclic Graph (DAG)
2. Detects cycles (fails fast if found)
3. Performs topological sort
4. Applies resources in dependency order

### Drift Detection & Auto-Correction

The operator continuously monitors managed resources through:

- **Event-driven watches**: Immediate reconciliation on resource changes
- **Watch predicates**: Only trigger on meaningful changes (Generation/Annotation)
- **Fast requeue**: 30-second periodic requeue for status reflection
- **Auto-correction**: Reverts manual changes to maintain desired state

### Garbage Collection

Automatic cleanup when:

- Database row is deleted
- Row's `activate` field becomes false
- Template no longer references the hub
- LynqNode CR is deleted (with finalizer-based cleanup)

Resources respect `DeletionPolicy`:

- `Delete`: Removed from cluster
- `Retain`: Orphaned with labels for manual cleanup

### Orphan Resource Management

When resources are removed from templates:

- Detected via comparison of `status.appliedResources`
- Resource key format: `kind/namespace/name@id`
- DeletionPolicy preserved in annotation: `lynq.sh/deletion-policy`
- Orphaned resources marked with:
  - Label: `lynq.sh/orphaned: "true"`
  - Annotations: `orphaned-at`, `orphaned-reason`
- Re-adoption: Orphan markers removed when resource re-added to template

**Orphan Detection → Cleanup Sequence:**

```mermaid
sequenceDiagram
    participant TNC as LynqNode Controller
    participant API as K8s API Server
    participant RES as Kubernetes Resource

    Note over TNC,RES: Phase 1: Orphan Detection

    TNC->>API: Get LynqNode status.appliedResources
    API-->>TNC: ["Deployment/ns/app@deploy", "Service/ns/svc@service", "ConfigMap/ns/old@config"]

    TNC->>TNC: Calculate current desired resources
    Note right of TNC: ["Deployment/ns/app@deploy", "Service/ns/svc@service"]<br/>ConfigMap removed from template

    TNC->>TNC: Detect orphans: appliedResources - desiredResources
    Note right of TNC: Orphan: "ConfigMap/ns/old@config"

    Note over TNC,RES: Phase 2: Orphan Cleanup

    TNC->>API: Get ConfigMap (read lynq.sh/deletion-policy annotation)
    API-->>TNC: ConfigMap with deletion-policy: "Retain"

    alt DeletionPolicy = Delete
        TNC->>API: Delete ConfigMap
        API-->>TNC: Deleted
    else DeletionPolicy = Retain
        TNC->>API: Patch ConfigMap: add orphan labels/annotations
        Note right of API: + lynq.sh/orphaned: "true"<br/>+ lynq.sh/orphaned-at: "2024-01-15T10:30:00Z"<br/>+ lynq.sh/orphaned-reason: "RemovedFromTemplate"
        API-->>TNC: Updated
    end

    Note over TNC,RES: Phase 3: Re-adoption (if resource added back)

    TNC->>TNC: ConfigMap re-added to template
    TNC->>API: Apply ConfigMap with SSA
    Note right of API: - lynq.sh/orphaned: removed<br/>- lynq.sh/orphaned-at: removed<br/>- lynq.sh/orphaned-reason: removed<br/>+ ownerReference: restored (if same namespace)
    API-->>TNC: Applied

    TNC->>API: Update LynqNode status.appliedResources
    Note right of API: ["Deployment/ns/app@deploy", "Service/ns/svc@service", "ConfigMap/ns/old@config"]
```

**Query Orphaned Resources:**

```bash
# Find all orphaned resources
kubectl get all -A -l lynq.sh/orphaned=true

# Find orphans from a specific node
kubectl get all -A -l lynq.sh/orphaned=true,lynq.sh/node=acme-web-app

# Find orphans by reason
kubectl get all -A -o json | jq '.items[] | select(.metadata.annotations["lynq.sh/orphaned-reason"] == "RemovedFromTemplate") | "\(.kind)/\(.metadata.namespace)/\(.metadata.name)"'

# Count orphans per namespace
kubectl get all -A -l lynq.sh/orphaned=true -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' | sort | uniq -c
```

## Performance Considerations

### Controller Concurrency

Configurable worker pools for each controller:

- `--hub-concurrency=N` (default: 3)
- `--form-concurrency=N` (default: 5)
- `--node-concurrency=N` (default: 10)

### Reconciliation Optimization

- **Fast status reflection**: 30-second requeue interval
- **Smart watch predicates**: Filter status-only updates
- **Event-driven architecture**: Immediate reaction to changes
- **Resource caching**: Frequently accessed resources cached

### Scalability

The operator is designed to scale horizontally:

- Leader election for single-writer pattern
- Optional sharding by namespace or node ID
- Resource-type worker pools for parallel processing
- SSA batching for bulk applies

## Infrastructure as Data in Practice

Lynq's architecture implements Infrastructure as Data through RecordOps:

1. **Database as Infrastructure API**: Your database schema defines your infrastructure specifications
2. **Continuous Reconciliation**: Controllers ensure infrastructure state matches database state
3. **Template-Based Provisioning**: One template definition applies to all records
4. **Automatic Lifecycle**: Database operations (INSERT/UPDATE/DELETE) trigger infrastructure changes

This architecture enables Infrastructure as Data where operational changes are simply database transactions.

## See Also

- [Infrastructure as Data](/recordops) - Understanding the paradigm
- [API Reference](/api) - Complete CRD specification
- [Policies](/policies) - Lifecycle management policies
- [Dependencies](/dependencies) - Dependency graph system
- [Monitoring](/monitoring) - Observability and metrics
