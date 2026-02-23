<template>
  <div class="landing-page">
    <!-- Hero with 3D Particles -->
    <HeroParticles />

    <!-- Problem vs Solution -->
    <ProblemSolution />

    <!-- How It Works Pipeline -->
    <InteractivePipeline />

    <!-- Safety Policies -->
    <SafetyControls />

    <!-- Dashboard Showcase -->
    <DashboardShowcase />

    <!-- Latest Blog Posts -->
    <LatestBlog />

    <!-- Final CTA -->
    <section class="cta-section">
      <!-- Floating elements: positioned relative to full-width section -->
      <div class="floating-elements">
        <div class="float-item float-1"><code>LynqHub</code></div>
        <div class="float-item float-2"><code>LynqForm</code></div>
        <div class="float-item float-3"><code>LynqNode</code></div>
        <div class="float-item float-4 db"><code>MySQL</code></div>
        <div class="float-item float-5 k8s"><code>Deployment</code></div>
        <div class="float-item float-6 k8s"><code>Service</code></div>
        <div class="float-item float-7 k8s"><code>Ingress</code></div>
        <div class="float-item float-8 k8s"><code>ConfigMap</code></div>
        <div class="float-item float-9 k8s"><code>Secret</code></div>
        <div class="float-item float-10 k8s"><code>StatefulSet</code></div>
        <div class="float-item float-11 k8s"><code>CronJob</code></div>
        <div class="float-item float-12 k8s"><code>PVC</code></div>
        <div class="float-item float-13 k8s"><code>Namespace</code></div>
        <div class="float-item float-14 db"><code>PostgreSQL</code></div>
      </div>
      <div class="cta-container">
        <div class="cta-content fade-up">
          <h2>Ready to Simplify Your Infrastructure?</h2>
          <p>Get started with Lynq in minutes. No complex setup required.</p>

          <div class="cta-buttons">
            <a href="/quickstart" class="cta-primary">
              Start Quickstart
              <span class="arrow">&#8594;</span>
            </a>
            <a href="https://github.com/k8s-lynq/lynq" class="cta-github" target="_blank">
              <svg class="github-icon" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
              </svg>
              View on GitHub
            </a>
          </div>
        </div>
      </div>
    </section>

    <!-- Footer -->
    <footer class="landing-footer">
      <div class="footer-content">
        <div class="footer-left">
          <img src="/logo.png" alt="Lynq" class="footer-logo" />
          <span class="footer-tagline">Infrastructure as Data</span>
        </div>
        <div class="footer-links">
          <a href="/quickstart">Quickstart</a>
          <a href="/concepts">Concepts</a>
          <a href="/policies">Policies</a>
          <a href="/blog/">Blog</a>
          <a href="https://github.com/k8s-lynq/lynq" target="_blank">GitHub</a>
        </div>
        <div class="footer-right">
          <span class="copyright">Open Source under Apache 2.0</span>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import HeroParticles from './landing/HeroParticles.vue'
import ProblemSolution from './landing/ProblemSolution.vue'
import InteractivePipeline from './landing/InteractivePipeline.vue'
import SafetyControls from './landing/SafetyControls.vue'
import DashboardShowcase from './landing/DashboardShowcase.vue'
import LatestBlog from './landing/LatestBlog.vue'
</script>

<style scoped>
.landing-page {
  width: 100%;
  background: #0a0a0f;
  color: #fff;
  overflow-x: hidden;
  position: relative;
}

/* CTA Section */
.cta-section {
  padding: 6rem 2rem;
  background: linear-gradient(180deg, #111118 0%, #0a0a0f 50%, #111118 100%);
  position: relative;
  overflow: hidden;
}

.cta-container {
  max-width: 800px;
  margin: 0 auto;
  position: relative;
}

.cta-content {
  text-align: center;
  position: relative;
  z-index: 10;
}

.cta-content h2 {
  font-size: clamp(2rem, 4vw, 3rem);
  font-weight: 700;
  line-height: 1.3;
  letter-spacing: -0.01em;
  margin: 0 0 1.5rem;
  background: linear-gradient(135deg, #fff 0%, rgba(255, 255, 255, 0.8) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.cta-content p {
  font-size: 1.25rem;
  line-height: 1.6;
  color: rgba(255, 255, 255, 0.6);
  margin: 0 0 2.5rem;
}

.cta-buttons {
  display: flex;
  justify-content: center;
  gap: 1rem;
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

.cta-github {
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

.cta-github:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
}

.github-icon {
  width: 20px;
  height: 20px;
}

/* Floating elements */
.floating-elements {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 1;
}

.float-item {
  position: absolute;
  padding: 0.5rem 1rem;
  background: rgba(102, 126, 234, 0.1);
  border: 1px solid rgba(102, 126, 234, 0.2);
  border-radius: 8px;
  font-size: 0.8rem;
  color: #a78bfa;
  opacity: 0.4;
  animation: float 6s ease-in-out infinite;
}

.float-item code {
  font-family: 'SF Mono', Monaco, monospace;
}

.float-1 {
  top: 15%;
  left: 3%;
  animation-delay: 0s;
}

.float-2 {
  top: 25%;
  right: 3%;
  animation-delay: 2s;
}

.float-3 {
  bottom: 15%;
  left: 5%;
  animation-delay: 4s;
}

.float-4 {
  top: 10%;
  left: 12%;
  animation-delay: 1s;
}

.float-5 {
  top: 40%;
  right: 2%;
  animation-delay: 3s;
}

.float-6 {
  bottom: 25%;
  right: 8%;
  animation-delay: 5s;
}

.float-7 {
  bottom: 10%;
  left: 15%;
  animation-delay: 2.5s;
}

.float-8 {
  top: 18%;
  right: 12%;
  animation-delay: 1.5s;
}

.float-9 {
  bottom: 35%;
  left: 2%;
  animation-delay: 3.5s;
}

.float-10 {
  top: 50%;
  left: 8%;
  animation-delay: 0.5s;
}

.float-11 {
  top: 5%;
  right: 6%;
  animation-delay: 4.5s;
}

.float-12 {
  bottom: 5%;
  right: 3%;
  animation-delay: 2s;
}

.float-13 {
  top: 35%;
  left: 1%;
  animation-delay: 5.5s;
}

.float-14 {
  bottom: 30%;
  right: 14%;
  animation-delay: 3s;
}

.float-item.k8s {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.2);
  color: #6ee7b7;
}

.float-item.db {
  background: rgba(59, 130, 246, 0.1);
  border-color: rgba(59, 130, 246, 0.2);
  color: #93c5fd;
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

@keyframes float {
  0%, 100% {
    transform: translateY(0) rotate(0deg);
    opacity: 0.3;
  }
  50% {
    transform: translateY(-20px) rotate(3deg);
    opacity: 0.5;
  }
}

/* Footer */
.landing-footer {
  padding: 2rem;
  background: #0a0a0f;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.footer-content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 1.5rem;
}

.footer-left {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.footer-logo {
  width: 32px;
  height: 32px;
}

.footer-tagline {
  font-size: 0.9rem;
  color: rgba(255, 255, 255, 0.5);
}

.footer-links {
  display: flex;
  gap: 1.5rem;
  flex-wrap: wrap;
}

.footer-links a {
  font-size: 0.9rem;
  color: rgba(255, 255, 255, 0.6);
  text-decoration: none;
  transition: color 0.3s ease;
}

.footer-links a:hover {
  color: #fff;
}

.footer-right {
  text-align: right;
}

.copyright {
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.4);
}

@media (max-width: 768px) {
  .cta-buttons {
    flex-direction: column;
    align-items: center;
  }

  .cta-primary,
  .cta-github {
    width: 100%;
    max-width: 280px;
    justify-content: center;
  }

  .floating-elements {
    display: none;
  }

  .footer-content {
    flex-direction: column;
    text-align: center;
  }

  .footer-right {
    text-align: center;
  }
}
</style>
