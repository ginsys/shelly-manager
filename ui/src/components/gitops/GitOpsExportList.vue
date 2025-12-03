<template>
  <DataTable :rows="rows" :loading="loading" :error="error || ''" :cols="8" :rowKey="(row: any) => row.export_id">
    <template #header>
      <th>Name</th>
      <th>Format</th>
      <th>Structure</th>
      <th>Files</th>
      <th>Size</th>
      <th>Status</th>
      <th>Created</th>
      <th>Actions</th>
    </template>
    <template #row="{ row }">
      <td>
        <div class="export-name">
          <strong>{{ row.name }}</strong>
          <div class="export-description" v-if="row.description">{{ row.description }}</div>
          <div class="export-id">ID: {{ row.export_id }}</div>
        </div>
      </td>
      <td>
        <span class="format-badge" :class="formatClass(row.format)">{{ formatLabel(row.format) }}</span>
      </td>
      <td>
        <span class="structure-badge">{{ structureLabel(row.repository_structure) }}</span>
      </td>
      <td>
        <div class="file-count">
          {{ row.file_count }} files
          <div class="device-count">{{ row.device_count }} devices</div>
        </div>
      </td>
      <td>
        <div v-if="row.total_size" class="file-size">{{ formatFileSize(row.total_size) }}</div>
        <span v-else class="no-data">—</span>
      </td>
      <td>
        <span :class="['status-badge', row.success ? 'success' : 'failure']">{{ row.success ? 'Success' : 'Failed' }}</span>
        <div v-if="!row.success && row.error_message" class="error-message">{{ row.error_message }}</div>
      </td>
      <td>
        <div class="time-info">
          {{ formatDate(row.created_at) }}
          <div class="created-by" v-if="row.created_by">by {{ row.created_by }}</div>
        </div>
      </td>
      <td>
        <div class="action-buttons">
          <button v-if="row.success" class="action-btn preview-btn" @click="$emit('preview', row)">👁 Preview</button>
          <button v-if="row.success" class="action-btn download-btn" @click="$emit('download', row.export_id)">⬇ Download</button>
          <button class="action-btn delete-btn" @click="$emit('delete', row)">🗑️</button>
        </div>
      </td>
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import DataTable from '@/components/DataTable.vue'
import { formatLabel, structureLabel, formatFileSize, formatDate } from '@/utils/format'

defineProps<{ rows: any[]; loading: boolean; error?: string | null }>()
defineEmits<{ preview: [any]; download: [string]; delete: [any] }>()

function formatClass(f: string) { return (f || '').replace(/\s+/g, '-') }
</script>

<style scoped>
.export-name { display: flex; flex-direction: column; gap: 2px }
.export-description { color: #6b7280; font-size: .875rem }
.export-id { color: #94a3b8; font-size: .75rem }
.format-badge { background: #eef2ff; color: #3730a3; padding: 2px 6px; border-radius: 4px; font-size: .75rem }
.structure-badge { background: #f3f4f6; color: #374151; padding: 2px 6px; border-radius: 4px; font-size: .75rem }
.file-size { display: flex; align-items: center; gap: 6px }
.no-data { color: #9ca3af }
.status-badge { padding: 2px 6px; border-radius: 4px; font-size: .75rem; font-weight: 600 }
.status-badge.success { background: #dcfce7; color: #065f46 }
.status-badge.failure { background: #fee2e2; color: #991b1b }
.time-info { display: flex; flex-direction: column; gap: 2px }
.created-by { color: #94a3b8; font-size: .75rem }
.action-buttons { display: flex; gap: 6px }
.action-btn { padding: 4px 8px; border: 1px solid #e5e7eb; border-radius: 4px; background: #fff; font-size: .75rem; cursor: pointer }
.preview-btn:hover { background: #f0fdf4; border-color: #10b981; color: #047857 }
.download-btn:hover { background: #f0f9ff; border-color: #0ea5e9; color: #0369a1 }
.delete-btn:hover { background: #fee2e2; border-color: #ef4444; color: #991b1b }
</style>
