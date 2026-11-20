<template>
  <DataTable
    :rows="exports"
    :loading="loading"
    :error="error"
    :cols="5"
    :rowKey="(row: any) => row.export_id"
  >
    <template #header>
      <th>Name</th>
      <th>Files</th>
      <th>Size</th>
      <th>Status</th>
      <th>Created</th>
      <th>Actions</th>
    </template>
    <template #row="{ row }">
      <td>
        <strong>{{ row.name || row.export_id }}</strong>
        <div class="muted" v-if="row.description">{{ row.description }}</div>
        <div class="muted path" v-if="row.file_path">{{ row.file_path }}</div>
      </td>
      <td>{{ row.record_count ?? 0 }}</td>
      <td>{{ row.file_size ? formatFileSize(row.file_size) : '—' }}</td>
      <td>
        <span :class="['status-badge', row.success ? 'success' : 'failure']">
          {{ row.success ? 'Success' : 'Failed' }}
        </span>
        <div v-if="!row.success && row.error_message" class="error-message">{{ row.error_message }}</div>
      </td>
      <td>{{ new Date(row.created_at).toLocaleString() }}</td>
      <td>
        <button class="action-btn" @click="emit('details', row)" title="Show the export record">👁️</button>
        <button class="action-btn" @click="emit('delete', row)" title="Delete the record (the files stay)">🗑️</button>
      </td>
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import type { ExportHistoryItem } from '@/api/export'
import DataTable from '@/components/DataTable.vue'

defineProps<{
  exports: ExportHistoryItem[]
  loading: boolean
  error: string
}>()

const emit = defineEmits<{
  details: [exportItem: ExportHistoryItem]
  delete: [exportItem: ExportHistoryItem]
}>()

function formatFileSize(bytes: number): string {
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = bytes > 0 ? Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), sizes.length - 1) : 0
  return parseFloat((bytes / Math.pow(1024, i)).toFixed(1)) + ' ' + sizes[i]
}
</script>

<style scoped>
.muted { font-size: 0.75rem; color: #6b7280; }
.path { font-family: monospace; word-break: break-all; }
.status-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
.status-badge.success { background: #d1fae5; color: #065f46; }
.status-badge.failure { background: #fee2e2; color: #991b1b; }
.error-message { font-size: 0.75rem; color: #dc2626; margin-top: 4px; }
.action-btn { padding: 4px 8px; margin-right: 4px; border: 1px solid #d1d5db; border-radius: 4px; background: white; cursor: pointer; }
.action-btn:hover { background: #f3f4f6; }
</style>
