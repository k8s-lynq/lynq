import { useState, useEffect, useCallback, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type {
  TopologyNode,
  ResourceStatus,
  ResourceMetadata,
} from "@/types/lynq";
import { formApi, nodeApi, type FormDetailsResponse } from "@/lib/api";
import {
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconCircleMinus,
  IconBox,
  IconStack2,
  IconActivity,
  IconRefresh,
  IconCircleDot,
  IconAlertTriangle,
  IconTrash,
  IconArchive,
  IconBolt,
  IconLock,
  IconCode,
  IconChevronDown,
  IconChevronRight,
  IconCopy,
  IconCheck,
  IconLoader2,
  IconVariable,
  IconFileCode,
  IconUnlink,
  IconCalendar,
  IconGripVertical,
  IconWifiOff,
  IconBroadcast,
} from "@tabler/icons-react";
import { useEvents, type ConnectionStatus } from "@/hooks/useEvents";
import type { TFunction } from "i18next";

const MIN_WIDTH = 360;
const MAX_WIDTH = 900;
const DEFAULT_WIDTH = 480;
const STORAGE_KEY = "lynq-drawer-width";

function getStoredWidth(): number {
  if (typeof window === "undefined") return DEFAULT_WIDTH;
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) {
    const parsed = parseInt(stored, 10);
    if (!isNaN(parsed) && parsed >= MIN_WIDTH && parsed <= MAX_WIDTH) {
      return parsed;
    }
  }
  return DEFAULT_WIDTH;
}

interface NodeDetailDrawerProps {
  node: TopologyNode | null;
  allNodes: TopologyNode[];
  open: boolean;
  onClose: () => void;
}

const STATUS_ICONS: Record<ResourceStatus, React.ReactNode> = {
  ready: <IconCircleCheck size={16} className="text-emerald-500" />,
  failed: <IconCircleX size={16} className="text-rose-500" />,
  pending: <IconClock size={16} className="text-amber-500" />,
  skipped: <IconCircleMinus size={16} className="text-slate-400" />,
};

const STATUS_COLORS: Record<ResourceStatus, string> = {
  ready: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20",
  failed: "bg-rose-500/10 text-rose-500 border-rose-500/20",
  pending: "bg-amber-500/10 text-amber-500 border-amber-500/20",
  skipped: "bg-slate-500/10 text-slate-400 border-slate-500/20",
};

export function NodeDetailDrawer({
  node,
  allNodes,
  open,
  onClose,
}: NodeDetailDrawerProps) {
  const [width, setWidth] = useState(getStoredWidth);
  const isResizing = useRef(false);
  const startX = useRef(0);
  const startWidth = useRef(width);
  const { t } = useTranslation();

  const TYPE_LABELS: Record<string, { label: string; icon: React.ReactNode }> =
    {
      hub: { label: t("drawer.lynqHub"), icon: <IconBox size={16} /> },
      form: { label: t("drawer.lynqForm"), icon: <IconStack2 size={16} /> },
      node: { label: t("drawer.lynqNode"), icon: <IconActivity size={16} /> },
      resource: { label: t("drawer.resource"), icon: <IconBox size={16} /> },
      orphan: {
        label: t("drawer.orphaned"),
        icon: <IconAlertTriangle size={16} className="text-amber-500" />,
      },
    };

  // Handle resize drag
  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      isResizing.current = true;
      startX.current = e.clientX;
      startWidth.current = width;
      document.body.style.cursor = "ew-resize";
      document.body.style.userSelect = "none";
    },
    [width]
  );

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing.current) return;
      const delta = startX.current - e.clientX;
      const newWidth = Math.min(
        MAX_WIDTH,
        Math.max(MIN_WIDTH, startWidth.current + delta)
      );
      setWidth(newWidth);
    };

    const handleMouseUp = () => {
      if (isResizing.current) {
        // Save to localStorage when resize ends
        localStorage.setItem(STORAGE_KEY, String(width));
      }
      isResizing.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };

    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);

    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [width]);

  if (!node) return null;

  const typeInfo = TYPE_LABELS[node.type] || { label: node.type, icon: null };

  // Find child nodes based on children IDs
  const childNodes = (node.children || [])
    .map((childId) => allNodes.find((n) => n.id === childId))
    .filter((n): n is TopologyNode => n !== undefined);

  return (
    <Sheet open={open} onOpenChange={(open) => !open && onClose()}>
      <SheetContent
        className="overflow-hidden p-0"
        style={{ width: `${width}px`, maxWidth: "90vw" }}
      >
        {/* Resize handle */}
        <Tooltip>
          <TooltipTrigger asChild>
            <div
              className="absolute left-0 top-0 bottom-0 w-2 cursor-ew-resize hover:bg-primary/10 active:bg-primary/20 flex items-center justify-center group z-10"
              onMouseDown={handleMouseDown}
            >
              <IconGripVertical
                size={24}
                className="text-muted-foreground/30 group-hover:text-muted-foreground/60"
              />
            </div>
          </TooltipTrigger>
          <TooltipContent side="left">
            {t("drawer.dragToResize")}
          </TooltipContent>
        </Tooltip>

        <div className="pl-4 pr-6 py-6 h-full flex flex-col overflow-hidden">
          <SheetHeader className="space-y-1 shrink-0">
            <div className="flex items-center gap-2">
              {typeInfo.icon}
              <Badge variant="outline" className="text-xs">
                {typeInfo.label}
              </Badge>
            </div>
            <SheetTitle className="flex items-center gap-2">
              {node.name}
              <Badge className={STATUS_COLORS[node.status]}>
                {node.status}
              </Badge>
            </SheetTitle>
            <SheetDescription>
              {t("drawer.namespace")}: {node.namespace}
            </SheetDescription>
          </SheetHeader>

          <Separator className="my-4 shrink-0" />

          <Tabs
            defaultValue="overview"
            className="flex-1 flex flex-col min-h-0"
          >
            <TabsList className="w-full shrink-0">
              <TabsTrigger value="overview" className="flex-1">
                {t("drawer.overview")}
              </TabsTrigger>
              <TabsTrigger value="resources" className="flex-1">
                {t("drawer.resources")}
              </TabsTrigger>
              {(node.type === "form" || node.type === "node") && (
                <TabsTrigger value="template" className="flex-1">
                  {node.type === "form"
                    ? t("drawer.template")
                    : t("drawer.yaml")}
                </TabsTrigger>
              )}
              <TabsTrigger value="events" className="flex-1">
                {t("drawer.events")}
              </TabsTrigger>
            </TabsList>

            <ScrollArea className="flex-1 mt-4 w-full overflow-x-hidden">
              <TabsContent value="overview" className="m-0">
                <OverviewTab node={node} />
              </TabsContent>

              <TabsContent value="resources" className="m-0 overflow-hidden">
                <ResourcesTab node={node} childNodes={childNodes} />
              </TabsContent>

              {node.type === "form" && (
                <TabsContent value="template" className="m-0">
                  <TemplateTab node={node} />
                </TabsContent>
              )}

              {node.type === "node" && (
                <TabsContent value="template" className="m-0">
                  <YAMLTab node={node} />
                </TabsContent>
              )}

              <TabsContent value="events" className="m-0">
                <EventsTab node={node} />
              </TabsContent>
            </ScrollArea>
          </Tabs>
        </div>
      </SheetContent>
    </Sheet>
  );
}

function OverviewTab({ node }: { node: TopologyNode }) {
  const { t } = useTranslation();
  return (
    <div className="space-y-4">
      {/* Status Card */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium">
            {t("drawer.status")}
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0">
          <div className="flex items-center gap-2">
            {STATUS_ICONS[node.status]}
            <span className="capitalize">{node.status}</span>
          </div>
        </CardContent>
      </Card>

      {/* Metrics Card */}
      {node.metrics && (
        <Card>
          <CardHeader className="py-3">
            <CardTitle className="text-sm font-medium">
              {t("drawer.metrics")}
            </CardTitle>
          </CardHeader>
          <CardContent className="py-3 pt-0">
            <div className="grid grid-cols-3 gap-4">
              <MetricItem
                label={t("drawer.desired")}
                value={node.metrics.desired}
                color="text-blue-500"
              />
              <MetricItem
                label={t("status.ready")}
                value={node.metrics.ready}
                color="text-emerald-500"
              />
              <MetricItem
                label={t("status.failed")}
                value={node.metrics.failed}
                color="text-rose-500"
              />
            </div>
          </CardContent>
        </Card>
      )}

      {/* Children Count */}
      {node.children && node.children.length > 0 && (
        <Card>
          <CardHeader className="py-3">
            <CardTitle className="text-sm font-medium">
              {t("drawer.children")}
            </CardTitle>
          </CardHeader>
          <CardContent className="py-3 pt-0">
            <p className="text-2xl font-bold">{node.children.length}</p>
            <p className="text-xs text-muted-foreground">
              {node.type === "hub" ? t("drawer.forms") : t("drawer.nodes")}
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function MetricItem({
  label,
  value,
  color,
}: {
  label: string;
  value: number;
  color: string;
}) {
  return (
    <div className="text-center">
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
      <p className="text-xs text-muted-foreground">{label}</p>
    </div>
  );
}

function ResourcesTab({
  node,
  childNodes,
}: {
  node: TopologyNode;
  childNodes: TopologyNode[];
}) {
  const { t } = useTranslation();
  // Filter to get only resource type children (for LynqNode)
  // or show forms/nodes for hub/form types
  const resourceChildren = childNodes.filter((n) => n.type === "resource");
  const formChildren = childNodes.filter((n) => n.type === "form");
  const nodeChildren = childNodes.filter((n) => n.type === "node");

  // Parse resource name to extract kind and name (format: "Kind/Name")
  const parseResourceName = (name: string) => {
    const parts = name.split("/");
    if (parts.length >= 2) {
      return { kind: parts[0], name: parts.slice(1).join("/") };
    }
    return { kind: t("drawer.resource"), name };
  };

  // Separate managed vs skipped resources
  const managedResources = resourceChildren.filter(
    (r) => r.status !== "skipped"
  );
  const skippedResources = resourceChildren.filter(
    (r) => r.status === "skipped"
  );

  return (
    <div className="space-y-4 w-full overflow-hidden">
      {/* Show resources for LynqNode */}
      {node.type === "node" && (
        <>
          {/* Info about resource status */}
          <div className="text-xs text-muted-foreground p-2 bg-muted/50 rounded">
            <p>
              <strong>{t("drawer.managedColon")}</strong>{" "}
              {t("drawer.managedInfo")}
            </p>
            <p>
              <strong>{t("drawer.skippedColon")}</strong>{" "}
              {t("drawer.skippedInfo")}
            </p>
          </div>

          <p className="text-sm text-muted-foreground">
            {t("drawer.managedResources")} ({managedResources.length}):
          </p>

          {managedResources.length > 0 ? (
            <div className="space-y-2 w-full overflow-hidden">
              {managedResources.map((resource) => {
                const parsed = parseResourceName(resource.name);
                return (
                  <ResourceItem
                    key={resource.id}
                    kind={parsed.kind}
                    name={parsed.name}
                    namespace={resource.namespace}
                    status={resource.status}
                    metadata={resource.metadata}
                  />
                );
              })}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground italic">
              {t("drawer.noResources")}
            </p>
          )}

          {/* Skipped resources section */}
          {skippedResources.length > 0 && (
            <>
              <p className="text-sm text-muted-foreground mt-4">
                {t("drawer.skippedResources")} ({skippedResources.length}):
              </p>
              <div className="space-y-2 w-full overflow-hidden">
                {skippedResources.map((resource) => {
                  const parsed = parseResourceName(resource.name);
                  return (
                    <ResourceItem
                      key={resource.id}
                      kind={parsed.kind}
                      name={parsed.name}
                      namespace={resource.namespace}
                      status={resource.status}
                      metadata={resource.metadata}
                    />
                  );
                })}
              </div>
            </>
          )}
        </>
      )}

      {/* Show forms for LynqHub */}
      {node.type === "hub" && (
        <>
          <p className="text-sm text-muted-foreground">
            {t("drawer.formsReferencing")} ({formChildren.length}):
          </p>
          {formChildren.length > 0 ? (
            <div className="space-y-2">
              {formChildren.map((form) => (
                <ChildNodeItem key={form.id} node={form} />
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground italic">
              {t("drawer.noFormsReference")}
            </p>
          )}
        </>
      )}

      {/* Show nodes for LynqForm */}
      {node.type === "form" && (
        <>
          <p className="text-sm text-muted-foreground">
            {t("drawer.nodesCreated")} ({nodeChildren.length}):
          </p>
          {nodeChildren.length > 0 ? (
            <div className="space-y-2">
              {nodeChildren.map((lynqNode) => (
                <ChildNodeItem key={lynqNode.id} node={lynqNode} />
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground italic">
              {t("drawer.noNodesCreated")}
            </p>
          )}
        </>
      )}

      {/* Show message for resource type */}
      {node.type === "resource" && (
        <p className="text-sm text-muted-foreground italic">
          {t("drawer.kubernetesResource")}
        </p>
      )}

      {/* Show orphan info */}
      {node.type === "orphan" && <OrphanInfo node={node} />}
    </div>
  );
}

// Orphan information display
function OrphanInfo({ node }: { node: TopologyNode }) {
  const { t } = useTranslation();
  const metadata = node.metadata;

  if (!metadata) {
    return (
      <p className="text-sm text-muted-foreground italic">
        {t("drawer.orphanedResourceNoLongerManaged")}
      </p>
    );
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "Unknown";
    try {
      return new Date(dateStr).toLocaleString();
    } catch {
      return dateStr;
    }
  };

  const reasonLabels: Record<string, string> = {
    RemovedFromTemplate: t("drawer.removedFromTemplate"),
    LynqNodeDeleted: t("drawer.lynqNodeDeleted"),
  };

  return (
    <div className="space-y-4">
      {/* Warning banner */}
      <div className="p-3 rounded-md bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800">
        <div className="flex items-start gap-2">
          <IconUnlink size={16} className="text-amber-500 mt-0.5 shrink-0" />
          <div>
            <p className="text-sm font-medium text-amber-700 dark:text-amber-400">
              {t("drawer.orphanedResource")}
            </p>
            <p className="text-xs text-amber-600 dark:text-amber-500 mt-1">
              {t("drawer.orphanedDescription")}
            </p>
          </div>
        </div>
      </div>

      {/* Orphan metadata */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium">
            {t("drawer.orphanDetails")}
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0 space-y-3">
          {/* Orphaned reason */}
          {metadata.orphanedReason && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground w-20">
                {t("drawer.reason")}:
              </span>
              <Badge variant="outline" className="text-xs">
                {reasonLabels[metadata.orphanedReason] ||
                  metadata.orphanedReason}
              </Badge>
            </div>
          )}

          {/* Orphaned at */}
          {metadata.orphanedAt && (
            <div className="flex items-center gap-2">
              <IconCalendar size={12} className="text-muted-foreground" />
              <span className="text-xs text-muted-foreground">
                {formatDate(metadata.orphanedAt)}
              </span>
            </div>
          )}

          {/* Original node */}
          {metadata.originalNode && (
            <div className="flex items-start gap-2">
              <span className="text-xs text-muted-foreground w-20">
                {t("drawer.original")}:
              </span>
              <div>
                <Badge variant="secondary" className="text-xs font-mono">
                  {metadata.originalNode}
                </Badge>
                {metadata.originalNodeNamespace && (
                  <span className="text-xs text-muted-foreground ml-1">
                    {t("drawer.in")} {metadata.originalNodeNamespace}
                  </span>
                )}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Action hint */}
      <div className="text-xs text-muted-foreground p-3 bg-muted/50 rounded-md">
        <p className="font-medium mb-1">{t("drawer.cleanupHint")}</p>
        <ul className="list-disc list-inside space-y-0.5 ml-1">
          <li>{t("drawer.deleteManually")}</li>
          <li>{t("drawer.readdToForm")}</li>
        </ul>
      </div>
    </div>
  );
}

function ResourceItem({
  kind,
  name,
  namespace,
  status,
  metadata,
}: {
  kind: string;
  name: string;
  namespace: string;
  status: ResourceStatus;
  metadata?: ResourceMetadata;
}) {
  const { t } = useTranslation();
  const isOnce = metadata?.creationPolicy === "Once";
  const isRetain = metadata?.deletionPolicy === "Retain";
  const isStuck = metadata?.conflictPolicy === "Stuck";
  const isForce = metadata?.conflictPolicy === "Force";

  // Show warning if this individual resource is failed and has Stuck policy (potential conflict)
  const mayBeStuck = isStuck && status === "failed";

  return (
    <div
      className={`p-2.5 rounded-md border overflow-hidden ${
        mayBeStuck
          ? "border-rose-300 bg-rose-50/50 dark:bg-rose-950/30"
          : "border-border bg-muted/30"
      }`}
    >
      {/* Header row: Kind badge + Name + Status - using grid for fixed column sizes */}
      <div className="grid grid-cols-[auto_1fr_auto] items-center gap-2">
        <Badge variant="outline" className="text-xs font-medium">
          {kind}
        </Badge>
        <span className="text-sm font-medium truncate min-w-0" title={name}>
          {name}
        </span>
        <div>{STATUS_ICONS[status]}</div>
      </div>

      {/* Namespace */}
      <p
        className="text-xs text-muted-foreground mt-1 truncate"
        title={namespace}
      >
        {namespace}
      </p>

      {/* Policy chips */}
      {metadata && (
        <div className="flex flex-wrap items-center gap-1.5 mt-2">
          {/* Creation Policy */}
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium cursor-help ${
                  isOnce
                    ? "bg-amber-100 text-amber-700 border border-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:border-amber-800"
                    : "bg-blue-100 text-blue-700 border border-blue-200 dark:bg-blue-950 dark:text-blue-400 dark:border-blue-800"
                }`}
              >
                {isOnce ? (
                  <IconCircleDot size={10} />
                ) : (
                  <IconRefresh size={10} />
                )}
                {isOnce ? t("drawer.once") : t("drawer.reconcile")}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {isOnce
                ? t("drawer.createdOnce")
                : t("drawer.continuouslyReconciled")}
            </TooltipContent>
          </Tooltip>

          {/* Deletion Policy */}
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium cursor-help ${
                  isRetain
                    ? "bg-purple-100 text-purple-700 border border-purple-200 dark:bg-purple-950 dark:text-purple-400 dark:border-purple-800"
                    : "bg-slate-100 text-slate-600 border border-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700"
                }`}
              >
                {isRetain ? <IconArchive size={10} /> : <IconTrash size={10} />}
                {isRetain ? t("drawer.retain") : t("drawer.delete")}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {isRetain
                ? t("drawer.resourceRetained")
                : t("drawer.resourceDeleted")}
            </TooltipContent>
          </Tooltip>

          {/* Conflict Policy - highlight if potentially stuck */}
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium cursor-help ${
                  mayBeStuck
                    ? "bg-rose-100 text-rose-700 border border-rose-300 animate-pulse dark:bg-rose-950 dark:text-rose-400 dark:border-rose-800"
                    : isForce
                    ? "bg-orange-100 text-orange-700 border border-orange-200 dark:bg-orange-950 dark:text-orange-400 dark:border-orange-800"
                    : "bg-slate-100 text-slate-600 border border-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700"
                }`}
              >
                {mayBeStuck ? (
                  <IconAlertTriangle size={10} />
                ) : isForce ? (
                  <IconBolt size={10} />
                ) : (
                  <IconLock size={10} />
                )}
                {mayBeStuck
                  ? t("drawer.mayBeStuck")
                  : isForce
                  ? t("drawer.force")
                  : t("drawer.stuck")}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {mayBeStuck
                ? t("drawer.mayBeStuckTooltip")
                : isForce
                ? t("drawer.forceTakesOwnership")
                : t("drawer.stopsReconciliation")}
            </TooltipContent>
          </Tooltip>
        </div>
      )}
    </div>
  );
}

function ChildNodeItem({ node }: { node: TopologyNode }) {
  const { t } = useTranslation();
  const TYPE_LABELS: Record<string, { label: string; icon: React.ReactNode }> =
    {
      hub: { label: t("drawer.lynqHub"), icon: <IconBox size={16} /> },
      form: { label: t("drawer.lynqForm"), icon: <IconStack2 size={16} /> },
      node: { label: t("drawer.lynqNode"), icon: <IconActivity size={16} /> },
      resource: { label: t("drawer.resource"), icon: <IconBox size={16} /> },
      orphan: {
        label: t("drawer.orphaned"),
        icon: <IconAlertTriangle size={16} className="text-amber-500" />,
      },
    };
  const typeInfo = TYPE_LABELS[node.type] || { label: node.type, icon: null };

  return (
    <div className="flex items-center justify-between p-2 rounded-md bg-muted/50">
      <div className="flex items-center gap-2 min-w-0 flex-1">
        {typeInfo.icon}
        <div className="min-w-0 flex-1">
          <span className="text-sm block truncate">{node.name}</span>
          <span className="text-xs text-muted-foreground block truncate">
            {node.namespace}
          </span>
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0 ml-2">
        {node.metrics && (
          <span className="text-xs text-muted-foreground">
            {node.metrics.ready}/{node.metrics.desired}
          </span>
        )}
        {STATUS_ICONS[node.status]}
      </div>
    </div>
  );
}

// Format relative time from ISO timestamp
function formatRelativeTime(
  t: TFunction<"translation", undefined>,
  timestamp: string
): string {
  if (!timestamp) return t("common.unknown");

  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) return t("time.daysAgo", { count: diffDays });
  if (diffHours > 0) return t("time.hoursAgo", { count: diffHours });
  if (diffMins > 0) return t("time.minutesAgo", { count: diffMins });
  if (diffSecs > 0) return t("time.secondsAgo", { count: diffSecs });
  return t("time.justNow");
}

// Connection status indicator component
function ConnectionIndicator({
  status,
  interval,
}: {
  status: ConnectionStatus;
  interval: number;
}) {
  const { t } = useTranslation();
  const statusConfig = {
    loading: {
      icon: IconBroadcast,
      color: "text-amber-500",
      label: t("drawer.loading"),
      badge: null,
      animate: "animate-pulse",
    },
    polling: {
      icon: IconRefresh,
      color: "text-blue-500",
      label: t("drawer.polling"),
      badge: `${interval / 1000}s`,
      animate: "",
    },
    disconnected: {
      icon: IconWifiOff,
      color: "text-slate-400",
      label: t("drawer.disconnected"),
      badge: null,
      animate: "",
    },
  };

  const config = statusConfig[status];
  const Icon = config.icon;

  return (
    <div className={`flex items-center gap-1.5 text-xs ${config.color}`}>
      <Icon size={12} className={config.animate} />
      <span>{config.label}</span>
      {config.badge && (
        <span className="px-1 py-0.5 rounded text-[10px] bg-current/10">
          {config.badge}
        </span>
      )}
    </div>
  );
}

function EventsTab({ node }: { node: TopologyNode }) {
  const { t } = useTranslation();
  const pollInterval = 5000; // 5 seconds for responsive updates
  const { events, loading, error, connectionStatus, lastUpdate, refetch } =
    useEvents({
      nodeName: node.name,
      namespace: node.namespace,
      enabled:
        node.type === "node" || node.type === "hub" || node.type === "form",
      maxEvents: 50,
      pollInterval,
    });

  // Filter events for this specific object type
  const filteredEvents = events.filter((event) => {
    // Match events for LynqNode, LynqHub, or LynqForm based on node type
    const kindMap: Record<string, string> = {
      node: "LynqNode",
      hub: "LynqHub",
      form: "LynqForm",
    };
    const expectedKind = kindMap[node.type];
    return !expectedKind || event.involvedObject.kind === expectedKind;
  });

  // Format last update time
  const formatLastUpdate = (
    t: TFunction<"translation", undefined>,
    date: Date | null
  ) => {
    if (!date) return null;
    const now = new Date();
    const diffSecs = Math.floor((now.getTime() - date.getTime()) / 1000);
    if (diffSecs < 5) return t("time.justNow");
    if (diffSecs < 60) return t("time.secondsAgo", { count: diffSecs });
    return t("time.minutesAgo", { count: Math.floor(diffSecs / 60) });
  };

  return (
    <div className="space-y-2">
      {/* Header with connection status */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex flex-col">
          <p className="text-sm text-muted-foreground">
            {t("drawer.recentEvents", { name: node.name })}
          </p>
          {lastUpdate && (
            <p className="text-[10px] text-muted-foreground/60">
              {t("drawer.updated")} {formatLastUpdate(t, lastUpdate)}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <ConnectionIndicator
            status={connectionStatus}
            interval={pollInterval}
          />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => refetch()}
                disabled={loading}
              >
                <IconRefresh
                  size={12}
                  className={loading ? "animate-spin" : ""}
                />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("drawer.refreshEvents")}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {/* Error state */}
      {error && (
        <div className="p-3 rounded-md border border-rose-500/20 bg-rose-500/10 text-rose-500 text-sm">
          {t("drawer.failedToLoadEvents")}: {error.message}
        </div>
      )}

      {/* Loading state */}
      {loading && events.length === 0 && (
        <div className="flex items-center justify-center py-8">
          <IconLoader2
            size={24}
            className="animate-spin text-muted-foreground"
          />
        </div>
      )}

      {/* Empty state */}
      {!loading && events.length === 0 && !error && (
        <div className="text-center py-8">
          <IconActivity
            size={32}
            className="mx-auto text-muted-foreground/50"
          />
          <p className="mt-2 text-sm text-muted-foreground">
            {t("drawer.noEventsFound")}
          </p>
        </div>
      )}

      {/* Events list */}
      {filteredEvents.map((event, i) => (
        <div
          key={`${event.reason}-${event.lastTimestamp}-${i}`}
          className="p-3 rounded-md border bg-card text-card-foreground"
        >
          <div className="flex items-center justify-between mb-1">
            <div className="flex items-center gap-2">
              <Badge
                variant={event.type === "Warning" ? "destructive" : "secondary"}
                className="text-xs"
              >
                {event.reason}
              </Badge>
              {event.count > 1 && (
                <span className="text-xs text-muted-foreground">
                  x{event.count}
                </span>
              )}
            </div>
            <span className="text-xs text-muted-foreground">
              {formatRelativeTime(t, event.lastTimestamp)}
            </span>
          </div>
          <p className="text-sm">{event.message}</p>
          {event.source.component && (
            <p className="text-xs text-muted-foreground mt-1">
              {t("drawer.source")}: {event.source.component}
            </p>
          )}
        </div>
      ))}
    </div>
  );
}

// YAML code block component with copy button
function YAMLCodeBlock({ yaml, title }: { yaml: string; title?: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(yaml);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [yaml]);

  return (
    <div className="relative">
      {title && (
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium">{title}</span>
        </div>
      )}
      <div className="relative group">
        <pre className="p-3 rounded-md bg-muted text-xs font-mono overflow-x-auto whitespace-pre-wrap break-all">
          {yaml}
        </pre>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="absolute top-2 right-2 h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
              onClick={handleCopy}
            >
              {copied ? (
                <IconCheck size={12} className="text-emerald-500" />
              ) : (
                <IconCopy size={12} />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            {copied ? t("common.copied") : t("common.copy")}
          </TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

// Template tab for LynqForm - shows template spec and available variables
function TemplateTab({ node }: { node: TopologyNode }) {
  const { t } = useTranslation();
  const [details, setDetails] = useState<FormDetailsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedResources, setExpandedResources] = useState<Set<string>>(
    new Set()
  );

  useEffect(() => {
    async function fetchDetails() {
      setLoading(true);
      setError(null);
      try {
        const data = await formApi.getDetails(node.name, node.namespace);
        setDetails(data);
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : t("drawer.failedToLoadTemplateDetails")
        );
      } finally {
        setLoading(false);
      }
    }
    fetchDetails();
  }, [node.name, node.namespace]);

  const toggleResource = useCallback((id: string) => {
    setExpandedResources((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <IconLoader2 size={24} className="animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return <div className="text-sm text-destructive py-4">{error}</div>;
  }

  if (!details) return null;

  const formSpec = details.form.spec as unknown as Record<string, unknown>;
  const resourceTypes = [
    "serviceAccounts",
    "deployments",
    "statefulSets",
    "daemonSets",
    "services",
    "ingresses",
    "configMaps",
    "secrets",
    "persistentVolumeClaims",
    "jobs",
    "cronJobs",
    "podDisruptionBudgets",
    "networkPolicies",
    "horizontalPodAutoscalers",
    "manifests",
  ];

  return (
    <div className="space-y-4">
      {/* Available Variables */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium flex items-center gap-2">
            <IconVariable size={16} />
            {t("drawer.availableVariables")}
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0">
          <div className="flex flex-wrap gap-1.5">
            {details.variables.map((variable) => (
              <Badge
                key={variable}
                variant="secondary"
                className="font-mono text-xs"
              >
                {variable}
              </Badge>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Hub Reference */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium">
            {t("forms.hubReference")}
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0">
          <Badge variant="outline" className="font-mono">
            {formSpec.hubId as string}
          </Badge>
        </CardContent>
      </Card>

      {/* TResource Definitions */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium flex items-center gap-2">
            <IconFileCode size={16} />
            {t("drawer.resourceTemplates")}
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0 space-y-2">
          {resourceTypes.map((type) => {
            const resources = formSpec[type] as
              | Array<{ id: string; spec?: unknown }>
              | undefined;
            if (!resources || resources.length === 0) return null;

            return (
              <div key={type} className="space-y-1">
                <div className="text-xs font-medium text-muted-foreground capitalize">
                  {type} ({resources.length})
                </div>
                {resources.map((resource) => {
                  const isExpanded = expandedResources.has(resource.id);
                  return (
                    <div key={resource.id}>
                      <button
                        className="flex items-center gap-2 w-full p-2 rounded-md hover:bg-muted/50 text-left"
                        onClick={() => toggleResource(resource.id)}
                      >
                        {isExpanded ? (
                          <IconChevronDown size={12} />
                        ) : (
                          <IconChevronRight size={12} />
                        )}
                        <Badge variant="outline" className="font-mono text-xs">
                          {resource.id}
                        </Badge>
                      </button>
                      {isExpanded && (
                        <div className="ml-5 mt-1">
                          <YAMLCodeBlock
                            yaml={JSON.stringify(resource, null, 2)}
                          />
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            );
          })}
        </CardContent>
      </Card>
    </div>
  );
}

// YAML tab for LynqNode - shows spec.data and applied resources
function YAMLTab({ node }: { node: TopologyNode }) {
  const { t } = useTranslation();
  const [nodeData, setNodeData] = useState<Record<string, unknown> | null>(
    null
  );
  const [resources, setResources] = useState<Array<Record<string, unknown>>>(
    []
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedResources, setExpandedResources] = useState<Set<string>>(
    new Set()
  );

  useEffect(() => {
    async function fetchData() {
      setLoading(true);
      setError(null);
      try {
        // Fetch node details and resources in parallel
        const [nodeResult, resourcesResult] = await Promise.all([
          nodeApi.get(node.name, node.namespace),
          nodeApi.getResources(node.name, node.namespace),
        ]);

        // Extract spec.data from node
        const data = (
          nodeResult as unknown as { spec?: { data?: Record<string, unknown> } }
        )?.spec?.data;
        setNodeData(data || {});

        // Get applied resources
        const items =
          (resourcesResult as { items?: Array<Record<string, unknown>> })
            ?.items || [];
        setResources(items);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : t("drawer.failedToLoadNodeData")
        );
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, [node.name, node.namespace]);

  const toggleResource = useCallback((id: string) => {
    setExpandedResources((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <IconLoader2 size={24} className="animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return <div className="text-sm text-destructive py-4">{error}</div>;
  }

  return (
    <div className="space-y-4">
      {/* Template Variables (spec.data) */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium flex items-center gap-2">
            <IconVariable size={16} />
            {t("drawer.templateVariables")}
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0">
          {nodeData && Object.keys(nodeData).length > 0 ? (
            <YAMLCodeBlock yaml={JSON.stringify(nodeData, null, 2)} />
          ) : (
            <p className="text-sm text-muted-foreground italic">
              {t("drawer.noTemplateData")}
            </p>
          )}
        </CardContent>
      </Card>

      {/* Applied Resources */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm font-medium flex items-center gap-2">
            <IconCode size={16} />
            {t("drawer.appliedResources")} ({resources.length})
          </CardTitle>
        </CardHeader>
        <CardContent className="py-3 pt-0 space-y-2">
          {resources.length > 0 ? (
            resources.map((resource, idx) => {
              const id = (resource._id as string) || `resource-${idx}`;
              const kind = resource.kind as string;
              const metadata = resource.metadata as
                | { name?: string; namespace?: string }
                | undefined;
              const name = metadata?.name || t("common.unknown");
              const namespace = metadata?.namespace;
              const hasError = !!resource.error;
              const isExpanded = expandedResources.has(id);

              return (
                <div key={id}>
                  <button
                    className={`flex items-center gap-2 w-full p-2 rounded-md hover:bg-muted/50 text-left ${
                      hasError ? "bg-rose-50 dark:bg-rose-950/30" : ""
                    }`}
                    onClick={() => toggleResource(id)}
                  >
                    {isExpanded ? (
                      <IconChevronDown size={12} />
                    ) : (
                      <IconChevronRight size={12} />
                    )}
                    <Badge variant="outline" className="text-xs shrink-0">
                      {kind}
                    </Badge>
                    <span className="text-sm truncate flex-1" title={name}>
                      {name}
                    </span>
                    {namespace && (
                      <span
                        className="text-xs text-muted-foreground truncate"
                        title={namespace}
                      >
                        {namespace}
                      </span>
                    )}
                    {hasError && (
                      <IconAlertTriangle
                        size={12}
                        className="text-rose-500 shrink-0"
                      />
                    )}
                  </button>
                  {isExpanded && (
                    <div className="ml-5 mt-1">
                      {hasError ? (
                        <div className="text-sm text-destructive p-2 bg-rose-50 dark:bg-rose-950/30 rounded">
                          {resource.error as string}
                        </div>
                      ) : (
                        <YAMLCodeBlock
                          yaml={JSON.stringify(resource, null, 2)}
                        />
                      )}
                    </div>
                  )}
                </div>
              );
            })
          ) : (
            <p className="text-sm text-muted-foreground italic">
              {t("drawer.noAppliedResources")}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
