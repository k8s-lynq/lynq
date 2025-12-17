<template>
  <div ref="containerRef" class="rollout-animation">
    <div class="animation-panels">
      <!-- Without maxSkew -->
      <div class="panel danger-panel">
        <div class="panel-header">
          <span class="panel-icon">⚠️</span>
          <span class="panel-title">Without maxSkew</span>
        </div>
        <div class="panel-subtitle">All nodes update simultaneously</div>
        <div class="nodes-grid">
          <div
            v-for="(node, index) in nodesWithout"
            :key="`without-${index}`"
            class="node"
            :class="node.state"
          >
            <div class="node-inner">
              <span class="node-icon">{{ getNodeIcon(node.state) }}</span>
            </div>
            <div class="node-label">{{ index + 1 }}</div>
          </div>
        </div>
        <div class="panel-status" :class="{ danger: phase === 'without-updating' }">
          {{ getWithoutStatus() }}
        </div>
      </div>

      <!-- With maxSkew -->
      <div class="panel safe-panel">
        <div class="panel-header">
          <span class="panel-icon">✅</span>
          <span class="panel-title">With maxSkew: 3</span>
        </div>
        <div class="panel-subtitle">Controlled sliding window</div>
        <div class="nodes-grid">
          <div
            v-for="(node, index) in nodesWith"
            :key="`with-${index}`"
            class="node"
            :class="node.state"
          >
            <div class="node-inner">
              <span class="node-icon">{{ getNodeIcon(node.state) }}</span>
            </div>
            <div class="node-label">{{ index + 1 }}</div>
          </div>
        </div>
        <div class="panel-status safe">
          {{ getWithStatus() }}
        </div>
      </div>
    </div>

    <!-- Progress indicator -->
    <div class="progress-section">
      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: progressPercent + '%' }"></div>
      </div>
      <div class="progress-label">{{ progressLabel }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue';

const containerRef = ref(null);
const hasStarted = ref(false);

// Node states: 'idle', 'updating', 'ready', 'error'
const NODE_COUNT = 12;
const MAX_SKEW = 3;

const nodesWithout = ref(Array(NODE_COUNT).fill(null).map(() => ({ state: 'idle' })));
const nodesWith = ref(Array(NODE_COUNT).fill(null).map(() => ({ state: 'idle' })));

const phase = ref('idle'); // 'idle', 'without-updating', 'without-error', 'with-updating', 'complete'
const withUpdateIndex = ref(0);
const progressPercent = ref(0);

const getNodeIcon = (state) => {
  switch (state) {
    case 'idle': return '○';
    case 'updating': return '↻';
    case 'ready': return '✓';
    case 'error': return '✗';
    default: return '○';
  }
};

const getWithoutStatus = () => {
  switch (phase.value) {
    case 'idle': return 'Waiting...';
    case 'without-updating': return '🔥 12 nodes updating at once!';
    case 'without-error': return '💥 Cluster overloaded!';
    default: return 'Template updated';
  }
};

const getWithStatus = () => {
  if (phase.value === 'idle') return 'Waiting...';
  if (phase.value === 'with-updating') {
    const ready = nodesWith.value.filter(n => n.state === 'ready').length;
    const updating = nodesWith.value.filter(n => n.state === 'updating').length;
    return `${ready}/${NODE_COUNT} ready, ${updating} updating`;
  }
  if (phase.value === 'complete') return '✅ All nodes updated safely!';
  return 'Controlled rollout';
};

const progressLabel = computed(() => {
  if (phase.value === 'idle') return 'Click to start';
  if (phase.value === 'complete') return 'Rollout complete - Restarting...';
  return 'Rolling update in progress';
});

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

let animationTimer = null;

const runAnimation = async () => {
  // Reset
  nodesWithout.value = Array(NODE_COUNT).fill(null).map(() => ({ state: 'idle' }));
  nodesWith.value = Array(NODE_COUNT).fill(null).map(() => ({ state: 'idle' }));
  phase.value = 'idle';
  progressPercent.value = 0;

  await sleep(500);

  // Phase 1: Without maxSkew - all update at once
  phase.value = 'without-updating';
  nodesWithout.value.forEach(node => { node.state = 'updating'; });
  await sleep(1500);

  // Show error/overload
  phase.value = 'without-error';
  nodesWithout.value.forEach((node, i) => {
    node.state = i % 3 === 0 ? 'error' : 'updating';
  });
  await sleep(2000);

  // Phase 2: With maxSkew - controlled rollout
  phase.value = 'with-updating';
  let readyCount = 0;

  // Initial batch
  for (let i = 0; i < MAX_SKEW; i++) {
    nodesWith.value[i].state = 'updating';
  }

  await sleep(800);

  // Sliding window
  for (let i = 0; i < NODE_COUNT; i++) {
    // Current node becomes ready
    nodesWith.value[i].state = 'ready';
    readyCount++;
    progressPercent.value = Math.round((readyCount / NODE_COUNT) * 100);

    // Start next node if within bounds
    const nextIndex = i + MAX_SKEW;
    if (nextIndex < NODE_COUNT) {
      nodesWith.value[nextIndex].state = 'updating';
    }

    await sleep(400);
  }

  phase.value = 'complete';
  progressPercent.value = 100;

  await sleep(3000);

  // Loop
  if (hasStarted.value) {
    runAnimation();
  }
};

let observer = null;

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting && !hasStarted.value) {
          hasStarted.value = true;
          runAnimation();
        }
      });
    },
    { threshold: 0.3 }
  );

  if (containerRef.value) {
    observer.observe(containerRef.value);
  }
});

onUnmounted(() => {
  hasStarted.value = false;
  if (animationTimer) clearTimeout(animationTimer);
  if (observer) observer.disconnect();
});
</script>

<style scoped>
.rollout-animation {
  background: linear-gradient(135deg, var(--vp-c-bg) 0%, var(--vp-c-bg-soft) 100%);
  border-radius: 16px;
  border: 1px solid var(--vp-c-divider);
  padding: 2rem;
  margin: 2rem 0;
}

.animation-panels {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2rem;
}

@media (max-width: 768px) {
  .animation-panels {
    grid-template-columns: 1fr;
  }
}

.panel {
  background: var(--vp-c-bg);
  border-radius: 12px;
  padding: 1.5rem;
  border: 2px solid var(--vp-c-divider);
  transition: all 0.3s ease;
}

.danger-panel {
  border-color: rgba(239, 68, 68, 0.3);
}

.safe-panel {
  border-color: rgba(16, 185, 129, 0.3);
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
}

.panel-icon {
  font-size: 1.25rem;
}

.panel-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
}

.panel-subtitle {
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
  margin-bottom: 1.25rem;
}

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.node {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
}

.node-inner {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.1rem;
  font-weight: 600;
  transition: all 0.3s ease;
  background: var(--vp-c-bg-soft);
  border: 2px solid var(--vp-c-divider);
}

.node-label {
  font-size: 0.7rem;
  color: var(--vp-c-text-3);
  font-weight: 500;
}

/* Node States */
.node.idle .node-inner {
  color: var(--vp-c-text-3);
  background: var(--vp-c-bg-soft);
}

.node.updating .node-inner {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.5);
  animation: pulse 0.8s ease-in-out infinite;
}

.node.ready .node-inner {
  color: #10b981;
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.5);
}

.node.error .node-inner {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.5);
  animation: shake 0.5s ease-in-out;
}

@keyframes pulse {
  0%, 100% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(245, 158, 11, 0.4);
  }
  50% {
    transform: scale(1.05);
    box-shadow: 0 0 0 8px rgba(245, 158, 11, 0);
  }
}

@keyframes shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-3px); }
  75% { transform: translateX(3px); }
}

.panel-status {
  text-align: center;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.75rem;
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
  color: var(--vp-c-text-2);
  transition: all 0.3s ease;
}

.panel-status.danger {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  animation: dangerPulse 1s ease-in-out infinite;
}

.panel-status.safe {
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
}

@keyframes dangerPulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

.progress-section {
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--vp-c-divider);
}

.progress-bar {
  height: 8px;
  background: var(--vp-c-bg-soft);
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 0.75rem;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #10b981, #059669);
  border-radius: 4px;
  transition: width 0.3s ease;
}

.progress-label {
  text-align: center;
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
}
</style>
