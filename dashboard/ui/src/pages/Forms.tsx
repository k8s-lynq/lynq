import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  IconFileCode,
  IconExternalLink,
  IconRefresh,
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
import { useForms, useFormPolling } from '@/contexts/FormContext'
import type { LynqForm } from '@/types/lynq'

function getFormStatus(form: LynqForm): 'ready' | 'pending' {
  const isReady = form.status?.conditions?.some(
    (c) => (c.type === 'Ready' || c.type === 'Applied') && c.status === 'True'
  )
  return isReady ? 'ready' : 'pending'
}

function getResourceTypes(form: LynqForm): string[] {
  const types: string[] = []
  if (form.spec.deployments?.length) types.push('Deployment')
  if (form.spec.statefulSets?.length) types.push('StatefulSet')
  if (form.spec.daemonSets?.length) types.push('DaemonSet')
  if (form.spec.services?.length) types.push('Service')
  if (form.spec.ingresses?.length) types.push('Ingress')
  if (form.spec.configMaps?.length) types.push('ConfigMap')
  if (form.spec.secrets?.length) types.push('Secret')
  if (form.spec.jobs?.length) types.push('Job')
  if (form.spec.cronJobs?.length) types.push('CronJob')
  if (form.spec.manifests?.length) types.push('Manifest')
  return types
}

function FormCard({ form }: { form: LynqForm }) {
  const { t } = useTranslation()
  const status = getFormStatus(form)
  const isReady = status === 'ready'
  const resourceTypes = getResourceTypes(form)

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2">
            <IconFileCode size={16} />
            {form.metadata.name}
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            {t('forms.hub')}: {form.spec.hubId}
          </p>
        </div>
        <Badge variant={status}>{isReady ? t('status.ready') : t('status.pending')}</Badge>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* Stats */}
          <div className="grid grid-cols-3 gap-4 text-center">
            <div>
              <p className="text-2xl font-bold">
                {form.status?.totalNodes || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('forms.total')}</p>
            </div>
            <div>
              <p className="text-2xl font-bold text-emerald-500">
                {form.status?.readyNodes || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.ready')}</p>
            </div>
            <div>
              <p className="text-2xl font-bold text-rose-500">
                {form.status?.failedNodes || 0}
              </p>
              <p className="text-xs text-muted-foreground">{t('status.failed')}</p>
            </div>
          </div>

          {/* Resource Types */}
          <div className="flex flex-wrap gap-1">
            {resourceTypes.map((type) => (
              <Badge key={type} variant="secondary" className="text-xs">
                {type}
              </Badge>
            ))}
          </div>

          {/* Rollout Progress */}
          {form.status?.rollout?.inProgress && (
            <div className="space-y-1">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t('forms.rollout')}</span>
                <span>{form.status.rollout.percentage}%</span>
              </div>
              <div className="h-2 rounded-full bg-secondary">
                <div
                  className="h-2 rounded-full bg-primary transition-all"
                  style={{ width: `${form.status.rollout.percentage}%` }}
                />
              </div>
            </div>
          )}

          {/* Actions */}
          <Button variant="outline" size="sm" className="w-full" asChild>
            <Link to={`/forms/${form.metadata.name}?namespace=${form.metadata.namespace}`}>
              <IconExternalLink size={12} className="mr-2" />
              {t('common.details')}
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

export function Forms() {
  const { t } = useTranslation()
  const { forms, loading, fetchForms } = useForms()
  const [searchParams, setSearchParams] = useSearchParams()

  // Get filter values from URL params
  const searchQuery = searchParams.get('search') || ''
  const statusFilter = searchParams.get('status') || 'all'
  const hubFilter = searchParams.get('hub') || 'all'
  const namespaceFilter = searchParams.get('namespace') || 'all'

  // Enable polling
  useFormPolling()

  // Get unique values for filter dropdowns
  const { hubs, namespaces } = useMemo(() => {
    const hubSet = new Set<string>()
    const nsSet = new Set<string>()

    forms.forEach((form) => {
      if (form.spec.hubId) hubSet.add(form.spec.hubId)
      if (form.metadata.namespace) nsSet.add(form.metadata.namespace)
    })

    return {
      hubs: Array.from(hubSet).sort(),
      namespaces: Array.from(nsSet).sort(),
    }
  }, [forms])

  // Filter forms
  const filteredForms = useMemo(() => {
    return forms.filter((form) => {
      // Search filter
      if (searchQuery) {
        const query = searchQuery.toLowerCase()
        const matchesName = form.metadata.name.toLowerCase().includes(query)
        const matchesHub = form.spec.hubId?.toLowerCase().includes(query)
        if (!matchesName && !matchesHub) return false
      }

      // Status filter
      if (statusFilter !== 'all') {
        const status = getFormStatus(form)
        if (status !== statusFilter) return false
      }

      // Hub filter
      if (hubFilter !== 'all' && form.spec.hubId !== hubFilter) {
        return false
      }

      // Namespace filter
      if (namespaceFilter !== 'all' && form.metadata.namespace !== namespaceFilter) {
        return false
      }

      return true
    })
  }, [forms, searchQuery, statusFilter, hubFilter, namespaceFilter])

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

  const hasActiveFilters = searchQuery || statusFilter !== 'all' || hubFilter !== 'all' || namespaceFilter !== 'all'

  // Calculate stats for filtered forms
  const stats = useMemo(() => {
    const readyCount = filteredForms.filter((f) => getFormStatus(f) === 'ready').length
    const pendingCount = filteredForms.filter((f) => getFormStatus(f) === 'pending').length
    const totalNodes = filteredForms.reduce((acc, f) => acc + (f.status?.totalNodes || 0), 0)
    const readyNodes = filteredForms.reduce((acc, f) => acc + (f.status?.readyNodes || 0), 0)
    return { readyCount, pendingCount, totalNodes, readyNodes }
  }, [filteredForms])

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">{t('forms.title')}</h2>
          <p className="text-muted-foreground">{t('forms.description')}</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => fetchForms()}
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
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              {/* Search */}
              <div className="relative">
                <IconSearch size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t('forms.searchByNameOrHub')}
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
                  <SelectItem value="pending">{t('status.pending')}</SelectItem>
                </SelectContent>
              </Select>

              {/* Hub filter */}
              <Select value={hubFilter} onValueChange={(v) => updateFilter('hub', v)}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t('filters.hub')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('forms.allHubs')}</SelectItem>
                  {hubs.map((hub) => (
                    <SelectItem key={hub} value={hub}>{hub}</SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Namespace filter */}
              <Select value={namespaceFilter} onValueChange={(v) => updateFilter('namespace', v)}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t('filters.namespace')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('forms.allNamespaces')}</SelectItem>
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
      {forms.length > 0 && (
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold">{filteredForms.length}</p>
              <p className="text-sm text-muted-foreground">
                {filteredForms.length !== forms.length ? `of ${forms.length} ` : ''}{t('forms.totalForms')}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold text-emerald-500">{stats.readyCount}</p>
              <p className="text-sm text-muted-foreground">{t('status.ready')}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold text-amber-500">{stats.pendingCount}</p>
              <p className="text-sm text-muted-foreground">{t('status.pending')}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6 text-center">
              <p className="text-3xl font-bold">
                <span className="text-emerald-500">{stats.readyNodes}</span>
                <span className="text-muted-foreground text-lg">/{stats.totalNodes}</span>
              </p>
              <p className="text-sm text-muted-foreground">{t('forms.totalNodes')}</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Form Grid */}
      {filteredForms.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <IconFileCode size={48} className="text-muted-foreground/50" />
            <h3 className="mt-4 text-lg font-medium">
              {hasActiveFilters ? t('forms.noMatchingForms') : t('forms.noFormsFound')}
            </h3>
            <p className="mt-2 text-sm text-muted-foreground">
              {loading
                ? t('forms.loadingForms')
                : hasActiveFilters
                  ? t('forms.adjustFilters')
                  : t('forms.createFormToStart')}
            </p>
            {hasActiveFilters && (
              <Button variant="outline" size="sm" onClick={clearFilters} className="mt-4">
                {t('forms.clearFilters')}
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filteredForms.map((form) => (
            <FormCard
              key={`${form.metadata.namespace}/${form.metadata.name}`}
              form={form}
            />
          ))}
        </div>
      )}
    </div>
  )
}
