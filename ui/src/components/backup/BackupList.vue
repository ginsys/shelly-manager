<template>
  <DataTable
    :rows="backups"
    :loading="loading"
    :error="error"
    :cols="8"
    :rowKey="(row: any) => row.backup_id"
  >
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
          <button
            v-if="row.success"
            class="action-btn download-btn"
            @click="emit('download', row.backup_id, row.name)"
            :disabled="downloading === row.backup_id"
            title="Download backup"
          >
            <span v-if="downloading === row.backup_id">⏳ Downloading...</span>
            <span v-else>⬇ Download</span>
          </button>
          <button
            v-if="row.success"
            class="action-btn restore-btn"
            @click="emit('restore', row)"
            title="Restore from backup"
          >
            ↩ Restore
          </button>
          <button
            class="action-btn delete-btn"
            @click="emit('delete', row)"
            title="Delete backup"
          >
            🗑️
          </button>
        </div>
      </td>
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import type { BackupItem } from '@/api/export'
import DataTable from '@/components/DataTable.vue'

interface Props {
  backups: BackupItem[]
  loading: boolean
  error: string
  downloading?: string
}

defineProps<Props>()

const emit = defineEmits<{
  download: [backupId: string, backupName: string]
  restore: [backup: BackupItem]
  delete: [backup: BackupItem]
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
.backup-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.backup-description {
  font-size: 0.75rem;
  color: #6b7280;
}

.backup-id {
  font-size: 0.75rem;
  color: #9ca3af;
  font-family: monospace;
}

.format-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  background: #dbeafe;
  color: #1e40af;
}

.file-size {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.checksum {
  font-size: 0.75rem;
  color: #6b7280;
  font-family: monospace;
}

.no-data {
  color: #9ca3af;
}

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}

.status-badge.success {
  background: #d1fae5;
  color: #065f46;
}

.status-badge.failure {
  background: #fee2e2;
  color: #991b1b;
}

.error-message {
  font-size: 0.75rem;
  color: #dc2626;
  margin-top: 4px;
}

.encryption-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}

.encryption-badge.encrypted {
  background: #fef3c7;
  color: #92400e;
}

.encryption-badge.plain {
  background: #f3f4f6;
  color: #6b7280;
}

.time-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.created-by {
  font-size: 0.75rem;
  color: #6b7280;
}

.action-buttons {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
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

.action-btn:hover:not(:disabled) {
  background: #f3f4f6;
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.download-btn:hover:not(:disabled) {
  border-color: #3b82f6;
  color: #3b82f6;
}

.restore-btn:hover:not(:disabled) {
  border-color: #10b981;
  color: #10b981;
}

.delete-btn:hover:not(:disabled) {
  border-color: #ef4444;
  color: #ef4444;
}
</style>
