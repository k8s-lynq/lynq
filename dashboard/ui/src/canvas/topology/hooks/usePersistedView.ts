import { useState, useCallback } from 'react'
import { PERSISTED_VIEW_KEY } from '../types'
import type { PersistedView, TierLevel, ViewTransform, TopologyFilters } from '../types'

const DEFAULT_FILTERS: TopologyFilters = {
  status: 'all',
  namespace: '',
  formId: '',
  showResources: false,
}

const DEFAULT_VIEW: PersistedView = {
  tier: 'sunburst',
  selectedFormId: null,
  transform: { x: 0, y: 0, scale: 1 },
  filters: DEFAULT_FILTERS,
}

function loadView(): PersistedView {
  try {
    const raw = localStorage.getItem(PERSISTED_VIEW_KEY)
    if (raw) return { ...DEFAULT_VIEW, ...JSON.parse(raw) }
  } catch {
    // ignore
  }
  return DEFAULT_VIEW
}

function saveView(v: PersistedView): void {
  try {
    localStorage.setItem(PERSISTED_VIEW_KEY, JSON.stringify(v))
  } catch {
    // ignore
  }
}

export function usePersistedView() {
  const [view, setView] = useState<PersistedView>(loadView)

  const update = useCallback((patch: Partial<PersistedView>) => {
    setView((prev) => {
      const next = { ...prev, ...patch }
      saveView(next)
      return next
    })
  }, [])

  const setTier = useCallback((tier: TierLevel) => update({ tier }), [update])
  const setSelectedFormId = useCallback((id: string | null) => update({ selectedFormId: id }), [update])
  const setTransform = useCallback((transform: ViewTransform) => update({ transform }), [update])
  const setFilters = useCallback((filters: TopologyFilters) => update({ filters }), [update])
  const resetFilters = useCallback(() => update({ filters: DEFAULT_FILTERS }), [update])

  return {
    view,
    setTier,
    setSelectedFormId,
    setTransform,
    setFilters,
    resetFilters,
  }
}
