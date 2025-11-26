<template>
  <div class="operator-diagram">
    <div class="diagram-container">
      <!-- SVG Canvas for all animations -->
      <svg
        class="diagram-svg"
        viewBox="0 0 1000 400"
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          <!-- Gradients for lines -->
          <linearGradient id="green-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#42b883" stop-opacity="0" />
            <stop offset="50%" stop-color="#42b883" stop-opacity="0.8" />
            <stop offset="100%" stop-color="#42b883" stop-opacity="0" />
          </linearGradient>

          <linearGradient id="blue-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#41d1ff" stop-opacity="0" />
            <stop offset="50%" stop-color="#41d1ff" stop-opacity="0.8" />
            <stop offset="100%" stop-color="#41d1ff" stop-opacity="0" />
          </linearGradient>

          <!-- Glow filters -->
          <filter id="glow-green">
            <feGaussianBlur stdDeviation="3" result="coloredBlur" />
            <feMerge>
              <feMergeNode in="coloredBlur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>

          <filter id="glow-blue">
            <feGaussianBlur stdDeviation="3" result="coloredBlur" />
            <feMerge>
              <feMergeNode in="coloredBlur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <!-- Input lines from Database to Operator -->
        <g class="input-lines">
          <path
            v-for="(line, index) in inputPaths"
            :key="`input-${index}`"
            :d="line"
            stroke="#42b883"
            stroke-width="1.5"
            fill="none"
            opacity="0.2"
          />

          <!-- Animated input lines -->
          <g v-for="(line, index) in inputPaths" :key="`input-anim-${index}`">
            <path
              :d="line"
              stroke="url(#green-gradient)"
              stroke-width="2"
              fill="none"
              class="animated-line"
              :style="{
                animationDelay: `${index * 0.6}s`,
                filter: 'url(#glow-green)',
              }"
            />
            <circle
              r="4"
              fill="#42b883"
              class="flow-dot"
              :style="{ animationDelay: `${index * 0.6}s` }"
            >
              <animateMotion
                :dur="`${2.5 + index * 0.15}s`"
                repeatCount="indefinite"
                :path="line"
                :begin="`${index * 0.6}s`"
              />
            </circle>
          </g>
        </g>

        <!-- Output lines from Operator to Kubernetes -->
        <g class="output-lines">
          <path
            v-for="(line, index) in outputPaths"
            :key="`output-${index}`"
            :d="line"
            stroke="#41d1ff"
            stroke-width="1.5"
            fill="none"
            opacity="0.2"
          />

          <!-- Animated output lines -->
          <g v-for="(line, index) in outputPaths" :key="`output-anim-${index}`">
            <path
              :d="line"
              stroke="url(#blue-gradient)"
              stroke-width="2"
              fill="none"
              class="animated-line"
              :style="{
                animationDelay: `${index * 0.5}s`,
                filter: 'url(#glow-blue)',
              }"
            />
            <circle
              r="4"
              fill="#41d1ff"
              class="flow-dot"
              :style="{ animationDelay: `${index * 0.5}s` }"
            >
              <animateMotion
                :dur="`${2.5 + index * 0.15}s`"
                repeatCount="indefinite"
                :path="line"
                :begin="`${index * 0.5}s`"
              />
            </circle>
          </g>
        </g>
      </svg>

      <!-- Node elements positioned absolutely -->
      <div class="nodes">
        <!-- Database Node -->
        <div
          class="node database-node"
          :style="{
            left: `${nodePositions.database.x}%`,
            top: `${nodePositions.database.y}%`,
          }"
        >
          <div class="node-icon">
            <svg width="70" height="70" viewBox="0 0 70 70" fill="none">
              <ellipse
                cx="35"
                cy="18"
                rx="22"
                ry="9"
                fill="#42b883"
                opacity="0.15"
              />
              <ellipse
                cx="35"
                cy="18"
                rx="22"
                ry="9"
                stroke="#42b883"
                stroke-width="2.5"
              />
              <path
                d="M13 18 L13 52 Q13 61 35 61 Q57 61 57 52 L57 18"
                stroke="#42b883"
                stroke-width="2.5"
                fill="none"
              />
              <ellipse
                cx="35"
                cy="52"
                rx="22"
                ry="9"
                stroke="#42b883"
                stroke-width="2.5"
                fill="none"
              />
              <path
                d="M13 30 Q13 39 35 39 Q57 39 57 30"
                stroke="#42b883"
                stroke-width="2"
                fill="none"
              />
              <path
                d="M13 41 Q13 50 35 50 Q57 50 57 41"
                stroke="#42b883"
                stroke-width="2"
                fill="none"
              />
            </svg>
          </div>
          <div class="node-title">Database</div>
        </div>

        <!-- Operator Node -->
        <div
          class="node operator-node"
          :style="{
            left: `${nodePositions.operator.x}%`,
            top: `${nodePositions.operator.y}%`,
          }"
        >
          <div class="node-icon">
            <div class="operator-glow"></div>
            <img src="/logo.png" alt="Lynq" class="operator-logo" />
          </div>
          <div class="node-title">Lynq</div>
        </div>
      </div>

      <!-- Resource nodes on the right -->
      <div class="resource-nodes">
        <div
          v-for="(resource, index) in resources"
          :key="resource"
          class="resource-node"
          :style="{
            top: `${resourcePositions[index]}%`,
            animationDelay: `${0.5 + index * 0.12}s`,
          }"
        >
          <div class="resource-label">{{ resource }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from "vue";

// Node positions in percentage
const nodePositions = ref({
  database: { x: 12, y: 54  }, // Left: 12%, Center: 54 %
  operator: { x: 50, y: 54  }, // Center: 50%, Center: 54 %
});

// Resources to display
const resources = ref([
  "Deployments",
  "Services",
  "Ingresses",
  "ConfigMaps",
  "Secrets",
  "CRDs",
]);

// Resource positions (vertical distribution)
const resourcePositions = ref([15, 27, 39, 51, 63, 75]);

// SVG path coordinates (based on viewBox 1000x400)
// Database at x=150, Operator at x=500, Resources at x=900
// Operator node occupies space from x=450 to x=550 (centered at x=500)
// Input lines end at x=450 (left edge), Output lines start at x=550 (right edge)

const inputPaths = ref([
  "M 150 180 Q 300 165 450 185",
  "M 150 190 Q 300 188 450 193",
  "M 150 200 Q 300 200 450 200",
  "M 150 210 Q 300 212 450 207",
  "M 150 220 Q 300 235 450 215",
]);

// Output paths to each resource
// y coordinates: 60, 108, 156, 204, 252, 300 (15%, 27%, 39%, 51%, 63%, 75% of 400)
const outputPaths = ref([
  "M 550 185 Q 725 120 900 60", // Namespaces
  "M 550 193 Q 725 145 900 108", // Deployments
  "M 550 200 Q 725 175 900 156", // Services
  "M 550 207 Q 725 202 900 204", // Ingresses
  "M 550 215 Q 725 230 900 252", // ConfigMaps
  "M 550 220 Q 725 255 900 300", // Secrets
]);
</script>

<style scoped>
.diagram-container {
  position: relative;
  width: 100%;
  height: 450px;
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

/* Animated line stroke animation */
@keyframes lineFlow {
  0% {
    stroke-dasharray: 0 1000;
    opacity: 0;
  }
  40% {
    opacity: 1;
  }
  100% {
    stroke-dasharray: 1000 0;
    opacity: 0;
  }
}

.animated-line {
  stroke-dasharray: 0 1000;
  animation: lineFlow 3s ease-in-out infinite;
}

/* Flow dot animation */
.flow-dot {
  filter: drop-shadow(0 0 6px currentColor);
  opacity: 0;
  animation: fadeInDot 0.5s ease-in forwards;
}

@keyframes fadeInDot {
  0% {
    opacity: 0;
  }
  100% {
    opacity: 0.9;
  }
}

/* Nodes container */
.nodes {
  position: relative;
  width: 100%;
  height: 100%;
  z-index: 10;
}

.node {
  position: absolute;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  transition: transform 0.3s ease;
}

.node:hover {
  transform: translate(-50%, -50%) scale(1.08);
}

.node-icon {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.15));
}

/* Operator logo */
.operator-logo {
  width: 90px;
  height: 90px;
  position: relative;
  z-index: 1;
  filter: drop-shadow(0 4px 12px rgba(100, 108, 255, 0.3));
  animation: operatorFloat 3s ease-in-out infinite;
}

@keyframes operatorFloat {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-8px);
  }
}

/* Operator glow effect */
@keyframes operatorPulse {
  0%,
  100% {
    opacity: 0.3;
    transform: scale(1);
  }
  50% {
    opacity: 0.6;
    transform: scale(1.15);
  }
}

.operator-glow {
  position: absolute;
  inset: -10px;
  background: radial-gradient(circle, #649affff 0%, transparent 70%);
  opacity: 0.3;
  animation: operatorPulse 2.5s ease-in-out infinite;
  pointer-events: none;
  border-radius: 50%;
}

.node-title {
  font-weight: 600;
  font-size: 1.05rem;
  color: var(--vp-c-text-1);
  text-align: center;
  white-space: nowrap;
}

/* Resource nodes */
.resource-nodes {
  position: absolute;
  right: 3%;
  top: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: space-around;
  padding: 2rem 0;
  z-index: 10;
}

@keyframes resourceSlideIn {
  0% {
    opacity: 0;
    transform: translateY(-50%) translateX(-40px);
  }
  100% {
    opacity: 1;
    transform: translateY(-50%) translateX(0);
  }
}

.resource-node {
  position: absolute;
  left: -120px;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  animation: resourceSlideIn 0.8s ease-out backwards;
  transition: all 0.3s ease;
}

.resource-node:hover {
  transform: translateY(-50%) translateX(8px);
}

.resource-label {
  padding: 0.5rem 1rem;
  background: linear-gradient(
    135deg,
    var(--vp-c-bg) 0%,
    var(--vp-c-bg-soft) 100%
  );
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--vp-c-text-2);
  white-space: nowrap;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
}

.resource-node:hover .resource-label {
  border-color: var(--vp-c-brand);
  color: var(--vp-c-brand);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

/* Responsive design */
@media (max-width: 968px) {
  .operator-diagram {
    padding: 2rem;
  }

  .diagram-container {
    height: 380px;
  }

  .resource-label {
    padding: 0.4rem 0.8rem;
    font-size: 0.8rem;
  }

  .node-icon svg {
    width: 60px !important;
    height: 60px !important;
  }

  .operator-logo {
    width: 75px !important;
    height: 75px !important;
  }
}

@media (max-width: 768px) {
  .diagram-container {
    height: 320px;
  }

  .node-icon svg {
    width: 50px !important;
    height: 50px !important;
  }

  .operator-logo {
    width: 65px !important;
    height: 65px !important;
  }

  .node-title {
    font-size: 0.95rem;
  }

  .resource-nodes {
    right: -5%;
    transform: scale(0.9);
  }

  .resource-label {
    padding: 0.35rem 0.65rem;
    font-size: 0.75rem;
  }
}

@media (max-width: 640px) {
  .operator-diagram {
    padding: 1.5rem;
  }

  .diagram-container {
    height: 280px;
  }

  .node-icon svg {
    width: 45px !important;
    height: 45px !important;
  }

  .operator-logo {
    width: 55px !important;
    height: 55px !important;
  }

  .node-title {
    font-size: 0.85rem;
  }

  .resource-nodes {
    right: -5%;
    transform: scale(0.45);
  }
}
</style>
