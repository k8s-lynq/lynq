import { useState, useEffect, useCallback, useMemo } from 'react'
import { topologyApi } from '@/lib/api'
import type { TopologyData, TopologyNode, TopologyEdge } from '@/types/lynq'

interface UseTopologyOptions {
  namespace?: string
  pollInterval?: number
  enabled?: boolean
}

interface UseTopologyResult {
  data: TopologyData | null
  loading: boolean
  error: Error | null
  refetch: () => Promise<void>
  // Computed helpers
  getNode: (id: string) => TopologyNode | undefined
  getChildren: (id: string) => TopologyNode[]
  getParent: (id: string) => TopologyNode | undefined
  hubNodes: TopologyNode[]
  formNodes: TopologyNode[]
  lynqNodes: TopologyNode[]
}

export function useTopology(options: UseTopologyOptions = {}): UseTopologyResult {
  const { namespace, pollInterval = 30000, enabled = true } = options

  const [data, setData] = useState<TopologyData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetch = useCallback(async () => {
    if (!enabled) return

    setLoading(true)
    setError(null)

    try {
      const result = await topologyApi.get(namespace)
      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      setLoading(false)
    }
  }, [namespace, enabled])

  useEffect(() => {
    fetch()

    if (enabled && pollInterval > 0) {
      const timer = setInterval(fetch, pollInterval)
      return () => clearInterval(timer)
    }
  }, [fetch, enabled, pollInterval])

  // Create lookup maps
  const nodeMap = useMemo(() => {
    if (!data) return new Map<string, TopologyNode>()
    return new Map(data.nodes.map((n) => [n.id, n]))
  }, [data])

  const edgesByTarget = useMemo(() => {
    if (!data) return new Map<string, TopologyEdge>()
    return new Map(data.edges.map((e) => [e.target, e]))
  }, [data])

  // Helper functions
  const getNode = useCallback(
    (id: string) => nodeMap.get(id),
    [nodeMap]
  )

  const getChildren = useCallback(
    (id: string) => {
      const node = nodeMap.get(id)
      if (!node?.children) return []
      return node.children.map((childId) => nodeMap.get(childId)).filter(Boolean) as TopologyNode[]
    },
    [nodeMap]
  )

  const getParent = useCallback(
    (id: string) => {
      const edge = edgesByTarget.get(id)
      if (!edge) return undefined
      return nodeMap.get(edge.source)
    },
    [nodeMap, edgesByTarget]
  )

  // Filtered node lists
  const hubNodes = useMemo(
    () => data?.nodes.filter((n) => n.type === 'hub') || [],
    [data]
  )

  const formNodes = useMemo(
    () => data?.nodes.filter((n) => n.type === 'form') || [],
    [data]
  )

  const lynqNodes = useMemo(
    () => data?.nodes.filter((n) => n.type === 'node') || [],
    [data]
  )

  return {
    data,
    loading,
    error,
    refetch: fetch,
    getNode,
    getChildren,
    getParent,
    hubNodes,
    formNodes,
    lynqNodes,
  }
}
