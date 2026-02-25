<template>
  <footer class="site-footer">
    <div class="footer-content">
      <div class="footer-left">
        <img src="/logo.png" alt="Lynq" class="footer-logo" />
        <span class="footer-tagline">Infrastructure as Data</span>
      </div>
      <div class="footer-links">
        <a href="/quickstart">Quickstart</a>
        <a href="/about-lynq">About</a>
        <a href="/policies">Policies</a>
        <a href="/blog/">Blog</a>
        <a href="https://github.com/k8s-lynq/lynq" target="_blank">GitHub</a>
      </div>
      <div class="footer-right">
        <span class="copyright">Open Source under Apache 2.0</span>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.site-footer {
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
  .footer-content {
    flex-direction: column;
    text-align: center;
  }

  .footer-right {
    text-align: center;
  }
}
</style>
