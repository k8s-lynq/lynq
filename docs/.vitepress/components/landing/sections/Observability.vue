<template>
  <section ref="rootRef" class="observability scroll-mt-20 px-8" :class="{ in: inView, breach }">
    <div class="mx-auto" style="max-width: 1040px">
      <SectionHeader
        label="Observability"
        title="Catch Problems Before They Spread"
        subtitle="26 Prometheus metrics, a pre-built Grafana dashboard, and 18 alert rules with runbooks. Whatever goes wrong — conflicts, failures, latency, workload degradation, a whole hub — the metric graphs it and the right alert pages you."
        accent="blue"
      />

      <div class="incident-wrap" @mouseenter="onEnter" @mouseleave="onLeave">
        <div class="incident grid gap-4" :key="active">
          <!-- Grafana-style metric panel with a developing breach -->
          <div class="panel graph-panel">
            <div class="panel-head">
              <span class="p-title font-mono">{{ scen.promql }}</span>
              <span class="p-tag font-mono">Grafana · {{ scen.tag }}</span>
            </div>
            <svg class="chart" viewBox="0 0 580 175" preserveAspectRatio="none" aria-hidden="true" :style="{ '--mcolor': scen.color }">
              <defs>
                <clipPath :id="clipId">
                  <rect x="0" y="0" width="580" :height="scen.thresholdY" />
                </clipPath>
              </defs>
              <line class="grid" x1="0" y1="42" x2="580" y2="42" />
              <line class="grid" x1="0" y1="84" x2="580" y2="84" />
              <line class="threshold" x1="0" :y1="scen.thresholdY" x2="580" :y2="scen.thresholdY" />
              <path class="area" :d="areaPath" />
              <path class="breach-area" :d="areaPath" :clip-path="`url(#${clipId})`" />
              <path class="line" :d="linePath" />
              <circle class="peak" :cx="peak.x" :cy="peak.y" r="4" />
            </svg>
            <div class="thr-label font-mono">{{ scen.thrLabel }}</div>
          </div>

          <!-- Alertmanager-style feed for this scenario -->
          <div class="feed">
            <div class="feed-title font-mono">Alertmanager</div>

            <div
              v-for="(a, i) in scen.alerts"
              :key="a.name"
              class="alert-card"
              :class="[a.severity, { fired: i < firedCount }]"
            >
              <div class="ac-head">
                <span class="bell" aria-hidden="true">&#128276;</span>
                <span class="ac-name font-mono">{{ a.name }}</span>
                <span class="sev font-mono" :class="'sev-' + a.severity">{{ a.severity }}</span>
              </div>
              <p class="ac-desc">
                <template v-for="(seg, si) in a.desc" :key="si"><code v-if="seg.c">{{ seg.c }}</code><span v-else>{{ seg.t }}</span></template>
              </p>
              <div v-if="a.runbook" class="ac-links">
                <a class="ac-link" :href="a.runbook">runbook &#8594;</a>
                <a class="ac-link" href="/monitoring">Grafana &#8599;</a>
              </div>
            </div>

            <p class="feed-foot" :class="{ show: firedCount >= scen.alerts.length }">{{ scen.resolution }}</p>
          </div>
        </div>

        <!-- Carousel controls -->
        <div class="carousel">
          <button class="c-arrow" type="button" @click.stop="go(active - 1)" aria-label="Previous scenario">&#8249;</button>
          <div class="dots" role="tablist">
            <button
              v-for="(s, i) in scenarios"
              :key="s.tag"
              class="dot"
              :class="{ on: i === active }"
              type="button"
              role="tab"
              :aria-selected="i === active"
              :aria-label="s.tag"
              @click.stop="go(i)"
            ></button>
          </div>
          <button class="c-arrow" type="button" @click.stop="go(active + 1)" aria-label="Next scenario">&#8250;</button>
        </div>
      </div>

      <div class="learn-more mt-10 text-center">
        <a
          class="inline-flex items-center gap-2 rounded-full border border-lynq-border bg-transparent px-6 py-[0.72rem] text-[0.95rem] font-medium! text-lynq-text! no-underline transition-colors duration-200 hover:border-white/20 hover:bg-white/[0.06]"
          href="/monitoring"
        >
          Set up monitoring
          <span aria-hidden="true">&#8594;</span>
        </a>
      </div>
    </div>
  </section>
</template>

<script setup>
/**
 * Observability — a carousel of alert-driven incidents. Each slide is a
 * Grafana-style metric graph that develops a breach; as it crosses the alert
 * threshold, the Alertmanager feed fires in the real escalation order shipped
 * in config/prometheus/alerts.yaml. The carousel cycles through distinct
 * failure modes (conflicts, resource failures, reconcile latency, hub-wide
 * failure) to show the breadth of what the 26 metrics + 18 alert rules catch.
 *
 * All names/exprs/severities/thresholds are verified against the repo. Motion:
 * the active slide draws in and its alerts fire on a timer; the carousel
 * auto-advances (pauses on hover, arrows/dots to navigate). Reduced motion
 * shows each slide already fired, no auto-advance, manual nav only.
 */
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import SectionHeader from '../primitives/SectionHeader.vue'
import { useInView } from '../composables/useInView.js'
import { useReducedMotion } from '../composables/useReducedMotion.js'

const rootRef = ref(null)
const { inView } = useInView(rootRef, { threshold: 0.25, once: true })
const reduced = useReducedMotion()
const clipId = 'obs-breach-clip'

// t: plain text segment, c: inline-code segment.
const scenarios = [
  {
    tag: 'conflicts',
    promql: 'rate(lynqnode_conflicts_total[5m])',
    color: 'var(--lynq-amber)',
    thresholdY: 125,
    thrLabel: 'alert if > 0.1/s for 10m',
    points: [[0, 156], [80, 155], [160, 157], [240, 153], [300, 148], [350, 138], [400, 120], [450, 96], [500, 70], [550, 50], [580, 42]],
    alerts: [
      { name: 'LynqNodeNewConflictsDetected', severity: 'info', desc: [{ t: 'New conflicts on ' }, { c: 'acme-corp-web-stack' }, { t: ' — ' }, { c: 'resource_kind=Service' }, { t: '.' }] },
      { name: 'LynqNodeHighConflictRate', severity: 'warning', desc: [{ t: '0.23 conflicts/sec on ' }, { c: 'Service' }, { t: ' · ' }, { c: 'policy=Stuck' }, { t: '. Review naming templates.' }], runbook: '/alert-runbooks#lynqnodehighconflictrate' },
    ],
    resolution: 'The metric caught the drift and paged before it spread — no resource was overwritten.',
  },
  {
    tag: 'failures',
    promql: 'lynqnode_resources_failed',
    color: 'var(--lynq-red)',
    thresholdY: 150,
    thrLabel: 'alert if > 0 for 5m',
    points: [[0, 155], [90, 155], [170, 155], [235, 155], [255, 112], [330, 112], [375, 112], [395, 70], [470, 70], [540, 70], [580, 70]],
    alerts: [
      { name: 'LynqNodeResourcesMismatch', severity: 'warning', desc: [{ t: 'ready ' }, { c: '1' }, { t: ' ≠ desired ' }, { c: '3' }, { t: ' on ' }, { c: 'beta-inc-web-stack' }, { t: ' — still reconciling.' }] },
      { name: 'LynqNodeResourcesFailed', severity: 'critical', desc: [{ c: 'beta-inc-web-stack' }, { t: ' has ' }, { c: '2' }, { t: ' failed resources — readiness timed out.' }], runbook: '/alert-runbooks#lynqnoderesourcesfailed' },
    ],
    resolution: 'Named the exact failing resource in seconds — long before a user noticed.',
  },
  {
    tag: 'latency',
    promql: 'histogram_quantile(0.95, …reconcile_duration_seconds…)',
    color: 'var(--lynq-blue)',
    thresholdY: 100,
    thrLabel: 'alert if p95 > 30s for 15m',
    points: [[0, 132], [80, 129], [160, 125], [240, 120], [300, 112], [350, 103], [400, 92], [450, 78], [500, 64], [550, 52], [580, 46]],
    alerts: [
      { name: 'LynqNodeReconciliationSlow', severity: 'warning', desc: [{ t: 'p95 reconcile ' }, { c: '41s' }, { t: ' on the success path — the apply pipeline is backing up.' }], runbook: '/alert-runbooks#lynqnodereconciliationslow' },
    ],
    resolution: 'Spotted the slowdown early — before the workqueue backed up cluster-wide.',
  },
  {
    tag: 'hub',
    promql: 'hub_failed / hub_desired',
    color: 'var(--lynq-amber)',
    thresholdY: 92,
    thrLabel: 'alert if > 50% of nodes failed',
    points: [[0, 150], [80, 148], [160, 145], [240, 139], [300, 130], [350, 116], [400, 100], [450, 84], [500, 66], [550, 52], [580, 46]],
    alerts: [
      { name: 'HubNodesFailure', severity: 'warning', desc: [{ t: 'Hub ' }, { c: 'mysql-prod' }, { t: ' — ' }, { c: '3' }, { t: ' nodes failing.' }] },
      { name: 'HubManyNodesFailure', severity: 'critical', desc: [{ t: 'Hub ' }, { c: 'mysql-prod' }, { t: ' — ' }, { c: '7' }, { t: ' failed nodes (>50%). Systemic issue.' }], runbook: '/alert-runbooks#hubmanynodesfailure' },
    ],
    resolution: 'One graph revealed a fleet-wide problem — not 50 scattered, unlinked errors.',
  },
]
const n = scenarios.length

const active = ref(0)
const scen = computed(() => scenarios[active.value])
const linePath = computed(() => 'M' + scen.value.points.map((p) => p.join(',')).join(' L'))
const areaPath = computed(() => linePath.value + ' L580,160 L0,160 Z')
const peak = computed(() => {
  const p = scen.value.points[scen.value.points.length - 1]
  return { x: p[0], y: p[1] }
})

const firedCount = ref(0)
const breach = ref(false)

const timers = new Set()
function clearAlertTimers() {
  timers.forEach((t) => clearTimeout(t))
  timers.clear()
}
function at(ms, fn) {
  const t = setTimeout(() => {
    timers.delete(t)
    fn()
  }, ms)
  timers.add(t)
}

function play() {
  clearAlertTimers()
  firedCount.value = 0
  breach.value = false
  const total = scen.value.alerts.length
  if (reduced.value) {
    firedCount.value = total
    breach.value = true
    return
  }
  at(1500, () => (firedCount.value = 1))
  at(3000, () => {
    breach.value = true
    firedCount.value = total
  })
}

let started = false
let paused = false
let autoTimer = null
function clearAuto() {
  if (autoTimer) {
    clearTimeout(autoTimer)
    autoTimer = null
  }
}
function scheduleAdvance() {
  clearAuto()
  if (reduced.value || paused || !started) return
  autoTimer = setTimeout(() => {
    active.value = (active.value + 1) % n
  }, 7000)
}
function go(i) {
  active.value = ((i % n) + n) % n
}

watch(inView, (v) => {
  if (v && !started) {
    started = true
    play()
    scheduleAdvance()
  }
})
watch(active, () => {
  if (!started) return
  play()
  scheduleAdvance()
})

function onEnter() {
  paused = true
  clearAuto()
}
function onLeave() {
  paused = false
  scheduleAdvance()
}

onBeforeUnmount(() => {
  clearAlertTimers()
  clearAuto()
})
</script>

<style scoped>
.observability {
  padding-block: var(--lynq-section-y);
  position: relative;
}
/* Cool blue "ops" signature — contrasts the red breach graphs. On ::before so
   the global section-transparency rule leaves it intact. */
.observability::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background:
    radial-gradient(54rem 34rem at 50% -4%, rgba(59, 130, 246, 0.12), transparent 62%),
    radial-gradient(40rem 30rem at 10% 100%, rgba(51, 172, 168, 0.06), transparent 60%);
}
.observability > * {
  position: relative;
  z-index: 1;
}
.observability :deep(.lynq-section-header h2) {
  font-size: var(--lynq-h2);
  font-weight: var(--lynq-heading-weight);
}

.incident {
  grid-template-columns: 1.5fr 1fr;
  align-items: stretch;
}

/* ── Panels ── */
.panel {
  background: #0b0b0e;
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius);
  padding: 0.85rem 1rem 1rem;
}
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}
.p-title {
  font-size: 0.76rem;
  color: var(--lynq-text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.p-tag {
  font-size: 0.64rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--lynq-text-faint);
  border: 1px solid var(--lynq-border);
  border-radius: 6px;
  padding: 0.12rem 0.45rem;
  flex: none;
}

.chart {
  width: 100%;
  height: 175px;
  display: block;
}
.grid {
  stroke: rgba(255, 255, 255, 0.05);
  stroke-width: 1;
}
.threshold {
  stroke: var(--lynq-text-faint);
  stroke-width: 1.3;
  stroke-dasharray: 4 4;
  opacity: 0.6;
  transition: stroke 0.4s ease, opacity 0.4s ease;
}
.observability.breach .threshold {
  stroke: var(--lynq-red);
  opacity: 0.9;
}
.area {
  fill: color-mix(in srgb, var(--mcolor) 12%, transparent);
  stroke: none;
}
.line {
  fill: none;
  stroke: var(--mcolor);
  stroke-width: 2;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.breach-area {
  fill: rgba(239, 68, 68, 0.24);
  stroke: none;
  opacity: 0;
  transition: opacity 0.5s ease;
}
.observability.breach .breach-area {
  opacity: 1;
}
.peak {
  fill: var(--mcolor);
  opacity: 0;
}
.observability.breach .peak {
  fill: var(--lynq-red);
}
.thr-label {
  margin-top: 0.4rem;
  font-size: 0.66rem;
  color: var(--lynq-text-faint);
  text-align: right;
}

/* ── Alert feed ── */
.feed {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.feed-title {
  font-size: 0.64rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--lynq-text-faint);
  margin-bottom: 0.1rem;
}

.alert-card {
  border: 1px solid var(--lynq-border);
  border-left-width: 3px;
  border-radius: var(--lynq-radius-sm);
  background: #0f0f13;
  padding: 0.6rem 0.8rem;
}
.alert-card.info { border-left-color: var(--lynq-blue); }
.alert-card.warning { border-left-color: var(--lynq-amber); }
.alert-card.critical { border-left-color: var(--lynq-red); }

.ac-head {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-bottom: 0.3rem;
}
.bell {
  font-size: 0.82rem;
}
.ac-name {
  font-size: 0.74rem;
  font-weight: 700;
  color: var(--lynq-text);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sev {
  font-size: 0.6rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  flex: none;
}
.sev-info { color: #7db5ff; background: rgba(59, 130, 246, 0.16); }
.sev-warning { color: var(--lynq-amber); background: rgba(245, 158, 11, 0.16); }
.sev-critical { color: #ff6b6b; background: rgba(239, 68, 68, 0.18); }

.ac-desc {
  margin: 0;
  font-size: 0.74rem;
  line-height: 1.45;
  color: var(--lynq-text-dim);
}
.ac-desc code {
  font-family: var(--lynq-mono);
  font-size: 0.7rem;
  color: var(--lynq-text);
  background: rgba(255, 255, 255, 0.06);
  padding: 0.05rem 0.28rem;
  border-radius: 4px;
}
.ac-links {
  display: flex;
  gap: 0.9rem;
  margin-top: 0.5rem;
}
.ac-link {
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  color: var(--lynq-accent) !important;
  text-decoration: none;
}
.ac-link:hover { color: var(--lynq-text) !important; }

.feed-foot {
  margin: 0.2rem 0 0;
  font-size: 0.74rem;
  line-height: 1.45;
  color: var(--lynq-green);
}

/* ── Carousel controls ── */
.carousel {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.9rem;
  margin-top: 1rem;
}
.c-arrow {
  appearance: none;
  border: 1px solid var(--lynq-border);
  background: transparent;
  color: var(--lynq-text-dim);
  width: 30px;
  height: 30px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 1rem;
  line-height: 1;
  transition: color 0.2s ease, border-color 0.2s ease, background 0.2s ease;
}
.c-arrow:hover {
  color: var(--lynq-text);
  border-color: rgba(255, 255, 255, 0.25);
  background: rgba(255, 255, 255, 0.05);
}
.dots {
  display: flex;
  gap: 0.5rem;
}
.dot {
  appearance: none;
  border: none;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.2);
  cursor: pointer;
  padding: 0;
  transition: background 0.2s ease, width 0.2s ease;
}
.dot.on {
  background: var(--lynq-accent);
  width: 22px;
}

/* ── Motion ── */
@media (prefers-reduced-motion: no-preference) {
  .incident {
    animation: obs-mount 0.4s ease both;
  }
  .line {
    stroke-dasharray: 900;
    stroke-dashoffset: 900;
  }
  .observability.in .line {
    animation: obs-draw 3s var(--lynq-ease) both;
  }
  .area {
    opacity: 0;
  }
  .observability.in .area {
    animation: obs-fade 2.6s ease both;
  }
  .observability.in.breach .peak {
    animation: obs-fade 0.3s ease both, obs-pulse 1.8s ease-in-out 0.3s infinite;
  }
  .alert-card {
    opacity: 0;
    transform: translateY(10px);
    transition: opacity 0.45s var(--lynq-ease), transform 0.45s var(--lynq-ease);
  }
  .alert-card.fired {
    opacity: 1;
    transform: none;
  }
  .alert-card.fired .bell {
    animation: obs-ring 0.6s ease;
  }
  .feed-foot {
    opacity: 0;
    transition: opacity 0.5s ease;
  }
  .feed-foot.show {
    opacity: 1;
  }
}

@keyframes obs-mount {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: none; }
}
@keyframes obs-draw { to { stroke-dashoffset: 0; } }
@keyframes obs-fade { to { opacity: 1; } }
@keyframes obs-pulse {
  0%, 100% { r: 4; opacity: 1; }
  50% { r: 6.5; opacity: 0.6; }
}
@keyframes obs-ring {
  0%, 100% { transform: rotate(0); }
  20% { transform: rotate(-16deg); }
  40% { transform: rotate(12deg); }
  60% { transform: rotate(-8deg); }
  80% { transform: rotate(4deg); }
}

@media (max-width: 820px) {
  .incident {
    grid-template-columns: 1fr;
  }
}
</style>
