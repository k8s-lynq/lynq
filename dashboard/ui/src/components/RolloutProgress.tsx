import { useTranslation } from 'react-i18next'
import { IconAlertTriangle } from '@tabler/icons-react'
import { Badge } from '@/components/ui/badge'
import { rolloutPercent, type RolloutView } from '@/lib/health'
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

// RolloutProgress shows a LynqForm rollout: progress is the nodes that completed the update over
// all nodes (RolloutView.completed), the operator's lynqform_rollout_progress, so a node
// recovering after the rollout reached it does not hold the bar below 100%.
// A Failed rollout (progressDeadlineSeconds exceeded, or an updated node failed) shows the
// operator's message, which names the stalled nodes.
//
// Nodes that completed the rollout and are now recovering are shown apart from the nodes still
// updating, and do not keep the rollout In progress. When the dashboard's phase differs from the
// operator's (an older operator reporting a recovery as InProgress, see rolloutView), the
// operator's phase and message are shown next to it.
export function RolloutProgress({
  rollout,
  view,
  compact = false,
}: {
  rollout: RolloutStatus
  view: RolloutView
  compact?: boolean
}) {
  const { t } = useTranslation()
  const percentage = rolloutPercent(view)
  const reportedDiffers = view.phase !== view.reportedPhase

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="flex items-center gap-2">
          {compact && <span className="text-muted-foreground">{t('forms.rollout')}</span>}
          <RolloutPhaseBadge phase={view.phase} />
          {view.recovering > 0 && (
            <Badge variant="secondary" title={t('forms.recoveringHint')}>
              {t('forms.recoveringCount', { count: view.recovering })}
            </Badge>
          )}
        </span>
        <span title={t('forms.completedHint')}>
          {view.completed} / {view.total} {t('forms.completedNodes')}
        </span>
      </div>
      {reportedDiffers && (
        <p
          className="text-xs text-muted-foreground"
          title={[t('forms.reportedPhaseHint'), rollout.message].filter(Boolean).join('\n')}
        >
          {t('forms.reportedPhase', { phase: t(`forms.rolloutPhase.${view.reportedPhase}`) })}
        </p>
      )}
      <div className="h-2 rounded-full bg-muted overflow-hidden">
        <div
          className={`h-full transition-all ${PHASE_BAR[view.phase]}`}
          style={{ width: `${percentage}%` }}
        />
      </div>
      {!compact && (
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>
            {t('forms.updatedNodes')}: {view.updated}
          </span>
          <span>
            {t('forms.readyUpdatedNodes')}: {view.readyUpdated}
          </span>
          <span>
            {t('forms.updatingNodes')}: {view.updating}
          </span>
          <span title={t('forms.recoveringHint')}>
            {t('forms.recoveringNodes')}: {view.recovering}
          </span>
          {rollout.targetGeneration !== undefined && (
            <span>
              {t('forms.targetGeneration')}: {rollout.targetGeneration}
            </span>
          )}
        </div>
      )}
      {view.phase === 'Failed' && (
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
