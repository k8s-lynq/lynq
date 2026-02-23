<template>
  <div class="hero-container" ref="containerRef">
    <canvas ref="canvasRef" class="particle-canvas"></canvas>
    <div class="particle-overlay" ref="overlayRef"></div>

    <div class="hero-content">
      <motion.div
        class="badge"
        :initial="{ opacity: 0, y: 20 }"
        :animate="{ opacity: 1, y: 0 }"
        :transition="{ duration: 0.6, delay: 0.2 }"
      >
        <span class="badge-icon">&#9670;</span>
        Infrastructure as Data
      </motion.div>

      <motion.h1
        class="hero-title"
        :initial="{ opacity: 0, y: 30 }"
        :animate="{ opacity: 1, y: 0 }"
        :transition="{ duration: 0.7, delay: 0.4 }"
      >
        Your Database.<br/>
        <span class="gradient-text">Your Infrastructure.</span>
      </motion.h1>

      <motion.p
        class="hero-subtitle"
        :initial="{ opacity: 0, y: 20 }"
        :animate="{ opacity: 1, y: 0 }"
        :transition="{ duration: 0.6, delay: 0.6 }"
      >
        Lynq turns database records into Kubernetes resources. Automatically.
      </motion.p>

      <motion.div
        class="hero-ctas"
        :initial="{ opacity: 0, y: 20 }"
        :animate="{ opacity: 1, y: 0 }"
        :transition="{ duration: 0.6, delay: 0.8 }"
      >
        <a href="/quickstart" class="cta-primary">
          Get Started
          <span class="arrow">&#8594;</span>
        </a>
        <a href="/blog/introducing-lynq-dashboard" class="cta-secondary">
          <span class="play-icon">&#9658;</span>
          Watch Demo
        </a>
      </motion.div>
    </div>

    <div class="flow-labels">
      <motion.div
        class="label label-left"
        :initial="{ opacity: 0, x: -20 }"
        :animate="{ opacity: 1, x: 0 }"
        :transition="{ duration: 0.6, delay: 1.0 }"
      >
        <div class="label-icon">&#128451;</div>
        <span>Database</span>
      </motion.div>
      <motion.div
        class="label label-center"
        :initial="{ opacity: 0, scale: 0.8 }"
        :animate="{ opacity: 1, scale: 1 }"
        :transition="{ duration: 0.6, delay: 1.2 }"
      >
        <img src="/logo.png" alt="Lynq" class="lynq-logo" />
      </motion.div>
      <motion.div
        class="label label-right"
        :initial="{ opacity: 0, x: 20 }"
        :animate="{ opacity: 1, x: 0 }"
        :transition="{ duration: 0.6, delay: 1.0 }"
      >
        <div class="label-icon">&#9096;</div>
        <span>Kubernetes</span>
      </motion.div>
    </div>

    <div class="scroll-indicator">
      <motion.div
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :transition="{ duration: 0.6, delay: 1.5 }"
      >
        <span>Scroll to explore</span>
        <div class="scroll-arrow">&#8595;</div>
      </motion.div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { motion } from 'motion-v'
import * as THREE from 'three'

const containerRef = ref(null)
const canvasRef = ref(null)
const overlayRef = ref(null)

let scene, camera, renderer, particles
let mouseX = 0, mouseY = 0
let animationId = null

const PARTICLE_COUNT = 800
const COLORS = {
  database: new THREE.Color(0x667eea),  // Purple - database side
  lynq: new THREE.Color(0x10b981),      // Green - Lynq center
  k8s: new THREE.Color(0x3b82f6)        // Blue - Kubernetes side
}

onMounted(() => {
  if (!canvasRef.value || !containerRef.value) return

  initThree()
  createParticles()
  animate()

  // Set default spotlight position to center
  if (overlayRef.value) {
    overlayRef.value.style.setProperty('--mx', '50%')
    overlayRef.value.style.setProperty('--my', '50%')
  }

  window.addEventListener('resize', onResize)
  window.addEventListener('mousemove', onMouseMove)
})

onUnmounted(() => {
  if (animationId) cancelAnimationFrame(animationId)
  window.removeEventListener('resize', onResize)
  window.removeEventListener('mousemove', onMouseMove)

  if (renderer) {
    renderer.dispose()
  }
})

function initThree() {
  const container = containerRef.value
  const canvas = canvasRef.value

  scene = new THREE.Scene()

  camera = new THREE.PerspectiveCamera(
    75,
    container.clientWidth / container.clientHeight,
    0.1,
    1000
  )
  camera.position.z = 50

  renderer = new THREE.WebGLRenderer({
    canvas,
    alpha: true,
    antialias: true
  })
  renderer.setClearColor(0x000000, 0)
  renderer.setSize(container.clientWidth, container.clientHeight)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
}

// Color zones: database (left) → lynq glow (center) → k8s (right)
// Transition happens in a narrow band around x=0
function getColorForX(x) {
  const t = (x + 80) / 160 // normalize to 0–1
  if (t < 0.4) {
    return COLORS.database.clone()
  } else if (t < 0.5) {
    // Transition: database → lynq
    return COLORS.database.clone().lerp(COLORS.lynq, (t - 0.4) * 10)
  } else if (t < 0.6) {
    // Transition: lynq → k8s
    return COLORS.lynq.clone().lerp(COLORS.k8s, (t - 0.5) * 10)
  } else {
    return COLORS.k8s.clone()
  }
}

function createParticles() {
  const geometry = new THREE.BufferGeometry()
  const positions = new Float32Array(PARTICLE_COUNT * 3)
  const colors = new Float32Array(PARTICLE_COUNT * 3)
  const sizes = new Float32Array(PARTICLE_COUNT)
  const velocities = new Float32Array(PARTICLE_COUNT * 3)
  const phases = new Float32Array(PARTICLE_COUNT)

  for (let i = 0; i < PARTICLE_COUNT; i++) {
    const i3 = i * 3

    // Start particles on the left side and spread across
    positions[i3] = (Math.random() - 0.5) * 120 - 20  // x: spread from left
    positions[i3 + 1] = (Math.random() - 0.5) * 60     // y: vertical spread
    positions[i3 + 2] = (Math.random() - 0.5) * 30     // z: depth

    // Color based on x position — sharp transition at center
    const color = getColorForX(positions[i3])

    colors[i3] = color.r
    colors[i3 + 1] = color.g
    colors[i3 + 2] = color.b

    // Size variation
    sizes[i] = Math.random() * 2 + 0.5

    // Velocity for flowing animation
    velocities[i3] = 0.1 + Math.random() * 0.15      // x: rightward flow
    velocities[i3 + 1] = (Math.random() - 0.5) * 0.02 // y: slight drift
    velocities[i3 + 2] = (Math.random() - 0.5) * 0.02 // z: slight drift

    // Phase for wave animation
    phases[i] = Math.random() * Math.PI * 2
  }

  geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3))
  geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3))
  geometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1))

  // Store velocities and phases for animation
  geometry.userData = { velocities, phases }

  const material = new THREE.ShaderMaterial({
    uniforms: {
      time: { value: 0 },
      mousePos: { value: new THREE.Vector2(0, 0) }
    },
    vertexShader: `
      attribute float size;
      attribute vec3 color;
      varying vec3 vColor;
      varying float vMorph;  // 0 = circle (data), 1 = square (K8s resource)
      uniform float time;
      uniform vec2 mousePos;

      void main() {
        vColor = color;

        vec3 pos = position;

        // Morph factor: left of center = circle, right = square
        // Transition zone: x -10 to +10 (center of the 160-wide spread)
        vMorph = smoothstep(-10.0, 10.0, pos.x);

        // Particles grow slightly in the center transition zone then settle
        float centerDist = abs(pos.x) / 10.0;
        float centerBoost = 1.0 + 0.4 * exp(-centerDist * centerDist);
        // Right-side particles are slightly larger (K8s resources)
        float sizeScale = mix(1.0, 1.3, vMorph) * centerBoost;

        // Wave motion
        pos.y += sin(pos.x * 0.05 + time) * 2.0;
        pos.z += cos(pos.x * 0.03 + time * 0.7) * 1.5;

        // Mouse influence
        float dist = length(vec2(pos.x, pos.y) - mousePos * 50.0);
        float influence = smoothstep(30.0, 0.0, dist);
        pos.y += influence * 5.0;
        pos.z += influence * 3.0;

        vec4 mvPosition = modelViewMatrix * vec4(pos, 1.0);
        gl_PointSize = size * sizeScale * (300.0 / -mvPosition.z);
        gl_Position = projectionMatrix * mvPosition;
      }
    `,
    fragmentShader: `
      varying vec3 vColor;
      varying float vMorph;

      void main() {
        vec2 uv = gl_PointCoord - vec2(0.5);

        // Circle SDF (data dot)
        float circleDist = length(uv);

        // Rounded square SDF (K8s resource block)
        vec2 d = abs(uv) - vec2(0.35);
        float squareDist = length(max(d, 0.0)) + min(max(d.x, d.y), 0.0) - 0.08;

        // Morph between shapes
        float dist = mix(circleDist, squareDist + 0.15, vMorph);

        if (dist > 0.5) discard;

        float alpha = 1.0 - smoothstep(0.3, 0.5, dist);
        alpha *= 0.8;

        // Center glow boost for particles in transition
        float transitionGlow = 0.3 * exp(-16.0 * (vMorph - 0.5) * (vMorph - 0.5));
        vec3 glow = vColor * (1.0 - dist * 2.0);

        gl_FragColor = vec4(vColor + glow * (0.3 + transitionGlow), alpha + transitionGlow * 0.3);
      }
    `,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending
  })

  particles = new THREE.Points(geometry, material)
  scene.add(particles)
}

function animate() {
  animationId = requestAnimationFrame(animate)

  const time = performance.now() * 0.001

  if (particles) {
    particles.material.uniforms.time.value = time
    particles.material.uniforms.mousePos.value.set(mouseX, mouseY)

    // Update particle positions for flow effect
    const positions = particles.geometry.attributes.position.array
    const colors = particles.geometry.attributes.color.array
    const { velocities } = particles.geometry.userData

    for (let i = 0; i < PARTICLE_COUNT; i++) {
      const i3 = i * 3

      // Move particles rightward
      positions[i3] += velocities[i3]
      positions[i3 + 1] += velocities[i3 + 1]
      positions[i3 + 2] += velocities[i3 + 2]

      // Reset particles that go too far right
      if (positions[i3] > 80) {
        positions[i3] = -80
        positions[i3 + 1] = (Math.random() - 0.5) * 60
        positions[i3 + 2] = (Math.random() - 0.5) * 30
      }

      // Update color based on new x position
      const color = getColorForX(positions[i3])

      colors[i3] = color.r
      colors[i3 + 1] = color.g
      colors[i3 + 2] = color.b
    }

    particles.geometry.attributes.position.needsUpdate = true
    particles.geometry.attributes.color.needsUpdate = true

    // Subtle rotation
    particles.rotation.y = Math.sin(time * 0.1) * 0.05
  }

  renderer.render(scene, camera)
}

function onResize() {
  if (!containerRef.value) return

  const container = containerRef.value
  camera.aspect = container.clientWidth / container.clientHeight
  camera.updateProjectionMatrix()
  renderer.setSize(container.clientWidth, container.clientHeight)
}

function onMouseMove(event) {
  if (!containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  // Normalize to -1..1 relative to the hero container, not the viewport
  mouseX = ((event.clientX - rect.left) / rect.width) * 2 - 1
  mouseY = -((event.clientY - rect.top) / rect.height) * 2 + 1

  // Update overlay spotlight position (px-based for radial-gradient)
  if (overlayRef.value) {
    overlayRef.value.style.setProperty('--mx', `${event.clientX - rect.left}px`)
    overlayRef.value.style.setProperty('--my', `${event.clientY - rect.top}px`)
  }
}
</script>

<style scoped>
.hero-container {
  position: relative;
  width: 100%;
  height: 100vh;
  min-height: 700px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: radial-gradient(ellipse at center, rgba(102, 126, 234, 0.1) 0%, transparent 60%),
              linear-gradient(180deg, #0a0a0f 0%, #111118 100%);
}

.particle-canvas {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  background: transparent;
  opacity: 0;
  animation: canvasFadeIn 0.8s ease 0.3s forwards;
}

@keyframes canvasFadeIn {
  to { opacity: 1; }
}

.particle-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background: radial-gradient(
    circle 280px at var(--mx, 50%) var(--my, 50%),
    rgba(140, 160, 255, 0.07) 0%,
    rgba(140, 160, 255, 0.03) 20%,
    transparent 40%,
    rgba(10, 10, 15, 0.55) 100%
  );
  transition: background 0.1s ease-out;
}

.hero-content {
  position: relative;
  z-index: 10;
  text-align: center;
  padding: 0 2rem;
  max-width: 900px;
}

.badge {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: rgba(102, 126, 234, 0.15);
  border: 1px solid rgba(102, 126, 234, 0.3);
  border-radius: 100px;
  font-size: 0.875rem;
  font-weight: 500;
  color: #a78bfa;
  margin-bottom: 1.5rem;
}

.badge-icon {
  font-size: 0.75rem;
}

.hero-title {
  font-size: clamp(2.5rem, 6vw, 4.5rem);
  font-weight: 700;
  line-height: 1.1;
  margin: 0 0 1.5rem;
  color: #fff;
  letter-spacing: -0.02em;
}

.gradient-text {
  background: linear-gradient(135deg, #667eea 0%, #10b981 50%, #3b82f6 100%);
  background-size: 200% 200%;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  animation: gradientShift 4s ease infinite;
}

@keyframes gradientShift {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}

.hero-subtitle {
  font-size: clamp(1rem, 2vw, 1.375rem);
  color: rgba(255, 255, 255, 0.7);
  margin: 0 0 2.5rem;
  line-height: 1.6;
}

.hero-ctas {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}

.cta-primary {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem 2rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  font-weight: 600;
  font-size: 1rem;
  border-radius: 12px;
  text-decoration: none;
  transition: all 0.3s ease;
  box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);
}

.cta-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 25px rgba(102, 126, 234, 0.5);
}

.cta-primary .arrow {
  transition: transform 0.3s ease;
}

.cta-primary:hover .arrow {
  transform: translateX(4px);
}

.cta-secondary {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem 2rem;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #fff;
  font-weight: 500;
  font-size: 1rem;
  border-radius: 12px;
  text-decoration: none;
  transition: all 0.3s ease;
}

.cta-secondary:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
}

.play-icon {
  font-size: 0.75rem;
}

.flow-labels {
  position: absolute;
  bottom: 15%;
  left: 0;
  right: 0;
  display: flex;
  justify-content: space-between;
  padding: 0 10%;
  pointer-events: none;
}

.label {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  color: rgba(255, 255, 255, 0.6);
  font-size: 0.875rem;
  font-weight: 500;
}

.label-icon {
  font-size: 1.5rem;
  opacity: 0.8;
}

.label-center {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
}

.lynq-logo {
  width: 48px;
  height: 48px;
  filter: drop-shadow(0 0 20px rgba(102, 126, 234, 0.5));
}

.scroll-indicator {
  position: absolute;
  bottom: 2rem;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  color: rgba(255, 255, 255, 0.4);
  font-size: 0.75rem;
  text-align: center;
}

.scroll-arrow {
  animation: bounce 2s infinite;
  display: flex;
  justify-content: center;
}

@keyframes bounce {
  0%, 20%, 50%, 80%, 100% { transform: translateY(0); }
  40% { transform: translateY(8px); }
  60% { transform: translateY(4px); }
}

@media (max-width: 768px) {
  .hero-container {
    min-height: 600px;
  }

  .flow-labels {
    display: none;
  }

  .hero-ctas {
    flex-direction: column;
    align-items: center;
  }

  .cta-primary,
  .cta-secondary {
    width: 100%;
    max-width: 280px;
    justify-content: center;
  }
}
</style>
