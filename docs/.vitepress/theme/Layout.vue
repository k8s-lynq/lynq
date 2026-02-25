<template>
  <DefaultTheme.Layout>
    <template #nav-bar-content-after>
      <GitHubStarBadge />
    </template>
    <template #layout-bottom>
      <SiteFooter />
    </template>
  </DefaultTheme.Layout>
</template>

<script setup>
import DefaultTheme from 'vitepress/theme';
import GitHubStarBadge from '../components/GitHubStarBadge.vue';
import SiteFooter from '../components/SiteFooter.vue';
</script>
