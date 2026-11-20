<template>
  <form class="gitops-form" data-testid="gitops-form" @submit.prevent="emit('submit', options())">
    <h3>Create GitOps Export</h3>
    <p class="hint">
      Writes a directory of YAML files on the server: one file per device, grouped as chosen below.
      A later export to the same directory replaces the files of the earlier one.
    </p>

    <label class="field">
      <span>Name</span>
      <input v-model.trim="name" required maxlength="100" data-testid="gitops-name" />
    </label>
    <label class="field">
      <span>Description</span>
      <input v-model.trim="description" maxlength="500" />
    </label>
    <label class="field">
      <span>Group devices by</span>
      <select v-model="groupBy" data-testid="gitops-group-by">
        <option value="location">Location</option>
        <option value="type">Device type</option>
      </select>
    </label>
    <label class="check"><input v-model="includeCommon" type="checkbox" /> Write the common settings file</label>
    <label class="check"><input v-model="includeTemplates" type="checkbox" /> Include configuration templates</label>

    <label class="field">
      <span>Devices (none selected: all)</span>
      <select v-model="deviceIds" multiple size="6" data-testid="gitops-devices">
        <option v-for="d in availableDevices" :key="d.id" :value="d.id">{{ d.name || d.ip }}</option>
      </select>
    </label>

    <pre v-if="preview" class="preview" data-testid="gitops-preview">{{ preview }}</pre>
    <div v-if="error" class="form-error"><strong>Error:</strong> {{ error }}</div>

    <div class="actions">
      <button type="button" class="secondary" @click="emit('cancel')">Cancel</button>
      <button type="button" class="secondary" :disabled="loading || !name" @click="emit('preview', options())">
        Preview files
      </button>
      <button type="submit" class="primary" :disabled="loading || !name">
        {{ loading ? 'Creating...' : 'Create Export' }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { GitOpsExportOptions } from '@/api/export'
import type { Device } from '@/api/types'

defineProps<{
  availableDevices: Device[]
  loading: boolean
  error: string
  preview: string
}>()

const emit = defineEmits<{
  submit: [options: GitOpsExportOptions]
  preview: [options: GitOpsExportOptions]
  'preview-stale': []
  cancel: []
}>()

const name = ref('')
const description = ref('')
const groupBy = ref<GitOpsExportOptions['group_by']>('location')
const includeCommon = ref(true)
const includeTemplates = ref(true)
const deviceIds = ref<number[]>([])

// A preview describes the options it was asked for: once one of them that shapes the files changes,
// the page drops the shown tree and any answer still on its way.
watch([groupBy, includeCommon, includeTemplates, deviceIds], () => emit('preview-stale'), { deep: true })

function options(): GitOpsExportOptions {
  return {
    name: name.value,
    description: description.value || undefined,
    group_by: groupBy.value,
    include_common: includeCommon.value,
    include_templates: includeTemplates.value,
    device_ids: deviceIds.value,
  }
}
</script>

<style scoped>
.gitops-form { padding: 24px; display: flex; flex-direction: column; gap: 12px; }
.gitops-form h3 { margin: 0; color: #1f2937; }
.hint { margin: 0; color: #6b7280; font-size: 0.875rem; }
.field { display: flex; flex-direction: column; gap: 4px; font-size: 0.875rem; color: #374151; }
.field input, .field select { padding: 8px; border: 1px solid #d1d5db; border-radius: 4px; }
.check { font-size: 0.875rem; color: #374151; }
.preview { margin: 0; padding: 12px; background: #f3f4f6; border-radius: 6px; max-height: 240px; overflow: auto; font-size: 0.75rem; }
.form-error { padding: 12px; background: #fee2e2; border: 1px solid #fecaca; border-radius: 4px; color: #991b1b; font-size: 0.875rem; }
.actions { display: flex; gap: 8px; justify-content: flex-end; }
.actions button { padding: 8px 16px; border-radius: 6px; cursor: pointer; }
.actions .primary { background: #3b82f6; color: white; border: none; }
.actions .secondary { background: white; color: #374151; border: 1px solid #d1d5db; }
.actions button:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
