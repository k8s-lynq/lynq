import { useEffect, useRef, useCallback, useState } from 'react'

interface UsePollingOptions {
  interval?: number
  enabled?: boolean
  immediate?: boolean
}

interface UsePollingResult<T> {
  data: T | null
  loading: boolean
  error: Error | null
  lastUpdated: Date | null
  refetch: () => Promise<void>
}

export function usePolling<T>(
  fetcher: () => Promise<T>,
  options: UsePollingOptions = {}
): UsePollingResult<T> {
  const { interval = 30000, enabled = true, immediate = true } = options

  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)

  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await fetcherRef.current()
      setData(result)
      setLastUpdated(new Date())
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!enabled) return

    if (immediate) {
      fetch()
    }

    const timer = setInterval(fetch, interval)
    return () => clearInterval(timer)
  }, [enabled, immediate, interval, fetch])

  return { data, loading, error, lastUpdated, refetch: fetch }
}

// Hook for polling with dependency tracking
export function usePollingWithDeps<T, D extends unknown[]>(
  fetcher: (...deps: D) => Promise<T>,
  deps: D,
  options: UsePollingOptions = {}
): UsePollingResult<T> {
  const { interval = 30000, enabled = true, immediate = true } = options

  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)

  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await fetcherRef.current(...deps)
      setData(result)
      setLastUpdated(new Date())
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  useEffect(() => {
    if (!enabled) return

    if (immediate) {
      fetch()
    }

    const timer = setInterval(fetch, interval)
    return () => clearInterval(timer)
  }, [enabled, immediate, interval, fetch])

  return { data, loading, error, lastUpdated, refetch: fetch }
}
