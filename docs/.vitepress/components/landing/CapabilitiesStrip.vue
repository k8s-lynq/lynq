<template>
  <div class="caps-strip">
    <div class="container">
      <div class="caps-grid">
        <div class="cap-item" v-for="cap in caps" :key="cap.label">
          <span class="cap-value">{{ cap.value }}</span>
          <span class="cap-label">{{ cap.label }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
const caps = [
  { value: '15', label: 'resource types' },
  { value: '3', label: 'lifecycle policies' },
  { value: 'MySQL & PostgreSQL', label: 'built-in datasources' },
  { value: 'Kubernetes 1.28+', label: 'cluster requirement' },
  { value: 'MIT', label: 'open source license' },
]
</script>

<style scoped>
.caps-strip {
  background: rgba(255, 255, 255, 0.02);
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  padding: 1.75rem 2rem;
}

.container {
  max-width: 1200px;
  margin: 0 auto;
}

.caps-grid {
  display: flex;
  justify-content: center;
  gap: 0;
  flex-wrap: wrap;
}

.cap-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 0.75rem 2.5rem;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  gap: 0.25rem;
}

.cap-item:last-child {
  border-right: none;
}

.cap-value {
  font-size: 1.125rem;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.01em;
  white-space: nowrap;
}

.cap-label {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.45);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .caps-grid {
    gap: 0.5rem;
  }

  .cap-item {
    padding: 0.5rem 1.25rem;
    border-right: none;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    width: calc(50% - 0.5rem);
    align-items: flex-start;
  }

  .cap-item:last-child {
    border-bottom: none;
  }
}
</style>
