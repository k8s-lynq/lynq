<template>
  <div class="text-center mb-12">
    <span
      class="inline-block text-sm font-semibold uppercase tracking-[0.1em] mb-3"
      :style="{ color: accentColor }"
      >{{ label }}</span
    >
    <h2
      class="section-h2 m-0 mb-4 text-lynq-text"
    >
      {{ title }}
    </h2>
    <p
      v-if="subtitle"
      class="mx-auto! my-0 max-w-3xl text-[1.1rem] text-lynq-dim leading-[1.6]"
    >
      {{ subtitle }}
    </p>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  accent: {
    type: String,
    default: 'purple',
    validator: (v) => ['purple', 'green', 'blue', 'red', 'amber'].includes(v),
  },
})

const accentColor = computed(() => `var(--lynq-${props.accent})`)
</script>
