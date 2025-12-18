---
layout: doc
title: Blog
sidebar: false
editLink: false
aside: false
prev: false
next: false
---

<script setup>
import { data as posts } from './posts.data.js'

const formatDate = (dateStr) => {
  const date = new Date(dateStr)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
}

const featured = posts[0]
const rest = posts.slice(1)
</script>

<div class="blog-header">
  <h1>Blog</h1>
  <p class="subtitle">Insights and lessons learned from building Lynq</p>
</div>

<div v-if="featured" class="featured-post">
  <span class="featured-label">Latest</span>
  <h2>
    <a :href="featured.url">{{ featured.title }}</a>
  </h2>
  <div class="featured-meta">
    <div class="author-info">
      <img v-if="featured.github" :src="`https://github.com/${featured.github}.png?size=40`" :alt="featured.author" class="author-avatar" />
      <span class="author">{{ featured.author }}</span>
    </div>
    <span class="separator">·</span>
    <span class="date">{{ formatDate(featured.date) }}</span>
    <span class="separator">·</span>
    <span class="reading-time">{{ featured.readingTime }} min read</span>
  </div>
  <p class="featured-description">{{ featured.description }}</p>
  <div v-if="featured.tags.length" class="tags">
    <span v-for="tag of featured.tags" :key="tag" class="tag">{{ tag }}</span>
  </div>
</div>

<div v-if="rest.length" class="blog-list">
  <article v-for="post of rest" :key="post.url" class="blog-item">
    <h3>
      <a :href="post.url">{{ post.title }}</a>
    </h3>
    <div class="blog-meta">
      <div class="author-info">
        <img v-if="post.github" :src="`https://github.com/${post.github}.png?size=32`" :alt="post.author" class="author-avatar small" />
        <span class="author">{{ post.author }}</span>
      </div>
      <span class="separator">·</span>
      <span class="date">{{ formatDate(post.date) }}</span>
      <span class="separator">·</span>
      <span class="reading-time">{{ post.readingTime }} min read</span>
    </div>
    <p class="description">{{ post.description }}</p>
  </article>
</div>

<div v-if="posts.length === 0" class="empty-state">
  <p>No posts yet. Stay tuned!</p>
</div>

<style scoped>
.blog-header {
  text-align: center;
  margin-bottom: 3rem;
  padding-bottom: 2rem;
  border-bottom: 1px solid var(--vp-c-divider);
}

.blog-header h1 {
  font-size: 2.5rem;
  margin: 0 0 0.5rem 0;
  border: none;
  padding: 0;
}

.blog-header .subtitle {
  font-size: 1.1rem;
  color: var(--vp-c-text-2);
  margin: 0;
}

.featured-post {
  background: var(--vp-c-bg-soft);
  border-radius: 12px;
  padding: 2rem;
  margin-bottom: 3rem;
  position: relative;
}

.featured-label {
  display: inline-block;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  padding: 0.25rem 0.75rem;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.featured-post h2 {
  margin: 0 0 0.75rem 0;
  font-size: 1.75rem;
  border: none;
  padding: 0;
}

.featured-post h2 a {
  color: var(--vp-c-text-1);
  text-decoration: none;
}

.featured-post h2 a:hover {
  color: var(--vp-c-brand-1);
}

.featured-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0;
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
  margin-bottom: 1rem;
}

.separator {
  margin: 0 0.5rem;
}

.author-info {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.author-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid var(--vp-c-divider);
}

.author-avatar.small {
  width: 22px;
  height: 22px;
}

.featured-description {
  color: var(--vp-c-text-2);
  line-height: 1.7;
  margin: 0 0 1rem 0;
  font-size: 1.05rem;
}

.tags {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.tag {
  font-size: 0.8rem;
  padding: 0.25rem 0.75rem;
  background: var(--vp-c-bg-alt);
  border-radius: 4px;
  color: var(--vp-c-text-2);
}

.blog-list {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.blog-item {
  padding: 1.5rem;
  background: var(--vp-c-bg-soft);
  border-radius: 8px;
  transition: transform 0.2s, box-shadow 0.2s;
}

.blog-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.blog-item h3 {
  margin: 0 0 0.5rem 0;
  font-size: 1.25rem;
}

.blog-item h3 a {
  color: var(--vp-c-text-1);
  text-decoration: none;
}

.blog-item h3 a:hover {
  color: var(--vp-c-brand-1);
}

.blog-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0;
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
  margin-bottom: 0.75rem;
}

.description {
  color: var(--vp-c-text-2);
  margin: 0;
  line-height: 1.6;
  font-size: 0.95rem;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--vp-c-text-2);
}
</style>
