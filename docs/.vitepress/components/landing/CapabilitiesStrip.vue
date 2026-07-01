<template>
  <div class="border-y border-white/[0.06] bg-white/[0.02] px-8 py-7">
    <div class="mx-auto max-w-[1040px]">
      <div class="flex flex-wrap justify-center">
        <div
          class="cap-item flex flex-col items-center gap-1 border-r border-white/[0.08] px-6 py-3 last:border-r-0"
          v-for="cap in caps"
          :key="cap.label"
        >
          <span class="text-lg font-bold tracking-[-0.01em] whitespace-nowrap text-white">{{ cap.value }}</span>
          <span class="text-xs uppercase tracking-[0.06em] whitespace-nowrap text-white/45">{{ cap.label }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
const caps = [
  { value: '16', label: 'resource types' },
  { value: '3', label: 'lifecycle policies' },
  { value: 'MySQL', label: 'built-in datasource' },
  { value: 'Kubernetes 1.28+', label: 'cluster requirement' },
  { value: 'Apache 2.0', label: 'open source license' },
]
</script>

<style scoped>
/* Responsive stacking only — layout/typography live in Tailwind utilities. */
@media (max-width: 768px) {
  .cap-item {
    width: calc(50% - 0.5rem);
    align-items: flex-start;
    padding: 0.5rem 1.25rem;
    border-right: none;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  }
  .cap-item:last-child {
    border-bottom: none;
  }
}
</style>
