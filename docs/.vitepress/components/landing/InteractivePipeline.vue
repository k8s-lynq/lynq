<template>
  <section class="pipeline-section">
    <div class="container">
      <div class="section-header fade-up">
        <span class="section-label">How It Works</span>
        <h2>From Data to Resources in Seconds</h2>
        <p class="section-subtitle">Click each step to see how Lynq transforms database records into Kubernetes resources</p>
      </div>

      <div
        class="pipeline-container fade-up"
        style="animation-delay: 0.2s"
        ref="pipelineRef"
      >
        <div class="pipeline">
          <!-- Step 1: Database -->
          <div
            class="step"
            :class="{ active: activeStep === 0 }"
            @click="setActiveStep(0)"
          >
            <div class="step-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <ellipse cx="12" cy="5" rx="9" ry="3"/>
                <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
              </svg>
            </div>
            <div class="step-label">Database</div>
            <div class="step-sublabel">MySQL / PostgreSQL</div>
          </div>

          <!-- Connection 1 -->
          <div class="connection">
            <svg viewBox="0 0 100 20" class="connection-line">
              <defs>
                <linearGradient id="connGrad1" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#667eea" />
                  <stop offset="100%" stop-color="#667eea" stop-opacity="0.5" />
                </linearGradient>
              </defs>
              <line x1="0" y1="10" x2="100" y2="10" stroke="url(#connGrad1)" stroke-width="2" stroke-dasharray="4,4">
                <animate attributeName="stroke-dashoffset" from="8" to="0" dur="1s" repeatCount="indefinite" />
              </line>
            </svg>
            <span class="connection-label">sync</span>
          </div>

          <!-- Step 2: LynqHub -->
          <div
            class="step hub-step"
            :class="{ active: activeStep === 1 }"
            @click="setActiveStep(1)"
          >
            <div class="step-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="3"/>
                <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/>
              </svg>
            </div>
            <div class="step-label">LynqHub</div>
            <div class="step-sublabel">Data Source</div>
          </div>

          <!-- Connection 2 -->
          <div class="connection">
            <svg viewBox="0 0 100 20" class="connection-line">
              <line x1="0" y1="10" x2="100" y2="10" stroke="url(#connGrad1)" stroke-width="2" stroke-dasharray="4,4">
                <animate attributeName="stroke-dashoffset" from="8" to="0" dur="1s" repeatCount="indefinite" />
              </line>
            </svg>
            <span class="connection-label">data</span>
          </div>

          <!-- Step 3: LynqForm -->
          <div
            class="step form-step"
            :class="{ active: activeStep === 2 }"
            @click="setActiveStep(2)"
          >
            <div class="step-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
                <line x1="16" y1="13" x2="8" y2="13"/>
                <line x1="16" y1="17" x2="8" y2="17"/>
                <line x1="10" y1="9" x2="8" y2="9"/>
              </svg>
            </div>
            <div class="step-label">LynqForm</div>
            <div class="step-sublabel">Template</div>
          </div>

          <!-- Connection 3 -->
          <div class="connection">
            <svg viewBox="0 0 100 20" class="connection-line">
              <defs>
                <linearGradient id="connGrad2" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#667eea" stop-opacity="0.5" />
                  <stop offset="100%" stop-color="#10b981" />
                </linearGradient>
              </defs>
              <line x1="0" y1="10" x2="100" y2="10" stroke="url(#connGrad2)" stroke-width="2" stroke-dasharray="4,4">
                <animate attributeName="stroke-dashoffset" from="8" to="0" dur="1s" repeatCount="indefinite" />
              </line>
            </svg>
            <span class="connection-label">apply</span>
          </div>

          <!-- Step 4: Kubernetes -->
          <div
            class="step k8s-step"
            :class="{ active: activeStep === 3 }"
            @click="setActiveStep(3)"
          >
            <div class="step-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 2L2 7l10 5 10-5-10-5z"/>
                <path d="M2 17l10 5 10-5"/>
                <path d="M2 12l10 5 10-5"/>
              </svg>
            </div>
            <div class="step-label">Kubernetes</div>
            <div class="step-sublabel">Resources</div>
          </div>
        </div>

        <!-- Detail Panel -->
        <transition name="fade-slide" mode="out-in">
          <div class="detail-panel" :key="activeStep">
            <div class="detail-content">
              <div class="detail-header">
                <span class="detail-step">Step {{ activeStep + 1 }}</span>
                <h3>{{ steps[activeStep].title }}</h3>
              </div>
              <p class="detail-description">{{ steps[activeStep].description }}</p>

              <div class="code-preview">
                <div class="code-header">
                  <span class="code-filename">{{ steps[activeStep].filename }}</span>
                  <span class="code-lang">YAML</span>
                </div>
                <pre class="code-content"><code>{{ steps[activeStep].code }}</code></pre>
              </div>
            </div>
          </div>
        </transition>
      </div>

      <!-- Data packets animation -->
      <div class="packets-container" v-if="pipelineVisible">
        <div
          v-for="i in 3"
          :key="'packet-' + i"
          class="data-packet"
          :style="{ animationDelay: `${i * 0.8}s` }"
        ></div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const activeStep = ref(0)
const pipelineRef = ref(null)
const pipelineVisible = ref(false)

const steps = [
  {
    title: 'Connect Your Database',
    description: 'LynqHub connects to your existing MySQL database and periodically syncs active records that should have Kubernetes resources.',
    filename: 'lynqhub.yaml',
    code: `apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-hub
spec:
  source:
    type: mysql
    syncInterval: 30s
    mysql:
      host: mysql.default.svc
      port: 3306
      username: node_reader
      passwordRef:
        name: mysql-credentials
        key: password
      database: nodes
      table: node_configs`
  },
  {
    title: 'Define Value Mappings',
    description: 'Map database columns to template variables. Lynq extracts the UID, activation status, and any custom fields you need.',
    filename: 'lynqhub.yaml',
    code: `spec:
  valueMappings:
    uid: node_id          # Unique identifier
    activate: is_active   # Boolean flag
  extraValueMappings:
    planId: subscription_plan
    nodeUrl: node_url`
  },
  {
    title: 'Create Templates',
    description: 'LynqForm defines what Kubernetes resources to create for each record. Use Go templates with your mapped values.',
    filename: 'lynqform.yaml',
    code: `apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: web-stack
spec:
  hubId: my-hub
  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: "{{ .uid }}"
        template:
          metadata:
            labels:
              app: "{{ .uid }}"
          spec:
            containers:
              - name: app
                image: nginx:stable`
  },
  {
    title: 'Resources Appear',
    description: 'Lynq automatically creates and manages Kubernetes resources for each database record. Add a row, get resources. Delete a row, resources are cleaned up.',
    filename: 'kubectl output',
    code: `$ kubectl get lynqnodes
NAME                  UID        FORM       READY  DESIRED  SKIPPED  CONFLICTED  CONDITIONS   AGE
acme-corp-web-stack   acme-corp  web-stack  3      3                 False       Reconciled   2m
beta-inc-web-stack    beta-inc   web-stack  2      2                 False       Reconciled   1m

$ kubectl get deployments
NAME              READY   AGE
acme-corp-app     1/1     2m
beta-inc-app      1/1     1m`
  }
]

function setActiveStep(step) {
  activeStep.value = step
}

let observer = null

onMounted(() => {
  if (pipelineRef.value) {
    observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          pipelineVisible.value = entry.isIntersecting
        })
      },
      { threshold: 0.2 }
    )
    observer.observe(pipelineRef.value)
  }
})

onUnmounted(() => {
  if (observer) observer.disconnect()
})
</script>

<style scoped>
.pipeline-section {
  padding: 6rem 2rem;
  background: linear-gradient(180deg, #0d0d12 0%, #111118 100%);
  position: relative;
  overflow: hidden;
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
  color: #10b981;
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

.pipeline-container {
  position: relative;
}

.pipeline {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  padding: 2rem 0;
  margin-bottom: 2rem;
}

.step {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.5rem;
  cursor: pointer;
  transition: all 0.3s ease;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 2px solid transparent;
  min-width: 120px;
}

.step:hover {
  background: rgba(255, 255, 255, 0.05);
  transform: translateY(-4px);
}

.step.active {
  background: rgba(102, 126, 234, 0.1);
  border-color: rgba(102, 126, 234, 0.4);
  transform: translateY(-4px);
}

.hub-step.active {
  background: rgba(102, 126, 234, 0.1);
  border-color: rgba(102, 126, 234, 0.5);
}

.form-step.active {
  background: rgba(102, 126, 234, 0.1);
  border-color: rgba(102, 126, 234, 0.5);
}

.k8s-step.active {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.5);
}

.step-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 0.75rem;
  color: #667eea;
}

.k8s-step .step-icon {
  color: #10b981;
}

.step-icon svg {
  width: 32px;
  height: 32px;
}

.step-label {
  font-size: 0.95rem;
  font-weight: 600;
  color: #fff;
  margin-bottom: 0.25rem;
}

.step-sublabel {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.5);
}

.connection {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 80px;
}

.connection-line {
  width: 100%;
  height: 20px;
}

.connection-label {
  font-size: 0.7rem;
  color: rgba(255, 255, 255, 0.4);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-top: 0.25rem;
}

.detail-panel {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  overflow: hidden;
}

.detail-content {
  padding: 2rem;
}

.detail-header {
  margin-bottom: 1rem;
}

.detail-step {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #667eea;
  margin-bottom: 0.5rem;
  display: block;
}

.detail-header h3 {
  font-size: 1.5rem;
  font-weight: 600;
  color: #fff;
  margin: 0;
}

.detail-description {
  font-size: 1rem;
  color: rgba(255, 255, 255, 0.7);
  line-height: 1.6;
  margin-bottom: 1.5rem;
}

.code-preview {
  background: #0a0a0f;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.code-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1rem;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.code-filename {
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.6);
  font-family: monospace;
}

.code-lang {
  font-size: 0.7rem;
  color: #667eea;
  text-transform: uppercase;
  font-weight: 600;
}

.code-content {
  padding: 1rem;
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.5;
  overflow-x: auto;
  color: #e2e8f0;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
}

.code-content code {
  white-space: pre;
}

/* Data packets animation */
.packets-container {
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 4px;
  pointer-events: none;
  transform: translateY(-50%);
  display: none;
}

.data-packet {
  position: absolute;
  width: 8px;
  height: 8px;
  background: #667eea;
  border-radius: 50%;
  box-shadow: 0 0 10px #667eea, 0 0 20px rgba(102, 126, 234, 0.5);
  animation: flowPacket 3s linear infinite;
}

@keyframes flowPacket {
  0% {
    left: 10%;
    opacity: 0;
  }
  10% {
    opacity: 1;
  }
  50% {
    background: #667eea;
  }
  90% {
    opacity: 1;
    background: #10b981;
  }
  100% {
    left: 90%;
    opacity: 0;
  }
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

/* Transitions */
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.3s ease;
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

@media (max-width: 900px) {
  .pipeline {
    flex-wrap: wrap;
    gap: 1rem;
  }

  .connection {
    display: none;
  }

  .step {
    width: calc(50% - 0.5rem);
    min-width: auto;
  }
}

@media (max-width: 500px) {
  .step {
    width: 100%;
  }
}
</style>
