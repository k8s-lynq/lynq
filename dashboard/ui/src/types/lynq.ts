// Lynq CRD Types - matching api/v1/*.go

// 'pending' is the progressing state (rolling out, recovering, or not reconciled yet); the UI
// labels it "Progressing". See lib/health.ts.
export type ResourceStatus = 'ready' | 'pending' | 'failed' | 'skipped'

// Common Kubernetes metadata
export interface ObjectMeta {
  name: string
  namespace: string
  uid?: string
  creationTimestamp?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

// Condition type used across all CRDs
export interface Condition {
  type: string
  status: 'True' | 'False' | 'Unknown'
  lastTransitionTime: string
  reason: string
  message: string
}

// LynqHub types
export interface LynqHubSpec {
  source: {
    mysql?: {
      host: string
      port: number
      database: string
      table: string
      userRef?: SecretKeyRef
      passwordRef?: SecretKeyRef
    }
    syncInterval: string
  }
  valueMappings: ValueMappings
  extraValueMappings?: ExtraValueMapping[]
}

export interface SecretKeyRef {
  name: string
  key: string
}

export interface ValueMappings {
  uid: string
  activate: string
}

export interface ExtraValueMapping {
  column: string
  variable: string
}

export interface LynqHubStatus {
  referencingTemplates: number
  desired: number
  ready: number
  failed: number
  lastSyncTime?: string
  conditions: Condition[]
}

export interface LynqHub {
  apiVersion: string
  kind: 'LynqHub'
  metadata: ObjectMeta
  spec: LynqHubSpec
  status?: LynqHubStatus
}

// LynqForm types
export interface TResource {
  id: string
  spec?: unknown
  dependIds?: string[]
  skipOnDependencyFailure?: boolean
  creationPolicy?: 'Once' | 'WhenNeeded'
  deletionPolicy?: 'Delete' | 'Retain'
  conflictPolicy?: 'Stuck' | 'Force'
  nameTemplate?: string
  targetNamespace?: string
  labelsTemplate?: Record<string, string>
  annotationsTemplate?: Record<string, string>
  waitForReady?: boolean
  timeoutSeconds?: number
  patchStrategy?: 'apply' | 'merge' | 'replace'
}

export interface LynqFormSpec {
  hubId: string
  serviceAccounts?: TResource[]
  deployments?: TResource[]
  statefulSets?: TResource[]
  daemonSets?: TResource[]
  services?: TResource[]
  ingresses?: TResource[]
  configMaps?: TResource[]
  secrets?: TResource[]
  persistentVolumeClaims?: TResource[]
  jobs?: TResource[]
  cronJobs?: TResource[]
  podDisruptionBudgets?: TResource[]
  networkPolicies?: TResource[]
  horizontalPodAutoscalers?: TResource[]
  manifests?: TResource[]
  rollout?: RolloutConfig
}

export interface RolloutConfig {
  maxSkew?: number
  progressDeadlineSeconds?: number
}

export type RolloutPhase = 'Idle' | 'InProgress' | 'Failed' | 'Complete'

// Matches RolloutStatus in api/v1/lynqform_types.go
export interface RolloutStatus {
  phase?: RolloutPhase
  targetGeneration?: number
  totalNodes?: number
  updatedNodes?: number
  updatingNodes?: number
  readyUpdatedNodes?: number
  startTime?: string
  completionTime?: string
  // On Failed, names the stalled nodes
  message?: string
}

// LynqFormStatus has no failed-node count: derive it from the nodes (lib/health.ts formNodeCounts)
export interface LynqFormStatus {
  observedGeneration?: number
  totalNodes: number
  readyNodes: number
  rollout?: RolloutStatus
  conditions: Condition[]
}

export interface LynqForm {
  apiVersion: string
  kind: 'LynqForm'
  metadata: ObjectMeta
  spec: LynqFormSpec
  status?: LynqFormStatus
}

// LynqNode types
// Note: hubRef is stored in metadata.labels["lynq.sh/hub"], not in spec
export interface LynqNodeSpec {
  uid: string
  templateRef: string
  data?: Record<string, unknown>
}

export interface LynqNodeStatus {
  desiredResources: number
  readyResources: number
  failedResources: number
  skippedResources: number
  skippedResourceIds?: string[]
  appliedResources?: string[]
  conditions: Condition[]
}

export interface LynqNode {
  apiVersion: string
  kind: 'LynqNode'
  metadata: ObjectMeta
  spec: LynqNodeSpec
  status?: LynqNodeStatus
}

// API Response types
export interface ListResponse<T> {
  items: T[]
  metadata?: {
    continue?: string
    resourceVersion?: string
  }
}

// Resource metadata for topology nodes
export interface ResourceMetadata {
  creationPolicy?: 'Once' | 'WhenNeeded'
  deletionPolicy?: 'Delete' | 'Retain'
  conflictPolicy?: 'Stuck' | 'Force'
  // Orphan metadata
  orphaned?: boolean
  orphanedAt?: string
  orphanedReason?: 'RemovedFromTemplate' | 'LynqNodeDeleted'
  originalNode?: string
  originalNodeNamespace?: string
  // LynqNode: Conflicted=True
  conflicted?: boolean
  // LynqForm: status.rollout.phase and message
  rolloutPhase?: RolloutPhase
  rolloutMessage?: string
}

// Topology types for visualization
export interface TopologyNode {
  id: string
  type: 'hub' | 'form' | 'node' | 'resource' | 'orphan'
  name: string
  namespace: string
  status: ResourceStatus
  children?: string[]
  metrics: {
    desired: number
    ready: number
    failed: number
  }
  metadata?: ResourceMetadata
}

export interface TopologyEdge {
  source: string
  target: string
}

export interface TopologyData {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

// Context selector
export interface KubeContext {
  name: string
  cluster: string
  user: string
  namespace?: string
  current: boolean
}

// Kubernetes Event types
export interface KubernetesEvent {
  type: 'Normal' | 'Warning'
  reason: string
  message: string
  firstTimestamp: string
  lastTimestamp: string
  count: number
  involvedObject: {
    kind: string
    name: string
    namespace: string
  }
  source: {
    component: string
  }
}

// Event stream message types for SSE
export interface EventStreamMessage {
  event: 'connected' | 'event' | 'heartbeat' | 'error'
  data: KubernetesEvent | { status: string } | { timestamp: number } | { error: string }
}
