<template>
  <div v-if="revealed" class="block font-mono whitespace-pre text-lynq-text">
    <span v-if="prompt" class="text-lynq-green mr-[0.6rem] select-none">{{ prompt }}</span>
    <span class="text">{{ text }}</span><span
      v-if="active"
      class="caret"
      aria-hidden="true"
    ></span>
  </div>
</template>

<script setup>
defineProps({
  prompt: { type: String, default: '' },
  text: { type: String, required: true },
  revealed: { type: Boolean, default: true },
  active: { type: Boolean, default: false },
})
</script>

<style scoped>
/* Layout/typography moved to Tailwind utilities in the template.
   The blinking caret stays here (animation + reduced-motion). */
.caret {
  display: inline-block;
  width: 0.5rem;
  height: 1em;
  margin-left: 1px;
  vertical-align: text-bottom;
  background: var(--lynq-text);
  animation: lynq-caret-blink 1s step-end infinite;
}

@keyframes lynq-caret-blink {
  0%,
  50% {
    opacity: 1;
  }
  50.01%,
  100% {
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .caret {
    animation: none;
  }
}
</style>
