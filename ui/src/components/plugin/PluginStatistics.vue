<template>
  <section class="stats-section">
    <div class="stats">
      <div class="card">
        <span class="stat-label">Total Plugins:</span>
        <span class="stat-value">{{ statistics.total }}</span>
      </div>
      <!-- Only truthful counts: total + per-category. Configured/enabled/error
           tallies were fiction (backend hardcodes status) and were removed. -->
      <div
        v-for="[category, count] in categoryEntries"
        :key="category"
        class="card"
      >
        <span class="stat-label">{{ category }}:</span>
        <span class="stat-value">{{ count }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface PluginStats {
  total: number
  byCategory: Record<string, number>
}

const props = defineProps<{
  statistics: PluginStats
}>()

const categoryEntries = computed(() => Object.entries(props.statistics.byCategory))
</script>

<style scoped>
.stats-section {
  margin-bottom: 24px;
}

.stats {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.card {
  border: 1px solid #e5e7eb;
  padding: 16px;
  border-radius: 6px;
  background: #ffffff;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 140px;
}

.stat-label {
  font-weight: 500;
  color: #6b7280;
}

.stat-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
}

.stat-value.success {
  color: #10b981;
}

.stat-value.available {
  color: #3b82f6;
}

.stat-value.disabled {
  color: #f59e0b;
}

.stat-value.error {
  color: #ef4444;
}
</style>
