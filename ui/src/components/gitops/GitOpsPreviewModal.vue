<template>
  <div v-if="show" class="modal-overlay" @click="emit('close')">
    <div class="modal-content preview-modal" @click.stop>
      <div class="modal-header">
        <h3>Export Preview: {{ exportItem?.name }}</h3>
        <button class="close-button" @click="emit('close')">✖</button>
      </div>

      <div class="preview-content" v-if="previewData">
        <div class="preview-summary">
          <div class="summary-item">
            <strong>Format:</strong> {{ formatLabel(exportItem?.format || '') }}
          </div>
          <div class="summary-item">
            <strong>Structure:</strong> {{ structureLabel(exportItem?.repository_structure || '') }}
          </div>
          <div class="summary-item">
            <strong>Files:</strong> {{ previewData.file_count }} files
          </div>
          <div class="summary-item">
            <strong>Size:</strong> {{ formatFileSize(previewData.total_size) }}
          </div>
        </div>

        <div v-if="previewData.files?.length" class="file-structure">
          <h4>File Structure</h4>
          <div class="file-tree">
            <div
              v-for="file in previewData.files"
              :key="file.path"
              class="file-item"
              :class="'file-type-' + file.type"
            >
              <span class="file-icon">{{ getFileIcon(file.type) }}</span>
              <span class="file-path">{{ file.path }}</span>
              <span class="file-size">{{ formatFileSize(file.size) }}</span>
              <div v-if="file.description" class="file-description">{{ file.description }}</div>
            </div>
          </div>
        </div>

        <div v-if="previewData.git_integration" class="git-integration">
          <h4>Git Integration</h4>
          <div class="integration-details">
            <div class="integration-item" :class="{ active: previewData.git_integration.repository_connected }">
              <span class="integration-icon">
                {{ previewData.git_integration.repository_connected ? '🔗' : '🔌' }}
              </span>
              Repository: {{ previewData.git_integration.repository_connected ? 'Connected' : 'Not Connected' }}
            </div>
            <div v-if="previewData.git_integration.last_commit" class="integration-item">
              <span class="integration-icon">📝</span>
              Last Commit: {{ previewData.git_integration.last_commit }}
            </div>
          </div>
        </div>

        <div v-if="previewData.warnings?.length" class="warnings-section">
          <h4>⚠️ Warnings</h4>
          <ul class="warnings-list">
            <li v-for="warning in previewData.warnings" :key="warning" class="warning-item">
              {{ warning }}
            </li>
          </ul>
        </div>
      </div>

      <div class="modal-actions">
        <button class="secondary-button" @click="emit('close')">Close</button>
        <button
          v-if="exportItem?.success"
          class="primary-button"
          @click="emit('download', exportItem.export_id, exportItem.name)"
        >
          📥 Download
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { GitOpsExportItem, GitOpsExportResult } from '@/api/export'

interface Props {
  show: boolean
  exportItem: GitOpsExportItem | null
  previewData: GitOpsExportResult | null
}

defineProps<Props>()

const emit = defineEmits<{
  close: []
  download: [exportId: string, exportName: string]
}>()

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
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

function structureLabel(structure: string): string {
  const labels: Record<string, string> = {
    monorepo: 'Monorepo',
    hierarchical: 'Hierarchical',
    'per-device': 'Per Device',
    flat: 'Flat'
  }
  return labels[structure] || structure
}

function getFileIcon(type: string): string {
  const icons: Record<string, string> = {
    config: '⚙️',
    template: '📋',
    variable: '🔢',
    readme: '📖',
    script: '📜'
  }
  return icons[type] || '📄'
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 16px;
}

.modal-content {
  background: white;
  border-radius: 8px;
  max-width: 900px;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
}

.preview-modal {
  max-width: 700px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid #e5e7eb;
}

.modal-header h3 {
  margin: 0;
  color: #1f2937;
}

.close-button {
  background: none;
  border: none;
  color: #6b7280;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 4px;
  line-height: 1;
  transition: color 0.2s;
}

.close-button:hover {
  color: #374151;
}

.preview-content {
  padding: 24px;
}

.preview-summary {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-bottom: 24px;
}

.summary-item {
  padding: 12px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 0.875rem;
}

.file-structure {
  margin-bottom: 24px;
}

.file-structure h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
}

.file-tree {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 16px;
  max-height: 300px;
  overflow-y: auto;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid #f1f5f9;
  font-size: 0.875rem;
}

.file-item:last-child {
  border-bottom: none;
}

.file-icon {
  font-size: 1rem;
  width: 20px;
  text-align: center;
}

.file-path {
  flex: 1;
  font-family: monospace;
}

.file-size {
  color: #6b7280;
  font-size: 0.75rem;
}

.file-description {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
  margin-left: 28px;
}

.git-integration {
  margin-bottom: 24px;
}

.git-integration h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
}

.integration-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.integration-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  font-size: 0.875rem;
}

.integration-item.active {
  background: #ecfdf5;
  border-color: #10b981;
  color: #065f46;
}

.integration-icon {
  font-size: 1rem;
}

.warnings-section {
  margin-bottom: 20px;
  padding: 16px;
  background: #fffbeb;
  border: 1px solid #fed7aa;
  border-radius: 6px;
}

.warnings-section h4 {
  margin: 0 0 12px 0;
  color: #d97706;
}

.warnings-list {
  margin: 0;
  padding-left: 20px;
}

.warning-item {
  color: #92400e;
  margin-bottom: 4px;
  font-size: 0.875rem;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding: 24px;
  border-top: 1px solid #e5e7eb;
}

.secondary-button {
  background: #f3f4f6;
  color: #374151;
  border: 1px solid #d1d5db;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.secondary-button:hover {
  background: #e5e7eb;
}

.primary-button {
  background-color: #3b82f6;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.primary-button:hover {
  background-color: #2563eb;
}

@media (max-width: 768px) {
  .preview-summary {
    grid-template-columns: 1fr;
  }

  .modal-content {
    margin: 8px;
  }
}
</style>
