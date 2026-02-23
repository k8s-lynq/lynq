<template>
  <section class="blog-section">
    <div class="container">
      <div class="section-header fade-up">
        <span class="section-label">Blog</span>
        <h2>Latest from the Blog</h2>
        <p class="section-subtitle">Insights on Infrastructure as Data, Kubernetes operations, and Lynq updates</p>
      </div>

      <div class="blog-grid">
        <a
          v-for="(post, index) in posts"
          :key="post.url"
          :href="post.url"
          class="blog-card fade-up"
          :style="{ animationDelay: `${0.1 * (index + 1)}s` }"
        >
          <div class="blog-card-inner">
            <div class="blog-meta">
              <time :datetime="post.date">{{ formatDate(post.date) }}</time>
              <span class="blog-tag" v-if="post.tags && post.tags.length">{{ post.tags[0] }}</span>
            </div>
            <h3>{{ post.title }}</h3>
            <p>{{ post.description }}</p>
            <span class="read-more">
              Read article
              <span class="arrow">&#8594;</span>
            </span>
          </div>
        </a>
      </div>

      <div class="blog-cta fade-up" style="animation-delay: 0.4s">
        <a href="/blog/" class="view-all">
          View all posts
          <span class="arrow">&#8594;</span>
        </a>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { data as allPosts } from '../../../blog/posts.data.js'

const posts = computed(() => allPosts.slice(0, 3))

function formatDate(raw) {
  const date = raw instanceof Date ? raw : new Date(raw)
  return date.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}
</script>

<style scoped>
.blog-section {
  padding: 6rem 2rem;
  background: linear-gradient(180deg, #111118 0%, #0a0a0f 100%);
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
  color: #a78bfa;
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

.blog-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.5rem;
  margin-bottom: 2.5rem;
}

.blog-card {
  text-decoration: none;
  color: inherit;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  transition: all 0.3s ease;
  overflow: hidden;
}

.blog-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(167, 139, 250, 0.3);
  transform: translateY(-4px);
}

.blog-card-inner {
  padding: 1.75rem;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.blog-meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.blog-meta time {
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.4);
}

.blog-tag {
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0.2rem 0.6rem;
  background: rgba(167, 139, 250, 0.15);
  color: #a78bfa;
  border-radius: 100px;
}

.blog-card h3 {
  font-size: 1.15rem;
  font-weight: 600;
  color: #fff;
  margin: 0 0 0.75rem;
  line-height: 1.4;
}

.blog-card p {
  font-size: 0.9rem;
  color: rgba(255, 255, 255, 0.5);
  line-height: 1.6;
  margin: 0 0 1.25rem;
  flex: 1;
}

.read-more {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.85rem;
  font-weight: 500;
  color: #a78bfa;
  transition: all 0.3s ease;
}

.blog-card:hover .read-more {
  color: #c4b5fd;
}

.read-more .arrow {
  transition: transform 0.3s ease;
}

.blog-card:hover .read-more .arrow {
  transform: translateX(4px);
}

.blog-cta {
  text-align: center;
}

.view-all {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1rem;
  font-weight: 500;
  color: #a78bfa;
  text-decoration: none;
  transition: all 0.3s ease;
}

.view-all:hover {
  color: #c4b5fd;
}

.view-all .arrow {
  transition: transform 0.3s ease;
}

.view-all:hover .arrow {
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

@media (max-width: 900px) {
  .blog-grid {
    grid-template-columns: 1fr;
  }
}
</style>
