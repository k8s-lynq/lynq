<template>
  <section
    class="live-transform scroll-mt-20 px-8"
    ref="sectionRef"
    @mouseenter="onPanelEnter"
    @mouseleave="onPanelLeave"
  >
    <div class="mx-auto" style="max-width: 1040px" ref="containerRef">
      <SectionHeader
        label="How It Works"
        title="From Data to Resources in Seconds"
        subtitle="Follow the full lifecycle — from a database row to running Kubernetes resources. Click any stage to jump."
        accent="green"
      />

      <!-- Stage rail: Database → LynqHub → LynqForm → Kubernetes -->
      <div class="rail flex items-stretch justify-center mb-10" role="tablist" aria-label="Pipeline stages">
        <template v-for="(stage, i) in stages" :key="stage.key">
          <button
            class="stage rounded-lynq border border-lynq-border flex flex-col items-center gap-1.5 px-4 py-5 text-lynq-text cursor-pointer"
            :class="[stage.key, { active: step === i, done: step > i }]"
            role="tab"
            :aria-selected="step === i"
            @click="onStageClick(i)"
          >
            <span class="stage-icon flex items-center justify-center text-lynq-purple" aria-hidden="true">
              <LineIcon :name="stage.icon" />
            </span>
            <span class="text-[0.95rem] font-semibold text-lynq-text">{{ stage.label }}</span>
            <span class="text-[0.72rem] text-lynq-faint">{{ stage.sublabel }}</span>
          </button>

          <div v-if="i < stages.length - 1" class="rail-arrow">
            <ArrowFlow
              direction="right"
              :color="i === stages.length - 2 ? 'green' : 'purple'"
              :active="!reduced && (step === i || step === i + 1)"
              :label="stage.edge"
            />
          </div>
        </template>
      </div>

      <!-- Detail panel: prose (left) + YAML / terminal (right).
           The panel reserves a FIXED height so switching steps never reflows
           the page (no CLS); shorter steps simply have extra padding room. -->
      <div class="panel rounded-lynq border border-lynq-border flex p-8">
        <transition name="fade-slide" mode="out-in">
          <div class="panel-inner w-full" :key="step">
            <div class="prose">
              <span class="step-index block font-mono text-[0.72rem] font-semibold uppercase tracking-[0.1em] text-lynq-accent mb-[0.6rem]">Step {{ step + 1 }} / {{ stages.length }}</span>
              <h3 class="text-[1.3rem] text-lynq-text m-0 mb-[0.9rem]">{{ pipelineSteps[step].title }}</h3>
              <p class="text-[0.98rem] leading-[1.65] text-lynq-dim m-0">{{ pipelineSteps[step].description }}</p>
            </div>

            <div class="detail min-w-0">
              <TerminalWindow v-if="step === 3" title="kubectl output" class="demo-object">
                <TerminalLine
                  v-for="(ln, li) in kubectlLines"
                  :key="li"
                  :prompt="ln.prompt"
                  :text="ln.text || ' '"
                  :revealed="reduced || li <= revealedLines"
                />
              </TerminalWindow>
              <YamlBlock
                v-else
                class="demo-object"
                :filename="pipelineSteps[step].filename"
                :code="pipelineSteps[step].code"
                :highlight-tokens="['{{ .uid }}']"
              />
            </div>
          </div>
        </transition>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import SectionHeader from '../primitives/SectionHeader.vue'
import ArrowFlow from '../primitives/ArrowFlow.vue'
import YamlBlock from '../primitives/YamlBlock.vue'
import TerminalWindow from '../primitives/TerminalWindow.vue'
import TerminalLine from '../primitives/TerminalLine.vue'
import LineIcon from '../primitives/LineIcon.vue'
import { useInView } from '../composables/useInView.js'
import { useReducedMotion } from '../composables/useReducedMotion.js'
import { useStepTimeline } from '../composables/useStepTimeline.js'
import { pipelineSteps, kubectlLines } from '../data/demoData.js'

// Static stage metadata. Icons are picked from the shared LineIcon set so the
// pipeline reads in the same restrained line-art style as the rest of the page.
const stages = [
  { key: 'db', label: 'Database', sublabel: 'MySQL', edge: 'sync', icon: 'database' },
  { key: 'hub', label: 'LynqHub', sublabel: 'Data Source', edge: 'data', icon: 'sync' },
  { key: 'form', label: 'LynqForm', sublabel: 'Template', edge: 'apply', icon: 'template' },
  { key: 'k8s', label: 'Kubernetes', sublabel: 'Resources', edge: '', icon: 'cube' },
]

const reduced = useReducedMotion()
const sectionRef = ref(null)
const containerRef = ref(null)
const { inView } = useInView(containerRef, { threshold: 0.25, once: true })

// Auto-advancing walkthrough. Holds on the final stage (no loop) for calm.
const { step, playing, play, pause, seek } = useStepTimeline({
  steps: stages.length,
  durations: 3800,
  loop: false,
})

// Start once the section scrolls into view (skip auto-motion when reduced).
watch(inView, (v) => {
  if (v && !reduced.value) play()
})

// Sub-reveal for the step-4 terminal: reveal kubectl lines one at a time once
// stage 3 is reached. Under reduced motion every line shows instantly.
const revealedLines = ref(0)
let lineTimer = null

function clearLineTimer() {
  if (lineTimer !== null) {
    clearInterval(lineTimer)
    lineTimer = null
  }
}

function startLineReveal() {
  clearLineTimer()
  revealedLines.value = 0
  if (typeof window === 'undefined') return
  lineTimer = setInterval(() => {
    if (revealedLines.value >= kubectlLines.length - 1) {
      clearLineTimer()
      return
    }
    revealedLines.value += 1
  }, 180)
}

watch(step, (s) => {
  if (s === 3 && !reduced.value) startLineReveal()
  else clearLineTimer()
})

// Clicking a stage scrubs to it (seek pauses auto-advance) and, if it's the
// terminal stage, kicks off the line reveal.
function onStageClick(i) {
  seek(i)
  if (i === 3 && !reduced.value) startLineReveal()
}

// Hovering the section pauses auto-advance so readers can dwell; leaving
// resumes only if the walkthrough hadn't yet reached the final held stage.
let wasPlayingBeforeHover = false
function onPanelEnter() {
  wasPlayingBeforeHover = playing.value
  if (playing.value) pause()
}
function onPanelLeave() {
  if (wasPlayingBeforeHover && step.value < stages.length - 1) play()
}

onMounted(() => {
  // Under reduced motion settle straight on the terminal stage; the template
  // shows all lines via the `reduced` guard, so no timer is needed.
  if (reduced.value) seek(stages.length - 1)
})

onBeforeUnmount(clearLineTimer)
</script>

<style scoped>
.live-transform {
  padding-block: var(--lynq-section-y);
  background: linear-gradient(180deg, var(--lynq-bg) 0%, var(--lynq-bg-2) 100%);
}

/* Lighter, smaller heading (v2 tone). */
.live-transform :deep(.lynq-section-header h2) {
  font-size: var(--lynq-h2);
  font-weight: var(--lynq-heading-weight);
}

/* ── Stage rail ── */
/* Static layout is on the template; the animated hover/active state transitions
   and the subtle base tint stay here (JS toggles .active / .done). */
.stage {
  min-width: 118px;
  background: rgba(255, 255, 255, 0.02);
  transition: transform var(--lynq-beat) var(--lynq-ease),
    background 0.3s ease, border-color 0.3s ease;
  font-family: inherit;
}

.stage:hover {
  background: rgba(255, 255, 255, 0.05);
  transform: translateY(-2px);
}

.stage.active {
  background: rgba(51, 172, 168, 0.1);
  border-color: rgba(51, 172, 168, 0.5);
  transform: translateY(-2px);
}

.stage.k8s.active {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.5);
}

.stage-icon {
  width: 44px;
  height: 44px;
  font-size: 28px;
}

.stage.k8s .stage-icon {
  color: var(--lynq-green);
}

.rail-arrow {
  flex: 0 0 70px;
  align-self: center;
  height: 40px;
  display: flex;
  align-items: center;
}

/* Calmer marching ants: slow the flow and soften the dashes so the rail reads
   as a gentle pulse rather than a busy conveyor. */
.rail-arrow :deep(.animated .dash) {
  animation-duration: 2s;
  opacity: 0.7;
}

/* ── Detail panel ──
   FIXED reserved height so switching between steps (different YAML lengths /
   the terminal) never changes the panel height → no page reflow / CLS. The
   tallest step (LynqForm YAML) sets the floor; shorter steps just pad out. */
.panel {
  background: rgba(255, 255, 255, 0.03);
  /* FIXED reserved height → no reflow / CLS when steps swap. */
  min-height: 460px;
}

.panel-inner {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.15fr);
  gap: 2rem;
  align-items: start;
}

/* Heading weight token has no Tailwind utility — keep it here. */
.prose h3 {
  font-weight: var(--lynq-heading-weight);
}

/* The code/terminal object must fit the lane with NO inner horizontal scroll.
   YAML wraps long lines; the wide kubectl table shrinks its font to fit the
   lane width instead of scrolling sideways. */
.demo-object {
  max-width: 100%;
  overflow: hidden;
}

.detail :deep(.lynq-yaml-block .content) {
  overflow-x: hidden;
  font-size: 0.76rem;
  line-height: 1.5;
}

.detail :deep(.lynq-yaml-block .content code) {
  white-space: pre-wrap;
  word-break: break-word;
}

.detail :deep(.lynq-terminal .body) {
  overflow-x: hidden;
  font-size: 0.62rem;
  line-height: 1.5;
}

.detail :deep(.lynq-terminal .lynq-terminal-line) {
  white-space: pre;
}

/* Cross-fade between steps (calm; short travel). */
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

@media (prefers-reduced-motion: reduce) {
  .stage,
  .fade-slide-enter-active,
  .fade-slide-leave-active {
    transition: none;
  }
}

@media (max-width: 860px) {
  .rail {
    flex-wrap: wrap;
    gap: 0.75rem;
  }

  .rail-arrow {
    display: none;
  }

  .stage {
    flex: 1 1 40%;
    min-width: 120px;
  }

  .panel {
    min-height: 0;
  }

  .panel-inner {
    grid-template-columns: 1fr;
    gap: 1.5rem;
  }

  /* The kubectl table stays wide on narrow screens; allow it to scroll within
     its own box only here (the desktop no-scroll rule still holds above). */
  .detail :deep(.lynq-terminal .body) {
    overflow-x: auto;
    font-size: 0.68rem;
  }
}
</style>
