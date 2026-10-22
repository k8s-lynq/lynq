import { IconCircleCheck, IconCircleX, IconClock } from '@tabler/icons-react'
import { Badge } from '@/components/ui/badge'
import { conditionTone } from '@/lib/health'
import type { Condition } from '@/types/lynq'

const TONE_ICON = {
  good: <IconCircleCheck size={16} className="text-emerald-500" />,
  bad: <IconCircleX size={16} className="text-rose-500" />,
  unknown: <IconClock size={16} className="text-amber-500" />,
}

const TONE_VARIANT = { good: 'ready', bad: 'failed', unknown: 'pending' } as const

// ConditionBadge colors a condition by what it means, not by its status: Degraded=False and
// Conflicted=False are healthy and render green.
export function ConditionBadge({ condition }: { condition: Condition }) {
  const tone = conditionTone(condition)

  return (
    <div className="flex items-center gap-2 p-3 rounded-lg border bg-card">
      {TONE_ICON[tone]}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium text-sm">{condition.type}</span>
          <Badge variant={TONE_VARIANT[tone]}>{condition.status}</Badge>
        </div>
        {condition.reason && (
          <p className="text-xs text-muted-foreground mt-0.5">{condition.reason}</p>
        )}
        {condition.message && (
          <p className="text-xs text-muted-foreground truncate mt-1">{condition.message}</p>
        )}
      </div>
      <span className="text-xs text-muted-foreground shrink-0">
        {new Date(condition.lastTransitionTime).toLocaleString()}
      </span>
    </div>
  )
}
