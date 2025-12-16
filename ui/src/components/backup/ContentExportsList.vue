<template>
  <section style="margin-top:24px">
    <h2>Content Exports (JSON / YAML / SMA)</h2>
    <DataTable
      :rows="contentExports"
      :loading="loading"
      :error="error"
      :cols="6"
      :rowKey="(row: any) => row.export_id"
    >
      <template #header>
        <th>Plugin</th>
        <th>Format</th>
        <th>Records</th>
        <th>Size</th>
        <th>Created</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.plugin_name }}</td>
        <td><span class="format-badge">{{ row.format?.toUpperCase?.() || row.plugin_name?.toUpperCase?.() }}</span></td>
        <td>{{ row.record_count ?? '—' }}</td>
        <td>
          <span v-if="row.file_size">{{ formatFileSize(row.file_size) }}</span>
          <span v-else class="no-data">—</span>
        </td>
        <td>{{ formatDate(row.created_at) }}</td>
        <td>
          <button class="action-btn download-btn" @click="emit('download', row.export_id)">⬇ Download</button>
        </td>
      </template>
    </DataTable>
  </section>
</template>

<script setup lang="ts">
import DataTable from '@/components/DataTable.vue'

interface ContentExport {
  export_id: string
  plugin_name: string
  format?: string
  record_count?: number
  file_size?: number
  created_at: string
  success: boolean
}

interface Props {
  contentExports: ContentExport[]
  loading?: boolean
  error?: string
}

defineProps<Props>()

const emit = defineEmits<{
  download: [exportId: string]
}>()

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

/**
 * Format date for display
 */
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString()
}
</script>

<style scoped>
.format-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  background: #dbeafe;
  color: #1e40af;
}

.no-data {
  color: #9ca3af;
}

.action-btn {
  padding: 4px 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: white;
  font-size: 0.75rem;
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover {
  background: #f3f4f6;
}

.download-btn:hover {
  border-color: #3b82f6;
  color: #3b82f6;
}
</style>
