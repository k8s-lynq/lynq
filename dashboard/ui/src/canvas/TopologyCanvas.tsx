import { useRef, useEffect, useState, useCallback } from 'react'
import type { TopologyNode, TopologyEdge, ResourceStatus } from '@/types/lynq'

type HighlightMode = 'none' | 'search' | 'problem'

interface TopologyCanvasProps {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
  onNodeClick?: (node: TopologyNode) => void
  onNodeExpand?: (node: TopologyNode) => void
  onNodeHover?: (node: TopologyNode | null) => void
  selectedNodeId?: string | null
  expandedNodes?: Set<string>
  highlightedNodeIds?: Set<string>
  dimNonHighlighted?: boolean
  focusNodeId?: string | null
  highlightMode?: HighlightMode
  className?: string
}

// Chevron click area width for expandable nodes
const CHEVRON_AREA_WIDTH = 28

interface LayoutNode extends TopologyNode {
  x: number
  y: number
  width: number
  height: number
  visible: boolean
}

interface ViewState {
  scale: number
  offsetX: number
  offsetY: number
}

// Shadcn-style color palette - clean and minimal
const STATUS_COLORS: Record<ResourceStatus, { accent: string; bg: string; text: string; border: string }> = {
  ready: {
    accent: '#0d9488',  // teal-600
    bg: '#f0fdfa',      // teal-50
    text: '#0f766e',    // teal-700
    border: '#99f6e4',  // teal-200
  },
  pending: {
    accent: '#d97706',  // amber-600
    bg: '#fffbeb',      // amber-50
    text: '#b45309',    // amber-700
    border: '#fde68a',  // amber-200
  },
  failed: {
    accent: '#dc2626',  // red-600
    bg: '#fef2f2',      // red-50
    text: '#b91c1c',    // red-700
    border: '#fecaca',  // red-200
  },
  skipped: {
    accent: '#64748b',  // slate-500
    bg: '#f8fafc',      // slate-50
    text: '#475569',    // slate-600
    border: '#e2e8f0',  // slate-200
  },
}

// Node dimensions - proper card proportions
const NODE_SIZES = {
  hub: { width: 200, height: 64 },
  form: { width: 200, height: 64 },
  node: { width: 180, height: 48 },  // Changed to card style
  resource: { width: 160, height: 32 },
  orphan: { width: 180, height: 40 },
}

const LEVEL_SPACING = 220
const NODE_SPACING = 100  // Increased for highlight effects
const RESOURCE_SPACING = 40  // Increased for better readability
const RADIUS = 12 // 0.75rem to match theme

export function TopologyCanvas({
  nodes,
  edges,
  onNodeClick,
  onNodeExpand,
  onNodeHover,
  selectedNodeId,
  expandedNodes = new Set(),
  highlightedNodeIds,
  dimNonHighlighted = false,
  focusNodeId,
  highlightMode = 'none',
  className,
}: TopologyCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [viewState, setViewState] = useState<ViewState>({
    scale: 1,
    offsetX: 0,
    offsetY: 0,
  })
  const [isDragging, setIsDragging] = useState(false)
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 })
  const [hoveredNode, setHoveredNode] = useState<LayoutNode | null>(null)
  const [layoutNodes, setLayoutNodes] = useState<LayoutNode[]>([])

  // Calculate layout
  useEffect(() => {
    const layout = calculateLayout(nodes, edges, expandedNodes)
    setLayoutNodes(layout)
  }, [nodes, edges, expandedNodes])

  // Draw canvas
  useEffect(() => {
    const canvas = canvasRef.current
    const ctx = canvas?.getContext('2d')
    if (!canvas || !ctx) return

    // Set canvas size
    const container = containerRef.current
    if (container) {
      const rect = container.getBoundingClientRect()
      canvas.width = rect.width * window.devicePixelRatio
      canvas.height = rect.height * window.devicePixelRatio
      canvas.style.width = `${rect.width}px`
      canvas.style.height = `${rect.height}px`
      ctx.scale(window.devicePixelRatio, window.devicePixelRatio)
    }

    // Clear canvas
    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // Apply view transform
    ctx.save()
    ctx.translate(viewState.offsetX, viewState.offsetY)
    ctx.scale(viewState.scale, viewState.scale)

    // Draw edges first (behind nodes)
    drawEdges(ctx, layoutNodes, edges)

    // Draw orphan section header if there are orphaned nodes
    const orphanNodes = layoutNodes.filter((n) => n.type === 'orphan' && n.visible)
    if (orphanNodes.length > 0) {
      const firstOrphan = orphanNodes[0]
      drawSectionHeader(ctx, 'Orphaned Resources', firstOrphan.x, firstOrphan.y - 32, 'warning')
    }

    // Draw nodes
    const hasHighlights = highlightedNodeIds && highlightedNodeIds.size > 0
    for (const node of layoutNodes) {
      if (!node.visible) continue
      const isHighlighted = hasHighlights ? highlightedNodeIds.has(node.id) : false
      const isDimmed = hasHighlights && dimNonHighlighted && !isHighlighted
      const isExpanded = expandedNodes.has(node.id)
      drawNode(ctx, node, node.id === selectedNodeId, node.id === hoveredNode?.id, isHighlighted, isDimmed, isExpanded, highlightMode)
    }

    ctx.restore()
  }, [layoutNodes, edges, viewState, selectedNodeId, hoveredNode, highlightedNodeIds, dimNonHighlighted, expandedNodes, highlightMode])

  // Wheel handler for zoom - must be attached imperatively with passive: false
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const handleWheel = (e: WheelEvent) => {
      e.preventDefault()
      const delta = e.deltaY > 0 ? 0.9 : 1.1

      const rect = canvas.getBoundingClientRect()
      const mouseX = e.clientX - rect.left
      const mouseY = e.clientY - rect.top

      setViewState((prev) => {
        const newScale = Math.min(Math.max(prev.scale * delta, 0.1), 3)
        return {
          scale: newScale,
          offsetX: mouseX - (mouseX - prev.offsetX) * (newScale / prev.scale),
          offsetY: mouseY - (mouseY - prev.offsetY) * (newScale / prev.scale),
        }
      })
    }

    canvas.addEventListener('wheel', handleWheel, { passive: false })
    return () => canvas.removeEventListener('wheel', handleWheel)
  }, [])

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (e.button === 0) {
      setIsDragging(true)
      setDragStart({ x: e.clientX - viewState.offsetX, y: e.clientY - viewState.offsetY })
    }
  }, [viewState.offsetX, viewState.offsetY])

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (isDragging) {
      setViewState((prev) => ({
        ...prev,
        offsetX: e.clientX - dragStart.x,
        offsetY: e.clientY - dragStart.y,
      }))
    } else {
      // Check hover
      const rect = canvasRef.current?.getBoundingClientRect()
      if (rect) {
        const x = (e.clientX - rect.left - viewState.offsetX) / viewState.scale
        const y = (e.clientY - rect.top - viewState.offsetY) / viewState.scale
        const node = findNodeAtPosition(layoutNodes, x, y)
        setHoveredNode(node)
        onNodeHover?.(node)

        if (canvasRef.current) {
          canvasRef.current.style.cursor = node ? 'pointer' : 'grab'
        }
      }
    }
  }, [isDragging, dragStart, viewState, layoutNodes, onNodeHover])

  const handleMouseUp = useCallback(() => {
    setIsDragging(false)
  }, [])

  const handleClick = useCallback((e: React.MouseEvent) => {
    const rect = canvasRef.current?.getBoundingClientRect()
    if (rect) {
      const x = (e.clientX - rect.left - viewState.offsetX) / viewState.scale
      const y = (e.clientY - rect.top - viewState.offsetY) / viewState.scale
      const node = findNodeAtPosition(layoutNodes, x, y)
      if (node) {
        // For expandable nodes (Hub/Form/Node), check if click is on chevron area
        if ((node.type === 'hub' || node.type === 'form' || node.type === 'node') && onNodeExpand) {
          const clickXRelative = x - node.x
          if (clickXRelative < CHEVRON_AREA_WIDTH) {
            // Clicked on chevron - toggle expand
            onNodeExpand(node)
            return
          }
        }
        // Clicked on card body or non-expandable node - open drawer
        onNodeClick?.(node)
      }
    }
  }, [viewState, layoutNodes, onNodeClick, onNodeExpand])

  // Center view on initial load only (when node count changes)
  useEffect(() => {
    const container = containerRef.current
    if (container && layoutNodes.length > 0) {
      const rect = container.getBoundingClientRect()
      const bounds = getNodesBounds(layoutNodes)

      const padding = 60
      const contentWidth = bounds.maxX - bounds.minX + padding * 2
      const contentHeight = bounds.maxY - bounds.minY + padding * 2

      const scaleX = rect.width / contentWidth
      const scaleY = rect.height / contentHeight
      const scale = Math.min(scaleX, scaleY, 1)

      const centerX = (bounds.minX + bounds.maxX) / 2
      const centerY = (bounds.minY + bounds.maxY) / 2

      setViewState({
        scale,
        offsetX: rect.width / 2 - centerX * scale,
        offsetY: rect.height / 2 - centerY * scale,
      })
    }
    // Only recenter when node count changes, not when positions update
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layoutNodes.length])

  // Center view on focused node
  useEffect(() => {
    if (!focusNodeId) return
    const container = containerRef.current
    const node = layoutNodes.find((n) => n.id === focusNodeId && n.visible)
    if (!container || !node) return

    const rect = container.getBoundingClientRect()
    const nodeCenterX = node.x + node.width / 2
    const nodeCenterY = node.y + node.height / 2

    // Animate to a comfortable zoom level and center on the node
    const targetScale = Math.max(viewState.scale, 1)
    setViewState({
      scale: targetScale,
      offsetX: rect.width / 2 - nodeCenterX * targetScale,
      offsetY: rect.height / 2 - nodeCenterY * targetScale,
    })
  }, [focusNodeId, layoutNodes])

  return (
    <div ref={containerRef} className={`relative w-full h-full ${className || ''}`}>
      <canvas
        ref={canvasRef}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onClick={handleClick}
        className="w-full h-full"
      />
      {/* Zoom controls */}
      <div className="absolute bottom-4 right-4 flex gap-1.5">
        <button
          onClick={() => setViewState((v) => ({ ...v, scale: Math.min(v.scale * 1.2, 3) }))}
          className="w-8 h-8 rounded-lg bg-card border border-border flex items-center justify-center hover:bg-accent text-sm font-medium transition-colors"
        >
          +
        </button>
        <button
          onClick={() => setViewState((v) => ({ ...v, scale: Math.max(v.scale * 0.8, 0.1) }))}
          className="w-8 h-8 rounded-lg bg-card border border-border flex items-center justify-center hover:bg-accent text-sm font-medium transition-colors"
        >
          -
        </button>
        <span className="px-3 py-1.5 rounded-lg bg-card border border-border text-xs font-mono text-muted-foreground">
          {Math.round(viewState.scale * 100)}%
        </span>
      </div>
    </div>
  )
}

// Layout calculation
function calculateLayout(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
  expandedNodes: Set<string>
): LayoutNode[] {
  const layoutNodes: LayoutNode[] = []

  // Group nodes by type
  const hubs = nodes.filter((n) => n.type === 'hub')
  const forms = nodes.filter((n) => n.type === 'form')
  const lynqNodes = nodes.filter((n) => n.type === 'node')
  const resources = nodes.filter((n) => n.type === 'resource')
  const orphans = nodes.filter((n) => n.type === 'orphan')

  // Create edge map for finding parents
  const parentMap = new Map<string, string>()
  for (const edge of edges) {
    parentMap.set(edge.target, edge.source)
  }

  // Group forms by their parent hub
  const formsByHub = new Map<string, TopologyNode[]>()
  for (const form of forms) {
    const parentId = parentMap.get(form.id) || '_orphan'
    if (!formsByHub.has(parentId)) {
      formsByHub.set(parentId, [])
    }
    formsByHub.get(parentId)!.push(form)
  }

  // Group nodes by their parent form
  const nodesByForm = new Map<string, TopologyNode[]>()
  for (const node of lynqNodes) {
    const parentId = parentMap.get(node.id) || '_orphan'
    if (!nodesByForm.has(parentId)) {
      nodesByForm.set(parentId, [])
    }
    nodesByForm.get(parentId)!.push(node)
  }

  // Group resources by their parent node
  const resourcesByNode = new Map<string, TopologyNode[]>()
  for (const resource of resources) {
    const parentId = parentMap.get(resource.id) || '_orphan'
    if (!resourcesByNode.has(parentId)) {
      resourcesByNode.set(parentId, [])
    }
    resourcesByNode.get(parentId)!.push(resource)
  }

  let currentY = 60

  // Layout hubs and their children hierarchically
  for (const hub of hubs) {
    const hubY = currentY
    const hubId = hub.id
    const isHubExpanded = expandedNodes.has(hubId)

    // Add hub node
    layoutNodes.push({
      ...hub,
      x: 80,
      y: hubY,
      width: NODE_SIZES.hub.width,
      height: NODE_SIZES.hub.height,
      visible: true,
    })

    // Get forms for this hub
    const hubForms = formsByHub.get(hubId) || []
    let formStartY = hubY

    if (isHubExpanded && hubForms.length > 0) {
      // Layout forms for this hub
      for (const form of hubForms) {
        const formId = form.id
        const isFormExpanded = expandedNodes.has(formId)

        layoutNodes.push({
          ...form,
          x: 80 + LEVEL_SPACING,
          y: formStartY,
          width: NODE_SIZES.form.width,
          height: NODE_SIZES.form.height,
          visible: true,
        })

        // Get nodes for this form
        const formNodes = nodesByForm.get(formId) || []

        if (isFormExpanded && formNodes.length > 0) {
          // Layout nodes for this form
          let nodeY = formStartY
          for (const lynqNode of formNodes) {
            const nodeId = lynqNode.id
            const isNodeExpanded = expandedNodes.has(nodeId)

            layoutNodes.push({
              ...lynqNode,
              x: 80 + LEVEL_SPACING * 2,
              y: nodeY,
              width: NODE_SIZES.node.width,
              height: NODE_SIZES.node.height,
              visible: true,
            })

            // Get resources for this node
            const nodeResources = resourcesByNode.get(nodeId) || []

            if (isNodeExpanded && nodeResources.length > 0) {
              // Layout resources for this node
              let resourceY = nodeY
              for (const resource of nodeResources) {
                layoutNodes.push({
                  ...resource,
                  x: 80 + LEVEL_SPACING * 3,
                  y: resourceY,
                  width: NODE_SIZES.resource.width,
                  height: NODE_SIZES.resource.height,
                  visible: true,
                })
                resourceY += RESOURCE_SPACING
              }
              nodeY = Math.max(nodeY + NODE_SPACING * 0.7, resourceY)
            } else {
              // Node collapsed - add resources as hidden
              for (const resource of nodeResources) {
                layoutNodes.push({
                  ...resource,
                  x: 80 + LEVEL_SPACING * 3,
                  y: nodeY,
                  width: NODE_SIZES.resource.width,
                  height: NODE_SIZES.resource.height,
                  visible: false,
                })
              }
              nodeY += NODE_SPACING * 0.7
            }
          }
          formStartY = Math.max(formStartY + NODE_SPACING, nodeY)
        } else {
          // Form collapsed - add nodes and resources as hidden
          for (const lynqNode of formNodes) {
            layoutNodes.push({
              ...lynqNode,
              x: 80 + LEVEL_SPACING * 2,
              y: formStartY,
              width: NODE_SIZES.node.width,
              height: NODE_SIZES.node.height,
              visible: false,
            })
            // Add hidden resources
            const nodeResources = resourcesByNode.get(lynqNode.id) || []
            for (const resource of nodeResources) {
              layoutNodes.push({
                ...resource,
                x: 80 + LEVEL_SPACING * 3,
                y: formStartY,
                width: NODE_SIZES.resource.width,
                height: NODE_SIZES.resource.height,
                visible: false,
              })
            }
          }
          formStartY += NODE_SPACING
        }
      }
      currentY = Math.max(currentY + NODE_SPACING, formStartY)
    } else {
      // Hub collapsed - add forms, nodes, and resources as hidden
      for (const form of hubForms) {
        layoutNodes.push({
          ...form,
          x: 80 + LEVEL_SPACING,
          y: hubY,
          width: NODE_SIZES.form.width,
          height: NODE_SIZES.form.height,
          visible: false,
        })
        // Add hidden nodes and resources
        const formNodes = nodesByForm.get(form.id) || []
        for (const lynqNode of formNodes) {
          layoutNodes.push({
            ...lynqNode,
            x: 80 + LEVEL_SPACING * 2,
            y: hubY,
            width: NODE_SIZES.node.width,
            height: NODE_SIZES.node.height,
            visible: false,
          })
          // Add hidden resources
          const nodeResources = resourcesByNode.get(lynqNode.id) || []
          for (const resource of nodeResources) {
            layoutNodes.push({
              ...resource,
              x: 80 + LEVEL_SPACING * 3,
              y: hubY,
              width: NODE_SIZES.resource.width,
              height: NODE_SIZES.resource.height,
              visible: false,
            })
          }
        }
      }
      currentY += NODE_SPACING
    }
  }

  // Layout orphaned resources in a separate section at the right
  if (orphans.length > 0) {
    const orphanStartX = 80 + LEVEL_SPACING * 4 + 80
    let orphanY = 60

    for (const orphan of orphans) {
      layoutNodes.push({
        ...orphan,
        x: orphanStartX,
        y: orphanY,
        width: NODE_SIZES.orphan.width,
        height: NODE_SIZES.orphan.height,
        visible: true,
      })
      orphanY += NODE_SIZES.orphan.height + 12
    }
  }

  return layoutNodes
}

// Section header drawing
function drawSectionHeader(
  ctx: CanvasRenderingContext2D,
  text: string,
  x: number,
  y: number,
  variant: 'default' | 'warning' = 'default'
) {
  ctx.save()
  ctx.font = "600 11px 'JetBrains Mono', monospace"
  const textWidth = ctx.measureText(text).width

  // Background pill
  ctx.beginPath()
  ctx.roundRect(x - 10, y - 10, textWidth + 20, 24, 8)
  ctx.fillStyle = variant === 'warning' ? '#fef3c7' : '#f0fdfa'
  ctx.fill()
  ctx.strokeStyle = variant === 'warning' ? '#fcd34d' : '#5eead4'
  ctx.lineWidth = 1
  ctx.stroke()

  // Text
  ctx.fillStyle = variant === 'warning' ? '#92400e' : '#0f766e'
  ctx.textAlign = 'left'
  ctx.textBaseline = 'middle'
  ctx.fillText(text, x, y + 2)
  ctx.restore()
}

// Drawing functions
function drawNode(
  ctx: CanvasRenderingContext2D,
  node: LayoutNode,
  selected: boolean,
  hovered: boolean,
  highlighted: boolean = false,
  dimmed: boolean = false,
  expanded: boolean = false,
  highlightMode: HighlightMode = 'none'
) {
  const colors = STATUS_COLORS[node.status]

  ctx.save()

  // Apply dimming for non-highlighted nodes
  if (dimmed) {
    ctx.globalAlpha = 0.25
  }

  // Enhanced shadow for highlighted nodes
  if (highlighted) {
    if (highlightMode === 'problem') {
      // Red glow for problem mode
      ctx.shadowColor = 'rgba(220, 38, 38, 0.6)' // red-600 glow
      ctx.shadowBlur = 20
    } else {
      // Teal glow for search mode
      ctx.shadowColor = 'rgba(13, 148, 136, 0.5)' // teal glow
      ctx.shadowBlur = 16
    }
    ctx.shadowOffsetY = 0
  } else {
    ctx.shadowColor = 'rgba(0, 0, 0, 0.08)'
    ctx.shadowBlur = selected ? 12 : hovered ? 8 : 4
    ctx.shadowOffsetY = selected ? 4 : 2
  }

  switch (node.type) {
    case 'hub':
      drawCardNode(ctx, node, colors, 'Hub', selected || highlighted, hovered, expanded)
      break
    case 'form':
      drawCardNode(ctx, node, colors, 'Form', selected || highlighted, hovered, expanded)
      break
    case 'node':
      drawLynqNode(ctx, node, colors, selected || highlighted, hovered, expanded)
      break
    case 'resource':
      drawResourceNode(ctx, node, colors, selected || highlighted, hovered)
      break
    case 'orphan':
      drawOrphanNode(ctx, node, selected || highlighted, hovered)
      break
  }

  // Draw highlight ring for highlighted nodes
  if (highlighted && !dimmed) {
    ctx.globalAlpha = 1
    ctx.shadowBlur = 0
    // Use red for problem mode, teal for search mode
    ctx.strokeStyle = highlightMode === 'problem' ? '#dc2626' : '#0d9488'
    ctx.lineWidth = highlightMode === 'problem' ? 3.5 : 3
    ctx.setLineDash(highlightMode === 'problem' ? [8, 4] : [6, 4])

    // All nodes are now cards, use rounded rect for highlight
    const radius = node.type === 'resource' ? 8 : RADIUS
    ctx.beginPath()
    ctx.roundRect(node.x - 4, node.y - 4, node.width + 8, node.height + 8, radius + 2)
    ctx.stroke()
    ctx.setLineDash([])
  }

  ctx.restore()
}

// Card-style node (Hub, Form)
function drawCardNode(
  ctx: CanvasRenderingContext2D,
  node: LayoutNode,
  colors: { accent: string; bg: string; text: string; border: string },
  label: string,
  selected: boolean,
  hovered: boolean,
  expanded: boolean = false
) {
  const { x, y, width, height } = node
  const chevronWidth = CHEVRON_AREA_WIDTH

  // Main card background
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, RADIUS)
  ctx.fillStyle = '#ffffff'
  ctx.fill()

  // Border
  ctx.strokeStyle = selected ? colors.accent : hovered ? colors.border : '#e2e8f0'
  ctx.lineWidth = selected ? 2 : 1
  ctx.stroke()

  // Reset shadow for text
  ctx.shadowBlur = 0
  ctx.shadowOffsetY = 0

  // Chevron area background (subtle hover hint)
  ctx.save()
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, RADIUS)
  ctx.clip()

  // Chevron area with accent color
  ctx.fillStyle = colors.bg
  ctx.fillRect(x, y, chevronWidth, height)

  // Separator line between chevron and content
  ctx.strokeStyle = colors.border
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(x + chevronWidth, y + 8)
  ctx.lineTo(x + chevronWidth, y + height - 8)
  ctx.stroke()
  ctx.restore()

  // Draw chevron icon (▶ or ▼)
  const chevronX = x + chevronWidth / 2
  const chevronY = y + height / 2
  const chevronSize = 5

  ctx.fillStyle = colors.text
  ctx.beginPath()
  if (expanded) {
    // Down chevron (▼)
    ctx.moveTo(chevronX - chevronSize, chevronY - chevronSize / 2)
    ctx.lineTo(chevronX + chevronSize, chevronY - chevronSize / 2)
    ctx.lineTo(chevronX, chevronY + chevronSize / 2)
  } else {
    // Right chevron (▶)
    ctx.moveTo(chevronX - chevronSize / 2, chevronY - chevronSize)
    ctx.lineTo(chevronX + chevronSize / 2, chevronY)
    ctx.lineTo(chevronX - chevronSize / 2, chevronY + chevronSize)
  }
  ctx.closePath()
  ctx.fill()

  // Content area starts after chevron
  const contentX = x + chevronWidth + 8

  // Type badge
  ctx.font = "600 9px 'JetBrains Mono', monospace"
  const labelText = label.toUpperCase()
  const labelWidth = ctx.measureText(labelText).width

  ctx.beginPath()
  ctx.roundRect(contentX, y + 10, labelWidth + 12, 18, 4)
  ctx.fillStyle = colors.bg
  ctx.fill()

  ctx.fillStyle = colors.text
  ctx.textAlign = 'left'
  ctx.textBaseline = 'middle'
  ctx.fillText(labelText, contentX + 6, y + 19)

  // Name
  ctx.fillStyle = '#0f172a'
  ctx.font = "600 12px 'JetBrains Mono', monospace"
  const maxNameWidth = node.metrics ? width - chevronWidth - 70 : width - chevronWidth - 16
  const displayName = truncateText(ctx, node.name, maxNameWidth)
  ctx.fillText(displayName, contentX, y + height - 18)

  // Metrics badge (right side, vertically centered)
  if (node.metrics) {
    const metricsText = `${node.metrics.ready}/${node.metrics.desired}`
    ctx.font = "600 10px 'JetBrains Mono', monospace"
    const metricsWidth = ctx.measureText(metricsText).width

    ctx.beginPath()
    ctx.roundRect(x + width - metricsWidth - 20, y + height - 28, metricsWidth + 12, 20, 4)
    ctx.fillStyle = colors.bg
    ctx.fill()

    ctx.fillStyle = colors.text
    ctx.textAlign = 'right'
    ctx.fillText(metricsText, x + width - 14, y + height - 17)
  }
}

// LynqNode - compact card with chevron for expandable resources
function drawLynqNode(
  ctx: CanvasRenderingContext2D,
  node: LayoutNode,
  colors: { accent: string; bg: string; text: string; border: string },
  selected: boolean,
  hovered: boolean,
  expanded: boolean = false
) {
  const { x, y, width, height } = node
  const chevronWidth = CHEVRON_AREA_WIDTH

  // Main card background
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, 8)
  ctx.fillStyle = '#ffffff'
  ctx.fill()

  // Border
  ctx.strokeStyle = selected ? colors.accent : hovered ? colors.border : '#e2e8f0'
  ctx.lineWidth = selected ? 2 : 1
  ctx.stroke()

  // Reset shadow for content
  ctx.shadowBlur = 0
  ctx.shadowOffsetY = 0

  // Chevron area background
  ctx.save()
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, 8)
  ctx.clip()
  ctx.fillStyle = colors.bg
  ctx.fillRect(x, y, chevronWidth, height)

  // Separator line
  ctx.strokeStyle = colors.border
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(x + chevronWidth, y + 6)
  ctx.lineTo(x + chevronWidth, y + height - 6)
  ctx.stroke()
  ctx.restore()

  // Draw chevron icon
  const chevronX = x + chevronWidth / 2
  const chevronY = y + height / 2
  const chevronSize = 4

  ctx.fillStyle = colors.text
  ctx.beginPath()
  if (expanded) {
    // Down chevron (▼)
    ctx.moveTo(chevronX - chevronSize, chevronY - chevronSize / 2)
    ctx.lineTo(chevronX + chevronSize, chevronY - chevronSize / 2)
    ctx.lineTo(chevronX, chevronY + chevronSize / 2)
  } else {
    // Right chevron (▶)
    ctx.moveTo(chevronX - chevronSize / 2, chevronY - chevronSize)
    ctx.lineTo(chevronX + chevronSize / 2, chevronY)
    ctx.lineTo(chevronX - chevronSize / 2, chevronY + chevronSize)
  }
  ctx.closePath()
  ctx.fill()

  // Status indicator dot
  ctx.beginPath()
  ctx.arc(x + chevronWidth + 12, y + height / 2, 5, 0, Math.PI * 2)
  ctx.fillStyle = colors.accent
  ctx.fill()

  // Name
  ctx.fillStyle = '#0f172a'
  ctx.font = "600 11px 'JetBrains Mono', monospace"
  ctx.textAlign = 'left'
  ctx.textBaseline = 'middle'
  const displayName = truncateText(ctx, node.name, width - chevronWidth - 32)
  ctx.fillText(displayName, x + chevronWidth + 24, y + height / 2)
}

// Resource node - simple compact style
function drawResourceNode(
  ctx: CanvasRenderingContext2D,
  node: LayoutNode,
  colors: { accent: string; bg: string; text: string; border: string },
  selected: boolean,
  hovered: boolean
) {
  const { x, y, width, height } = node

  // Background
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, 6)
  ctx.fillStyle = '#ffffff'
  ctx.fill()

  // Border
  ctx.strokeStyle = selected ? colors.accent : hovered ? colors.border : '#e2e8f0'
  ctx.lineWidth = selected ? 1.5 : 1
  ctx.stroke()

  // Left accent dot
  ctx.beginPath()
  ctx.arc(x + 10, y + height / 2, 3, 0, Math.PI * 2)
  ctx.fillStyle = colors.accent
  ctx.fill()

  // Reset shadow
  ctx.shadowBlur = 0
  ctx.shadowOffsetY = 0

  // Name
  ctx.fillStyle = '#475569'
  ctx.font = "500 10px 'JetBrains Mono', monospace"
  ctx.textAlign = 'left'
  ctx.textBaseline = 'middle'
  const displayName = truncateText(ctx, node.name, width - 24)
  ctx.fillText(displayName, x + 20, y + height / 2)
}

// Orphan node with dashed border
function drawOrphanNode(
  ctx: CanvasRenderingContext2D,
  node: LayoutNode,
  selected: boolean,
  hovered: boolean
) {
  const { x, y, width, height } = node
  const colors = STATUS_COLORS.pending

  // Background
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, 8)
  ctx.fillStyle = '#fffbeb'
  ctx.fill()

  // Dashed border
  ctx.setLineDash([6, 4])
  ctx.strokeStyle = selected ? '#d97706' : hovered ? '#fcd34d' : '#fde68a'
  ctx.lineWidth = selected ? 2 : 1
  ctx.stroke()
  ctx.setLineDash([])

  // Reset shadow
  ctx.shadowBlur = 0
  ctx.shadowOffsetY = 0

  // Warning icon (triangle)
  ctx.fillStyle = '#d97706'
  ctx.font = "11px 'JetBrains Mono', monospace"
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText('⚠', x + 14, y + height / 2)

  // Name
  ctx.fillStyle = colors.text
  ctx.font = "500 10px 'JetBrains Mono', monospace"
  ctx.textAlign = 'left'
  const displayName = truncateText(ctx, node.name, width - 36)
  ctx.fillText(displayName, x + 28, y + height / 2)
}

// Clean edge drawing
function drawEdges(ctx: CanvasRenderingContext2D, nodes: LayoutNode[], edges: TopologyEdge[]) {
  const nodeMap = new Map(nodes.map((n) => [n.id, n]))

  ctx.save()

  for (const edge of edges) {
    const source = nodeMap.get(edge.source)
    const target = nodeMap.get(edge.target)

    if (!source || !target || !source.visible || !target.visible) continue

    const startX = source.x + source.width
    const startY = source.y + source.height / 2
    const endX = target.x
    const endY = target.y + target.height / 2

    // Draw smooth bezier curve
    ctx.beginPath()
    ctx.moveTo(startX, startY)
    const controlX1 = startX + (endX - startX) * 0.5
    const controlX2 = startX + (endX - startX) * 0.5
    ctx.bezierCurveTo(controlX1, startY, controlX2, endY, endX, endY)
    ctx.strokeStyle = '#d1d5db' // gray-300
    ctx.lineWidth = 1.5
    ctx.lineCap = 'round'
    ctx.stroke()

    // Draw small circle at the end instead of arrow
    ctx.beginPath()
    ctx.arc(endX - 2, endY, 3, 0, Math.PI * 2)
    ctx.fillStyle = '#9ca3af' // gray-400
    ctx.fill()
  }

  ctx.restore()
}

// Utility functions
function findNodeAtPosition(nodes: LayoutNode[], x: number, y: number): LayoutNode | null {
  // Check in reverse order (top nodes drawn last)
  for (let i = nodes.length - 1; i >= 0; i--) {
    const node = nodes[i]
    if (!node.visible) continue

    // All nodes are now rectangles/cards
    if (x >= node.x && x <= node.x + node.width &&
        y >= node.y && y <= node.y + node.height) {
      return node
    }
  }
  return null
}

function getNodesBounds(nodes: LayoutNode[]) {
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity

  for (const node of nodes) {
    if (!node.visible) continue
    minX = Math.min(minX, node.x)
    minY = Math.min(minY, node.y)
    maxX = Math.max(maxX, node.x + node.width)
    maxY = Math.max(maxY, node.y + node.height)
  }

  return { minX, minY, maxX, maxY }
}

function truncateText(ctx: CanvasRenderingContext2D, text: string, maxWidth: number): string {
  const metrics = ctx.measureText(text)
  if (metrics.width <= maxWidth) return text

  let truncated = text
  while (ctx.measureText(truncated + '…').width > maxWidth && truncated.length > 0) {
    truncated = truncated.slice(0, -1)
  }
  return truncated + '…'
}
