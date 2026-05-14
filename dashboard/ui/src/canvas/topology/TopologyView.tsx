import { useEffect, useMemo, useRef, useState } from 'react'
import type { TopologyData, TopologyNode } from '@/types/lynq'
import { RadialClusterView } from './RadialClusterView'
import { FilterChips } from './controls/FilterChips'
import { OrphanTrigger, OrphanPanelDrawer } from './controls/OrphanPanel'
import { usePersistedView } from './hooks/usePersistedView'
import type { TopologyFilters } from './types'

interface TopologyViewProps {
  data: TopologyData
  loading?: boolean
  searchQuery?: string
  highlightedNodeIds?: Set<string>
  highlightMode?: 'none' | 'search' | 'problem'
  problemMode?: boolean
  problemCount?: number
  selectedNodeId?: string | null
  focusedNodeId?: string | null
  onNodeClick: (node: TopologyNode) => void
}

export function TopologyView({
  data,
  loading,
  searchQuery: _searchQuery,
  highlightedNodeIds,
  highlightMode,
  problemMode,
  problemCount = 0,
  selectedNodeId,
  focusedNodeId,
  onNodeClick,
}: TopologyViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  // Start at 0×0; RadialClusterView is only mounted after ResizeObserver gives
  // the real dimensions, so the first zoom fit always uses the correct center.
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 })
  const { view, setFilters, resetFilters } = usePersistedView()
  const [orphanPanelOpen, setOrphanPanelOpen] = useState(false)

  // Observe container size
  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const e = entries[0]
      setContainerSize({ width: e.contentRect.width, height: e.contentRect.height })
    })
    ro.observe(el)
    setContainerSize({ width: el.clientWidth, height: el.clientHeight })
    return () => ro.disconnect()
  }, [])

  const formNodes = useMemo(() => data.nodes.filter((n) => n.type === 'form'), [data])
  const orphanNodes = useMemo(() => data.nodes.filter((n) => n.type === 'orphan'), [data])
  const namespaces = useMemo(
    () => [...new Set(data.nodes.map((n) => n.namespace).filter(Boolean))],
    [data]
  )

  // Apply filters: filter the data passed to the radial view.
  // When showResources is on, resource nodes whose parent LynqNode passed the
  // filter are also included — buildHierarchy handles the rest.
  const filteredData = useMemo((): TopologyData => {
    const f = view.filters
    if (f.status === 'all' && !f.namespace && !f.formId) return data

    const nodeMap = new Map(data.nodes.map(n => [n.id, n]))
    const allowed = new Set<string>()

    for (const n of data.nodes) {
      if (n.type === 'hub') { allowed.add(n.id); continue }
      if (n.type === 'resource') continue  // resolved after main loop
      if (n.type === 'orphan') continue
      if (f.namespace && n.namespace !== f.namespace) continue
      if (f.formId && n.type === 'form' && n.id !== f.formId) continue
      if (f.formId && n.type === 'node') {
        // Resolve parent form via edge first, fall back to form.children membership
        const parentEdge = data.edges.find((e) => e.target === n.id)
        if (parentEdge) {
          if (parentEdge.source !== f.formId) continue
        } else {
          const parentForm = data.nodes.find(
            (m) => m.type === 'form' && (m.children ?? []).includes(n.id)
          )
          if (parentForm && parentForm.id !== f.formId) continue
        }
      }
      if (f.status !== 'all' && n.type === 'node' && n.status !== f.status) continue
      allowed.add(n.id)
    }

    // Always include forms that still have children after filter
    for (const n of data.nodes) {
      if (n.type === 'form') {
        const hasChild = (n.children ?? []).some((id) => allowed.has(id))
        if (hasChild || (f.formId === n.id)) allowed.add(n.id)
      }
    }

    // When resources are shown, include resource nodes whose parent passed the filter
    if (f.showResources) {
      for (const edge of data.edges) {
        const target = nodeMap.get(edge.target)
        if (target?.type === 'resource' && allowed.has(edge.source)) {
          allowed.add(edge.target)
        }
      }
    }

    return {
      nodes: data.nodes.filter((n) => allowed.has(n.id)),
      edges: data.edges.filter((e) => allowed.has(e.source) && allowed.has(e.target)),
    }
  }, [data, view.filters])

  const isStale = loading && data.nodes.length > 0

  return (
    <div ref={containerRef} className="relative w-full h-full overflow-hidden">
      {/* Toolbar */}
      <div className="absolute top-3 left-3 right-3 z-20 flex items-center justify-between gap-2 flex-wrap pointer-events-none">
        {/* Left: filters */}
        <div className="flex items-center gap-2 pointer-events-auto">
          <FilterChips
            filters={view.filters}
            formNodes={formNodes}
            namespaces={namespaces}
            onChange={(f: TopologyFilters) => setFilters(f)}
            onReset={resetFilters}
          />
        </div>

        {/* Right: status badges + orphan */}
        <div className="flex items-center gap-2 pointer-events-auto">
          {problemMode && problemCount > 0 && (
            <span className="inline-flex items-center gap-1 text-xs bg-destructive text-destructive-foreground px-2 py-1 rounded-full animate-pulse">
              ⚠ {problemCount} problem{problemCount !== 1 ? 's' : ''}
            </span>
          )}
          {problemMode && problemCount === 0 && (
            <span className="inline-flex items-center gap-1 text-xs bg-secondary text-secondary-foreground px-2 py-1 rounded-full">
              ✓ No problems
            </span>
          )}
          {isStale && !problemMode && (
            <span className="text-xs text-muted-foreground bg-background/80 px-2 py-1 rounded border">
              Refreshing…
            </span>
          )}
          <OrphanTrigger orphans={orphanNodes} onOpen={() => setOrphanPanelOpen(true)} />
        </div>
      </div>

      {/* Radial cluster — only mount once the container has real dimensions */}
      {containerSize.width > 0 && containerSize.height > 0 && (
        <RadialClusterView
          data={filteredData}
          width={containerSize.width}
          height={containerSize.height}
          highlightedNodeIds={highlightedNodeIds}
          highlightMode={highlightMode}
          onNodeClick={onNodeClick}
          selectedNodeId={selectedNodeId}
          focusedNodeId={focusedNodeId}
          showResources={view.filters.showResources}
        />
      )}

      {/* Orphan panel */}
      {orphanPanelOpen && (
        <OrphanPanelDrawer
          orphans={orphanNodes}
          onNodeClick={onNodeClick}
          onClose={() => setOrphanPanelOpen(false)}
        />
      )}
    </div>
  )
}
