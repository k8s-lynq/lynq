<template>
  <section class="live-transform scroll-mt-20 px-8" ref="sectionRef">
    <div class="mx-auto" style="max-width: 1040px" ref="containerRef">
      <SectionHeader
        label="How It Works"
        title="From Data to Resources in Seconds"
        subtitle="A real operator session — apply two manifests, then watch every row reconcile into named, running Kubernetes resources."
        accent="green"
      />

      <!-- Single authentic terminal that replays the whole workflow. -->
      <div class="term rounded-lynq-sm border border-lynq-border overflow-hidden" @click="onTermClick">
        <div class="term-bar">
          <span class="dots" aria-hidden="true"><i></i><i></i><i></i></span>
          <span class="term-title">lynq — kubectl · zsh</span>
          <button
            v-if="finished && !reduced"
            class="term-replay"
            type="button"
            @click.stop="replay"
            aria-label="Replay session"
          >↻ replay</button>
          <span v-else class="term-replay-spacer" aria-hidden="true"></span>
        </div>

        <div class="term-scroll" ref="scrollRef" role="log" aria-label="Terminal session">
          <div
            v-for="(ln, i) in rendered"
            :key="i"
            class="tline"
            :class="ln.tone"
          ><span v-if="ln.prompt" class="tprompt">{{ ln.prompt }}</span><span class="ttxt">{{ ln.text || ' ' }}</span></div>

          <div v-if="typing" class="tline cmd"><span class="tprompt">{{ typing.prompt }}</span><span class="ttxt">{{ typing.text }}</span><span class="caret" aria-hidden="true"></span></div>
          <div v-else-if="atPrompt" class="tline cmd"><span class="tprompt">$</span><span class="ttxt"> </span><span class="caret" aria-hidden="true"></span></div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import SectionHeader from '../primitives/SectionHeader.vue'
import { useInView } from '../composables/useInView.js'
import { useReducedMotion } from '../composables/useReducedMotion.js'
import { terminalSession } from '../data/demoData.js'

const reduced = useReducedMotion()
const sectionRef = ref(null)
const containerRef = ref(null)
const scrollRef = ref(null)
const { inView } = useInView(containerRef, { threshold: 0.25, once: true })

// Committed lines (fully printed), the in-progress command being typed, and
// whether the session has settled at a final idle prompt.
const rendered = ref([])
const typing = ref(null) // { prompt, text } | null
const atPrompt = ref(false)
const finished = ref(false)

// Cancellation: every run captures a token; a newer run (or unmount) bumps the
// counter so the older async loop bails on its next await. All timeouts are
// tracked so they can be cleared on reset/teardown (no leaks, SSR-safe).
let runToken = 0
const timers = new Set()
function clearTimers() {
  timers.forEach((t) => clearTimeout(t))
  timers.clear()
}
function wait(ms) {
  return new Promise((res) => {
    const t = setTimeout(() => {
      timers.delete(t)
      res()
    }, ms)
    timers.add(t)
  })
}
function scrollBottom() {
  nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

// Reduced motion / SSR: print the full transcript at once, no typing or waits.
function renderStatic() {
  clearTimers()
  runToken++
  rendered.value = terminalSession.map((s) => {
    if (s.type === 'gap') return { prompt: '', text: '', tone: '' }
    if (s.type === 'cmd') return { prompt: s.prompt || '$', text: s.text, tone: 'cmd' }
    return { prompt: '', text: s.text, tone: s.tone || '' }
  })
  typing.value = null
  atPrompt.value = true
  finished.value = true
}

async function run() {
  clearTimers()
  const token = ++runToken
  rendered.value = []
  typing.value = null
  atPrompt.value = false
  finished.value = false

  for (const step of terminalSession) {
    if (token !== runToken) return

    if (step.type === 'cmd') {
      const prompt = step.prompt || '$'
      typing.value = { prompt, text: '' }
      for (const ch of step.text) {
        if (token !== runToken) return
        typing.value = { prompt, text: typing.value.text + ch }
        scrollBottom()
        // Human-ish keystroke cadence with slight jitter.
        await wait(28 + Math.random() * 46)
      }
      // Pause after Enter, mimicking command dispatch + API latency.
      await wait(430)
      if (token !== runToken) return
      rendered.value.push({ prompt, text: step.text, tone: 'cmd' })
      typing.value = null
      scrollBottom()
    } else if (step.type === 'gap') {
      rendered.value.push({ prompt: '', text: '', tone: '' })
      scrollBottom()
      await wait(step.delay ?? 130)
    } else {
      rendered.value.push({ prompt: '', text: step.text, tone: step.tone || '' })
      scrollBottom()
      await wait(step.delay ?? 85)
    }
  }

  if (token !== runToken) return
  atPrompt.value = true
  finished.value = true
}

// Start exactly once when the section first scrolls into view.
let started = false
function start() {
  if (started) return
  started = true
  if (reduced.value) renderStatic()
  else run()
}
function replay() {
  if (!reduced.value) run()
}
function onTermClick() {
  // Click-to-replay once the session has settled (matches the visible control).
  if (finished.value && !reduced.value) run()
}

watch(inView, (v) => {
  if (v) start()
})

onMounted(() => {
  // If the observer never fires (reduced motion users often have it disabled
  // anyway) settle the static transcript so the section is never blank.
  if (reduced.value) start()
})

onBeforeUnmount(() => {
  runToken++
  clearTimers()
})
</script>

<style scoped>
.live-transform {
  padding-block: var(--lynq-section-y);
  background: linear-gradient(180deg, var(--lynq-bg) 0%, var(--lynq-bg-2) 100%);
}

.live-transform :deep(.lynq-section-header h2) {
  font-size: var(--lynq-h2);
  font-weight: var(--lynq-heading-weight);
}

/* ── Terminal chrome (matches TerminalWindow primitive) ── */
.term {
  background: var(--lynq-bg);
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  cursor: default;
}

.term-bar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0.9rem;
  background: #252538;
  border-bottom: 1px solid var(--lynq-border);
}

.dots {
  display: flex;
  gap: 0.5rem;
  flex: none;
}
.dots i {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 9999px;
}
.dots i:nth-child(1) { background: #ff5f57; }
.dots i:nth-child(2) { background: #febc2e; }
.dots i:nth-child(3) { background: #28c840; }

.term-title {
  flex: 1;
  text-align: center;
  font-family: var(--lynq-mono);
  font-size: 0.78rem;
  color: var(--lynq-text-dim);
}

.term-replay,
.term-replay-spacer {
  flex: none;
  min-width: 74px;
  text-align: right;
}
.term-replay {
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  color: var(--lynq-text-dim);
  background: transparent;
  border: 0;
  cursor: pointer;
  padding: 0.15rem 0.3rem;
  border-radius: 6px;
  transition: color 0.2s ease, background 0.2s ease;
}
.term-replay:hover {
  color: var(--lynq-text);
  background: rgba(255, 255, 255, 0.08);
}

/* ── Scrolling body ──
   Fixed height → no CLS; scrolls like a real terminal as output streams. */
.term-scroll {
  height: 440px;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 1rem 1.1rem;
  font-family: var(--lynq-mono);
  font-size: 0.82rem;
  line-height: 1.6;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.15) transparent;
}
.term-scroll::-webkit-scrollbar { width: 8px; }
.term-scroll::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15);
  border-radius: 4px;
}

.tline {
  white-space: pre;
  color: rgba(255, 255, 255, 0.72);
}
.tprompt {
  color: var(--lynq-green);
  margin-right: 0.6rem;
  user-select: none;
}
.tline.cmd .ttxt { color: var(--lynq-text); }
.tline.ok .ttxt { color: #4fd1cb; }
.tline.dim .ttxt { color: rgba(255, 255, 255, 0.38); }

.caret {
  display: inline-block;
  width: 0.5rem;
  height: 1em;
  margin-left: 2px;
  vertical-align: text-bottom;
  background: var(--lynq-text);
  animation: term-caret 1s step-end infinite;
}
@keyframes term-caret {
  0%, 50% { opacity: 1; }
  50.01%, 100% { opacity: 0; }
}

@media (prefers-reduced-motion: reduce) {
  .caret { animation: none; }
}

@media (max-width: 860px) {
  .term-scroll {
    height: 380px;
    overflow-x: auto;
    font-size: 0.72rem;
  }
}
</style>
