<template>
  <section class="scale bg-lynq-bg" id="scale">
    <div class="scale-inner mx-auto">
      <SectionHeader
        label="Rollout safety"
        title="A big update, without the thundering herd"
        subtitle="A template edit, a bulk update, or a large insert can touch — or create — nodes across the whole graph at once, and doing them all together would stampede your API server. maxSkew caps how many change at a time. Pick a trigger, drag maxSkew, and watch it roll through the topology."
        accent="purple"
      />

      <div class="skew">
        <div class="skew-head">
          <div class="seg" role="tablist">
            <button
              v-for="(sc, key) in SCENARIOS"
              :key="key"
              class="seg-btn"
              :class="{ on: scenario === key }"
              @click="scenario = key"
            >{{ sc.tab }}</button>
          </div>
          <div class="ctrl">
            <span class="ctrl-lbl">maxSkew</span>
            <input
              class="ctrl-range"
              type="range"
              min="1"
              max="10"
              step="1"
              v-model.number="maxSkew"
              aria-label="maxSkew concurrency cap"
            />
            <span class="ctrl-val">{{ maxSkew }}</span>
          </div>
        </div>

        <div class="trigger">
          <span class="trg-tag">{{ active.tag }}</span>
          <code class="trg-cmd">{{ active.trigger }}</code>
          <span class="trg-sub">{{ active.triggerSub }}</span>
        </div>

        <!-- Real dashboard topology layout: d3 radial cluster (hub → forms →
             nodes) with the same radialCurve edges. Positions are computed once
             with d3-hierarchy; only per-node state is reactive, so the graph
             animates (creation, state changes) without ever re-laying-out. -->
        <div class="topo">
          <svg
            v-if="ready"
            class="topo-svg"
            :class="{ 'is-insert': scenario === 'insert' }"
            :viewBox="viewBox"
            preserveAspectRatio="xMidYMid meet"
            aria-hidden="true"
          >
            <g class="edges">
              <path
                v-for="e in edges"
                :key="e.key"
                class="edge"
                :class="e.idx == null ? 'edge-hub' : 'st-' + states[e.idx]"
                :d="e.d"
              />
            </g>

            <!-- hub -->
            <g class="hub" :transform="`translate(${hub.px},${hub.py})`">
              <circle class="hub-halo" r="30" />
              <circle class="hub-track" r="22" />
              <circle
                class="hub-prog"
                r="22"
                transform="rotate(-90)"
                :style="{ strokeDasharray: HUB_C, strokeDashoffset: HUB_C * (1 - readyFraction) }"
              />
              <circle class="hub-core" r="12" />
              <text class="hub-lbl" y="1">Hub</text>
            </g>

            <!-- forms -->
            <g v-for="f in formNodes" :key="f.id" class="form" :transform="`translate(${f.px},${f.py})`">
              <circle class="form-ring" r="10" />
              <circle class="form-core" r="5.5" />
            </g>

            <!-- nodes -->
            <g
              v-for="n in leafNodes"
              :key="n.idx"
              class="node"
              :class="'st-' + states[n.idx]"
              :transform="`translate(${n.px},${n.py})`"
            >
              <circle class="node-core node-inner" r="6.5" />
            </g>
          </svg>
        </div>

        <div class="stat">
          <span class="stat-item"><i class="lg lg-upd"></i> {{ active.verb }} <b>{{ updatingCount }}</b> / {{ maxSkew }}</span>
          <span class="stat-item"><i class="lg lg-new"></i> {{ active.verbDone }} <b>{{ updatedCount }}</b> / {{ targeted }}</span>
          <span v-if="skippedCount" class="stat-item"><i class="lg lg-skip"></i> {{ skippedCount }} unaffected</span>
          <span v-else class="stat-item"><i class="lg lg-old"></i> pending {{ pendingCount }}</span>
        </div>

        <p class="skew-note">
          {{ active.note }}
          <span class="skew-drag">Drag maxSkew ↑ for a faster rollout, ↓ for a gentler one.</span>
        </p>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import SectionHeader from '../primitives/SectionHeader.vue'

// ── topology data: hub → 2 forms → node fans (same shape the dashboard uses) ──
const FORMS = ['worker', 'web-stack']
const PER_FORM = 18
const count = FORMS.length * PER_FORM // 36
const R = 152 // outer radius (leaves); forms sit at R/2
const PAD = 46
const HUB_C = +(2 * Math.PI * 22).toFixed(2)

function buildTree() {
  return {
    id: 'hub',
    type: 'hub',
    children: FORMS.map((name, fi) => ({
      id: name,
      type: 'form',
      children: Array.from({ length: PER_FORM }, (_, k) => ({
        id: `${name}-${k}`,
        type: 'node',
        idx: fi * PER_FORM + k,
      })),
    })),
  }
}

// radialCurve — identical formula to TopologyGraph.vue (bezier along the radii)
function radialCurve(s, t) {
  const midR = (s.rad + t.rad) / 2
  const c1x = midR * Math.cos(s.ang - Math.PI / 2)
  const c1y = midR * Math.sin(s.ang - Math.PI / 2)
  const c2x = midR * Math.cos(t.ang - Math.PI / 2)
  const c2y = midR * Math.sin(t.ang - Math.PI / 2)
  return `M${s.px},${s.py}C${c1x},${c1y} ${c2x},${c2y} ${t.px},${t.py}`
}

const ready = ref(false)
const viewBox = ref(`${-R - PAD} ${-R - PAD} ${2 * (R + PAD)} ${2 * (R + PAD)}`)
const hub = ref({ px: 0, py: 0 })
const formNodes = ref([])
const leafNodes = ref([])
const edges = ref([])

// ── scenarios ──
const maxSkew = ref(4)
const scenario = ref('template')
const SCENARIOS = {
  template: {
    tab: 'Template edit',
    tag: 'YAML',
    verb: 'updating',
    verbDone: 'updated',
    trigger: 'web-stack.yaml edited',
    triggerSub: 'every node re-renders → all reconcile',
    note: 'A single template edit reconciles every node — a thundering herd that can melt the control plane if it all happens at once. maxSkew rolls it through a few at a time while the rest keep serving the current version.',
  },
  rows: {
    tab: 'Bulk row update',
    tag: 'SQL',
    verb: 'updating',
    verbDone: 'updated',
    trigger: "UPDATE node_configs SET plan='pro' WHERE region='us'",
    triggerSub: 'only matched rows reconcile — the rest are untouched',
    note: "A bulk UPDATE only matches the rows in its WHERE clause, so just those nodes reconcile — the unaffected ones stay on the current version, untouched. maxSkew still paces the ones that do change, a few at a time.",
  },
  insert: {
    tab: 'Large insert',
    tag: 'SQL',
    verb: 'creating',
    verbDone: 'created',
    trigger: `INSERT INTO node_configs … ${count} new rows`,
    triggerSub: `${count} new nodes → each provisioned from scratch`,
    note: 'A large insert adds a whole batch of brand-new nodes at once. Rather than creating every Deployment, Service and Ingress in one burst, maxSkew provisions them a few at a time — the cluster absorbs the new load smoothly instead of spiking.',
  },
}
const active = computed(() => SCENARIOS[scenario.value])

// which nodes the "bulk update" WHERE clause matches (~22 of 36, scattered)
const ROW_TARGETS = (() => {
  const order = Array.from({ length: count }, (_, i) => i).sort(
    (a, b) => ((a * 37 + 13) % 97) - ((b * 37 + 13) % 97)
  )
  return new Set(order.slice(0, 22))
})()

// ── scheduler ──
const states = ref(new Array(count).fill('pending'))
let remaining = new Array(count).fill(0)
let total = new Array(count).fill(1)
let nextIdx = 0
let matIdx = count
let doneAt = 0
let timer = null
let reduced = false
const TICK = 110
const MAT_PER_TICK = 1

const updatingCount = computed(() => states.value.filter((s) => s === 'updating').length)
const updatedCount = computed(() => states.value.filter((s) => s === 'updated').length)
const skippedCount = computed(() => states.value.filter((s) => s === 'skip').length)
const targeted = computed(() => count - skippedCount.value)
const pendingCount = computed(() => targeted.value - updatingCount.value - updatedCount.value)
const readyFraction = computed(() => (targeted.value ? updatedCount.value / targeted.value : 0))

function initStates() {
  const insert = scenario.value === 'insert'
  const rows = scenario.value === 'rows'
  return Array.from({ length: count }, (_, i) => {
    if (insert) return 'empty'
    if (rows && !ROW_TARGETS.has(i)) return 'skip' // WHERE clause miss → untouched
    return 'pending'
  })
}

function reset() {
  if (reduced) {
    states.value = initStates().map((s) => (s === 'skip' ? 'skip' : 'updated'))
    return
  }
  states.value = initStates()
  remaining = new Array(count).fill(0)
  total = new Array(count).fill(1)
  nextIdx = 0
  matIdx = scenario.value === 'insert' ? 0 : count
  doneAt = 0
}

function tick() {
  const s = states.value
  // insert: new rows land fast (empty → pending), ahead of the throttled creation
  if (matIdx < count) {
    const to = Math.min(count, matIdx + MAT_PER_TICK)
    for (let i = matIdx; i < to; i++) s[i] = 'pending'
    matIdx = to
  }
  for (let i = 0; i < count; i++) {
    if (s[i] === 'updating') {
      remaining[i] -= TICK
      if (remaining[i] <= 0) s[i] = 'updated'
    }
  }
  let inflight = s.filter((x) => x === 'updating').length
  while (inflight < maxSkew.value && nextIdx < matIdx) {
    // skip non-target ("unaffected") nodes without consuming a slot
    if (s[nextIdx] !== 'pending') { nextIdx++; continue }
    s[nextIdx] = 'updating'
    const slow = (nextIdx * 37 + 3) % 10 === 0
    const dur = slow ? 9000 + ((nextIdx * 29) % 40) * 100 : 620 + ((nextIdx * 53) % 11) * 90
    total[nextIdx] = dur
    remaining[nextIdx] = dur
    nextIdx++
    inflight++
  }
  if (nextIdx >= count && inflight === 0) {
    if (!doneAt) doneAt = Date.now()
    else if (Date.now() - doneAt > 2000) reset()
  }
}

// build the layout once (client-only; d3 kept out of SSR)
onMounted(async () => {
  reduced =
    typeof window !== 'undefined' &&
    window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches

  const d3h = await import('d3-hierarchy')
  const root = d3h.hierarchy(buildTree())
  d3h.cluster().size([2 * Math.PI, R])(root)
  root.each((n) => {
    n.rad = n.y
    n.ang = n.x
    n.px = +(n.y * Math.cos(n.x - Math.PI / 2)).toFixed(2)
    n.py = +(n.y * Math.sin(n.x - Math.PI / 2)).toFixed(2)
  })

  hub.value = { px: root.px, py: root.py }
  formNodes.value = root.children.map((f) => ({ id: f.data.id, px: f.px, py: f.py }))
  leafNodes.value = root.leaves().map((n) => ({ idx: n.data.idx, px: n.px, py: n.py }))
  edges.value = root.links().map((l) => ({
    key: `${l.source.data.id}--${l.target.data.id}`,
    d: radialCurve(l.source, l.target),
    idx: l.target.data.type === 'node' ? l.target.data.idx : null,
  }))
  ready.value = true

  reset()
  if (!reduced) timer = setInterval(tick, TICK)
})
watch(scenario, () => reset())
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.scale {
  width: 100%;
  padding: var(--lynq-section-y) 2rem;
  scroll-margin-top: 5rem;
}
.scale-inner { max-width: 1040px; }

.skew {
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius);
  background: rgba(255, 255, 255, 0.015);
  padding: 1.75rem;
}
.skew-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 1.2rem;
}

/* scenario toggle */
.seg {
  display: inline-flex;
  padding: 3px;
  gap: 2px;
  border: 1px solid var(--lynq-border);
  border-radius: 9px;
  background: rgba(255, 255, 255, 0.03);
}
.seg-btn {
  font-family: var(--lynq-mono);
  font-size: 0.66rem;
  color: var(--lynq-text-dim);
  background: transparent;
  border: 0;
  border-radius: 6px;
  padding: 0.32rem 0.7rem;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}
.seg-btn:hover { color: var(--lynq-text); }
.seg-btn.on { color: #33aca8; background: rgba(51, 172, 168, 0.12); }

/* maxSkew slider */
.ctrl { display: flex; align-items: center; gap: 0.7rem; }
.ctrl-lbl { font-family: var(--lynq-mono); font-size: 0.66rem; color: var(--lynq-text-dim); }
.ctrl-range {
  -webkit-appearance: none;
  appearance: none;
  width: 150px;
  height: 4px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.14);
  cursor: pointer;
}
.ctrl-range::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 15px;
  height: 15px;
  border-radius: 50%;
  background: #33aca8;
  border: 2px solid #0a0a0a;
  box-shadow: 0 0 0 1px rgba(51, 172, 168, 0.6);
  cursor: pointer;
}
.ctrl-range::-moz-range-thumb {
  width: 15px;
  height: 15px;
  border-radius: 50%;
  background: #33aca8;
  border: 2px solid #0a0a0a;
  cursor: pointer;
}
.ctrl-val {
  min-width: 1.4rem;
  text-align: center;
  font-family: var(--lynq-mono);
  font-size: 0.82rem;
  font-weight: 600;
  color: #33aca8;
  border: 1px solid rgba(51, 172, 168, 0.45);
  background: rgba(51, 172, 168, 0.1);
  border-radius: 6px;
  padding: 1px 6px;
}

/* trigger */
.trigger {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.55rem;
  margin-bottom: 0.4rem;
  padding: 0.6rem 0.8rem;
  border: 1px solid var(--lynq-border);
  border-radius: 10px;
  background: #0c0c10;
}
.trg-tag {
  font-family: var(--lynq-mono);
  font-size: 0.54rem;
  color: #4fd1cb;
  border: 1px solid rgba(79, 209, 203, 0.4);
  border-radius: 4px;
  padding: 1px 6px;
  flex: 0 0 auto;
}
.trg-cmd { font-family: var(--lynq-mono); font-size: 0.72rem; color: var(--lynq-text); }
.trg-sub { font-family: var(--lynq-mono); font-size: 0.62rem; color: var(--lynq-text-faint); }

/* topology */
.topo { width: 100%; min-height: 380px; }
.topo-svg { display: block; width: 100%; height: 380px; }

/* edges (bezier, thin) */
.edge {
  fill: none;
  stroke: rgba(255, 255, 255, 0.08);
  stroke-width: 1;
  transition: stroke 0.3s ease, opacity 0.3s ease;
}
.edge-hub { stroke: rgba(51, 172, 168, 0.3); stroke-width: 1.4; }
.edge.st-empty { opacity: 0; }
.edge.st-skip { stroke: rgba(255, 255, 255, 0.05); }
.edge.st-updating { stroke: rgba(245, 158, 11, 0.5); }
.edge.st-updated { stroke: rgba(51, 172, 168, 0.28); }

/* hub */
.hub-halo { fill: rgba(51, 172, 168, 0.08); }
.hub-track { fill: #0c0c10; stroke: rgba(255, 255, 255, 0.1); stroke-width: 3; }
.hub-prog {
  fill: none;
  stroke: #33aca8;
  stroke-width: 3;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.4s ease;
}
.hub-core { fill: rgba(51, 172, 168, 0.9); }
.hub-lbl {
  fill: #05201f;
  font-family: var(--lynq-mono);
  font-size: 8px;
  font-weight: 700;
  text-anchor: middle;
  dominant-baseline: middle;
}

/* forms */
.form-ring { fill: none; stroke: rgba(51, 172, 168, 0.5); stroke-width: 2; }
.form-core { fill: rgba(51, 172, 168, 0.85); }

/* nodes — simple status-coloured circles (dashboard leaf style) */
.node-inner {
  transform-box: fill-box;
  transform-origin: center;
  transition: transform 0.32s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.32s ease, fill 0.28s ease;
}
.node-core {
  fill: rgba(255, 255, 255, 0.24); /* pending / waiting */
  stroke: #0a0a0a;
  stroke-width: 1.4;
}
.st-skip .node-core { fill: #5b6472; } /* unaffected — stays on current version */
.st-updating .node-core { fill: var(--lynq-amber); }
.st-updated .node-core { fill: #33aca8; }
.st-empty .node-inner { transform: scale(0.1); opacity: 0; } /* insert: not created yet */

@media (prefers-reduced-motion: no-preference) {
  .st-updating .node-core { animation: st-pulse 1s ease-in-out infinite; }
  .is-insert .st-updated .node-inner { animation: st-born 0.42s ease-in-out 4; }
}
@keyframes st-pulse {
  0%, 100% { filter: drop-shadow(0 0 1px rgba(245, 158, 11, 0.4)); }
  50%      { filter: drop-shadow(0 0 5px rgba(245, 158, 11, 0.9)); }
}
@keyframes st-born {
  0%, 100% { filter: none; }
  50%      { filter: drop-shadow(0 0 6px rgba(79, 209, 203, 0.95)); }
}

/* live stat */
.stat {
  display: flex;
  flex-wrap: wrap;
  gap: 1.25rem;
  margin-top: 0.4rem;
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  color: var(--lynq-text-faint);
}
.stat-item { display: inline-flex; align-items: center; gap: 0.4rem; }
.stat-item b { color: var(--lynq-text); font-weight: 600; }
.lg { width: 10px; height: 10px; border-radius: 3px; }
.lg-old { background: rgba(255, 255, 255, 0.16); }
.lg-upd { background: var(--lynq-amber); }
.lg-new { background: rgba(51, 172, 168, 0.7); }
.lg-skip { background: #5b6472; }

.skew-note {
  margin: 1.1rem 0 0;
  font-size: 0.9rem;
  line-height: 1.55;
  color: var(--lynq-text-dim);
}
.skew-drag { color: var(--lynq-accent); }

@media (max-width: 760px) {
  .scale { padding: 4rem 1.25rem; }
  .skew-head { flex-direction: column; align-items: flex-start; }
}
</style>
