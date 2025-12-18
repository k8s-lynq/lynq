import { createContentLoader } from 'vitepress'

const WORDS_PER_MINUTE = 100

export default createContentLoader('blog/*.md', {
  excerpt: true,
  includeSrc: true,
  transform(raw) {
    return raw
      .filter(page => page.url !== '/blog/')
      .sort((a, b) => {
        return new Date(b.frontmatter.date) - new Date(a.frontmatter.date)
      })
      .map(page => {
        // Calculate reading time from source content
        const content = page.src || ''
        // Remove code blocks and frontmatter for more accurate count
        const textOnly = content
          .replace(/^---[\s\S]*?---/, '') // Remove frontmatter
          .replace(/```[\s\S]*?```/g, '') // Remove code blocks
          .replace(/`[^`]*`/g, '')        // Remove inline code
          .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1') // Keep link text only
          .replace(/[#*_~>\-|]/g, '')     // Remove markdown syntax
        const wordCount = textOnly.split(/\s+/).filter(w => w.length > 0).length
        const readingTime = Math.max(1, Math.ceil(wordCount / WORDS_PER_MINUTE))

        return {
          title: page.frontmatter.title,
          url: page.url,
          date: page.frontmatter.date,
          author: page.frontmatter.author || 'Lynq Team',
          github: page.frontmatter.github || null,
          description: page.frontmatter.description || page.excerpt,
          tags: page.frontmatter.tags || [],
          readingTime,
        }
      })
  }
})
