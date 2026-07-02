<template>
  <section ref="rootRef" class="policy-controls scroll-mt-20 px-8" :class="{ in: inView }">
    <div class="mx-auto" style="max-width: 1040px">
      <SectionHeader
        label="Safety & Control"
        title="Policies That Protect Your Cluster"
        subtitle="Data-driven automation is only safe if it knows what not to touch. Every resource in a LynqForm carries three per-resource policies — each a guardrail against a specific way automation could do damage."
        accent="purple"
      />

      <div class="cards grid grid-cols-3 gap-6 items-stretch">
        <div
          v-for="card in cards"
          :key="card.field"
          class="policy-card flex flex-col gap-4 p-6 bg-lynq-card border border-lynq-border rounded-lynq"
        >
          <div class="pc-field flex items-center gap-2">
            <svg class="pc-shield" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M12 3l7 3v5.5c0 4.2-2.9 7.1-7 8.5-4.1-1.4-7-4.3-7-8.5V6l7-3z"
                stroke="currentColor"
                stroke-width="1.6"
                stroke-linejoin="round"
              />
            </svg>
            <span class="font-mono text-[0.92rem] font-bold text-lynq-text">{{ card.field }}</span>
          </div>

          <div class="pc-threat">
            <span class="warn" aria-hidden="true">&#9888;</span>
            <span>{{ card.threat }}</span>
          </div>

          <div class="pc-opts flex flex-col gap-3">
            <div
              v-for="opt in card.options"
              :key="opt.value"
              class="pc-opt"
              :class="{ protective: opt.protective }"
            >
              <div class="pc-opt-top flex items-center justify-between gap-2">
                <span class="pc-val">{{ opt.value }}</span>
                <span class="pc-chip">
                  <span v-if="opt.protective" class="tick" aria-hidden="true">&#10003;</span>{{ opt.chip }}
                </span>
              </div>
              <p class="pc-desc m-0">{{ opt.desc }}</p>
            </div>
          </div>
        </div>
      </div>

      <div class="learn-more mt-10 text-center">
        <a
          class="inline-flex items-center gap-2 rounded-full border border-lynq-border bg-transparent px-6 py-[0.72rem] text-[0.95rem] font-medium! text-lynq-text! no-underline transition-colors duration-200 hover:border-white/20 hover:bg-white/[0.06]"
          href="/policies"
        >
          Learn more about policies
          <span aria-hidden="true">&#8594;</span>
        </a>
      </div>
    </div>
  </section>
</template>

<script setup>
/**
 * PolicyControls — three cards, one per real per-resource policy
 * (conflictPolicy / deletionPolicy / creationPolicy). Each card is framed as a
 * guardrail: a one-line THREAT (the specific way automation could do damage)
 * followed by the policy's two values, with the protective choice marked by a
 * green ✓ chip. No toggles, no YAML, no redundant value pills — the whole point
 * is that each policy value is legible at a glance.
 *
 * Motion is a single scroll-reveal: the cards stagger in and each protective
 * chip "engages" with a small pop. Under reduced motion everything is static
 * (guarded by prefers-reduced-motion in the stylesheet).
 */
import { ref } from 'vue'
import SectionHeader from '../primitives/SectionHeader.vue'
import { useInView } from '../composables/useInView.js'

const rootRef = ref(null)
const { inView } = useInView(rootRef, { threshold: 0.2, once: true })

// Default value is listed first (matches kubectl defaults); the protective
// choice — the one that guards against this card's threat — carries the ✓ chip.
const cards = [
  {
    field: 'conflictPolicy',
    threat: 'Another controller already owns this Service.',
    options: [
      {
        value: 'Stuck',
        desc: 'Halts and surfaces the conflict — overwrites nothing.',
        chip: 'safe default',
        protective: true,
      },
      {
        value: 'Force',
        desc: 'Takes ownership deliberately via SSA force=true.',
        chip: 'opt-in',
        protective: false,
      },
    ],
  },
  {
    field: 'deletionPolicy',
    threat: 'A row is deactivated — is its data wiped with it?',
    options: [
      {
        value: 'Delete',
        desc: 'Removes the resource along with the row.',
        chip: 'default',
        protective: false,
      },
      {
        value: 'Retain',
        desc: 'Keeps the PVC — drops the ownerRef, adds an orphan marker.',
        chip: 'keeps data',
        protective: true,
      },
    ],
  },
  {
    field: 'creationPolicy',
    threat: 'Every reconcile could re-run a one-time init Job.',
    options: [
      {
        value: 'WhenNeeded',
        desc: 'Re-applies whenever the rendered spec drifts.',
        chip: 'default',
        protective: false,
      },
      {
        value: 'Once',
        desc: 'Creates once, then never touches it again.',
        chip: 'runs once',
        protective: true,
      },
    ],
  },
]
</script>

<style scoped>
.policy-controls {
  padding-block: var(--lynq-section-y);
  background: var(--lynq-bg);
}

.policy-controls :deep(.lynq-section-header h2) {
  font-size: var(--lynq-h2);
  font-weight: var(--lynq-heading-weight);
}

/* ── Card ── */
.pc-shield {
  width: 18px;
  height: 18px;
  color: var(--lynq-accent);
  flex: none;
}

/* Threat callout — the damage this policy guards against (amber). */
.pc-threat {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  min-height: 3.4em;
  padding: 0.65rem 0.75rem;
  border-radius: var(--lynq-radius-sm);
  background: rgba(245, 158, 11, 0.07);
  border: 1px solid rgba(245, 158, 11, 0.18);
  font-size: 0.85rem;
  line-height: 1.4;
  color: var(--lynq-text-dim);
}
.pc-threat .warn {
  color: var(--lynq-amber);
  flex: none;
  font-size: 0.95rem;
  line-height: 1.4;
}

/* Two option blocks; the protective one is green-outlined and tinted. */
.pc-opt {
  padding: 0.7rem 0.8rem;
  border-radius: var(--lynq-radius-sm);
  border: 1px solid var(--lynq-border);
}
.pc-opt.protective {
  border-color: rgba(16, 185, 129, 0.32);
  background: rgba(16, 185, 129, 0.05);
}

.pc-val {
  font-family: var(--lynq-mono);
  font-size: 0.9rem;
  font-weight: 700;
  color: var(--lynq-text-dim);
}
.pc-opt.protective .pc-val {
  color: var(--lynq-text);
}

.pc-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-family: var(--lynq-mono);
  font-size: 0.66rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  white-space: nowrap;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  color: var(--lynq-text-faint);
  background: rgba(255, 255, 255, 0.05);
}
.pc-opt.protective .pc-chip {
  color: var(--lynq-green);
  background: rgba(16, 185, 129, 0.14);
}
.pc-chip .tick {
  font-size: 0.72rem;
}

.pc-desc {
  margin-top: 0.4rem;
  min-height: 2.5em;
  font-size: 0.82rem;
  line-height: 1.45;
  color: var(--lynq-text-dim);
}

/* ── Motion: scroll-reveal stagger + guardrail chip "engage" ──
   All gated on no-preference so reduced-motion renders the settled layout. */
@media (prefers-reduced-motion: no-preference) {
  .policy-card {
    opacity: 0;
    transform: translateY(16px);
    transition: opacity 0.6s var(--lynq-ease), transform 0.6s var(--lynq-ease);
  }
  .policy-controls.in .policy-card {
    opacity: 1;
    transform: none;
  }
  .policy-controls.in .policy-card:nth-child(2) {
    transition-delay: 0.1s;
  }
  .policy-controls.in .policy-card:nth-child(3) {
    transition-delay: 0.2s;
  }

  .pc-opt.protective .pc-chip {
    opacity: 0;
  }
  .policy-controls.in .pc-opt.protective .pc-chip {
    animation: pc-chip-engage 0.5s var(--lynq-ease) 0.55s both;
  }
  .policy-controls.in .policy-card:nth-child(2) .pc-opt.protective .pc-chip {
    animation-delay: 0.65s;
  }
  .policy-controls.in .policy-card:nth-child(3) .pc-opt.protective .pc-chip {
    animation-delay: 0.75s;
  }
}

@keyframes pc-chip-engage {
  from {
    opacity: 0;
    transform: scale(0.82);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

@media (max-width: 900px) {
  .cards {
    grid-template-columns: 1fr;
    max-width: 30rem;
    margin: 0 auto;
  }
}
</style>
