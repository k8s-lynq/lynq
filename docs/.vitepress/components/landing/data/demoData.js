/**
 * Canonical sample dataset for the rebuilt Lynq landing page.
 *
 * Every landing section (Hero demo, LiveTransform, BeforeAfter, pipeline,
 * dashboard) draws from this file so the story stays internally consistent:
 * the same three node rows, the same value mappings, the same resource names.
 *
 * Story: a MySQL `node_configs` table -> LynqHub reads active rows -> LynqForm
 * renders resources per row -> Kubernetes gets real named resources.
 */

/**
 * @typedef {Object} NodeConfigRow
 * @property {string} node_id           - unique row identifier (maps to .uid)
 * @property {number} is_active         - 1/0 activation flag (maps to .activate)
 * @property {string} subscription_plan - plan tier (maps to .planId)
 * @property {string} node_url          - hostname/url (maps to .nodeUrl)
 */

/**
 * The three seed rows. gamma-llc is inactive (is_active=0) so it produces no
 * Kubernetes resources — used to demonstrate the activate flag.
 * @type {NodeConfigRow[]}
 */
export const nodeConfigs = [
  { node_id: 'acme-corp', is_active: 1, subscription_plan: 'pro', node_url: 'acme.example.com' },
  { node_id: 'beta-inc', is_active: 1, subscription_plan: 'free', node_url: 'beta.example.com' },
  { node_id: 'gamma-llc', is_active: 0, subscription_plan: 'pro', node_url: 'gamma.example.com' },
]

/**
 * A fourth row used to demonstrate a live INSERT into the table.
 * @type {NodeConfigRow}
 */
export const insertRow = {
  node_id: 'delta-co',
  is_active: 1,
  subscription_plan: 'pro',
  node_url: 'delta.example.com',
}

/** Column order for rendering the MySQL-ish table. @type {string[]} */
export const tableColumns = ['node_id', 'is_active', 'subscription_plan', 'node_url']

/** Required LynqHub value mappings (column -> required variable). */
export const valueMappings = {
  uid: 'node_id',
  activate: 'is_active',
}

/** Extra value mappings (column -> custom template variable). */
export const extraValueMappings = {
  planId: 'subscription_plan',
  nodeUrl: 'node_url',
}

/**
 * The Kubernetes resources a single active row produces via the demo LynqForm.
 * @param {string} uid
 * @returns {{ kind: string, name: string }[]}
 */
export function resourcesForUid(uid) {
  return [
    { kind: 'Deployment', name: `${uid}-app` },
    { kind: 'Service', name: `${uid}-svc` },
    { kind: 'Ingress', name: `${uid}-web` },
  ]
}

/**
 * The four-step "How It Works" walkthrough. Ported VERBATIM from
 * components/landing/InteractivePipeline.vue so the pipeline story is
 * domain-accurate and shared across sections.
 * @type {{ title: string, description: string, filename: string, code: string }[]}
 */
export const pipelineSteps = [
  {
    title: 'Connect Your Database',
    description: 'LynqHub polls your MySQL table at the configured syncInterval (default: 30 seconds). Any row where the activate column is truthy gets a corresponding LynqNode CR. Existing infrastructure keeps running if the database goes temporarily offline.',
    filename: 'lynqhub.yaml',
    code: `apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-hub
spec:
  source:
    type: mysql
    syncInterval: 1m
    mysql:
      host: mysql.default.svc
      port: 3306
      username: node_reader
      passwordRef:
        name: mysql-credentials
        key: password
      database: nodes
      table: node_configs`
  },
  {
    title: 'Map Your Columns',
    description: 'Map table columns to Lynq\'s required fields: uid (unique identifier per row) and activate (on/off switch). Use extraValueMappings for any additional columns your templates need — plan tiers, hostnames, replica counts, feature flags.',
    filename: 'lynqhub.yaml',
    code: `spec:
  valueMappings:
    uid: node_id          # Required: unique row identifier
    activate: is_active   # Required: boolean on/off flag
  extraValueMappings:
    planId: subscription_plan
    nodeUrl: node_url`
  },
  {
    title: 'Define Resource Templates',
    description: 'LynqForm defines exactly which Kubernetes resources to create per active row, using Go template syntax with sprig functions. Per-resource policies (conflictPolicy, deletionPolicy, creationPolicy) give fine-grained control over lifecycle behavior. Multiple LynqForms can reference the same hub.',
    filename: 'lynqform.yaml',
    code: `apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: web-stack
spec:
  hubId: my-hub
  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      conflictPolicy: Stuck   # halt if another controller owns this
      deletionPolicy: Delete  # clean up when row is deactivated
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: "{{ .uid }}"
        template:
          metadata:
            labels:
              app: "{{ .uid }}"
          spec:
            containers:
              - name: app
                image: nginx:stable`
  },
  {
    title: 'Resources Tracked Per Node',
    description: 'Each active row gets a LynqNode CR that tracks readiness, failures, conflicts, and skipped resources. The Ready condition is True only when all resources pass their readiness checks. Deactivate a row and resources are cleaned up; conflictPolicy: Stuck surfaces ownership conflicts before they cause damage.',
    filename: 'kubectl output',
    code: `$ kubectl get lynqnodes
NAME                  UID        FORM       READY  DESIRED  SKIPPED  CONFLICTED  CONDITIONS   AGE
acme-corp-web-stack   acme-corp  web-stack  3      3                 False       Reconciled   2m
beta-inc-web-stack    beta-inc   web-stack  2      2                 False       Reconciled   1m

$ kubectl get deployments
NAME              READY   AGE
acme-corp-app     1/1     2m
beta-inc-app      1/1     1m`
  }
]

/**
 * The step-4 kubectl output as a single string (mirrors pipelineSteps[3].code).
 * @type {string}
 */
export const kubectlOutput = pipelineSteps[3].code

/**
 * The same kubectl output decomposed into lines for line-by-line terminal
 * reveal. `prompt` is set on command lines ('$'); output lines have prompt ''.
 * @type {{ prompt: string, text: string }[]}
 */
export const kubectlLines = [
  { prompt: '$', text: 'kubectl get lynqnodes' },
  { prompt: '', text: 'NAME                  UID        FORM       READY  DESIRED  SKIPPED  CONFLICTED  CONDITIONS   AGE' },
  { prompt: '', text: 'acme-corp-web-stack   acme-corp  web-stack  3      3                 False       Reconciled   2m' },
  { prompt: '', text: 'beta-inc-web-stack    beta-inc   web-stack  2      2                 False       Reconciled   1m' },
  { prompt: '', text: '' },
  { prompt: '$', text: 'kubectl get deployments' },
  { prompt: '', text: 'NAME              READY   AGE' },
  { prompt: '', text: 'acme-corp-app     1/1     2m' },
  { prompt: '', text: 'beta-inc-app      1/1     1m' },
]

/**
 * The "old way" — hand-rolled kubectl toil the BeforeAfter section contrasts
 * against Lynq. Each line is a terminal line; `tone` flags styling intent
 * ('cmd' | 'comment' | 'error'). This is the manual work Lynq replaces: for a
 * NEW customer row you'd run these by hand, per node, and still get drift.
 * @type {{ prompt: string, text: string, tone: 'cmd' | 'comment' | 'error' }[]}
 */
export const oldWayCommands = [
  { prompt: '#', text: 'new customer signed up: beta-inc', tone: 'comment' },
  { prompt: '$', text: 'kubectl create deployment beta-inc-app --image=nginx:stable', tone: 'cmd' },
  { prompt: '$', text: 'kubectl expose deployment beta-inc-app --name beta-inc-svc --port 80', tone: 'cmd' },
  { prompt: '$', text: 'kubectl create ingress beta-inc-web --rule="beta.example.com/*=beta-inc-svc:80"', tone: 'cmd' },
  { prompt: '$', text: 'git add . && git commit -m "add beta-inc" && git push', tone: 'cmd' },
  { prompt: '#', text: 'repeat for every customer, forever…', tone: 'comment' },
  { prompt: '!', text: 'drift detected: someone kubectl-edited beta-inc-app replicas', tone: 'error' },
]

/**
 * The reconciliation lifecycle of a single node (`acme-corp`), told as a
 * stepped walkthrough for the ReconcileWalkthrough section (windflow-style
 * StepStage). Each step drives BOTH the right-hand reasoning/action panel AND
 * the left-hand "screen" that the section renders from existing primitives
 * (DataTable, YamlBlock, ResourceCard, StatusBadge, TerminalWindow) keyed off
 * the step id/index.
 *
 * Each step also carries a `view` label for the faux app-window title bar in the
 * left screen, so the walkthrough reads like a real tool being driven.
 *
 * @type {{ id: string, label: string, title: string, reasoning: string, action: string, view: string }[]}
 */
export const reconcileSteps = [
  {
    id: 'row',
    label: 'Row',
    title: 'A row appears',
    view: 'MySQL · node_configs',
    reasoning:
      'A new row lands in node_configs with is_active=1 — the on switch for this node.',
    action: 'DB: INSERT node_configs → acme-corp',
  },
  {
    id: 'sync',
    label: 'Sync',
    title: 'LynqHub syncs',
    view: 'kubectl',
    reasoning:
      'LynqHub polls MySQL on its syncInterval, sees the active row, and creates one LynqNode CR for it.',
    action: 'CREATE LynqNode/acme-corp-web-stack',
  },
  {
    id: 'render',
    label: 'Render',
    title: 'Template renders',
    view: 'web-stack.yaml',
    reasoning:
      'The web-stack LynqForm renders once for this row. {{ .uid }} becomes acme-corp, producing 3 concrete resources.',
    action: 'RENDER web-stack → 3 resources',
  },
  {
    id: 'apply-deployment',
    label: 'Apply',
    title: 'Deployment applied',
    view: 'Lynq Dashboard',
    reasoning:
      'Server-Side Apply creates the Deployment with field-manager: lynq. It starts Pending.',
    action: 'SSA apply Deployment/acme-corp-app',
  },
  {
    id: 'apply-net',
    label: 'Apply',
    title: 'Service & Ingress',
    view: 'Lynq Dashboard',
    reasoning:
      'Service and Ingress are applied in dependency order. All three now exist, still converging.',
    action: 'SSA apply Service/acme-corp-svc, Ingress/acme-corp-web',
  },
  {
    id: 'ready-pods',
    label: 'Ready',
    title: 'Pods become ready',
    view: 'Lynq Dashboard',
    reasoning:
      'The Deployment reports availableReplicas ≥ replicas; readiness gates pass one by one.',
    action: 'Deployment/acme-corp-app → Ready',
  },
  {
    id: 'done',
    label: 'Done',
    title: 'Node is Ready',
    view: 'kubectl',
    reasoning:
      'Every resource passed its readiness check, so the LynqNode Ready condition flips True. kubectl confirms 3/3.',
    action: 'LynqNode/acme-corp-web-stack → Ready 3/3',
  },
]
