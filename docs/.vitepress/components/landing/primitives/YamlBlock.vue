<template>
  <div class="bg-lynq-bg border border-lynq-border rounded-lynq-sm overflow-hidden">
    <div
      class="flex justify-between items-center px-[0.9rem] py-[0.6rem] bg-white/[0.03] border-b border-lynq-border"
    >
      <span class="font-mono text-[0.78rem] text-lynq-dim">{{ filename }}</span>
      <span
        class="font-mono text-[0.68rem] font-semibold uppercase tracking-[0.06em] text-lynq-purple"
        >{{ lang }}</span
      >
    </div>
    <pre
      class="content m-0 p-4 font-mono text-[0.82rem] leading-[1.55] text-[#e2e8f0] overflow-x-auto"
    ><code><template
      v-for="(seg, i) in segments"
      :key="i"
    ><span
      v-if="seg.hl"
      class="tok"
    >{{ seg.text }}</span><template v-else>{{ seg.text }}</template></template></code></pre>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  filename: { type: String, required: true },
  lang: { type: String, default: 'yaml' },
  code: { type: String, required: true },
  // Substrings to wrap in a highlight span, e.g. ['{{ .uid }}'].
  highlightTokens: { type: Array, default: () => [] },
})

/**
 * Split `code` into segments where any occurrence of a highlight token becomes
 * a `{ text, hl: true }` segment. Purely string-based (no v-html) so it is
 * XSS-safe. Rendering interleaves the segments as text / spans.
 */
const segments = computed(() => {
  const tokens = (props.highlightTokens || []).filter((t) => t && t.length)
  if (!tokens.length) {
    return [{ text: props.code, hl: false }]
  }

  // Escape tokens for a single alternation regex; longest-first so overlapping
  // tokens prefer the longer match.
  const escaped = [...tokens]
    .sort((a, b) => b.length - a.length)
    .map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
  const re = new RegExp(`(${escaped.join('|')})`, 'g')

  const out = []
  let last = 0
  let m
  while ((m = re.exec(props.code)) !== null) {
    if (m.index > last) {
      out.push({ text: props.code.slice(last, m.index), hl: false })
    }
    out.push({ text: m[0], hl: true })
    last = m.index + m[0].length
    // Guard against zero-length matches (shouldn't happen with our tokens).
    if (m.index === re.lastIndex) re.lastIndex++
  }
  if (last < props.code.length) {
    out.push({ text: props.code.slice(last), hl: false })
  }
  return out
})
</script>

<style scoped>
/* Container/header/content styling moved to Tailwind utilities in the template.
   Kept here: pre-formatting on the nested <code> and the highlight-token color. */
.content code {
  white-space: pre;
}

.tok {
  color: var(--lynq-accent);
  background: rgba(79, 209, 203, 0.12);
  border-radius: 3px;
  padding: 0 2px;
}
</style>
