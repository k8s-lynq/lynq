const ko = {
  // Common
  common: {
    loading: "로딩 중...",
    error: "오류",
    refresh: "새로고침",
    search: "검색",
    filter: "필터",
    clearAll: "모두 지우기",
    viewAll: "모두 보기",
    details: "상세",
    back: "뒤로",
    retry: "재시도",
    copy: "클립보드에 복사",
    copied: "복사됨!",
    home: "홈",
    documentation: "문서",
    github: "GitHub",
    language: "언어",
  },

  // Navigation
  nav: {
    overview: "개요",
    topology: "토폴로지",
    hubs: "허브",
    forms: "폼",
    nodes: "노드",
    dashboard: "대시보드",
  },

  // Status
  status: {
    ready: "준비됨",
    pending: "대기 중",
    failed: "실패",
    skipped: "건너뜀",
    all: "모든 상태",
  },

  // Theme
  theme: {
    switchToLight: "라이트 모드로 전환",
    switchToDark: "다크 모드로 전환",
  },

  // Search
  search: {
    placeholder: "검색...",
    searchHubsFormsNodes: "허브, 폼, 노드 검색...",
    noResults: "결과가 없습니다.",
    navigation: "탐색",
    allHubs: "모든 허브",
    allForms: "모든 폼",
    allNodes: "모든 노드",
    moreNodes: "+{{count}}개 더...",
  },

  // Overview Page
  overview: {
    title: "개요",
    healthOverview: "상태 개요",
    healthDescription: "전체 리소스 유형의 상태 분포",
    nodeStatus: "노드 상태",
    nodeStatusDescription: "노드 상태 분포",
    quickActions: "빠른 작업",
    quickActionsDescription: "자주 사용하는 작업 및 탐색",
    viewTopology: "토폴로지 보기",
    manageHubs: "허브 관리",
    manageForms: "폼 관리",
    viewAllNodes: "모든 노드 보기",
    recentHubs: "최근 허브",
    recentForms: "최근 폼",
    recentNodes: "최근 노드",
    noHubs: "허브가 없습니다. LynqHub를 생성하여 시작하세요.",
    noForms: "폼이 없습니다. LynqForm을 생성하여 템플릿을 정의하세요.",
    noNodes: "노드가 없습니다. 노드는 허브에 의해 자동 생성됩니다.",
    resources: "리소스",
  },

  // Hubs Page
  hubs: {
    title: "허브",
    description: "외부 데이터소스 연결",
    noHubsFound: "허브가 없습니다",
    loadingHubs: "허브 로딩 중...",
    createHubToStart: "LynqHub 리소스를 생성하여 시작하세요.",
    desired: "목표",
    templates: "템플릿",
    syncInterval: "동기화 주기",
    database: "데이터베이스",
    backToHubs: "허브 목록으로",
    errorLoadingHub: "허브 로딩 오류",
    datasourceConfig: "데이터소스 설정",
    host: "호스트",
    port: "포트",
    table: "테이블",
    userSecret: "사용자 시크릿",
    passwordSecret: "비밀번호 시크릿",
    lastSync: "마지막 동기화",
    valueMappings: "값 매핑",
    requiredMappings: "필수 매핑",
    uidColumn: "UID 컬럼",
    activateColumn: "활성화 컬럼",
    extraMappings: "추가 매핑",
    conditions: "조건",
    relatedNodes: "관련 노드",
    noNodesYet: "이 허브에서 생성된 노드가 없습니다.",
    moreNodes: "+{{count}}개 더",
  },

  // Forms Page
  forms: {
    title: "폼",
    description: "리소스 템플릿",
    noFormsFound: "폼이 없습니다",
    noMatchingForms: "일치하는 폼이 없습니다",
    loadingForms: "폼 로딩 중...",
    createFormToStart: "LynqForm 리소스를 생성하여 템플릿을 정의하세요.",
    adjustFilters: "필터를 조정하여 원하는 항목을 찾아보세요.",
    clearFilters: "필터 지우기",
    searchByNameOrHub: "이름 또는 허브로 검색...",
    allHubs: "모든 허브",
    allNamespaces: "모든 네임스페이스",
    totalForms: "전체 폼",
    totalNodes: "전체 노드",
    hub: "허브",
    total: "전체",
    rollout: "롤아웃",
    backToForms: "폼 목록으로",
    errorLoadingForm: "폼 로딩 오류",
    hubReference: "허브 참조",
    resourceTypes: "리소스 유형",
    nodeStatus: "노드 상태",
    relatedNodes: "관련 노드",
    viewAllNodes: "모든 노드 보기",
    noNodesYet: "이 폼에서 생성된 노드가 없습니다.",
    moreNodes: "+{{count}}개 더",
    dependencies: "의존성",
    nodesUpdated: "업데이트된 노드",
    serviceAccounts: "서비스 계정",
    deployments: "배포",
    statefulSets: "스테이트풀셋",
    daemonSets: "데몬셋",
    services: "서비스",
    ingresses: "인그레스",
    configMaps: "컨피그맵",
    secrets: "시크릿",
    persistentVolumeClaims: "PVC",
    jobs: "잡",
    cronJobs: "크론잡",
    podDisruptionBudgets: "PDB",
    networkPolicies: "네트워크 정책",
    horizontalPodAutoscalers: "HPA",
    manifests: "사용자 정의 매니페스트",
  },

  // Nodes Page
  nodes: {
    title: "노드",
    description: "개별 노드 인스턴스",
    noNodesFound: "노드가 없습니다",
    noMatchingNodes: "일치하는 노드가 없습니다",
    loadingNodes: "노드 로딩 중...",
    nodesAutoCreated:
      "허브가 활성 행을 동기화할 때 노드가 자동으로 생성됩니다.",
    adjustFilters: "필터를 조정하여 원하는 항목을 찾아보세요.",
    clearFilters: "필터 지우기",
    searchByNameOrUid: "이름 또는 UID로 검색...",
    allHubs: "모든 허브",
    allForms: "모든 폼",
    allNamespaces: "모든 네임스페이스",
    totalNodes: "전체 노드",
    uid: "UID",
    form: "폼",
    backToNodes: "노드 목록으로",
    errorLoadingNode: "노드 로딩 오류",
    resourceStatus: "리소스 상태",
    managedColon: "관리됨:",
    resourcesActivelyReconciled: "활발하게 조정되는 리소스 (현재 템플릿)",
    skippedColon: "건너뜀:",
    resourcesSkippedDueToDependencyFailure: "의존성 실패로 인해 건너뛴 리소스",
    statusReflectsReconciliationState:
      "참고: 상태는 조정 상태를 반영하며, K8s Ready 상태는 아닙니다",
    noResourcesAppliedYet: "아직 적용된 리소스가 없습니다.",

    resourcesFailed: "{{count}}개의 리소스 실패",
    checkConditionsTab:
      "오류 세부 정보를 보려면 조건 탭을 확인하세요. 실패한 리소스는 적용된 리소스 목록에 포함되지 않습니다.",
    noConditionsReported: "보고된 조건이 없습니다.",
  },

  // Topology Page
  topology: {
    title: "토폴로지 뷰",
    problems: "문제",
    exitProblemMode: "문제 강조 모드 종료",
    highlightFailed: "실패한 노드 강조",
    searchNodes: "노드 검색...",
    clearSearch: "검색 지우기",
    found: "발견",
    expandAll: "모두 펼치기",
    expandAllTooltip: "모든 허브와 폼 펼치기",
    collapseAll: "모두 접기",
    collapseAllTooltip: "펼쳐진 노드 모두 접기",
    autoRefreshInterval: "자동 새로고침 주기",
    refreshNow: "지금 새로고침",
    fullscreen: "전체 화면",
    exitFullscreen: "전체 화면 종료",
    loadingTopology: "토폴로지 로딩 중...",
    failedToLoad: "토폴로지 로딩 실패",
    noResourcesFound: "리소스가 없습니다",
    createHubToStart: "LynqHub를 생성하여 첫 번째 파이프라인을 시작하세요.",
    problemsDetected: "{{count}}개의 문제 감지됨",
    noProblemsDetected: "문제가 감지되지 않음",
    refreshing: "새로고침 중...",
  },

  // Node Detail Drawer
  drawer: {
    namespace: "네임스페이스",
    overview: "개요",
    resources: "리소스",
    template: "템플릿",
    yaml: "YAML",
    events: "이벤트",
    status: "상태",
    metrics: "메트릭",
    children: "하위 항목",
    dragToResize: "드래그하여 크기 조절",

    // Resource types
    lynqHub: "LynqHub",
    lynqForm: "LynqForm",
    lynqNode: "LynqNode",
    resource: "리소스",
    orphaned: "고아 상태",

    // Metrics
    desired: "목표",

    // Resources tab
    managedResources: "관리 리소스",
    skippedResources: "건너뛴 리소스",
    managedInfo: "현재 템플릿의 리소스",
    skippedInfo: "의존성 실패만 해당",
    noResources: "이 노드에 대한 리소스가 없습니다.",
    formsReferencing: "이 허브를 참조하는 폼",
    noFormsReference: "이 허브를 참조하는 폼이 없습니다.",
    nodesCreated: "이 폼에서 생성된 노드",
    noNodesCreated: "이 폼에서 생성된 노드가 없습니다.",
    kubernetesResource: "LynqNode가 관리하는 Kubernetes 리소스입니다.",

    // Policies
    once: "한 번",
    reconcile: "조정",
    retain: "유지",
    delete: "삭제",
    stuck: "중단",
    force: "강제",
    createdOnce: "한 번 생성, 조정 없음",
    continuouslyReconciled: "지속적으로 조정됨",
    resourceRetained: "노드 삭제 후 리소스 유지",
    resourceDeleted: "노드와 함께 리소스 삭제",
    mayBeStuck: "충돌로 인해 리소스가 중단되었을 수 있음 - 소유권 확인",
    forceTakesOwnership: "충돌 시 강제로 소유권 획득",
    stopsReconciliation: "충돌 시 조정 중단",

    // Orphan
    orphanedResource: "고아 리소스",
    orphanedDescription:
      "이 리소스는 더 이상 어떤 LynqNode에서도 관리되지 않습니다. DeletionPolicy에 따라 유지되었습니다.",
    orphanDetails: "고아 상세 정보",
    reason: "이유",
    original: "원본",
    removedFromTemplate: "템플릿에서 제거됨",
    lynqNodeDeleted: "LynqNode 삭제됨",
    cleanupHint: "이 리소스를 정리하려면:",
    deleteManually: "더 이상 필요하지 않으면 수동으로 삭제",
    readdToForm: "LynqForm에 다시 추가하여 관리",

    // Template tab
    availableVariables: "사용 가능한 변수",
    resourceTemplates: "리소스 템플릿",

    // YAML tab
    templateVariables: "템플릿 변수",
    noTemplateData: "템플릿 데이터 없음",
    appliedResources: "적용된 리소스",
    noAppliedResources: "적용된 리소스 없음",

    // Events tab
    recentEvents: "{{name}}의 최근 이벤트",
    updated: "업데이트",
    refreshEvents: "이벤트 새로고침",
    failedToLoadEvents: "이벤트 로딩 실패",
    noEventsFound: "이 리소스에 대한 이벤트가 없습니다",
    source: "소스",

    // Connection status
    loading: "로딩 중",
    polling: "폴링 중",
    disconnected: "연결 끊김",
    forms: "폼",
    nodes: "노드",
    managedColon: "관리됨:",
    skippedColon: "건너뜀:",
    orphanedResourceNoLongerManaged:
      "이것은 Lynq에 의해 더 이상 관리되지 않는 고아 리소스입니다.",
    in: "에서",
    mayBeStuckTooltip: "충돌로 인해 리소스가 중단될 수 있음 - 소유권 확인",
    failedToLoadTemplateDetails: "템플릿 세부 정보를 로드하지 못했습니다",
    failedToLoadNodeData: "노드 데이터를 로드하지 못했습니다",
  },

  // Filters
  filters: {
    status: "상태",
    hub: "허브",
    form: "폼",
    namespace: "네임스페이스",
  },

  // Charts
  charts: {
    ready: "준비됨",
    failed: "실패",
    pending: "대기 중",
  },

  // Time
  time: {
    off: "끄기",
    justNow: "방금 전",
    secondsAgo: "{{count}}초 전",
    minutesAgo: "{{count}}분 전",
    hoursAgo: "{{count}}시간 전",
    daysAgo: "{{count}}일 전",
  },
} as const;

export default ko;
