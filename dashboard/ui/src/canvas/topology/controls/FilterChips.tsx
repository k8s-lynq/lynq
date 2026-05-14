import { useCallback } from 'react'
import type { TopologyNode, ResourceStatus } from '@/types/lynq'
import type { TopologyFilters } from '../types'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { IconX } from '@tabler/icons-react'

interface FilterChipsProps {
  filters: TopologyFilters
  formNodes: TopologyNode[]
  namespaces: string[]
  onChange: (filters: TopologyFilters) => void
  onReset: () => void
}

const STATUS_OPTIONS: { value: ResourceStatus | 'all'; label: string }[] = [
  { value: 'all', label: 'All Status' },
  { value: 'ready', label: 'Ready' },
  { value: 'failed', label: 'Failed' },
  { value: 'pending', label: 'Pending' },
  { value: 'skipped', label: 'Skipped' },
]

export function FilterChips({ filters, formNodes, namespaces, onChange, onReset }: FilterChipsProps) {
  const hasActiveFilter = filters.status !== 'all' || !!filters.namespace || !!filters.formId

  const set = useCallback(<K extends keyof TopologyFilters>(key: K, value: TopologyFilters[K]) => {
    onChange({ ...filters, [key]: value })
  }, [filters, onChange])

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <Select value={filters.status} onValueChange={(v) => set('status', v as ResourceStatus | 'all')}>
        <SelectTrigger className="h-7 text-xs w-32">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {STATUS_OPTIONS.map((o) => (
            <SelectItem key={o.value} value={o.value} className="text-xs">
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {namespaces.length > 1 && (
        <Select value={filters.namespace || '__all__'} onValueChange={(v) => set('namespace', v === '__all__' ? '' : v)}>
          <SelectTrigger className="h-7 text-xs w-36">
            <SelectValue placeholder="All Namespaces" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__" className="text-xs">All Namespaces</SelectItem>
            {namespaces.map((ns) => (
              <SelectItem key={ns} value={ns} className="text-xs">{ns}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {formNodes.length > 1 && (
        <Select value={filters.formId || '__all__'} onValueChange={(v) => set('formId', v === '__all__' ? '' : v)}>
          <SelectTrigger className="h-7 text-xs w-36">
            <SelectValue placeholder="All Forms" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__" className="text-xs">All Forms</SelectItem>
            {formNodes.map((f) => (
              <SelectItem key={f.id} value={f.id} className="text-xs">{f.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {hasActiveFilter && (
        <Button variant="ghost" size="sm" className="h-7 px-2 text-xs gap-1" onClick={onReset}>
          <IconX size={12} />
          Clear
        </Button>
      )}
    </div>
  )
}
