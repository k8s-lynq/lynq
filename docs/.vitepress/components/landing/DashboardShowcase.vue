<template>
  <section class="dashboard-section">
    <div class="container">
      <div class="section-header fade-up">
        <span class="section-label">Visibility</span>
        <h2>See Everything at a Glance</h2>
        <p class="section-subtitle">Monitor Hub-Form-Node relationships and quickly identify issues</p>
      </div>

      <div class="browser-mockup fade-up-scale" style="animation-delay: 0.2s">
        <!-- Browser Chrome -->
        <div class="browser-chrome">
          <div class="browser-buttons">
            <span class="btn-close"></span>
            <span class="btn-minimize"></span>
            <span class="btn-maximize"></span>
          </div>
          <div class="browser-address">
            <span class="lock-icon">&#128274;</span>
            <span class="url">localhost:8080/topology</span>
          </div>
          <div class="browser-actions"></div>
        </div>

        <!-- Dashboard Content -->
        <div class="browser-content">
          <video
            v-if="showVideo"
            class="dashboard-video"
            autoplay
            loop
            muted
            playsinline
          >
            <source src="/blog-assets/dashboard-example.mov" type="video/quicktime" />
            <source src="/blog-assets/dashboard-example.mov" type="video/mp4" />
          </video>
          <img
            v-else
            src="/blog-assets/dashboard-topology-view.png"
            alt="Lynq Dashboard Topology View"
            class="dashboard-image"
          />
        </div>
      </div>

      <!-- Feature list -->
      <div class="features-row">
        <div
          v-for="(feature, index) in features"
          :key="feature.title"
          class="feature-item fade-up"
          :style="{ animationDelay: `${0.1 * index}s` }"
        >
          <span class="feature-icon">{{ feature.icon }}</span>
          <div class="feature-text">
            <strong>{{ feature.title }}</strong>
            <span>{{ feature.description }}</span>
          </div>
        </div>
      </div>

      <div class="dashboard-cta fade-up" style="animation-delay: 0.3s">
        <a href="/dashboard" class="cta-button">
          Install Dashboard
          <span class="arrow">&#8594;</span>
        </a>
        <a href="https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart" class="try-demo" target="_blank">
          Try interactive demo
          <span class="external">&#8599;</span>
        </a>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref } from 'vue'

const showVideo = ref(true)
const features = [
  {
    icon: '📊',
    title: 'Resource Status',
    description: 'Ready, pending, failed at a glance'
  },
  {
    icon: '🔍',
    title: 'Quick Search',
    description: '⌘K to find any resource instantly'
  },
  {
    icon: '🌐',
    title: 'i18n Support',
    description: 'English & Korean languages'
  },
  {
    icon: '🌙',
    title: 'Dark Mode',
    description: 'Easy on the eyes'
  }
]
</script>

<style scoped>
.dashboard-section {
  padding: 6rem 2rem;
  background: linear-gradient(180deg, #0a0a0f 0%, #111118 100%);
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
  color: #3b82f6;
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

.browser-mockup {
  background: #1a1a2e;
  border-radius: 12px;
  overflow: hidden;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.05);
  margin-bottom: 3rem;
}

.browser-chrome {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem 1rem;
  background: #252538;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.browser-buttons {
  display: flex;
  gap: 0.5rem;
}

.browser-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close {
  background: #ff5f57;
}

.btn-minimize {
  background: #febc2e;
}

.btn-maximize {
  background: #28c840;
}

.browser-address {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 6px;
  max-width: 400px;
}

.lock-icon {
  font-size: 0.75rem;
  opacity: 0.5;
}

.url {
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.7);
  font-family: monospace;
}

.browser-actions {
  width: 80px;
}

.browser-content {
  position: relative;
  overflow: hidden;
  background: #0d0d12;
}

.dashboard-video,
.dashboard-image {
  display: block;
  width: 100%;
  height: auto;
}

.features-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
  margin-bottom: 3rem;
}

.feature-item {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1rem;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 12px;
}

.feature-icon {
  font-size: 1.5rem;
}

.feature-text {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.feature-text strong {
  font-size: 0.95rem;
  color: #fff;
}

.feature-text span {
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.5);
}

.dashboard-cta {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 2rem;
  flex-wrap: wrap;
}

.cta-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem 2rem;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  color: #fff;
  font-weight: 600;
  font-size: 1rem;
  border-radius: 12px;
  text-decoration: none;
  transition: all 0.3s ease;
  box-shadow: 0 4px 15px rgba(59, 130, 246, 0.4);
}

.cta-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 25px rgba(59, 130, 246, 0.5);
}

.cta-button .arrow {
  transition: transform 0.3s ease;
}

.cta-button:hover .arrow {
  transform: translateX(4px);
}

.try-demo {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: rgba(255, 255, 255, 0.7);
  font-size: 1rem;
  text-decoration: none;
  transition: color 0.3s ease;
}

.try-demo:hover {
  color: #fff;
}

.external {
  font-size: 0.9rem;
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

@keyframes fadeUpScale {
  from {
    opacity: 0;
    transform: translateY(50px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.fade-up {
  animation: fadeUp 0.6s ease both;
}

.fade-up-scale {
  animation: fadeUpScale 0.8s ease both;
}

@media (max-width: 768px) {
  .browser-address {
    display: none;
  }

  .features-row {
    grid-template-columns: 1fr 1fr;
  }

}

@media (max-width: 500px) {
  .features-row {
    grid-template-columns: 1fr;
  }
}
</style>
