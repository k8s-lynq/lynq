<template>
  <section
    class="hero-demo relative flex w-full flex-col items-center overflow-hidden bg-lynq-bg px-8 pt-44 pb-20"
    ref="rootRef"
  >
    <!-- Interactive grid backdrop (Inspira-style): cells light teal under the
         cursor and fade as it moves on, and a subtle left→right "current" sweeps
         column by column — the Database → Infrastructure flow, dissolved into the
         background. Pure CSS; edge-masked so it stays ambient. -->
    <div
      class="grid-bg"
      :style="{ gridTemplateColumns: `repeat(${cols}, 1fr)` }"
      aria-hidden="true"
    >
      <span
        v-for="i in cellCount"
        :key="i"
        class="gcell"
        :style="{ '--col': (i - 1) % cols }"
      ></span>
    </div>

    <div class="pointer-events-none relative z-10 mx-auto flex w-full max-w-[1040px] flex-col items-center text-center">
      <!-- headline + CTAs -->
      <div class="flex min-w-0 max-w-[760px] flex-col items-center">
        <motion.div
          class="mb-8 inline-flex items-center gap-2 rounded-full border border-lynq-border bg-white/[0.03] px-[0.9rem] py-[0.4rem] font-mono text-[0.78rem] tracking-[0.02em] text-lynq-dim"
          :initial="entrance.hidden ? { opacity: 0, y: 16 } : false"
          :animate="entrance.animate"
          :transition="{ duration: 0.6, delay: 0.1 }"
        >
          <span class="text-lynq-accent" aria-hidden="true">&#9670;</span>
          Infrastructure as Data
        </motion.div>

        <motion.h1
          class="hero-title m-0 mb-6 text-lynq-text"
          :initial="entrance.hidden ? { opacity: 0, y: 24 } : false"
          :animate="entrance.animate"
          :transition="{ duration: 0.7, delay: 0.25 }"
        >
          Your Database.<br />
          <span class="hero-title-accent text-lynq-accent">Your Infrastructure.</span>
        </motion.h1>

        <motion.p
          class="mx-auto mb-11 max-w-[46ch] text-[1.125rem] leading-[1.62] text-lynq-dim"
          :initial="entrance.hidden ? { opacity: 0, y: 16 } : false"
          :animate="entrance.animate"
          :transition="{ duration: 0.6, delay: 0.45 }"
        >
          Lynq turns database records into Kubernetes resources. Automatically.
        </motion.p>

        <motion.div
          class="flex flex-wrap justify-center gap-3"
          :initial="entrance.hidden ? { opacity: 0, y: 16 } : false"
          :animate="entrance.animate"
          :transition="{ duration: 0.6, delay: 0.6 }"
        >
          <a
            href="/quickstart"
            class="cta-primary group pointer-events-auto inline-flex items-center gap-2 rounded-full bg-lynq-text! px-6 py-[0.72rem] text-[0.95rem] font-medium! text-[#0a0a0a]! no-underline transition-opacity duration-200 hover:opacity-90"
          >
            Get Started
            <span class="arrow inline-block transition-transform duration-200 group-hover:translate-x-[3px]" aria-hidden="true">&#8594;</span>
          </a>
          <a
            href="https://github.com/k8s-lynq/lynq"
            class="pointer-events-auto inline-flex items-center gap-2 rounded-full border border-lynq-border bg-transparent px-6 py-[0.72rem] text-[0.95rem] font-medium! text-lynq-text! no-underline transition-colors duration-200 hover:border-white/20 hover:bg-white/[0.06]"
            target="_blank"
            rel="noopener"
          >
            <svg class="h-[18px] w-[18px]" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path
                d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"
              />
            </svg>
            View on GitHub
          </a>
        </motion.div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { motion } from 'motion-v'
import { useReducedMotion } from '../composables/useReducedMotion.js'

const reduced = useReducedMotion()
const rootRef = ref(null)

/* Headline entrance stagger. Only animates on the client after mount; SSR /
 * reduced-motion render the resting state so first paint matches hydration. */
const entrance = computed(() => {
  const settled = { opacity: 1, y: 0 }
  if (reduced.value) return { hidden: false, animate: settled }
  return { hidden: true, animate: settled }
})

/* Interactive grid backdrop. Cell count/columns are measured from the hero so
 * the grid fills it exactly (no wasted DOM); a handful of cells are pre-lit and
 * gently pulse. SSR renders 0 cells (decorative, aria-hidden) → filled on mount. */
const CELL = 46
const cols = ref(0)
const cellCount = ref(0)
let ro = null

function measure() {
  const el = rootRef.value
  if (!el) return
  const w = el.clientWidth
  const h = el.clientHeight
  if (w <= 0 || h <= 0) return
  const c = Math.ceil(w / CELL)
  cols.value = c
  cellCount.value = c * Math.ceil(h / CELL)
}

onMounted(() => {
  measure()
  if (typeof ResizeObserver !== 'undefined') {
    ro = new ResizeObserver(() => measure())
    if (rootRef.value) ro.observe(rootRef.value)
  }
})
onBeforeUnmount(() => {
  if (ro) ro.disconnect()
})
</script>

<style scoped>
/* ---- Interactive grid backdrop ---- */
.grid-bg {
  position: absolute;
  inset: 0;
  z-index: 0;
  display: grid;
  grid-auto-rows: 46px;
  pointer-events: auto;
  /* fade to an ambient backdrop toward the edges */
  -webkit-mask-image: radial-gradient(ellipse 82% 64% at 50% 42%, #000 22%, transparent 82%);
  mask-image: radial-gradient(ellipse 82% 64% at 50% 42%, #000 22%, transparent 82%);
}
.gcell {
  position: relative;
  border-right: 1px solid rgba(255, 255, 255, 0.035);
  border-bottom: 1px solid rgba(255, 255, 255, 0.035);
  transition: background 0.7s ease; /* slow fade-out leaves a trail on hover */
}
.gcell:hover {
  background: rgba(51, 172, 168, 0.24);
  transition: background 0s; /* light up instantly under the cursor */
}
/* the Database → Infrastructure current: an overlay that pulses per column, so a
   faint teal band sweeps left → right across the grid. Uses ::before so it never
   clashes with the hover background. */
.gcell::before {
  content: '';
  position: absolute;
  inset: 0;
  background: rgba(79, 209, 203, 0.16);
  opacity: 0;
}
@media (prefers-reduced-motion: no-preference) {
  .gcell::before {
    animation: g-flow 7s linear infinite;
    animation-delay: calc(var(--col, 0) * 0.16s);
  }
}
@keyframes g-flow {
  0%       { opacity: 0; }
  4%       { opacity: 1; }
  11%      { opacity: 0; }
  100%     { opacity: 0; }
}

/* Solid accent for the second headline line. A VitePress reset can strip the
   transparent gradient-text-clip approach, so pin the fill color explicitly. */
.hero-title .hero-title-accent {
  color: var(--lynq-accent);
  -webkit-text-fill-color: var(--lynq-accent);
  background: none;
}

/* ---- responsive ---- */
@media (max-width: 900px) {
  .hero-demo {
    padding: 5.5rem 1.25rem 4rem;
  }
}
</style>
