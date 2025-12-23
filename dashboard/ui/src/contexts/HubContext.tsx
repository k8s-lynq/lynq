import {
  createContext,
  useContext,
  useReducer,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react'
import type { LynqHub, ListResponse } from '@/types/lynq'
import { hubApi } from '@/lib/api'

interface HubState {
  hubs: LynqHub[]
  selectedHub: LynqHub | null
  loading: boolean
  error: string | null
  lastUpdated: Date | null
}

type HubAction =
  | { type: 'FETCH_START' }
  | { type: 'FETCH_SUCCESS'; payload: LynqHub[] }
  | { type: 'FETCH_ERROR'; payload: string }
  | { type: 'SELECT_HUB'; payload: LynqHub | null }
  | { type: 'UPDATE_HUB'; payload: LynqHub }

const initialState: HubState = {
  hubs: [],
  selectedHub: null,
  loading: false,
  error: null,
  lastUpdated: null,
}

function hubReducer(state: HubState, action: HubAction): HubState {
  switch (action.type) {
    case 'FETCH_START':
      return { ...state, loading: true, error: null }
    case 'FETCH_SUCCESS':
      return {
        ...state,
        hubs: action.payload,
        loading: false,
        lastUpdated: new Date(),
      }
    case 'FETCH_ERROR':
      return { ...state, loading: false, error: action.payload }
    case 'SELECT_HUB':
      return { ...state, selectedHub: action.payload }
    case 'UPDATE_HUB':
      return {
        ...state,
        hubs: state.hubs.map((h) =>
          h.metadata.name === action.payload.metadata.name ? action.payload : h
        ),
        selectedHub:
          state.selectedHub?.metadata.name === action.payload.metadata.name
            ? action.payload
            : state.selectedHub,
      }
    default:
      return state
  }
}

interface HubContextValue extends HubState {
  fetchHubs: (namespace?: string) => Promise<void>
  selectHub: (hub: LynqHub | null) => void
  getHub: (name: string, namespace?: string) => Promise<LynqHub>
}

const HubContext = createContext<HubContextValue | null>(null)

export function HubProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(hubReducer, initialState)

  const fetchHubs = useCallback(async (namespace?: string) => {
    dispatch({ type: 'FETCH_START' })
    try {
      const response: ListResponse<LynqHub> = await hubApi.list(namespace)
      dispatch({ type: 'FETCH_SUCCESS', payload: response.items })
    } catch (err) {
      dispatch({
        type: 'FETCH_ERROR',
        payload: err instanceof Error ? err.message : 'Failed to fetch hubs',
      })
    }
  }, [])

  const selectHub = useCallback((hub: LynqHub | null) => {
    dispatch({ type: 'SELECT_HUB', payload: hub })
  }, [])

  const getHub = useCallback(async (name: string, namespace?: string) => {
    const hub = await hubApi.get(name, namespace)
    dispatch({ type: 'UPDATE_HUB', payload: hub })
    return hub
  }, [])

  return (
    <HubContext.Provider value={{ ...state, fetchHubs, selectHub, getHub }}>
      {children}
    </HubContext.Provider>
  )
}

export function useHubs() {
  const context = useContext(HubContext)
  if (!context) {
    throw new Error('useHubs must be used within a HubProvider')
  }
  return context
}

// Polling hook for hubs
export function useHubPolling(namespace?: string, interval = 30000) {
  const { fetchHubs } = useHubs()

  useEffect(() => {
    fetchHubs(namespace)
    const timer = setInterval(() => fetchHubs(namespace), interval)
    return () => clearInterval(timer)
  }, [fetchHubs, namespace, interval])
}
