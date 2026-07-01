<template>
  <span
    class="lynq-status-badge inline-flex items-center gap-[0.4rem] rounded-full px-[0.65rem] py-[0.2rem] font-mono text-[0.72rem] font-semibold leading-none whitespace-nowrap"
    :class="`is-${status}`"
    :data-status="status"
  >
    <span
      class="dot inline-flex items-center justify-center min-w-[0.7rem] h-[0.7rem] rounded-full text-[0.6rem]"
      :class="{ pulse: showPulse }"
      aria-hidden="true"
      >{{ glyph }}</span
    >
    <span class="text">{{ label }}</span>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useReducedMotion } from '../composables/useReducedMotion.js'

const props = defineProps({
  status: {
    type: String,
    required: true,
    validator: (v) =>
      ['pending', 'ready', 'failed', 'conflicted', 'retained', 'skipped'].includes(v),
  },
  // pulse defaults on for `pending` only; explicit false disables it.
  pulse: { type: Boolean, default: true },
})

const reduced = useReducedMotion()

const MAP = {
  pending: { label: 'Pending', glyph: '' },
  ready: { label: 'Ready', glyph: '✓' },
  failed: { label: 'Failed', glyph: '✕' },
  conflicted: { label: 'Conflicted', glyph: '⚠' },
  retained: { label: 'Retained', glyph: '🔒' },
  skipped: { label: 'Skipped', glyph: '–' },
}

const label = computed(() => MAP[props.status]?.label ?? props.status)
const glyph = computed(() => MAP[props.status]?.glyph ?? '')

// Pulse only for pending, only when opted in, never under reduced motion.
const showPulse = computed(
  () => props.status === 'pending' && props.pulse && !reduced.value
)
</script>

<style scoped>
/* Base layout/typography moved to Tailwind utilities in the template.
   Kept here: status color variants (which also drive the child .dot color),
   the empty-dot shape, and the pulse animation. */

/* When there's no glyph, render a solid colored dot. */
.dot:empty {
  width: 0.55rem;
  height: 0.55rem;
  min-width: 0.55rem;
}

.is-pending {
  color: var(--lynq-amber);
  background: rgba(245, 158, 11, 0.14);
}
.is-pending .dot {
  background: var(--lynq-amber);
}

.is-ready {
  color: var(--lynq-green);
  background: rgba(16, 185, 129, 0.14);
}

.is-failed {
  color: var(--lynq-red);
  background: rgba(239, 68, 68, 0.14);
}

.is-conflicted {
  color: var(--lynq-amber);
  background: rgba(245, 158, 11, 0.14);
}

.is-retained {
  color: var(--lynq-blue);
  background: rgba(59, 130, 246, 0.14);
}

.is-skipped {
  color: var(--lynq-text-dim);
  background: rgba(255, 255, 255, 0.06);
}
.is-skipped .dot {
  background: var(--lynq-text-faint);
}

.pulse {
  animation: lynq-badge-pulse 1.4s var(--lynq-ease) infinite;
}

@keyframes lynq-badge-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(245, 158, 11, 0.5);
  }
  50% {
    box-shadow: 0 0 0 4px rgba(245, 158, 11, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .pulse {
    animation: none;
  }
}
</style>
