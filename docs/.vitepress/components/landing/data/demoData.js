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

/* ── Faithful kubectl-style column formatter ──
   Left-pads every cell except the last to fixed widths so the rendered rows
   line up exactly like real `kubectl get` table output (monospace, space-
   separated columns — kubectl does not colorise `get` output). */
const cols = (widths, ...cells) =>
  cells.map((c, i) => (i === cells.length - 1 ? String(c) : String(c).padEnd(widths[i]))).join('')

const NODE_W = [22, 7, 9, 14]
const DEPLOY_W = [32, 8, 13, 12]
const SVC_W = [26, 12, 15, 10]
const ING_W = [26, 8, 20, 10, 8]

/**
 * A scripted, end-to-end operator session for the LiveTransform section, played
 * inside a single authentic terminal. Each step is one of:
 *   - `cmd`  a command line typed character-by-character. `prompt` defaults to
 *            the shell '$'; SQL steps set it to 'mysql>' / '    ->'.
 *   - `out`  an output line printed after `delay` ms (default fast)
 *   - `gap`  a blank spacer line
 * `tone` tints a line: 'ok' (subtle green payoff), 'dim' (muted — comments, ^C).
 *
 * Three scenarios play back-to-back so the whole data-driven lifecycle is
 * visible in one shell: (1) apply the manifests and watch READY 0/3 → 3/3;
 * (2) INSERT a new MySQL row → a new LynqNode + resources appear; (3) UPDATE
 * is_active → 0 → Lynq garbage-collects that node's Deployment/Service/Ingress,
 * with no `kubectl delete`. Domain-accurate: apiVersion operator.lynq.sh/v1,
 * CRD short names, `{uid}-app/-svc/-web` resource names.
 * @type {{ type:'cmd'|'out'|'gap', text?:string, prompt?:string, tone?:'ok'|'dim', delay?:number }[]}
 */
export const terminalSession = [
  { type: 'cmd', text: 'kubectl apply -f lynqhub.yaml' },
  { type: 'out', text: 'lynqhub.operator.lynq.sh/my-hub created', tone: 'ok' },
  { type: 'gap' },
  { type: 'cmd', text: 'kubectl apply -f lynqform.yaml' },
  { type: 'out', text: 'lynqform.operator.lynq.sh/web-stack created', tone: 'ok' },
  { type: 'gap' },

  // LynqHub has read the two active rows and is materialising LynqNodes.
  { type: 'cmd', text: 'kubectl get lynqnodes -w' },
  { type: 'out', text: cols(NODE_W, 'NAME', 'READY', 'DESIRED', 'CONDITIONS', 'AGE') },
  { type: 'out', text: cols(NODE_W, 'acme-corp-web-stack', '0/3', '3', 'Reconciling', '0s'), delay: 650 },
  { type: 'out', text: cols(NODE_W, 'beta-inc-web-stack', '0/3', '3', 'Reconciling', '0s'), delay: 750 },
  { type: 'out', text: cols(NODE_W, 'acme-corp-web-stack', '1/3', '3', 'Reconciling', '1s'), delay: 850 },
  { type: 'out', text: cols(NODE_W, 'beta-inc-web-stack', '2/3', '3', 'Reconciling', '2s'), delay: 850 },
  { type: 'out', text: cols(NODE_W, 'acme-corp-web-stack', '3/3', '3', 'Reconciled', '3s'), tone: 'ok', delay: 900 },
  { type: 'out', text: cols(NODE_W, 'beta-inc-web-stack', '3/3', '3', 'Reconciled', '4s'), tone: 'ok', delay: 700 },
  { type: 'out', text: '^C', tone: 'dim', delay: 550 },
  { type: 'gap' },

  // The real, named Kubernetes resources that now exist for each row.
  { type: 'cmd', text: 'kubectl get deploy,svc,ing' },
  { type: 'out', text: cols(DEPLOY_W, 'NAME', 'READY', 'UP-TO-DATE', 'AVAILABLE', 'AGE') },
  { type: 'out', text: cols(DEPLOY_W, 'deployment.apps/acme-corp-app', '1/1', '1', '1', '5s') },
  { type: 'out', text: cols(DEPLOY_W, 'deployment.apps/beta-inc-app', '1/1', '1', '1', '5s') },
  { type: 'gap' },
  { type: 'out', text: cols(SVC_W, 'NAME', 'TYPE', 'CLUSTER-IP', 'PORT(S)', 'AGE') },
  { type: 'out', text: cols(SVC_W, 'service/acme-corp-svc', 'ClusterIP', '10.96.12.31', '80/TCP', '5s') },
  { type: 'out', text: cols(SVC_W, 'service/beta-inc-svc', 'ClusterIP', '10.96.44.9', '80/TCP', '5s') },
  { type: 'gap' },
  { type: 'out', text: cols(ING_W, 'NAME', 'CLASS', 'HOSTS', 'PORTS', 'AGE') },
  { type: 'out', text: cols(ING_W, 'ingress/acme-corp-web', 'nginx', 'acme.example.com', '80', '5s') },
  { type: 'out', text: cols(ING_W, 'ingress/beta-inc-web', 'nginx', 'beta.example.com', '80', '5s') },
  { type: 'gap' },

  // ── Scenario 2: a new customer signs up → INSERT a row → a LynqNode appears.
  { type: 'out', text: '# a new customer signs up — just insert the row', tone: 'dim', delay: 500 },
  { type: 'cmd', text: 'mysql -h mysql.default.svc -u node_reader -p nodes' },
  { type: 'out', text: 'Enter password:', delay: 300 },
  { type: 'cmd', prompt: 'mysql>', text: 'INSERT INTO node_configs (node_id, is_active, subscription_plan, node_url)' },
  { type: 'cmd', prompt: '    ->', text: "VALUES ('delta-co', 1, 'pro', 'delta.example.com');" },
  { type: 'out', text: 'Query OK, 1 row affected (0.01 sec)', tone: 'ok', delay: 250 },
  { type: 'cmd', prompt: 'mysql>', text: 'exit' },
  { type: 'out', text: 'Bye', tone: 'dim' },
  { type: 'gap' },

  // LynqHub re-syncs and materialises a LynqNode for the new row.
  { type: 'out', text: '# LynqHub re-syncs and reconciles the new row', tone: 'dim', delay: 500 },
  { type: 'cmd', text: 'kubectl get lynqnodes -w' },
  { type: 'out', text: cols(NODE_W, 'NAME', 'READY', 'DESIRED', 'CONDITIONS', 'AGE') },
  { type: 'out', text: cols(NODE_W, 'acme-corp-web-stack', '3/3', '3', 'Reconciled', '3m') },
  { type: 'out', text: cols(NODE_W, 'beta-inc-web-stack', '3/3', '3', 'Reconciled', '3m') },
  { type: 'out', text: cols(NODE_W, 'delta-co-web-stack', '0/3', '3', 'Reconciling', '0s'), tone: 'ok', delay: 700 },
  { type: 'out', text: cols(NODE_W, 'delta-co-web-stack', '2/3', '3', 'Reconciling', '2s'), delay: 900 },
  { type: 'out', text: cols(NODE_W, 'delta-co-web-stack', '3/3', '3', 'Reconciled', '3s'), tone: 'ok', delay: 900 },
  { type: 'out', text: '^C', tone: 'dim', delay: 500 },
  { type: 'gap' },
  { type: 'cmd', text: 'kubectl get deploy' },
  { type: 'out', text: cols([16, 8], 'NAME', 'READY', 'AGE') },
  { type: 'out', text: cols([16, 8], 'acme-corp-app', '1/1', '3m') },
  { type: 'out', text: cols([16, 8], 'beta-inc-app', '1/1', '3m') },
  { type: 'out', text: cols([16, 8], 'delta-co-app', '1/1', '4s'), tone: 'ok' },
  { type: 'gap' },

  // ── Scenario 3: deactivate a row (is_active → 0) → its resources are GC'd.
  { type: 'out', text: '# acme-corp churns — flip is_active to 0, no kubectl delete', tone: 'dim', delay: 500 },
  { type: 'cmd', text: 'mysql -h mysql.default.svc -u node_reader -p nodes' },
  { type: 'out', text: 'Enter password:', delay: 300 },
  { type: 'cmd', prompt: 'mysql>', text: "UPDATE node_configs SET is_active = 0 WHERE node_id = 'acme-corp';" },
  { type: 'out', text: 'Query OK, 1 row affected (0.01 sec)' },
  { type: 'out', text: 'Rows matched: 1  Changed: 1  Warnings: 0', tone: 'ok', delay: 250 },
  { type: 'cmd', prompt: 'mysql>', text: 'exit' },
  { type: 'out', text: 'Bye', tone: 'dim' },
  { type: 'gap' },

  // The LynqNode is deleted and its Deployment/Service/Ingress garbage-collected.
  { type: 'out', text: '# Lynq deletes the LynqNode and cleans up its resources', tone: 'dim', delay: 500 },
  { type: 'cmd', text: 'kubectl get lynqnodes' },
  { type: 'out', text: cols(NODE_W, 'NAME', 'READY', 'DESIRED', 'CONDITIONS', 'AGE') },
  { type: 'out', text: cols(NODE_W, 'beta-inc-web-stack', '3/3', '3', 'Reconciled', '4m') },
  { type: 'out', text: cols(NODE_W, 'delta-co-web-stack', '3/3', '3', 'Reconciled', '1m') },
  { type: 'out', text: '# acme-corp-web-stack is gone', tone: 'dim', delay: 400 },
  { type: 'gap' },
  { type: 'cmd', text: 'kubectl get deploy' },
  { type: 'out', text: cols([16, 8], 'NAME', 'READY', 'AGE') },
  { type: 'out', text: cols([16, 8], 'beta-inc-app', '1/1', '4m') },
  { type: 'out', text: cols([16, 8], 'delta-co-app', '1/1', '1m') },
  { type: 'out', text: '# acme-corp-app removed automatically — no manual cleanup', tone: 'ok', delay: 400 },
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
