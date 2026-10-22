// Health classification for Lynq CRs, shared by every page that reads raw CRs.
//
// The LynqNode rule mirrors the operator's classifyNode (internal/controller/node_health.go),
// and the BFF topology mirrors it again (dashboard/bff/internal/api/health.go). All three must
// agree: when the dashboard counted every Ready!=True node as failed, it showed a node that was
// merely recovering a lost pod as broken while the operator (hub status, alerts, rollout) did
// not. Change them together, and keep health.test.ts and health_test.go on the same cases.
//
// Hubs and forms are classified from their own LynqNodes with the same rule: any failed node
// fails them. LynqHub status.ready/status.failed are not used, because their meaning depends on
// the operator version (see HubNodeHealth).
//
// The 'pending' key means "progressing": rolling out, recovering, or not reconciled yet. It is
// kept as the key (URLs, persisted topology filters and colors use it) and only labelled
// "Progressing" in the UI.

import type { Condition, LynqForm, LynqHub, LynqNode, RolloutPhase, RolloutStatus } from '@/types/lynq'

export type Health = 'ready' | 'pending' | 'failed'

function isTrue(conditions: Condition[] | undefined, type: string): boolean {
  return conditions?.some((c) => c.type === type && c.status === 'True') ?? false
}

function isFalse(conditions: Condition[] | undefined, type: string): boolean {
  return conditions?.some((c) => c.type === type && c.status === 'False') ?? false
}

// classifyNode: failed resources, a conflict or Degraded=True is failed, and failure wins over
// a stale Ready=True. Ready=True is ready. Everything else (Ready=False/NotAllResourcesReady
// with Degraded=False, or no conditions yet) is progressing.
export function classifyNode(node: Pick<LynqNode, 'status'>): Health {
  const conds = node.status?.conditions
  if (
    (node.status?.failedResources ?? 0) > 0 ||
    isTrue(conds, 'Conflicted') ||
    isTrue(conds, 'Degraded')
  ) {
    return 'failed'
  }
  if (isTrue(conds, 'Ready')) return 'ready'
  return 'pending'
}

export interface HealthCounts {
  ready: number
  pending: number
  failed: number
}

const emptyCounts = (): HealthCounts => ({ ready: 0, pending: 0, failed: 0 })

export function countHealth<T>(items: T[], classify: (item: T) => Health): HealthCounts {
  const counts = emptyCounts()
  for (const item of items) counts[classify(item)]++
  return counts
}

// NodeHealthIndex classifies every LynqNode once and tallies it by form and by hub, so a page
// listing F forms or H hubs makes one pass over the nodes instead of F×N.
export interface NodeHealthIndex {
  byForm: Map<string, HealthCounts>
  byHub: Map<string, HealthCounts>
  // The form's own LynqNodes, for its rollout breakdown (rolloutView)
  nodesByForm: Map<string, RolloutNode[]>
}

const HUB_LABEL = 'lynq.sh/hub'
const nsKey = (namespace: string, name: string) => `${namespace}/${name}`

export function indexNodeHealth(
  nodes: Pick<LynqNode, 'metadata' | 'spec' | 'status'>[],
): NodeHealthIndex {
  const index: NodeHealthIndex = { byForm: new Map(), byHub: new Map(), nodesByForm: new Map() }
  const tally = (map: Map<string, HealthCounts>, key: string, health: Health) => {
    const counts = map.get(key) ?? emptyCounts()
    counts[health]++
    map.set(key, counts)
  }
  for (const node of nodes) {
    const health = classifyNode(node)
    const namespace = node.metadata.namespace
    if (node.spec?.templateRef) {
      const key = nsKey(namespace, node.spec.templateRef)
      tally(index.byForm, key, health)
      const own = index.nodesByForm.get(key)
      if (own) own.push(node)
      else index.nodesByForm.set(key, [node])
    }
    const hub = node.metadata.labels?.[HUB_LABEL]
    if (hub) tally(index.byHub, nsKey(namespace, hub), health)
  }
  return index
}

// formNodeCounts is the classification of one form's LynqNodes. LynqFormStatus has no
// failed-node count, so the form pages derive it from the nodes themselves.
export function formNodeCounts(
  index: NodeHealthIndex,
  form: { name: string; namespace: string },
): HealthCounts {
  return index.byForm.get(nsKey(form.namespace, form.name)) ?? emptyCounts()
}

// classifyForm: an invalid template, a stalled rollout (phase Failed) or any failed LynqNode of
// the form is failed — the hub's rule, so one failed node turns both red. Applied=True means
// every node is Ready. Anything else (NotAllNodesReady, NoNodes) is progressing.
//
// nodes is null until the node list has loaded. The form's own failure signals still decide;
// otherwise the result is null (unknown) rather than a health that could hide a failed node.
export function classifyForm(
  form: Pick<LynqForm, 'status'>,
  nodes: HealthCounts | null,
): Health | null {
  const conds = form.status?.conditions
  if (isFalse(conds, 'Valid') || form.status?.rollout?.phase === 'Failed') return 'failed'
  if (!nodes) return null
  if (nodes.failed > 0) return 'failed'
  if (isTrue(conds, 'Applied')) return 'ready'
  return 'pending'
}

// hasActiveRollout: Idle (or no phase) means there is no rollout to show
export function hasActiveRollout(
  rollout?: RolloutStatus,
): rollout is RolloutStatus & { phase: RolloutPhase } {
  return !!rollout?.phase && rollout.phase !== 'Idle'
}

// formNodes is one form's LynqNodes
export function formNodes(
  index: NodeHealthIndex,
  form: { name: string; namespace: string },
): RolloutNode[] {
  return index.nodesByForm.get(nsKey(form.namespace, form.name)) ?? []
}

// isRolloutPending mirrors the operator's updateCompleted (internal/controller/
// rollout_progress.go): a node is still in its LynqForm rollout while its completion marker is
// present and does not name its current template generation. An empty marker (just updated)
// and one left for an older generation (a rollback, then a re-upgrade) are both pending. A node
// without the marker predates it and counts as completed, but only when markerSupported (see
// RolloutView); otherwise it is pending.
export function isRolloutPending(
  annotations: Record<string, string> | undefined,
  markerSupported = true,
): boolean {
  const completed = annotations?.['lynq.sh/rollout-completed-generation']
  if (completed === undefined) return !markerSupported
  return completed === '' || completed !== annotations?.['lynq.sh/template-generation']
}

// RolloutNode is what the rollout breakdown reads from a LynqNode
export type RolloutNode = Pick<LynqNode, 'metadata' | 'status'>

// RolloutView is a LynqForm rollout as the dashboard shows it, counted from the form's LynqNodes
// (dashboard/bff/internal/api/health.go newRolloutView mirrors it; both run the "rollout" cases
// of dashboard/testdata/health-cases.json).
//
// An updated node (template generation == the form's generation) is one of:
//   - readyUpdated: Ready for its current spec (the operator's reachedReadyForCurrentSpec).
//   - recovering: not Ready, but it completed this rollout (isRolloutPending is false, which
//     includes a node without the completion marker) and is not failed. It regressed after the
//     rollout reached it (lost pod, node drain, external restart), so it does not hold the
//     rollout back.
//   - updating: everything else. A failed node stays here: it keeps its maxSkew slot, and the
//     operator reports the rollout Failed over it whether or not it completed.
//
// A node without the completion marker counts as completed only when another updated node of
// the form carries it. When none does, the operator predates the marker (Lynq v1.1.23-alpha.1
// and earlier) and a node without it cannot be told apart from one still rolling out (the tail
// of a real rollout looks like a lost pod), so every updated node that is not Ready is updating,
// nothing is recovering and the phase is the reported one.
//
// completed counts the updated nodes that completed this rollout, the operator's rollout progress
// (lynqform_rollout_progress = completed / total): readyUpdated plus every other updated node that
// is not pending (isRolloutPending), failed ones included — a failed node completed its update
// too, and the operator counts it there. Without the marker (an older operator) no node that is
// not readyUpdated is completed, so completed is readyUpdated: that operator's own progress.
//
// phase is the operator's status.rollout.phase (reportedPhase), except one case: an operator
// that writes the marker but counts a recovering node as updating turns a Complete rollout back
// to InProgress on a lost pod. When every node is updated and the only nodes not Ready are
// recovering, phase is Complete and reportedPhase keeps the operator's InProgress. Failed is
// never overridden.
export interface RolloutView {
  phase: RolloutPhase
  reportedPhase: RolloutPhase
  total: number
  updated: number
  readyUpdated: number
  updating: number
  recovering: number
  completed: number
}

// rolloutView returns null when there is no rollout to show (hasActiveRollout). nodes are the
// form's own LynqNodes, or null until the node list has loaded; the counts then come from the
// form status and the phase is the reported one.
export function rolloutView(
  form: Pick<LynqForm, 'metadata' | 'status'>,
  nodes: RolloutNode[] | null,
): RolloutView | null {
  const rollout = form.status?.rollout
  if (!hasActiveRollout(rollout)) return null
  const reportedPhase = rollout.phase
  if (!nodes) {
    return {
      phase: reportedPhase,
      reportedPhase,
      total: rollout.totalNodes ?? 0,
      updated: rollout.updatedNodes ?? 0,
      readyUpdated: rollout.readyUpdatedNodes ?? 0,
      updating: rollout.updatingNodes ?? 0,
      recovering: 0,
      // updatingNodes is every updated node whose update has not completed (Lynq v1.1.23-alpha.1
      // and earlier: every updated node not Ready for its spec), so this is the operator's progress
      completed: (rollout.updatedNodes ?? 0) - (rollout.updatingNodes ?? 0),
    }
  }

  const target = String(form.metadata.generation ?? rollout.targetGeneration ?? '')
  const view: RolloutView = {
    phase: reportedPhase,
    reportedPhase,
    total: nodes.length,
    updated: 0,
    readyUpdated: 0,
    updating: 0,
    recovering: 0,
    completed: 0,
  }
  const updated = nodes.filter(
    (node) => node.metadata.annotations?.['lynq.sh/template-generation'] === target,
  )
  const markerSupported = updated.some(
    (node) => node.metadata.annotations?.['lynq.sh/rollout-completed-generation'] !== undefined,
  )
  for (const node of updated) {
    const annotations = node.metadata.annotations
    view.updated++
    const health = classifyNode(node)
    const pending = isRolloutPending(annotations, markerSupported)
    if (health === 'ready' && node.status?.observedGeneration === node.metadata.generation) {
      view.readyUpdated++
      view.completed++
      continue
    }
    if (!pending) view.completed++
    if (health !== 'failed' && !pending) {
      view.recovering++
    } else {
      view.updating++
    }
  }
  if (
    reportedPhase === 'InProgress' &&
    view.total > 0 &&
    view.updated === view.total &&
    view.updating === 0 &&
    view.recovering > 0
  ) {
    view.phase = 'Complete'
  }
  return view
}

// rolloutPercent is the share of nodes that completed the rollout, rounded down so the bar only
// reaches 100% once every node has completed.
export function rolloutPercent(view: Pick<RolloutView, 'completed' | 'total'>): number {
  return view.total > 0 ? Math.floor((view.completed / view.total) * 100) : 0
}

// HubNodeHealth is a hub's node breakdown, counted from its LynqNodes rather than read from
// status.ready/status.failed: Lynq v1.1.23-alpha.1 and earlier count every Ready!=True node in
// status.failed, so a node recovering a pod would fail the hub there while the Nodes page shows
// it progressing. desired still comes from status.desired; desired nodes that do not exist yet
// (not created, or creation held back by maxSkew) are notCreated and count as progressing.
export interface HubNodeHealth {
  desired: number
  ready: number
  progressing: number
  failed: number
  notCreated: number
}

export function hubNodeHealth(
  hub: Pick<LynqHub, 'metadata' | 'status'>,
  index: NodeHealthIndex,
): HubNodeHealth {
  const counts = index.byHub.get(nsKey(hub.metadata.namespace, hub.metadata.name)) ?? emptyCounts()
  const desired = hub.status?.desired ?? 0
  // Nodes beyond desired (being deleted) leave nothing to create, not a negative count
  const notCreated = Math.max(desired - counts.ready - counts.pending - counts.failed, 0)
  return {
    desired,
    ready: counts.ready,
    progressing: counts.pending + notCreated,
    failed: counts.failed,
    notCreated,
  }
}

// isHubDisconnected: the hub Ready condition reports the datasource connection, not node health.
export function isHubDisconnected(hub: Pick<LynqHub, 'status'>): boolean {
  return isFalse(hub.status?.conditions, 'Ready')
}

// classifyHub: a lost datasource or any failed LynqNode is failed. A connected hub with nothing
// progressing is ready. Otherwise progressing. nodes is null until the node list has loaded;
// then only a lost datasource is known, and the result is null (unknown) otherwise.
export function classifyHub(
  hub: Pick<LynqHub, 'status'>,
  nodes: HubNodeHealth | null,
): Health | null {
  if (isHubDisconnected(hub)) return 'failed'
  if (!nodes) return null
  if (nodes.failed > 0) return 'failed'
  if (isTrue(hub.status?.conditions, 'Ready') && nodes.progressing === 0) return 'ready'
  return 'pending'
}

// Conditions whose True means trouble. Every other condition reads True as good.
const NEGATIVE_POLARITY = new Set(['Degraded', 'Conflicted'])

// Reasons for a positive condition being False that mean "still progressing", not "broken":
// a node rolling out or recovering a pod, or a form waiting on such nodes or on its first node.
// The node or form itself decides whether it failed (classifyNode, classifyForm).
const PROGRESSING_REASONS = new Set(['NotAllResourcesReady', 'NotAllNodesReady', 'NoNodes'])

export type ConditionTone = 'good' | 'bad' | 'unknown'

// conditionTone gives a condition its color by meaning, not by status: Degraded=False is the
// healthy state of a node that is merely progressing and must not render as a red X, and
// Ready=False/NotAllResourcesReady is progressing (amber), the same as the node's own state.
export function conditionTone(
  condition: Pick<Condition, 'type' | 'status'> & { reason?: string },
): ConditionTone {
  if (condition.status !== 'True' && condition.status !== 'False') return 'unknown'
  const positive = condition.status === 'True'
  if (positive === NEGATIVE_POLARITY.has(condition.type)) {
    return condition.reason && PROGRESSING_REASONS.has(condition.reason) ? 'unknown' : 'bad'
  }
  return 'good'
}
