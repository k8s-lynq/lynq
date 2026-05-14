import { useMemo } from 'react'
import { useEvents } from './useEvents'

export interface ConflictInfo {
  kind: string
  namespace: string
  name: string
  policy: string
  message: string
  lastSeen: string
  count: number
}

interface UseNodeConflictsOptions {
  nodeName: string
  namespace?: string
  enabled?: boolean
}

interface UseNodeConflictsResult {
  hasConflict: boolean
  conflictsByKey: Map<string, ConflictInfo>
  conflictCount: number
}

// Matches: "Resource conflict detected for {ns}/{name} (Kind: {kind}, Policy: {policy})"
const CONFLICT_REGEX =
  /Resource conflict detected for (?<ns>[^/]+)\/(?<name>\S+) \(Kind: (?<kind>\w+), Policy: (?<policy>\w+)\)/

export function useNodeConflicts({
  nodeName,
  namespace,
  enabled = true,
}: UseNodeConflictsOptions): UseNodeConflictsResult {
  const { events } = useEvents({
    nodeName,
    namespace,
    enabled,
    maxEvents: 50,
    pollInterval: 10000,
  })

  return useMemo(() => {
    const conflictsByKey = new Map<string, ConflictInfo>()

    for (const event of events) {
      if (event.reason !== 'ResourceConflict') continue
      const match = CONFLICT_REGEX.exec(event.message)
      if (!match?.groups) continue

      const { ns, name, kind, policy } = match.groups
      const key = `${kind}/${ns}/${name}`

      // Keep only the first match per key — events are ordered newest-first by the API
      if (!conflictsByKey.has(key)) {
        conflictsByKey.set(key, {
          kind,
          namespace: ns,
          name,
          policy,
          message: event.message,
          lastSeen: event.lastTimestamp,
          count: event.count,
        })
      }
    }

    return {
      hasConflict: conflictsByKey.size > 0,
      conflictsByKey,
      conflictCount: conflictsByKey.size,
    }
  }, [events])
}
