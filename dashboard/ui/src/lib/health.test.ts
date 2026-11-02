import { describe, expect, it } from 'vitest'
import type { LynqForm, LynqHub, LynqNode } from '@/types/lynq'
// The same table bff/internal/api/health_test.go runs, so the two classifiers cannot drift apart
import sharedCases from '../../../testdata/health-cases.json'
import {
  classifyForm,
  classifyHub,
  classifyNode,
  conditionTone,
  formNodeCounts,
  hubNodeHealth,
  indexNodeHealth,
  isHubDisconnected,
  isRolloutPending,
  rolloutPercent,
  rolloutView,
  type Health,
  type HubNodeHealth,
  type RolloutNode,
  type RolloutView,
} from './health'

interface SharedCase {
  name: string
  status: Record<string, unknown>
  // Form and hub cases: the statuses of their own LynqNodes
  nodes?: Record<string, unknown>[]
  want: Health
  wantCounts?: Omit<HubNodeHealth, 'desired'>
}

const cases = sharedCases as unknown as Record<'node' | 'form' | 'hub', SharedCase[]>

interface RolloutCase {
  name: string
  generation: number
  status: Record<string, unknown>
  nodes: { annotations: Record<string, string>; generation: number; status: object }[]
  want: Omit<RolloutView, 'reportedPhase'>
}

const rolloutCases = (sharedCases as unknown as { rollout: RolloutCase[] }).rollout

const node = (name: string, form: string, namespace: string, status: object, hub = 'mysql') =>
  ({
    metadata: { name, namespace, labels: { 'lynq.sh/hub': hub } },
    spec: { templateRef: form },
    status,
  }) as unknown as LynqNode

// A case's nodes, as nodes of the form "web" and the hub "mysql" in "default"
const ownNodes = (c: SharedCase) =>
  indexNodeHealth((c.nodes ?? []).map((status, i) => node(`n${i}-web`, 'web', 'default', status)))

const ownMeta = (name: string) => ({ metadata: { name, namespace: 'default' } })

describe('shared health cases', () => {
  it.each(cases.node)('node: $name', ({ status, want }) => {
    expect(classifyNode({ status } as unknown as LynqNode)).toBe(want)
  })
  it.each(cases.form)('form: $name', (c) => {
    const counts = formNodeCounts(ownNodes(c), ownMeta('web').metadata)
    expect(classifyForm({ status: c.status } as unknown as LynqForm, counts)).toBe(c.want)
  })
  it.each(cases.hub)('hub: $name', (c) => {
    const hub = { ...ownMeta('mysql'), status: c.status } as unknown as LynqHub
    const health = hubNodeHealth(hub, ownNodes(c))
    expect(classifyHub(hub, health)).toBe(c.want)
    const { ready, progressing, failed, notCreated } = health
    expect({ ready, progressing, failed, notCreated }).toEqual(c.wantCounts)
  })
})

describe('shared rollout cases', () => {
  it.each(rolloutCases)('$name', (c) => {
    const form = {
      metadata: { name: 'web', namespace: 'default', generation: c.generation },
      status: c.status,
    } as unknown as LynqForm
    const nodes = c.nodes.map(
      (n, i) =>
        ({
          metadata: { name: `n${i}-web`, namespace: 'default', annotations: n.annotations, generation: n.generation },
          status: n.status,
        }) as unknown as RolloutNode,
    )
    const reportedPhase = (c.status.rollout as { phase: string }).phase
    expect(rolloutView(form, nodes)).toEqual({ ...c.want, reportedPhase })
  })
})

describe('rolloutView', () => {
  const rollout = {
    phase: 'InProgress',
    totalNodes: 126,
    updatedNodes: 126,
    updatingNodes: 1,
    readyUpdatedNodes: 125,
  }
  const form = (r: object) =>
    ({ metadata: { name: 'web', namespace: 'default', generation: 3 }, status: { rollout: r } }) as unknown as LynqForm

  it('shows no rollout when there is none or it is Idle', () => {
    expect(rolloutView(form({}), [])).toBeNull()
    expect(rolloutView(form({ phase: 'Idle' }), [])).toBeNull()
  })
  it('takes the counts and phase from the form status until the node list loads', () => {
    expect(rolloutView(form(rollout), null)).toEqual({
      phase: 'InProgress',
      reportedPhase: 'InProgress',
      total: 126,
      updated: 126,
      readyUpdated: 125,
      updating: 1,
      recovering: 0,
      completed: 125,
    })
  })
})

describe('rolloutPercent', () => {
  it.each([
    [0, 0, 0],
    [1, 2, 50],
    // 125/126 is 99.2%: the bar reaches 100% only once every node completed
    [125, 126, 99],
    [199, 200, 99],
    [126, 126, 100],
  ])('%i of %i completed is %i%%', (completed, total, want) => {
    expect(rolloutPercent({ completed, total })).toBe(want)
  })
})

describe('node list not loaded yet', () => {
  const connected = {
    status: { conditions: [{ type: 'Ready', status: 'True', reason: 'DatabaseConnected' }] },
  } as unknown as LynqHub

  it('leaves a connected hub unknown instead of guessing a state', () => {
    expect(classifyHub(connected, null)).toBeNull()
  })
  it('still fails a hub whose datasource is lost', () => {
    const hub = {
      status: { conditions: [{ type: 'Ready', status: 'False', reason: 'DatabaseConnectionFailed' }] },
    } as unknown as LynqHub
    expect(classifyHub(hub, null)).toBe('failed')
  })
  it('leaves a form unknown unless the form itself failed', () => {
    const applied = {
      status: { conditions: [{ type: 'Applied', status: 'True', reason: 'AllNodesReady' }] },
    } as unknown as LynqForm
    const stalled = { status: { rollout: { phase: 'Failed' } } } as unknown as LynqForm
    expect(classifyForm(applied, null)).toBeNull()
    expect(classifyForm(stalled, null)).toBe('failed')
  })
})

describe('isHubDisconnected', () => {
  it('reads Ready=False as a lost datasource', () => {
    const hub = {
      status: { conditions: [{ type: 'Ready', status: 'False', reason: 'DatabaseConnectionFailed' }] },
    } as unknown as LynqHub
    expect(isHubDisconnected(hub)).toBe(true)
  })
  it('does not treat a missing condition as disconnected', () => {
    expect(isHubDisconnected({ status: { conditions: [] } } as unknown as LynqHub)).toBe(false)
  })
})

describe('indexNodeHealth', () => {
  const nodes = [
    node('a', 'web', 'default', { conditions: [{ type: 'Ready', status: 'True' }] }),
    node('b', 'web', 'default', {
      conditions: [
        { type: 'Ready', status: 'False', reason: 'NotAllResourcesReady' },
        { type: 'Degraded', status: 'False', reason: 'Healthy' },
      ],
    }),
    node('c', 'web', 'default', { failedResources: 1 }),
    node('d', 'worker', 'default', { failedResources: 1 }, 'postgres'),
    node('e', 'web', 'other', { failedResources: 1 }),
  ]
  const index = indexNodeHealth(nodes)

  it('counts only a form’s own nodes in its namespace', () => {
    expect(formNodeCounts(index, { name: 'web', namespace: 'default' })).toEqual({
      ready: 1,
      pending: 1,
      failed: 1,
    })
  })
  it('counts only a hub’s own nodes in its namespace', () => {
    const hub = { metadata: { name: 'mysql', namespace: 'default' }, status: { desired: 3 } }
    expect(hubNodeHealth(hub as unknown as LynqHub, index)).toEqual({
      desired: 3,
      ready: 1,
      progressing: 1,
      failed: 1,
      notCreated: 0,
    })
  })
  it('gives a form or hub without nodes zero counts', () => {
    expect(formNodeCounts(index, { name: 'api', namespace: 'default' })).toEqual({
      ready: 0,
      pending: 0,
      failed: 0,
    })
  })
})

describe('isRolloutPending', () => {
  const ann = (gen: string, completed?: string) => ({
    'lynq.sh/template-generation': gen,
    ...(completed === undefined ? {} : { 'lynq.sh/rollout-completed-generation': completed }),
  })
  it.each([
    ['just updated (empty marker)', ann('4', ''), true],
    ['completed for the current generation', ann('4', '4'), false],
    ['marker left for an older generation', ann('4', '3'), true],
    ['no marker (predates it)', ann('4'), false],
  ] as const)('%s', (_, annotations, want) => {
    expect(isRolloutPending(annotations)).toBe(want)
  })
  it('is not pending without annotations', () => {
    expect(isRolloutPending(undefined)).toBe(false)
  })
  it('is pending without the marker when the operator does not write it', () => {
    expect(isRolloutPending(ann('4'), false)).toBe(true)
    expect(isRolloutPending(ann('4', '4'), false)).toBe(false)
  })
})

describe('conditionTone', () => {
  it.each([
    ['Ready', 'True', undefined, 'good'],
    ['Ready', 'False', undefined, 'bad'],
    ['Applied', 'True', undefined, 'good'],
    ['Valid', 'False', 'ValidationFailed', 'bad'],
    // Negative-polarity conditions: False is the healthy state
    ['Degraded', 'False', 'Healthy', 'good'],
    ['Degraded', 'True', 'ResourcesFailed', 'bad'],
    ['Conflicted', 'False', undefined, 'good'],
    ['Conflicted', 'True', undefined, 'bad'],
    ['Ready', 'Unknown', undefined, 'unknown'],
    // Progressing, not broken: the same amber as the node's or form's own state
    ['Ready', 'False', 'NotAllResourcesReady', 'unknown'],
    ['Applied', 'False', 'NotAllNodesReady', 'unknown'],
    ['Applied', 'False', 'NoNodes', 'unknown'],
    // A lost datasource is broken
    ['Ready', 'False', 'DatabaseConnectionFailed', 'bad'],
  ] as const)('%s=%s (%s) is %s', (type, status, reason, want) => {
    expect(conditionTone({ type, status, reason })).toBe(want)
  })
})
