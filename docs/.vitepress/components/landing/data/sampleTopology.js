// Static demo TopologyData for the landing page topology graph.
// Shape matches dashboard/ui/src/types/lynq.ts:
//   TopologyNode { id, type, name, namespace, status, children?, metrics{desired,ready,failed}, metadata? }
//   TopologyEdge { source, target }
//   TopologyData { nodes, edges }
//
// Scenario: one hub (haulla-hub) → 2 forms (web-stack, worker) → 10 LynqNodes
// (8 ready, 1 pending, 1 failed). One node expands into a few resources
// (Deployment/Service/Ingress). One orphaned Job sits separate.

const NS = 'default'

// LynqNode leaf definitions: [id, name, status]
const webNodes = [
  ['node-acme-web', 'acme-corp', 'ready'],
  ['node-beta-web', 'beta-inc', 'ready'],
  ['node-gamma-web', 'gamma-llc', 'ready'],
  ['node-delta-web', 'delta-co', 'pending'],
  ['node-epsilon-web', 'epsilon-io', 'ready'],
]

const workerNodes = [
  ['node-acme-worker', 'acme-corp', 'ready'],
  ['node-beta-worker', 'beta-inc', 'ready'],
  ['node-gamma-worker', 'gamma-llc', 'failed'],
  ['node-delta-worker', 'delta-co', 'ready'],
  ['node-zeta-worker', 'zeta-ltd', 'ready'],
]

function leaf([id, name, status]) {
  return {
    id,
    type: 'node',
    name,
    namespace: NS,
    status,
    metrics: { desired: 1, ready: status === 'ready' ? 1 : 0, failed: status === 'failed' ? 1 : 0 },
  }
}

// Resource children under the first web node (acme-corp), matching the real example.
const acmeResources = [
  { id: 'res-acme-deploy', name: 'Deployment/acme-corp-web', status: 'ready' },
  { id: 'res-acme-svc', name: 'Service/acme-corp-web', status: 'ready' },
  { id: 'res-acme-ing', name: 'Ingress/acme-corp', status: 'ready' },
]

const resourceNodes = acmeResources.map((r) => ({
  id: r.id,
  type: 'resource',
  name: r.name,
  namespace: NS,
  status: r.status,
  metrics: { desired: 1, ready: r.status === 'ready' ? 1 : 0, failed: 0 },
  metadata: { deletionPolicy: 'Delete' },
}))

const allNodeLeaves = [...webNodes, ...workerNodes]

// Aggregate for the hub/forms (buildHierarchy recomputes from leaves, but keep
// consistent static values too — hub reads healthy at 10/10 like the example).
const webReady = webNodes.filter((n) => n[2] === 'ready').length
const webFailed = webNodes.filter((n) => n[2] === 'failed').length
const workerReady = workerNodes.filter((n) => n[2] === 'ready').length
const workerFailed = workerNodes.filter((n) => n[2] === 'failed').length

export const sampleTopology = {
  nodes: [
    // Hub
    {
      id: 'hub-haulla',
      type: 'hub',
      name: 'haulla-hub',
      namespace: NS,
      status: 'ready',
      children: ['form-web-stack', 'form-worker'],
      metrics: { desired: 10, ready: 10, failed: 0 },
    },

    // Forms
    {
      id: 'form-web-stack',
      type: 'form',
      name: 'web-stack',
      namespace: NS,
      status: 'ready',
      children: webNodes.map((n) => n[0]),
      metrics: { desired: webNodes.length, ready: webReady, failed: webFailed },
    },
    {
      id: 'form-worker',
      type: 'form',
      name: 'worker',
      namespace: NS,
      status: 'ready',
      children: workerNodes.map((n) => n[0]),
      metrics: { desired: workerNodes.length, ready: workerReady, failed: workerFailed },
    },

    // LynqNode leaves — attach resource children to the first web node
    ...allNodeLeaves.map((n) => {
      const base = leaf(n)
      if (n[0] === 'node-acme-web') base.children = acmeResources.map((r) => r.id)
      return base
    }),

    // Resource children
    ...resourceNodes,

    // Orphaned Job (sits separate — not linked into the hub tree)
    {
      id: 'orphan-db-migration',
      type: 'orphan',
      name: 'Job/haulla-db-migration',
      namespace: NS,
      status: 'failed',
      metrics: { desired: 1, ready: 0, failed: 1 },
      metadata: {
        orphaned: true,
        orphanedReason: 'RemovedFromTemplate',
        orphanedAt: '2026-06-30T12:00:00Z',
        originalNode: 'node-acme-web',
      },
    },
  ],

  edges: [
    // hub → forms
    { source: 'hub-haulla', target: 'form-web-stack' },
    { source: 'hub-haulla', target: 'form-worker' },

    // form → nodes
    ...webNodes.map((n) => ({ source: 'form-web-stack', target: n[0] })),
    ...workerNodes.map((n) => ({ source: 'form-worker', target: n[0] })),

    // node → resources
    ...acmeResources.map((r) => ({ source: 'node-acme-web', target: r.id })),
  ],
}

export default sampleTopology
