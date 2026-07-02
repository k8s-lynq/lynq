<script setup>
import { useData } from 'vitepress'

const { frontmatter } = useData()

const formatDate = (dateStr) => {
  const date = new Date(dateStr)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
}
</script>

<template>
  <div class="blog-post-meta">
    <div class="meta-info">
      <span class="date">{{ formatDate(frontmatter.date) }}</span>
      <span class="separator">·</span>
      <span class="author">{{ frontmatter.author }}</span>
    </div>
    <div v-if="frontmatter.tags?.length" class="tags">
      <span v-for="tag of frontmatter.tags" :key="tag" class="tag">{{ tag }}</span>
    </div>
  </div>
</template>

<style scoped>
.blog-post-meta {
  margin-top: 1rem;
  margin-bottom: 2rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid var(--lynq-border);
}

.meta-info {
  font-size: 0.95rem;
  color: var(--lynq-text-dim);
  margin-bottom: 0.75rem;
}

.separator {
  margin: 0 0.5rem;
  color: var(--lynq-text-faint);
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
</style>
