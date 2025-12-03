<template>
  <div class="modal-overlay" @click="$emit('close')">
    <div class="modal-content preview-modal" @click.stop>
      <div class="modal-header">
        <h3>Export Preview: {{ item?.name }}</h3>
        <button class="close-button" @click="$emit('close')">✖</button>
      </div>

      <div class="preview-content" v-if="data">
        <div class="preview-summary">
          <div class="summary-item"><strong>Format:</strong> {{ formatLabel(item?.format || '') }}</div>
          <div class="summary-item"><strong>Structure:</strong> {{ structureLabel(item?.repository_structure || '') }}</div>
          <div class="summary-item"><strong>Files:</strong> {{ data.file_count }} files</div>
          <div class="summary-item"><strong>Size:</strong> {{ formatFileSize(data.total_size) }}</div>
        </div>

        <div v-if="data.files?.length" class="file-structure">
          <h4>File Structure</h4>
          <div class="file-tree">
            <div v-for="file in data.files" :key="file.path" class="file-item" :class="'file-type-' + file.type">
              <span class="file-icon">{{ getFileIcon(file.type) }}</span>
              <span class="file-path">{{ file.path }}</span>
              <span class="file-size">{{ formatFileSize(file.size) }}</span>
              <div v-if="file.description" class="file-description">{{ file.description }}</div>
            </div>
          </div>
        </div>

        <div v-if="data.git_integration" class="git-integration">
          <h4>Git Integration</h4>
          <div class="integration-details">
            <div class="integration-item" :class="{ active: data.git_integration.repository_connected }">
              <span class="integration-icon">{{ data.git_integration.repository_connected ? '🔗' : '🔌' }}</span>
              Repository: {{ data.git_integration.repository_connected ? 'Connected' : 'Not Connected' }}
            </div>
            <div v-if="data.git_integration.last_commit" class="integration-item">
              <span class="integration-icon">📝</span>
              Last Commit: {{ data.git_integration.last_commit }}
            </div>
          </div>
        </div>

        <div v-if="data.warnings?.length" class="warnings-section">
          <h4>⚠️ Warnings</h4>
          <ul class="warnings-list">
            <li v-for="warning in data.warnings" :key="warning" class="warning-item">{{ warning }}</li>
          </ul>
        </div>

        <div class="modal-actions">
          <button class="secondary-button" @click="$emit('close')">Close</button>
          <button v-if="item?.success" class="primary-button" :disabled="downloadingId === item?.export_id" @click="$emit('download', item?.export_id, item?.name || '')">
            <span v-if="downloadingId === item?.export_id">⏳</span>
            <span v-else>📥</span>
            Download
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatFileSize, formatLabel, structureLabel, getFileIcon } from '@/utils/format'

defineProps<{ item: any | null; data: any | null; downloadingId?: string | null }>()
defineEmits<{ close: []; download: [string, string] }>()
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, .45); display: flex; align-items: center; justify-content: center; z-index: 1000 }
.modal-content { background: #fff; border-radius: 8px; box-shadow: 0 25px 50px -12px rgba(0,0,0,.25); padding: 16px; max-width: 900px; width: 90% }
.modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px }
.close-button { border: 1px solid #e5e7eb; background: #fff; border-radius: 6px; padding: 4px 8px; cursor: pointer }
.preview-content { display: grid; gap: 12px }
.preview-summary { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap: 6px }
.file-tree { border: 1px solid #e5e7eb; border-radius: 8px; padding: 8px; max-height: 360px; overflow: auto }
.file-item { display: grid; grid-template-columns: 28px 1fr 120px; align-items: center; padding: 4px 6px; border-bottom: 1px solid #f1f5f9 }
.file-item:last-child { border-bottom: 0 }
.file-icon { font-size: 1rem; text-align: center }
.file-path { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace }
.file-size { color: #64748b; text-align: right }
.integration-details { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap: 8px }
.integration-item { display: flex; align-items: center; gap: 8px }
.integration-item.active { color: #065f46 }
.warnings-list { margin: 0; padding-left: 18px }
.warning-item { color: #92400e }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px }
.primary-button { background: #0ea5e9; color: #fff; border: 1px solid #0ea5e9; border-radius: 6px; padding: 6px 12px }
.secondary-button { background: #f8fafc; color: #334155; border: 1px solid #e2e8f0; border-radius: 6px; padding: 6px 12px }
</style>
