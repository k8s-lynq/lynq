import { useState, useEffect, useRef, useCallback } from 'react'
import type { KubernetesEvent } from '@/types/lynq'
import { eventsApi } from '@/lib/api'

export type ConnectionStatus = 'loading' | 'polling' | 'disconnected'

interface UseEventsOptions {
  nodeName: string
  namespace?: string
  enabled?: boolean
  maxEvents?: number
  pollInterval?: number
}

interface UseEventsResult {
  events: KubernetesEvent[]
  loading: boolean
  error: Error | null
  connectionStatus: ConnectionStatus
  lastUpdate: Date | null
  refetch: () => Promise<void>
}

export function useEvents(options: UseEventsOptions): UseEventsResult {
  const {
    nodeName,
    namespace,
    enabled = true,
    maxEvents = 50,
    pollInterval = 5000,
  } = options

  const [events, setEvents] = useState<KubernetesEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected')
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null)

  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Fetch function
  const fetchEvents = useCallback(async () => {
    if (!nodeName) return

    setLoading(true)
    try {
      const response = await eventsApi.getEvents(nodeName, namespace)
      setEvents((response.items || []).slice(0, maxEvents))
      setError(null)
      setLastUpdate(new Date())
      setConnectionStatus('polling')
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      setLoading(false)
    }
  }, [nodeName, namespace, maxEvents])

  // Polling effect
  useEffect(() => {
    if (!enabled || !nodeName) {
      setConnectionStatus('disconnected')
      setEvents([])
      return
    }

    // Set loading status for initial fetch
    setConnectionStatus('loading')

    // Initial fetch
    fetchEvents()

    // Start polling interval
    intervalRef.current = setInterval(fetchEvents, pollInterval)

    // Cleanup
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [enabled, nodeName, namespace, pollInterval, fetchEvents])

  return {
    events,
    loading,
    error,
    connectionStatus,
    lastUpdate,
    refetch: fetchEvents,
  }
}
