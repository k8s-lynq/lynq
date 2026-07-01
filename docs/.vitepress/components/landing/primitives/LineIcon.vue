<template>
  <svg
    class="inline-block w-[1em] h-[1em] shrink-0 align-[-0.125em]"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
    v-html="path"
  />
</template>

<script setup>
import { computed } from 'vue'

/**
 * A tiny inline line-icon set (1.5px stroke, currentColor, sized 1em) that
 * replaces emoji across the landing sections. Each icon is drawn on a 24x24
 * viewBox. Pick by `name`; unknown names render nothing (empty svg) so a typo
 * never throws.
 *
 * Available names:
 *   database sync template cube shield lock refresh check x topology health
 *   events search bolt doc
 */
const props = defineProps({
  name: { type: String, required: true },
})

// Inner SVG markup per icon. Kept as raw path/shape strings so the outer <svg>
// owns stroke/size/color once. All coordinates are on a 24x24 grid.
const ICONS = {
  // Stacked cylinder — datasource / MySQL table.
  database:
    '<ellipse cx="12" cy="5" rx="7" ry="3"/><path d="M5 5v6c0 1.66 3.13 3 7 3s7-1.34 7-3V5"/><path d="M5 11v6c0 1.66 3.13 3 7 3s7-1.34 7-3v-6"/>',
  // Circular arrows — periodic sync / poll.
  sync:
    '<path d="M20 11a8 8 0 0 0-14.5-4.5L3 9"/><path d="M4 13a8 8 0 0 0 14.5 4.5L21 15"/><path d="M3 4v5h5"/><path d="M21 20v-5h-5"/>',
  // Layered squares — a form / blueprint that renders many.
  template:
    '<rect x="3" y="3" width="13" height="13" rx="2"/><path d="M8 21h9a2 2 0 0 0 2-2V9"/>',
  // Isometric box — a rendered/applied resource.
  cube:
    '<path d="M12 2.5 21 7v10l-9 4.5L3 17V7l9-4.5Z"/><path d="M3 7l9 4.5L21 7"/><path d="M12 11.5V21.5"/>',
  // Shield — policy / safety.
  shield: '<path d="M12 3 5 6v5c0 4.5 3 8 7 10 4-2 7-5.5 7-10V6l-7-3Z"/>',
  // Padlock — retain / protected.
  lock:
    '<rect x="4.5" y="10.5" width="15" height="10" rx="2"/><path d="M8 10.5V7a4 4 0 0 1 8 0v3.5"/>',
  // Single circular arrow — force reapply / drift correction.
  refresh:
    '<path d="M21 12a9 9 0 1 1-2.64-6.36"/><path d="M21 3v6h-6"/>',
  // Checkmark — ready / success.
  check: '<path d="M4.5 12.5 9.5 17.5 20 6.5"/>',
  // Cross — error / removed / the old way.
  x: '<path d="M6 6l12 12"/><path d="M18 6 6 18"/>',
  // Connected nodes — dependency graph / topology.
  topology:
    '<circle cx="6" cy="6" r="2.5"/><circle cx="18" cy="6" r="2.5"/><circle cx="12" cy="18" r="2.5"/><path d="M7.7 8 10.5 15.8"/><path d="M16.3 8 13.5 15.8"/>',
  // Heartbeat pulse — readiness / health checks.
  health: '<path d="M2 12h4l2.5-6 4 12 2.5-6H22"/>',
  // Bell — events emitted.
  events:
    '<path d="M6 9a6 6 0 0 1 12 0c0 5 2 6 2 6H4s2-1 2-6Z"/><path d="M10 20a2 2 0 0 0 4 0"/>',
  // Magnifier — search / detection / drift watch.
  search:
    '<circle cx="10.5" cy="10.5" r="6.5"/><path d="M20 20 15.5 15.5"/>',
  // Lightning bolt — fast / immediate reconcile.
  bolt: '<path d="M13 2 4 14h6l-1 8 9-12h-6l1-8Z"/>',
  // Document — manifest / YAML / template output.
  doc:
    '<path d="M6 2h8l5 5v13a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1Z"/><path d="M14 2v5h5"/><path d="M9 13h6"/><path d="M9 17h6"/>',
}

const path = computed(() => ICONS[props.name] ?? '')
</script>
