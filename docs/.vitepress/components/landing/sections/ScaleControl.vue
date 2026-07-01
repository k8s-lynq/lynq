<template>
  <section class="scale bg-lynq-bg" id="scale">
    <div class="scale-inner mx-auto">
      <SectionHeader
        label="Rollout safety"
        title="A big update, without the thundering herd"
        subtitle="A template edit, a bulk update, or a large insert can touch — or create — every node at once, and doing them all at the same time would stampede your API server. maxSkew caps how many change at a time. Pick a trigger, drag maxSkew, and watch the fleet roll over safely."
        accent="purple"
      />

      <!-- Interactive maxSkew. A client-side scheduler keeps exactly ≤ maxSkew
           nodes "updating" at once across the whole fleet; each finishes at its
           own pace and the next pending node starts the moment a slot frees.
           Drag the slider (1–10) to change the cap live. SSR-safe: renders all
           pending on the server, the sim starts on mount; reduced-motion shows
           the finished fleet. -->
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

        <!-- what triggered the rollout (changes with the scenario) -->
        <div class="trigger">
          <span class="trg-tag">{{ active.tag }}</span>
          <code class="trg-cmd">{{ active.trigger }}</code>
          <span class="trg-sub">{{ active.triggerSub }}</span>
        </div>

        <div class="grid" :class="{ 'is-insert': scenario === 'insert' }" aria-hidden="true">
          <span
            v-for="(s, i) in states"
            :key="i"
            class="cell"
            :class="'s-' + s"
          ><span class="cell-fill" :style="{ width: fill[i] + '%' }"></span></span>
        </div>

        <div class="stat">
          <span class="stat-item"><i class="lg lg-upd"></i> {{ active.verb }} <b>{{ updatingCount }}</b> / {{ maxSkew }}</span>
          <span class="stat-item"><i class="lg lg-new"></i> {{ active.verbDone }} <b>{{ updatedCount }}</b> / {{ count }}</span>
          <span class="stat-item"><i class="lg lg-old"></i> pending {{ pendingCount }}</span>
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

const count = 96
const maxSkew = ref(4)

// Two triggers that both fan out to the fleet — the bounded rollout below is the
// same for either, which is the point: whatever changes, maxSkew paces it.
const scenario = ref('template')
const SCENARIOS = {
  template: {
    tab: 'Template edit',
    tag: 'YAML',
    verb: 'updating',
    verbDone: 'updated',
    trigger: 'web-stack.yaml edited',
    triggerSub: 'every LynqForm re-renders → all nodes reconcile',
    note: 'A single template edit would otherwise reconcile every node at once — a thundering herd that can melt the control plane. maxSkew rolls it through a few at a time while the rest keep serving the current version.',
  },
  rows: {
    tab: 'Bulk row update',
    tag: 'SQL',
    verb: 'updating',
    verbDone: 'updated',
    trigger: "UPDATE node_configs SET plan='pro' WHERE region='us'",
    triggerSub: `${count} rows changed → each affected node re-reconciles`,
    note: 'A bulk row update touches a whole batch of existing resources at once. Instead of rewriting them all together, maxSkew applies the changes gradually — live workloads stay stable while the fleet catches up a few nodes at a time.',
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

// reactive per-node state: 'empty' | 'pending' | 'updating' | 'updated'
//  - empty:   the row doesn't exist yet (only used by the insert scenario)
//  - pending: the node exists and is queued to reconcile
//  - updating: reconciling now (≤ maxSkew of these)
//  - updated:  done
const states = ref(new Array(count).fill('pending'))
// reactive per-node progress bar (0–100) — each app fills as it readies
const fill = ref(new Array(count).fill(0))
// non-reactive per-node timing
let remaining = new Array(count).fill(0)
let total = new Array(count).fill(1)
let nextIdx = 0 // next node to start reconciling
let matIdx = count // rows that exist yet (insert lands them over time; else all exist)
let doneAt = 0
let timer = null
let reduced = false
const TICK = 110
const MAT_PER_TICK = 3 // INSERT lands rows fast — creation is the throttled part

const updatingCount = computed(() => states.value.filter((s) => s === 'updating').length)
const updatedCount = computed(() => states.value.filter((s) => s === 'updated').length)
const pendingCount = computed(() => count - updatingCount.value - updatedCount.value)

// (re)initialise for the current scenario. Insert starts from EMPTY slots (rows
// don't exist yet); the others start with every node already present & pending.
function reset() {
  if (reduced) {
    states.value = new Array(count).fill('updated')
    fill.value = new Array(count).fill(100)
    return
  }
  const insert = scenario.value === 'insert'
  states.value = new Array(count).fill(insert ? 'empty' : 'pending')
  fill.value = new Array(count).fill(0)
  remaining = new Array(count).fill(0)
  total = new Array(count).fill(1)
  nextIdx = 0
  matIdx = insert ? 0 : count
  doneAt = 0
}

function tick() {
  const s = states.value
  const f = fill.value
  // Large insert: new rows land fast (empty → pending) — the DB write is cheap;
  // it's the resource creation below that maxSkew paces. This makes the grid
  // populate with pending nodes quickly while the amber creation wave lags.
  if (matIdx < count) {
    const to = Math.min(count, matIdx + MAT_PER_TICK)
    for (let i = matIdx; i < to; i++) s[i] = 'pending'
    matIdx = to
  }
  // advance in-flight reconciles; grow each progress bar; complete when done
  for (let i = 0; i < count; i++) {
    if (s[i] === 'updating') {
      remaining[i] -= TICK
      if (remaining[i] <= 0) {
        s[i] = 'updated'
        f[i] = 100
      } else {
        f[i] = Math.round((1 - remaining[i] / total[i]) * 100)
      }
    }
  }
  // start pending nodes up to the cap — but only ones that already exist (< matIdx)
  let inflight = s.filter((x) => x === 'updating').length
  while (inflight < maxSkew.value && nextIdx < matIdx) {
    s[nextIdx] = 'updating'
    // varied ready time so nodes don't finish in lock-step; ~1 in 10 is a slow
    // straggler that takes 10s+ (a stuck pull, a laggy readiness probe…) — it
    // ties up one maxSkew slot while the others keep rolling. Deterministic.
    const slow = (nextIdx * 37 + 3) % 10 === 0
    const dur = slow
      ? 10000 + ((nextIdx * 29) % 45) * 100 // 10.0–14.5s
      : 420 + ((nextIdx * 53) % 11) * 70 // 0.42–1.12s
    total[nextIdx] = dur
    remaining[nextIdx] = dur
    f[nextIdx] = 0
    nextIdx++
    inflight++
  }
  // whole fleet done → hold briefly, then replay the scenario from scratch
  if (nextIdx >= count && inflight === 0) {
    if (!doneAt) doneAt = Date.now()
    else if (Date.now() - doneAt > 1800) reset()
  }
}

onMounted(() => {
  reduced =
    typeof window !== 'undefined' &&
    window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  reset()
  if (!reduced) timer = setInterval(tick, TICK)
})
// switching scenario replays it from the start (so insert shows rows arriving)
watch(scenario, () => reset())
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.scale {
  width: 100%;
  padding: 6rem 2rem;
  scroll-margin-top: 5rem;
}
.scale-inner { max-width: 1040px; }

/* ---- stage ---- */
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
  margin-bottom: 1.4rem;
}
.skew-h { font-family: var(--lynq-mono); font-size: 0.72rem; color: var(--lynq-text-dim); }
.skew-h b { color: var(--lynq-text); font-weight: 500; }

/* interactive control */
.ctrl {
  display: flex;
  align-items: center;
  gap: 0.7rem;
}
.ctrl-lbl {
  font-family: var(--lynq-mono);
  font-size: 0.66rem;
  color: var(--lynq-text-dim);
}
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
.seg-btn.on {
  color: #33aca8;
  background: rgba(51, 172, 168, 0.12);
}

/* trigger row — the change that kicked off the rollout */
.trigger {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.55rem;
  margin-bottom: 1.3rem;
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

/* dense fleet grid */
.grid {
  display: grid;
  grid-template-columns: repeat(24, 1fr);
  gap: 5px;
}
/* each cell is a mini progress track; the inner fill grows as the node readies */
.cell {
  position: relative;
  height: 16px;
  border-radius: 3px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.07);
  transition: transform 0.3s ease, opacity 0.3s ease, background 0.3s ease;
}
/* insert scenario: the row doesn't exist yet — it pops in when inserted */
.s-empty {
  background: transparent;
  transform: scale(0.3);
  opacity: 0;
}
.cell-fill {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 0;
  border-radius: 3px;
  transition: width 0.12s linear;
}
/* updating → amber fill grows (this node's progress); done → full teal */
.s-updating .cell-fill {
  background: var(--lynq-amber);
  box-shadow: 0 0 8px 1px rgba(245, 158, 11, 0.55);
}
.s-updated .cell-fill { background: rgba(51, 172, 168, 0.65); }

@media (prefers-reduced-motion: no-preference) {
  .s-updating .cell-fill { animation: sc-pulse 1s ease-in-out infinite; }
  /* insert only: a freshly-created node blinks a few times to say "brand new" */
  .grid.is-insert .s-updated .cell-fill { animation: sc-born 0.42s ease-in-out 4; }
}
@keyframes sc-pulse {
  0%, 100% { box-shadow: 0 0 6px 1px rgba(245, 158, 11, 0.45); }
  50%      { box-shadow: 0 0 12px 2px rgba(245, 158, 11, 0.8); }
}
/* newborn blink: brighten + glow, settling back to the steady "updated" teal */
@keyframes sc-born {
  0%, 100% { background: rgba(51, 172, 168, 0.65); box-shadow: none; }
  50%      { background: #6ff0ea; box-shadow: 0 0 9px 2px rgba(79, 209, 203, 0.85); }
}

/* live stat readout */
.stat {
  display: flex;
  flex-wrap: wrap;
  gap: 1.25rem;
  margin-top: 1.4rem;
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  color: var(--lynq-text-faint);
}
.stat-item { display: inline-flex; align-items: center; gap: 0.4rem; }
.stat-item b { color: var(--lynq-text); font-weight: 600; }
.lg { width: 10px; height: 10px; border-radius: 3px; }
.lg-old { background: rgba(255, 255, 255, 0.14); }
.lg-upd { background: var(--lynq-amber); }
.lg-new { background: rgba(51, 172, 168, 0.65); }

.skew-note {
  margin: 1.1rem 0 0;
  font-size: 0.9rem;
  line-height: 1.55;
  color: var(--lynq-text-dim);
}
.skew-note b { color: var(--lynq-text); font-weight: 600; }
.skew-drag { color: var(--lynq-accent); }

@media (max-width: 760px) {
  .scale { padding: 4rem 1.25rem; }
  .grid { grid-template-columns: repeat(16, 1fr); }
  .skew-head { flex-direction: column; align-items: flex-start; }
}
</style>
