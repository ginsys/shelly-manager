<template>
  <div class="modal-overlay" @click="$emit('close')">
    <div class="modal-content restore-modal" @click.stop>
      <div class="modal-header">
        <h3>Restore from Backup</h3>
        <button class="close-button" @click="$emit('close')">✖</button>
      </div>

      <div class="restore-content">
        <div class="backup-info">
          <h4>{{ backup?.name }}</h4>
          <p>{{ backup?.description }}</p>
          <div class="backup-details">
            <span>Format: {{ (backup?.format || '').toUpperCase() }}</span> •
            <span>Devices: {{ backup?.device_count }}</span> •
            <span>Size: {{ formatFileSize(backup?.file_size || 0) }}</span>
          </div>
        </div>

        <form @submit.prevent="$emit('execute')" class="restore-form">
          <div class="form-section">
            <h4>Restore Options</h4>

            <label class="checkbox-label">
              <input :checked="model.include_settings" @change="onToggle('include_settings', $event)" type="checkbox" />
              <span>Restore Device Settings</span>
            </label>

            <label class="checkbox-label">
              <input :checked="model.include_schedules" @change="onToggle('include_schedules', $event)" type="checkbox" />
              <span>Restore Schedules</span>
            </label>

            <label class="checkbox-label">
              <input :checked="model.include_metrics" @change="onToggle('include_metrics', $event)" type="checkbox" />
              <span>Restore Historical Metrics</span>
            </label>

            <label class="checkbox-label">
              <input :checked="model.dry_run" @change="onToggle('dry_run', $event)" type="checkbox" />
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

            <div v-if="preview.warnings?.length" class="warnings">
              <h5>⚠️ Warnings</h5>
              <ul>
                <li v-for="warning in preview.warnings" :key="warning">{{ warning }}</li>
              </ul>
            </div>

            <div v-if="preview.conflicts?.length" class="conflicts">
              <h5>❌ Conflicts</h5>
              <ul>
                <li v-for="conflict in preview.conflicts" :key="conflict">{{ conflict }}</li>
              </ul>
            </div>
          </div>

          <div v-if="error" class="form-error">
            <strong>Error:</strong> {{ error }}
          </div>

          <div class="modal-actions">
            <button type="button" @click="$emit('preview')" class="secondary-button" :disabled="loading">Preview Changes</button>
            <button type="button" @click="$emit('close')" class="secondary-button">Cancel</button>
            <button type="submit" class="primary-button" :disabled="loading || (preview?.conflicts?.length > 0)">
              {{ loading ? 'Restoring...' : (model.dry_run ? 'Run Preview' : 'Execute Restore') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ 
  backup: any | null
  options: { include_settings: boolean; include_schedules: boolean; include_metrics: boolean; dry_run: boolean }
  preview: any | null
  loading: boolean
  error: string | null
}>()
const emit = defineEmits<{ 'update:options': [any]; close: []; preview: []; execute: [] }>()

const model = computed({
  get: () => props.options,
  set: (v) => emit('update:options', v)
})

function onToggle(key: keyof typeof props.options, e: Event) {
  const next = { ...model.value, [key]: (e.target as HTMLInputElement).checked }
  emit('update:options', next)
}

function formatFileSize(bytes: number): string {
  if (!bytes) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let i = -1
  do { bytes = bytes / 1024; i++ } while (bytes >= 1024 && i < units.length - 1)
  return `${bytes.toFixed(1)} ${units[i]}`
}
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, .45); display: flex; align-items: center; justify-content: center; z-index: 1000 }
.modal-content { background: #fff; border-radius: 8px; box-shadow: 0 25px 50px -12px rgba(0,0,0,.25); padding: 16px; max-width: 900px; width: 90% }
.modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px }
.close-button { border: 1px solid #e5e7eb; background: #fff; border-radius: 6px; padding: 4px 8px; cursor: pointer }
.restore-content { display: grid; gap: 12px }
.backup-info h4 { margin: 0 }
.backup-details { color: #64748b; font-size: .875rem }
.restore-form { display: grid; gap: 12px }
.form-section { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px }
.checkbox-label { display: flex; align-items: center; gap: 8px; margin: 4px 0 }
.restore-preview { border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px }
.preview-stats { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap: 6px }
.warnings, .conflicts { background: #fff7ed; border: 1px solid #fde68a; padding: 8px; border-radius: 8px; margin-top: 8px }
.form-error { color: #b91c1c }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px }
.primary-button { background: #0ea5e9; color: #fff; border: 1px solid #0ea5e9; border-radius: 6px; padding: 6px 12px }
.secondary-button { background: #f8fafc; color: #334155; border: 1px solid #e2e8f0; border-radius: 6px; padding: 6px 12px }
</style>

