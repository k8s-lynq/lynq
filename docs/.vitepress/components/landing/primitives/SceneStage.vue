<template>
  <div class="flex flex-col gap-5" ref="rootRef">
    <div class="min-w-0">
      <slot
        :step="step"
        :playing="playing"
        :progress="progress"
        :seek="seek"
        :next="next"
        :prev="prev"
        :toggle="toggle"
      />
    </div>

    <div
      class="flex items-center gap-4 flex-wrap justify-center"
      role="group"
      aria-label="Walkthrough controls"
    >
      <button
        class="ctrl inline-flex items-center justify-center w-8 h-8 rounded-full border border-lynq-border bg-lynq-card text-lynq-text text-[0.7rem] cursor-pointer"
        type="button"
        :disabled="reduced"
        :aria-label="playing ? 'Pause walkthrough' : 'Play walkthrough'"
        @click="toggle"
      >
        <span v-if="playing" aria-hidden="true">❚❚</span>
        <span v-else aria-hidden="true">▶</span>
      </button>

      <div class="flex gap-2" role="tablist">
        <button
          v-for="i in steps"
          :key="i - 1"
          class="dot w-[0.55rem] h-[0.55rem] rounded-full border-none p-0 bg-lynq-faint cursor-pointer"
          type="button"
          :class="{ on: step === i - 1 }"
          :aria-label="`Go to step ${i}`"
          :aria-selected="step === i - 1"
          @click="seek(i - 1)"
        ></button>
      </div>

      <span
        v-if="reduced"
        class="font-mono text-[0.7rem] text-lynq-faint"
      >
        Reduced motion: showing final state
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useInView } from '../composables/useInView.js'
import { useStepTimeline } from '../composables/useStepTimeline.js'
import { useReducedMotion } from '../composables/useReducedMotion.js'

const props = defineProps({
  steps: { type: Number, required: true },
  loop: { type: Boolean, default: false },
  // Start the timeline automatically when scrolled into view.
  autoInView: { type: Boolean, default: true },
  // Optional per-beat durations (ms) passed through to the timeline.
  durations: { type: [Array, Number], default: undefined },
})

const rootRef = ref(null)
const reduced = useReducedMotion()

const { inView } = useInView(rootRef, { threshold: 0.25, once: true })

const { step, playing, progress, play, pause, toggle, seek, next, prev } =
  useStepTimeline({
    steps: props.steps,
    durations: props.durations,
    loop: props.loop,
    autoStart: false,
    respectReducedMotion: true,
  })

// Kick off auto-play the first time the stage scrolls into view (unless the
// user prefers reduced motion — the timeline itself settles on the final frame).
watch(inView, (visible) => {
  if (visible && props.autoInView && !reduced.value) {
    play()
  }
})
</script>

<style scoped>
/* Static layout/sizing/color moved to Tailwind utilities in the template.
   Kept here: transitions + interactive (hover/disabled/active-dot) states,
   which toggle on class/pseudo and read cleaner as scoped rules. */
.ctrl {
  transition: border-color 0.2s var(--lynq-ease), color 0.2s var(--lynq-ease);
}

.ctrl:hover:not(:disabled) {
  border-color: var(--lynq-purple);
  color: var(--lynq-purple);
}

.ctrl:disabled {
  opacity: 0.4;
  cursor: default;
}

.dot {
  transition: background 0.2s var(--lynq-ease), transform 0.2s var(--lynq-ease);
}

.dot.on {
  background: var(--lynq-purple);
  transform: scale(1.25);
}
</style>
