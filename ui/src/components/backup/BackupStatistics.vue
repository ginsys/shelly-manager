<template>
  <section class="stats-section">
    <div class="stats">
      <div class="card">
        <span class="stat-label">Total:</span>
        <span class="stat-value">{{ statistics.total || 0 }}</span>
      </div>
      <div class="card">
        <span class="stat-label">Success:</span>
        <span class="stat-value success">{{ statistics.success || 0 }}</span>
      </div>
      <div class="card">
        <span class="stat-label">Failed:</span>
        <span class="stat-value failure">{{ statistics.failure || 0 }}</span>
      </div>
      <div class="card">
        <span class="stat-label">Total Size:</span>
        <span class="stat-value">{{ formatFileSize(statistics.total_size || 0) }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { BackupStatistics } from '@/api/export'

interface Props {
  statistics: BackupStatistics
}

defineProps<Props>()

/**
 * Format file size for display
 */
function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
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
  min-width: 120px;
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

.stat-value.failure {
  color: #ef4444;
}
</style>
