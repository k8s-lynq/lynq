---
url: 'https://lynq.sh/policies-examples.md'
description: >-
  Practical examples of combining CreationPolicy, DeletionPolicy,
  ConflictPolicy, and PatchStrategy in real-world LynqForms.
---

# Policy Combinations - Practical Examples

Real-world examples showing how to combine CreationPolicy, DeletionPolicy, ConflictPolicy, and PatchStrategy for different resource types.

## Overview

This guide demonstrates how policies work together through four common scenarios:

| Example | Use Case | Key Pattern | Policies |
|---------|----------|-------------|----------|
| [Example 1](#example-1-stateful-data-pvc) | Persistent Data | Immutable + Retained | `Once + Retain + Stuck` |
| [Example 2](#example-2-init-job) | One-time Setup | Run Once + Cleanup | `Once + Delete + Force` |
| [Example 3](#example-3-application-deployment) | Main Application | Sync + Cleanup | `WhenNeeded + Delete + Stuck` |
| [Example 4](#example-4-shared-infrastructure) | Shared Config | Sync + Retained | `WhenNeeded + Retain + Force` |

## Example 1: Stateful Data (PVC)

**Use Case:** Persistent storage that must survive node lifecycle changes and never lose data.

### Configuration

```yaml
persistentVolumeClaims:
  - id: data
    creationPolicy: Once        # Create only once
    deletionPolicy: Retain      # Keep data after node deletion
    conflictPolicy: Stuck       # Don't overwrite existing PVCs
    patchStrategy: apply        # Standard SSA
    nameTemplate: "{{ .uid }}-data"
    spec:
      apiVersion: v1
      kind: PersistentVolumeClaim
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### Lifecycle Flow

```mermaid
flowchart TD
    Start([Node Created])
    CheckExists{PVC Exists?}
    HasAnnotation{Has created-once<br/>annotation?}
    CreatePVC[Create PVC<br/>+ Add annotation<br/>+ Label tracking only<br/>NO ownerReference]
    SkipCreate[Skip Creation<br/>Count as Ready]
    TemplateChange[Template Updated]
    SkipUpdate[Skip Update<br/>CreationPolicy=Once]
    NodeDelete[Node Deleted]
    RemoveLabels[Remove Tracking Labels<br/>Add Orphan Labels]
    PVCRetained[PVC Retained in Cluster<br/>Data Preserved]
    ConflictDetect{PVC owned by<br/>another controller?}
    MarkDegraded[Mark Node Degraded<br/>Emit ResourceConflict Event]

    Start --> CheckExists
    CheckExists -->|No| ConflictDetect
    CheckExists -->|Yes| HasAnnotation

    ConflictDetect -->|No| CreatePVC
    ConflictDetect -->|Yes| MarkDegraded

    HasAnnotation -->|Yes| SkipCreate
    HasAnnotation -->|No| CreatePVC

    CreatePVC --> TemplateChange
    SkipCreate --> TemplateChange
    TemplateChange --> SkipUpdate

    SkipUpdate --> NodeDelete
    NodeDelete --> RemoveLabels
    RemoveLabels --> PVCRetained

    classDef createStyle fill:#e8f5e9,stroke:#4caf50,stroke-width:2px;
    classDef skipStyle fill:#fff3e0,stroke:#ff9800,stroke-width:2px;
    classDef retainStyle fill:#e3f2fd,stroke:#2196f3,stroke-width:2px;
    classDef errorStyle fill:#ffebee,stroke:#f44336,stroke-width:2px;

    class CreatePVC createStyle;
    class SkipCreate,SkipUpdate skipStyle;
    class PVCRetained,RemoveLabels retainStyle;
    class MarkDegraded errorStyle;
```

### Rationale

* **`Once`**: PVC spec shouldn't change (size immutable in many storage classes)
* **`Retain`**: Data survives node deletion - **NO ownerReference** set to prevent automatic deletion
* **`Stuck`**: Safety - don't overwrite someone else's PVC on initial creation
* **`apply`**: Standard SSA for declarative management

### Key Behaviors

* ✅ Created once, never updated (even if template changes)
* ✅ Survives node deletion (label-based tracking)
* ✅ Safe conflict detection on initial creation
* 📊 Data persists indefinitely
* ⚠️ **Important**: Once `created-once` annotation is set, `ApplyResource` is never called again

### Delete and Recreate Scenario

::: warning CreationPolicy=Once Limitation
With `CreationPolicy: Once`, the operator **SKIPS** resources that have the `created-once` annotation. This means on Node recreation:

* **NO re-adoption** occurs
* **Orphan markers remain** on the resource
* **NO conflict detection** (ApplyResource is not called)
* Resource is **counted as Ready** but not actively managed
  :::

**Scenario Timeline:**

```mermaid
sequenceDiagram
    participant User
    participant Node as LynqNode CR
    participant Operator
    participant PVC as PVC (acme-data)

    Note over Node,PVC: Phase 1: Initial State
    User->>Node: Create Node (uid: acme)
    Node->>Operator: Reconcile
    Operator->>PVC: Create PVC<br/>+ created-once: true<br/>+ NO ownerReference
    Note over PVC: Active & Managed Once

    Note over Node,PVC: Phase 2: Deletion
    User->>Node: Delete Node
    Node->>Operator: Finalizer runs
    Operator->>PVC: Add orphan labels<br/>(created-once: true REMAINS)
    Note over PVC: Orphaned but exists<br/>orphaned: true<br/>created-once: true

    Note over Node,PVC: Phase 3: Recreation (Same UID)
    User->>Node: Create Node (uid: acme)
    Node->>Operator: Reconcile
    Operator->>PVC: Check exists & has created-once?
    PVC-->>Operator: Yes, has created-once
    Operator->>Operator: SKIP (continue)<br/>Count as Ready<br/>NO ApplyResource call
    Note over PVC: STILL ORPHANED<br/>orphaned: true remains<br/>created-once: true remains
```

**Manual Recovery:**

```bash
# Remove the created-once annotation to allow re-adoption
kubectl annotate pvc acme-data lynq.sh/created-once-

# Next reconciliation will call ApplyResource and remove orphan markers
```

***

## Example 2: Init Job

**Use Case:** One-time initialization task that runs once per node and cleans up after node deletion.

### Configuration

```yaml
jobs:
  - id: init
    creationPolicy: Once        # Run only once
    deletionPolicy: Delete      # Clean up after node deletion
    conflictPolicy: Force       # Re-create if needed
    patchStrategy: replace      # Exact job spec
    nameTemplate: "{{ .uid }}-init"
    spec:
      apiVersion: batch/v1
      kind: Job
      spec:
        template:
          spec:
            containers:
            - name: init
              image: busybox
              command: ["sh", "-c", "echo Initializing {{ .uid }}"]
            restartPolicy: Never
```

### Lifecycle Flow

```mermaid
flowchart TD
    Start([Node Created])
    CheckExists{Job Exists?}
    HasAnnotation{Has created-once<br/>annotation?}
    CheckConflict{Job owned by<br/>another controller?}
    ForceApply[Force Take Ownership<br/>SSA with force=true]
    CreateJob[Create Job<br/>+ Add annotation<br/>+ ownerReference]
    SkipCreate[Skip Creation<br/>Job Already Completed]
    RunJob[Job Executes Once]
    TemplateChange[Template Updated]
    SkipUpdate[Skip Update<br/>CreationPolicy=Once<br/>Job keeps running]
    ManualDelete[User Manually<br/>Deletes Job]
    RecreateJob[Recreate Job<br/>on Next Reconcile]
    NodeDelete[Node Deleted]
    AutoDelete[Kubernetes GC<br/>Deletes Job<br/>via ownerReference]
    Cleanup[Cleanup Complete]

    Start --> CheckExists
    CheckExists -->|No| CreateJob
    CheckExists -->|Yes| HasAnnotation

    HasAnnotation -->|Yes| SkipCreate
    HasAnnotation -->|No| CheckConflict

    CheckConflict -->|Yes| ForceApply
    CheckConflict -->|No| CreateJob

    CreateJob --> RunJob
    ForceApply --> RunJob
    SkipCreate --> TemplateChange

    RunJob --> TemplateChange
    TemplateChange --> SkipUpdate
    SkipUpdate --> ManualDelete
    ManualDelete --> RecreateJob
    RecreateJob --> NodeDelete

    SkipUpdate --> NodeDelete
    NodeDelete --> AutoDelete
    AutoDelete --> Cleanup

    classDef createStyle fill:#e8f5e9,stroke:#4caf50,stroke-width:2px;
    classDef skipStyle fill:#fff3e0,stroke:#ff9800,stroke-width:2px;
    classDef deleteStyle fill:#ffebee,stroke:#f44336,stroke-width:2px;
    classDef forceStyle fill:#fce4ec,stroke:#e91e63,stroke-width:2px;

    class CreateJob,RecreateJob createStyle;
    class SkipCreate,SkipUpdate skipStyle;
    class AutoDelete,Cleanup deleteStyle;
    class ForceApply forceStyle;
```

### Rationale

* **`Once`**: Initialization runs only once - even if template changes, job won't re-run
* **`Delete`**: No need to keep job history after node deletion
* **`Force`**: Operator owns this resource exclusively - takes ownership if conflict
* **`replace`**: Ensures exact job spec match

### Key Behaviors

* ✅ Runs once per node lifetime
* ✅ Automatically cleaned up on node deletion
* ✅ Force takes ownership from conflicts
* 🔄 Re-creates if manually deleted (but still runs only once due to created-once annotation)

***

## Example 3: Application Deployment

**Use Case:** Main application that should stay synchronized with template changes and clean up completely on deletion.

### Configuration

```yaml
deployments:
  - id: app
    creationPolicy: WhenNeeded  # Keep updated
    deletionPolicy: Delete      # Clean up on deletion
    conflictPolicy: Stuck       # Safe default
    patchStrategy: apply        # Kubernetes best practice
    nameTemplate: "{{ .uid }}-app"
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: 2
        selector:
          matchLabels:
            app: "{{ .uid }}"
        template:
          metadata:
            labels:
              app: "{{ .uid }}"
          spec:
            containers:
            - name: app
              image: "nginx:latest"
```

### Lifecycle Flow

```mermaid
flowchart TD
    Start([Node Created])
    CheckExists{Deployment<br/>Exists?}
    CheckConflict{Owned by another<br/>controller?}
    MarkDegraded[Mark Node Degraded<br/>Stop Reconciliation<br/>Emit ResourceConflict]
    CreateDeploy[Create Deployment<br/>SSA with fieldManager<br/>+ ownerReference]
    DeployRunning[Deployment Running]
    TemplateChange[Template Updated<br/>DB Data Changed]
    ApplyUpdate[Apply Changes<br/>SSA updates only<br/>managed fields]
    DriftDetect[Manual Change<br/>Detected]
    AutoCorrect[Auto-Correct Drift<br/>Revert to desired state]
    NodeDelete[Node Deleted]
    AutoDelete[Kubernetes GC<br/>Deletes Deployment<br/>+ Pods + ReplicaSets]
    Cleanup[Complete Cleanup]

    Start --> CheckExists
    CheckExists -->|No| CreateDeploy
    CheckExists -->|Yes| CheckConflict

    CheckConflict -->|Yes| MarkDegraded
    CheckConflict -->|No| DeployRunning

    CreateDeploy --> DeployRunning
    DeployRunning --> TemplateChange
    TemplateChange --> ApplyUpdate
    ApplyUpdate --> DeployRunning

    DeployRunning --> DriftDetect
    DriftDetect --> AutoCorrect
    AutoCorrect --> DeployRunning

    DeployRunning --> NodeDelete
    NodeDelete --> AutoDelete
    AutoDelete --> Cleanup

    classDef createStyle fill:#e8f5e9,stroke:#4caf50,stroke-width:2px;
    classDef updateStyle fill:#e3f2fd,stroke:#2196f3,stroke-width:2px;
    classDef deleteStyle fill:#ffebee,stroke:#f44336,stroke-width:2px;
    classDef errorStyle fill:#fce4ec,stroke:#e91e63,stroke-width:2px;

    class CreateDeploy createStyle;
    class ApplyUpdate,AutoCorrect updateStyle;
    class AutoDelete,Cleanup deleteStyle;
    class MarkDegraded errorStyle;
```

### Rationale

* **`WhenNeeded`**: Always keep deployment in sync with template and database
* **`Delete`**: Standard cleanup via ownerReference
* **`Stuck`**: Safe default - investigate conflicts rather than force override
* **`apply`**: SSA best practice - preserves fields from other controllers (e.g., HPA)

### Key Behaviors

* ✅ Continuously synchronized with template
* ✅ Auto-corrects manual drift
* ✅ Plays well with other controllers (HPA, VPA)
* ✅ Complete cleanup on deletion
* ⚠️ Stops on conflicts for safety

***

## Example 4: Shared Infrastructure

**Use Case:** Configuration data that should stay updated but survive node deletion for debugging or shared resource references.

### Configuration

```yaml
configMaps:
  - id: shared-config
    creationPolicy: WhenNeeded  # Maintain config
    deletionPolicy: Retain      # Keep config for investigation
    conflictPolicy: Force       # Operator manages configs
    patchStrategy: apply        # SSA
    nameTemplate: "{{ .uid }}-shared-config"
    spec:
      apiVersion: v1
      kind: ConfigMap
      data:
        config.json: |
          {
            "tenantId": "{{ .uid }}",
            "environment": "production",
            "version": "1.0"
          }
```

### Lifecycle Flow

```mermaid
flowchart TD
    Start([Node Created])
    CheckExists{ConfigMap<br/>Exists?}
    CheckConflict{Owned by another<br/>controller?}
    ForceTake[Force Take Ownership<br/>SSA with force=true<br/>+ Label tracking only<br/>NO ownerReference]
    CreateCM[Create ConfigMap<br/>SSA apply<br/>+ Label tracking only<br/>NO ownerReference]
    CMActive[ConfigMap Active]
    TemplateChange[Template Updated<br/>DB Data Changed]
    ApplyUpdate[Apply Changes<br/>SSA updates config data<br/>Force if conflict]
    OtherPodRef[Other Pods/Services<br/>Reference ConfigMap]
    NodeDelete[Node Deleted]
    RemoveLabels[Remove Tracking Labels<br/>Add Orphan Labels<br/>+ Timestamp + Reason]
    CMRetained[ConfigMap Retained<br/>Available for Investigation<br/>or Shared Use]

    Start --> CheckExists
    CheckExists -->|No| CreateCM
    CheckExists -->|Yes| CheckConflict

    CheckConflict -->|Yes| ForceTake
    CheckConflict -->|No| CMActive

    CreateCM --> CMActive
    ForceTake --> CMActive

    CMActive --> TemplateChange
    TemplateChange --> ApplyUpdate
    ApplyUpdate --> CMActive

    CMActive --> OtherPodRef
    OtherPodRef --> CMActive

    CMActive --> NodeDelete
    NodeDelete --> RemoveLabels
    RemoveLabels --> CMRetained

    classDef createStyle fill:#e8f5e9,stroke:#4caf50,stroke-width:2px;
    classDef updateStyle fill:#e3f2fd,stroke:#2196f3,stroke-width:2px;
    classDef retainStyle fill:#fff3e0,stroke:#ff9800,stroke-width:2px;
    classDef forceStyle fill:#fce4ec,stroke:#e91e63,stroke-width:2px;

    class CreateCM createStyle;
    class ApplyUpdate updateStyle;
    class RemoveLabels,CMRetained retainStyle;
    class ForceTake forceStyle;
```

### Rationale

* **`WhenNeeded`**: Keep configmap data updated as template/database changes
* **`Retain`**: ConfigMap might be referenced by other resources or needed for debugging - **NO ownerReference** to prevent deletion
* **`Force`**: Operator is authoritative for this config - takes ownership if conflict exists
* **`apply`**: SSA for declarative configuration management

### Key Behaviors

* ✅ Continuously synchronized with changes
* ✅ Force takes ownership from conflicts
* ✅ Survives node deletion (label-based tracking)
* 📊 Available for investigation post-deletion
* 🔗 Can be referenced by non-node resources

### Delete and Recreate with WhenNeeded

Unlike Example 1 (PVC with `Once`), resources with `WhenNeeded` automatically re-adopt on recreation:

```mermaid
sequenceDiagram
    participant User
    participant Node as LynqNode CR
    participant Operator
    participant ConfigMap as ConfigMap<br/>(acme-shared-config)

    Note over Node,ConfigMap: Phase 1: Active Updates
    User->>Node: Create Node (uid: acme)
    Node->>Operator: Reconcile
    Operator->>ConfigMap: Create ConfigMap<br/>Labels: lynqnode=acme-web<br/>NO ownerReference
    Note over ConfigMap: Active & Managed<br/>Syncs with template

    User->>Node: Update Template<br/>(change config data)
    Node->>Operator: Reconcile
    Operator->>ConfigMap: Apply Updates<br/>Force if conflict
    Note over ConfigMap: Updated with new data

    Note over Node,ConfigMap: Phase 2: Deletion & Retention
    User->>Node: Delete Node
    Node->>Operator: Finalizer runs
    Operator->>ConfigMap: Remove tracking labels<br/>Add orphan labels
    Note over ConfigMap: Orphaned but exists<br/>Last data preserved

    Note over Node,ConfigMap: Phase 3: Re-adoption & Resume Updates
    User->>Node: Create Node (uid: acme)
    Node->>Operator: Reconcile
    Operator->>ConfigMap: Check exists & orphan?
    ConfigMap-->>Operator: Yes, found orphan
    Operator->>ConfigMap: Re-adopt + Apply Updates
    Note over ConfigMap: Active & Managed again<br/>Updates resume

    User->>Node: Update Template<br/>(more changes)
    Node->>Operator: Reconcile
    Operator->>ConfigMap: Apply Updates
    Note over ConfigMap: Syncs continuously
```

**Key Differences from Example 1:**

| Aspect | Example 1 (PVC)Once + Retain | Example 4 (ConfigMap)WhenNeeded + Retain |
|--------|-----------------------------------|-----------------------------------------------|
| **Updates** | 🚫 Never (frozen after creation) | ✅ Always (syncs with template) |
| **Retention** | ✅ Yes (orphaned on delete) | ✅ Yes (orphaned on delete) |
| **Re-adoption** | ❌ No (skipped due to created-once) | ✅ Yes (automatic on recreate) |
| **Force Ownership** | ❌ No (Stuck policy) | ✅ Yes (Force policy) |

***

## Policy Combinations Summary

Quick reference comparing all four examples:

| Aspect | PVC (Stateful) | Init Job | App Deployment | Shared Config |
|--------|----------------|----------|----------------|---------------|
| **CreationPolicy** | `Once` | `Once` | `WhenNeeded` | `WhenNeeded` |
| **DeletionPolicy** | `Retain` | `Delete` | `Delete` | `Retain` |
| **ConflictPolicy** | `Stuck` | `Force` | `Stuck` | `Force` |
| **PatchStrategy** | `apply` | `replace` | `apply` | `apply` |
| **ownerReference** | ❌ No | ✅ Yes | ✅ Yes | ❌ No |
| **Updates** | 🚫 Never | 🚫 Never | ✅ Always | ✅ Always |
| **Survives Deletion** | ✅ Yes | ❌ No | ❌ No | ✅ Yes |
| **Auto-Cleanup** | ❌ Manual | ✅ Auto (GC) | ✅ Auto (GC) | ❌ Manual |
| **Drift Correction** | N/A (Once) | N/A (Once) | ✅ Yes | ✅ Yes |
| **Conflict Handling** | ⚠️ Stop | 💪 Force | ⚠️ Stop | 💪 Force |

**Legend:**

* ✅ Enabled / Yes
* ❌ Disabled / No
* 🚫 Never updates
* ⚠️ Safe mode (stops on conflict)
* 💪 Aggressive (forces ownership)
* N/A: Not applicable

## Decision Tree

Choose the right policy combination for your use case:

```mermaid
flowchart TD
    Start([Choose Policy Combination])
    Q1{Resource holds<br/>persistent data?}
    Q2{Needs continuous<br/>updates?}
    Q3{Runs only once?}
    Q4{Should survive<br/>node deletion?}
    Q5{Conflict<br/>tolerance?}

    Result1[Example 1: PVC<br/>Once + Retain + Stuck]
    Result2[Example 2: Init Job<br/>Once + Delete + Force]
    Result3[Example 3: App Deployment<br/>WhenNeeded + Delete + Stuck]
    Result4[Example 4: Shared Config<br/>WhenNeeded + Retain + Force]

    Start --> Q1
    Q1 -->|Yes| Q4
    Q1 -->|No| Q2

    Q4 -->|Yes| Result1
    Q4 -->|No| Q3

    Q3 -->|Yes| Q5
    Q3 -->|No| Q2

    Q5 -->|Force| Result2
    Q5 -->|Stuck| Result1

    Q2 -->|Yes| Q4
    Q2 -->|No| Q3

    Q4 -->|Yes| Result4
    Q4 -->|No| Result3

    classDef decisionStyle fill:#fff3e0,stroke:#ff9800,stroke-width:2px;
    classDef resultStyle fill:#e8f5e9,stroke:#4caf50,stroke-width:2px;

    class Q1,Q2,Q3,Q4,Q5 decisionStyle;
    class Result1,Result2,Result3,Result4 resultStyle;
```

## See Also

* [Policies Guide](policies.md) - Core concepts and policy types
* [Field-Level Ignore Control](field-ignore.md) - Fine-grained field management
* [Dependencies Guide](dependencies.md) - Resource ordering
* [Troubleshooting](troubleshooting.md) - Common policy issues
