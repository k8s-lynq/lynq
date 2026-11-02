import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import type { Health } from '@/lib/health'

// HealthBadge shows a three-state health. null is "not known yet" (a hub or form whose
// LynqNodes have not loaded) and renders as a neutral loading badge, never as a state.
export function HealthBadge({ health }: { health: Health | null }) {
  const { t } = useTranslation()
  if (!health) {
    return (
      <Badge variant="outline" className="text-muted-foreground">
        {t('common.loading')}
      </Badge>
    )
  }
  return <Badge variant={health}>{t(`status.${health}`)}</Badge>
}
