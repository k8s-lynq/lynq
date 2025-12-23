const en = {
  // Common
  common: {
    loading: "Loading...",
    error: "Error",
    refresh: "Refresh",
    search: "Search",
    filter: "Filters",
    clearAll: "Clear all",
    viewAll: "View all",
    details: "Details",
    back: "Back",
    retry: "Retry",
    copy: "Copy to clipboard",
    copied: "Copied!",
    home: "Home",
    documentation: "Documentation",
    github: "GitHub",
    language: "Language",
  },

  // Navigation
  nav: {
    overview: "Overview",
    topology: "Topology",
    hubs: "Hubs",
    forms: "Forms",
    nodes: "Nodes",
    dashboard: "Dashboard",
  },

  // Status
  status: {
    ready: "Ready",
    pending: "Pending",
    failed: "Failed",
    skipped: "Skipped",
    all: "All Status",
  },

  // Theme
  theme: {
    switchToLight: "Switch to light mode",
    switchToDark: "Switch to dark mode",
  },

  // Search
  search: {
    placeholder: "Search...",
    searchHubsFormsNodes: "Type to search hubs, forms, nodes...",
    noResults: "No results found.",
    navigation: "Navigation",
    allHubs: "All Hubs",
    allForms: "All Forms",
    allNodes: "All Nodes",
    moreNodes: "+{{count}} more nodes...",
  },

  // Overview Page
  overview: {
    title: "Overview",
    healthOverview: "Health Overview",
    healthDescription: "Status distribution across all resource types",
    nodeStatus: "Node Status",
    nodeStatusDescription: "Distribution of node health",
    quickActions: "Quick Actions",
    quickActionsDescription: "Common tasks and navigation",
    viewTopology: "View Topology",
    manageHubs: "Manage Hubs",
    manageForms: "Manage Forms",
    viewAllNodes: "View All Nodes",
    recentHubs: "Recent Hubs",
    recentForms: "Recent Forms",
    recentNodes: "Recent Nodes",
    noHubs: "No hubs found. Create a LynqHub to get started.",
    noForms: "No forms found. Create a LynqForm to define templates.",
    noNodes: "No nodes found. Nodes are auto-created by hubs.",
    resources: "Resources",
  },

  // Hubs Page
  hubs: {
    title: "Hubs",
    description: "External datasource connections",
    noHubsFound: "No Hubs Found",
    loadingHubs: "Loading hubs...",
    createHubToStart: "Create a LynqHub resource to get started.",
    desired: "Desired",
    templates: "Templates",
    syncInterval: "Sync Interval",
    database: "Database",
    backToHubs: "Back to Hubs",
    errorLoadingHub: "Error Loading Hub",
    datasourceConfig: "Datasource Configuration",
    host: "Host",
    port: "Port",
    table: "Table",
    userSecret: "User Secret",
    passwordSecret: "Password Secret",
    lastSync: "Last Sync",
    valueMappings: "Value Mappings",
    requiredMappings: "Required Mappings",
    uidColumn: "UID Column",
    activateColumn: "Activate Column",
    extraMappings: "Extra Mappings",
    conditions: "Conditions",
    relatedNodes: "Related Nodes",
    noNodesYet: "No nodes created from this hub yet.",
    moreNodes: "+{{count}} more nodes",
  },

  // Forms Page
  forms: {
    title: "Forms",
    description: "Resource templates",
    noFormsFound: "No Forms Found",
    noMatchingForms: "No Matching Forms",
    loadingForms: "Loading forms...",
    createFormToStart: "Create a LynqForm resource to define templates.",
    adjustFilters:
      "Try adjusting your filters to find what you're looking for.",
    clearFilters: "Clear Filters",
    searchByNameOrHub: "Search by name or hub...",
    allHubs: "All Hubs",
    allNamespaces: "All Namespaces",
    totalForms: "Total Forms",
    totalNodes: "Total Nodes",
    hub: "Hub",
    total: "Total",
    rollout: "Rollout",
    backToForms: "Back to Forms",
    errorLoadingForm: "Error Loading Form",
    hubReference: "Hub Reference",
    resourceTypes: "Resource Types",
    nodeStatus: "Node Status",
    relatedNodes: "Related Nodes",
    viewAllNodes: "View All Nodes",
    moreNodes: "+{{count}} more nodes",
    dependencies: "deps",
    nodesUpdated: "nodes updated",
    serviceAccounts: "Service Accounts",
    deployments: "Deployments",
    statefulSets: "StatefulSets",
    daemonSets: "DaemonSets",
    services: "Services",
    ingresses: "Ingresses",
    configMaps: "ConfigMaps",
    secrets: "Secrets",
    persistentVolumeClaims: "PVCs",
    jobs: "Jobs",
    cronJobs: "CronJobs",
    podDisruptionBudgets: "PDBs",
    networkPolicies: "Network Policies",
    horizontalPodAutoscalers: "HPAs",
    manifests: "Custom Manifests",
  },

  // Nodes Page
  nodes: {
    title: "Nodes",
    description: "Individual node instances",
    noNodesFound: "No Nodes Found",
    noMatchingNodes: "No Matching Nodes",
    loadingNodes: "Loading nodes...",
    nodesAutoCreated:
      "Nodes are created automatically when Hub syncs active rows.",
    adjustFilters:
      "Try adjusting your filters to find what you're looking for.",
    clearFilters: "Clear Filters",
    searchByNameOrUid: "Search by name or UID...",
    allHubs: "All Hubs",
    allForms: "All Forms",
    allNamespaces: "All Namespaces",
    totalNodes: "Total Nodes",
    uid: "UID",
    form: "Form",
    backToNodes: "Back to Nodes",
    errorLoadingNode: "Error Loading Node",
    resourceStatus: "Resource Status",
    managedColon: "Managed:",
    resourcesActivelyReconciled:
      "Resources actively reconciled (in current template)",
    skippedColon: "Skipped:",
    resourcesSkippedDueToDependencyFailure:
      "Resources skipped due to dependency failure",
    statusReflectsReconciliationState:
      "Note: Status reflects reconciliation state, not K8s Ready condition",
    noResourcesAppliedYet: "No resources applied yet.",

    resourcesFailed: "{{count}} resource(s) failed",
    checkConditionsTab:
      "Check the Conditions tab for error details. Failed resources are not included in the applied resources list.",
    noConditionsReported: "No conditions reported.",
  },

  // Topology Page
  topology: {
    title: "Topology View",
    problems: "Problems",
    exitProblemMode: "Exit problem highlight mode",
    highlightFailed: "Highlight failed nodes",
    searchNodes: "Search nodes...",
    clearSearch: "Clear search",
    found: "found",
    expandAll: "Expand All",
    expandAllTooltip: "Expand all Hubs and Forms",
    collapseAll: "Collapse All",
    collapseAllTooltip: "Collapse all expanded nodes",
    autoRefreshInterval: "Auto-refresh interval",
    refreshNow: "Refresh now",
    fullscreen: "Fullscreen",
    exitFullscreen: "Exit fullscreen",
    loadingTopology: "Loading topology...",
    failedToLoad: "Failed to load topology",
    noResourcesFound: "No resources found",
    createHubToStart:
      "Create a LynqHub to get started with your first pipeline.",
    problemsDetected: "{{count}} problem(s) detected",
    noProblemsDetected: "No problems detected",
    refreshing: "Refreshing...",
  },

  // Node Detail Drawer
  drawer: {
    namespace: "Namespace",
    overview: "Overview",
    resources: "Resources",
    template: "Template",
    yaml: "YAML",
    events: "Events",
    status: "Status",
    metrics: "Metrics",
    children: "Children",
    dragToResize: "Drag to resize",

    // Resource types
    lynqHub: "LynqHub",
    lynqForm: "LynqForm",
    lynqNode: "LynqNode",
    resource: "Resource",
    orphaned: "Orphaned",

    // Metrics
    desired: "Desired",

    // Resources tab
    managedResources: "Managed Resources",
    skippedResources: "Skipped Resources",
    managedInfo: "Resources in current template",
    skippedInfo: "Dependency failure only",
    noResources: "No resources found for this node.",
    formsReferencing: "Forms referencing this hub",
    noFormsReference: "No forms reference this hub.",
    nodesCreated: "Nodes created from this form",
    noNodesCreated: "No nodes created from this form.",
    kubernetesResource: "This is a Kubernetes resource managed by a LynqNode.",

    // Policies
    once: "Once",
    reconcile: "Reconcile",
    retain: "Retain",
    delete: "Delete",
    stuck: "Stuck",
    force: "Force",
    createdOnce: "Created once, no reconciliation",
    continuouslyReconciled: "Continuously reconciled",
    resourceRetained: "Resource retained after node deletion",
    resourceDeleted: "Resource deleted with node",
    mayBeStuck: "Resource may be stuck due to conflict - check ownership",
    forceTakesOwnership: "Force takes ownership on conflict",
    stopsReconciliation: "Stops reconciliation on conflict",

    // Orphan
    orphanedResource: "Orphaned Resource",
    orphanedDescription:
      "This resource is no longer managed by any LynqNode. It was retained based on its DeletionPolicy.",
    orphanDetails: "Orphan Details",
    reason: "Reason",
    original: "Original",
    removedFromTemplate: "Removed from Template",
    lynqNodeDeleted: "LynqNode Deleted",
    cleanupHint: "To clean up this resource:",
    deleteManually: "Delete it manually if no longer needed",
    readdToForm: "Re-add it to a LynqForm to manage it again",

    // Template tab
    availableVariables: "Available Variables",
    resourceTemplates: "Resource Templates",

    // YAML tab
    templateVariables: "Template Variables",
    noTemplateData: "No template data",
    appliedResources: "Applied Resources",
    noAppliedResources: "No applied resources",

    // Events tab
    recentEvents: "Recent events for {{name}}",
    updated: "Updated",
    refreshEvents: "Refresh events",
    failedToLoadEvents: "Failed to load events",
    noEventsFound: "No events found for this resource",
    source: "Source",

    // Connection status
    loading: "Loading",
    polling: "Polling",
    disconnected: "Disconnected",
    forms: "Forms",
    nodes: "Nodes",
    managedColon: "Managed:",
    skippedColon: "Skipped:",
    orphanedResourceNoLongerManaged:
      "This is an orphaned resource no longer managed by Lynq.",
    in: "in",
    mayBeStuckTooltip:
      "Resource may be stuck due to conflict - check ownership",
    failedToLoadTemplateDetails: "Failed to load template details",
    failedToLoadNodeData: "Failed to load node data",
  },

  // Filters
  filters: {
    status: "Status",
    hub: "Hub",
    form: "Form",
    namespace: "Namespace",
  },

  // Charts
  charts: {
    ready: "Ready",
    failed: "Failed",
    pending: "Pending",
  },

  // Time
  time: {
    off: "Off",
    justNow: "Just now",
    secondsAgo: "{{count}}s ago",
    minutesAgo: "{{count}}m ago",
    hoursAgo: "{{count}}h ago",
    daysAgo: "{{count}}d ago",
  },
} as const;

export default en;
