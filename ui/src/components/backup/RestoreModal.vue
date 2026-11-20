<template>
  <div v-if="show" class="modal-overlay" @click="emit('close')">
    <div class="modal-content restore-modal" @click.stop>
      <div class="modal-header">
        <h3>Restore Backup</h3>
        <button class="close-button" @click="emit('close')">✖</button>
      </div>

      <div class="restore-content">
        <div class="backup-info">
          <h4>{{ backup?.name }}</h4>
          <p>{{ backup?.description }}</p>
          <div class="backup-details">
            <span>Format: {{ backup?.format.toUpperCase() }}</span> •
            <span>Size: {{ formatFileSize(backup?.file_size || 0) }}</span>
          </div>
        </div>

        <div v-if="staged" class="restore-notice" data-testid="restore-staged">
          The restore is applied when the server next starts. Restart the server to apply it.
        </div>
        <div v-else class="restore-notice" data-testid="restore-two-steps">
          <strong>A restore is applied when the server next starts.</strong>
          Staging checks the backup and keeps a copy; the running database is not touched until you
          restart the server.
        </div>

        <div v-if="!restoreSupported" class="form-error" data-testid="restore-sqlite-only">
          Restore is only available for SQLite. The check below still works.
        </div>

        <div v-if="preview" class="restore-preview">
          <h4>{{ preview.valid ? 'The backup file can be opened' : 'The backup file is not usable' }}</h4>
          <div class="preview-stats">
            <div>Size on disk: {{ formatFileSize(preview.file_size) }}</div>
            <div>Records when created: {{ preview.record_count }}</div>
          </div>

          <div v-if="preview.warnings?.length" class="warnings">
            <h5>Notes</h5>
            <ul>
              <li v-for="warning in preview.warnings" :key="warning">
                {{ warning }}
              </li>
            </ul>
          </div>

          <div v-if="preview.errors?.length" class="conflicts">
            <h5>Problems</h5>
            <ul>
              <li v-for="problem in preview.errors" :key="problem">
                {{ problem }}
              </li>
            </ul>
          </div>
        </div>

        <div v-if="error" class="form-error">
          <strong>Error:</strong> {{ error }}
        </div>

        <p v-if="canStage && replacesPending" class="replaces-note">
          This replaces the restore already staged.
        </p>

        <div class="modal-actions">
          <button type="button" @click="emit('close')" class="secondary-button">
            Close
          </button>
          <template v-if="!staged">
            <button
              type="button"
              class="primary-button"
              :disabled="loading || staging"
              @click="emit('preview-restore')"
            >
              {{ loading ? 'Checking...' : 'Check backup' }}
            </button>
            <button
              v-if="canStage"
              type="button"
              class="primary-button stage-button"
              :disabled="staging"
              @click="emit('stage-restore')"
            >
              {{ staging ? 'Staging...' : 'Stage restore' }}
            </button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { BackupItem, RestorePreview } from '@/api/export'

interface Props {
  show: boolean
  backup: BackupItem | null
  preview: RestorePreview | null
  loading: boolean
  error: string
  // Staging is offered after a usable check, on a database that supports it. The check itself
  // works on every database.
  restoreSupported?: boolean
  staging?: boolean
  staged?: boolean
  replacesPending?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  restoreSupported: true,
  staging: false,
  staged: false,
  replacesPending: false,
})

const canStage = computed(() => props.restoreSupported && props.preview?.valid === true)

const emit = defineEmits<{
  close: []
  'preview-restore': []
  'stage-restore': []
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

.restore-notice {
  margin-bottom: 24px;
  padding: 12px 16px;
  background: #fffbeb;
  border: 1px solid #fde68a;
  border-radius: 6px;
  color: #92400e;
  font-size: 0.875rem;
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

.replaces-note {
  margin: 0 0 16px;
  color: #92400e;
  font-size: 0.875rem;
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
