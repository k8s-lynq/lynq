<template>
  <div ref="containerRef" class="blast-radius-animation">
    <div class="animation-header">
      <span class="header-icon">💥</span>
      <span class="header-title">Blast Radius: Template Update Impact</span>
    </div>

    <div class="animation-content">
      <!-- Template Change Indicator -->
      <div class="template-section">
        <div class="template-card" :class="{ updated: phase !== 'idle' }">
          <div class="template-label">LynqForm Template</div>
          <div class="template-change">
            <code class="old-value" :class="{ struck: phase !== 'idle' }">image: app:v1</code>
            <span class="arrow" v-if="phase !== 'idle'">→</span>
            <code class="new-value" v-if="phase !== 'idle'">image: app:v2</code>
          </div>
        </div>
        <div class="propagation-arrow" :class="{ active: phase !== 'idle' }">
          <span>▼</span>
          <span class="propagation-label">Applies to all nodes</span>
        </div>
      </div>

      <!-- Nodes Grid -->
      <div class="nodes-section">
        <div class="nodes-header">
          <span class="nodes-count">{{ nodeCount }} Nodes</span>
          <span class="nodes-status" :class="statusClass">{{ statusText }}</span>
        </div>
        <div class="nodes-grid">
          <div
            v-for="(node, index) in nodes"
            :key="index"
            class="node"
            :class="node.state"
          >
            <div class="node-inner">
              <span class="node-icon">{{ getNodeIcon(node.state) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Impact Metrics -->
      <div class="metrics-section">
        <div class="metric danger">
          <span class="metric-value">{{ crashingCount }}</span>
          <span class="metric-label">CrashLoopBackOff</span>
        </div>
        <div class="metric warning">
          <span class="metric-value">{{ updatingCount }}</span>
          <span class="metric-label">Updating</span>
        </div>
        <div class="metric success">
          <span class="metric-value">{{ healthyCount }}</span>
          <span class="metric-label">Healthy</span>
        </div>
      </div>

      <!-- Warning Message - always rendered, visibility controlled -->
      <div class="warning-message" :class="{ visible: phase === 'paralyzed' }">
        <span class="warning-icon">🚨</span>
        <span class="warning-text">Cluster paralyzed! All services affected.</span>
      </div>
    </div>

    <div class="restart-hint" :class="{ visible: phase === 'paralyzed' }">
      Restarting demo...
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';

const containerRef = ref(null);
const hasStarted = ref(false);

const nodeCount = 24; // Representative sample
const nodes = ref(Array(nodeCount).fill(null).map(() => ({ state: 'healthy' })));
const phase = ref('idle'); // 'idle', 'updating', 'crashing', 'paralyzed'

const getNodeIcon = (state) => {
  switch (state) {
    case 'healthy': return '✓';
    case 'updating': return '↻';
    case 'crashing': return '✗';
    default: return '○';
  }
};

const crashingCount = computed(() => nodes.value.filter(n => n.state === 'crashing').length);
const updatingCount = computed(() => nodes.value.filter(n => n.state === 'updating').length);
const healthyCount = computed(() => nodes.value.filter(n => n.state === 'healthy').length);

const statusClass = computed(() => {
  if (phase.value === 'paralyzed') return 'danger';
  if (phase.value === 'crashing') return 'warning';
  if (phase.value === 'updating') return 'updating';
  return 'healthy';
});

const statusText = computed(() => {
  switch (phase.value) {
    case 'idle': return 'All healthy';
    case 'updating': return 'All updating simultaneously...';
    case 'crashing': return 'Pods failing!';
    case 'paralyzed': return 'CLUSTER DOWN';
    default: return '';
  }
});

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

const runAnimation = async () => {
  // Reset
  nodes.value = Array(nodeCount).fill(null).map(() => ({ state: 'healthy' }));
  phase.value = 'idle';

  await sleep(1500);

  // Phase 1: Template updated, all nodes start updating
  phase.value = 'updating';
  nodes.value.forEach(node => { node.state = 'updating'; });

  await sleep(2000);

  // Phase 2: Bug in v2 causes crashes - gradual failure
  phase.value = 'crashing';

  // First wave of crashes
  for (let i = 0; i < nodeCount; i += 3) {
    nodes.value[i].state = 'crashing';
  }
  await sleep(500);

  // Second wave
  for (let i = 1; i < nodeCount; i += 3) {
    nodes.value[i].state = 'crashing';
  }
  await sleep(500);

  // Final wave - almost everything crashes
  for (let i = 2; i < nodeCount; i += 3) {
    if (Math.random() > 0.2) {
      nodes.value[i].state = 'crashing';
    }
  }

  await sleep(1000);

  // Phase 3: Cluster paralyzed
  phase.value = 'paralyzed';

  await sleep(4000);

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
  if (observer) observer.disconnect();
});
</script>

<style scoped>
.blast-radius-animation {
  background: linear-gradient(135deg, var(--vp-c-bg) 0%, var(--vp-c-bg-soft) 100%);
  border-radius: 16px;
  border: 2px solid rgba(239, 68, 68, 0.3);
  padding: 1.5rem;
  margin: 2rem 0;
  min-height: 480px;
}

@media (max-width: 640px) {
  .blast-radius-animation {
    min-height: 520px;
  }
}

.animation-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--vp-c-divider);
}

.header-icon {
  font-size: 1.5rem;
}

.header-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
}

.animation-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

/* Template Section */
.template-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
}

.template-card {
  background: var(--vp-c-bg);
  border: 2px solid var(--vp-c-divider);
  border-radius: 8px;
  padding: 1rem 1.5rem;
  transition: all 0.3s ease;
}

.template-card.updated {
  border-color: #f59e0b;
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.2);
}

.template-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--vp-c-text-2);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.5rem;
}

.template-change {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-family: var(--vp-font-family-mono);
}

.old-value {
  color: var(--vp-c-text-2);
  transition: all 0.3s ease;
}

.old-value.struck {
  text-decoration: line-through;
  opacity: 0.5;
}

.arrow {
  color: #f59e0b;
  font-weight: bold;
}

.new-value {
  color: #f59e0b;
  font-weight: 600;
}

.propagation-arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: var(--vp-c-text-3);
  font-size: 1.25rem;
  opacity: 0;
  transition: all 0.3s ease;
}

.propagation-arrow.active {
  opacity: 1;
  color: #ef4444;
  animation: pulse-down 1s ease-in-out infinite;
}

.propagation-label {
  font-size: 0.75rem;
  margin-top: 0.25rem;
}

@keyframes pulse-down {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(4px); }
}

/* Nodes Section */
.nodes-section {
  background: var(--vp-c-bg);
  border-radius: 12px;
  padding: 1rem;
  border: 1px solid var(--vp-c-divider);
}

.nodes-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.nodes-count {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--vp-c-text-2);
}

.nodes-status {
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.25rem 0.75rem;
  border-radius: 4px;
  transition: all 0.3s ease;
}

.nodes-status.healthy {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.nodes-status.updating {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.nodes-status.warning {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.nodes-status.danger {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  animation: danger-pulse 0.5s ease-in-out infinite;
}

@keyframes danger-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 0.5rem;
}

@media (max-width: 640px) {
  .nodes-grid {
    grid-template-columns: repeat(6, 1fr);
  }
}

.node {
  display: flex;
  align-items: center;
  justify-content: center;
}

.node-inner {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.9rem;
  font-weight: 600;
  transition: all 0.3s ease;
  background: var(--vp-c-bg-soft);
  border: 2px solid var(--vp-c-divider);
}

.node.healthy .node-inner {
  color: #10b981;
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.3);
}

.node.updating .node-inner {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.5);
}

.node.updating .node-icon {
  display: inline-block;
  animation: spin 1s linear infinite;
}

.node.crashing .node-inner {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.5);
  animation: shake 0.3s ease-in-out infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-2px); }
  75% { transform: translateX(2px); }
}

/* Metrics Section */
.metrics-section {
  display: flex;
  justify-content: center;
  gap: 2rem;
  min-height: 52px;
}

.metric {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
}

.metric-value {
  font-size: 1.5rem;
  font-weight: 700;
}

.metric-label {
  font-size: 0.75rem;
  color: var(--vp-c-text-2);
}

.metric.danger .metric-value {
  color: #ef4444;
}

.metric.warning .metric-value {
  color: #f59e0b;
}

.metric.success .metric-value {
  color: #10b981;
}

/* Warning Message */
.warning-message {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 8px;
  padding: 1rem;
  min-height: 50px;
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.3s ease, visibility 0.3s ease;
}

.warning-message.visible {
  opacity: 1;
  visibility: visible;
  animation: warning-flash 1s ease-in-out infinite;
}

.warning-icon {
  font-size: 1.25rem;
}

.warning-text {
  font-weight: 600;
  color: #ef4444;
}

@keyframes warning-flash {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.8; }
}

.restart-hint {
  text-align: center;
  font-size: 0.8rem;
  color: var(--vp-c-text-3);
  margin-top: 1rem;
  min-height: 20px;
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.3s ease, visibility 0.3s ease;
}

.restart-hint.visible {
  opacity: 1;
  visibility: visible;
}
</style>
