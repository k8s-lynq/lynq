import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { IconDatabase, IconExternalLink, IconRefresh } from '@tabler/icons-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useHubs, useHubPolling } from '@/contexts/HubContext'
import type { LynqHub } from '@/types/lynq'

function HubCard({ hub }: { hub: LynqHub }) {
  const { t } = useTranslation()
  const isReady = hub.status?.conditions?.some(
    (c) => c.type === 'Ready' && c.status === 'True'
  )
  const status = isReady ? 'ready' : 'pending'

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2">
            <IconDatabase size={16} />
            {hub.metadata.name}
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            {hub.metadata.namespace}
          </p>
        </div>
        <Badge variant={status}>{isReady ? t('status.ready') : t('status.pending')}</Badge>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* Stats */}
          <div className="grid grid-cols-3 gap-4 text-center">
            <div>
              <p className="text-2xl font-bold">{hub.status?.desired || 0}</p>
              <p className="text-xs text-muted-foreground">{t('hubs.desired')}</p>
            </div>
            <div>
              <p className="text-2xl font-bold text-emerald-500">
                {hub.status?.ready || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.ready')}</p>
            </div>
            <div>
              <p className="text-2xl font-bold text-rose-500">
                {hub.status?.failed || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.failed')}</p>
            </div>
          </div>

          {/* Details */}
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t('hubs.templates')}</span>
              <span>{hub.status?.referencingTemplates || 0}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t('hubs.syncInterval')}</span>
              <span>{hub.spec.source.syncInterval}</span>
            </div>
            {hub.spec.source.mysql && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t('hubs.database')}</span>
                <span className="truncate max-w-[150px]">
                  {hub.spec.source.mysql.database}
                </span>
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="flex gap-2">
            <Button variant="outline" size="sm" className="flex-1" asChild>
              <Link to={`/hubs/${hub.metadata.name}`}>
                <IconExternalLink size={12} className="mr-2" />
                {t('common.details')}
              </Link>
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function Hubs() {
  const { t } = useTranslation()
  const { hubs, loading, fetchHubs } = useHubs()

  // Enable polling
  useHubPolling()

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">{t('hubs.title')}</h2>
          <p className="text-muted-foreground">
            {t('hubs.description')}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => fetchHubs()}
          disabled={loading}
        >
          <IconRefresh size={16} className={`mr-2 ${loading ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </Button>
      </div>

      {/* Hub Grid */}
      {hubs.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <IconDatabase size={48} className="text-muted-foreground/50" />
            <h3 className="mt-4 text-lg font-medium">{t('hubs.noHubsFound')}</h3>
            <p className="mt-2 text-sm text-muted-foreground">
              {loading
                ? t('hubs.loadingHubs')
                : t('hubs.createHubToStart')}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {hubs.map((hub) => (
            <HubCard key={`${hub.metadata.namespace}/${hub.metadata.name}`} hub={hub} />
          ))}
        </div>
      )}
    </div>
  )
}
