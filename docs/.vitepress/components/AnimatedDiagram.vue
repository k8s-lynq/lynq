<template>
  <div class="operator-diagram">
    <div class="diagram-container">
      <!-- SVG Canvas for flow lines -->
      <svg
        class="diagram-svg"
        viewBox="0 0 1000 400"
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          <!-- Animated gradient for input flow -->
          <linearGradient id="flow-gradient-in" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#00618A" stop-opacity="0">
              <animate attributeName="offset" values="-0.5;1" dur="2s" repeatCount="indefinite" />
            </stop>
            <stop offset="30%" stop-color="#00618A" stop-opacity="0.8">
              <animate attributeName="offset" values="-0.2;1.3" dur="2s" repeatCount="indefinite" />
            </stop>
            <stop offset="60%" stop-color="#00618A" stop-opacity="0">
              <animate attributeName="offset" values="0.1;1.6" dur="2s" repeatCount="indefinite" />
            </stop>
          </linearGradient>

          <!-- Animated gradient for output flow -->
          <linearGradient id="flow-gradient-out" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#326ce5" stop-opacity="0">
              <animate attributeName="offset" values="-0.5;1" dur="2s" repeatCount="indefinite" />
            </stop>
            <stop offset="30%" stop-color="#326ce5" stop-opacity="0.8">
              <animate attributeName="offset" values="-0.2;1.3" dur="2s" repeatCount="indefinite" />
            </stop>
            <stop offset="60%" stop-color="#326ce5" stop-opacity="0">
              <animate attributeName="offset" values="0.1;1.6" dur="2s" repeatCount="indefinite" />
            </stop>
          </linearGradient>

          <!-- Glow filters -->
          <filter id="glow-mysql" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="coloredBlur" />
            <feMerge>
              <feMergeNode in="coloredBlur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>

          <filter id="glow-k8s" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="coloredBlur" />
            <feMerge>
              <feMergeNode in="coloredBlur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>

          <!-- Data packet symbol -->
          <symbol id="data-packet" viewBox="0 0 20 20">
            <rect x="2" y="2" width="16" height="16" rx="3" fill="currentColor" opacity="0.9" />
            <line x1="6" y1="7" x2="14" y2="7" stroke="white" stroke-width="1.5" stroke-linecap="round" />
            <line x1="6" y1="10" x2="14" y2="10" stroke="white" stroke-width="1.5" stroke-linecap="round" />
            <line x1="6" y1="13" x2="10" y2="13" stroke="white" stroke-width="1.5" stroke-linecap="round" />
          </symbol>

          <!-- Resource symbol -->
          <symbol id="resource-cube" viewBox="0 0 24 24">
            <path d="M12 2L2 7v10l10 5 10-5V7L12 2z" fill="currentColor" opacity="0.15" />
            <path d="M12 2L2 7v10l10 5 10-5V7L12 2z" fill="none" stroke="currentColor" stroke-width="1.5" />
            <path d="M12 22V12M2 7l10 5 10-5" fill="none" stroke="currentColor" stroke-width="1.5" />
          </symbol>
        </defs>

        <!-- Background connection lines (static, subtle) -->
        <g class="connection-lines-bg">
          <path
            d="M 180 200 C 280 200 320 200 420 200"
            stroke="var(--vp-c-divider)"
            stroke-width="2"
            fill="none"
            stroke-dasharray="6 4"
            opacity="0.4"
          />
          <path
            d="M 580 200 C 680 200 720 200 820 200"
            stroke="var(--vp-c-divider)"
            stroke-width="2"
            fill="none"
            stroke-dasharray="6 4"
            opacity="0.4"
          />
        </g>

        <!-- Animated flow: MySQL to Lynq -->
        <g class="flow-mysql-lynq">
          <!-- Data packet animations -->
          <g v-for="i in 3" :key="`mysql-flow-${i}`">
            <!-- Glowing dot -->
            <circle
              r="5"
              fill="#00618A"
              filter="url(#glow-mysql)"
              opacity="0"
            >
              <animate
                attributeName="opacity"
                values="0;0.8;0.8;0"
                dur="2s"
                :begin="`${(i - 1) * 0.7}s`"
                repeatCount="indefinite"
              />
              <animateMotion
                dur="2s"
                :begin="`${(i - 1) * 0.7}s`"
                repeatCount="indefinite"
                path="M 180 200 C 280 200 320 200 420 200"
              />
            </circle>
          </g>

          <!-- Flow trail effect -->
          <path
            d="M 180 200 C 280 200 320 200 420 200"
            stroke="url(#flow-gradient-in)"
            stroke-width="3"
            fill="none"
            stroke-linecap="round"
          />
        </g>

        <!-- Animated flow: Lynq to Kubernetes -->
        <g class="flow-lynq-k8s">
          <!-- Resource stream animations -->
          <g v-for="i in 3" :key="`k8s-flow-${i}`">
            <circle
              r="5"
              fill="#326ce5"
              filter="url(#glow-k8s)"
              opacity="0"
            >
              <animate
                attributeName="opacity"
                values="0;0.8;0.8;0"
                dur="2s"
                :begin="`${0.3 + (i - 1) * 0.7}s`"
                repeatCount="indefinite"
              />
              <animateMotion
                dur="2s"
                :begin="`${0.3 + (i - 1) * 0.7}s`"
                repeatCount="indefinite"
                path="M 580 200 C 680 200 720 200 820 200"
              />
            </circle>
          </g>

          <!-- Flow trail -->
          <path
            d="M 580 200 C 680 200 720 200 820 200"
            stroke="url(#flow-gradient-out)"
            stroke-width="3"
            fill="none"
            stroke-linecap="round"
          />
        </g>
      </svg>

      <!-- Node elements -->
      <div class="nodes-container">
        <!-- MySQL Database -->
        <div class="node mysql-node">
          <div class="node-glow mysql-glow"></div>
          <div class="node-icon-wrapper">
            <img src="/mysql-icon.svg" alt="MySQL" class="node-icon mysql-icon" />
          </div>
          <div class="node-label">
            <span class="label-text">Database</span>
            <span class="label-subtext">MySQL Records</span>
          </div>
          <!-- Animated data rows -->
          <div class="data-rows">
            <div v-for="i in 4" :key="`row-${i}`" class="data-row" :style="{ animationDelay: `${i * 0.3}s` }">
              <span class="row-dot"></span>
              <span class="row-line"></span>
            </div>
          </div>
        </div>

        <!-- Lynq Operator (Center) -->
        <div class="node lynq-node">
          <div class="node-icon-wrapper lynq-wrapper">
            <img src="/logo.png" alt="Lynq" class="node-icon lynq-icon" />
          </div>
          <div class="node-label">
            <span class="label-text">Lynq</span>
            <span class="label-subtext">Operator</span>
          </div>
        </div>

        <!-- Kubernetes -->
        <div class="node k8s-node">
          <div class="node-glow k8s-glow"></div>
          <div class="node-icon-wrapper">
            <img src="/k8s-logo.svg" alt="Kubernetes" class="node-icon k8s-icon" />
          </div>
          <div class="node-label">
            <span class="label-text">Kubernetes</span>
            <span class="label-subtext">Resources</span>
          </div>
          <!-- Resource indicators - below K8s node -->
          <div class="resource-indicators">
            <div
              v-for="slot in 4"
              :key="slot - 1"
              class="resource-indicator"
              :class="{ transitioning: transitioningSlot === slot - 1 }"
            >
              <span class="resource-dot" :class="`resource-${slot - 1}`"></span>
              <span class="resource-name">{{ getResource(slot - 1) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Flow labels -->
      <div class="flow-labels">
        <div class="flow-label left">
          <span class="flow-text">Sync</span>
        </div>
        <div class="flow-label right">
          <span class="flow-text">Apply</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue';

// Resource options for each slot (cycles through these)
const resourceOptions = [
  ['Deploy', 'CM', 'STS', 'SA'],
  ['Service', 'PVC', 'DS', 'NetPol'],
  ['Ingress', 'Job', 'HPA', 'NS'],
  ['Secret', 'CronJob', 'PDB', 'CRD'],
];

// Current index for each slot
const slotIndices = reactive([0, 0, 0, 0]);
const transitioningSlot = ref(-1);

// Get current resource for each slot
const getResource = (slot) => resourceOptions[slot][slotIndices[slot]];

let intervalId = null;
let currentSlot = 0;

const cycleNextSlot = () => {
  // Mark current slot as transitioning
  transitioningSlot.value = currentSlot;

  setTimeout(() => {
    // Update the slot's index
    slotIndices[currentSlot] = (slotIndices[currentSlot] + 1) % resourceOptions[currentSlot].length;
    transitioningSlot.value = -1;

    // Move to next slot
    currentSlot = (currentSlot + 1) % 4;
  }, 200);
};

onMounted(() => {
  intervalId = setInterval(cycleNextSlot, 800);
});

onUnmounted(() => {
  if (intervalId) clearInterval(intervalId);
});
</script>

<style scoped>
.operator-diagram {
  position: relative;
  width: 100%;
  padding: 1rem 0;
}

.diagram-container {
  position: relative;
  width: 100%;
  height: 400px;
  max-width: 900px;
  margin: 0 auto;
}

/* SVG Canvas */
.diagram-svg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 1;
}

/* Nodes container */
.nodes-container {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 2rem;
  z-index: 10;
}

/* Individual node */
.node {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  z-index: 10;
}

/* Node glow effects */
.node-glow {
  position: absolute;
  width: 120px;
  height: 120px;
  border-radius: 50%;
  opacity: 0.3;
  filter: blur(20px);
  animation: pulseGlow 3s ease-in-out infinite;
  pointer-events: none;
}

.mysql-glow {
  background: radial-gradient(circle, #00618A 0%, transparent 70%);
}

.k8s-glow {
  background: radial-gradient(circle, #326ce5 0%, transparent 70%);
}

@keyframes pulseGlow {
  0%, 100% {
    opacity: 0.2;
    transform: scale(1);
  }
  50% {
    opacity: 0.4;
    transform: scale(1.1);
  }
}

/* Node icon wrapper */
.node-icon-wrapper {
  position: relative;
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--vp-c-bg);
  border-radius: 20px;
  border: 2px solid var(--vp-c-divider);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.node:hover .node-icon-wrapper {
  transform: translateY(-4px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
}

.lynq-wrapper {
  width: 100px;
  height: 100px;
  border-radius: 24px;
  border: 2px solid var(--vp-c-divider);
  background: var(--vp-c-bg);
}

/* Node icons */
.node-icon {
  width: 50px;
  height: 50px;
  object-fit: contain;
  transition: transform 0.3s ease;
}

.mysql-icon {
  width: 55px;
  height: 55px;
}

.lynq-icon {
  width: 70px;
  height: 70px;
}

.k8s-icon {
  width: 60px;
  height: 60px;
}

/* Node labels */
.node-label {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
}

.label-text {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
}

.label-subtext {
  font-size: 0.8rem;
  color: var(--vp-c-text-3);
  font-weight: 500;
}

/* Data rows animation for MySQL */
.data-rows {
  position: absolute;
  top: -60px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.data-row {
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0;
  animation: dataRowPulse 2.4s ease-in-out infinite;
}

.row-dot {
  width: 6px;
  height: 6px;
  background: #00618A;
  border-radius: 50%;
}

.row-line {
  width: 30px;
  height: 3px;
  background: linear-gradient(90deg, #00618A 0%, transparent 100%);
  border-radius: 2px;
}

@keyframes dataRowPulse {
  0%, 100% {
    opacity: 0;
    transform: translateX(-10px);
  }
  20%, 80% {
    opacity: 0.8;
    transform: translateX(0);
  }
}

/* Resource indicators - below K8s node */
.resource-indicators {
  display: grid;
  grid-template-columns: repeat(2, 56px);
  gap: 6px;
  margin-top: 0.5rem;
}

.resource-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  width: 56px;
  height: 24px;
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  font-size: 0.65rem;
  font-weight: 600;
  color: var(--vp-c-text-2);
  transition: opacity 0.2s ease, transform 0.2s ease, background 0.2s ease, border-color 0.2s ease;
  overflow: hidden;
  white-space: nowrap;
}

.resource-indicator.transitioning {
  opacity: 0;
  transform: scale(0.9);
}

.resource-indicator:hover {
  background: var(--vp-c-bg);
  border-color: var(--vp-c-brand);
}

.resource-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #326ce5;
}

.resource-0 { background: #10b981; }
.resource-1 { background: #3b82f6; }
.resource-2 { background: #f59e0b; }
.resource-3 { background: #8b5cf6; }

.resource-name {
  color: var(--vp-c-text-2);
}

/* Flow labels */
.flow-labels {
  position: absolute;
  top: 58%;
  left: 0;
  right: 0;
  display: flex;
  justify-content: center;
  gap: 30%;
  pointer-events: none;
  z-index: 5;
}

.flow-label {
  padding: 0.35rem 0.75rem;
  background: var(--vp-c-bg-soft);
  border-radius: 12px;
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--vp-c-text-3);
  opacity: 0;
  animation: fadeInLabel 0.8s ease-out 1s forwards;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

@keyframes fadeInLabel {
  to {
    opacity: 0.8;
  }
}

/* Responsive */
@media (max-width: 768px) {
  .diagram-container {
    height: 350px;
  }

  .nodes-container {
    padding: 0 1rem;
  }

  .node-icon-wrapper {
    width: 60px;
    height: 60px;
    border-radius: 16px;
  }

  .lynq-wrapper {
    width: 80px;
    height: 80px;
    border-radius: 20px;
  }

  .node-icon {
    width: 35px;
    height: 35px;
  }

  .mysql-icon {
    width: 40px;
    height: 40px;
  }

  .lynq-icon {
    width: 55px;
    height: 55px;
  }

  .k8s-icon {
    width: 45px;
    height: 45px;
  }

  .label-text {
    font-size: 0.95rem;
  }

  .label-subtext {
    font-size: 0.7rem;
  }

  .data-rows {
    top: -50px;
  }

  .resource-indicators {
    grid-template-columns: repeat(2, 50px);
    gap: 4px;
  }

  .resource-indicator {
    width: 50px;
    height: 22px;
    font-size: 0.58rem;
  }

  .flow-labels {
    gap: 25%;
  }

  .flow-label {
    padding: 0.3rem 0.6rem;
    font-size: 0.6rem;
  }

  .node-glow {
    width: 80px;
    height: 80px;
  }
}

@media (max-width: 640px) {
  .diagram-container {
    height: 300px;
  }

  .node-icon-wrapper {
    width: 50px;
    height: 50px;
    border-radius: 12px;
  }

  .lynq-wrapper {
    width: 65px;
    height: 65px;
    border-radius: 16px;
  }

  .node-icon {
    width: 28px;
    height: 28px;
  }

  .mysql-icon {
    width: 32px;
    height: 32px;
  }

  .lynq-icon {
    width: 45px;
    height: 45px;
  }

  .k8s-icon {
    width: 38px;
    height: 38px;
  }

  .label-text {
    font-size: 0.85rem;
  }

  .label-subtext {
    font-size: 0.65rem;
  }

  .data-rows {
    display: none;
  }

  .resource-indicators {
    grid-template-columns: repeat(2, 44px);
    gap: 3px;
  }

  .resource-indicator {
    width: 44px;
    height: 20px;
    font-size: 0.52rem;
    gap: 3px;
  }

  .resource-dot {
    width: 5px;
    height: 5px;
  }

  .flow-labels {
    display: none;
  }
}
</style>
