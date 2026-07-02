<template>
  <div ref="containerRef" class="topo-root">
    <svg ref="svgRef" :width="width" height="100%" class="topo-svg" :style="{ width: width + 'px' }">
      <g ref="gRef" />
    </svg>

    <!-- Floating tooltip -->
    <div ref="tooltipRef" class="topo-tooltip" />

    <!-- Zoom controls -->
    <div class="topo-zoom-controls">
      <button class="topo-zoom-btn" @click="zoomIn">+</button>
      <button class="topo-zoom-btn" @click="zoomOut">&#8722;</button>
      <button class="topo-zoom-btn topo-zoom-btn-fit" title="Fit" @click="resetZoom">&#9187;</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { STATUS_COLORS } from './constants.js'

const props = defineProps({
  data: { type: Object, required: true },
  showResources: { type: Boolean, default: false },
})

// ─────────────────────────────────────────
// Constants (verbatim from RadialClusterView.tsx)
// ─────────────────────────────────────────

const HUB_R = 44
const FORM_R = 18
const RESOURCE_HALF = 3   // half-size of resource squares (visual)
const CANVAS_PAD = 60
const LINK_COLOR = 'rgba(255, 255, 255, 0.18)'
const RESOURCE_LINK_COLOR = 'rgba(255, 255, 255, 0.08)'

function statusColor(status, field = 'accent') {
  return STATUS_COLORS[status]?.[field] ?? '#94a3b8'
}

// Node radius adapts to leaf count so nodes never overlap at initial zoom
function adaptiveNodeR(leafCount) {
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

function buildHierarchy(data, showResources) {
  const nodeMap = new Map(data.nodes.map((n) => [n.id, n]))
  const maxDepth = showResources ? 3 : 2

  // Build parent→children map from both children[] arrays and edges
  const childrenMap = new Map()
  for (const n of data.nodes) {
    for (const childId of (n.children ?? [])) {
      if (!childrenMap.has(n.id)) childrenMap.set(n.id, new Set())
      childrenMap.get(n.id).add(childId)
    }
  }
  for (const edge of data.edges) {
    if (!childrenMap.has(edge.source)) childrenMap.set(edge.source, new Set())
    childrenMap.get(edge.source).add(edge.target)
  }

  const hubs = data.nodes.filter((n) => n.type === 'hub')
  if (hubs.length === 0) return null

  function toHierarchyDatum(n, depth = 0) {
    const childIds = depth < maxDepth ? [...(childrenMap.get(n.id) ?? [])] : []
    const children = childIds
      .map((id) => nodeMap.get(id))
      .filter((c) => {
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
      const leaves = []
      const collect = (nodes) => {
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
// Donut arc helper — needs d3Arc, injected on mount
// ─────────────────────────────────────────

let d3Arc = null

function buildDonutSegments(ready, failed, pending, outerR, innerR) {
  const total = ready + failed + pending || 1
  const segments = []
  if (ready > 0) segments.push({ value: ready, status: 'ready' })
  if (failed > 0) segments.push({ value: failed, status: 'failed' })
  if (pending > 0) segments.push({ value: pending, status: 'pending' })
  if (segments.length === 0) segments.push({ value: 1, status: 'skipped' })

  const arcGen = d3Arc()
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
// Helpers (verbatim)
// ─────────────────────────────────────────

function radialCurve(d) {
  const s = d.source
  const t = d.target
  const midR = (s.y + t.y) / 2
  const sAngle = s.x - Math.PI / 2
  const tAngle = t.x - Math.PI / 2
  const cx1 = midR * Math.cos(sAngle)
  const cy1 = midR * Math.sin(sAngle)
  const cx2 = midR * Math.cos(tAngle)
  const cy2 = midR * Math.sin(tAngle)
  return `M${s.px},${s.py} C${cx1},${cy1} ${cx2},${cy2} ${t.px},${t.py}`
}

function nodeOpacity(id, nodeType, highlighted, mode) {
  if (!mode || mode === 'none' || !highlighted || highlighted.size === 0) return 1
  if (highlighted.has(id)) return 1
  // Resource nodes dim more aggressively during highlight modes
  return nodeType === 'resource' ? 0.1 : 0.25
}

function linkOpacity(targetId, isResource, highlighted, mode) {
  const base = isResource ? 0.18 : 0.5
  if (!mode || mode === 'none' || !highlighted || highlighted.size === 0) return base
  return highlighted.has(targetId) ? (isResource ? 0.4 : 0.8) : (isResource ? 0.04 : 0.08)
}

function drawNode(el, d, nodeR, selectedNodeId, focusedNodeId, highlighted, highlightMode) {
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
    el.append('circle').attr('r', HUB_R * 0.52).attr('fill', '#0c0c10').attr('fill-opacity', 0.96)
    el.append('text')
      .attr('text-anchor', 'middle').attr('y', -6)
      .attr('font-size', 10).attr('font-weight', '700').attr('fill', '#ededed')
      .text(datum.name.length > 10 ? datum.name.slice(0, 10) + '…' : datum.name)
    el.append('text')
      .attr('text-anchor', 'middle').attr('y', 7)
      .attr('font-size', 9).attr('fill', 'rgba(255, 255, 255, 0.6)')
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
    el.append('circle').attr('r', FORM_R * 0.52).attr('fill', '#0c0c10').attr('fill-opacity', 0.9)
    el.append('text')
      .attr('text-anchor', 'middle').attr('y', FORM_R + 13)
      .attr('font-size', 9).attr('fill', 'rgba(255, 255, 255, 0.72)').attr('font-weight', '500')
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
        .attr('fill', 'rgba(255, 255, 255, 0.72)')
        .text(datum.name.length > 14 ? datum.name.slice(0, 14) + '…' : datum.name)
    }
  }
}

// Parse "Kind/Name" from resource node name field
function parseResourceName(raw) {
  const slash = raw.indexOf('/')
  if (slash === -1) return { kind: raw, name: raw }
  return { kind: raw.slice(0, slash), name: raw.slice(slash + 1) }
}

function showTooltip(event, d, el, container) {
  if (!el || !container) return
  const datum = d.data
  let lines

  if (datum.nodeType === 'resource') {
    const { kind, name } = parseResourceName(datum.name)
    lines = [
      `<span style="font-weight:600">${name}</span>`,
      `<span style="font-size:11px;color:#2b928e;font-weight:500">${kind}</span>`,
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

function hideTooltip(el) {
  if (el) el.style.display = 'none'
}

// ─────────────────────────────────────────
// Vue host — refs + effects (React ref/useEffect/useMemo translated)
// ─────────────────────────────────────────

const containerRef = ref(null)
const svgRef = ref(null)
const gRef = ref(null)
const tooltipRef = ref(null)

// non-interactive on the landing page — kept as local no-ops so the D3 code stays identical
const highlightedNodeIds = undefined
const highlightMode = 'none'
const selectedNodeId = null
const focusedNodeId = null

const width = ref(0)
const height = ref(0)

// D3 modules loaded lazily inside onMounted (SSR-safe)
let d3h = null      // d3-hierarchy
let selectFn = null // d3-selection select
let d3Zoom = null
let zoomIdentity = null

let zoomBehavior = null
let lastFitOuterR = null
let prefersReducedMotion = false

let hierarchyData = null
let layoutResult = null
let resizeObserver = null

function computeHierarchy() {
  hierarchyData = buildHierarchy(props.data, props.showResources)
}

function computeLayout() {
  if (!hierarchyData || !d3h) { layoutResult = null; return }
  const root = d3h.hierarchy(hierarchyData)
  const leafCount = root.leaves().length

  const nodeR = adaptiveNodeR(leafCount)

  // Grow outerR if the container is too small for non-overlapping nodes.
  const spacingUnit = props.showResources ? Math.max(RESOURCE_HALF * 2.5, nodeR * 2.5) : nodeR * 2.5
  const minRFromLeaves = Math.ceil((spacingUnit * leafCount) / (2 * Math.PI))
  const containerR = Math.max(10, Math.min(width.value, height.value) / 2 - CANVAS_PAD)
  const outerR = Math.max(minRFromLeaves, containerR)

  const clusterLayout = d3h.cluster()
    .size([2 * Math.PI, outerR])
    .separation((a, b) => (a.parent === b.parent ? 1 : leafCount < 20 ? 2 : 1.2))

  const layoutRoot = clusterLayout(root)

  // Polar → Cartesian
  layoutRoot.each((node) => {
    node.px = node.y * Math.cos(node.x - Math.PI / 2)
    node.py = node.y * Math.sin(node.x - Math.PI / 2)
  })

  layoutResult = { root: layoutRoot, outerR, nodeR }
}

// ── Zoom behavior (setup + animated fit) ──
function setupZoomAndFit() {
  const svg = svgRef.value
  const g = gRef.value
  if (!svg || !g) return

  const zoom = d3Zoom()
    .scaleExtent([0.1, 6])
    .on('zoom', (event) => {
      selectFn(g).attr('transform', event.transform.toString())
    })

  zoomBehavior = zoom
  selectFn(svg).call(zoom)

  if (layoutResult && width.value > 50 && height.value > 50 && layoutResult.outerR !== lastFitOuterR) {
    lastFitOuterR = layoutResult.outerR
    const fitScale = Math.min(
      0.95,
      Math.max(0.1, (Math.min(width.value, height.value) / 2 - 10) / layoutResult.outerR)
    )
    selectFn(svg).transition().duration(prefersReducedMotion ? 0 : 500).call(
      zoom.transform,
      zoomIdentity.translate(width.value / 2, height.value / 2).scale(fitScale)
    )
  }
}

function teardownZoom() {
  const svg = svgRef.value
  if (svg) selectFn(svg).on('.zoom', null)
}

// ── D3 enter / update / exit render ──
function render() {
  if (!layoutResult || !gRef.value) return
  const { root: layoutNodes, nodeR } = layoutResult
  const reduced = prefersReducedMotion
  const g = selectFn(gRef.value)

  const nodes = layoutNodes.descendants()
  const links = layoutNodes.links()

  // ── Links (bezier curves) ──
  const linkSel = g.selectAll('path.radial-link')
    .data(links, (d) => `${d.source.data.id}--${d.target.data.id}`)

  const isResourceLink = (d) => d.target.data.nodeType === 'resource'

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
  const nodeSel = g.selectAll('g.radial-node')
    .data(nodes, (d) => d.data.id)

  const nodeEnter = nodeSel.enter()
    .append('g')
    .attr('class', 'radial-node')
    .attr('transform', (d) => `translate(${d.px},${d.py}) scale(0)`)
    .attr('cursor', 'pointer')
    .attr('role', 'button')
    .attr('tabindex', '0')
    .attr('aria-label', (d) => `${d.data.nodeType} ${d.data.name}`)
    .on('click', (_event, d) => onNodeClick(d.data._original))
    .on('mouseenter', (event, d) => showTooltip(event, d, tooltipRef.value, containerRef.value))
    .on('mouseleave', () => hideTooltip(tooltipRef.value))
    .on('keydown', (event, d) => {
      if (event.key === 'Enter' || event.key === ' ') onNodeClick(d.data._original)
    })

  // Resource nodes animate faster and with a tighter stagger
  nodeEnter.transition()
    .duration((d) => reduced ? 0 : (d.data.nodeType === 'resource' ? 250 : 500))
    .delay((_d, i) => Math.min(i * (props.showResources ? 8 : 15), 500))
    .attr('transform', (d) => `translate(${d.px},${d.py}) scale(1)`)

  const nodeMerge = nodeEnter.merge(nodeSel)

  nodeMerge.transition().duration(reduced ? 0 : 400)
    .attr('transform', (d) => `translate(${d.px},${d.py}) scale(1)`)
    .attr('opacity', (d) => nodeOpacity(d.data.id, d.data.nodeType, highlightedNodeIds, highlightMode))

  nodeSel.exit()
    .transition().duration(reduced ? 0 : 180)
    .attr('transform', (d) => `translate(${d.px},${d.py}) scale(0)`)
    .remove()

  // Redraw node visuals (clears inner elements then redraws)
  nodeMerge.each(function (d) {
    const el = selectFn(this)
    el.selectAll('*').remove()
    drawNode(el, d, nodeR, selectedNodeId, focusedNodeId, highlightedNodeIds, highlightMode)
  })
}

// Landing page is non-interactive-by-default: click is a no-op
function onNodeClick(_original) { /* no-op on landing */ }

function resetZoom() {
  const svg = svgRef.value
  if (!svg || !zoomBehavior || !layoutResult) return
  const fitScale = Math.min(0.95, Math.max(0.1, (Math.min(width.value, height.value) / 2 - 10) / layoutResult.outerR))
  selectFn(svg).transition().duration(400).call(
    zoomBehavior.transform,
    zoomIdentity.translate(width.value / 2, height.value / 2).scale(fitScale)
  )
}

function zoomIn() {
  const svg = svgRef.value
  if (svg && zoomBehavior) selectFn(svg).transition().duration(200).call(zoomBehavior.scaleBy, 1.3)
}

function zoomOut() {
  const svg = svgRef.value
  if (svg && zoomBehavior) selectFn(svg).transition().duration(200).call(zoomBehavior.scaleBy, 0.77)
}

// Full rebuild + redraw pipeline
function rebuild() {
  computeHierarchy()
  computeLayout()
  teardownZoom()
  setupZoomAndFit()
  render()
}

onMounted(async () => {
  // Dynamic imports keep all D3/DOM strictly client-side (SSR-safe).
  const [hierarchy, shape, selection, zoom] = await Promise.all([
    import('d3-hierarchy'),
    import('d3-shape'),
    import('d3-selection'),
    import('d3-zoom'),
  ])
  await import('d3-transition')

  d3h = hierarchy
  d3Arc = shape.arc
  selectFn = selection.select
  d3Zoom = zoom.zoom
  zoomIdentity = zoom.zoomIdentity

  prefersReducedMotion = typeof window !== 'undefined'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches

  const el = containerRef.value
  if (!el) return

  // Measure the container; only draw once width>0 && height>0. Redraw on resize.
  resizeObserver = new ResizeObserver((entries) => {
    const e = entries[0]
    const w = e.contentRect.width
    const h = e.contentRect.height
    const changed = w !== width.value || h !== height.value
    width.value = w
    height.value = h
    if (w > 0 && h > 0 && changed) rebuild()
  })
  resizeObserver.observe(el)

  width.value = el.clientWidth
  height.value = el.clientHeight
  if (width.value > 0 && height.value > 0) rebuild()
})

onBeforeUnmount(() => {
  if (resizeObserver) resizeObserver.disconnect()
  teardownZoom()
})

// Redraw when data / showResources change (translated from useMemo deps)
watch(() => [props.data, props.showResources], () => {
  if (!d3h || width.value <= 0 || height.value <= 0) return
  rebuild()
})
</script>

<style scoped>
/* Container equivalent to dashboard's "relative w-full h-full" */
.topo-root {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.topo-svg {
  display: block;
  width: 100%;
  height: 100%;
}

/* Tooltip — equivalent to Tailwind:
   pointer-events-none absolute z-50 hidden rounded-lg border bg-popover
   px-3 py-2 text-sm shadow-md max-w-[240px] */
.topo-tooltip {
  pointer-events: none;
  position: absolute;
  z-index: 50;
  display: none;
  border-radius: 0.5rem;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: #16161c;
  color: #ededed;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  line-height: 1.35;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1);
  max-width: 240px;
}

/* Zoom controls — equivalent to:
   absolute bottom-4 right-4 flex flex-col gap-1 */
.topo-zoom-controls {
  position: absolute;
  bottom: 1rem;
  right: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  z-index: 40;
}

/* Buttons — equivalent to:
   w-7 h-7 rounded border bg-background hover:bg-accent flex items-center
   justify-center text-sm cursor-pointer */
.topo-zoom-btn {
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 0.25rem;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.05);
  color: #ededed;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.875rem;
  line-height: 1;
  cursor: pointer;
  padding: 0;
  transition: background 0.15s ease;
}

.topo-zoom-btn:hover {
  background: rgba(255, 255, 255, 0.12);
}

.topo-zoom-btn-fit {
  font-size: 0.625rem;
}
</style>
