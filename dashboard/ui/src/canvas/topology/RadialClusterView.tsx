import { useEffect, useRef, useCallback, useMemo } from 'react'
import * as d3 from 'd3-hierarchy'
import { arc as d3Arc } from 'd3-shape'
import { zoom as d3Zoom, zoomIdentity } from 'd3-zoom'
import { select } from 'd3-selection'
import 'd3-transition'
import type { ZoomBehavior } from 'd3-zoom'
import type { Selection } from 'd3-selection'
import type { TopologyData, TopologyNode, ResourceStatus } from '@/types/lynq'
import { STATUS_COLORS } from './constants'

// ─────────────────────────────────────────
// Types
// ─────────────────────────────────────────

interface HierarchyDatum {
  id: string
  name: string
  namespace: string
  status: ResourceStatus
  nodeType: 'hub' | 'form' | 'node' | 'resource' | 'orphan'
  metrics: { desired: number; ready: number; failed: number }
  children?: HierarchyDatum[]
  _original: TopologyNode
}

interface LayoutResult {
  root: d3.HierarchyPointNode<HierarchyDatum>
  outerR: number
  nodeR: number
}

interface RadialClusterViewProps {
  data: TopologyData
  width: number
  height: number
  highlightedNodeIds?: Set<string>
  highlightMode?: 'none' | 'search' | 'problem'
  onNodeClick: (node: TopologyNode) => void
  selectedNodeId?: string | null
  focusedNodeId?: string | null
  showResources?: boolean
}

// ─────────────────────────────────────────
// Constants
// ─────────────────────────────────────────

const HUB_R = 44
const FORM_R = 18
const RESOURCE_HALF = 3   // half-size of resource squares (visual)
const CANVAS_PAD = 60
const LINK_COLOR = '#cbd5e1'
const RESOURCE_LINK_COLOR = '#e2e8f0'

function statusColor(status: ResourceStatus, field: 'accent' | 'bg' = 'accent') {
  return STATUS_COLORS[status]?.[field] ?? '#94a3b8'
}

// Node radius adapts to leaf count so nodes never overlap at initial zoom
function adaptiveNodeR(leafCount: number): number {
  if (leafCount > 800) return 2.0
  if (leafCount > 400) return 2.5
  if (leafCount > 150) return 3.5
  if (leafCount > 80) return 5
  if (leafCount > 40) return 6
  return 8
}

// ─────────────────────────────────────────
// Build D3 hierarchy
// Unions children[] and edges; supports multiple hubs via virtual root.
// When showResources is true, walks one level deeper (hub→form→node→resource).
// ─────────────────────────────────────────

function buildHierarchy(data: TopologyData, showResources: boolean): HierarchyDatum | null {
  const nodeMap = new Map(data.nodes.map((n) => [n.id, n]))
  const maxDepth = showResources ? 3 : 2

  // Build parent→children map from both children[] arrays and edges
  const childrenMap = new Map<string, Set<string>>()
  for (const n of data.nodes) {
    for (const childId of (n.children ?? [])) {
      if (!childrenMap.has(n.id)) childrenMap.set(n.id, new Set())
      childrenMap.get(n.id)!.add(childId)
    }
  }
  for (const edge of data.edges) {
    if (!childrenMap.has(edge.source)) childrenMap.set(edge.source, new Set())
    childrenMap.get(edge.source)!.add(edge.target)
  }

  const hubs = data.nodes.filter((n) => n.type === 'hub')
  if (hubs.length === 0) return null

  function toHierarchyDatum(n: TopologyNode, depth = 0): HierarchyDatum {
    const childIds = depth < maxDepth ? [...(childrenMap.get(n.id) ?? [])] : []
    const children = childIds
      .map((id) => nodeMap.get(id))
      .filter((c): c is TopologyNode => {
        if (!c) return false
        if (c.type === 'orphan') return false
        // At depth < maxDepth-1 (i.e., not the last expansion level), skip resources
        if (!showResources && c.type === 'resource') return false
        return true
      })
      .map((c) => toHierarchyDatum(c, depth + 1))

    // Compute hub/form metrics from actual leaf statuses — bypasses stale operator metrics
    let metrics = n.metrics
    if ((n.type === 'hub' || n.type === 'form') && children.length > 0) {
      const leaves: HierarchyDatum[] = []
      const collect = (nodes: HierarchyDatum[]) => {
        for (const c of nodes) {
          if (!c.children || c.children.length === 0) leaves.push(c)
          else collect(c.children)
        }
      }
      collect(children)
      // Only aggregate from LynqNode leaves (not resource leaves) to keep hub metrics stable
      const nodeLeavesOnly = leaves.filter(l => l.nodeType === 'node')
      const src = nodeLeavesOnly.length > 0 ? nodeLeavesOnly : leaves
      if (src.length > 0) {
        metrics = {
          desired: nodeLeavesOnly.length > 0 ? src.length : (n.metrics?.desired ?? src.length),
          ready: src.filter((l) => l.status === 'ready').length,
          failed: src.filter((l) => l.status === 'failed').length,
        }
      }
    }

    return {
      id: n.id,
      name: n.name,
      namespace: n.namespace,
      status: n.status,
      nodeType: n.type,
      metrics,
      children: children.length > 0 ? children : undefined,
      _original: n,
    }
  }

  if (hubs.length === 1) return toHierarchyDatum(hubs[0])

  // Multiple hubs: virtual root
  return {
    id: '__root__',
    name: 'root',
    namespace: '',
    status: 'ready',
    nodeType: 'hub',
    metrics: { desired: 0, ready: 0, failed: 0 },
    children: hubs.map((h) => toHierarchyDatum(h)),
    _original: hubs[0],
  }
}

// ─────────────────────────────────────────
// Donut arc helper
// ─────────────────────────────────────────

function buildDonutSegments(
  ready: number,
  failed: number,
  pending: number,
  outerR: number,
  innerR: number,
): { d: string; color: string }[] {
  const total = ready + failed + pending || 1
  const segments: { value: number; status: ResourceStatus }[] = []
  if (ready > 0) segments.push({ value: ready, status: 'ready' })
  if (failed > 0) segments.push({ value: failed, status: 'failed' })
  if (pending > 0) segments.push({ value: pending, status: 'pending' })
  if (segments.length === 0) segments.push({ value: 1, status: 'skipped' })

  const arcGen = d3Arc<{ startAngle: number; endAngle: number }>()
    .innerRadius(innerR)
    .outerRadius(outerR)
    .padAngle(0.03)
    .cornerRadius(2)

  let angle = -Math.PI / 2
  return segments.map(({ value, status }) => {
    const span = (value / total) * 2 * Math.PI
    const seg = arcGen({ startAngle: angle, endAngle: angle + span }) ?? ''
    angle += span
    return { d: seg, color: statusColor(status) }
  })
}

// ─────────────────────────────────────────
// Main component
// ─────────────────────────────────────────

export function RadialClusterView({
  data,
  width,
  height,
  highlightedNodeIds,
  highlightMode,
  onNodeClick,
  selectedNodeId,
  focusedNodeId,
  showResources = false,
}: RadialClusterViewProps) {
  const svgRef = useRef<SVGSVGElement>(null)
  const gRef = useRef<SVGGElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const zoomRef = useRef<ZoomBehavior<SVGSVGElement, unknown> | null>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)
  const lastFitOuterR = useRef<number | null>(null)
  const onNodeClickRef = useRef(onNodeClick)
  const prefersReducedMotion = useRef(
    typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )

  useEffect(() => { onNodeClickRef.current = onNodeClick }, [onNodeClick])

  // ── Build hierarchy ──────────────────────
  const hierarchyData = useMemo(
    () => buildHierarchy(data, showResources),
    [data, showResources]
  )

  // ── Compute layout with adaptive sizing ──
  const layoutResult = useMemo((): LayoutResult | null => {
    if (!hierarchyData) return null
    const root = d3.hierarchy(hierarchyData)
    const leafCount = root.leaves().length

    const nodeR = adaptiveNodeR(leafCount)

    // Grow outerR if the container is too small for non-overlapping nodes.
    // Resource squares are RESOURCE_HALF*2 wide, use that as min spacing unit.
    const spacingUnit = showResources ? Math.max(RESOURCE_HALF * 2.5, nodeR * 2.5) : nodeR * 2.5
    const minRFromLeaves = Math.ceil((spacingUnit * leafCount) / (2 * Math.PI))
    const containerR = Math.max(10, Math.min(width, height) / 2 - CANVAS_PAD)
    const outerR = Math.max(minRFromLeaves, containerR)

    const clusterLayout = d3.cluster<HierarchyDatum>()
      .size([2 * Math.PI, outerR])
      .separation((a, b) => (a.parent === b.parent ? 1 : leafCount < 20 ? 2 : 1.2))

    const layoutRoot = clusterLayout(root)

    // Polar → Cartesian
    layoutRoot.each((node) => {
      const n = node as d3.HierarchyPointNode<HierarchyDatum> & { px: number; py: number }
      n.px = node.y * Math.cos(node.x - Math.PI / 2)
      n.py = node.y * Math.sin(node.x - Math.PI / 2)
    })

    return { root: layoutRoot, outerR, nodeR }
  }, [hierarchyData, width, height, showResources])

  // ── Zoom behavior ────────────────────────
  // Re-attach event listener on resize, but only set initial transform once.
  // Re-fit (animated) whenever outerR changes — handles both resize and resource toggle.
  useEffect(() => {
    const svg = svgRef.current
    const g = gRef.current
    if (!svg || !g) return

    const zoom = d3Zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 6])
      .on('zoom', (event) => {
        select(g).attr('transform', event.transform.toString())
      })

    zoomRef.current = zoom
    select(svg).call(zoom)

    if (layoutResult && width > 50 && height > 50 && layoutResult.outerR !== lastFitOuterR.current) {
      lastFitOuterR.current = layoutResult.outerR
      const fitScale = Math.min(
        0.95,
        Math.max(0.1, (Math.min(width, height) / 2 - 10) / layoutResult.outerR)
      )
      // Animated re-fit — smooth when toggling resources on/off
      select(svg).transition().duration(prefersReducedMotion.current ? 0 : 500).call(
        zoom.transform,
        zoomIdentity.translate(width / 2, height / 2).scale(fitScale)
      )
    }

    return () => { select(svg).on('.zoom', null) }
  }, [width, height, layoutResult])

  // ── D3 enter / update / exit render ─────
  useEffect(() => {
    if (!layoutResult || !gRef.current) return
    const { root: layoutNodes, nodeR } = layoutResult
    const reduced = prefersReducedMotion.current
    const g = select(gRef.current)

    const nodes = layoutNodes.descendants() as (d3.HierarchyPointNode<HierarchyDatum> & { px: number; py: number })[]
    const links = layoutNodes.links()

    // ── Links (bezier curves) ──
    // Split links into regular and resource links for separate styling
    const linkSel = g.selectAll<SVGPathElement, typeof links[0]>('path.radial-link')
      .data(links, (d) => `${d.source.data.id}--${d.target.data.id}`)

    const isResourceLink = (d: typeof links[0]) => d.target.data.nodeType === 'resource'

    linkSel.enter()
      .append('path')
      .attr('class', 'radial-link')
      .attr('fill', 'none')
      .attr('stroke', (d) => isResourceLink(d) ? RESOURCE_LINK_COLOR : LINK_COLOR)
      .attr('stroke-width', (d) => isResourceLink(d) ? 0.7 : 1.2)
      .attr('stroke-dasharray', (d) => isResourceLink(d) ? '2 3' : 'none')
      .attr('stroke-opacity', 0)
      .attr('d', (d) => radialCurve(d))
      .transition().duration(reduced ? 0 : 600)
      .attr('stroke-opacity', (d) => linkOpacity(d.target.data.id, isResourceLink(d), highlightedNodeIds, highlightMode))

    linkSel
      .transition().duration(reduced ? 0 : 400)
      .attr('stroke', (d) => isResourceLink(d) ? RESOURCE_LINK_COLOR : LINK_COLOR)
      .attr('stroke-width', (d) => isResourceLink(d) ? 0.7 : 1.2)
      .attr('stroke-dasharray', (d) => isResourceLink(d) ? '2 3' : 'none')
      .attr('d', (d) => radialCurve(d))
      .attr('stroke-opacity', (d) => linkOpacity(d.target.data.id, isResourceLink(d), highlightedNodeIds, highlightMode))

    linkSel.exit()
      .transition().duration(reduced ? 0 : 200)
      .attr('stroke-opacity', 0)
      .remove()

    // ── Node groups ──
    const nodeSel = g.selectAll<SVGGElement, typeof nodes[0]>('g.radial-node')
      .data(nodes, (d) => d.data.id)

    const nodeEnter = nodeSel.enter()
      .append('g')
      .attr('class', 'radial-node')
      .attr('transform', (d) => `translate(${d.px},${d.py}) scale(0)`)
      .attr('cursor', 'pointer')
      .attr('role', 'button')
      .attr('tabindex', '0')
      .attr('aria-label', (d) => `${d.data.nodeType} ${d.data.name}`)
      .on('click', (_event, d) => onNodeClickRef.current(d.data._original))
      .on('mouseenter', (event, d) => showTooltip(event, d, tooltipRef.current, containerRef.current))
      .on('mouseleave', () => hideTooltip(tooltipRef.current))
      .on('keydown', (event, d) => {
        if (event.key === 'Enter' || event.key === ' ') onNodeClickRef.current(d.data._original)
      })

    // Resource nodes animate faster and with a tighter stagger
    nodeEnter.transition()
      .duration((d) => reduced ? 0 : (d.data.nodeType === 'resource' ? 250 : 500))
      .delay((_d, i) => Math.min(i * (showResources ? 8 : 15), 500))
      .attr('transform', (d) => `translate(${d.px},${d.py}) scale(1)`)

    const nodeMerge = nodeEnter.merge(
      nodeSel as unknown as Selection<SVGGElement, typeof nodes[0], SVGGElement, unknown>
    )

    nodeMerge.transition().duration(reduced ? 0 : 400)
      .attr('transform', (d) => `translate(${d.px},${d.py}) scale(1)`)
      .attr('opacity', (d) => nodeOpacity(d.data.id, d.data.nodeType, highlightedNodeIds, highlightMode))

    nodeSel.exit()
      .transition().duration(reduced ? 0 : 180)
      .attr('transform', (d) => `translate(${(d as typeof nodes[0]).px},${(d as typeof nodes[0]).py}) scale(0)`)
      .remove()

    // Redraw node visuals (clears inner elements then redraws)
    nodeMerge.each(function(d) {
      const el = select(this)
      el.selectAll('*').remove()
      drawNode(el, d, nodeR, selectedNodeId, focusedNodeId, highlightedNodeIds, highlightMode)
    })

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layoutResult, highlightedNodeIds, highlightMode, selectedNodeId, focusedNodeId, showResources])

  // ── Pan to focused node when search cycling ──
  useEffect(() => {
    if (!focusedNodeId || !layoutResult || !svgRef.current || !zoomRef.current) return
    const found = (
      layoutResult.root.descendants() as (d3.HierarchyPointNode<HierarchyDatum> & { px: number; py: number })[]
    ).find((d) => d.data.id === focusedNodeId)
    if (!found) return
    const scale = Math.min(2, (Math.min(width, height) / 2 - 20) / (found.y + 40))
    select(svgRef.current).transition().duration(400).call(
      zoomRef.current.transform,
      zoomIdentity
        .translate(width / 2 - found.px * scale, height / 2 - found.py * scale)
        .scale(scale)
    )
  }, [focusedNodeId, width, height, layoutResult])

  const resetZoom = useCallback(() => {
    const svg = svgRef.current
    if (!svg || !zoomRef.current || !layoutResult) return
    const fitScale = Math.min(0.95, Math.max(0.1, (Math.min(width, height) / 2 - 10) / layoutResult.outerR))
    select(svg).transition().duration(400).call(
      zoomRef.current.transform,
      zoomIdentity.translate(width / 2, height / 2).scale(fitScale)
    )
  }, [width, height, layoutResult])

  // Scale guard: warn when resource count is high
  const resourceCount = useMemo(
    () => data.nodes.filter(n => n.type === 'resource').length,
    [data]
  )
  const showScaleWarning = showResources && resourceCount > 2000

  return (
    <div ref={containerRef} className="relative w-full h-full">
      <svg ref={svgRef} width={width} height={height} className="w-full h-full">
        <g ref={gRef} />
      </svg>

      {/* Floating tooltip */}
      <div
        ref={tooltipRef}
        className="pointer-events-none absolute z-50 hidden rounded-lg border bg-popover px-3 py-2 text-sm shadow-md max-w-[240px]"
      />

      {/* Scale warning */}
      {showScaleWarning && (
        <div className="absolute bottom-16 right-4 text-[10px] text-amber-600 bg-amber-50 border border-amber-200 rounded px-2 py-1">
          {resourceCount.toLocaleString()} resources — performance may vary
        </div>
      )}

      {/* Zoom controls */}
      <div className="absolute bottom-4 right-4 flex flex-col gap-1">
        <button
          onClick={() => {
            const svg = svgRef.current
            if (svg && zoomRef.current) select(svg).transition().duration(200).call(zoomRef.current.scaleBy, 1.3)
          }}
          className="w-7 h-7 rounded border bg-background hover:bg-accent flex items-center justify-center text-sm cursor-pointer"
        >+</button>
        <button
          onClick={() => {
            const svg = svgRef.current
            if (svg && zoomRef.current) select(svg).transition().duration(200).call(zoomRef.current.scaleBy, 0.77)
          }}
          className="w-7 h-7 rounded border bg-background hover:bg-accent flex items-center justify-center text-sm cursor-pointer"
        >−</button>
        <button
          onClick={resetZoom}
          className="w-7 h-7 rounded border bg-background hover:bg-accent flex items-center justify-center text-[10px] cursor-pointer"
          title="Fit"
        >⊡</button>
      </div>
    </div>
  )
}

// ─────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────

function radialCurve(d: d3.HierarchyPointLink<HierarchyDatum>): string {
  const s = d.source as d3.HierarchyPointNode<HierarchyDatum> & { px: number; py: number }
  const t = d.target as d3.HierarchyPointNode<HierarchyDatum> & { px: number; py: number }
  const midR = (s.y + t.y) / 2
  const sAngle = s.x - Math.PI / 2
  const tAngle = t.x - Math.PI / 2
  const cx1 = midR * Math.cos(sAngle)
  const cy1 = midR * Math.sin(sAngle)
  const cx2 = midR * Math.cos(tAngle)
  const cy2 = midR * Math.sin(tAngle)
  return `M${s.px},${s.py} C${cx1},${cy1} ${cx2},${cy2} ${t.px},${t.py}`
}

function nodeOpacity(
  id: string,
  nodeType: string,
  highlighted: Set<string> | undefined,
  mode: string | undefined
): number {
  if (!mode || mode === 'none' || !highlighted || highlighted.size === 0) return 1
  if (highlighted.has(id)) return 1
  // Resource nodes dim more aggressively during highlight modes
  return nodeType === 'resource' ? 0.1 : 0.25
}

function linkOpacity(
  targetId: string,
  isResource: boolean,
  highlighted: Set<string> | undefined,
  mode: string | undefined
): number {
  const base = isResource ? 0.18 : 0.5
  if (!mode || mode === 'none' || !highlighted || highlighted.size === 0) return base
  return highlighted.has(targetId) ? (isResource ? 0.4 : 0.8) : (isResource ? 0.04 : 0.08)
}

function drawNode(
  el: Selection<SVGGElement, unknown, null, undefined>,
  d: d3.HierarchyPointNode<HierarchyDatum> & { px: number; py: number },
  nodeR: number,
  selectedNodeId: string | null | undefined,
  focusedNodeId: string | null | undefined,
  highlighted: Set<string> | undefined,
  highlightMode: string | undefined,
) {
  const datum = d.data
  const isSelected = selectedNodeId === datum.id
  const isFocused = focusedNodeId === datum.id
  const isHighlighted = highlighted?.has(datum.id) ?? false
  const accent = statusColor(datum.status)

  if (datum.nodeType === 'hub') {
    // Hub: large donut showing ready/failed proportions
    const segs = buildDonutSegments(datum.metrics.ready, datum.metrics.failed, 0, HUB_R, HUB_R * 0.55)
    segs.forEach(({ d: path, color }) => {
      el.append('path').attr('d', path).attr('fill', color).attr('fill-opacity', 0.9)
    })
    el.append('circle').attr('r', HUB_R * 0.52).attr('fill', 'white').attr('fill-opacity', 0.95)
    el.append('text')
      .attr('text-anchor', 'middle').attr('y', -6)
      .attr('font-size', 10).attr('font-weight', '700').attr('fill', '#1e293b')
      .text(datum.name.length > 10 ? datum.name.slice(0, 10) + '…' : datum.name)
    el.append('text')
      .attr('text-anchor', 'middle').attr('y', 7)
      .attr('font-size', 9).attr('fill', '#64748b')
      .text(`${datum.metrics.ready}/${datum.metrics.desired}`)
    if (datum.metrics.failed > 0) {
      el.append('text')
        .attr('text-anchor', 'middle').attr('y', 18)
        .attr('font-size', 8).attr('fill', '#dc2626')
        .text(`${datum.metrics.failed} failed`)
    }
    if (isSelected || isFocused) {
      el.append('circle').attr('r', HUB_R + 6).attr('fill', 'none')
        .attr('stroke', isFocused ? '#f59e0b' : accent)
        .attr('stroke-width', 2.5).attr('stroke-dasharray', '5 3')
    }

  } else if (datum.nodeType === 'form') {
    // Form: medium donut showing node health proportions
    el.append('circle').attr('r', FORM_R + 2).attr('fill', 'white').attr('fill-opacity', 0.0)
    const pending = Math.max(datum.metrics.desired - datum.metrics.ready - datum.metrics.failed, 0)
    const segs = buildDonutSegments(datum.metrics.ready, datum.metrics.failed, pending, FORM_R, FORM_R * 0.55)
    segs.forEach(({ d: path, color }) => {
      el.append('path').attr('d', path).attr('fill', color)
    })
    el.append('circle').attr('r', FORM_R * 0.52).attr('fill', statusColor(datum.status, 'bg'))
    el.append('text')
      .attr('text-anchor', 'middle').attr('y', FORM_R + 13)
      .attr('font-size', 9).attr('fill', '#475569').attr('font-weight', '500')
      .text(datum.name.length > 12 ? datum.name.slice(0, 12) + '…' : datum.name)
    if (isHighlighted && highlightMode !== 'none') {
      el.append('circle').attr('r', FORM_R + 5).attr('fill', 'none')
        .attr('stroke', highlightMode === 'problem' ? '#dc2626' : '#0d9488')
        .attr('stroke-width', 2).attr('stroke-dasharray', '4 2')
    }
    if (isSelected || isFocused) {
      el.append('circle').attr('r', FORM_R + 4).attr('fill', 'none')
        .attr('stroke', isFocused ? '#f59e0b' : accent)
        .attr('stroke-width', 2)
    }

  } else if (datum.nodeType === 'resource') {
    // Resource: small rounded square, angled to follow radial direction
    const half = RESOURCE_HALF
    const angle = (d.x - Math.PI / 2) * (180 / Math.PI)

    if (isHighlighted && highlightMode !== 'none') {
      el.append('rect')
        .attr('x', -(half + 3.5)).attr('y', -(half + 3.5))
        .attr('width', (half + 3.5) * 2).attr('height', (half + 3.5) * 2)
        .attr('rx', 2.5)
        .attr('fill', 'none')
        .attr('stroke', highlightMode === 'problem' ? '#dc2626' : '#0d9488')
        .attr('stroke-width', 1.2)
        .attr('stroke-dasharray', '3 2')
        .attr('transform', `rotate(${angle + 45})`)
    }

    el.append('rect')
      .attr('x', -half).attr('y', -half)
      .attr('width', half * 2).attr('height', half * 2)
      .attr('rx', 1)
      .attr('fill', accent)
      .attr('fill-opacity', datum.status === 'skipped' ? 0.5 : 0.82)
      .attr('stroke', 'white')
      .attr('stroke-width', 0.8)
      .attr('transform', `rotate(${angle + 45})`)

    // Inner mark for failed state (white X)
    if (datum.status === 'failed') {
      const s = half * 0.45
      el.append('line')
        .attr('x1', -s).attr('y1', -s).attr('x2', s).attr('y2', s)
        .attr('stroke', 'white').attr('stroke-width', 1).attr('stroke-opacity', 0.9)
      el.append('line')
        .attr('x1', s).attr('y1', -s).attr('x2', -s).attr('y2', s)
        .attr('stroke', 'white').attr('stroke-width', 1).attr('stroke-opacity', 0.9)
    }

    if (isSelected) {
      el.append('rect')
        .attr('x', -(half + 3)).attr('y', -(half + 3))
        .attr('width', (half + 3) * 2).attr('height', (half + 3) * 2)
        .attr('rx', 2)
        .attr('fill', 'none')
        .attr('stroke', accent).attr('stroke-width', 1.5)
        .attr('transform', `rotate(${angle + 45})`)
    }
    if (isFocused) {
      el.append('rect')
        .attr('x', -(half + 4)).attr('y', -(half + 4))
        .attr('width', (half + 4) * 2).attr('height', (half + 4) * 2)
        .attr('rx', 2.5)
        .attr('fill', 'none')
        .attr('stroke', '#f59e0b').attr('stroke-width', 2)
        .attr('transform', `rotate(${angle + 45})`)
    }

  } else {
    // LynqNode leaf: small circle, color = status
    const r = nodeR
    if (isHighlighted && highlightMode !== 'none') {
      el.append('circle').attr('r', r + 4).attr('fill', 'none')
        .attr('stroke', highlightMode === 'problem' ? '#dc2626' : '#0d9488')
        .attr('stroke-width', 1.5).attr('stroke-dasharray', '3 2')
    }
    el.append('circle')
      .attr('r', r)
      .attr('fill', accent)
      .attr('fill-opacity', 0.85)
      .attr('stroke', 'white')
      .attr('stroke-width', r > 5 ? 1.5 : 1)
    if (datum.status === 'failed') {
      el.append('circle').attr('r', Math.max(1.5, r * 0.3)).attr('fill', 'white').attr('fill-opacity', 0.8)
    }
    if (isSelected) {
      el.append('circle').attr('r', r + 3.5).attr('fill', 'none').attr('stroke', accent).attr('stroke-width', 2)
    }
    if (isFocused) {
      el.append('circle').attr('r', r + 4).attr('fill', 'none').attr('stroke', '#f59e0b').attr('stroke-width', 2.5)
    }
    // Angle-aware labels — only when siblings are few enough to be readable
    const totalSiblings = d.parent?.children?.length ?? 1
    const labelThreshold = r <= 4 ? 20 : 30
    if (totalSiblings <= labelThreshold) {
      const angle = (d.x - Math.PI / 2) * (180 / Math.PI)
      const flip = d.x > Math.PI / 2 && d.x < (3 * Math.PI) / 2
      el.append('text')
        .attr('transform', `rotate(${angle}) translate(${r + 4},0) rotate(${flip ? 180 : 0})`)
        .attr('text-anchor', flip ? 'end' : 'start')
        .attr('dominant-baseline', 'middle')
        .attr('font-size', 9)
        .attr('fill', '#64748b')
        .text(datum.name.length > 14 ? datum.name.slice(0, 14) + '…' : datum.name)
    }
  }
}

// Parse "Kind/Name" from resource node name field
function parseResourceName(raw: string): { kind: string; name: string } {
  const slash = raw.indexOf('/')
  if (slash === -1) return { kind: raw, name: raw }
  return { kind: raw.slice(0, slash), name: raw.slice(slash + 1) }
}

function showTooltip(
  event: MouseEvent,
  d: d3.HierarchyPointNode<HierarchyDatum>,
  el: HTMLDivElement | null,
  container: HTMLDivElement | null,
) {
  if (!el || !container) return
  const datum = d.data
  let lines: string[]

  if (datum.nodeType === 'resource') {
    const { kind, name } = parseResourceName(datum.name)
    lines = [
      `<span style="font-weight:600">${name}</span>`,
      `<span style="font-size:11px;color:#7c3aed;font-weight:500">${kind}</span>`,
      datum.namespace
        ? `<span style="font-size:11px;color:#64748b">${datum.namespace}</span>`
        : '',
      `<span style="font-size:11px;color:${datum.status === 'failed' ? '#dc2626' : datum.status === 'skipped' ? '#64748b' : '#0d9488'}">${datum.status}</span>`,
    ].filter(Boolean)
  } else {
    lines = [
      `<span style="font-weight:600">${datum.name}</span>`,
      `<span style="font-size:11px;color:#64748b">${datum.namespace}</span>`,
      datum.nodeType !== 'hub' && datum.nodeType !== 'form'
        ? `<span style="font-size:11px;color:${datum.status === 'failed' ? '#dc2626' : '#64748b'}">${datum.status}</span>`
        : `<span style="font-size:11px">${datum.metrics.ready}/${datum.metrics.desired} ready${datum.metrics.failed > 0 ? ` · <span style="color:#dc2626">${datum.metrics.failed} failed</span>` : ''}</span>`,
    ]
  }

  el.innerHTML = lines.join('<br/>')
  el.style.display = 'block'
  const rect = container.getBoundingClientRect()
  el.style.left = `${event.clientX - rect.left + 14}px`
  el.style.top = `${event.clientY - rect.top - 8}px`
}

function hideTooltip(el: HTMLDivElement | null) {
  if (el) el.style.display = 'none'
}
