<template>
  <div class="blog-post-footer">
    <a href="/blog/" class="back-link">
      <span class="arrow">←</span>
      <span>Back to Blog</span>
    </a>
  </div>
</template>

<style scoped>
.blog-post-footer {
  margin-top: 3rem;
  padding-top: 2rem;
  border-top: 1px solid var(--vp-c-divider);
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--vp-c-text-2);
  text-decoration: none;
  font-size: 0.95rem;
  transition: color 0.2s;
}

.back-link:hover {
  color: var(--vp-c-brand-1);
}

.arrow {
  font-size: 1.1rem;
}
</style>
