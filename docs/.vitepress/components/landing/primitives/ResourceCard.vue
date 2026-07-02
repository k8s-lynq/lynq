<template>
  <div
    class="flex items-stretch bg-lynq-card border border-lynq-border rounded-lynq-sm overflow-hidden"
    :data-status="status"
  >
    <span
      class="flex-none w-1"
      :style="{ background: railColor }"
      aria-hidden="true"
    ></span>
    <div class="flex-1 min-w-0 px-[0.85rem] py-[0.65rem] flex flex-col gap-[0.3rem]">
      <div class="flex items-center justify-between gap-3">
        <span
          class="font-mono text-[0.7rem] font-bold uppercase tracking-[0.06em]"
          :style="{ color: railColor }"
          >{{ kind }}</span
        >
        <StatusBadge :status="status" />
      </div>
      <div class="font-mono text-[0.9rem] text-lynq-text truncate">{{ name }}</div>
      <div v-if="meta" class="font-mono text-[0.7rem] text-lynq-faint">{{ meta }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  kind: { type: String, required: true },
  name: { type: String, required: true },
  status: { type: String, default: 'ready' },
  meta: { type: String, default: '' },
})

// Color rail keyed by resource kind; unknown kinds fall back to purple.
const KIND_COLOR = {
  Deployment: 'var(--lynq-purple)',
  Service: 'var(--lynq-blue)',
  Ingress: 'var(--lynq-green)',
  ConfigMap: 'var(--lynq-purple)',
  Secret: 'var(--lynq-amber)',
  PVC: 'var(--lynq-blue)',
  PersistentVolumeClaim: 'var(--lynq-blue)',
  Namespace: 'var(--lynq-accent)',
  StatefulSet: 'var(--lynq-purple)',
}

const railColor = computed(() => KIND_COLOR[props.kind] ?? 'var(--lynq-purple)')
</script>
