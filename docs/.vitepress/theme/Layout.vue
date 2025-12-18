<template>
  <DefaultTheme.Layout>
    <template #nav-bar-content-after>
      <GitHubStarBadge />
    </template>
  </DefaultTheme.Layout>
</template>

<script setup>
import DefaultTheme from 'vitepress/theme';
import GitHubStarBadge from '../components/GitHubStarBadge.vue';
</script>
