<template>
  <section ref="rootRef" class="dashboard-section scroll-mt-20 px-8">
    <div class="mx-auto" style="max-width: 1040px">
      <div class="section-header reveal text-center mb-12" :class="{ 'in-view': inView }">
        <span class="section-label inline-block text-[0.875rem] font-semibold uppercase tracking-[0.1em] text-lynq-accent mb-3">Dashboard</span>
        <h2>Observe the Full Hub → Form → Node Graph</h2>
        <p class="section-subtitle text-[1.1rem] text-lynq-dim m-0">A web UI that shows live resource health, reconciliation events, and topology relationships — no kubectl required</p>
      </div>

      <div class="browser-mockup reveal-scale bg-lynq-card overflow-hidden mb-12" :class="{ 'in-view': inView }" style="transition-delay: 0.15s">
        <!-- Browser Chrome -->
        <div class="browser-chrome flex items-center gap-4 px-4 py-3">
          <div class="browser-buttons flex gap-2">
            <span class="btn-close"></span>
            <span class="btn-minimize"></span>
            <span class="btn-maximize"></span>
          </div>
          <div class="browser-address flex-1 flex items-center gap-2 px-4 py-2 rounded-md max-w-[400px]">
            <LineIcon name="lock" class="lock-icon" />
            <span class="url text-[0.8rem] font-mono">localhost:8080/topology</span>
          </div>
          <div class="browser-actions w-20"></div>
        </div>

        <!-- Dashboard Content: live topology graph (client-only, SSR-safe) -->
        <div class="browser-content">
          <ClientOnly>
            <TopologyGraph :data="sampleTopology" />
          </ClientOnly>
        </div>
      </div>

      <!-- Feature list -->
      <div class="features-row grid gap-6 mb-12">
        <div
          v-for="(feature, index) in features"
          :key="feature.title"
          class="feature-item reveal flex items-start gap-4 p-4 border border-lynq-border rounded-xl"
          :class="{ 'in-view': inView }"
          :style="{ transitionDelay: `${0.25 + 0.08 * index}s` }"
        >
          <span class="feature-icon inline-flex text-[1.4rem] text-lynq-accent leading-none"><LineIcon :name="feature.icon" /></span>
          <div class="feature-text flex flex-col gap-1">
            <strong class="text-[0.95rem] text-lynq-text">{{ feature.title }}</strong>
            <span class="text-[0.85rem] text-lynq-faint">{{ feature.description }}</span>
          </div>
        </div>
      </div>

      <div class="dashboard-cta reveal flex justify-center items-center gap-8 flex-wrap" :class="{ 'in-view': inView }" style="transition-delay: 0.3s">
        <a href="/dashboard" class="cta-button">
          Install Dashboard
          <span class="arrow">&#8594;</span>
        </a>
        <a href="https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart" class="try-demo" target="_blank">
          Try interactive demo
          <span class="external">&#8599;</span>
        </a>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref } from 'vue'
import { useInView } from './composables/useInView.js'
import LineIcon from './primitives/LineIcon.vue'
import TopologyGraph from './topology/TopologyGraph.vue'
import { sampleTopology } from './data/sampleTopology.js'

const rootRef = ref(null)
const { inView } = useInView(rootRef, { threshold: 0.2, once: true })

const features = [
  {
    icon: 'topology',
    title: 'Topology View',
    description: 'Hub → Form → Node hierarchy with live status'
  },
  {
    icon: 'health',
    title: 'Resource Health',
    description: 'Ready, pending, failed counts per node'
  },
  {
    icon: 'events',
    title: 'Event Stream',
    description: 'Reconciliation events and error details per node'
  },
  {
    icon: 'search',
    title: 'Quick Search',
    description: '⌘K to find any hub, form, or node instantly'
  }
]
</script>

<style scoped>
.dashboard-section {
  padding-block: var(--lynq-section-y);
  background: linear-gradient(180deg, var(--lynq-bg) 0%, var(--lynq-bg-2) 100%);
}

.section-header h2 {
  /* Literal values: the --lynq-h2/--lynq-heading-weight custom props do not
     resolve in scoped component CSS on this page. Matched to windflow H2. */
  font-size: clamp(2rem, 5vw, 3.75rem);
  font-weight: 500;
  color: #ededed;
  letter-spacing: -0.025em;
  margin: 0 0 1rem;
  line-height: 1.15;
}

/* Browser mockup: layout on the template; the 12px chrome radius + depth shadow
   stay here. */
.browser-mockup {
  border-radius: 12px;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.05);
}

.browser-chrome {
  background: #252538;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.browser-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close {
  background: #ff5f57;
}

.btn-minimize {
  background: #febc2e;
}

.btn-maximize {
  background: #28c840;
}

.browser-address {
  background: rgba(0, 0, 0, 0.3);
}

.lock-icon {
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.5);
}

.url {
  color: rgba(255, 255, 255, 0.7);
}

.browser-content {
  position: relative;
  overflow: hidden;
  /* Dark canvas to match the rest of the page. A subtle radial lift toward the
     center gives the graph depth without breaking the black theme. Fixed height
     prevents layout shift (no CLS) and keeps the graph from causing horizontal
     scroll — the graph fills it and auto-fits via zoom. */
  height: 520px;
  background:
    radial-gradient(circle at 50% 45%, #141419 0%, #0b0b0e 58%, #050505 100%);
}

@media (max-width: 768px) {
  .browser-content {
    height: 380px;
  }
}

/* auto-fit responsive columns: no clean Tailwind arbitrary, keep the grid
   template (and its narrow-screen overrides) here; gap/margin are on template. */
.features-row {
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
}

/* Subtle card tint — layout/border/radius on template. */
.feature-item {
  background: rgba(255, 255, 255, 0.02);
}

.dashboard-cta .cta-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.85rem 1.9rem;
  background: linear-gradient(135deg, var(--lynq-purple) 0%, var(--lynq-accent) 100%);
  /* weight/color are overridden by the .VPHome .landing-page a !important reset,
     so restate them !important to keep the pill on-brand (windflow pills are 500). */
  color: #04100f !important;
  font-weight: 500 !important;
  font-size: 0.95rem;
  border-radius: 9999px;
  text-decoration: none;
  transition: transform 0.3s var(--lynq-ease), box-shadow 0.3s var(--lynq-ease);
  box-shadow: 0 4px 15px rgba(51, 172, 168, 0.35);
}

.cta-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 25px rgba(51, 172, 168, 0.45);
}

.cta-button .arrow {
  transition: transform 0.3s ease;
}

.cta-button:hover .arrow {
  transform: translateX(4px);
}

.try-demo {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--lynq-text-dim);
  font-size: 1rem;
  text-decoration: none;
  transition: color 0.3s ease;
}

.try-demo:hover {
  color: var(--lynq-text);
}

.external {
  font-size: 0.9rem;
}

/* Scroll-triggered reveal: hidden until the section intersects (inView adds
   .in-view), then eases into place. Replaces the previous always-on keyframe
   animations so the mockup animates in on scroll. */
.reveal {
  opacity: 0;
  transform: translateY(30px);
  transition:
    opacity var(--lynq-reveal, 0.6s) var(--lynq-ease),
    transform var(--lynq-reveal, 0.6s) var(--lynq-ease);
}

.reveal-scale {
  opacity: 0;
  transform: translateY(50px) scale(0.96);
  transition:
    opacity 0.8s var(--lynq-ease),
    transform 0.8s var(--lynq-ease);
}

.reveal.in-view,
.reveal-scale.in-view {
  opacity: 1;
  transform: translateY(0) scale(1);
}

@media (prefers-reduced-motion: reduce) {
  .reveal,
  .reveal-scale {
    opacity: 1;
    transform: none;
    transition: none;
  }
}

@media (max-width: 768px) {
  .browser-address {
    display: none;
  }

  .features-row {
    grid-template-columns: 1fr 1fr;
  }

}

@media (max-width: 500px) {
  .features-row {
    grid-template-columns: 1fr;
  }
}
</style>
