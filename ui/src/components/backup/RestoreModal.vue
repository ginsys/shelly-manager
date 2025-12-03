<template>
  <div v-if="show" class="modal-overlay" @click="emit('close')">
    <div class="modal-content restore-modal" @click.stop>
      <div class="modal-header">
        <h3>Restore from Backup</h3>
        <button class="close-button" @click="emit('close')">✖</button>
      </div>

      <div class="restore-content">
        <div class="backup-info">
          <h4>{{ backup?.name }}</h4>
          <p>{{ backup?.description }}</p>
          <div class="backup-details">
            <span>Format: {{ backup?.format.toUpperCase() }}</span> •
            <span>Devices: {{ backup?.device_count }}</span> •
            <span>Size: {{ formatFileSize(backup?.file_size || 0) }}</span>
          </div>
        </div>

        <form @submit.prevent="emit('execute')" class="restore-form">
          <div class="form-section">
            <h4>Restore Options</h4>

            <label class="checkbox-label">
              <input v-model="localOptions.include_settings" type="checkbox" />
              <span>Restore Device Settings</span>
            </label>

            <label class="checkbox-label">
              <input v-model="localOptions.include_schedules" type="checkbox" />
              <span>Restore Schedules</span>
            </label>

            <label class="checkbox-label">
              <input v-model="localOptions.include_metrics" type="checkbox" />
              <span>Restore Historical Metrics</span>
            </label>

            <label class="checkbox-label">
              <input v-model="localOptions.dry_run" type="checkbox" />
              <span>Dry Run (Preview only)</span>
            </label>
          </div>

          <div v-if="preview" class="restore-preview">
            <h4>Restore Preview</h4>
            <div class="preview-stats">
              <div>Devices: {{ preview.device_count }}</div>
              <div>Settings: {{ preview.settings_count }}</div>
              <div>Schedules: {{ preview.schedules_count }}</div>
              <div>Metrics: {{ preview.metrics_count }}</div>
            </div>

            <div v-if="preview.warnings.length" class="warnings">
              <h5>⚠️ Warnings</h5>
              <ul>
                <li v-for="warning in preview.warnings" :key="warning">
                  {{ warning }}
                </li>
              </ul>
            </div>

            <div v-if="preview.conflicts.length" class="conflicts">
              <h5>❌ Conflicts</h5>
              <ul>
                <li v-for="conflict in preview.conflicts" :key="conflict">
                  {{ conflict }}
                </li>
              </ul>
            </div>
          </div>

          <div v-if="error" class="form-error">
            <strong>Error:</strong> {{ error }}
          </div>

          <div class="modal-actions">
            <button type="button" @click="emit('preview-restore')" class="secondary-button" :disabled="loading">
              Preview Changes
            </button>
            <button type="button" @click="emit('close')" class="secondary-button">
              Cancel
            </button>
            <button
              type="submit"
              class="primary-button"
              :disabled="loading || (preview?.conflicts.length > 0)"
            >
              {{ loading ? 'Restoring...' : (localOptions.dry_run ? 'Run Preview' : 'Execute Restore') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { BackupItem, RestoreRequest, RestorePreview } from '@/api/export'

interface Props {
  show: boolean
  backup: BackupItem | null
  options: RestoreRequest
  preview: RestorePreview | null
  loading: boolean
  error: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
  'preview-restore': []
  execute: []
  'update:options': [options: RestoreRequest]
}>()

const localOptions = reactive<RestoreRequest>({ ...props.options })

// Watch for changes in localOptions and emit them
watch(localOptions, (newOptions) => {
  emit('update:options', { ...newOptions })
}, { deep: true })

// Watch for external options changes
watch(() => props.options, (newOptions) => {
  Object.assign(localOptions, newOptions)
}, { deep: true })

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
  max-width: 800px;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
}

.restore-modal {
  max-width: 600px;
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

.restore-content {
  padding: 24px;
}

.backup-info {
  margin-bottom: 24px;
  padding: 16px;
  background: #f3f4f6;
  border-radius: 6px;
}

.backup-info h4 {
  margin: 0 0 8px 0;
  color: #1f2937;
}

.backup-info p {
  margin: 0 0 12px 0;
  color: #4b5563;
}

.backup-details {
  font-size: 0.875rem;
  color: #6b7280;
}

.form-section {
  margin-bottom: 24px;
}

.form-section h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
  font-size: 1rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  cursor: pointer;
}

.restore-preview {
  margin-bottom: 24px;
  padding: 16px;
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
}

.restore-preview h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
}

.preview-stats {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  margin-bottom: 16px;
  font-size: 0.875rem;
}

.warnings, .conflicts {
  margin-bottom: 12px;
}

.warnings h5, .conflicts h5 {
  margin: 0 0 8px 0;
  color: #1f2937;
}

.warnings ul, .conflicts ul {
  margin: 0;
  padding-left: 20px;
  font-size: 0.875rem;
}

.warnings {
  color: #d97706;
}

.conflicts {
  color: #dc2626;
}

.form-error {
  margin-bottom: 16px;
  padding: 12px;
  background: #fee2e2;
  border: 1px solid #fecaca;
  border-radius: 4px;
  color: #991b1b;
  font-size: 0.875rem;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.primary-button, .secondary-button {
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.primary-button {
  background-color: #3b82f6;
  color: white;
  border: none;
}

.primary-button:hover:not(:disabled) {
  background-color: #2563eb;
}

.primary-button:disabled {
  background-color: #9ca3af;
  cursor: not-allowed;
}

.secondary-button {
  background: white;
  border: 1px solid #d1d5db;
  color: #374151;
}

.secondary-button:hover:not(:disabled) {
  background: #f3f4f6;
}

.secondary-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
