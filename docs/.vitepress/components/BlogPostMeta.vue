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
  border-bottom: 1px solid var(--vp-c-divider);
}

.meta-info {
  font-size: 0.95rem;
  color: var(--vp-c-text-2);
  margin-bottom: 0.75rem;
}

.separator {
  margin: 0 0.5rem;
}

.tags {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.tag {
  font-size: 0.8rem;
  padding: 0.25rem 0.75rem;
  background: var(--vp-c-bg-soft);
  border-radius: 4px;
  color: var(--vp-c-text-2);
}
</style>
