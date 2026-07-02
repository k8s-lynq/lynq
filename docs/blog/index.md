---
layout: doc
title: Blog
sidebar: false
editLink: false
aside: false
prev: false
next: false
pageClass: blog-page
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
/* Tone-matched to the landing page: Pretendard headings, teal accent, pure
   near-black cards with hairline borders and the 18px landing radius. Uses the
   global --lynq-* tokens (declared in tailwind.css :root). */
.blog-header {
  text-align: center;
  margin-bottom: 3.5rem;
  padding-bottom: 2rem;
  border-bottom: 1px solid var(--lynq-border);
}

.blog-header h1 {
  font-family: var(--lynq-font);
  font-size: clamp(2.4rem, 5vw, 3.25rem);
  font-weight: 500;
  letter-spacing: -0.03em;
  color: var(--lynq-text);
  margin: 0 0 0.6rem 0;
  border: none;
  padding: 0;
}

.blog-header .subtitle {
  font-size: 1.1rem;
  color: var(--lynq-text-dim);
  margin: 0;
}

.featured-post {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius);
  padding: 2rem;
  margin-bottom: 2.5rem;
  transition: border-color 0.25s ease, transform 0.25s ease;
}

.featured-post:hover {
  border-color: rgba(255, 255, 255, 0.16);
}

.featured-label {
  display: inline-block;
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--lynq-accent);
  background: color-mix(in srgb, var(--lynq-purple) 15%, transparent);
  padding: 0.25rem 0.7rem;
  border-radius: 999px;
  margin-bottom: 1.1rem;
}

.featured-post h2 {
  font-family: var(--lynq-font);
  margin: 0 0 0.75rem 0;
  font-size: 1.8rem;
  font-weight: 500;
  letter-spacing: -0.02em;
  border: none;
  padding: 0;
}

.featured-post h2 a {
  color: var(--lynq-text);
  text-decoration: none;
}

.featured-post h2 a:hover {
  color: var(--lynq-accent);
}

.featured-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0;
  font-size: 0.9rem;
  color: var(--lynq-text-faint);
  margin-bottom: 1rem;
}

.separator {
  margin: 0 0.5rem;
  color: var(--lynq-text-faint);
}

.author-info {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.author {
  color: var(--lynq-text-dim);
}

.author-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid var(--lynq-border);
}

.author-avatar.small {
  width: 22px;
  height: 22px;
}

.featured-description {
  color: var(--lynq-text-dim);
  line-height: 1.7;
  margin: 0 0 1.25rem 0;
  font-size: 1.05rem;
}

.tags {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.tag {
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  padding: 0.2rem 0.6rem;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--lynq-border);
  border-radius: 999px;
  color: var(--lynq-text-dim);
}

.blog-list {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.blog-item {
  padding: 1.5rem 1.75rem;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius);
  transition: transform 0.25s ease, border-color 0.25s ease, box-shadow 0.25s ease;
}

.blog-item:hover {
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.16);
  box-shadow: 0 10px 30px -12px rgba(0, 0, 0, 0.55);
}

.blog-item h3 {
  font-family: var(--lynq-font);
  margin: 0 0 0.5rem 0;
  font-size: 1.3rem;
  font-weight: 500;
  letter-spacing: -0.015em;
}

.blog-item h3 a {
  color: var(--lynq-text);
  text-decoration: none;
}

.blog-item h3 a:hover {
  color: var(--lynq-accent);
}

.blog-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0;
  font-size: 0.85rem;
  color: var(--lynq-text-faint);
  margin-bottom: 0.75rem;
}

.description {
  color: var(--lynq-text-dim);
  margin: 0;
  line-height: 1.6;
  font-size: 0.95rem;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--lynq-text-dim);
}
</style>
