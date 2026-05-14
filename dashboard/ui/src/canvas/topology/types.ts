import type { ResourceStatus, TopologyNode } from '@/types/lynq'

// Which tier is active
export type TierLevel = 'sunburst' | 'radial'

// Viewport transform
export interface ViewTransform {
  x: number
  y: number
  scale: number
}

// Sunburst arc datum
export interface SunburstDatum {
  id: string
  type: 'hub' | 'form' | 'status-bucket'
  label: string
  namespace: string
  status: ResourceStatus
  count: number          // node count (for status bucket) or form node count
  children?: SunburstDatum[]
  // resolved by layout
  startAngle?: number
  endAngle?: number
  innerRadius?: number
  outerRadius?: number
  node?: TopologyNode    // original topology node if type === 'form'
}

// Radial tree node
export interface RadialNode {
  id: string
  label: string
  namespace: string
  status: ResourceStatus
  angle: number          // radians, 0 = up (12 o'clock)
  radius: number         // distance from center
  x: number              // cartesian x (derived)
  y: number              // cartesian y (derived)
  node: TopologyNode
  // animation state
  animPhase?: 'enter' | 'stable' | 'exit'
  enterDelayMs?: number
}

// Filter state
export interface TopologyFilters {
  status: ResourceStatus | 'all'
  namespace: string
  formId: string
}

// Tooltip content
export interface TooltipInfo {
  x: number
  y: number
  title: string
  lines: string[]
}

// Persisted view state key in localStorage
export const PERSISTED_VIEW_KEY = 'lynq-topology-view'

export interface PersistedView {
  tier: TierLevel
  selectedFormId: string | null
  transform: ViewTransform
  filters: TopologyFilters
}
