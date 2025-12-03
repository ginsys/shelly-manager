<template>
  <section class="stats-section">
    <div class="stats">
      <div class="card"><span class="stat-label">Total:</span><span class="stat-value">{{ stats.total || 0 }}</span></div>
      <div class="card"><span class="stat-label">Success:</span><span class="stat-value success">{{ stats.success || 0 }}</span></div>
      <div class="card"><span class="stat-label">Failed:</span><span class="stat-value failure">{{ stats.failure || 0 }}</span></div>
      <div class="card"><span class="stat-label">Files:</span><span class="stat-value">{{ stats.total_files || 0 }}</span></div>
      <div class="card"><span class="stat-label">Total Size:</span><span class="stat-value">{{ formatFileSize(stats.total_size || 0) }}</span></div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { formatFileSize } from '@/utils/format'
defineProps<{ stats: { total?: number; success?: number; failure?: number; total_files?: number; total_size?: number } }>()
</script>

<style scoped>
.stats-section { margin: 16px 0 }
.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px }
.card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; background: #fff }
.stat-label { color: #6b7280; margin-right: 6px }
.stat-value { color: #111827; font-weight: 700 }
.stat-value.success { color: #059669 }
.stat-value.failure { color: #dc2626 }
</style>
