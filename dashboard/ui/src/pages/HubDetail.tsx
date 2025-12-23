import { useEffect, useState } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  IconDatabase,
  IconArrowLeft,
  IconRefresh,
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconFileCode,
  IconBox,
  IconCalendar,
  IconServer,
  IconTable,
  IconKey,
} from '@tabler/icons-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { hubApi } from '@/lib/api'
import type { LynqHub, LynqNode, Condition } from '@/types/lynq'

function ConditionBadge({ condition }: { condition: Condition }) {
  const isTrue = condition.status === 'True'
  const isFalse = condition.status === 'False'

  return (
    <div className="flex items-center gap-2 p-3 rounded-lg border bg-card">
      {isTrue ? (
        <IconCircleCheck size={16} className="text-emerald-500" />
      ) : isFalse ? (
        <IconCircleX size={16} className="text-rose-500" />
      ) : (
        <IconClock size={16} className="text-amber-500" />
      )}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium text-sm">{condition.type}</span>
          <Badge variant={isTrue ? 'ready' : isFalse ? 'failed' : 'pending'}>
            {condition.status}
          </Badge>
        </div>
        {condition.message && (
          <p className="text-xs text-muted-foreground truncate mt-1">
            {condition.message}
          </p>
        )}
      </div>
      <span className="text-xs text-muted-foreground shrink-0">
        {new Date(condition.lastTransitionTime).toLocaleString()}
      </span>
    </div>
  )
}

export function HubDetail() {
  const { name } = useParams<{ name: string }>()
  const [searchParams] = useSearchParams()
  const namespace = searchParams.get('namespace') || undefined
  const { t } = useTranslation()

  const [hub, setHub] = useState<LynqHub | null>(null)
  const [nodes, setNodes] = useState<LynqNode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchData = async () => {
    if (!name) return
    setLoading(true)
    setError(null)
    try {
      const [hubData, nodesData] = await Promise.all([
        hubApi.get(name, namespace),
        hubApi.getNodes(name, namespace),
      ])
      setHub(hubData)
      setNodes(nodesData.items || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : t('hubs.errorLoadingHub'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name, namespace])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <IconRefresh size={32} className="animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error || !hub) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/hubs">
            <IconArrowLeft size={16} className="mr-2" />
            {t('hubs.backToHubs')}
          </Link>
        </Button>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <IconCircleX size={48} className="text-rose-500" />
            <h3 className="mt-4 text-lg font-medium">{t('hubs.errorLoadingHub')}</h3>
            <p className="mt-2 text-sm text-muted-foreground">{error}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const isReady = hub.status?.conditions?.some(
    (c) => c.type === 'Ready' && c.status === 'True'
  )
  const mysql = hub.spec.source.mysql

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" asChild>
            <Link to="/hubs">
              <IconArrowLeft size={16} />
            </Link>
          </Button>
          <div>
            <div className="flex items-center gap-2">
              <IconDatabase size={20} />
              <h1 className="text-2xl font-bold">{hub.metadata.name}</h1>
              <Badge variant={isReady ? 'ready' : 'pending'}>
                {isReady ? t('status.ready') : t('status.pending')}
              </Badge>
            </div>
            <p className="text-muted-foreground">{hub.metadata.namespace}</p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={fetchData} disabled={loading}>
          <IconRefresh size={16} className={`mr-2 ${loading ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('hubs.templates')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconFileCode size={16} className="text-muted-foreground" />
              <span className="text-2xl font-bold">
                {hub.status?.referencingTemplates || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('hubs.desired')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconBox size={16} className="text-blue-500" />
              <span className="text-2xl font-bold text-blue-500">
                {hub.status?.desired || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('status.ready')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleCheck size={16} className="text-emerald-500" />
              <span className="text-2xl font-bold text-emerald-500">
                {hub.status?.ready || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('status.failed')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleX size={16} className="text-rose-500" />
              <span className="text-2xl font-bold text-rose-500">
                {hub.status?.failed || 0}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Datasource Info */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <IconServer size={16} />
              {t('hubs.datasourceConfig')}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {mysql && (
              <>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground">{t('hubs.host')}</p>
                    <p className="font-mono text-sm">{mysql.host}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">{t('hubs.port')}</p>
                    <p className="font-mono text-sm">{mysql.port}</p>
                  </div>
                </div>
                <Separator />
                <div className="grid grid-cols-2 gap-4">
                  <div className="flex items-center gap-2">
                    <IconTable size={16} className="text-muted-foreground" />
                    <div>
                      <p className="text-sm text-muted-foreground">{t('hubs.database')}</p>
                      <p className="font-mono text-sm">{mysql.database}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <IconTable size={16} className="text-muted-foreground" />
                    <div>
                      <p className="text-sm text-muted-foreground">{t('hubs.table')}</p>
                      <p className="font-mono text-sm">{mysql.table}</p>
                    </div>
                  </div>
                </div>
                <Separator />
                <div className="grid grid-cols-2 gap-4">
                  {mysql.userRef && (
                    <div className="flex items-center gap-2">
                      <IconKey size={16} className="text-muted-foreground" />
                      <div>
                        <p className="text-sm text-muted-foreground">{t('hubs.userSecret')}</p>
                        <p className="font-mono text-sm">
                          {mysql.userRef.name}/{mysql.userRef.key}
                        </p>
                      </div>
                    </div>
                  )}
                  {mysql.passwordRef && (
                    <div className="flex items-center gap-2">
                      <IconKey size={16} className="text-muted-foreground" />
                      <div>
                        <p className="text-sm text-muted-foreground">{t('hubs.passwordSecret')}</p>
                        <p className="font-mono text-sm">
                          {mysql.passwordRef.name}/{mysql.passwordRef.key}
                        </p>
                      </div>
                    </div>
                  )}
                </div>
              </>
            )}
            <Separator />
            <div className="flex items-center gap-2">
              <IconCalendar size={16} className="text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">{t('hubs.syncInterval')}</p>
                <p className="font-mono text-sm">{hub.spec.source.syncInterval}</p>
              </div>
            </div>
            {hub.status?.lastSyncTime && (
              <div className="flex items-center gap-2">
                <IconRefresh size={16} className="text-muted-foreground" />
                <div>
                  <p className="text-sm text-muted-foreground">{t('hubs.lastSync')}</p>
                  <p className="text-sm">
                    {new Date(hub.status.lastSyncTime).toLocaleString()}
                  </p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Value Mappings */}
        <Card>
          <CardHeader>
            <CardTitle>{t('hubs.valueMappings')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <p className="text-sm font-medium">{t('hubs.requiredMappings')}</p>
              <div className="grid grid-cols-2 gap-2">
                <div className="p-2 rounded bg-muted">
                  <p className="text-xs text-muted-foreground">{t('hubs.uidColumn')}</p>
                  <p className="font-mono text-sm">{hub.spec.valueMappings.uid}</p>
                </div>
                <div className="p-2 rounded bg-muted">
                  <p className="text-xs text-muted-foreground">{t('hubs.activateColumn')}</p>
                  <p className="font-mono text-sm">{hub.spec.valueMappings.activate}</p>
                </div>
              </div>
            </div>
            {hub.spec.extraValueMappings && hub.spec.extraValueMappings.length > 0 && (
              <>
                <Separator />
                <div className="space-y-2">
                  <p className="text-sm font-medium">{t('hubs.extraMappings')}</p>
                  <div className="space-y-1">
                    {hub.spec.extraValueMappings.map((mapping, i) => (
                      <div key={i} className="flex items-center gap-2 p-2 rounded bg-muted">
                        <span className="font-mono text-sm">{mapping.column}</span>
                        <span className="text-muted-foreground">→</span>
                        <Badge variant="secondary" className="font-mono">
                          .{mapping.variable}
                        </Badge>
                      </div>
                    ))}
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Conditions */}
      {hub.status?.conditions && hub.status.conditions.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t('hubs.conditions')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {hub.status.conditions.map((condition, i) => (
              <ConditionBadge key={i} condition={condition} />
            ))}
          </CardContent>
        </Card>
      )}

      {/* Related Nodes */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>{t('hubs.relatedNodes')} ({nodes.length})</span>
            <Button variant="outline" size="sm" asChild>
              <Link to={`/nodes?hub=${hub.metadata.name}`}>{t('common.viewAll')}</Link>
            </Button>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {nodes.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              {t('hubs.noNodesYet')}
            </p>
          ) : (
            <div className="space-y-2">
              {nodes.slice(0, 10).map((node) => {
                const nodeReady = node.status?.conditions?.some(
                  (c) => c.type === 'Ready' && c.status === 'True'
                )
                return (
                  <Link
                    key={`${node.metadata.namespace}-${node.metadata.name}`}
                    to={`/nodes/${node.metadata.name}?namespace=${node.metadata.namespace}`}
                    className="flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <IconBox size={16} className="text-muted-foreground" />
                      <div>
                        <p className="font-medium">{node.metadata.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {node.spec.templateRef}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm text-muted-foreground">
                        {node.status?.readyResources || 0}/{node.status?.desiredResources || 0}
                      </span>
                      {nodeReady ? (
                        <IconCircleCheck size={16} className="text-emerald-500" />
                      ) : (
                        <IconClock size={16} className="text-amber-500" />
                      )}
                    </div>
                  </Link>
                )
              })}
              {nodes.length > 10 && (
                <p className="text-sm text-muted-foreground text-center pt-2">
                  {t('hubs.moreNodes', { count: nodes.length - 10 })}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
