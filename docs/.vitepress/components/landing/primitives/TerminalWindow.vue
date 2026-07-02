<template>
  <div
    class="bg-lynq-bg border border-lynq-border rounded-lynq-sm overflow-hidden shadow-[0_25px_50px_-12px_rgba(0,0,0,0.5)]"
  >
    <div
      class="flex items-center gap-3 px-[0.9rem] py-[0.6rem] bg-[#252538] border-b border-lynq-border"
    >
      <div class="flex gap-2" aria-hidden="true">
        <span class="w-3 h-3 rounded-full bg-[#ff5f57]"></span>
        <span class="w-3 h-3 rounded-full bg-[#febc2e]"></span>
        <span class="w-3 h-3 rounded-full bg-[#28c840]"></span>
      </div>
      <span
        v-if="title"
        class="flex-1 text-center font-mono text-[0.78rem] text-lynq-dim"
        >{{ title }}</span
      >
      <!-- Keep the title visually centered by balancing the dots width. -->
      <span class="w-[54px] flex-none" aria-hidden="true"></span>
    </div>
    <div
      class="p-4 font-mono text-[0.85rem] leading-[1.55] text-lynq-text overflow-x-auto"
    >
      <slot />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '' },
})
</script>
