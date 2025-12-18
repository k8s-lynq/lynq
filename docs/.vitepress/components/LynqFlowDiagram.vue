<template>
  <div class="lynq-flow-diagram">
    <!-- Row 1: Database → LynqHub → LynqNodes -->
    <div class="flow-row">
      <!-- Database -->
      <div class="flow-node node-database" :class="{ active: activeStep >= 1 }">
        <div class="node-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <ellipse cx="12" cy="5" rx="9" ry="3"/>
            <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
            <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
          </svg>
        </div>
        <div class="node-title">Database</div>
        <div class="node-desc">Data Source</div>
      </div>

      <!-- Arrow 1 -->
      <div class="flow-arrow" :class="{ active: activeStep >= 2 }">
        <svg width="50" height="24" viewBox="0 0 50 24">
          <defs>
            <marker id="ah1" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="currentColor"/>
            </marker>
          </defs>
          <line x1="0" y1="12" x2="40" y2="12" stroke="currentColor" stroke-width="2" marker-end="url(#ah1)"/>
        </svg>
        <span class="arrow-label">query</span>
      </div>

      <!-- LynqHub -->
      <div class="flow-node node-hub" :class="{ active: activeStep >= 2 }">
        <div class="node-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <circle cx="12" cy="12" r="4"/>
            <line x1="12" y1="2" x2="12" y2="6"/>
            <line x1="12" y1="18" x2="12" y2="22"/>
            <line x1="2" y1="12" x2="6" y2="12"/>
            <line x1="18" y1="12" x2="22" y2="12"/>
          </svg>
        </div>
        <div class="node-title">LynqHub</div>
        <div class="node-desc">Sync 30s</div>
      </div>

      <!-- Arrow 2 -->
      <div class="flow-arrow" :class="{ active: activeStep >= 3 }">
        <svg width="50" height="24" viewBox="0 0 50 24">
          <defs>
            <marker id="ah2" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="currentColor"/>
            </marker>
          </defs>
          <line x1="0" y1="12" x2="40" y2="12" stroke="currentColor" stroke-width="2" marker-end="url(#ah2)"/>
        </svg>
        <span class="arrow-label">create</span>
      </div>

      <!-- LynqNodes with LynqForm -->
      <div class="nodes-group" :class="{ active: activeStep >= 3 }">
        <div class="lynqform-badge">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
          </svg>
          <span>LynqForm</span>
        </div>
        <div class="flow-node node-nodes">
          <div class="node-icon">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="7" height="7" rx="1"/>
              <rect x="14" y="3" width="7" height="7" rx="1"/>
              <rect x="3" y="14" width="7" height="7" rx="1"/>
              <rect x="14" y="14" width="7" height="7" rx="1"/>
            </svg>
          </div>
          <div class="node-title">LynqNodes</div>
          <div class="node-items-inline">
            <span>a</span><span>b</span><span>c</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Down Arrow -->
    <div class="flow-down-arrow" :class="{ active: activeStep >= 4 }">
      <svg width="24" height="40" viewBox="0 0 24 40">
        <defs>
          <marker id="ah3" markerWidth="8" markerHeight="6" refX="3" refY="3" orient="auto">
            <polygon points="0 0, 6 3, 0 6" fill="currentColor"/>
          </marker>
        </defs>
        <line x1="12" y1="0" x2="12" y2="32" stroke="currentColor" stroke-width="2" marker-end="url(#ah3)"/>
      </svg>
      <span class="arrow-label">apply</span>
    </div>

    <!-- Row 2: K8s Resources -->
    <div class="flow-row row-resources">
      <div class="flow-node node-resources" :class="{ active: activeStep >= 4 }">
        <div class="node-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <div class="node-title">Kubernetes Resources</div>
        <div class="resource-tags">
          <span class="resource-tag">Deployment</span>
          <span class="resource-tag">Service</span>
          <span class="resource-tag">Ingress</span>
          <span class="resource-tag">ConfigMap</span>
        </div>
      </div>
    </div>

    <!-- Progress & Controls -->
    <div class="controls">
      <div class="progress-dots">
        <span
          v-for="step in 4"
          :key="step"
          class="dot"
          :class="{ active: activeStep >= step, current: activeStep === step }"
          @click="setStep(step)"
        ></span>
      </div>
      <button class="play-btn" @click="toggleAutoPlay">
        <svg v-if="!isPlaying" width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <polygon points="5 3 19 12 5 21 5 3"/>
        </svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <rect x="6" y="4" width="4" height="16"/>
          <rect x="14" y="4" width="4" height="16"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue';

const activeStep = ref(0);
const isPlaying = ref(false);
let intervalId = null;

const setStep = (step) => {
  activeStep.value = step;
};

const toggleAutoPlay = () => {
  if (isPlaying.value) {
    stopAutoPlay();
  } else {
    startAutoPlay();
  }
};

const startAutoPlay = () => {
  isPlaying.value = true;
  activeStep.value = 0;
  intervalId = setInterval(() => {
    activeStep.value = activeStep.value < 4 ? activeStep.value + 1 : 0;
  }, 1200);
};

const stopAutoPlay = () => {
  isPlaying.value = false;
  if (intervalId) {
    clearInterval(intervalId);
    intervalId = null;
  }
};

onMounted(() => {
  setTimeout(startAutoPlay, 500);
});

onBeforeUnmount(() => {
  stopAutoPlay();
});
</script>

<style scoped>
.lynq-flow-diagram {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.5rem;
  padding-top: 3rem;
  background: var(--vp-c-bg-soft);
  border-radius: 12px;
  margin: 1.5rem 0;
  gap: 0.5rem;
  overflow: visible;
}

/* Flow Rows */
.flow-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
  overflow: visible;
}

.row-resources {
  margin-top: 0.25rem;
}

/* Flow Nodes */
.flow-node {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 0.75rem 1rem;
  background: var(--vp-c-bg);
  border: 2px solid var(--vp-c-divider);
  border-radius: 10px;
  min-width: 90px;
  opacity: 0.3;
  transform: scale(0.95);
  transition: all 0.4s ease;
}

.flow-node.active,
.nodes-group.active .flow-node {
  opacity: 1;
  transform: scale(1);
}

.node-database.active { border-color: #42b883; }
.node-hub.active { border-color: #667eea; }
.nodes-group.active .node-nodes { border-color: #41d1ff; }
.node-resources.active { border-color: #f59e0b; }

.node-icon {
  margin-bottom: 0.25rem;
}

.node-database .node-icon { color: #42b883; }
.node-hub .node-icon { color: #667eea; }
.node-nodes .node-icon { color: #41d1ff; }
.node-resources .node-icon { color: #f59e0b; }

.node-title {
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
}

.node-desc {
  font-size: 0.65rem;
  color: var(--vp-c-text-3);
}

.node-items-inline {
  display: flex;
  gap: 0.25rem;
  margin-top: 0.25rem;
}

.node-items-inline span {
  font-size: 0.6rem;
  padding: 0.1rem 0.3rem;
  background: var(--vp-c-bg-soft);
  border-radius: 3px;
  color: var(--vp-c-text-2);
}

/* Nodes Group with LynqForm */
.nodes-group {
  position: relative;
  opacity: 0.3;
  transition: opacity 0.4s ease;
  overflow: visible;
  padding-top: 28px;
  margin-top: -28px;
}

.nodes-group.active {
  opacity: 1;
}

.lynqform-badge {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.5rem;
  background: var(--vp-c-bg);
  border: 1.5px solid #764ba2;
  border-radius: 6px;
  font-size: 0.65rem;
  font-weight: 600;
  color: #764ba2;
  white-space: nowrap;
  z-index: 10;
}

/* Arrows */
.flow-arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: var(--vp-c-divider);
  opacity: 0.3;
  transition: all 0.4s ease;
}

.flow-arrow.active {
  opacity: 1;
  color: var(--vp-c-brand);
}

.flow-down-arrow {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--vp-c-divider);
  opacity: 0.3;
  transition: all 0.4s ease;
}

.flow-down-arrow.active {
  opacity: 1;
  color: var(--vp-c-brand);
}

.arrow-label {
  font-size: 0.6rem;
  color: var(--vp-c-text-3);
}

/* Resources Node - wider */
.node-resources {
  min-width: 200px;
}

.resource-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-top: 0.35rem;
  justify-content: center;
}

.resource-tag {
  font-size: 0.6rem;
  padding: 0.15rem 0.4rem;
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border-radius: 4px;
}

/* Controls */
.controls {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--vp-c-divider);
}

.progress-dots {
  display: flex;
  gap: 0.5rem;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--vp-c-divider);
  cursor: pointer;
  transition: all 0.3s ease;
}

.dot.active {
  background: var(--vp-c-brand);
}

.dot.current {
  box-shadow: 0 0 0 3px rgba(var(--vp-c-brand-rgb), 0.3);
}

.play-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: var(--vp-c-bg);
  border: 1px solid var(--vp-c-divider);
  border-radius: 50%;
  color: var(--vp-c-text-2);
  cursor: pointer;
  transition: all 0.3s ease;
}

.play-btn:hover {
  border-color: var(--vp-c-brand);
  color: var(--vp-c-brand);
}

/* Responsive */
@media (max-width: 640px) {
  .flow-row {
    flex-direction: column;
    gap: 0.5rem;
  }

  .flow-arrow {
    transform: rotate(90deg);
  }

  .flow-down-arrow {
    display: none;
  }

  .row-resources {
    margin-top: 0.5rem;
  }

  .lynqform-badge {
    position: static;
    transform: none;
    margin-bottom: 0.25rem;
  }
}
</style>
