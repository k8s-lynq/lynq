<template>
  <div
    ref="root"
    class="lynq-stepstage flex flex-col gap-5 border border-lynq-border rounded-lynq bg-lynq-card p-5"
    @mouseenter="onPauseIntent"
    @mouseleave="onResumeIntent"
    @focusin="onPauseIntent"
    @focusout="onResumeIntent"
  >
    <!-- 0. Optional context block. Rendered ABOVE the filmstrip inside the outer
         frame (windflow's persona/goal/summary cluster). Renders nothing when
         the consumer supplies no #context slot content. -->
    <div
      v-if="$slots.context"
      class="border border-lynq-border rounded-lynq-sm bg-white/[0.02] px-[1.1rem] py-4"
    >
      <slot name="context" />
    </div>

    <!-- 1. Filmstrip: one chip per step, each a tiny 16:9 mini-preview + label —
         it reads like a filmstrip of frames. Active chip gets the accent border
         ring and auto-scrolls into view. Clicking a chip seeks + pauses
         auto-advance. -->
    <div
      class="filmstrip flex gap-2 p-1 overflow-x-auto"
      ref="strip"
      role="tablist"
      aria-label="Reconciliation steps"
    >
      <button
        v-for="(s, i) in steps"
        :key="s.id ?? i"
        ref="chipEls"
        type="button"
        role="tab"
        class="chip"
        :class="{ active: i === step }"
        :aria-selected="i === step"
        @click="onChip(i)"
      >
        <span class="chip-preview" aria-hidden="true">
          <span class="cp-index font-mono text-[0.66rem] text-lynq-faint">{{
            String(i + 1).padStart(2, '0')
          }}</span>
        </span>
        <span class="chip-label font-semibold whitespace-nowrap">{{ s.label }}</span>
      </button>
    </div>

    <!-- 2. Fixed-height split. Total height is reserved via minHeight so the
         stage never reflows as steps change. -->
    <div class="split" :style="{ minHeight: resolvedMinHeight }">
      <!-- LEFT: the section supplies the visual for the active step via the
           scoped #screen slot. The area is fixed-height and clips overflow so a
           too-tall/too-wide screen can never grow the stage. -->
      <div class="screen">
        <div class="screen-inner">
          <slot name="screen" :step="current" :index="step">
            <!-- Fallback keeps default (step 0) render valid with no consumer. -->
            <div class="screen-placeholder">{{ current?.title }}</div>
          </slot>
        </div>
      </div>

      <!-- RIGHT: annotation panel. Only the reasoning box scrolls (vertically). -->
      <div class="panel flex flex-col gap-4 p-6">
        <div class="panel-head flex flex-col gap-[0.4rem]">
          <span class="panel-step inline-flex items-baseline gap-2">
            <span class="ps-num font-mono text-base font-bold text-lynq-accent"
              >#{{ step + 1 }}</span
            >
            <span
              class="ps-word font-mono text-[0.72rem] tracking-[0.06em] uppercase text-lynq-faint"
              >Reconcile</span
            >
          </span>
          <h3
            class="panel-title m-0 text-[1.2rem] font-medium text-lynq-text leading-[1.2]"
          >
            {{ current?.title }}
          </h3>
        </div>

        <div class="reasoning">
          <p class="m-0 text-lynq-dim text-[0.95rem] leading-[1.6]">
            {{ current?.reasoning }}
          </p>
        </div>

        <div
          class="action flex items-baseline gap-2 flex-wrap pt-3 border-t border-lynq-border"
        >
          <span
            class="action-label text-[0.72rem] uppercase tracking-[0.06em] text-lynq-faint"
            >Action:</span
          >
          <code class="action-value font-mono text-[0.82rem] text-lynq-text break-words">{{
            current?.action
          }}</code>
        </div>

        <div class="controls flex items-center justify-between gap-4">
          <button
            type="button"
            class="playpause inline-flex items-center gap-[0.4rem] px-[0.8rem] py-[0.4rem] rounded-full border border-lynq-border bg-lynq-bg2 text-lynq-dim text-[0.8rem] font-semibold cursor-pointer"
            :aria-label="playing ? 'Pause walkthrough' : 'Play walkthrough'"
            @click="onToggle"
          >
            <LineIcon v-if="!playing" name="bolt" />
            <LineIcon v-else name="sync" />
            <span>{{ playing ? 'Pause' : 'Play' }}</span>
          </button>
          <span class="progress-dots inline-flex gap-[0.3rem]" aria-hidden="true">
            <span
              v-for="(s, i) in steps"
              :key="i"
              class="pdot"
              :class="{ on: i <= step }"
            />
          </span>
        </div>
      </div>
    </div>

    <!-- 3. Caption. -->
    <p
      v-if="caption"
      class="caption m-0 text-center text-[0.82rem] text-lynq-faint"
    >
      {{ caption }}
    </p>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useStepTimeline } from '../composables/useStepTimeline.js'
import { useInView } from '../composables/useInView.js'
import { useReducedMotion } from '../composables/useReducedMotion.js'
import LineIcon from './LineIcon.vue'

/**
 * A reusable, FIXED-HEIGHT stepped-walkthrough stage — the windflow-style
 * replica. Auto-advances through `steps`, loops, pauses on hover/focus, and
 * settles on the final frame under reduced motion. The consuming section
 * supplies each step's visual through the scoped `#screen` slot; this primitive
 * owns the filmstrip, the reasoning/action panel, timing, and the fixed frame.
 *
 * CLS contract: the stage's total height is constant across steps. The split is
 * reserved at `minHeight`; the LEFT screen area is fixed-height and clips
 * overflow; only the reasoning box scrolls, and only vertically. Nothing here
 * ever scrolls horizontally or grows the stage.
 */
const props = defineProps({
  // Array of { id, label, title, reasoning, action }.
  steps: { type: Array, required: true },
  // Auto-advance interval in ms.
  interval: { type: Number, default: 2700 },
  // Reserved fixed stage height (the split's min-height). Number => px.
  minHeight: { type: [String, Number], required: true },
  // Small line under the stage.
  caption: { type: String, default: '' },
})

const root = ref(null)
const strip = ref(null)
const chipEls = ref([])

const reduced = useReducedMotion()
const { inView } = useInView(root, { threshold: 0.35, once: true })

const { step, playing, play, pause, toggle, seek } = useStepTimeline({
  steps: props.steps.length,
  durations: props.interval,
  loop: true,
  respectReducedMotion: true,
})

const current = computed(() => props.steps[step.value] ?? props.steps[0])

const resolvedMinHeight = computed(() =>
  typeof props.minHeight === 'number' ? `${props.minHeight}px` : props.minHeight
)

// Tracks whether auto-play is desired for this stage (in view, not reduced).
// User pause via hover/focus/button is layered on top of this.
const wantAuto = ref(false)

// Start auto-play once scrolled into view (client only; timeline is mounted).
watch(inView, (v) => {
  if (v && !reduced.value) {
    wantAuto.value = true
    play()
  }
})

onMounted(() => {
  // If the observer never fires (e.g. tall viewport already showing it), and
  // motion is allowed, still begin when mounted + in view is resolved by watch.
  if (inView.value && !reduced.value) {
    wantAuto.value = true
    play()
  }
})

// Pause on hover/focus; resume on leave only if auto-play is still desired.
function onPauseIntent() {
  if (playing.value) pause()
}
function onResumeIntent() {
  if (wantAuto.value && !reduced.value && !playing.value) play()
}

function onToggle() {
  // Manual toggle also governs the auto-resume intent so leaving hover after an
  // explicit pause does not silently resume.
  if (playing.value) {
    wantAuto.value = false
    pause()
  } else {
    wantAuto.value = true
    toggle()
  }
}

// Clicking a chip seeks to that step and pauses auto-advance (windflow behavior).
function onChip(i) {
  wantAuto.value = false
  seek(i)
}

// Keep the active chip visible by scrolling ONLY the filmstrip's own
// horizontal overflow — never the page. scrollIntoView (even block:'nearest')
// yanks the whole window back to the walkthrough once it's scrolled off-screen,
// so we adjust the scroll container's scrollLeft directly instead.
watch(
  step,
  () => {
    nextTick(() => {
      const el = chipEls.value?.[step.value]
      if (!el) return
      // Find the nearest horizontally-scrollable ancestor (the filmstrip).
      let scroller = el.parentElement
      while (scroller && scroller.scrollWidth <= scroller.clientWidth) {
        scroller = scroller.parentElement
      }
      if (!scroller) return
      // Delta between chip and scroller in viewport space, mapped back to
      // scrollLeft — independent of offsetParent quirks.
      const chipRect = el.getBoundingClientRect()
      const scRect = scroller.getBoundingClientRect()
      const target =
        scroller.scrollLeft +
        (chipRect.left - scRect.left) -
        (scroller.clientWidth - chipRect.width) / 2
      scroller.scrollTo({
        left: Math.max(0, target),
        behavior: reduced.value ? 'auto' : 'smooth',
      })
    })
  },
  { flush: 'post' }
)
</script>

<style scoped>
/* HYBRID migration: static layout/spacing/typography/color now live as Tailwind
   utilities in the template. This scoped block keeps only what must stay CSS:
   the filmstrip's horizontal-scroll + scrollbar hiding, the chip/preview
   transitions and active-ring states, the fixed-height split (grid + reserved
   frames + overflow clipping — the CLS contract), the reasoning vertical
   scroll, and the transition/hover states on interactive controls. */

/* ---- Filmstrip: horizontal scroll container (scrollLeft JS-driven) ---- */
.filmstrip {
  scrollbar-width: none;
}
.filmstrip::-webkit-scrollbar {
  display: none;
}

/* Each chip is a small filmstrip frame: a 16:9 mini-preview box + a label.
   Base layout is scoped alongside its transitions/states for cohesion. */
.chip {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.35rem 0.75rem 0.35rem 0.35rem;
  border-radius: 10px;
  border: 1px solid var(--lynq-border);
  background: var(--lynq-bg-2);
  color: var(--lynq-text-dim);
  font-size: 0.82rem;
  cursor: pointer;
  transition: border-color 0.2s var(--lynq-ease), color 0.2s var(--lynq-ease),
    background 0.2s var(--lynq-ease);
}
.chip:hover {
  color: var(--lynq-text);
  border-color: rgba(79, 209, 203, 0.4);
}
.chip.active {
  border-color: var(--lynq-accent);
  color: var(--lynq-text);
  background: rgba(79, 209, 203, 0.1);
}

/* 16:9 mini-preview thumbnail — stylized frame, dim by default; the active
   chip's preview gets an accent ring so the strip reads as frames. */
.chip-preview {
  flex: 0 0 auto;
  position: relative;
  width: 46px;
  height: 26px;
  border-radius: 5px;
  border: 1px solid var(--lynq-border);
  background: linear-gradient(
    135deg,
    rgba(255, 255, 255, 0.05),
    rgba(255, 255, 255, 0.015)
  );
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  transition: border-color 0.2s var(--lynq-ease), box-shadow 0.2s var(--lynq-ease);
}
.chip.active .chip-preview {
  border-color: var(--lynq-accent);
  box-shadow: 0 0 0 1px var(--lynq-accent);
  background: linear-gradient(
    135deg,
    rgba(79, 209, 203, 0.22),
    rgba(79, 209, 203, 0.06)
  );
}
.chip.active .cp-index {
  color: var(--lynq-accent);
}

/* ---- Fixed-height split (CLS contract) ---- */
.split {
  display: grid;
  grid-template-columns: 1.5fr 1fr;
  gap: 1.25rem;
  align-items: stretch;
}

.screen,
.panel {
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius-sm);
  /* Subtle inset panels layered on the outer frame (windflow look). */
  background: rgba(255, 255, 255, 0.02);
  /* Both halves fill the reserved split height so total height is constant. */
  min-height: 0;
  height: auto;
}

/* LEFT screen: fixed frame, clips any overflow so it can't grow the stage. */
.screen {
  position: relative;
  overflow: hidden;
  padding: 1.5rem;
}
.screen-inner {
  position: absolute;
  inset: 1.5rem;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  /* Center sparse screens so they don't leave a big top-left void. */
  justify-content: center;
  align-items: stretch;
}
.screen-placeholder {
  margin: auto;
  color: var(--lynq-text-dim);
  font-family: var(--lynq-mono);
  font-size: 0.9rem;
}

/* Only this box scrolls, and only vertically. Fixed flex area within the panel. */
.reasoning {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 0.5rem;
}

/* Interactive control transitions/hover — kept scoped (motion-adjacent). */
.playpause {
  transition: color 0.2s var(--lynq-ease), border-color 0.2s var(--lynq-ease);
}
.playpause:hover {
  color: var(--lynq-text);
  border-color: rgba(79, 209, 203, 0.4);
}
.pdot {
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 999px;
  background: var(--lynq-border);
  transition: background 0.3s var(--lynq-ease);
}
.pdot.on {
  background: var(--lynq-accent);
}

/* Stack on narrow viewports; each half keeps its own fixed frame so the total
   reserved height (minHeight) still applies and nothing reflows per step. */
@media (max-width: 780px) {
  .split {
    grid-template-columns: 1fr;
  }
}
</style>
