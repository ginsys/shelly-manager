<template>
  <DataTable :rows="rows" :loading="loading" :error="error || ''" :cols="8" :rowKey="(row: any) => row.backup_id">
    <template #header>
      <th>Name</th>
      <th>Format</th>
      <th>Devices</th>
      <th>Size</th>
      <th>Status</th>
      <th>Encrypted</th>
      <th>Created</th>
      <th>Actions</th>
    </template>
    <template #row="{ row }">
      <td>
        <div class="backup-name">
          <strong>{{ row.name }}</strong>
          <div class="backup-description" v-if="row.description">{{ row.description }}</div>
          <div class="backup-id">ID: {{ row.backup_id }}</div>
        </div>
      </td>
      <td>
        <span class="format-badge">{{ row.format.toUpperCase() }}</span>
      </td>
      <td>{{ row.device_count }}</td>
      <td>
        <div v-if="row.file_size" class="file-size">
          {{ formatFileSize(row.file_size) }}
          <div class="checksum" v-if="row.checksum">
            {{ row.checksum.substring(0, 8) }}...
          </div>
        </div>
        <span v-else class="no-data">—</span>
      </td>
      <td>
        <span :class="['status-badge', row.success ? 'success' : 'failure']">
          {{ row.success ? 'Success' : 'Failed' }}
        </span>
        <div v-if="!row.success && row.error_message" class="error-message">
          {{ row.error_message }}
        </div>
      </td>
      <td>
        <span :class="['encryption-badge', row.encrypted ? 'encrypted' : 'plain']">
          {{ row.encrypted ? '🔒 Yes' : '🔓 No' }}
        </span>
      </td>
      <td>
        <div class="time-info">
          {{ formatDate(row.created_at) }}
          <div class="created-by" v-if="row.created_by">
            by {{ row.created_by }}
          </div>
        </div>
      </td>
      <td>
        <div class="action-buttons">
          <button v-if="row.success" class="action-btn download-btn" @click="$emit('download', row.backup_id, row.name)" title="Download backup">
            ⬇ Download
          </button>
          <button v-if="row.success" class="action-btn restore-btn" @click="$emit('restore', row)" title="Restore from backup">
            ↩ Restore
          </button>
          <button class="action-btn delete-btn" @click="$emit('delete', row)" title="Delete backup">🗑️</button>
        </div>
      </td>
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import DataTable from '@/components/DataTable.vue'
import { formatFileSize, formatDate } from '@/utils/format'

defineProps<{ rows: any[]; loading: boolean; error?: string | null }>()
defineEmits<{ download: [string, string]; restore: [any]; delete: [any] }>()
</script>

<style scoped>
.backup-name { display: flex; flex-direction: column; gap: 2px }
.backup-description { color: #6b7280; font-size: .875rem }
.backup-id { color: #94a3b8; font-size: .75rem }
.format-badge { background: #eef2ff; color: #3730a3; padding: 2px 6px; border-radius: 4px; font-size: .75rem }
.file-size { display: flex; align-items: center; gap: 6px }
.checksum { color: #94a3b8; font-size: .75rem }
.no-data { color: #9ca3af }
.status-badge { padding: 2px 6px; border-radius: 4px; font-size: .75rem; font-weight: 600 }
.status-badge.success { background: #dcfce7; color: #065f46 }
.status-badge.failure { background: #fee2e2; color: #991b1b }
.encryption-badge { font-size: .875rem }
.time-info { display: flex; flex-direction: column; gap: 2px }
.created-by { color: #94a3b8; font-size: .75rem }
.action-buttons { display: flex; gap: 6px }
.action-btn { padding: 4px 8px; border: 1px solid #e5e7eb; border-radius: 4px; background: #fff; font-size: .75rem; cursor: pointer }
.download-btn:hover { background: #f0f9ff; border-color: #0ea5e9; color: #0369a1 }
.restore-btn:hover { background: #dcfce7; border-color: #10b981; color: #047857 }
.delete-btn:hover { background: #fee2e2; border-color: #ef4444; color: #991b1b }
</style>
