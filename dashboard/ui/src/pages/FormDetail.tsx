import { useEffect, useState } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  IconFileCode,
  IconArrowLeft,
  IconRefresh,
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconDatabase,
  IconBox,
  IconGitBranch,
  IconChevronDown,
  IconChevronRight,
  IconCopy,
  IconCheck,
  IconCircleDot,
  IconArchive,
  IconTrash,
  IconLock,
  IconBolt,
} from '@tabler/icons-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { formApi, type FormDetailsResponse } from '@/lib/api'
import type { LynqForm, Condition, TResource } from '@/types/lynq'

function ConditionBadge({ condition }: { condition: Condition }) {
  const { t } = useTranslation()
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

function TResourceCard({
  resource,
  kind,
}: {
  resource: TResource
  kind: string
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(JSON.stringify(resource, null, 2))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const isOnce = resource.creationPolicy === 'Once'
  const isRetain = resource.deletionPolicy === 'Retain'
  const isForce = resource.conflictPolicy === 'Force'

  return (
    <div className="border rounded-lg">
      <button
        className="flex items-center gap-2 w-full p-3 text-left hover:bg-muted/50"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? (
          <IconChevronDown size={16} />
        ) : (
          <IconChevronRight size={16} />
        )}
        <Badge variant="outline">{kind}</Badge>
        <span className="font-mono text-sm flex-1">{resource.id}</span>

        {/* Policy badges */}
        <div className="flex items-center gap-1">
          <span
            className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium ${
              isOnce
                ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
                : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
            }`}
            title={isOnce ? t('drawer.createdOnce') : t('drawer.continuouslyReconciled')}
          >
            {isOnce ? <IconCircleDot size={10} /> : <IconRefresh size={10} />}
          </span>
          <span
            className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium ${
              isRetain
                ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
                : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'
            }`}
            title={isRetain ? t('drawer.resourceRetained') : t('drawer.resourceDeleted')}
          >
            {isRetain ? <IconArchive size={10} /> : <IconTrash size={10} />}
          </span>
          <span
            className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium ${
              isForce
                ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
                : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'
            }`}
            title={isForce ? t('drawer.forceTakesOwnership') : t('drawer.stopsReconciliation')}
          >
            {isForce ? <IconBolt size={10} /> : <IconLock size={10} />}
          </span>
        </div>

        {resource.dependIds && resource.dependIds.length > 0 && (
          <Badge variant="secondary" className="text-xs">
            <IconGitBranch size={12} className="mr-1" />
            {resource.dependIds.length} {t('forms.dependencies')}
          </Badge>
        )}
      </button>

      {expanded && (
        <div className="border-t p-3 bg-muted/30">
          <div className="flex justify-end mb-2">
            <Button variant="ghost" size="sm" onClick={handleCopy}>
              {copied ? (
                <IconCheck size={12} className="mr-1 text-emerald-500" />
              ) : (
                <IconCopy size={12} className="mr-1" />
              )}
              {copied ? t('common.copied') : t('common.copy')}
            </Button>
          </div>

          {resource.dependIds && resource.dependIds.length > 0 && (
            <div className="mb-3">
              <p className="text-xs font-medium text-muted-foreground mb-1">
                {t('forms.dependencies')}
              </p>
              <div className="flex flex-wrap gap-1">
                {resource.dependIds.map((depId) => (
                  <Badge key={depId} variant="outline" className="font-mono text-xs">
                    {depId}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          <pre className="text-xs font-mono overflow-x-auto whitespace-pre-wrap bg-background p-3 rounded border">
            {JSON.stringify(resource.spec || resource, null, 2)}
          </pre>
        </div>
      )}
    </div>
  )
}

const RESOURCE_TYPE_LABELS: Record<string, { label: string; kind: string }> = {
  serviceAccounts: { label: 'forms.serviceAccounts', kind: 'ServiceAccount' },
  deployments: { label: 'forms.deployments', kind: 'Deployment' },
  statefulSets: { label: 'forms.statefulSets', kind: 'StatefulSet' },
  daemonSets: { label: 'forms.daemonSets', kind: 'DaemonSet' },
  services: { label: 'forms.services', kind: 'Service' },
  ingresses: { label: 'forms.ingresses', kind: 'Ingress' },
  configMaps: { label: 'forms.configMaps', kind: 'ConfigMap' },
  secrets: { label: 'forms.secrets', kind: 'Secret' },
  persistentVolumeClaims: { label: 'forms.persistentVolumeClaims', kind: 'PersistentVolumeClaim' },
  jobs: { label: 'forms.jobs', kind: 'Job' },
  cronJobs: { label: 'forms.cronJobs', kind: 'CronJob' },
  podDisruptionBudgets: { label: 'forms.podDisruptionBudgets', kind: 'PodDisruptionBudget' },
  networkPolicies: { label: 'forms.networkPolicies', kind: 'NetworkPolicy' },
  horizontalPodAutoscalers: { label: 'forms.horizontalPodAutoscalers', kind: 'HorizontalPodAutoscaler' },
  manifests: { label: 'forms.manifests', kind: 'Manifest' },
}

export function FormDetail() {
  const { name } = useParams<{ name: string }>()
  const [searchParams] = useSearchParams()
  const namespace = searchParams.get('namespace') || undefined
  const { t } = useTranslation()

  const [details, setDetails] = useState<FormDetailsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchData = async () => {
    if (!name) return
    setLoading(true)
    setError(null)
    try {
      const data = await formApi.getDetails(name, namespace)
      setDetails(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('forms.errorLoadingForm'))
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

  if (error || !details) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/forms">
            <IconArrowLeft size={16} className="mr-2" />
            {t('forms.backToForms')}
          </Link>
        </Button>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <IconCircleX size={48} className="text-rose-500" />
            <h3 className="mt-4 text-lg font-medium">{t('forms.errorLoadingForm')}</h3>
            <p className="mt-2 text-sm text-muted-foreground">{error}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const { form, variables } = details
  const isReady = form.status?.conditions?.some(
    (c) => (c.type === 'Ready' || c.type === 'Applied') && c.status === 'True'
  )
  const formSpec = form.spec as LynqForm['spec']

  // Count total resources
  const resourceCounts = Object.entries(RESOURCE_TYPE_LABELS).reduce(
    (acc, [key]) => {
      const resources = (formSpec as unknown as Record<string, TResource[] | undefined>)[key]
      if (resources && resources.length > 0) {
        acc.total += resources.length
        acc.types.push({ key, count: resources.length })
      }
      return acc
    },
    { total: 0, types: [] as { key: string; count: number }[] }
  )

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" asChild>
            <Link to="/forms">
              <IconArrowLeft size={16} />
            </Link>
          </Button>
          <div>
            <div className="flex items-center gap-2">
              <IconFileCode size={20} />
              <h1 className="text-2xl font-bold">{form.metadata.name}</h1>
              <Badge variant={isReady ? 'ready' : 'pending'}>
                {isReady ? t('status.ready') : t('status.pending')}
              </Badge>
            </div>
            <p className="text-muted-foreground">{form.metadata.namespace}</p>
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
              {t('forms.hubReference')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Link
              to={`/hubs/${formSpec.hubId}?namespace=${form.metadata.namespace}`}
              className="flex items-center gap-2 hover:underline"
            >
              <IconDatabase size={16} className="text-muted-foreground" />
              <span className="font-mono">{formSpec.hubId}</span>
            </Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('forms.totalNodes')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconBox size={16} className="text-blue-500" />
              <span className="text-2xl font-bold">
                {form.status?.totalNodes || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('status.ready')} {t('nav.nodes')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleCheck size={16} className="text-emerald-500" />
              <span className="text-2xl font-bold text-emerald-500">
                {form.status?.readyNodes || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('status.failed')} {t('nav.nodes')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleX size={16} className="text-rose-500" />
              <span className="text-2xl font-bold text-rose-500">
                {form.status?.failedNodes || 0}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Rollout Progress */}
      {form.status?.rollout?.inProgress && (
        <Card>
          <CardHeader>
            <CardTitle>{t('forms.rollout')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span>
                  {form.status.rollout.updatedNodes} / {form.status.rollout.totalNodes} {t('forms.nodesUpdated')}
                </span>
                <span>{form.status.rollout.percentage}%</span>
              </div>
              <div className="h-2 bg-muted rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary transition-all"
                  style={{ width: `${form.status.rollout.percentage}%` }}
                />
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Template Variables */}
        <Card>
          <CardHeader>
            <CardTitle>{t('drawer.availableVariables')} ({variables.length})</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {variables.map((v) => (
                <Badge key={v} variant="secondary" className="font-mono">
                  .{v}
                </Badge>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Resource Summary */}
        <Card>
          <CardHeader>
            <CardTitle>{t('forms.resourceTypes')} ({resourceCounts.total})</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {resourceCounts.types.map(({ key, count }) => (
                <Badge key={key} variant="outline">
                  {t(RESOURCE_TYPE_LABELS[key]?.label) || key}: {count}
                </Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Conditions */}
      {form.status?.conditions && form.status.conditions.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t('hubs.conditions')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {form.status.conditions.map((condition, i) => (
              <ConditionBadge key={i} condition={condition} />
            ))}
          </CardContent>
        </Card>
      )}

      {/* Resource Templates */}
      <Card>
        <CardHeader>
          <CardTitle>{t('drawer.resourceTemplates')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          {Object.entries(RESOURCE_TYPE_LABELS).map(([key, { label, kind }]) => {
            const resources = (formSpec as unknown as Record<string, TResource[] | undefined>)[key]
            if (!resources || resources.length === 0) return null

            return (
              <div key={key}>
                <h4 className="font-medium mb-3 flex items-center gap-2">
                  {t(label)}
                  <Badge variant="secondary">{resources.length}</Badge>
                </h4>
                <div className="space-y-2">
                  {resources.map((resource) => (
                    <TResourceCard key={resource.id} resource={resource} kind={kind} />
                  ))}
                </div>
                <Separator className="mt-4" />
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}
