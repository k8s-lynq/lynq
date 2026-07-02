<template>
  <div
    class="lynq-arrow-flow flex items-center justify-center gap-[0.35rem]"
    :class="[`dir-${direction}`, { animated }]"
  >
    <svg
      class="line"
      :viewBox="direction === 'right' ? '0 0 100 12' : '0 0 12 100'"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      <line
        v-if="direction === 'right'"
        x1="0"
        y1="6"
        x2="100"
        y2="6"
        :stroke="strokeColor"
        stroke-width="2"
        stroke-dasharray="4 4"
        class="dash"
      />
      <line
        v-else
        x1="6"
        y1="0"
        x2="6"
        y2="100"
        :stroke="strokeColor"
        stroke-width="2"
        stroke-dasharray="4 4"
        class="dash"
      />
    </svg>
    <span
      v-if="label"
      class="font-mono text-[0.66rem] uppercase tracking-[0.06em] opacity-[0.85]"
      :style="{ color: strokeColor }"
      >{{ label }}</span
    >
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useReducedMotion } from '../composables/useReducedMotion.js'

const props = defineProps({
  direction: {
    type: String,
    default: 'right',
    validator: (v) => ['right', 'down'].includes(v),
  },
  color: {
    type: String,
    default: 'purple',
    validator: (v) => ['purple', 'accent', 'green', 'blue', 'red', 'amber'].includes(v),
  },
  active: { type: Boolean, default: true },
  label: { type: String, default: '' },
})

const reduced = useReducedMotion()

const strokeColor = computed(() => `var(--lynq-${props.color})`)

// Marching ants only when active AND motion is allowed; otherwise a static
// dashed line.
const animated = computed(() => props.active && !reduced.value)
</script>

<style scoped>
/* Base flex layout + label typography moved to Tailwind utilities in the
   template. Kept here: direction-dependent line sizing (coupled to the toggled
   .dir-* class) and the marching-ants animation. */
.dir-right {
  flex-direction: column;
}

.dir-down {
  flex-direction: row;
}

.dir-right .line {
  width: 100%;
  min-width: 40px;
  height: 12px;
}

.dir-down .line {
  height: 100%;
  min-height: 40px;
  width: 12px;
}

.animated .dash {
  animation: lynq-march 0.8s linear infinite;
}

@keyframes lynq-march {
  to {
    stroke-dashoffset: -8;
  }
}

@media (prefers-reduced-motion: reduce) {
  .animated .dash {
    animation: none;
  }
}
</style>
