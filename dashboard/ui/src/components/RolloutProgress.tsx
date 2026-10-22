import { useTranslation } from 'react-i18next'
import { IconAlertTriangle } from '@tabler/icons-react'
import { Badge } from '@/components/ui/badge'
import type { RolloutPhase, RolloutStatus } from '@/types/lynq'

const PHASE_VARIANT: Record<RolloutPhase, 'pending' | 'failed' | 'ready' | 'secondary'> = {
  InProgress: 'pending',
  Failed: 'failed',
  Complete: 'ready',
  Idle: 'secondary',
}

const PHASE_BAR: Record<RolloutPhase, string> = {
  InProgress: 'bg-amber-500',
  Failed: 'bg-rose-500',
  Complete: 'bg-emerald-500',
  Idle: 'bg-primary',
}

export function RolloutPhaseBadge({ phase }: { phase: RolloutPhase }) {
  const { t } = useTranslation()
  return <Badge variant={PHASE_VARIANT[phase]}>{t(`forms.rolloutPhase.${phase}`)}</Badge>
}

// RolloutProgress shows a LynqForm rollout: progress is nodes updated AND Ready over all nodes.
// A Failed rollout (progressDeadlineSeconds exceeded, or an updated node failed) shows the
// operator's message, which names the stalled nodes.
export function RolloutProgress({
  rollout,
  compact = false,
}: {
  rollout: RolloutStatus & { phase: RolloutPhase }
  compact?: boolean
}) {
  const { t } = useTranslation()
  const total = rollout.totalNodes ?? 0
  const readyUpdated = rollout.readyUpdatedNodes ?? 0
  const percentage = total > 0 ? Math.round((readyUpdated / total) * 100) : 0

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="flex items-center gap-2">
          {compact && <span className="text-muted-foreground">{t('forms.rollout')}</span>}
          <RolloutPhaseBadge phase={rollout.phase} />
        </span>
        <span>
          {readyUpdated} / {total} {t('forms.readyUpdatedNodes')}
        </span>
      </div>
      <div className="h-2 rounded-full bg-muted overflow-hidden">
        <div
          className={`h-full transition-all ${PHASE_BAR[rollout.phase]}`}
          style={{ width: `${percentage}%` }}
        />
      </div>
      {!compact && (
        <div className="flex gap-4 text-xs text-muted-foreground">
          <span>
            {t('forms.updatedNodes')}: {rollout.updatedNodes ?? 0}
          </span>
          <span>
            {t('forms.updatingNodes')}: {rollout.updatingNodes ?? 0}
          </span>
          {rollout.targetGeneration !== undefined && (
            <span>
              {t('forms.targetGeneration')}: {rollout.targetGeneration}
            </span>
          )}
        </div>
      )}
      {rollout.phase === 'Failed' && (
        <div className="flex items-start gap-2 rounded-md border border-rose-200 dark:border-rose-900 p-2 text-xs">
          <IconAlertTriangle size={14} className="text-rose-500 shrink-0 mt-0.5" />
          <div>
            <p className="font-medium text-rose-500">{t('forms.rolloutStalled')}</p>
            {rollout.message && (
              <p className={`text-muted-foreground ${compact ? 'line-clamp-2' : ''}`}>
                {rollout.message}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
