<template>
  <div class="landing-page">
    <!-- Fixed ambient backdrop: brand radial glows + a faint, masked dot grid.
         Sits behind every section (which are made transparent below) so the page
         reads with depth and brand presence instead of a flat black void. -->
    <div class="page-bg" aria-hidden="true">
      <div class="pb-glow"></div>
      <div class="pb-grid"></div>
    </div>

    <!-- Hero centerpiece: DB row -> K8s resources transform -->
    <HeroDemo />

    <!-- Capabilities Strip -->
    <CapabilitiesStrip />

    <!-- Feature grid: animated capability cards (windflow-style) -->
    <FeatureGrid />

    <!-- Reconcile Walkthrough: a row becoming a running app, step by step -->
    <ReconcileWalkthrough />

    <!-- Live Transform: from data to resources -->
    <LiveTransform />

    <!-- Before / After: the old way vs. Lynq -->
    <BeforeAfter />

    <!-- Policies That Protect -->
    <PolicyControls />

    <!-- Reconcile at scale without a thundering herd (topology-based) -->
    <ScaleTopology />

    <!-- Observability: Prometheus metrics + pre-built Grafana dashboard -->
    <Observability />

    <!-- Dashboard Showcase -->
    <DashboardShowcase />

    <!-- Latest Blog Posts -->
    <LatestBlog />

    <!-- Final CTA -->
    <section class="relative overflow-hidden bg-lynq-bg px-8 py-36">
      <!-- Endlessly drifting brand-teal waves behind the CTA (decorative) -->
      <div class="cta-waves" aria-hidden="true">
        <div class="cta-wave cta-wave-1"></div>
        <div class="cta-wave cta-wave-2"></div>
        <div class="cta-wave cta-wave-3"></div>
      </div>
      <!-- Data-to-resource flow backdrop: skeleton "DB rows" fly in from the
           left edge and morph into K8s resource chips as they cross the
           section's center line (decorative). The morph is position-based:
           two perfectly superimposed layers run identical travel animations,
           and complementary soft gradient masks reveal the skeleton only left
           of center and the chip only right of center, so each item visually
           transforms exactly where it crosses the beam. -->
      <div class="resource-flow" aria-hidden="true">
        <div class="flow-beam"></div>
        <div class="flow-layer flow-layer-skel">
          <div
            v-for="item in flowItems"
            :key="'skel-' + item.kind"
            class="flow-item"
            :style="item.style"
          >
            <span class="flow-skel"><i></i><i></i><i></i><i></i></span>
          </div>
        </div>
        <div class="flow-layer flow-layer-chip">
          <div
            v-for="item in flowItems"
            :key="'chip-' + item.kind"
            class="flow-item"
            :style="item.style"
          >
            <code class="flow-chip">
              <span class="chip-badge">{{ item.abbr }}</span>
              <span class="chip-name">{{ item.kind }}</span>
            </code>
          </div>
        </div>
      </div>
      <div class="relative z-10 mx-auto max-w-[800px]">
        <div class="cta-content fade-up relative z-10 text-center">
          <h2 class="m-0 mb-6 text-lynq-text">Start Automating Infrastructure from Your Database</h2>
          <p class="m-0 mb-10 text-[1.25rem] leading-[1.6] text-lynq-dim">Requires Kubernetes and cert-manager. The quickstart provisions a full local environment — MySQL, Lynq, and sample resources — using automated setup scripts.</p>

          <div class="cta-buttons flex flex-wrap justify-center gap-4">
            <a
              href="/quickstart"
              class="cta-primary group inline-flex items-center justify-center gap-2 rounded-full bg-lynq-text! px-7 py-[0.8rem] text-[0.98rem] font-medium text-[#0a0a0a]! no-underline transition-opacity duration-200 hover:opacity-90"
            >
              Start Quickstart
              <span class="arrow inline-block transition-transform duration-200 group-hover:translate-x-[3px]">&#8594;</span>
            </a>
            <a
              href="https://github.com/k8s-lynq/lynq"
              class="cta-github inline-flex items-center justify-center gap-2 rounded-full border border-lynq-border bg-transparent px-7 py-[0.8rem] text-[0.98rem] font-medium text-lynq-text! no-underline transition-colors duration-200 hover:border-white/20 hover:bg-white/[0.06]"
              target="_blank"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
              </svg>
              View on GitHub
            </a>
          </div>
        </div>
      </div>
    </section>

  </div>
</template>

<script setup>
import { defineAsyncComponent, h } from 'vue'

// Supported resource kinds, animated as a data→resource flow behind the CTA.
// abbr = kubectl-style short name shown as a badge; color groups by category
// (workloads blue, networking teal, config amber, storage purple, policy green,
// identity/cluster pink, raw manifests gray).
const resourceKinds = [
  { kind: 'ServiceAccount', abbr: 'sa', color: '#f472b6' },
  { kind: 'Deployment', abbr: 'deploy', color: '#60a5fa' },
  { kind: 'StatefulSet', abbr: 'sts', color: '#60a5fa' },
  { kind: 'DaemonSet', abbr: 'ds', color: '#60a5fa' },
  { kind: 'Service', abbr: 'svc', color: '#4fd1cb' },
  { kind: 'Ingress', abbr: 'ing', color: '#4fd1cb' },
  { kind: 'ConfigMap', abbr: 'cm', color: '#fbbf24' },
  { kind: 'Secret', abbr: 'sec', color: '#fbbf24' },
  { kind: 'PersistentVolumeClaim', abbr: 'pvc', color: '#a78bfa' },
  { kind: 'Job', abbr: 'job', color: '#60a5fa' },
  { kind: 'CronJob', abbr: 'cj', color: '#60a5fa' },
  { kind: 'PodDisruptionBudget', abbr: 'pdb', color: '#34d399' },
  { kind: 'NetworkPolicy', abbr: 'netpol', color: '#4fd1cb' },
  { kind: 'HorizontalPodAutoscaler', abbr: 'hpa', color: '#34d399' },
  { kind: 'Namespace', abbr: 'ns', color: '#f472b6' },
  { kind: 'Manifest', abbr: 'raw', color: '#9ca3af' },
]

// 8 lanes hugging the top/bottom bands of the section, clear of the centered
// heading/paragraph/buttons. Two kinds share each lane a half-cycle apart
// (same duration, delays offset by dur/2) so they can never collide. Negative
// delays pre-populate the scene so items are already mid-flight on first paint.
const FLOW_LANES = [6, 13, 20, 27, 85.5, 89, 92.5, 96]
const flowItems = resourceKinds.map((res, i) => {
  const lane = i % FLOW_LANES.length
  const pair = Math.floor(i / FLOW_LANES.length) // 0 or 1
  const dur = 16 + ((lane * 3) % 6)              // 16s … 21s, per lane
  const delay = -(((lane * 4.7) + 2) % dur) - pair * (dur / 2)
  return {
    ...res,
    style: {
      top: `${FLOW_LANES[lane]}%`,
      '--dur': `${dur}s`,
      '--delay': `${delay}s`,
      '--chip-c': res.color,
    },
  }
})

// Invisible sizing skeleton mirroring HeroDemo's above-the-fold footprint to
// prevent layout shift (CLS) while the async chunk loads. delay:0 shows it
// immediately.
const HeroDemoSkeleton = {
  render() {
    return h('div', {
      style: {
        position: 'relative',
        width: '100%',
        height: '100vh',
        minHeight: '700px',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
        background: 'radial-gradient(ellipse at center, rgba(51,172,168,0.1) 0%, transparent 60%), linear-gradient(180deg, #0a0a0f 0%, #111118 100%)',
      },
    }, [
      h('div', {
        style: {
          visibility: 'hidden',
          textAlign: 'center',
          padding: '0 2rem',
          maxWidth: '900px',
        },
      }, [
        h('div', { style: { padding: '0.5rem 1rem', fontSize: '0.875rem', marginBottom: '1.5rem' } }, 'Infrastructure as Data'),
        h('h1', { style: { fontSize: 'clamp(2.5rem, 6vw, 4.5rem)', fontWeight: '700', lineHeight: '1.1', margin: '0 0 1.5rem' } }, [
          'Your Database.',
          h('br'),
          'Your Infrastructure.',
        ]),
        h('p', { style: { fontSize: 'clamp(1rem, 2vw, 1.375rem)', margin: '0 0 2.5rem', lineHeight: '1.6' } },
          'Lynq turns database records into Kubernetes resources. Automatically.'
        ),
        h('div', { style: { display: 'flex', gap: '1rem', justifyContent: 'center', flexWrap: 'wrap' } }, [
          h('span', { style: { padding: '1rem 2rem', fontSize: '1rem', borderRadius: '12px' } }, 'Get Started →'),
          h('span', { style: { padding: '1rem 2rem', fontSize: '1rem', borderRadius: '12px' } }, '◆ View on GitHub'),
        ]),
      ]),
    ])
  }
}

const HeroDemo = defineAsyncComponent({
  loader: () => import('./landing/sections/HeroDemo.vue'),
  loadingComponent: HeroDemoSkeleton,
  delay: 0,
})
import BeforeAfter from './landing/sections/BeforeAfter.vue'
import ReconcileWalkthrough from './landing/sections/ReconcileWalkthrough.vue'
import FeatureGrid from './landing/sections/FeatureGrid.vue'
import LiveTransform from './landing/sections/LiveTransform.vue'
import PolicyControls from './landing/sections/PolicyControls.vue'
// ScaleControl.vue (grid-based) is kept as a backup, unreferenced.
import ScaleTopology from './landing/sections/ScaleTopology.vue'
import Observability from './landing/sections/Observability.vue'
import DashboardShowcase from './landing/DashboardShowcase.vue'
import LatestBlog from './landing/LatestBlog.vue'
import CapabilitiesStrip from './landing/CapabilitiesStrip.vue'
</script>

<style scoped>
/* Root wrapper: keeps overflow-x clipping for the whole page. */
.landing-page {
  width: 100%;
  color: #fff;
  overflow-x: hidden;
  position: relative;
}

/* ── Ambient backdrop ──
   A single fixed layer behind the whole page. Sections (except the hero, which
   owns its own backdrop) are made transparent so these glows + grid show
   through, giving the page depth and a consistent brand wash instead of flat
   black. */
.page-bg {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
}
.pb-glow {
  position: absolute;
  inset: 0;
  transform-origin: 50% 42%;
  background:
    radial-gradient(58rem 48rem at 6% 0%, rgba(51, 172, 168, 0.22), transparent 58%),
    radial-gradient(54rem 50rem at 100% 36%, rgba(59, 130, 246, 0.14), transparent 56%),
    radial-gradient(60rem 50rem at 50% 108%, rgba(51, 172, 168, 0.18), transparent 58%);
}
/* Ambient breathing — a clearly visible swell, still calm. */
@media (prefers-reduced-motion: no-preference) {
  .pb-glow {
    animation: pb-pulse 5s ease-in-out infinite;
  }
}
@keyframes pb-pulse {
  0%, 100% { opacity: 0.5; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.08); }
}
.pb-grid {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(255, 255, 255, 0.09) 1.2px, transparent 1.4px);
  background-size: 34px 34px;
  opacity: 1;
  /* Even texture across the viewport, fading only near the far edges so it
     reads as a consistent surface rather than a band at the top. */
  -webkit-mask-image: radial-gradient(ellipse 96% 96% at 50% 42%, #000 0%, rgba(0, 0, 0, 0.7) 58%, transparent 94%);
  mask-image: radial-gradient(ellipse 96% 96% at 50% 42%, #000 0%, rgba(0, 0, 0, 0.7) 58%, transparent 94%);
}

/* Lift real content above the backdrop, and let the backdrop show through every
   section except the self-contained hero. This also retires the leftover
   off-palette gradient the blog section used. */
.landing-page > *:not(.page-bg) {
  position: relative;
  z-index: 1;
}
.landing-page > section:not(.hero-demo),
.landing-page > div:not(.page-bg) {
  background: transparent !important;
  background-image: none !important;
}

/* ── CTA waves ──
   Three stacked, horizontally tiling wave bands pinned to the bottom of the
   final CTA. Each band scrolls its background-position at a different speed
   (one in reverse) so the layers slide past each other in an endless drift.
   The wave path is periodic (same y + slope at both edges) so repeat-x tiles
   seamlessly. A top-fade mask keeps the section's upper half clean for text. */
.cta-waves {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  overflow: hidden;
  -webkit-mask-image: linear-gradient(to bottom, transparent 18%, #000 62%);
  mask-image: linear-gradient(to bottom, transparent 18%, #000 62%);
}
.cta-wave {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  background-repeat: repeat-x;
  background-position: 0 bottom;
  background-size: 1440px 100%;
}
.cta-wave-1 {
  height: 190px;
  opacity: 0.5;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1440 220' preserveAspectRatio='none'%3E%3Cpath fill='%2333aca8' fill-opacity='0.28' d='M0,110 C240,190 480,30 720,110 C960,190 1200,30 1440,110 L1440,220 L0,220 Z'/%3E%3C/svg%3E");
}
.cta-wave-2 {
  height: 150px;
  opacity: 0.45;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1440 220' preserveAspectRatio='none'%3E%3Cpath fill='%233b82f6' fill-opacity='0.22' d='M0,120 C180,60 360,180 720,120 C1080,60 1260,180 1440,120 L1440,220 L0,220 Z'/%3E%3C/svg%3E");
}
.cta-wave-3 {
  height: 110px;
  opacity: 0.55;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1440 220' preserveAspectRatio='none'%3E%3Cpath fill='%2333aca8' fill-opacity='0.4' d='M0,130 C240,70 480,190 720,130 C960,70 1200,190 1440,130 L1440,220 L0,220 Z'/%3E%3C/svg%3E");
}
@media (prefers-reduced-motion: no-preference) {
  .cta-wave-1 { animation: wave-drift 26s linear infinite; }
  .cta-wave-2 { animation: wave-drift-reverse 19s linear infinite; }
  .cta-wave-3 { animation: wave-drift 13s linear infinite; }
}
@keyframes wave-drift {
  from { background-position-x: 0; }
  to { background-position-x: 1440px; }
}
@keyframes wave-drift-reverse {
  from { background-position-x: 0; }
  to { background-position-x: -1440px; }
}

/* ── Data → resource flow (CTA backdrop) ──
   Two superimposed layers run identical left→right travel animations per item.
   Complementary soft gradient masks reveal the skeleton layer only left of
   center and the chip layer only right of center, so each item crossfades into
   its chip form exactly where it crosses the center beam — a position-based
   morph, independent of animation timing. */
.resource-flow {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  overflow: hidden;
  opacity: 0.55;
  /* Soft fade at the section's left/right edges. */
  -webkit-mask-image: linear-gradient(to right, transparent, #000 7%, #000 93%, transparent);
  mask-image: linear-gradient(to right, transparent, #000 7%, #000 93%, transparent);
}

/* Transformation line, split into two short segments confined to the lane
   bands so no line runs behind the centered heading/paragraph/buttons. */
.flow-beam {
  position: absolute;
  inset: 0;
}
.flow-beam::before,
.flow-beam::after {
  content: '';
  position: absolute;
  left: 50%;
  width: 1px;
  background: linear-gradient(
    to bottom,
    transparent,
    rgba(79, 209, 203, 0.35) 30%,
    rgba(79, 209, 203, 0.35) 70%,
    transparent
  );
}
.flow-beam::before {
  top: 3%;
  height: 28%;
}
.flow-beam::after {
  bottom: 1%;
  height: 17%;
}

.flow-layer {
  position: absolute;
  inset: 0;
}
/* Skeleton form lives left of the beam, chip form right of it; the 10%-wide
   gradient overlap around the center is where the morph reads. */
.flow-layer-skel {
  -webkit-mask-image: linear-gradient(to right, #000 45%, transparent 55%);
  mask-image: linear-gradient(to right, #000 45%, transparent 55%);
}
.flow-layer-chip {
  -webkit-mask-image: linear-gradient(to right, transparent 45%, #000 55%);
  mask-image: linear-gradient(to right, transparent 45%, #000 55%);
}

.flow-item {
  position: absolute;
  left: 0;
  transform: translateX(-280px);
}
@media (prefers-reduced-motion: no-preference) {
  .flow-item {
    animation: flow-travel var(--dur) linear var(--delay) infinite;
    will-change: transform;
  }
}
@keyframes flow-travel {
  from { transform: translateX(-280px); }
  to { transform: translateX(calc(100vw + 60px)); }
}

/* Skeleton form: a horizontally long "DB row" of shimmering cells. Sized to
   the exact height/radius of the chip form so the center-line morph swaps one
   silhouette for the other instead of jumping in size. */
.flow-skel {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  box-sizing: border-box;
  width: 230px;
  height: 34px;
  padding: 0 0.6rem;
  border-radius: 9px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.04);
}
.flow-skel i {
  display: block;
  height: 10px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.14);
  overflow: hidden;
  position: relative;
}
.flow-skel i:nth-child(1) {
  width: 18%;
  background: rgba(79, 209, 203, 0.28);
}
.flow-skel i:nth-child(2) { width: 34%; }
.flow-skel i:nth-child(3) { width: 22%; }
.flow-skel i:nth-child(4) { width: 26%; }
/* Shimmer sweep across each cell. */
@media (prefers-reduced-motion: no-preference) {
  .flow-skel i::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(100deg, transparent 20%, rgba(255, 255, 255, 0.25) 50%, transparent 80%);
    transform: translateX(-100%);
    animation: skel-shimmer 1.6s linear infinite;
  }
}
@keyframes skel-shimmer {
  to { transform: translateX(100%); }
}

/* Chip form: a small resource "card" — kubectl short-name badge tinted by the
   resource's category color, kind name in mono, soft gradient surface. */
.flow-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  box-sizing: border-box;
  height: 34px;
  padding: 0 0.7rem 0 0.38rem;
  border-radius: 9px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(180deg, rgba(28, 34, 40, 0.92), rgba(12, 16, 20, 0.92));
  box-shadow:
    0 4px 16px rgba(0, 0, 0, 0.4),
    0 0 14px color-mix(in srgb, var(--chip-c) 12%, transparent);
  white-space: nowrap;
}
.chip-badge {
  display: inline-block;
  padding: 0.14rem 0.42rem;
  border-radius: 6px;
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--chip-c);
  background: color-mix(in srgb, var(--chip-c) 16%, transparent);
}
.chip-name {
  font-family: var(--lynq-mono);
  font-size: 0.78rem;
  color: rgba(237, 237, 237, 0.85);
}

/* Entrance animation for the CTA content. */
@keyframes fadeUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.fade-up {
  animation: fadeUp 0.6s ease both;
}

@media (max-width: 768px) {
  .cta-buttons {
    flex-direction: column;
    align-items: center;
  }

  .cta-buttons .cta-primary,
  .cta-buttons .cta-github {
    width: 100%;
    max-width: 280px;
  }

  .resource-flow {
    display: none;
  }
}
</style>
