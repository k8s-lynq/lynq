<script setup>
import { motion, useScroll, useTransform } from 'motion-v'

const { scrollYProgress } = useScroll()
const heroOpacity = useTransform(scrollYProgress, [0, 0.15], [1, 0])
const heroScale = useTransform(scrollYProgress, [0, 0.15], [1, 0.95])
</script>

<template>
  <div class="landing-wrapper">
    <!-- Scroll Progress Bar -->
    <motion.div
      :style="{ scaleX: scrollYProgress }"
      class="progress-bar"
    />

    <!-- Hero Section -->
    <motion.div
      class="hero-section"
      :style="{ opacity: heroOpacity, scale: heroScale }"
    >
      <div class="hero-content">
        <div class="hero-grid">
          <motion.div
            class="hero-text"
            :initial="{ opacity: 0, x: -60 }"
            :animate="{ opacity: 1, x: 0 }"
            :transition="{ duration: 0.8, ease: 'easeOut' }"
          >
            <motion.div
              class="hero-badge"
              :initial="{ opacity: 0, y: -20 }"
              :animate="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.2, duration: 0.5 }"
            >
              <span class="hero-badge-dot"></span>
              Infrastructure as Data
            </motion.div>
            <motion.h1
              class="hero-title"
              :initial="{ opacity: 0, y: 30 }"
              :animate="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.3, duration: 0.7 }"
            >
              Database records become Kubernetes resources
            </motion.h1>
            <motion.p
              class="hero-subtitle"
              :initial="{ opacity: 0, y: 30 }"
              :animate="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.5, duration: 0.7 }"
            >
              Insert a row, provision infrastructure. Update a field, reconfigure resources.
              Delete a record, clean up everything. No YAML. No CI/CD delays.
            </motion.p>
            <motion.div
              class="hero-actions"
              :initial="{ opacity: 0, y: 30 }"
              :animate="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.7, duration: 0.7 }"
            >
              <a href="https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart" class="hero-btn hero-btn-primary" target="_blank">
                Try Demo
              </a>
              <a href="/quickstart" class="hero-btn hero-btn-secondary">
                Get Started
              </a>
              <a href="/dashboard" class="hero-btn hero-btn-secondary">
                Dashboard
              </a>
            </motion.div>
          </motion.div>
          <motion.div
            class="hero-visual"
            :initial="{ opacity: 0, x: 60, rotateY: -15 }"
            :animate="{ opacity: 1, x: 0, rotateY: 0 }"
            :transition="{ delay: 0.4, duration: 1, ease: 'easeOut' }"
          >
            <div class="dashboard-preview">
              <img src="/blog-assets/dashboard-topology-view.png" alt="Lynq Dashboard - Topology View" />
              <div class="dashboard-label">
                <span>Topology View</span>
              </div>
            </div>
          </motion.div>
        </div>
      </div>
    </motion.div>

    <!-- Video Section -->
    <div class="section video-section">
      <div class="section-inner">
        <motion.div
          class="section-header"
          :initial="{ opacity: 0, y: 40 }"
          :in-view="{ opacity: 1, y: 0 }"
          :transition="{ duration: 0.6 }"
          :in-view-options="{ once: true, amount: 0.3 }"
        >
          <div class="section-divider"></div>
          <h2 class="section-title">See It in Action</h2>
          <p class="section-subtitle">
            Watch how Lynq Dashboard visualizes the relationship between Hub, Form, and Node in real-time
          </p>
        </motion.div>
        <motion.div
          class="video-container"
          :initial="{ opacity: 0, y: 60, scale: 0.9 }"
          :in-view="{ opacity: 1, y: 0, scale: 1 }"
          :transition="{ duration: 0.8, ease: 'easeOut' }"
          :in-view-options="{ once: true, amount: 0.2 }"
        >
          <video controls autoplay loop muted playsinline>
            <source src="/blog-assets/dashboard-example.mov" type="video/quicktime">
            <source src="/blog-assets/dashboard-example.mov" type="video/mp4">
          </video>
        </motion.div>
      </div>
    </div>

    <!-- How It Works -->
    <div class="section how-section">
      <div class="section-inner">
        <motion.div
          class="section-header"
          :initial="{ opacity: 0, y: 40 }"
          :in-view="{ opacity: 1, y: 0 }"
          :transition="{ duration: 0.6 }"
          :in-view-options="{ once: true, amount: 0.3 }"
        >
          <div class="section-divider"></div>
          <h2 class="section-title">How It Works</h2>
          <p class="section-subtitle">
            Three CRDs work together to turn database records into Kubernetes resources
          </p>
        </motion.div>
        <div class="how-flow">
          <motion.div
            v-for="(step, index) in [
              { icon: '🗄️', text: 'LynqHub' },
              { icon: '📋', text: 'LynqForm' },
              { icon: '🎯', text: 'LynqNode' },
              { icon: '☸️', text: 'Resources' }
            ]"
            :key="step.text"
            class="how-step-wrapper"
            :initial="{ opacity: 0, y: 30 }"
            :in-view="{ opacity: 1, y: 0 }"
            :transition="{ delay: index * 0.15, duration: 0.5 }"
            :in-view-options="{ once: true, amount: 0.5 }"
          >
            <div class="how-step">
              <span class="how-step-icon">{{ step.icon }}</span>
              <span class="how-step-text">{{ step.text }}</span>
            </div>
            <span v-if="index < 3" class="how-arrow">→</span>
          </motion.div>
        </div>
        <motion.div
          :initial="{ opacity: 0, y: 40 }"
          :in-view="{ opacity: 1, y: 0 }"
          :transition="{ delay: 0.6, duration: 0.6 }"
          :in-view-options="{ once: true, amount: 0.3 }"
        >
          <AnimatedDiagram />
        </motion.div>
      </div>
    </div>

    <!-- Problem Mode Section -->
    <div class="section problem-section">
      <div class="section-inner">
        <div class="problem-grid">
          <motion.div
            class="problem-text"
            :initial="{ opacity: 0, x: -50 }"
            :in-view="{ opacity: 1, x: 0 }"
            :transition="{ duration: 0.7 }"
            :in-view-options="{ once: true, amount: 0.3 }"
          >
            <h3>Find Problems Instantly</h3>
            <p>
              Problem Mode highlights failed nodes and automatically expands the tree to show their parent Hub and Form.
              No need to scan through hundreds of resources or run complex kubectl commands.
            </p>
            <div class="problem-features">
              <motion.div
                v-for="(feature, index) in [
                  { icon: '🔴', text: 'Failed nodes are highlighted in red' },
                  { icon: '📊', text: 'Problem count badges on each level' },
                  { icon: '🌳', text: 'Auto-expand to problem sources' },
                  { icon: '⚡', text: 'Real-time status updates' }
                ]"
                :key="feature.text"
                class="problem-feature"
                :initial="{ opacity: 0, x: -30 }"
                :in-view="{ opacity: 1, x: 0 }"
                :transition="{ delay: 0.2 + index * 0.1, duration: 0.5 }"
                :in-view-options="{ once: true, amount: 0.5 }"
              >
                <span class="problem-feature-icon">{{ feature.icon }}</span>
                <span>{{ feature.text }}</span>
              </motion.div>
            </div>
          </motion.div>
          <motion.div
            class="problem-image"
            :initial="{ opacity: 0, x: 50, scale: 0.95 }"
            :in-view="{ opacity: 1, x: 0, scale: 1 }"
            :transition="{ duration: 0.7 }"
            :in-view-options="{ once: true, amount: 0.3 }"
          >
            <img src="/blog-assets/dashboard-topology-problem.png" alt="Problem Mode - Failed nodes highlighted" />
          </motion.div>
        </div>
      </div>
    </div>

    <!-- Features Section -->
    <div class="section">
      <div class="section-inner">
        <motion.div
          class="section-header"
          :initial="{ opacity: 0, y: 40 }"
          :in-view="{ opacity: 1, y: 0 }"
          :transition="{ duration: 0.6 }"
          :in-view-options="{ once: true, amount: 0.3 }"
        >
          <div class="section-divider"></div>
          <h2 class="section-title">Why Teams Choose Lynq</h2>
          <p class="section-subtitle">
            A complete platform for database-driven infrastructure provisioning
          </p>
        </motion.div>
        <div class="features-grid">
          <motion.div
            v-for="(feature, index) in [
              { icon: '🗄️', title: 'Database as Source of Truth', desc: 'Read from MySQL, provision to Kubernetes. Your existing database becomes the control plane.', color: 'linear-gradient(90deg, #10b981, #34d399)' },
              { icon: '📋', title: 'Template-Based Resources', desc: 'Go templates with 200+ Sprig functions. Define once, instantiate for every row.', color: 'linear-gradient(90deg, #3b82f6, #60a5fa)' },
              { icon: '🔄', title: 'Server-Side Apply', desc: 'Declarative resource management with conflict detection and automatic drift correction.', color: 'linear-gradient(90deg, #8b5cf6, #a78bfa)' },
              { icon: '📊', title: 'Visual Dashboard', desc: 'Topology view, problem mode, and detailed status. See your infrastructure at a glance.', color: 'linear-gradient(90deg, #f59e0b, #fbbf24)' },
              { icon: '🎯', title: 'Dependency Graph', desc: 'Define resource dependencies with automatic ordering and failure isolation.', color: 'linear-gradient(90deg, #ef4444, #f87171)' },
              { icon: '🚀', title: 'Safe Rollouts', desc: 'Control blast radius with maxSkew. Bad changes affect only N nodes, not your entire fleet.', color: 'linear-gradient(90deg, #06b6d4, #22d3ee)' }
            ]"
            :key="feature.title"
            class="feature-card"
            :style="{ '--feature-color': feature.color }"
            :initial="{ opacity: 0, y: 40 }"
            :in-view="{ opacity: 1, y: 0 }"
            :transition="{ delay: (index % 3) * 0.1, duration: 0.5 }"
            :in-view-options="{ once: true, amount: 0.2 }"
            :while-hover="{ y: -8, transition: { duration: 0.2 } }"
          >
            <span class="feature-icon">{{ feature.icon }}</span>
            <h4 class="feature-title">{{ feature.title }}</h4>
            <p class="feature-desc">{{ feature.desc }}</p>
          </motion.div>
        </div>
      </div>
    </div>

    <!-- Overview Screenshot -->
    <div class="section">
      <div class="section-inner">
        <motion.div
          class="section-header"
          :initial="{ opacity: 0, y: 40 }"
          :in-view="{ opacity: 1, y: 0 }"
          :transition="{ duration: 0.6 }"
          :in-view-options="{ once: true, amount: 0.3 }"
        >
          <div class="section-divider"></div>
          <h2 class="section-title">Complete Visibility</h2>
          <p class="section-subtitle">
            Overview page shows Hub, Form, Node status at a glance with interactive charts
          </p>
        </motion.div>
        <motion.div
          class="video-container overview-container"
          :initial="{ opacity: 0, y: 60, scale: 0.95 }"
          :in-view="{ opacity: 1, y: 0, scale: 1 }"
          :transition="{ duration: 0.8 }"
          :in-view-options="{ once: true, amount: 0.2 }"
        >
          <img src="/blog-assets/dashboard-overview.png" alt="Dashboard Overview" />
        </motion.div>
      </div>
    </div>

    <!-- CTA Section -->
    <div class="section cta-section">
      <div class="section-inner">
        <motion.div
          class="cta-card"
          :initial="{ opacity: 0, y: 50 }"
          :in-view="{ opacity: 1, y: 0 }"
          :transition="{ duration: 0.7 }"
          :in-view-options="{ once: true, amount: 0.3 }"
        >
          <div class="cta-content">
            <motion.h2
              class="cta-title"
              :initial="{ opacity: 0, y: 20 }"
              :in-view="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.2, duration: 0.5 }"
              :in-view-options="{ once: true }"
            >
              Ready to Get Started?
            </motion.h2>
            <motion.p
              class="cta-subtitle"
              :initial="{ opacity: 0, y: 20 }"
              :in-view="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.3, duration: 0.5 }"
              :in-view-options="{ once: true }"
            >
              Try Lynq in our browser-based playground or deploy to your cluster in minutes
            </motion.p>
            <motion.div
              class="cta-actions"
              :initial="{ opacity: 0, y: 20 }"
              :in-view="{ opacity: 1, y: 0 }"
              :transition="{ delay: 0.4, duration: 0.5 }"
              :in-view-options="{ once: true }"
            >
              <a href="https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart" class="hero-btn hero-btn-primary" target="_blank">
                Interactive Demo
              </a>
              <a href="/installation" class="hero-btn hero-btn-secondary">
                Installation Guide
              </a>
              <a href="https://github.com/k8s-lynq/lynq" class="hero-btn hero-btn-secondary" target="_blank">
                GitHub
              </a>
            </motion.div>
            <div class="stats-row">
              <motion.div
                v-for="(stat, index) in [
                  { value: '3', label: 'CRDs' },
                  { value: '200+', label: 'Template Functions' },
                  { value: 'Any', label: 'K8s Resource' },
                  { value: 'K8s 1.31+', label: 'Supported' }
                ]"
                :key="stat.label"
                class="stat-item"
                :initial="{ opacity: 0, y: 30 }"
                :in-view="{ opacity: 1, y: 0 }"
                :transition="{ delay: 0.5 + index * 0.1, duration: 0.5 }"
                :in-view-options="{ once: true }"
              >
                <p class="stat-value">{{ stat.value }}</p>
                <p class="stat-label">{{ stat.label }}</p>
              </motion.div>
            </div>
          </div>
        </motion.div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ===== Root Wrapper ===== */
.landing-wrapper {
  width: 100%;
  overflow-x: hidden;
}

/* ===== Progress Bar ===== */
.progress-bar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, #667eea, #764ba2);
  transform-origin: left;
  z-index: 1000;
}

/* ===== Animation Keyframes ===== */
@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

@keyframes gradientShift {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}

@keyframes glow {
  0%, 100% { box-shadow: 0 0 20px rgba(102, 126, 234, 0.3); }
  50% { box-shadow: 0 0 40px rgba(102, 126, 234, 0.6); }
}

@keyframes rotateGradient {
  0% { --gradient-angle: 0deg; }
  100% { --gradient-angle: 360deg; }
}

@property --gradient-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

/* ===== Hero Section ===== */
.hero-section {
  position: relative;
  width: 100%;
  padding: 4rem 2rem 3rem;
  min-height: 90vh;
  display: flex;
  align-items: center;
  justify-content: center;
}

.hero-section::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background:
    radial-gradient(ellipse 80% 50% at 50% -20%, rgba(102, 126, 234, 0.3) 0%, transparent 50%),
    radial-gradient(ellipse 60% 40% at 80% 60%, rgba(118, 75, 162, 0.2) 0%, transparent 50%),
    radial-gradient(ellipse 50% 30% at 20% 80%, rgba(59, 130, 246, 0.15) 0%, transparent 50%);
  animation: pulse 8s ease-in-out infinite;
  pointer-events: none;
}

.hero-content {
  position: relative;
  width: 100%;
  max-width: 1400px;
  margin: 0 auto;
  z-index: 1;
}

.hero-grid {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  gap: 4rem;
  align-items: center;
}

.hero-text {
  display: flex;
  flex-direction: column;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: rgba(102, 126, 234, 0.15);
  border: 1px solid rgba(102, 126, 234, 0.3);
  border-radius: 100px;
  font-size: 0.85rem;
  font-weight: 600;
  color: #a5b4fc;
  margin-bottom: 1.5rem;
  backdrop-filter: blur(10px);
  width: fit-content;
}

.hero-badge-dot {
  width: 8px;
  height: 8px;
  background: #10b981;
  border-radius: 50%;
  animation: pulse 2s ease-in-out infinite;
}

.hero-title {
  font-size: clamp(2.5rem, 5vw, 4rem);
  font-weight: 800;
  line-height: 1.1;
  margin: 0 0 1.5rem;
  background: linear-gradient(135deg, #fff 0%, #a5b4fc 50%, #c4b5fd 100%);
  background-size: 200% 200%;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  animation: gradientShift 6s ease-in-out infinite;
}

.hero-subtitle {
  font-size: 1.25rem;
  color: var(--vp-c-text-2);
  line-height: 1.7;
  margin: 0 0 2rem;
  max-width: 500px;
}

.hero-actions {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.hero-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.875rem 1.75rem;
  border-radius: 12px;
  font-weight: 600;
  font-size: 1rem;
  text-decoration: none;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.hero-btn-primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  box-shadow: 0 4px 20px rgba(102, 126, 234, 0.4);
}

.hero-btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 30px rgba(102, 126, 234, 0.5);
}

.hero-btn-secondary {
  background: rgba(255, 255, 255, 0.05);
  color: var(--vp-c-text-1);
  border: 1px solid rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(10px);
}

.hero-btn-secondary:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(102, 126, 234, 0.5);
  transform: translateY(-2px);
}

/* ===== Dashboard Preview ===== */
.hero-visual {
  position: relative;
  perspective: 1000px;
}

.dashboard-preview {
  position: relative;
  border-radius: 16px;
  overflow: hidden;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.1);
  animation: float 6s ease-in-out infinite;
}

.dashboard-preview::before {
  content: '';
  position: absolute;
  inset: -2px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.5), rgba(118, 75, 162, 0.5), rgba(59, 130, 246, 0.5));
  border-radius: 18px;
  z-index: -1;
  animation: glow 4s ease-in-out infinite;
}

.dashboard-preview img {
  width: 100%;
  height: auto;
  display: block;
}

.dashboard-label {
  position: absolute;
  bottom: 1rem;
  right: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(10px);
  border-radius: 8px;
  font-size: 0.8rem;
  color: #a5b4fc;
  font-weight: 500;
}

/* ===== Section Styles ===== */
.section {
  position: relative;
  width: 100%;
  padding: 5rem 2rem;
}

.section-inner {
  max-width: 1200px;
  margin: 0 auto;
}

.section-divider {
  width: 60px;
  height: 3px;
  background: linear-gradient(90deg, #667eea, #764ba2);
  border-radius: 2px;
  margin: 0 auto 2rem;
}

.section-header {
  text-align: center;
  margin-bottom: 3rem;
}

.section-title {
  font-size: clamp(2rem, 4vw, 2.75rem);
  font-weight: 700;
  margin: 0 0 1.5rem;
  padding-top: 0;
  border-top: none;
  color: var(--vp-c-text-1);
}

.section-subtitle {
  font-size: 1.1rem;
  color: var(--vp-c-text-2);
  max-width: 700px;
  margin: 0 auto;
  line-height: 1.7;
}

/* ===== Video Section ===== */
.video-section {
  position: relative;
}

.video-section::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 100vw;
  height: 100%;
  background: linear-gradient(180deg, transparent 0%, rgba(102, 126, 234, 0.05) 50%, transparent 100%);
  z-index: -1;
}

.video-container {
  position: relative;
  border-radius: 20px;
  overflow: hidden;
  box-shadow:
    0 30px 60px -15px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.1);
  max-width: 1000px;
  margin: 0 auto;
}

.video-container::before {
  content: '';
  position: absolute;
  inset: -3px;
  background: conic-gradient(
    from var(--gradient-angle),
    #667eea,
    #764ba2,
    #f093fb,
    #667eea
  );
  border-radius: 22px;
  z-index: -1;
  animation: rotateGradient 8s linear infinite;
  opacity: 0.7;
}

.video-container video,
.video-container img {
  width: 100%;
  height: auto;
  display: block;
}

.overview-container::before {
  animation: none;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.3), rgba(118, 75, 162, 0.3));
}

/* ===== Problem Mode Section ===== */
.problem-section {
  position: relative;
}

.problem-section::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 100vw;
  height: 100%;
  background: linear-gradient(180deg, transparent 0%, rgba(239, 68, 68, 0.03) 50%, transparent 100%);
  z-index: -1;
}

.problem-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 3rem;
  align-items: center;
}

.problem-text h3 {
  font-size: 1.75rem;
  font-weight: 700;
  margin: 0 0 1rem;
  padding-top: 0;
  border-top: none;
  color: var(--vp-c-text-1);
}

.problem-text p {
  font-size: 1.05rem;
  color: var(--vp-c-text-2);
  line-height: 1.7;
  margin: 0 0 1.5rem;
}

.problem-features {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.problem-feature {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.95rem;
  color: var(--vp-c-text-2);
}

.problem-feature-icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(239, 68, 68, 0.15);
  border-radius: 8px;
  font-size: 1rem;
  flex-shrink: 0;
}

.problem-image {
  position: relative;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.4);
}

.problem-image::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, #ef4444, #f97316);
  z-index: 1;
}

.problem-image img {
  width: 100%;
  height: auto;
  display: block;
}

/* ===== Features Grid ===== */
.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.5rem;
}

.feature-card {
  position: relative;
  padding: 2rem;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  cursor: default;
}

.feature-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--feature-color, linear-gradient(90deg, #667eea, #764ba2));
  opacity: 0;
  transition: opacity 0.3s ease;
}

.feature-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.15);
  box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.3);
}

.feature-card:hover::before {
  opacity: 1;
}

.feature-icon {
  font-size: 2rem;
  margin-bottom: 1rem;
  display: block;
}

.feature-title {
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0 0 0.75rem;
  padding-top: 0;
  border-top: none;
  color: var(--vp-c-text-1);
}

.feature-desc {
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
  line-height: 1.6;
  margin: 0;
}

/* ===== How It Works ===== */
.how-section {
  position: relative;
}

.how-section::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 100vw;
  height: 100%;
  background: linear-gradient(180deg, transparent 0%, rgba(16, 185, 129, 0.03) 50%, transparent 100%);
  z-index: -1;
}

.how-flow {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  margin-bottom: 3rem;
  flex-wrap: wrap;
}

.how-step-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.how-step {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  transition: all 0.3s ease;
}

.how-step:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(102, 126, 234, 0.3);
  transform: translateY(-2px);
}

.how-step-icon {
  font-size: 1.5rem;
}

.how-step-text {
  font-size: 0.95rem;
  font-weight: 500;
  color: var(--vp-c-text-1);
}

.how-arrow {
  font-size: 1.25rem;
  color: var(--vp-c-text-3);
  margin: 0 0.25rem;
}

/* ===== CTA Section ===== */
.cta-section {
  position: relative;
}

.cta-section::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 100vw;
  height: 100%;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.1) 0%, rgba(118, 75, 162, 0.1) 100%);
  z-index: -1;
}

.cta-card {
  text-align: center;
  padding: 4rem;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  position: relative;
  overflow: hidden;
}

.cta-card::before {
  content: '';
  position: absolute;
  top: -50%;
  left: -50%;
  width: 200%;
  height: 200%;
  background: conic-gradient(
    from var(--gradient-angle),
    transparent 0deg,
    rgba(102, 126, 234, 0.1) 60deg,
    transparent 120deg
  );
  animation: rotateGradient 10s linear infinite;
}

.cta-content {
  position: relative;
  z-index: 1;
}

.cta-title {
  font-size: 2rem;
  font-weight: 700;
  margin: 0 0 1rem;
  padding-top: 0;
  border-top: none;
  color: var(--vp-c-text-1);
}

.cta-subtitle {
  font-size: 1.1rem;
  color: var(--vp-c-text-2);
  margin: 0 0 2rem;
  max-width: 500px;
  margin-left: auto;
  margin-right: auto;
}

.cta-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}

/* ===== Stats ===== */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 2rem;
  margin-top: 3rem;
}

.stat-item {
  text-align: center;
}

.stat-value {
  font-size: 2.5rem;
  font-weight: 800;
  line-height: 1.2;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
  padding: 0.25rem 0;
}

.stat-label {
  font-size: 0.9rem;
  color: var(--vp-c-text-3);
  margin: 0.5rem 0 0;
}

/* ===== Responsive ===== */
@media (max-width: 1024px) {
  .hero-grid {
    grid-template-columns: 1fr;
    gap: 3rem;
    text-align: center;
  }

  .hero-text {
    align-items: center;
  }

  .hero-subtitle {
    max-width: none;
    margin-left: auto;
    margin-right: auto;
  }

  .hero-actions {
    justify-content: center;
  }

  .problem-grid {
    grid-template-columns: 1fr;
    gap: 2rem;
  }

  .features-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .hero-section {
    padding: 2rem 1rem;
    min-height: auto;
  }

  .section {
    padding: 3rem 1rem;
  }

  .hero-actions {
    flex-direction: column;
  }

  .hero-btn {
    justify-content: center;
  }

  .how-flow {
    flex-direction: column;
  }

  .how-step-wrapper {
    flex-direction: column;
  }

  .how-arrow {
    transform: rotate(90deg);
  }

  .features-grid {
    grid-template-columns: 1fr;
  }

  .stats-row {
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
  }

  .cta-card {
    padding: 2rem;
  }
}
</style>
