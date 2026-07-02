// Ported verbatim from dashboard/ui/src/canvas/topology/constants.ts
// Hex values are self-contained; the graph does NOT depend on Tailwind.

export const STATUS_COLORS = {
  ready: {
    accent: '#0d9488',
    bg: '#f0fdfa',
    text: '#0f766e',
    border: '#99f6e4',
  },
  pending: {
    accent: '#d97706',
    bg: '#fffbeb',
    text: '#b45309',
    border: '#fde68a',
  },
  failed: {
    accent: '#dc2626',
    bg: '#fef2f2',
    text: '#b91c1c',
    border: '#fecaca',
  },
  skipped: {
    accent: '#64748b',
    bg: '#f8fafc',
    text: '#475569',
    border: '#e2e8f0',
  },
}

// Sunburst dimensions
export const SUNBURST_CENTER_RADIUS = 60      // Hub circle
export const SUNBURST_FORM_INNER = 80         // Form ring inner
export const SUNBURST_FORM_OUTER = 150        // Form ring outer
export const SUNBURST_STATUS_INNER = 165      // Status ring inner
export const SUNBURST_STATUS_OUTER = 220      // Status ring outer

// Radial tree
export const RADIAL_NODE_RADIUS = 8
export const RESOURCE_R = 3            // visual half-size of resource squares

// Zoom thresholds (with hysteresis)
export const ZOOM_TIER1_MAX = 2.0             // Above this → Tier 2
export const ZOOM_TIER1_HYSTERESIS = 0.1      // Deadband to prevent oscillation

// Animation durations (ms)
export const ANIM_ENTER = 400
export const ANIM_UPDATE = 300
export const ANIM_EXIT = 300
export const ANIM_TIER_TRANSITION = 400
export const ANIM_TOOLTIP_DELAY = 150
export const ANIM_TOOLTIP_HIDE = 100

// Max simultaneous enter animations
export const ANIM_MAX_CONCURRENT_ENTER = 8
export const ANIM_STAGGER_MS = 30

// Orphan status color
export const ORPHAN_COLOR = '#d97706'
