import {
  createContext,
  useContext,
  useReducer,
  useCallback,
  useEffect,
  useRef,
  type ReactNode,
} from 'react'
import type { LynqNode, ListResponse } from '@/types/lynq'
import { nodeApi } from '@/lib/api'

interface NodeState {
  nodes: LynqNode[]
  selectedNode: LynqNode | null
  loading: boolean
  error: string | null
  lastUpdated: Date | null
}

type NodeAction =
  | { type: 'FETCH_START' }
  | { type: 'FETCH_SUCCESS'; payload: LynqNode[] }
  | { type: 'FETCH_ERROR'; payload: string }
  | { type: 'SELECT_NODE'; payload: LynqNode | null }
  | { type: 'UPDATE_NODE'; payload: LynqNode }

const initialState: NodeState = {
  nodes: [],
  selectedNode: null,
  loading: false,
  error: null,
  lastUpdated: null,
}

function nodeReducer(state: NodeState, action: NodeAction): NodeState {
  switch (action.type) {
    case 'FETCH_START':
      return { ...state, loading: true, error: null }
    case 'FETCH_SUCCESS':
      return {
        ...state,
        nodes: action.payload,
        loading: false,
        lastUpdated: new Date(),
      }
    case 'FETCH_ERROR':
      return { ...state, loading: false, error: action.payload }
    case 'SELECT_NODE':
      return { ...state, selectedNode: action.payload }
    case 'UPDATE_NODE':
      return {
        ...state,
        nodes: state.nodes.map((n) =>
          n.metadata.name === action.payload.metadata.name ? action.payload : n
        ),
        selectedNode:
          state.selectedNode?.metadata.name === action.payload.metadata.name
            ? action.payload
            : state.selectedNode,
      }
    default:
      return state
  }
}

interface NodeContextValue extends NodeState {
  // loaded: the node list has been fetched successfully at least once. Until then an empty
  // nodes array means "unknown", not "no nodes", and counts derived from it must not show 0.
  loaded: boolean
  fetchNodes: (namespace?: string) => Promise<void>
  selectNode: (node: LynqNode | null) => void
  getNode: (name: string, namespace?: string) => Promise<LynqNode>
}

const NodeContext = createContext<NodeContextValue | null>(null)

export function NodeProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(nodeReducer, initialState)

  // Listing every LynqNode is the dashboard's most expensive request. A fetch for the same
  // namespace already in flight (a page's poll and a refresh click, or two pages' polls racing
  // on navigation) is joined instead of issued again.
  const inFlight = useRef(new Map<string, Promise<void>>())

  const fetchNodes = useCallback((namespace?: string) => {
    const key = namespace ?? ''
    const pending = inFlight.current.get(key)
    if (pending) return pending
    const request = (async () => {
      dispatch({ type: 'FETCH_START' })
      try {
        const response: ListResponse<LynqNode> = await nodeApi.list(namespace)
        dispatch({ type: 'FETCH_SUCCESS', payload: response.items })
      } catch (err) {
        dispatch({
          type: 'FETCH_ERROR',
          payload: err instanceof Error ? err.message : 'Failed to fetch nodes',
        })
      } finally {
        inFlight.current.delete(key)
      }
    })()
    inFlight.current.set(key, request)
    return request
  }, [])

  const selectNode = useCallback((node: LynqNode | null) => {
    dispatch({ type: 'SELECT_NODE', payload: node })
  }, [])

  const getNode = useCallback(async (name: string, namespace?: string) => {
    const node = await nodeApi.get(name, namespace)
    dispatch({ type: 'UPDATE_NODE', payload: node })
    return node
  }, [])

  return (
    <NodeContext.Provider
      value={{ ...state, loaded: state.lastUpdated !== null, fetchNodes, selectNode, getNode }}
    >
      {children}
    </NodeContext.Provider>
  )
}

export function useNodes() {
  const context = useContext(NodeContext)
  if (!context) {
    throw new Error('useNodes must be used within a NodeProvider')
  }
  return context
}

// Polling hook for nodes. A hidden tab skips its polls (the node list is a full LynqNode LIST
// against the API server) and catches up as soon as it becomes visible again.
export function useNodePolling(namespace?: string, interval = 30000) {
  const { fetchNodes } = useNodes()

  useEffect(() => {
    fetchNodes(namespace)
    const timer = setInterval(() => {
      if (document.visibilityState !== 'hidden') fetchNodes(namespace)
    }, interval)
    let skippedWhileHidden = false
    const onVisibilityChange = () => {
      if (document.visibilityState === 'hidden') skippedWhileHidden = true
      else if (skippedWhileHidden) {
        skippedWhileHidden = false
        fetchNodes(namespace)
      }
    }
    document.addEventListener('visibilitychange', onVisibilityChange)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  }, [fetchNodes, namespace, interval])
}
