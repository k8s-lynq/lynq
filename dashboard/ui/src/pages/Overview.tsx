import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  IconDatabase,
  IconFileCode,
  IconBox,
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconChevronRight,
  IconActivity,
  IconArrowRight,
} from '@tabler/icons-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart'
import { PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, CartesianGrid } from 'recharts'
import { useHubs, useHubPolling } from '@/contexts/HubContext'
import { useForms, useFormPolling } from '@/contexts/FormContext'
import { useNodes, useNodePolling } from '@/contexts/NodeContext'

function KPICard({
  title,
  icon: Icon,
  total,
  ready,
  failed,
  loading,
  href,
}: {
  title: string
  icon: React.ComponentType<{ size?: number; className?: string }>
  total: number
  ready: number
  failed: number
  loading?: boolean
  href: string
}) {
  const navigate = useNavigate()
  const pending = total - ready - failed

  return (
    <Card
      className="cursor-pointer transition-all hover:shadow-md hover:border-primary/50 group"
      onClick={() => navigate(href)}
    >
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <div className="flex items-center gap-1">
          <Icon size={16} className="text-muted-foreground" />
          <IconChevronRight size={14} className="text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex items-center gap-2">
            <IconClock size={16} className="animate-spin text-muted-foreground" />
            <span className="text-muted-foreground">Loading...</span>
          </div>
        ) : (
          <>
            <div className="text-2xl font-bold">{total}</div>
            <div className="mt-2 flex gap-2">
              <Badge variant="ready" className="flex items-center gap-1">
                <IconCircleCheck size={12} />
                {ready}
              </Badge>
              {failed > 0 && (
                <Badge variant="failed" className="flex items-center gap-1">
                  <IconCircleX size={12} />
                  {failed}
                </Badge>
              )}
              {pending > 0 && (
                <Badge variant="pending" className="flex items-center gap-1">
                  <IconClock size={12} />
                  {pending}
                </Badge>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

// Chart config is created inside the component to use translations
function useStatusChartConfig() {
  const { t } = useTranslation()
  return {
    ready: {
      label: t('charts.ready'),
      color: "hsl(var(--chart-1))",
    },
    failed: {
      label: t('charts.failed'),
      color: "hsl(var(--chart-2))",
    },
    pending: {
      label: t('charts.pending'),
      color: "hsl(var(--chart-3))",
    },
  } satisfies ChartConfig
}


function StatusPieChart({
  ready,
  failed,
  pending,
  emptyMessage,
}: {
  ready: number
  failed: number
  pending: number
  emptyMessage: string
}) {
  const statusChartConfig = useStatusChartConfig()
  const total = ready + failed + pending
  if (total === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-[160px] text-muted-foreground">
        <IconActivity size={32} className="opacity-50 mb-2" />
        <span className="text-sm">{emptyMessage}</span>
      </div>
    )
  }

  const data = [
    { name: 'ready', value: ready, fill: 'hsl(142 76% 36%)' },
    { name: 'failed', value: failed, fill: 'hsl(0 84% 60%)' },
    { name: 'pending', value: pending, fill: 'hsl(45 93% 47%)' },
  ].filter(d => d.value > 0)

  return (
    <ChartContainer config={statusChartConfig} className="h-[160px] w-full">
      <PieChart>
        <ChartTooltip content={<ChartTooltipContent hideLabel />} />
        <Pie
          data={data}
          dataKey="value"
          nameKey="name"
          cx="50%"
          cy="50%"
          innerRadius={40}
          outerRadius={60}
          strokeWidth={2}
          stroke="hsl(var(--background))"
        >
          {data.map((entry, index) => (
            <Cell key={`cell-${index}`} fill={entry.fill} />
          ))}
        </Pie>
      </PieChart>
    </ChartContainer>
  )
}

function HealthOverviewChart({ data }: { data: { name: string; ready: number; failed: number; pending: number }[] }) {
  const statusChartConfig = useStatusChartConfig()
  return (
    <ChartContainer config={statusChartConfig} className="h-[200px] w-full">
      <BarChart data={data} layout="vertical">
        <CartesianGrid strokeDasharray="3 3" horizontal={true} vertical={false} />
        <XAxis type="number" />
        <YAxis type="category" dataKey="name" width={70} />
        <ChartTooltip content={<ChartTooltipContent />} />
        <Bar dataKey="ready" stackId="a" fill="hsl(142 76% 36%)" radius={[0, 0, 0, 0]} />
        <Bar dataKey="failed" stackId="a" fill="hsl(0 84% 60%)" radius={[0, 0, 0, 0]} />
        <Bar dataKey="pending" stackId="a" fill="hsl(45 93% 47%)" radius={[0, 4, 4, 0]} />
      </BarChart>
    </ChartContainer>
  )
}

function getStatus(conditions?: { type: string; status: string }[]): 'ready' | 'failed' | 'pending' {
  if (!conditions) return 'pending'
  const readyCondition = conditions.find(
    (c) => c.type === 'Ready' || c.type === 'Applied'
  )
  if (!readyCondition) return 'pending'
  if (readyCondition.status === 'True') return 'ready'
  if (readyCondition.status === 'False') return 'failed'
  return 'pending'
}

function RecentItemsList<T extends { metadata: { name: string; namespace: string; creationTimestamp?: string }; status?: { conditions?: { type: string; status: string }[] } }>({
  items,
  titleKey,
  icon: Icon,
  basePath,
  emptyMessage,
}: {
  items: T[]
  titleKey: string
  icon: React.ComponentType<{ size?: number; className?: string }>
  basePath: string
  emptyMessage: string
}) {
  const navigate = useNavigate()
  const { t } = useTranslation()

  // Sort by creation timestamp (newest first) and take top 5
  const recentItems = [...items]
    .sort((a, b) => {
      const aTime = a.metadata.creationTimestamp ? new Date(a.metadata.creationTimestamp).getTime() : 0
      const bTime = b.metadata.creationTimestamp ? new Date(b.metadata.creationTimestamp).getTime() : 0
      return bTime - aTime
    })
    .slice(0, 5)

  const statusIcons = {
    ready: <IconCircleCheck size={14} className="text-emerald-500" />,
    failed: <IconCircleX size={14} className="text-rose-500" />,
    pending: <IconClock size={14} className="text-amber-500" />,
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base flex items-center gap-2">
            <Icon size={18} className="text-muted-foreground" />
            {t(titleKey)}
          </CardTitle>
          <Button
            variant="ghost"
            size="sm"
            className="text-xs"
            onClick={() => navigate(`/${basePath}`)}
          >
            {t('common.viewAll')}
            <IconArrowRight size={14} className="ml-1" />
          </Button>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {recentItems.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4 text-center">
            {emptyMessage}
          </p>
        ) : (
          <div className="space-y-2">
            {recentItems.map((item) => {
              const status = getStatus(item.status?.conditions)
              return (
                <div
                  key={`${item.metadata.namespace}-${item.metadata.name}`}
                  className="flex items-center justify-between p-2 rounded-lg hover:bg-muted/50 cursor-pointer transition-colors"
                  onClick={() => navigate(`/${basePath}/${item.metadata.name}?namespace=${item.metadata.namespace}`)}
                >
                  <div className="flex items-center gap-2 min-w-0 flex-1">
                    {statusIcons[status]}
                    <span className="font-medium truncate">{item.metadata.name}</span>
                  </div>
                  <Badge variant="outline" className="text-xs shrink-0 ml-2">
                    {item.metadata.namespace}
                  </Badge>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function Overview() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { hubs, loading: hubsLoading } = useHubs()
  const { forms, loading: formsLoading } = useForms()
  const { nodes, loading: nodesLoading } = useNodes()

  // Enable polling
  useHubPolling()
  useFormPolling()
  useNodePolling()

  // Calculate hub stats
  const hubStats = {
    total: hubs.length,
    ready: hubs.filter(
      (h) => h.status?.conditions?.some((c) => c.type === 'Ready' && c.status === 'True')
    ).length,
    failed: hubs.filter(
      (h) => h.status?.conditions?.some((c) => c.type === 'Ready' && c.status === 'False')
    ).length,
  }

  // Calculate form stats (LynqForm uses 'Applied' condition instead of 'Ready')
  const formStats = {
    total: forms.length,
    ready: forms.filter(
      (f) => f.status?.conditions?.some((c) =>
        (c.type === 'Ready' || c.type === 'Applied') && c.status === 'True'
      )
    ).length,
    failed: forms.filter(
      (f) => f.status?.conditions?.some((c) =>
        (c.type === 'Ready' || c.type === 'Applied') && c.status === 'False'
      )
    ).length,
  }

  // Calculate node stats
  const nodeStats = {
    total: nodes.length,
    ready: nodes.filter(
      (n) => n.status?.conditions?.some((c) => c.type === 'Ready' && c.status === 'True')
    ).length,
    failed: nodes.filter(
      (n) => n.status?.conditions?.some((c) => c.type === 'Ready' && c.status === 'False')
    ).length,
  }

  // Calculate resource stats from nodes
  const resourceStats = nodes.reduce(
    (acc, node) => {
      acc.desired += node.status?.desiredResources || 0
      acc.ready += node.status?.readyResources || 0
      acc.failed += node.status?.failedResources || 0
      return acc
    },
    { desired: 0, ready: 0, failed: 0 }
  )

  // Data for health overview bar chart
  const healthData = [
    {
      name: t('nav.hubs'),
      ready: hubStats.ready,
      failed: hubStats.failed,
      pending: hubStats.total - hubStats.ready - hubStats.failed,
    },
    {
      name: t('nav.forms'),
      ready: formStats.ready,
      failed: formStats.failed,
      pending: formStats.total - formStats.ready - formStats.failed,
    },
    {
      name: t('nav.nodes'),
      ready: nodeStats.ready,
      failed: nodeStats.failed,
      pending: nodeStats.total - nodeStats.ready - nodeStats.failed,
    },
    {
      name: t('overview.resources'),
      ready: resourceStats.ready,
      failed: resourceStats.failed,
      pending: resourceStats.desired - resourceStats.ready - resourceStats.failed,
    },
  ]

  const isLoading = hubsLoading || formsLoading || nodesLoading

  return (
    <div className="space-y-6">
      {/* KPI Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <KPICard
          title={t('nav.hubs')}
          icon={IconDatabase}
          total={hubStats.total}
          ready={hubStats.ready}
          failed={hubStats.failed}
          loading={hubsLoading}
          href="/hubs"
        />
        <KPICard
          title={t('nav.forms')}
          icon={IconFileCode}
          total={formStats.total}
          ready={formStats.ready}
          failed={formStats.failed}
          loading={formsLoading}
          href="/forms"
        />
        <KPICard
          title={t('nav.nodes')}
          icon={IconBox}
          total={nodeStats.total}
          ready={nodeStats.ready}
          failed={nodeStats.failed}
          loading={nodesLoading}
          href="/nodes"
        />
        <KPICard
          title={t('overview.resources')}
          icon={IconActivity}
          total={resourceStats.desired}
          ready={resourceStats.ready}
          failed={resourceStats.failed}
          loading={nodesLoading}
          href="/nodes"
        />
      </div>

      {/* Charts Section */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {/* Health Overview Bar Chart */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <IconActivity size={18} />
              {t('overview.healthOverview')}
            </CardTitle>
            <CardDescription>{t('overview.healthDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="h-[200px] flex items-center justify-center">
                <IconClock size={24} className="animate-spin text-muted-foreground" />
              </div>
            ) : (
              <HealthOverviewChart data={healthData} />
            )}
          </CardContent>
        </Card>

        {/* Node Status Pie Chart */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <IconBox size={18} />
              {t('overview.nodeStatus')}
            </CardTitle>
            <CardDescription>{t('overview.nodeStatusDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            {nodesLoading ? (
              <div className="h-[160px] flex items-center justify-center">
                <IconClock size={24} className="animate-spin text-muted-foreground" />
              </div>
            ) : (
              <>
                <StatusPieChart
                  ready={nodeStats.ready}
                  failed={nodeStats.failed}
                  pending={nodeStats.total - nodeStats.ready - nodeStats.failed}
                  emptyMessage={t('overview.noNodes')}
                />
                <div className="flex justify-center gap-4 mt-2">
                  <div className="flex items-center gap-1.5">
                    <div className="w-2.5 h-2.5 rounded-full bg-emerald-500" />
                    <span className="text-xs text-muted-foreground">{t('charts.ready')}</span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <div className="w-2.5 h-2.5 rounded-full bg-rose-500" />
                    <span className="text-xs text-muted-foreground">{t('charts.failed')}</span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <div className="w-2.5 h-2.5 rounded-full bg-amber-500" />
                    <span className="text-xs text-muted-foreground">{t('charts.pending')}</span>
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent Items */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <RecentItemsList
          items={hubs}
          titleKey="overview.recentHubs"
          icon={IconDatabase}
          basePath="hubs"
          emptyMessage={t('overview.noHubs')}
        />
        <RecentItemsList
          items={forms}
          titleKey="overview.recentForms"
          icon={IconFileCode}
          basePath="forms"
          emptyMessage={t('overview.noForms')}
        />
        <RecentItemsList
          items={nodes}
          titleKey="overview.recentNodes"
          icon={IconBox}
          basePath="nodes"
          emptyMessage={t('overview.noNodes')}
        />
      </div>

      {/* Quick Actions */}
      <Card>
        <CardHeader>
          <CardTitle>{t('overview.quickActions')}</CardTitle>
          <CardDescription>{t('overview.quickActionsDescription')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => navigate('/topology')}>
              {t('overview.viewTopology')}
              <IconArrowRight size={14} className="ml-2" />
            </Button>
            <Button variant="outline" onClick={() => navigate('/hubs')}>
              {t('overview.manageHubs')}
              <IconArrowRight size={14} className="ml-2" />
            </Button>
            <Button variant="outline" onClick={() => navigate('/forms')}>
              {t('overview.manageForms')}
              <IconArrowRight size={14} className="ml-2" />
            </Button>
            <Button variant="outline" onClick={() => navigate('/nodes')}>
              {t('overview.viewAllNodes')}
              <IconArrowRight size={14} className="ml-2" />
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
