<template>
  <div
    class="lynq-data-table bg-lynq-card border border-lynq-border rounded-lynq-sm overflow-hidden"
  >
    <div
      v-if="caption"
      class="flex items-center gap-2 px-[0.9rem] py-[0.6rem] border-b border-lynq-border bg-white/[0.02]"
    >
      <span class="text-lynq-blue text-[0.85rem]" aria-hidden="true">▤</span>
      <span class="font-mono text-[0.8rem] text-lynq-dim">{{ caption }}</span>
    </div>
    <div class="overflow-x-auto">
      <table class="w-full border-collapse font-mono text-[0.82rem]">
        <thead>
          <tr>
            <th
              v-for="col in columns"
              :key="col"
              class="text-left px-[0.9rem] py-[0.55rem] text-lynq-faint font-semibold lowercase border-b border-lynq-border whitespace-nowrap"
            >
              {{ col }}
            </th>
          </tr>
        </thead>
        <tbody>
          <slot />
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
defineProps({
  columns: { type: Array, required: true },
  caption: { type: String, default: '' },
})
</script>

<style scoped>
/* Container/caption/table styling moved to Tailwind utilities in the template.
   Kept here: shared cell borders on SLOTTED rows (TableRow.vue), which must be
   reached via :deep since they aren't in this component's own template. */

/* Row/cell styling lives in TableRow.vue; keep shared borders here via
   :deep so slotted <tr>/<td> pick them up. */
.lynq-data-table :deep(td) {
  padding: 0.5rem 0.9rem;
  border-bottom: 1px solid var(--lynq-border);
  color: var(--lynq-text);
  white-space: nowrap;
}

.lynq-data-table :deep(tbody tr:last-child td) {
  border-bottom: none;
}
</style>
