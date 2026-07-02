<template>
  <tr class="lynq-table-row" :class="[`state-${state}`, { active }]" :data-state="state">
    <td v-for="(cell, i) in cells" :key="i">{{ cell }}</td>
  </tr>
</template>

<script setup>
defineProps({
  cells: { type: Array, required: true },
  state: {
    type: String,
    default: 'idle',
    validator: (v) => ['idle', 'highlight', 'inserting', 'deactivating'].includes(v),
  },
  active: { type: Boolean, default: false },
})
</script>

<style scoped>
.lynq-table-row td {
  transition: background 0.3s var(--lynq-ease), color 0.3s var(--lynq-ease);
  border-left: 3px solid transparent;
}

.lynq-table-row td:first-child {
  border-left-width: 3px;
}

/* highlight: purple left-rail + subtle wash on the first cell. */
.state-highlight td:first-child {
  border-left-color: var(--lynq-purple);
}
.state-highlight td {
  background: rgba(51, 172, 168, 0.08);
}

/* inserting: slide + fade in (disabled under reduced motion). */
.state-inserting {
  animation: lynq-row-insert 0.5s var(--lynq-ease) both;
}
.state-inserting td:first-child {
  border-left-color: var(--lynq-green);
}
.state-inserting td {
  background: rgba(16, 185, 129, 0.08);
}

/* deactivating: desaturate + strike-through. */
.state-deactivating td {
  color: var(--lynq-text-faint);
  text-decoration: line-through;
  background: rgba(255, 255, 255, 0.015);
}

.active td {
  background: rgba(51, 172, 168, 0.06);
}

@keyframes lynq-row-insert {
  from {
    opacity: 0;
    transform: translateY(-8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .state-inserting {
    animation: none;
  }
}
</style>
