<template>
  <div class="landing-page">
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
      <!-- Static, faint backdrop of supported resource kinds (decorative) -->
      <div class="resource-grid" aria-hidden="true">
        <code
          v-for="kind in resourceKinds"
          :key="kind"
          class="rounded-lynq-sm border border-lynq-border px-3 py-[0.35rem] font-mono text-[0.8rem] whitespace-nowrap text-lynq-accent"
        >{{ kind }}</code>
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

// Supported resource kinds rendered as a faint, static backdrop behind the CTA.
const resourceKinds = [
  'ServiceAccount', 'Deployment', 'StatefulSet', 'DaemonSet',
  'Service', 'Ingress', 'ConfigMap', 'Secret',
  'PersistentVolumeClaim', 'Job', 'CronJob', 'PodDisruptionBudget',
  'NetworkPolicy', 'HorizontalPodAutoscaler', 'Namespace', 'Manifest',
]

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
  background: var(--lynq-bg);
  color: #fff;
  overflow-x: hidden;
  position: relative;
}

/* Static, decorative CTA backdrop: faint monospace grid of resource kinds.
   Kept in scoped CSS for the radial mask + absolute fill (complex rules). */
.resource-grid {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  display: flex;
  flex-wrap: wrap;
  align-content: center;
  justify-content: center;
  gap: 0.75rem 1rem;
  padding: 3rem 2rem;
  opacity: 0.12;
  overflow: hidden;
  -webkit-mask-image: radial-gradient(ellipse 70% 60% at center, transparent 30%, #000 85%);
  mask-image: radial-gradient(ellipse 70% 60% at center, transparent 30%, #000 85%);
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

  .resource-grid {
    display: none;
  }
}
</style>
