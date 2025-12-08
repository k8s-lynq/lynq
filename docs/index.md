---
layout: home
---

<style scoped>
.custom-hero {
  position: relative;
  width: 100vw;
  margin-left: 50%;
  transform: translateX(-50%);
  padding: 5rem 2rem 4rem;
  overflow: hidden;
}

.custom-hero::before {
  content: '';
  position: fixed;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 100%;
  height: 100%;
  z-index: -1;
  background: radial-gradient(
    ellipse 35% 28% at 50% 25%,
    rgba(100, 108, 255, 0.5) 0%,
    rgba(88, 96, 224, 0.3) 30%,
    rgba(74, 82, 196, 0.18) 50%,
    transparent 70%
  );
  opacity: 1;
  pointer-events: none;
  animation: gradientPulse 8s ease-in-out infinite;
}

/* Light mode - much more visible gradient */
html:not(.dark) .custom-hero::before {
  background: radial-gradient(
    ellipse 40% 32% at 50% 25%,
    rgba(100, 108, 255, 0.35) 0%,
    rgba(88, 96, 224, 0.22) 30%,
    rgba(74, 82, 196, 0.12) 50%,
    rgba(74, 82, 196, 0.04) 65%,
    transparent 80%
  );
}

@keyframes gradientPulse {
  0%, 100% {
    opacity: 0.5;
    transform: translateX(-50%) scale(0.95);
  }
  50% {
    opacity: 1.2;
    transform: translateX(-50%) scale(1.15);
  }
}

.hero-content {
  position: relative;
  max-width: 1200px;
  margin: 0 auto;
  text-align: center;
  z-index: 1;
}

.hero-tagline {
  font-size: clamp(2.5rem, 7vw, 4.5rem);
  font-weight: 800;
  line-height: 1.15;
  margin: 0 0 1.5rem;
  color: var(--vp-c-text-1);
  letter-spacing: -0.02em;
  opacity: 0;
  animation: fadeInUp 1s ease-out 0.1s forwards;
}

.hero-description {
  font-size: clamp(1.15rem, 2.5vw, 1.5rem);
  color: var(--vp-c-text-2);
  margin: 0 0 2.5rem;
  max-width: 800px;
  margin-left: auto;
  margin-right: auto;
  line-height: 1.6;
  font-weight: 500;
  opacity: 0;
  animation: fadeInUp 1s ease-out 0.3s forwards;
}

.hero-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
  margin-bottom: 3rem;
  opacity: 0;
  animation: fadeInUp 1s ease-out 0.5s forwards;
}

.hero-action {
  position: relative;
  display: inline-block;
  padding: 1rem 2.5rem;
  border-radius: 12px;
  font-weight: 600;
  font-size: 1.05rem;
  text-decoration: none;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  z-index: 1;
}

.hero-action.brand {
  background: linear-gradient(
    135deg,
    rgba(102, 126, 234, 0.15) 0%,
    rgba(118, 75, 162, 0.15) 100%
  );
  color: white;
  border: 1px solid rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  box-shadow:
    0 8px 32px rgba(102, 126, 234, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.15),
    inset 0 -1px 0 rgba(0, 0, 0, 0.1);
  position: relative;
  isolation: isolate;
}

.hero-action.brand::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 12px;
  background: linear-gradient(
    180deg,
    rgba(255, 255, 255, 0.12) 0%,
    rgba(255, 255, 255, 0.02) 50%,
    rgba(0, 0, 0, 0.05) 100%
  );
  pointer-events: none;
  z-index: 1;
}

.hero-action.brand::after {
  content: '';
  position: absolute;
  top: -50%;
  left: -50%;
  width: 200%;
  height: 200%;
  background: conic-gradient(
    from var(--gradient-angle),
    transparent 0deg,
    rgba(102, 126, 234, 0.5) 60deg,
    rgba(118, 75, 162, 0.5) 120deg,
    rgba(240, 147, 251, 0.5) 180deg,
    transparent 240deg
  );
  animation: rotateGradient 4s linear infinite;
  opacity: 0;
  transition: opacity 0.4s ease;
  filter: blur(15px);
  z-index: 0;
}

.hero-action.brand:hover::after {
  opacity: 1;
}

.hero-action.brand:hover {
  transform: translateY(-3px);
  border-color: rgba(255, 255, 255, 0.3);
  background: linear-gradient(
    135deg,
    rgba(102, 126, 234, 0.25) 0%,
    rgba(118, 75, 162, 0.25) 100%
  );
  box-shadow:
    0 12px 40px rgba(102, 126, 234, 0.35),
    inset 0 1px 0 rgba(255, 255, 255, 0.2),
    inset 0 -1px 0 rgba(0, 0, 0, 0.1);
}

.hero-action.alt {
  background: rgba(255, 255, 255, 0.03);
  color: var(--vp-c-text-1);
  border: 2px solid rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(10px);
}

.hero-action.alt:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(102, 126, 234, 0.5);
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
}

@property --gradient-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

@keyframes rotateGradient {
  0% {
    --gradient-angle: 0deg;
  }
  100% {
    --gradient-angle: 360deg;
  }
}

.hero-diagram {
  opacity: 0;
  animation: fadeInUp 1s ease-out 0.7s forwards;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 768px) {
  .custom-hero {
    padding: 3rem 1.5rem 2.5rem;
  }

  .hero-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .hero-action {
    text-align: center;
  }
}
</style>

<div class="custom-hero">
<div class="hero-content">
<h1 class="hero-tagline">Database-Driven<br/>Kubernetes Automation</h1>
<p class="hero-description">
Turn database rows into production-ready infrastructure.<br/>
Automatically. Welcome to RecordOps.
</p>

<div class="hero-actions">
<a href="https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart" class="hero-action brand" target="_blank" rel="noopener noreferrer">🎮 Try Demo</a>
<a href="/quickstart" class="hero-action alt">Get Started</a>
<a href="https://github.com/k8s-lynq/lynq" class="hero-action alt" target="_blank" rel="noopener noreferrer">GitHub</a>
</div>

<div class="hero-diagram">
<AnimatedDiagram />
</div>
</div>
</div>

## Why Teams Choose Lynq

<p style="text-align: center; color: var(--vp-c-text-2); margin: 0 0 4rem; font-size: 1.1rem; line-height: 1.6; max-width: 900px; margin-left: auto; margin-right: auto;">
Watch how Lynq transforms complex infrastructure management into elegant automation through <strong>RecordOps</strong>—where your database becomes the control plane for your infrastructure
</p>

<WhyLynqCards />

## See It in Real-Time

<p style="text-align: center; color: var(--vp-c-text-2); margin: 0 0 2rem; font-size: 1.05rem; line-height: 1.6; max-width: 800px; margin-left: auto; margin-right: auto;">
Watch how a simple database change instantly triggers infrastructure updates across your Kubernetes cluster
</p>

<RowToStackBanner />

<div style="text-align: center; margin: 3rem 0;">
  <p style="color: var(--vp-c-text-2); margin-bottom: 1.5rem; font-size: 1rem;">
    Want to understand the architecture? Learn how LynqHub, LynqForm, and LynqNode work together to enable RecordOps.
  </p>
  <div style="display: flex; gap: 1rem; justify-content: center; flex-wrap: wrap;">
    <a href="/recordops" style="display: inline-flex; align-items: center; gap: 0.5rem; padding: 0.75rem 1.5rem; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; border-radius: 8px; text-decoration: none; font-weight: 600; transition: all 0.2s ease; box-shadow: 0 4px 14px rgba(102, 126, 234, 0.3);">
      What is RecordOps? →
    </a>
    <a href="/how-it-works" style="display: inline-flex; align-items: center; gap: 0.5rem; padding: 0.75rem 1.5rem; background: var(--vp-c-brand-soft); color: var(--vp-c-brand); border-radius: 8px; text-decoration: none; font-weight: 600; transition: all 0.2s ease;">
      How It Works →
    </a>
  </div>
</div>

::: tip Start in 5 minutes
Follow the [Quick Start Guide](/quickstart) to see this in action with a working MySQL database and sample templates.
:::

## Try It Now — No Setup Required

<div style="display: flex; flex-direction: column; align-items: center; padding: 2rem; background: linear-gradient(135deg, var(--vp-c-bg-soft) 0%, var(--vp-c-bg-elv) 100%); border-radius: 16px; border: 1px solid var(--vp-c-divider); margin: 2.5rem 0; text-align: center;">
  <div style="font-size: 3rem; margin-bottom: 1rem;">🎮</div>
  <h3 style="margin: 0 0 0.75rem; font-size: 1.5rem; font-weight: 700; color: var(--vp-c-text-1);">Interactive Playground</h3>
  <p style="margin: 0 0 1.5rem; color: var(--vp-c-text-2); max-width: 600px; line-height: 1.6;">
    Experience Lynq hands-on in our browser-based Kubernetes environment. Add tenants, watch resources appear, and see automated cleanup — all in 10 minutes.
  </p>
  <a href="https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart" target="_blank" rel="noopener noreferrer" style="display: inline-flex; align-items: center; gap: 0.5rem; padding: 0.875rem 2rem; background: linear-gradient(135deg, #10b981 0%, #059669 100%); color: white; border-radius: 10px; text-decoration: none; font-weight: 600; font-size: 1.05rem; transition: all 0.2s ease; box-shadow: 0 4px 14px rgba(16, 185, 129, 0.3);">
    <span>Launch Interactive Demo</span>
    <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
  </a>
  <p style="margin: 1rem 0 0; font-size: 0.85rem; color: var(--vp-c-text-3);">
    Powered by Killercoda • Free to use
  </p>
</div>

## Fine-Grained Control with Policies

<p style="text-align: center; color: var(--vp-c-text-2); margin: 0 0 2.5rem; font-size: 1.05rem; line-height: 1.6; max-width: 800px; margin-left: auto; margin-right: auto;">
Customize resource lifecycle behavior with powerful policies for creation, deletion, and conflict resolution. Try the interactive simulators to see how each policy works in real-time.
</p>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.5rem; margin: 2.5rem 0">
  <div style="padding: 1.5rem; background: var(--vp-c-bg-soft); border-radius: 8px; border: 1px solid var(--vp-c-divider);">
    <strong style="display: block; margin-bottom: 0.75rem; font-size: 1.05rem; color: var(--vp-c-text-1);">🔄 Creation Policies</strong>
    <p style="margin: 0; font-size: 0.9rem; color: var(--vp-c-text-2); line-height: 1.6">
      Control when resources are applied: <code>Once</code> for immutable resources like init Jobs, or <code>WhenNeeded</code> for dynamic updates
    </p>
  </div>

  <div style="padding: 1.5rem; background: var(--vp-c-bg-soft); border-radius: 8px; border: 1px solid var(--vp-c-divider);">
    <strong style="display: block; margin-bottom: 0.75rem; font-size: 1.05rem; color: var(--vp-c-text-1);">🗑️ Deletion Policies</strong>
    <p style="margin: 0; font-size: 0.9rem; color: var(--vp-c-text-2); line-height: 1.6">
      Choose cleanup behavior: <code>Delete</code> to remove resources automatically, or <code>Retain</code> to preserve them after node deletion
    </p>
  </div>

  <div style="padding: 1.5rem; background: var(--vp-c-bg-soft); border-radius: 8px; border: 1px solid var(--vp-c-divider);">
    <strong style="display: block; margin-bottom: 0.75rem; font-size: 1.05rem; color: var(--vp-c-text-1);">⚠️ Conflict Policies</strong>
    <p style="margin: 0; font-size: 0.9rem; color: var(--vp-c-text-2); line-height: 1.6">
      Handle ownership conflicts: <code>Stuck</code> to prevent overwrites, or <code>Force</code> to take ownership with Server-Side Apply
    </p>
  </div>
</div>

<div style="text-align: center; margin-top: 2rem;">
  <a href="/policies" style="display: inline-flex; align-items: center; gap: 0.5rem; padding: 0.75rem 1.5rem; background: var(--vp-c-brand-soft); color: var(--vp-c-brand); border-radius: 8px; text-decoration: none; font-weight: 600; transition: all 0.2s ease;">
    Explore Policy Simulators →
  </a>
</div>

## Production Ready

<div style="display: flex; align-items: center; gap: 1rem; padding: 1.25rem; background: var(--vp-c-bg-soft); border-radius: 8px; margin: 2.5rem 0">
  <div style="font-size: 2rem">✅</div>
  <div>
    <strong style="font-size: 1.05rem">Validated on Kubernetes v1.28 – v1.33</strong>
    <p style="margin: 0.5rem 0 0; font-size: 0.9rem; color: var(--vp-c-text-2)">
      Production-tested across multiple versions • See <a href="/installation#kubernetes-compatibility">compatibility details</a>
    </p>
  </div>
</div>

## Learn More

<p style="text-align: center; color: var(--vp-c-text-2); margin: 0 0 3rem; font-size: 1.05rem; line-height: 1.6; max-width: 800px; margin-left: auto; margin-right: auto;">
Explore comprehensive guides, API references, and real-world integration examples
</p>

<style scoped>
.doc-card {
  position: relative;
  padding: 2rem;
  background: var(--vp-c-bg-soft);
  border-radius: 12px;
  border: 1px solid var(--vp-c-divider);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: visible;
}

.doc-card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 12px;
  padding: 1px;
  background: linear-gradient(135deg, var(--card-color), transparent 50%, var(--card-color-light));
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
}

.doc-card:hover {
  border-color: transparent;
  box-shadow:
    0 8px 32px rgba(0, 0, 0, 0.08),
    0 0 0 1px var(--card-color-alpha);
  transform: translateY(-4px);
  background: var(--vp-c-bg);
}

.doc-card:hover::before {
  opacity: 1;
}

.doc-card-icon {
  font-size: 2.5rem;
  margin-bottom: 1rem;
  display: block;
  filter: grayscale(0.2);
  transition: all 0.3s ease;
}

.doc-card:hover .doc-card-icon {
  filter: grayscale(0);
  transform: scale(1.1);
}

.doc-card-title {
  margin: 0 0 1.25rem;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
}

.doc-links {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.doc-link-item {
  display: block;
}

.doc-link-item a {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.5rem 0;
  text-decoration: none;
  transition: all 0.2s ease;
  border-radius: 6px;
}

.doc-link-item a:hover {
  padding-left: 0.5rem;
}

.doc-link-title {
  font-weight: 600;
  font-size: 0.95rem;
  color: var(--vp-c-brand);
}

.doc-link-item a:hover .doc-link-title {
  color: var(--vp-c-brand-dark);
}

.doc-link-desc {
  font-size: 0.875rem;
  color: var(--vp-c-text-2);
  line-height: 1.5;
}
</style>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1.5rem; margin: 2.5rem 0">
  <div class="doc-card" style="--card-color: #42b883; --card-color-light: #7dd3ae; --card-color-alpha: rgba(66, 184, 131, 0.2)">
    <span class="doc-card-icon">🚀</span>
    <h3 class="doc-card-title">Getting Started</h3>
    <ul class="doc-links">
      <li class="doc-link-item">
        <a href="/installation">
          <span class="doc-link-title">Installation</span>
          <span class="doc-link-desc">Deploy the operator to your cluster</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/quickstart">
          <span class="doc-link-title">Quick Start</span>
          <span class="doc-link-desc">Complete tutorial in 5 minutes</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/local-development-minikube">
          <span class="doc-link-title">Local Development</span>
          <span class="doc-link-desc">Set up with Minikube</span>
        </a>
      </li>
    </ul>
  </div>

  <div class="doc-card" style="--card-color: #3b82f6; --card-color-light: #93bbfd; --card-color-alpha: rgba(59, 130, 246, 0.2)">
    <span class="doc-card-icon">📚</span>
    <h3 class="doc-card-title">Core Concepts</h3>
    <ul class="doc-links">
      <li class="doc-link-item">
        <a href="/recordops">
          <span class="doc-link-title">RecordOps</span>
          <span class="doc-link-desc">Database-driven infrastructure paradigm</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/architecture">
          <span class="doc-link-title">Architecture</span>
          <span class="doc-link-desc">System design and reconciliation flow</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/api">
          <span class="doc-link-title">API Reference</span>
          <span class="doc-link-desc">Complete CRD specification</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/templates">
          <span class="doc-link-title">Templates</span>
          <span class="doc-link-desc">Go templates with 200+ functions</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/policies">
          <span class="doc-link-title">Policies</span>
          <span class="doc-link-desc">Lifecycle and conflict management</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/datasource">
          <span class="doc-link-title">Datasources</span>
          <span class="doc-link-desc">External data integration (MySQL)</span>
        </a>
      </li>
    </ul>
  </div>

  <div class="doc-card" style="--card-color: #f59e0b; --card-color-light: #fbbf5a; --card-color-alpha: rgba(245, 158, 11, 0.2)">
    <span class="doc-card-icon">⚙️</span>
    <h3 class="doc-card-title">Operations</h3>
    <ul class="doc-links">
      <li class="doc-link-item">
        <a href="/monitoring">
          <span class="doc-link-title">Monitoring</span>
          <span class="doc-link-desc">Prometheus metrics, alerts, and Grafana</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/performance">
          <span class="doc-link-title">Performance</span>
          <span class="doc-link-desc">Tuning and scalability</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/security">
          <span class="doc-link-title">Security</span>
          <span class="doc-link-desc">RBAC, credentials, and best practices</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/troubleshooting">
          <span class="doc-link-title">Troubleshooting</span>
          <span class="doc-link-desc">Common issues and solutions</span>
        </a>
      </li>
    </ul>
  </div>

  <div class="doc-card" style="--card-color: #8b5cf6; --card-color-light: #b794f6; --card-color-alpha: rgba(139, 92, 246, 0.2)">
    <span class="doc-card-icon">🔌</span>
    <h3 class="doc-card-title">Integrations</h3>
    <ul class="doc-links">
      <li class="doc-link-item">
        <a href="/integration-crossplane">
          <span class="doc-link-title">Crossplane</span>
          <span class="doc-link-desc">K8s-native cloud provisioning</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/integration-external-dns">
          <span class="doc-link-title">External DNS</span>
          <span class="doc-link-desc">Automatic DNS per node</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/integration-flux">
          <span class="doc-link-title">Flux</span>
          <span class="doc-link-desc">GitOps-based deployment</span>
        </a>
      </li>
      <li class="doc-link-item">
        <a href="/integration-argocd">
          <span class="doc-link-title">Argo CD</span>
          <span class="doc-link-desc">GitOps delivery pipeline</span>
        </a>
      </li>
    </ul>
  </div>
</div>

## Join the Community

<p style="text-align: center; color: var(--vp-c-text-2); margin: 0 0 2.5rem; font-size: 1.05rem; line-height: 1.6; max-width: 800px; margin-left: auto; margin-right: auto;">
Open source and actively maintained. Contributions, feedback, and questions are welcome
</p>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1.5rem; margin: 2.5rem 0 0">
  <div style="padding: 1.5rem; background: var(--vp-c-bg-soft); border-radius: 8px">
    <h3 style="margin: 0 0 0.75rem">📦 GitHub Repository</h3>
    <p style="margin: 0 0 0.5rem">
      <a href="https://github.com/k8s-lynq/lynq" target="_blank" rel="noopener noreferrer">k8s-lynq/lynq</a>
    </p>
    <p style="margin: 0; font-size: 0.9rem; color: var(--vp-c-text-2)">
      Source code, releases, and project roadmap
    </p>
  </div>

  <div style="padding: 1.5rem; background: var(--vp-c-bg-soft); border-radius: 8px">
    <h3 style="margin: 0 0 0.75rem">🐛 Issue Tracker</h3>
    <p style="margin: 0 0 0.5rem">
      <a href="https://github.com/k8s-lynq/lynq/issues" target="_blank" rel="noopener noreferrer">Report Issues</a>
    </p>
    <p style="margin: 0; font-size: 0.9rem; color: var(--vp-c-text-2)">
      Bug reports, feature requests, and discussions
    </p>
  </div>

  <div style="padding: 1.5rem; background: var(--vp-c-bg-soft); border-radius: 8px">
    <h3 style="margin: 0 0 0.75rem">📖 Documentation</h3>
    <p style="margin: 0 0 0.5rem">
      <a href="/installation">Get Started →</a>
    </p>
    <p style="margin: 0; font-size: 0.9rem; color: var(--vp-c-text-2)">
      Comprehensive guides and API reference
    </p>
  </div>
</div>
