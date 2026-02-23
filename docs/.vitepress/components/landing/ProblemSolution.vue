<template>
  <section class="problem-solution">
    <div class="container">
      <div class="section-header fade-up">
        <span class="section-label">The Challenge</span>
        <h2>Scale Without the Pain</h2>
      </div>

      <div class="split-container">
        <!-- Problem Side -->
        <div class="side problem-side fade-left" style="animation-delay: 0.2s">
          <div class="side-header">
            <span class="side-icon problem-icon">&#10060;</span>
            <h3>The Old Way</h3>
          </div>

          <div class="visual-wrapper" ref="chaosRef">
            <svg viewBox="0 0 280 180" preserveAspectRatio="xMidYMid meet" class="visual-svg">
              <defs>
                <linearGradient id="chaosGrad" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="280" y2="180">
                  <stop offset="0%" stop-color="#ef4444" stop-opacity="0.5" />
                  <stop offset="100%" stop-color="#dc2626" stop-opacity="0.25" />
                </linearGradient>
              </defs>
              <!-- Tangled connections between resources -->
              <path
                v-for="(edge, i) in chaosEdges"
                :key="'ce-' + i"
                :d="edge.d"
                fill="none"
                stroke="url(#chaosGrad)"
                stroke-width="1.5"
                :stroke-dasharray="edge.len"
                :stroke-dashoffset="chaosAnimated ? 0 : edge.len"
                :style="{ transition: `stroke-dashoffset ${0.8 + i * 0.15}s ease ${i * 0.08}s` }"
              />
              <!-- Resource nodes -->
              <g v-for="(node, i) in chaosNodes" :key="'cn-' + i">
                <rect
                  :x="node.x - node.w / 2"
                  :y="node.y - node.h / 2"
                  :width="node.w"
                  :height="node.h"
                  rx="4"
                  fill="#1a1a2e"
                  stroke="#ef4444"
                  stroke-width="1"
                  :opacity="chaosAnimated ? 0.9 : 0"
                  :style="{ transition: `opacity 0.4s ease ${0.3 + i * 0.06}s` }"
                />
                <text
                  :x="node.x"
                  :y="node.y + 3.5"
                  text-anchor="middle"
                  fill="#ef4444"
                  font-size="9"
                  font-weight="600"
                  font-family="'SF Mono', Monaco, monospace"
                  :opacity="chaosAnimated ? 0.8 : 0"
                  :style="{ transition: `opacity 0.4s ease ${0.4 + i * 0.06}s` }"
                >{{ node.label }}</text>
              </g>
            </svg>
          </div>

          <ul class="points">
            <li>
              <span class="point-icon">&#128736;</span>
              Manual kubectl for each tenant
            </li>
            <li>
              <span class="point-icon">&#128221;</span>
              Git commits for every change
            </li>
            <li>
              <span class="point-icon">&#128165;</span>
              Constant drift between DB and cluster
            </li>
          </ul>
        </div>

        <!-- Divider -->
        <div class="divider">
          <div class="divider-line"></div>
          <div class="divider-badge">vs</div>
          <div class="divider-line"></div>
        </div>

        <!-- Solution Side -->
        <div class="side solution-side fade-right" style="animation-delay: 0.4s">
          <div class="side-header">
            <span class="side-icon solution-icon">&#10004;</span>
            <h3>With Lynq</h3>
          </div>

          <div class="visual-wrapper" ref="orderRef">
            <svg viewBox="0 0 340 180" preserveAspectRatio="xMidYMid meet" class="visual-svg">
              <defs>
                <linearGradient id="orderGrad" gradientUnits="userSpaceOnUse" x1="45" y1="0" x2="290" y2="0">
                  <stop offset="0%" stop-color="#667eea" stop-opacity="0.8" />
                  <stop offset="100%" stop-color="#10b981" stop-opacity="0.8" />
                </linearGradient>
                <marker id="orderArrow" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
                  <polygon points="0 0, 6 3, 0 6" fill="#10b981" opacity="0.7" />
                </marker>
              </defs>

              <!-- Connection lines -->
              <line
                v-for="(line, i) in orderLines"
                :key="'ol-' + i"
                :x1="line.x1" :y1="line.y1" :x2="line.x2" :y2="line.y2"
                stroke="url(#orderGrad)"
                stroke-width="1.5"
                :marker-end="line.arrow ? 'url(#orderArrow)' : ''"
                :stroke-dasharray="line.len"
                :stroke-dashoffset="orderAnimated ? 0 : line.len"
                :style="{ transition: `stroke-dashoffset ${0.6 + i * 0.1}s ease ${i * 0.08}s` }"
              />

              <!-- Pipeline nodes (source + Lynq) -->
              <g v-for="(node, i) in orderNodes" :key="'on-' + i">
                <rect
                  :x="node.x - node.w / 2"
                  :y="node.y - node.h / 2"
                  :width="node.w"
                  :height="node.h"
                  :rx="node.rx"
                  fill="#1a1a2e"
                  :stroke="node.color"
                  :stroke-width="node.highlight ? 2 : 1.5"
                  :opacity="orderAnimated ? 1 : 0"
                  :style="{ transition: `opacity 0.4s ease ${0.3 + i * 0.06}s` }"
                />
                <text
                  :x="node.x"
                  :y="node.y + 3.5"
                  text-anchor="middle"
                  :fill="node.color"
                  :font-size="node.highlight ? 11 : 9"
                  font-weight="600"
                  font-family="'SF Mono', Monaco, monospace"
                  :opacity="orderAnimated ? 1 : 0"
                  :style="{ transition: `opacity 0.4s ease ${0.35 + i * 0.06}s` }"
                >{{ node.label }}</text>
              </g>
            </svg>
          </div>

          <ul class="points">
            <li>
              <span class="point-icon success">&#128196;</span>
              Define once in LynqForm
            </li>
            <li>
              <span class="point-icon success">&#9889;</span>
              Data drives provisioning
            </li>
            <li>
              <span class="point-icon success">&#128260;</span>
              Automatic sync & cleanup
            </li>
          </ul>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const chaosRef = ref(null)
const orderRef = ref(null)
const chaosAnimated = ref(false)
const orderAnimated = ref(false)

// ── Shared helpers ──
function dist(x1, y1, x2, y2) {
  // Add buffer to ensure stroke-dashoffset animation always covers the full line
  return Math.ceil(Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2)) + 2
}

// ── Chaos side ──
// K8s resource nodes scattered across the viewBox
const chaosNodes = [
  { id: 'deploy', x: 60,  y: 40,  w: 44, h: 18, label: 'deploy' },
  { id: 'svc',    x: 200, y: 35,  w: 30, h: 18, label: 'svc' },
  { id: 'ing',    x: 140, y: 30,  w: 28, h: 18, label: 'ing' },
  { id: 'ns',     x: 230, y: 90,  w: 26, h: 18, label: 'ns' },
  { id: 'cm',     x: 50,  y: 110, w: 28, h: 18, label: 'cm' },
  { id: 'pvc',    x: 160, y: 100, w: 30, h: 18, label: 'pvc' },
  { id: 'sa',     x: 100, y: 150, w: 26, h: 18, label: 'sa' },
  { id: 'secret', x: 220, y: 150, w: 40, h: 18, label: 'secret' },
]

// Connections: [fromId, toId, curveOffset] — offset bows the line to create crossings
const chaosConnections = [
  ['deploy', 'svc',    { cx: 0, cy: -20 }],
  ['deploy', 'pvc',    { cx: 30, cy: 10 }],
  ['svc',    'ns',     { cx: 15, cy: 15 }],
  ['ing',    'sa',     { cx: -40, cy: 20 }],  // crosses deploy→pvc
  ['cm',     'svc',    { cx: 20, cy: -40 }],  // crosses deploy→svc
  ['ns',     'cm',     { cx: 0, cy: -30 }],   // crosses multiple
  ['pvc',    'secret', { cx: -10, cy: 30 }],
  ['sa',     'ns',     { cx: 30, cy: -20 }],  // crosses pvc→secret
  ['deploy', 'secret', { cx: 40, cy: 50 }],
  ['ing',    'ns',     { cx: 20, cy: 30 }],
]

function chaosNodeById(id) {
  return chaosNodes.find((n) => n.id === id)
}

// Derive bezier curve paths from node centers + curve offset
const chaosEdges = chaosConnections.map(([fromId, toId, offset]) => {
  const a = chaosNodeById(fromId)
  const b = chaosNodeById(toId)
  const mx = (a.x + b.x) / 2 + offset.cx
  const my = (a.y + b.y) / 2 + offset.cy
  const d1 = dist(a.x, a.y, mx, my)
  const d2 = dist(mx, my, b.x, b.y)
  return {
    d: `M ${a.x},${a.y} Q ${mx},${my} ${b.x},${b.y}`,
    len: d1 + d2,
  }
})

// ── Order side ──
// Layout: DB/Row/Data → Lynq (hub) → deploy/svc/ing (organized output)
// viewBox: 340 x 180
const orderNodes = [
  // Source data (left)
  { id: 'db',     x: 45,  y: 35,  w: 36, h: 18, rx: 4, label: 'DB',     color: '#667eea' },
  { id: 'row',    x: 45,  y: 90,  w: 36, h: 18, rx: 4, label: 'Row',    color: '#667eea' },
  { id: 'data',   x: 45,  y: 145, w: 36, h: 18, rx: 4, label: 'Data',   color: '#667eea' },
  // Lynq operator (center, highlighted)
  { id: 'lynq',   x: 145, y: 90,  w: 48, h: 24, rx: 6, label: 'Lynq',   color: '#10b981', highlight: true },
  // Output K8s resources (right, neatly fanned out)
  { id: 'deploy', x: 265, y: 35,  w: 44, h: 18, rx: 4, label: 'deploy', color: '#10b981' },
  { id: 'svc',    x: 265, y: 90,  w: 30, h: 18, rx: 4, label: 'svc',    color: '#10b981' },
  { id: 'ing',    x: 265, y: 145, w: 28, h: 18, rx: 4, label: 'ing',    color: '#10b981' },
]

function orderNodeById(id) {
  return orderNodes.find((n) => n.id === id)
}

function rightEdge(node) {
  return { x: node.x + node.w / 2, y: node.y }
}

function leftEdge(node) {
  return { x: node.x - node.w / 2, y: node.y }
}

// Connections: [fromId, toId, hasArrow]
// Input lines (→Lynq) have no arrow to avoid clutter at convergence point
// Output lines (Lynq→) have arrows to show direction
const orderConnections = [
  ['db', 'lynq', false],
  ['row', 'lynq', false],
  ['data', 'lynq', false],
  ['lynq', 'deploy', true],
  ['lynq', 'svc', true],
  ['lynq', 'ing', true],
]

const arrowGap = 4

const orderLines = orderConnections.map(([fromId, toId, hasArrow]) => {
  const from = rightEdge(orderNodeById(fromId))
  const to = leftEdge(orderNodeById(toId))
  const dx = to.x - from.x
  const dy = to.y - from.y
  const d = Math.sqrt(dx * dx + dy * dy)
  const gap = hasArrow ? arrowGap : 0
  const ratio = d > 0 ? (d - gap) / d : 1
  const x2 = from.x + dx * ratio
  const y2 = from.y + dy * ratio
  return {
    x1: from.x,
    y1: from.y,
    x2,
    y2,
    arrow: hasArrow,
    len: dist(from.x, from.y, x2, y2),
  }
})

// ── Intersection observers ──
let chaosObserver = null
let orderObserver = null

onMounted(() => {
  const opts = { threshold: 0.3 }

  if (chaosRef.value) {
    chaosObserver = new IntersectionObserver((entries) => {
      entries.forEach((e) => { if (e.isIntersecting) chaosAnimated.value = true })
    }, opts)
    chaosObserver.observe(chaosRef.value)
  }

  if (orderRef.value) {
    orderObserver = new IntersectionObserver((entries) => {
      entries.forEach((e) => { if (e.isIntersecting) orderAnimated.value = true })
    }, opts)
    orderObserver.observe(orderRef.value)
  }
})

onUnmounted(() => {
  if (chaosObserver) chaosObserver.disconnect()
  if (orderObserver) orderObserver.disconnect()
})
</script>

<style scoped>
.problem-solution {
  padding: 6rem 2rem;
  background: linear-gradient(180deg, #111118 0%, #0d0d12 100%);
}

.container {
  max-width: 1200px;
  margin: 0 auto;
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
  color: #667eea;
  margin-bottom: 0.75rem;
}

.section-header h2 {
  font-size: clamp(2rem, 4vw, 3rem);
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.split-container {
  display: flex;
  align-items: stretch;
  gap: 2rem;
}

.side {
  flex: 1;
  padding: 2rem;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.problem-side {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.05) 0%, transparent 100%);
  border-color: rgba(239, 68, 68, 0.15);
}

.solution-side {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.05) 0%, transparent 100%);
  border-color: rgba(16, 185, 129, 0.15);
}

.side-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
}

.side-icon {
  font-size: 1.25rem;
}

.problem-icon {
  color: #ef4444;
}

.solution-icon {
  color: #10b981;
}

.side-header h3 {
  font-size: 1.25rem;
  font-weight: 600;
  color: #fff;
  margin: 0;
}

.visual-wrapper {
  margin-bottom: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.visual-svg {
  width: 100%;
  max-width: 340px;
  height: auto;
}

.points {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.points li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.95rem;
  color: rgba(255, 255, 255, 0.8);
}

.point-icon {
  font-size: 1rem;
  opacity: 0.7;
}

.point-icon.success {
  color: #10b981;
  opacity: 1;
}

.divider {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 0 0.5rem;
  flex-shrink: 0;
}

.divider-line {
  width: 2px;
  flex: 1;
  background: linear-gradient(180deg, transparent, rgba(255, 255, 255, 0.15), transparent);
}

.divider-badge {
  font-size: 0.75rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.5);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.03);
}

@keyframes fadeUp {
  from { opacity: 0; transform: translateY(30px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes fadeLeft {
  from { opacity: 0; transform: translateX(-40px); }
  to { opacity: 1; transform: translateX(0); }
}

@keyframes fadeRight {
  from { opacity: 0; transform: translateX(40px); }
  to { opacity: 1; transform: translateX(0); }
}

.fade-up {
  animation: fadeUp 0.6s ease both;
}

.fade-left {
  animation: fadeLeft 0.6s ease both;
}

.fade-right {
  animation: fadeRight 0.6s ease both;
}

@media (max-width: 900px) {
  .split-container {
    flex-direction: column;
  }

  .divider {
    flex-direction: row;
    padding: 0.75rem 0;
  }

  .divider-line {
    width: auto;
    height: 2px;
    flex: 1;
    background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.15), transparent);
  }
}
</style>
