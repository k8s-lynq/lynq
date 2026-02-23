<template>
  <section class="safety-section">
    <div class="grid-background"></div>
    <div class="container">
      <div class="section-header fade-up">
        <span class="section-label">Safety First</span>
        <h2>Control at Every Step</h2>
        <p class="section-subtitle">Built-in policies prevent disasters before they happen</p>
      </div>

      <div class="policies-grid">
        <!-- Conflict Policy -->
        <div
          class="policy-card fade-up"
          style="animation-delay: 0.1s"
          @mouseenter="activePolicy = 'conflict'"
          @mouseleave="activePolicy = null"
        >
          <div class="policy-visual">
            <svg viewBox="0 0 160 100" preserveAspectRatio="xMidYMid meet" class="policy-svg">
              <defs>
                <marker id="ah-red" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
                  <polygon points="0 0, 6 3, 0 6" fill="#ef4444" />
                </marker>
                <marker id="ah-blue" markerWidth="6" markerHeight="6" refX="1" refY="3" orient="auto">
                  <polygon points="6 0, 0 3, 6 6" fill="#667eea" />
                </marker>
              </defs>
              <!-- Shield centered (narrowed: x 60–100) -->
              <path
                class="shield"
                d="M80 16 L100 28 L100 52 Q100 68 80 74 Q60 68 60 52 L60 28 Z"
                fill="none"
                stroke="#ef4444"
                stroke-width="2"
                :class="{ active: activePolicy === 'conflict' }"
              />
              <!-- Left arrow (stops 20px before shield left edge at x=60) -->
              <g :class="{ active: activePolicy === 'conflict' }">
                <line
                  class="arrow arrow-left"
                  x1="6" y1="45" x2="34" y2="45"
                  stroke="#ef4444" stroke-width="2.5"
                  marker-end="url(#ah-red)"
                />
                <!-- Right arrow (stops 28px after shield right edge at x=100) -->
                <line
                  class="arrow arrow-right"
                  x1="154" y1="45" x2="134" y2="45"
                  stroke="#667eea" stroke-width="2.5"
                  marker-end="url(#ah-blue)"
                />
              </g>
              <!-- X mark when blocked -->
              <g class="block-mark" :class="{ active: activePolicy === 'conflict' }">
                <line x1="72" y1="37" x2="88" y2="53" stroke="#10b981" stroke-width="3" stroke-linecap="round" />
                <line x1="88" y1="37" x2="72" y2="53" stroke="#10b981" stroke-width="3" stroke-linecap="round" />
              </g>
              <!-- Labels -->
              <text x="22" y="38" text-anchor="middle" fill="#ef4444" font-size="8" opacity="0.6">Owner A</text>
              <text x="140" y="38" text-anchor="middle" fill="#667eea" font-size="8" opacity="0.6">Owner B</text>
            </svg>
          </div>
          <div class="policy-content">
            <div class="policy-tag">conflictPolicy</div>
            <h3>Conflict Detection</h3>
            <p>Stops reconciliation if another controller already owns the resource. No silent overwrites.</p>
          </div>
          <div class="policy-code">
            <code>conflictPolicy: Stuck</code>
          </div>
        </div>

        <!-- MaxSkew Policy -->
        <div
          class="policy-card fade-up"
          style="animation-delay: 0.2s"
          @mouseenter="activePolicy = 'maxskew'"
          @mouseleave="activePolicy = null"
        >
          <div class="policy-visual">
            <svg viewBox="0 0 160 100" preserveAspectRatio="xMidYMid meet" class="policy-svg">
              <!-- 5 nodes evenly spaced -->
              <g class="nodes-row">
                <rect
                  v-for="i in 5"
                  :key="'node-' + i"
                  :x="14 + (i - 1) * 28"
                  y="20"
                  width="22"
                  height="30"
                  rx="4"
                  :class="['node', { updating: activePolicy === 'maxskew' && i <= 2 }]"
                />
                <!-- Progress bar inside updating nodes -->
                <rect
                  v-for="i in 2"
                  :key="'bar-' + i"
                  :x="17 + (i - 1) * 28"
                  y="40"
                  width="16"
                  height="3"
                  rx="1.5"
                  fill="#667eea"
                  :opacity="activePolicy === 'maxskew' ? 0.8 : 0"
                  style="transition: opacity 0.3s ease"
                />
              </g>
              <!-- Wave line spanning full width -->
              <path
                class="wave"
                d="M10 68 Q30 56, 50 68 T90 68 T130 68 T150 68"
                fill="none"
                stroke="#667eea"
                stroke-width="2"
                :class="{ active: activePolicy === 'maxskew' }"
              />
              <!-- Bracket showing "max 2" -->
              <path d="M14 14 L14 10 L62 10 L62 14" fill="none" stroke="#667eea" stroke-width="1" opacity="0.5" />
              <text x="38" y="8" text-anchor="middle" fill="rgba(255,255,255,0.5)" font-size="7">max 2</text>
              <!-- Label -->
              <text x="80" y="88" text-anchor="middle" fill="rgba(255,255,255,0.4)" font-size="8">
                at a time
              </text>
            </svg>
          </div>
          <div class="policy-content">
            <div class="policy-tag">maxSkew</div>
            <h3>Gradual Rollout</h3>
            <p>Limit concurrent updates. Change 500 resources safely, a few at a time.</p>
          </div>
          <div class="policy-code">
            <code>maxSkew: 2</code>
          </div>
        </div>

        <!-- Deletion Policy -->
        <div
          class="policy-card fade-up"
          style="animation-delay: 0.3s"
          @mouseenter="activePolicy = 'deletion'"
          @mouseleave="activePolicy = null"
        >
          <div class="policy-visual">
            <svg viewBox="0 0 160 100" preserveAspectRatio="xMidYMid meet" class="policy-svg">
              <!-- Data cylinder centered -->
              <g class="data-cylinder" :class="{ active: activePolicy === 'deletion' }">
                <ellipse cx="80" cy="24" rx="34" ry="10" fill="none" stroke="#10b981" stroke-width="2" />
                <path d="M46 24 L46 60 Q46 70 80 70 Q114 70 114 60 L114 24" fill="none" stroke="#10b981" stroke-width="2" />
                <ellipse cx="80" cy="60" rx="34" ry="10" fill="none" stroke="#10b981" stroke-width="2" />
                <!-- Data rows inside cylinder -->
                <line x1="58" y1="36" x2="102" y2="36" stroke="#10b981" stroke-width="0.8" opacity="0.3" />
                <line x1="58" y1="44" x2="102" y2="44" stroke="#10b981" stroke-width="0.8" opacity="0.3" />
                <line x1="58" y1="52" x2="102" y2="52" stroke="#10b981" stroke-width="0.8" opacity="0.3" />
              </g>
              <!-- Shield overlay on hover -->
              <g class="shield-overlay" :class="{ active: activePolicy === 'deletion' }">
                <path
                  d="M80 16 L106 26 L106 54 Q106 68 80 72 Q54 68 54 54 L54 26 Z"
                  fill="rgba(16, 185, 129, 0.12)"
                  stroke="#10b981"
                  stroke-width="1.5"
                />
                <text x="80" y="52" text-anchor="middle" fill="#10b981" font-size="20" font-weight="bold">&#10003;</text>
              </g>
              <!-- Label -->
              <text x="80" y="90" text-anchor="middle" fill="rgba(255,255,255,0.4)" font-size="8">
                data protected
              </text>
            </svg>
          </div>
          <div class="policy-content">
            <div class="policy-tag">deletionPolicy</div>
            <h3>Data Protection</h3>
            <p>Keep critical resources (like PVCs) even when the source record is deleted.</p>
          </div>
          <div class="policy-code">
            <code>deletionPolicy: Retain</code>
          </div>
        </div>
      </div>

      <div class="safety-cta fade-up" style="animation-delay: 0.4s">
        <a href="/policies" class="learn-more">
          Learn more about policies
          <span class="arrow">&#8594;</span>
        </a>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref } from 'vue'

const activePolicy = ref(null)
</script>

<style scoped>
.safety-section {
  padding: 6rem 2rem;
  background: linear-gradient(180deg, #111118 0%, #0a0a0f 100%);
  position: relative;
  overflow: hidden;
}

.grid-background {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(102, 126, 234, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(102, 126, 234, 0.03) 1px, transparent 1px);
  background-size: 60px 60px;
  mask-image: radial-gradient(ellipse at center, black 20%, transparent 70%);
  pointer-events: none;
}

.container {
  max-width: 1200px;
  margin: 0 auto;
  position: relative;
}

.section-header {
  text-align: center;
  margin-bottom: 3rem;
}

.section-label {
  display: inline-block;
  font-size: 0.875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #ef4444;
  margin-bottom: 0.75rem;
}

.section-header h2 {
  font-size: clamp(2rem, 4vw, 3rem);
  font-weight: 700;
  color: #fff;
  margin: 0 0 1rem;
}

.section-subtitle {
  font-size: 1.1rem;
  color: rgba(255, 255, 255, 0.6);
  margin: 0;
}

.policies-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1.5rem;
  margin-bottom: 3rem;
}

.policy-card {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 16px;
  padding: 1.5rem;
  transition: all 0.3s ease;
  cursor: default;
}

.policy-card:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
  transform: translateY(-4px);
}

.policy-visual {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 1.25rem;
}

.policy-svg {
  width: 100%;
  max-width: 240px;
  height: auto;
}

/* Conflict Animation */
.shield {
  transition: all 0.4s ease;
  opacity: 0.5;
}

.shield.active {
  opacity: 1;
  stroke: #10b981;
  animation: shieldPulse 1s ease infinite;
}

g .arrow {
  opacity: 0.5;
  transition: all 0.3s ease;
}

g.active .arrow {
  opacity: 1;
}

g.active .arrow-left {
  animation: arrowPushLeft 1s ease infinite;
}

g.active .arrow-right {
  animation: arrowPushRight 1s ease infinite;
}

.block-mark {
  opacity: 0;
  transition: opacity 0.3s ease 0.3s;
}

.block-mark.active {
  opacity: 1;
}

@keyframes shieldPulse {
  0%, 100% { filter: drop-shadow(0 0 3px #10b981); }
  50% { filter: drop-shadow(0 0 8px #10b981); }
}

@keyframes arrowPushLeft {
  0%, 100% { transform: translateX(0); }
  50% { transform: translateX(5px); }
}

@keyframes arrowPushRight {
  0%, 100% { transform: translateX(0); }
  50% { transform: translateX(-5px); }
}

/* MaxSkew Animation */
.node {
  fill: rgba(102, 126, 234, 0.4);
  stroke: #667eea;
  stroke-width: 1.5;
  transition: all 0.3s ease;
}

.node.updating {
  fill: #667eea;
  animation: nodeUpdate 0.8s ease infinite;
}

.wave {
  opacity: 0.4;
  transition: opacity 0.3s ease;
}

.wave.active {
  opacity: 1;
  stroke-dasharray: 200;
  stroke-dashoffset: 200;
  animation: waveFlow 2s linear infinite;
}

@keyframes nodeUpdate {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

@keyframes waveFlow {
  to { stroke-dashoffset: 0; }
}

/* Deletion Animation */
.data-cylinder {
  opacity: 0.5;
  transform: scale(1);
  transform-origin: 80px 45px; /* center of cylinder */
  transition: all 0.4s ease;
}

.data-cylinder.active {
  opacity: 0.25;
  transform: scale(0.88);
}

.shield-overlay {
  opacity: 0;
  transition: all 0.4s ease;
  transform-origin: center;
}

.shield-overlay.active {
  opacity: 1;
  animation: shieldAppear 0.5s ease;
}

@keyframes shieldAppear {
  0% { transform: scale(0.8); opacity: 0; }
  100% { transform: scale(1); opacity: 1; }
}

.policy-content {
  margin-bottom: 1rem;
}

.policy-tag {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  background: rgba(102, 126, 234, 0.15);
  border-radius: 100px;
  font-size: 0.75rem;
  font-family: monospace;
  color: #a78bfa;
  margin-bottom: 0.75rem;
}

.policy-content h3 {
  font-size: 1.25rem;
  font-weight: 600;
  color: #fff;
  margin: 0 0 0.5rem;
}

.policy-content p {
  font-size: 0.95rem;
  color: rgba(255, 255, 255, 0.6);
  line-height: 1.5;
  margin: 0;
}

.policy-code {
  padding: 0.75rem 1rem;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.policy-code code {
  font-size: 0.85rem;
  color: #10b981;
  font-family: 'SF Mono', Monaco, monospace;
}

.safety-cta {
  text-align: center;
}

.learn-more {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1rem;
  font-weight: 500;
  color: #667eea;
  text-decoration: none;
  transition: all 0.3s ease;
}

.learn-more:hover {
  color: #a78bfa;
}

.learn-more .arrow {
  transition: transform 0.3s ease;
}

.learn-more:hover .arrow {
  transform: translateX(4px);
}

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
  .policies-grid {
    grid-template-columns: 1fr;
  }
}
</style>
