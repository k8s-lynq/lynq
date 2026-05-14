import { useState } from 'react'
import type { TopologyNode } from '@/types/lynq'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { IconAlertTriangle, IconX } from '@tabler/icons-react'
import { STATUS_COLORS, ORPHAN_COLOR } from '../constants'

interface OrphanPanelProps {
  orphans: TopologyNode[]
  onNodeClick: (node: TopologyNode) => void
}

interface OrphanTriggerProps {
  orphans: TopologyNode[]
  onOpen: () => void
}

export function OrphanTrigger({ orphans, onOpen }: OrphanTriggerProps) {
  if (orphans.length === 0) return null
  return (
    <Button
      variant="outline"
      size="sm"
      className="h-7 text-xs gap-1.5 border-amber-300 text-amber-700 hover:bg-amber-50"
      onClick={onOpen}
    >
      <IconAlertTriangle size={13} />
      {orphans.length} orphaned
    </Button>
  )
}

interface OrphanPanelDrawerProps {
  orphans: TopologyNode[]
  onNodeClick: (node: TopologyNode) => void
  onClose: () => void
}

export function OrphanPanelDrawer({ orphans, onNodeClick, onClose }: OrphanPanelDrawerProps) {
  return (
    <div className="absolute top-0 right-0 bottom-0 w-72 bg-background border-l shadow-xl z-40 flex flex-col">
      <div className="flex items-center justify-between px-4 py-3 border-b flex-shrink-0">
        <div className="flex items-center gap-2">
          <IconAlertTriangle size={16} style={{ color: ORPHAN_COLOR }} />
          <span className="font-medium text-sm">Orphaned Resources</span>
          <Badge variant="outline" className="text-xs">{orphans.length}</Badge>
        </div>
        <button
          onClick={onClose}
          className="text-muted-foreground hover:text-foreground cursor-pointer"
        >
          <IconX size={16} />
        </button>
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-1 min-h-0">
        {orphans.map((node) => (
          <button
            key={node.id}
            className="w-full text-left rounded px-3 py-2 hover:bg-accent text-sm flex items-center gap-2 cursor-pointer"
            onClick={() => { onNodeClick(node); onClose() }}
          >
            <div
              className="w-2 h-2 rounded-full flex-shrink-0"
              style={{ backgroundColor: STATUS_COLORS[node.status]?.accent ?? '#94a3b8' }}
            />
            <div className="min-w-0">
              <p className="font-medium truncate">{node.name}</p>
              <p className="text-xs text-muted-foreground truncate">{node.namespace}</p>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}

// Legacy combined component kept for convenience
export function OrphanBadge({ orphans, onNodeClick }: OrphanPanelProps) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <OrphanTrigger orphans={orphans} onOpen={() => setOpen(true)} />
      {open && <OrphanPanelDrawer orphans={orphans} onNodeClick={onNodeClick} onClose={() => setOpen(false)} />}
    </>
  )
}
