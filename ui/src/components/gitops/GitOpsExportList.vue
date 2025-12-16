<template>
  <DataTable
    :rows="exports"
    :loading="loading"
    :error="error"
    :cols="8"
    :rowKey="(row: any) => row.export_id"
  >
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
        <div v-if="row.total_size" class="file-size">
          {{ formatFileSize(row.total_size) }}
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
            class="action-btn preview-btn"
            @click="emit('preview', row)"
            title="Preview structure"
          >
            👁️
          </button>
          <button
            v-if="row.success"
            class="action-btn download-btn"
            @click="emit('download', row.export_id, row.name)"
            :disabled="downloading === row.export_id"
            title="Download export"
          >
            <span v-if="downloading === row.export_id">⏳</span>
            <span v-else>📥</span>
          </button>
          <button
            class="action-btn delete-btn"
            @click="emit('delete', row)"
            title="Delete export"
          >
            🗑️
          </button>
        </div>
      </td>
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import type { GitOpsExportItem } from '@/api/export'
import DataTable from '@/components/DataTable.vue'

interface Props {
  exports: GitOpsExportItem[]
  loading: boolean
  error: string
  downloading: string
}

defineProps<Props>()

const emit = defineEmits<{
  preview: [exportItem: GitOpsExportItem]
  download: [exportId: string, exportName: string]
  delete: [exportItem: GitOpsExportItem]
}>()

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString()
}

function formatLabel(format: string): string {
  const labels: Record<string, string> = {
    terraform: 'Terraform',
    ansible: 'Ansible',
    kubernetes: 'Kubernetes',
    'docker-compose': 'Docker Compose',
    yaml: 'YAML'
  }
  return labels[format] || format.toUpperCase()
}

function formatClass(format: string): string {
  return `format-${format.replace(/[^a-zA-Z0-9]/g, '-').toLowerCase()}`
}

function structureLabel(structure: string): string {
  const labels: Record<string, string> = {
    monorepo: 'Monorepo',
    hierarchical: 'Hierarchical',
    'per-device': 'Per Device',
    flat: 'Flat'
  }
  return labels[structure] || structure
}
</script>

<style scoped>
.export-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.export-description {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
}

.export-id {
  font-size: 0.75rem;
  color: #6b7280;
  font-family: monospace;
}

.format-badge {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.format-terraform {
  background: #ede9fe;
  color: #6b21a8;
}

.format-ansible {
  background: #fef3c7;
  color: #92400e;
}

.format-kubernetes {
  background: #dbeafe;
  color: #1e40af;
}

.format-docker-compose {
  background: #dcfce7;
  color: #166534;
}

.format-yaml {
  background: #f3f4f6;
  color: #374151;
}

.structure-badge {
  background: #e5e7eb;
  color: #374151;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.file-count {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.device-count {
  font-size: 0.75rem;
  color: #6b7280;
}

.file-size {
  font-weight: 500;
}

.status-badge {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: uppercase;
}

.status-badge.success {
  background: #dcfce7;
  color: #166534;
}

.status-badge.failure {
  background: #fee2e2;
  color: #991b1b;
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

.error-message {
  font-size: 0.75rem;
  color: #dc2626;
  margin-top: 2px;
}

.no-data {
  color: #9ca3af;
  font-style: italic;
}

.action-buttons {
  display: flex;
  gap: 4px;
  align-items: center;
}

.action-btn {
  background: none;
  border: 1px solid #d1d5db;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 0.875rem;
}

.action-btn:hover:not(:disabled) {
  background: #f3f4f6;
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.preview-btn:hover:not(:disabled) {
  background: #dbeafe;
  border-color: #3b82f6;
}

.download-btn:hover:not(:disabled) {
  background: #dcfce7;
  border-color: #10b981;
}

.delete-btn:hover:not(:disabled) {
  background: #fee2e2;
  border-color: #dc2626;
}
</style>
