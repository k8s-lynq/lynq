import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  IconBox,
  IconExternalLink,
  IconRefresh,
  IconCircleCheck,
  IconCircleX,
  IconAlertCircle,
  IconSearch,
  IconFilter,
  IconX,
} from '@tabler/icons-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useNodes, useNodePolling } from '@/contexts/NodeContext'
import type { LynqNode, ResourceStatus } from '@/types/lynq'

function getNodeStatus(node: LynqNode): ResourceStatus {
  const readyCondition = node.status?.conditions?.find((c) => c.type === 'Ready')
  if (!readyCondition) return 'pending'
  if (readyCondition.status === 'True') return 'ready'
  if (readyCondition.status === 'False') return 'failed'
  return 'pending'
}

function StatusIcon({ status }: { status: ResourceStatus }) {
  switch (status) {
    case 'ready':
      return <IconCircleCheck size={16} className="text-emerald-500" />
    case 'failed':
      return <IconCircleX size={16} className="text-rose-500" />
    case 'pending':
      return <IconAlertCircle size={16} className="text-amber-500" />
    default:
      return <IconAlertCircle size={16} className="text-slate-400" />
  }
}

function NodeCard({ node }: { node: LynqNode }) {
  const { t } = useTranslation()
  const status = getNodeStatus(node)
  const uid = node.spec.data?.uid as string | undefined
  const hubRef = node.metadata.labels?.['lynq.sh/hub']

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            <IconBox size={16} />
            {node.metadata.name}
          </CardTitle>
          <p className="text-xs text-muted-foreground">
            {node.metadata.namespace}
          </p>
        </div>
        <Badge variant={status}>
          <StatusIcon status={status} />
          <span className="ml-1 capitalize">{t(`status.${status}`)}</span>
        </Badge>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {/* UID */}
          {uid && (
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">{t('nodes.uid')}</span>
              <span className="font-mono text-xs">{uid}</span>
            </div>
          )}

          {/* References */}
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">{t('filters.hub')}</span>
            <span>{hubRef || '-'}</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">{t('nodes.form')}</span>
            <span>{node.spec.templateRef}</span>
          </div>

          {/* Resource Stats */}
          <div className="grid grid-cols-4 gap-2 pt-2 text-center">
            <div>
              <p className="text-lg font-bold">
                {node.status?.desiredResources || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('hubs.desired')}</p>
            </div>
            <div>
              <p className="text-lg font-bold text-emerald-500">
                {node.status?.readyResources || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.ready')}</p>
            </div>
            <div>
              <p className="text-lg font-bold text-rose-500">
                {node.status?.failedResources || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.failed')}</p>
            </div>
            <div>
              <p className="text-lg font-bold text-slate-400">
                {node.status?.skippedResources || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.skipped')}</p>
            </div>
          </div>

          {/* Actions */}
          <Button variant="outline" size="sm" className="w-full" asChild>
            <Link to={`/nodes/${node.metadata.name}?namespace=${node.metadata.namespace}`}>
              <IconExternalLink size={12} className="mr-2" />
              {t('common.details')}
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

export function Nodes() {
  const { t } = useTranslation()
  const { nodes, loading, fetchNodes } = useNodes()
  const [searchParams, setSearchParams] = useSearchParams()

  // Get filter values from URL params
  const searchQuery = searchParams.get('search') || ''
  const statusFilter = searchParams.get('status') || 'all'
  const hubFilter = searchParams.get('hub') || 'all'
  const formFilter = searchParams.get('form') || 'all'
  const namespaceFilter = searchParams.get('namespace') || 'all'

  // Enable polling
  useNodePolling()

  // Get unique values for filter dropdowns
  const { hubs, forms, namespaces } = useMemo(() => {
    const hubSet = new Set<string>()
    const formSet = new Set<string>()
    const nsSet = new Set<string>()

    nodes.forEach((node) => {
      // hubRef is stored in labels, not spec
      const hubRef = node.metadata.labels?.['lynq.sh/hub']
      if (hubRef) hubSet.add(hubRef)
      if (node.spec.templateRef) formSet.add(node.spec.templateRef)
      if (node.metadata.namespace) nsSet.add(node.metadata.namespace)
    })

    return {
      hubs: Array.from(hubSet).sort(),
      forms: Array.from(formSet).sort(),
      namespaces: Array.from(nsSet).sort(),
    }
  }, [nodes])

  // Filter nodes
  const filteredNodes = useMemo(() => {
    return nodes.filter((node) => {
      // Search filter
      if (searchQuery) {
        const query = searchQuery.toLowerCase()
        const matchesName = node.metadata.name.toLowerCase().includes(query)
        const matchesUid = (node.spec.data?.uid as string)?.toLowerCase().includes(query)
        if (!matchesName && !matchesUid) return false
      }

      // Status filter
      if (statusFilter !== 'all') {
        const status = getNodeStatus(node)
        if (status !== statusFilter) return false
      }

      // Hub filter (hubRef is in labels)
      const nodeHubRef = node.metadata.labels?.['lynq.sh/hub']
      if (hubFilter !== 'all' && nodeHubRef !== hubFilter) {
        return false
      }

      // Form filter
      if (formFilter !== 'all' && node.spec.templateRef !== formFilter) {
        return false
      }

      // Namespace filter
      if (namespaceFilter !== 'all' && node.metadata.namespace !== namespaceFilter) {
        return false
      }

      return true
    })
  }, [nodes, searchQuery, statusFilter, hubFilter, formFilter, namespaceFilter])

  // Update URL params
  const updateFilter = (key: string, value: string) => {
    const newParams = new URLSearchParams(searchParams)
    if (value === 'all' || value === '') {
      newParams.delete(key)
    } else {
      newParams.set(key, value)
    }
    setSearchParams(newParams)
  }

  // Clear all filters
  const clearFilters = () => {
    setSearchParams(new URLSearchParams())
  }

  const hasActiveFilters = searchQuery || statusFilter !== 'all' || hubFilter !== 'all' || formFilter !== 'all' || namespaceFilter !== 'all'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">{t('nodes.title')}</h2>
          <p className="text-muted-foreground">
            {t('nodes.description')}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => fetchNodes()}
          disabled={loading}
        >
          <IconRefresh size={16} className={`mr-2 ${loading ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </Button>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-2">
              <IconFilter size={16} className="text-muted-foreground" />
              <span className="text-sm font-medium">{t('common.filter')}</span>
              {hasActiveFilters && (
                <Button variant="ghost" size="sm" onClick={clearFilters} className="h-6 px-2 text-xs">
                  <IconX size={12} className="mr-1" />
                  {t('common.clearAll')}
                </Button>
              )}
            </div>
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
              {/* Search */}
              <div className="relative">
                <IconSearch size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t('nodes.searchByNameOrUid')}
                  value={searchQuery}
                  onChange={(e) => updateFilter('search', e.target.value)}
                  className="pl-8 h-9"
                />
              </div>

              {/* Status filter */}
              <Select value={statusFilter} onValueChange={(v) => updateFilter('status', v)}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t('filters.status')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('status.all')}</SelectItem>
                  <SelectItem value="ready">{t('status.ready')}</SelectItem>
                  <SelectItem value="failed">{t('status.failed')}</SelectItem>
                  <SelectItem value="pending">{t('status.pending')}</SelectItem>
                </SelectContent>
              </Select>

              {/* Hub filter */}
              <Select value={hubFilter} onValueChange={(v) => updateFilter('hub', v)}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t('filters.hub')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('nodes.allHubs')}</SelectItem>
                  {hubs.map((hub) => (
                    <SelectItem key={hub} value={hub}>{hub}</SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Form filter */}
              <Select value={formFilter} onValueChange={(v) => updateFilter('form', v)}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t('filters.form')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('nodes.allForms')}</SelectItem>
                  {forms.map((form) => (
                    <SelectItem key={form} value={form}>{form}</SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Namespace filter */}
              <Select value={namespaceFilter} onValueChange={(v) => updateFilter('namespace', v)}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t('filters.namespace')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('nodes.allNamespaces')}</SelectItem>
                  {namespaces.map((ns) => (
                    <SelectItem key={ns} value={ns}>{ns}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Summary Stats */}
      {nodes.length > 0 && (
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold">{filteredNodes.length}</p>
              <p className="text-sm text-muted-foreground">
                {filteredNodes.length !== nodes.length ? `of ${nodes.length} ` : ''}{t('nodes.totalNodes')}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold text-emerald-500">
                {filteredNodes.filter((n) => getNodeStatus(n) === 'ready').length}
              </p>
              <p className="text-sm text-muted-foreground">{t('status.ready')}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold text-rose-500">
                {filteredNodes.filter((n) => getNodeStatus(n) === 'failed').length}
              </p>
              <p className="text-sm text-muted-foreground">{t('status.failed')}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold text-amber-500">
                {filteredNodes.filter((n) => getNodeStatus(n) === 'pending').length}
              </p>
              <p className="text-sm text-muted-foreground">{t('status.pending')}</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Node Grid */}
      {filteredNodes.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <IconBox size={48} className="text-muted-foreground/50" />
            <h3 className="mt-4 text-lg font-medium">
              {hasActiveFilters ? t('nodes.noMatchingNodes') : t('nodes.noNodesFound')}
            </h3>
            <p className="mt-2 text-sm text-muted-foreground">
              {loading
                ? t('nodes.loadingNodes')
                : hasActiveFilters
                  ? t('nodes.adjustFilters')
                  : t('nodes.nodesAutoCreated')}
            </p>
            {hasActiveFilters && (
              <Button variant="outline" size="sm" onClick={clearFilters} className="mt-4">
                {t('nodes.clearFilters')}
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {filteredNodes.map((node) => (
            <NodeCard
              key={`${node.metadata.namespace}/${node.metadata.name}`}
              node={node}
            />
          ))}
        </div>
      )}
    </div>
  )
}
