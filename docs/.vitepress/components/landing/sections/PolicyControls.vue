<template>
  <section ref="rootRef" class="policy-controls scroll-mt-20 px-8">
    <div class="mx-auto" style="max-width: 1040px">
      <SectionHeader
        label="Safety & Control"
        title="Policies That Protect Your Cluster"
        subtitle="Every resource in a LynqForm carries three independent lifecycle policies. Fine-grained, per-resource control over what happens on conflict, on deactivation, and on re-reconcile — so automation never overwrites what it shouldn't."
        accent="purple"
      />

      <div class="cards grid grid-cols-3 gap-6 items-stretch">
        <div
          v-for="(card, ci) in cards"
          :key="card.field"
          class="policy-card flex flex-col gap-4 p-6 bg-lynq-card border border-lynq-border rounded-lynq"
          :class="{ 'is-pinned': cardState[ci].pinned }"
          @mouseenter="onEnter(ci)"
          @mouseleave="onLeave(ci)"
          @focusin="onEnter(ci)"
          @focusout="onLeave(ci)"
        >
          <div class="card-head flex items-baseline justify-between gap-3">
            <span class="field-name font-mono text-[0.9rem] font-bold text-lynq-text">{{ card.field }}</span>
            <span class="current-value font-mono text-[0.78rem] font-semibold px-[0.55rem] py-[0.15rem] rounded-full" :data-value="valueOf(ci)">{{ card.values[valueOf(ci)] }}</span>
          </div>

          <!-- Segmented toggle: dim until the card is pinned (hover/focus) or
               reduced-motion is active, at which point it's the primary control. -->
          <div class="segmented flex gap-1 p-1 w-full rounded-full bg-lynq-bg2 border border-lynq-border" :class="{ active: cardState[ci].pinned || reduced }" role="group">
            <button
              v-for="(label, i) in card.values"
              :key="label"
              type="button"
              class="seg-btn"
              :class="{ on: valueOf(ci) === i }"
              :aria-pressed="valueOf(ci) === i"
              @click="select(ci, i)"
            >
              {{ label }}
            </button>
          </div>

          <div class="card-demo h-[9.5rem]">
            <!-- conflictPolicy -->
            <div v-if="card.field === 'conflictPolicy'" class="demo-body flex flex-col gap-3 h-full">
              <ResourceCard
                v-if="valueOf(ci) === 0"
                kind="Service"
                name="acme-corp-svc"
                status="conflicted"
                meta="field-manager: helm"
              />
              <ResourceCard
                v-else
                kind="Service"
                name="acme-corp-svc"
                status="ready"
                meta="field-manager: lynq"
              />
              <p class="demo-caption m-0 min-h-[2.4em] text-[0.82rem] leading-[1.45] text-lynq-dim">
                {{ valueOf(ci) === 0
                  ? 'Halted — ownership conflict surfaced, nothing overwritten.'
                  : 'Took ownership via Server-Side Apply force=true.' }}
              </p>
            </div>

            <!-- deletionPolicy -->
            <div v-else-if="card.field === 'deletionPolicy'" class="demo-body flex flex-col gap-3 h-full">
              <div class="stack flex flex-col gap-2">
                <div class="removed-card flex items-center gap-[0.6rem] px-[0.85rem] py-[0.55rem] rounded-lynq-sm">
                  <span class="removed-kind">Deployment</span>
                  <span class="removed-name">beta-inc-app</span>
                  <span class="removed-tag">removed</span>
                </div>
                <div v-if="valueOf(ci) === 0" class="removed-card flex items-center gap-[0.6rem] px-[0.85rem] py-[0.55rem] rounded-lynq-sm">
                  <span class="removed-kind">PVC</span>
                  <span class="removed-name">beta-inc-data</span>
                  <span class="removed-tag">removed</span>
                </div>
                <ResourceCard
                  v-else
                  kind="PVC"
                  name="beta-inc-data"
                  status="retained"
                  meta="lynq.sh/orphaned=true"
                />
              </div>
              <p class="demo-caption m-0 min-h-[2.4em] text-[0.82rem] leading-[1.45] text-lynq-dim">
                {{ valueOf(ci) === 0
                  ? 'Row deactivated — both resources removed from the cluster.'
                  : 'Deployment removed, but the PVC is kept — ownerRef dropped, orphan marker added.' }}
              </p>
            </div>

            <!-- creationPolicy -->
            <div v-else class="demo-body flex flex-col gap-3 h-full">
              <ResourceCard
                v-if="valueOf(ci) === 0"
                kind="Secret"
                name="acme-corp-tls"
                status="ready"
                meta="reconciled every pass"
              />
              <ResourceCard
                v-else
                kind="Secret"
                name="acme-corp-tls"
                status="skipped"
                meta="lynq.sh/created-once"
              />
              <p class="demo-caption m-0 min-h-[2.4em] text-[0.82rem] leading-[1.45] text-lynq-dim">
                {{ valueOf(ci) === 0
                  ? 'Reconciled continuously — drift corrected on every pass.'
                  : 'Created once — subsequent reconciles skip it entirely.' }}
              </p>
            </div>
          </div>

          <YamlBlock
            filename="lynqform.yaml"
            :code="card.snippet(valueOf(ci))"
            :highlight-tokens="card.highlight(valueOf(ci))"
          />
        </div>
      </div>

      <div class="learn-more mt-10 text-center">
        <a href="/policies">Learn more about policies <span aria-hidden="true">&#8594;</span></a>
      </div>
    </div>
  </section>
</template>

<script setup>
/**
 * PolicyControls — three side-by-side cards, one per real per-resource policy
 * (conflictPolicy / deletionPolicy / creationPolicy). Each card is a mini scene
 * that auto-toggles between the policy's two values on a slow loop while the
 * section is in view; hovering (or focusing) pins the card, pausing auto and
 * revealing a segmented toggle for manual flipping. Leaving resumes auto.
 *
 * Under reduced motion there is no auto-toggle: the safe default value
 * (index 0 — Stuck / Delete / WhenNeeded) is shown statically and the manual
 * toggle stays usable.
 *
 * Each card owns its own 2-beat looping timeline. To keep the whole section in
 * a single .vue file (so scoped styles apply to all markup), the card markup is
 * authored in this template and per-card state is held in parallel arrays
 * indexed by card position — no child component is imported.
 */
import { ref, reactive, computed, watch } from 'vue'
import SectionHeader from '../primitives/SectionHeader.vue'
import ResourceCard from '../primitives/ResourceCard.vue'
import YamlBlock from '../primitives/YamlBlock.vue'
import { useInView } from '../composables/useInView.js'
import { useStepTimeline } from '../composables/useStepTimeline.js'
import { useReducedMotion } from '../composables/useReducedMotion.js'

const rootRef = ref(null)
const { inView } = useInView(rootRef, { threshold: 0.2, once: true })
const reduced = useReducedMotion()

// values[0] is always the safe/default value (shown under reduced motion).
const cards = [
  {
    field: 'conflictPolicy',
    values: ['Stuck', 'Force'],
    snippet: (i) =>
      i === 0
        ? 'deletionPolicy: Delete\nconflictPolicy: Stuck'
        : 'deletionPolicy: Delete\nconflictPolicy: Force',
    highlight: (i) => (i === 0 ? ['Stuck'] : ['Force']),
  },
  {
    field: 'deletionPolicy',
    values: ['Delete', 'Retain'],
    snippet: (i) =>
      i === 0
        ? 'nameTemplate: "{{ .uid }}-data"\ndeletionPolicy: Delete'
        : 'nameTemplate: "{{ .uid }}-data"\ndeletionPolicy: Retain',
    highlight: (i) => (i === 0 ? ['Delete'] : ['Retain']),
  },
  {
    field: 'creationPolicy',
    values: ['WhenNeeded', 'Once'],
    snippet: (i) =>
      i === 0
        ? 'nameTemplate: "{{ .uid }}-tls"\ncreationPolicy: WhenNeeded'
        : 'nameTemplate: "{{ .uid }}-tls"\ncreationPolicy: Once',
    highlight: (i) => (i === 0 ? ['WhenNeeded'] : ['Once']),
  },
]

// One independent looping timeline per card. Each is a 2-beat loop that
// auto-toggles value 0 <-> 1 while in view.
const timelines = cards.map(() =>
  useStepTimeline({
    steps: 2,
    durations: 2600,
    loop: true,
    autoStart: false,
    respectReducedMotion: true,
  })
)

// Per-card interaction state.
const cardState = reactive(
  cards.map(() => ({ pinned: false, manualValue: null }))
)

// The value actually rendered for card ci:
//  - pinned with a manual selection -> that selection
//  - reduced motion -> safe default (0)
//  - otherwise -> the card's timeline step
function valueOf(ci) {
  const st = cardState[ci]
  if (st.pinned && st.manualValue !== null) return st.manualValue
  if (reduced.value) return 0
  return timelines[ci].step.value
}

// Kick off every card's auto-toggle once the section scrolls into view (unless
// reduced motion or the card is already pinned).
watch(
  inView,
  (v) => {
    if (!v) return
    timelines.forEach((t, ci) => {
      if (!reduced.value && !cardState[ci].pinned) t.play()
    })
  },
  { immediate: true }
)

function onEnter(ci) {
  const st = cardState[ci]
  st.pinned = true
  // Pin at the currently displayed value so there's no visual jump.
  st.manualValue = valueOf(ci)
  timelines[ci].pause()
}

function onLeave(ci) {
  const st = cardState[ci]
  st.pinned = false
  st.manualValue = null
  if (inView.value && !reduced.value) timelines[ci].play()
}

function select(ci, i) {
  const st = cardState[ci]
  st.manualValue = i
  // seek() pauses the timeline (scrub semantics) and keeps the step in sync.
  timelines[ci].seek(i)
}
</script>

<style scoped>
.policy-controls {
  padding-block: var(--lynq-section-y);
  background: var(--lynq-bg);
}

/* Align the SectionHeader h2 to the v2 heading tokens (lighter, a step down).
   The primitive is frozen, so override it locally via a deep selector. */
.policy-controls :deep(.lynq-section-header h2) {
  font-size: var(--lynq-h2);
  font-weight: var(--lynq-heading-weight);
}

/* Colored link: the global .landing-page reset forces `color: inherit`, so keep
   this in scoped CSS where it already wins for the accent link. */
.learn-more a {
  color: var(--lynq-accent);
  font-weight: 600;
  font-size: 0.95rem;
  text-decoration: none;
  transition: color 0.2s var(--lynq-ease);
}

.learn-more a:hover {
  color: var(--lynq-text);
}

/* ---- PolicyCard ----
   Layout on the template (equal height via grid stretch). The pin transition +
   pinned lift/glow (JS-toggled .is-pinned) stay here. */
.policy-card {
  transition: border-color 0.25s var(--lynq-ease), transform 0.25s var(--lynq-ease),
    box-shadow 0.25s var(--lynq-ease);
}

.policy-card.is-pinned {
  border-color: var(--lynq-accent);
  transform: translateY(-3px);
  box-shadow: 0 10px 30px -12px rgba(0, 0, 0, 0.55);
}

/* Pin the code snippet to the bottom edge of the card (it is the last child). */
.policy-card > :last-child {
  margin-top: auto;
}

/* Base pill tint; the two value states below override it (values are always
   0 or 1, so this is the fallback). */
.current-value {
  background: rgba(255, 255, 255, 0.06);
  color: var(--lynq-text-dim);
}

/* Default value (index 0) reads as the "safe" state -> green tint. The alternate
   (index 1) uses the purple accent tint — amber stays reserved for genuine
   Pending/Conflicted resource status, not for policy-value labels. */
.current-value[data-value='0'] {
  color: var(--lynq-green);
  background: rgba(16, 185, 129, 0.14);
}
.current-value[data-value='1'] {
  color: var(--lynq-accent);
  background: color-mix(in srgb, var(--lynq-accent) 16%, transparent);
}

/* Segmented control base layout is on the template; the dim→bright pin state
   transition (JS-toggled .active) stays here. */
.segmented {
  opacity: 0.85;
  transition: opacity 0.2s var(--lynq-ease), border-color 0.2s var(--lynq-ease);
}

.segmented.active {
  opacity: 1;
  border-color: var(--lynq-accent);
}

.seg-btn {
  appearance: none;
  border: none;
  background: transparent;
  cursor: pointer;
  flex: 1;
  text-align: center;
  font-family: var(--lynq-mono);
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--lynq-text-dim);
  padding: 0.4rem 0.7rem;
  border-radius: 999px;
  transition: background 0.2s var(--lynq-ease), color 0.2s var(--lynq-ease);
}

.seg-btn.on {
  background: var(--lynq-accent);
  color: #fff;
}

.seg-btn:not(.on):hover {
  color: var(--lynq-text);
  background: rgba(255, 255, 255, 0.04);
}

.seg-btn:focus-visible {
  outline: 2px solid var(--lynq-accent);
  outline-offset: 2px;
}

/* card-demo fixed height (h-[9.5rem]) is on the template — reserves the row so
   flipping a policy value never changes the card height (no reflow / CLS). */

/* A "removed" resource: struck-through, faded, red-tinted. Layout on template;
   the red tint / dashed border / fade stay here. */
.removed-card {
  background: rgba(239, 68, 68, 0.06);
  border: 1px dashed rgba(239, 68, 68, 0.35);
  opacity: 0.65;
}

.removed-kind {
  font-family: var(--lynq-mono);
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--lynq-red);
}

.removed-name {
  font-family: var(--lynq-mono);
  font-size: 0.9rem;
  color: var(--lynq-text-dim);
  text-decoration: line-through;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.removed-tag {
  font-family: var(--lynq-mono);
  font-size: 0.68rem;
  color: var(--lynq-red);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

@media (max-width: 900px) {
  .cards {
    grid-template-columns: 1fr;
    max-width: 30rem;
    margin: 0 auto;
  }
}
</style>
