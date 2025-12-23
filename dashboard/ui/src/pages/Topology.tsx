import { useState, useCallback, useMemo, useRef, useEffect } from 'react'
import type { KeyboardEvent as ReactKeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { TopologyCanvas } from '@/canvas/TopologyCanvas'
import { NodeDetailDrawer } from '@/components/NodeDetailDrawer'
import { useTopology } from '@/hooks/useTopology'
import type { TopologyNode } from '@/types/lynq'
import {
  IconSitemap,
  IconRefresh,
  IconMaximize,
  IconMinimize,
  IconLoader2,
  IconAlertCircle,
  IconBox,
  IconStack2,
  IconActivity,
  IconSearch,
  IconX,
  IconClock,
  IconAlertTriangle,
} from '@tabler/icons-react'

export function Topology() {
  const { t } = useTranslation()
  const [pollInterval, setPollInterval] = useState(30000)
  const { data, loading, error, refetch, hubNodes, formNodes, lynqNodes } = useTopology({
    pollInterval,
  })

  const [selectedNode, setSelectedNode] = useState<TopologyNode | null>(null)
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set())
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [isSearchFocused, setIsSearchFocused] = useState(false)
  const [focusedMatchIndex, setFocusedMatchIndex] = useState(0)
  const [problemMode, setProblemMode] = useState(false)
  const searchInputRef = useRef<HTMLInputElement>(null)

  // Build parent map from edges for expanding tree
  const parentMap = useMemo(() => {
    if (!data) return new Map<string, string>()
    const map = new Map<string, string>()
    for (const edge of data.edges) {
      map.set(edge.target, edge.source)
    }
    return map
  }, [data])

  // Calculate problem nodes (failed status)
  const { problemNodeIds, problemCount } = useMemo(() => {
    if (!data) return { problemNodeIds: new Set<string>(), problemCount: 0 }

    const problemIds = new Set<string>()
    for (const node of data.nodes) {
      if (node.status === 'failed') {
        problemIds.add(node.id)
      }
    }
    return { problemNodeIds: problemIds, problemCount: problemIds.size }
  }, [data])

  // Calculate highlighted nodes based on search query
  const { searchHighlightedIds, matchedNodeIds } = useMemo(() => {
    if (!searchQuery.trim() || !data) {
      return { searchHighlightedIds: new Set<string>(), matchedNodeIds: [] as string[] }
    }

    const query = searchQuery.toLowerCase().trim()
    const matchedIds = new Set<string>()
    const matchedIdsArray: string[] = []

    for (const node of data.nodes) {
      // Search in node name
      if (node.name.toLowerCase().includes(query)) {
        matchedIds.add(node.id)
        matchedIdsArray.push(node.id)
      }
      // Search in node type (but avoid duplicates)
      else if (node.type.toLowerCase().includes(query)) {
        matchedIds.add(node.id)
        matchedIdsArray.push(node.id)
      }
    }

    return { searchHighlightedIds: matchedIds, matchedNodeIds: matchedIdsArray }
  }, [searchQuery, data])

  // Determine which nodes to highlight and mode
  const { highlightedNodeIds, dimNonHighlighted, highlightMode } = useMemo(() => {
    // Problem mode takes precedence when active and there are problems
    if (problemMode && problemCount > 0) {
      return {
        highlightedNodeIds: problemNodeIds,
        dimNonHighlighted: true,
        highlightMode: 'problem' as const,
      }
    }
    // Search mode when there's a search query
    if (searchQuery.trim() && searchHighlightedIds.size > 0) {
      return {
        highlightedNodeIds: searchHighlightedIds,
        dimNonHighlighted: true,
        highlightMode: 'search' as const,
      }
    }
    // No highlighting
    return {
      highlightedNodeIds: new Set<string>(),
      dimNonHighlighted: false,
      highlightMode: 'none' as const,
    }
  }, [problemMode, problemCount, problemNodeIds, searchQuery, searchHighlightedIds])

  // Get ancestor nodes that need to be expanded
  const getAncestors = useCallback((nodeId: string): string[] => {
    const ancestors: string[] = []
    let currentId = nodeId
    while (parentMap.has(currentId)) {
      const parentId = parentMap.get(currentId)!
      ancestors.push(parentId)
      currentId = parentId
    }
    return ancestors
  }, [parentMap])

  // Expand ancestors of focused node (for search)
  const focusedNodeId = matchedNodeIds[focusedMatchIndex] || null

  useEffect(() => {
    if (!focusedNodeId) return

    // Get all ancestor nodes that need to be expanded
    const ancestors = getAncestors(focusedNodeId)
    if (ancestors.length === 0) return

    // Expand all ancestors
    setExpandedNodes((prev) => {
      const next = new Set(prev)
      let changed = false
      for (const ancestorId of ancestors) {
        if (!next.has(ancestorId)) {
          next.add(ancestorId)
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [focusedNodeId, getAncestors])

  // Auto-expand ancestors of problem nodes when problem mode is activated
  useEffect(() => {
    if (!problemMode || problemNodeIds.size === 0) return

    setExpandedNodes((prev) => {
      const next = new Set(prev)
      let changed = false
      for (const problemId of problemNodeIds) {
        const ancestors = getAncestors(problemId)
        for (const ancestorId of ancestors) {
          if (!next.has(ancestorId)) {
            next.add(ancestorId)
            changed = true
          }
        }
      }
      return changed ? next : prev
    })
  }, [problemMode, problemNodeIds, getAncestors])

  // Reset focus index when search query changes
  useEffect(() => {
    setFocusedMatchIndex(0)
  }, [searchQuery])

  // Keyboard shortcut for search (Cmd/Ctrl + F)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
        e.preventDefault()
        searchInputRef.current?.focus()
      }
      if (e.key === 'Escape' && isSearchFocused) {
        setSearchQuery('')
        searchInputRef.current?.blur()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isSearchFocused])

  // Handle Enter key in search input to cycle through matches
  const handleSearchKeyDown = useCallback((e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && matchedNodeIds.length > 0) {
      e.preventDefault()
      if (e.shiftKey) {
        // Shift+Enter: go to previous match
        setFocusedMatchIndex((prev) =>
          prev === 0 ? matchedNodeIds.length - 1 : prev - 1
        )
      } else {
        // Enter: go to next match
        setFocusedMatchIndex((prev) =>
          (prev + 1) % matchedNodeIds.length
        )
      }
    }
  }, [matchedNodeIds.length])

  // Handle click on node body - open detail drawer
  const handleNodeClick = useCallback((node: TopologyNode) => {
    setSelectedNode(node)
  }, [])

  // Handle click on chevron - toggle expand/collapse
  const handleNodeExpand = useCallback((node: TopologyNode) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev)
      if (next.has(node.id)) {
        next.delete(node.id)
      } else {
        next.add(node.id)
      }
      return next
    })
  }, [])

  const handleCloseDrawer = useCallback(() => {
    setSelectedNode(null)
  }, [])

  const handleExpandAll = useCallback(() => {
    if (!data) return
    const allIds = new Set([
      ...data.nodes.filter((n) => n.type === 'hub').map((n) => n.id),
      ...data.nodes.filter((n) => n.type === 'form').map((n) => n.id),
    ])
    setExpandedNodes(allIds)
  }, [data])

  const handleCollapseAll = useCallback(() => {
    setExpandedNodes(new Set())
  }, [])

  const toggleFullscreen = useCallback(() => {
    setIsFullscreen((prev) => !prev)
  }, [])

  const toggleProblemMode = useCallback(() => {
    setProblemMode((prev) => !prev)
    // Clear search when toggling problem mode
    if (!problemMode) {
      setSearchQuery('')
    }
  }, [problemMode])

  return (
    <div className={`h-full ${isFullscreen ? 'fixed inset-0 z-50 bg-background' : ''}`}>
      <Card className="h-full flex flex-col">
        <CardHeader className="flex-shrink-0 pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <IconSitemap size={20} />
              {t('topology.title')}
            </CardTitle>
            <div className="flex items-center gap-2">
              {/* Problem Mode Toggle */}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant={problemMode ? 'default' : 'outline'}
                    size="sm"
                    onClick={toggleProblemMode}
                    className={problemMode ? 'bg-rose-600 hover:bg-rose-700 text-white' : ''}
                  >
                    <IconAlertTriangle size={14} className="mr-1.5" />
                    {t('topology.problems')}
                    {problemCount > 0 && (
                      <Badge
                        variant={problemMode ? 'secondary' : 'destructive'}
                        className={`ml-1.5 h-5 min-w-[20px] px-1.5 ${problemMode ? 'bg-white/20 text-white' : ''}`}
                      >
                        {problemCount}
                      </Badge>
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {problemMode ? t('topology.exitProblemMode') : t('topology.highlightFailed')}
                </TooltipContent>
              </Tooltip>

              {/* Search input */}
              <div className="relative">
                <IconSearch size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
                <Input
                  ref={searchInputRef}
                  type="text"
                  placeholder={t('topology.searchNodes')}
                  value={searchQuery}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                    setSearchQuery(e.target.value)
                    // Disable problem mode when searching
                    if (e.target.value && problemMode) {
                      setProblemMode(false)
                    }
                  }}
                  onKeyDown={handleSearchKeyDown}
                  onFocus={() => setIsSearchFocused(true)}
                  onBlur={() => setIsSearchFocused(false)}
                  className="w-48 h-8 pl-8 pr-16 text-sm"
                />
                {searchQuery ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        onClick={() => setSearchQuery('')}
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground cursor-pointer"
                      >
                        <IconX size={14} />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent>{t('topology.clearSearch')}</TooltipContent>
                  </Tooltip>
                ) : (
                  <kbd className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground">
                    <span className="text-xs">⌘</span>F
                  </kbd>
                )}
              </div>

              {/* Search results count */}
              {searchQuery && (
                <Badge variant={searchHighlightedIds.size > 0 ? 'secondary' : 'outline'} className="text-xs tabular-nums">
                  {searchHighlightedIds.size > 0
                    ? `${focusedMatchIndex + 1}/${searchHighlightedIds.size}`
                    : `0 ${t('topology.found')}`}
                </Badge>
              )}

              {/* Stats badges */}
              <div className="hidden lg:flex items-center gap-2 ml-2">
                <Badge variant="outline" className="gap-1">
                  <IconBox size={12} />
                  {hubNodes.length} {t('nav.hubs')}
                </Badge>
                <Badge variant="outline" className="gap-1">
                  <IconStack2 size={12} />
                  {formNodes.length} {t('nav.forms')}
                </Badge>
                <Badge variant="outline" className="gap-1">
                  <IconActivity size={12} />
                  {lynqNodes.length} {t('nav.nodes')}
                </Badge>
              </div>

              {/* Actions */}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleExpandAll}
                    disabled={loading || !data}
                  >
                    {t('topology.expandAll')}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t('topology.expandAllTooltip')}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleCollapseAll}
                    disabled={loading || expandedNodes.size === 0}
                  >
                    {t('topology.collapseAll')}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t('topology.collapseAllTooltip')}</TooltipContent>
              </Tooltip>

              {/* Poll interval + Refresh group */}
              <div className="flex items-center h-8 rounded-lg border border-input bg-transparent shadow-sm">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="flex items-center gap-1.5 px-2 text-muted-foreground cursor-help">
                      <IconClock size={14} />
                    </div>
                  </TooltipTrigger>
                  <TooltipContent>{t('topology.autoRefreshInterval')}</TooltipContent>
                </Tooltip>
                <Select
                  value={String(pollInterval)}
                  onValueChange={(value) => setPollInterval(Number(value))}
                >
                  <SelectTrigger className="w-16 h-8 border-0 shadow-none rounded-none text-xs focus:ring-0">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">{t('time.off')}</SelectItem>
                    <SelectItem value="10000">10s</SelectItem>
                    <SelectItem value="30000">30s</SelectItem>
                    <SelectItem value="60000">1m</SelectItem>
                    <SelectItem value="300000">5m</SelectItem>
                  </SelectContent>
                </Select>
                <div className="w-px h-4 bg-border" />
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      onClick={() => refetch()}
                      disabled={loading}
                      className="flex items-center justify-center w-8 h-8 rounded-r-lg hover:bg-accent disabled:opacity-50 disabled:pointer-events-none transition-colors cursor-pointer"
                    >
                      <IconRefresh size={14} className={loading ? 'animate-spin' : ''} />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>{t('topology.refreshNow')}</TooltipContent>
                </Tooltip>
              </div>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={toggleFullscreen}
                  >
                    {isFullscreen ? (
                      <IconMinimize size={16} />
                    ) : (
                      <IconMaximize size={16} />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {isFullscreen ? t('topology.exitFullscreen') : t('topology.fullscreen')}
                </TooltipContent>
              </Tooltip>
            </div>
          </div>
        </CardHeader>

        <CardContent className="flex-1 p-0 relative">
          {/* Loading State */}
          {loading && !data && (
            <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
              <div className="flex flex-col items-center gap-2">
                <IconLoader2 size={32} className="animate-spin text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  {t('topology.loadingTopology')}
                </p>
              </div>
            </div>
          )}

          {/* Error State */}
          {error && (
            <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
              <div className="flex flex-col items-center gap-2 text-center max-w-sm">
                <IconAlertCircle size={32} className="text-destructive" />
                <p className="text-sm font-medium">{t('topology.failedToLoad')}</p>
                <p className="text-xs text-muted-foreground">
                  {error.message}
                </p>
                <Button variant="outline" size="sm" onClick={() => refetch()}>
                  <IconRefresh size={16} className="mr-2" />
                  {t('common.retry')}
                </Button>
              </div>
            </div>
          )}

          {/* Empty State */}
          {!loading && !error && data && data.nodes.length === 0 && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="flex flex-col items-center gap-2 text-center">
                <IconSitemap size={48} className="text-muted-foreground/50" />
                <h3 className="text-lg font-medium">{t('topology.noResourcesFound')}</h3>
                <p className="text-sm text-muted-foreground">
                  {t('topology.createHubToStart')}
                </p>
              </div>
            </div>
          )}

          {/* Canvas */}
          {data && data.nodes.length > 0 && (
            <TopologyCanvas
              nodes={data.nodes}
              edges={data.edges}
              onNodeClick={handleNodeClick}
              onNodeExpand={handleNodeExpand}
              selectedNodeId={selectedNode?.id}
              expandedNodes={expandedNodes}
              highlightedNodeIds={highlightedNodeIds}
              dimNonHighlighted={dimNonHighlighted}
              focusNodeId={focusedNodeId}
              highlightMode={highlightMode}
              className="h-full"
            />
          )}

          {/* Problem mode indicator */}
          {problemMode && problemCount > 0 && (
            <div className="absolute top-2 left-2">
              <Badge variant="destructive" className="gap-1.5 animate-pulse">
                <IconAlertTriangle size={12} />
                {t('topology.problemsDetected', { count: problemCount })}
              </Badge>
            </div>
          )}

          {/* Problem mode - no problems */}
          {problemMode && problemCount === 0 && (
            <div className="absolute top-2 left-2">
              <Badge variant="secondary" className="gap-1.5">
                <IconAlertTriangle size={12} />
                {t('topology.noProblemsDetected')}
              </Badge>
            </div>
          )}

          {/* Loading overlay when refreshing */}
          {loading && data && !problemMode && (
            <div className="absolute top-2 left-2">
              <Badge variant="secondary" className="gap-1">
                <IconLoader2 size={12} className="animate-spin" />
                {t('topology.refreshing')}
              </Badge>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Node Detail Drawer */}
      <NodeDetailDrawer
        node={selectedNode}
        allNodes={data?.nodes || []}
        open={!!selectedNode}
        onClose={handleCloseDrawer}
      />
    </div>
  )
}
