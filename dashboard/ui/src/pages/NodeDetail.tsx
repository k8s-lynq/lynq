import { useEffect, useState } from "react";
import { useParams, useSearchParams, Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  IconBox,
  IconArrowLeft,
  IconRefresh,
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconCircleMinus,
  IconDatabase,
  IconFileCode,
  IconCopy,
  IconCheck,
  IconVariable,
  IconAlertTriangle,
} from "@tabler/icons-react";
// Note: Clock is used in ConditionBadge for Unknown status
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { nodeApi } from "@/lib/api";
import type { LynqNode, Condition } from "@/types/lynq";

function ConditionBadge({ condition }: { condition: Condition }) {
  const isTrue = condition.status === "True";
  const isFalse = condition.status === "False";

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
          <Badge variant={isTrue ? "ready" : isFalse ? "failed" : "pending"}>
            {condition.status}
          </Badge>
        </div>
        {condition.reason && (
          <p className="text-xs text-muted-foreground mt-0.5">
            {condition.reason}
          </p>
        )}
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
  );
}

interface AppliedResource {
  kind: string;
  namespace: string;
  name: string;
  id: string;
  isSkipped: boolean; // Skipped due to dependency failure
}

function parseAppliedResource(
  s: string,
  skippedIds: Set<string>
): AppliedResource | null {
  // Format: "Kind/Namespace/Name@ID"
  const atIdx = s.lastIndexOf("@");
  let id = "";
  let rest = s;

  if (atIdx >= 0) {
    rest = s.substring(0, atIdx);
    id = s.substring(atIdx + 1);
  }

  const parts = rest.split("/");
  if (parts.length >= 3) {
    return {
      kind: parts[0],
      namespace: parts[1],
      name: parts.slice(2).join("/"),
      id,
      isSkipped: skippedIds.has(id), // Only true if explicitly in skippedResourceIds
    };
  }
  return null;
}

function ResourceRow({
  resource,
  showSkippedWarning = false,
}: {
  resource: AppliedResource;
  showSkippedWarning?: boolean;
}) {
  // Resources in appliedResources are being actively managed/reconciled
  // isSkipped = true means skipped due to dependency failure (NOT because of K8s Ready status)
  const icon = resource.isSkipped ? (
    <IconCircleMinus size={16} className="text-slate-400" />
  ) : (
    <IconCircleCheck size={16} className="text-emerald-500" />
  );

  return (
    <div
      className={`flex items-center gap-3 p-3 rounded-lg border hover:bg-muted/50 transition-colors ${
        resource.isSkipped && showSkippedWarning
          ? "border-amber-300 bg-amber-50/30 dark:bg-amber-950/20"
          : ""
      }`}
    >
      {icon}
      <Badge variant="outline" className="shrink-0">
        {resource.kind}
      </Badge>
      <div className="flex-1 min-w-0">
        <p className="font-medium text-sm truncate">{resource.name}</p>
        <p className="text-xs text-muted-foreground">{resource.namespace}</p>
      </div>
      {resource.id && (
        <Badge variant="secondary" className="font-mono text-xs shrink-0">
          {resource.id}
        </Badge>
      )}
    </div>
  );
}

export function NodeDetail() {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const namespace = searchParams.get("namespace") || undefined;
  const { t } = useTranslation();

  const [node, setNode] = useState<LynqNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const fetchData = async () => {
    if (!name) return;
    setLoading(true);
    setError(null);
    try {
      const data = await nodeApi.get(name, namespace);
      setNode(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t("nodes.errorLoadingNode")
      );
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [name, namespace]);

  const handleCopyData = async () => {
    if (!node) return;
    const data = (node.spec as { data?: Record<string, unknown> })?.data || {};
    await navigator.clipboard.writeText(JSON.stringify(data, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <IconRefresh size={32} className="animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !node) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/nodes">
            <IconArrowLeft size={16} className="mr-2" />
            {t("nodes.backToNodes")}
          </Link>
        </Button>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <IconCircleX size={48} className="text-rose-500" />
            <h3 className="mt-4 text-lg font-medium">
              {t("nodes.errorLoadingNode")}
            </h3>
            <p className="mt-2 text-sm text-muted-foreground">{error}</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  const isReady = node.status?.conditions?.some(
    (c) => c.type === "Ready" && c.status === "True"
  );
  // hubRef is stored in labels, not spec
  const hubRef = node.metadata.labels?.["lynq.sh/hub"];
  const nodeSpec = node.spec;
  // Only trust skippedResourceIds if skippedResources count is actually set and > 0
  // This works around a controller bug where Service/Ingress IDs are incorrectly added to skippedResourceIds
  const hasActualSkippedResources = (node.status?.skippedResources ?? 0) > 0;
  const skippedIds = hasActualSkippedResources
    ? new Set(node.status?.skippedResourceIds || [])
    : new Set<string>();

  // Parse applied resources
  // Note: Resources in appliedResources are ALL actively managed (reconciliation targets)
  const appliedResources = (node.status?.appliedResources || [])
    .map((r) => parseAppliedResource(r, skippedIds))
    .filter((r): r is AppliedResource => r !== null);

  // Group: All resources in appliedResources are managed, unless explicitly skipped (with valid count)
  const managedResources = appliedResources.filter((r) => !r.isSkipped);
  const skippedResources = appliedResources.filter((r) => r.isSkipped);

  // Status breakdown for chart
  const statusData = [
    {
      label: "Ready",
      count: node.status?.readyResources || 0,
      color: "bg-emerald-500",
    },
    {
      label: "Failed",
      count: node.status?.failedResources || 0,
      color: "bg-rose-500",
    },
    {
      label: "Skipped",
      count: node.status?.skippedResources || 0,
      color: "bg-slate-400",
    },
  ];
  const totalResources = node.status?.desiredResources || 0;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" asChild>
            <Link to="/nodes">
              <IconArrowLeft size={16} />
            </Link>
          </Button>
          <div>
            <div className="flex items-center gap-2">
              <IconBox size={20} />
              <h1 className="text-2xl font-bold">{node.metadata.name}</h1>
              <Badge variant={isReady ? "ready" : "pending"}>
                {isReady ? t("status.ready") : t("status.pending")}
              </Badge>
            </div>
            <p className="text-muted-foreground">{node.metadata.namespace}</p>
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={fetchData}
          disabled={loading}
        >
          <IconRefresh
            size={16}
            className={`mr-2 ${loading ? "animate-spin" : ""}`}
          />
          {t("common.refresh")}
        </Button>
      </div>

      {/* References */}
      <div className="flex gap-4">
        {hubRef && (
          <Link
            to={`/hubs/${hubRef}?namespace=${node.metadata.namespace}`}
            className="flex items-center gap-2 px-3 py-2 rounded-lg border hover:bg-muted transition-colors"
          >
            <IconDatabase size={16} className="text-muted-foreground" />
            <span className="text-sm">{t("filters.hub")}: </span>
            <span className="font-mono text-sm">{hubRef}</span>
          </Link>
        )}
        <Link
          to={`/forms/${node.spec.templateRef}?namespace=${node.metadata.namespace}`}
          className="flex items-center gap-2 px-3 py-2 rounded-lg border hover:bg-muted transition-colors"
        >
          <IconFileCode size={16} className="text-muted-foreground" />
          <span className="text-sm">{t("nodes.form")}: </span>
          <span className="font-mono text-sm">{node.spec.templateRef}</span>
        </Link>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("hubs.desired")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold text-blue-500">
              {node.status?.desiredResources || 0}
            </span>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("status.ready")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleCheck size={16} className="text-emerald-500" />
              <span className="text-2xl font-bold text-emerald-500">
                {node.status?.readyResources || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("status.failed")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleX size={16} className="text-rose-500" />
              <span className="text-2xl font-bold text-rose-500">
                {node.status?.failedResources || 0}
              </span>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("status.skipped")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <IconCircleMinus size={16} className="text-slate-400" />
              <span className="text-2xl font-bold text-slate-400">
                {node.status?.skippedResources || 0}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Status Progress Bar */}
      {totalResources > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">
              {t("nodes.resourceStatus")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-3 rounded-full overflow-hidden flex bg-muted">
              {statusData.map((item) => {
                const width = (item.count / totalResources) * 100;
                if (width === 0) return null;
                return (
                  <div
                    key={item.label}
                    className={`${item.color} transition-all`}
                    style={{ width: `${width}%` }}
                    title={`${item.label}: ${item.count}`}
                  />
                );
              })}
            </div>
            <div className="flex justify-between mt-2 text-xs text-muted-foreground">
              {statusData.map((item) => (
                <span key={item.label}>
                  {item.label}: {item.count}
                </span>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Tabs */}
      <Tabs defaultValue="resources">
        <TabsList>
          <TabsTrigger value="resources">{t("drawer.resources")}</TabsTrigger>
          <TabsTrigger value="data">
            {t("drawer.templateVariables")}
          </TabsTrigger>
          <TabsTrigger value="conditions">{t("hubs.conditions")}</TabsTrigger>
        </TabsList>

        <TabsContent value="resources" className="space-y-4">
          {/* Info about resource status */}
          <div className="text-xs text-muted-foreground p-3 bg-muted/50 rounded-lg">
            <p>
              <strong>{t("nodes.managedColon")}</strong>{" "}
              {t("nodes.resourcesActivelyReconciled")}
            </p>
            <p>
              <strong>{t("nodes.skippedColon")}</strong>{" "}
              {t("nodes.resourcesSkippedDueToDependencyFailure")}
            </p>
            <p className="mt-1 italic">
              {t("nodes.statusReflectsReconciliationState")}
            </p>
          </div>

          {/* Managed Resources (active reconciliation targets) */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <IconCircleCheck size={16} className="text-emerald-500" />
                {t("drawer.managedResources")} ({managedResources.length})
              </CardTitle>
            </CardHeader>
            <CardContent>
              {managedResources.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">
                  {t("nodes.noResourcesAppliedYet")}
                </p>
              ) : (
                <div className="space-y-2">
                  {managedResources.map((resource, i) => (
                    <ResourceRow key={i} resource={resource} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Skipped Resources (due to dependency failure) */}
          {skippedResources.length > 0 && (
            <Card className="border-amber-200 dark:border-amber-800">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <IconCircleMinus size={16} className="text-slate-400" />
                  {t("drawer.skippedResources")} ({skippedResources.length})
                  <span className="text-xs font-normal text-muted-foreground ml-2">
                    ({t("nodes.dependencyFailure")})
                  </span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {skippedResources.map((resource, i) => (
                    <ResourceRow
                      key={i}
                      resource={resource}
                      showSkippedWarning
                    />
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Failed hint */}
          {(node.status?.failedResources || 0) > 0 && (
            <Card className="border-rose-200 dark:border-rose-900">
              <CardContent className="flex items-start gap-3 py-4">
                <IconAlertTriangle
                  size={20}
                  className="text-rose-500 shrink-0 mt-0.5"
                />
                <div>
                  <p className="font-medium text-rose-500">
                    {t("nodes.resourcesFailed", {
                      count: node.status?.failedResources,
                    })}
                  </p>
                  <p className="text-sm text-muted-foreground mt-1">
                    {t("nodes.checkConditionsTab")}
                  </p>
                </div>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="data">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center justify-between">
                <span className="flex items-center gap-2">
                  <IconVariable size={16} />
                  {t("drawer.templateVariables")}
                </span>
                <Button variant="ghost" size="sm" onClick={handleCopyData}>
                  {copied ? (
                    <IconCheck size={12} className="mr-1 text-emerald-500" />
                  ) : (
                    <IconCopy size={12} className="mr-1" />
                  )}
                  {copied ? t("common.copied") : t("common.copy")}
                </Button>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {nodeSpec.data && Object.keys(nodeSpec.data).length > 0 ? (
                <pre className="text-sm font-mono bg-muted p-4 rounded-lg overflow-x-auto">
                  {JSON.stringify(nodeSpec.data, null, 2)}
                </pre>
              ) : (
                <p className="text-sm text-muted-foreground text-center py-8">
                  {t("drawer.noTemplateData")}
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="conditions">
          <Card>
            <CardHeader>
              <CardTitle>Conditions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {node.status?.conditions && node.status.conditions.length > 0 ? (
                node.status.conditions.map((condition, i) => (
                  <ConditionBadge key={i} condition={condition} />
                ))
              ) : (
                <p className="text-sm text-muted-foreground text-center py-8">
                  {t("nodes.noConditionsReported")}
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
