<template>
  <section id="create-backup" class="create-section">
    <h2>Create Backup or Content Export</h2>
    <div class="grid-2">
      <div class="form-field">
        <label class="field-label">Create Type</label>
        <select :value="createType" @change="on('createType', $event)" class="form-select">
          <option value="backup">Backup (Provider Snapshot)</option>
          <option value="json">Content Export: JSON</option>
          <option value="yaml">Content Export: YAML</option>
          <option value="sma">Content Export: SMA</option>
        </select>
      </div>
      <div class="form-field">
        <label class="field-label">Run Mode</label>
        <select :value="runMode" @change="on('runMode', $event)" class="form-select">
          <option value="now">Run Now</option>
          <option value="schedule">Schedule</option>
        </select>
      </div>
    </div>
    <div class="grid-2">
      <div class="form-field">
        <label class="field-label">Name</label>
        <input :value="createName" @input="on('createName', $event)" class="form-input" placeholder="e.g. Pre-maintenance snapshot" />
      </div>
      <div class="form-field">
        <label class="field-label">Description</label>
        <input :value="createDesc" @input="on('createDesc', $event)" class="form-input" placeholder="Optional description" />
      </div>
    </div>

    <!-- Backup options -->
    <div class="grid-2" v-if="createType === 'backup'">
      <div class="form-field">
        <label class="field-label">Compression</label>
        <select :value="createCompression" @change="on('createCompression', $event)" class="form-select">
          <option value="none">None</option>
          <option value="gzip">Gzip</option>
          <option value="zip">Zip</option>
        </select>
        <div class="field-help" v-if="providerName">
          Database: {{ providerName }} {{ providerVersion }}
        </div>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input :value="createOutputDir" @input="on('createOutputDir', $event)" class="form-input" placeholder="./data/backups" />
      </div>
    </div>

    <!-- Schedule options -->
    <div class="grid-2" v-if="runMode === 'schedule'">
      <div class="form-field">
        <label class="field-label">Schedule Interval</label>
        <select :value="schedulePreset" class="form-select" @change="$emit('update:schedulePreset', ($event.target as HTMLSelectElement).value); $emit('apply-preset')">
          <option value="">Custom…</option>
          <option value="15 minutes">Every 15 minutes</option>
          <option value="1 hour">Every hour</option>
          <option value="6 hours">Every 6 hours</option>
          <option value="24 hours">Daily</option>
        </select>
        <input :value="scheduleInterval" @input="on('scheduleInterval', $event)" class="form-input" placeholder="e.g. 1 hour, 24 hours" style="margin-top:8px" />
        <div class="field-help">Use format like "15 minutes", "1 hour", or "1 day".</div>
      </div>
      <div class="form-field">
        <label class="field-label">Enabled</label>
        <select :value="scheduleEnabled ? 'true' : 'false'" class="form-select" @change="$emit('update:scheduleEnabled', (($event.target as HTMLSelectElement).value) === 'true')">
          <option value="true">Enabled</option>
          <option value="false">Disabled</option>
        </select>
      </div>
    </div>

    <!-- Content export options -->
    <div class="grid-2" v-if="createType === 'json'">
      <div class="form-field">
        <label class="field-label">JSON Options</label>
        <label class="checkbox-label">
          <input type="checkbox" :checked="jsonPretty" @change="$emit('update:jsonPretty', ($event.target as HTMLInputElement).checked)" />
          <span>Pretty-print JSON</span>
        </label>
        <label class="checkbox-label">
          <input type="checkbox" :checked="jsonIncludeDiscovered" @change="$emit('update:jsonIncludeDiscovered', ($event.target as HTMLInputElement).checked)" />
          <span>Include discovered devices</span>
        </label>
        <label class="field-label" style="margin-top:8px">Compression</label>
        <select :value="jsonCompression" class="form-select" @change="$emit('update:jsonCompression', ($event.target as HTMLSelectElement).value)">
          <option value="none">None</option>
          <option value="gzip">Gzip</option>
          <option value="zip">Zip</option>
        </select>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input :value="exportOutputDir" @input="on('exportOutputDir', $event)" class="form-input" placeholder="./data/exports" />
      </div>
    </div>

    <div class="grid-2" v-if="createType === 'yaml'">
      <div class="form-field">
        <label class="field-label">YAML Options</label>
        <label class="checkbox-label">
          <input type="checkbox" :checked="yamlIncludeDiscovered" @change="$emit('update:yamlIncludeDiscovered', ($event.target as HTMLInputElement).checked)" />
          <span>Include discovered devices</span>
        </label>
        <label class="field-label" style="margin-top:8px">Compression</label>
        <select :value="yamlCompression" class="form-select" @change="$emit('update:yamlCompression', ($event.target as HTMLSelectElement).value)">
          <option value="none">None</option>
          <option value="gzip">Gzip</option>
          <option value="zip">Zip</option>
        </select>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input :value="exportOutputDir" @input="on('exportOutputDir', $event)" class="form-input" placeholder="./data/exports" />
      </div>
    </div>

    <div class="grid-2" v-if="createType === 'sma'">
      <div class="form-field">
        <label class="field-label">SMA Options</label>
        <div class="grid-2">
          <div>
            <label class="field-label">Compression level (1-9)</label>
            <input class="form-input" type="number" min="1" max="9" :value="smaCompressionLevel" @input="$emit('update:smaCompressionLevel', Number(($event.target as HTMLInputElement).value))" />
          </div>
        </div>
        <div class="grid-2">
          <label class="checkbox-label">
            <input type="checkbox" :checked="smaIncludeDiscovered" @change="$emit('update:smaIncludeDiscovered', ($event.target as HTMLInputElement).checked)" />
            <span>Include discovered devices</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" :checked="smaIncludeNetworkSettings" @change="$emit('update:smaIncludeNetworkSettings', ($event.target as HTMLInputElement).checked)" />
            <span>Include network settings</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" :checked="smaIncludePluginConfigs" @change="$emit('update:smaIncludePluginConfigs', ($event.target as HTMLInputElement).checked)" />
            <span>Include plugin configurations</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" :checked="smaIncludeSystemSettings" @change="$emit('update:smaIncludeSystemSettings', ($event.target as HTMLInputElement).checked)" />
            <span>Include system settings</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" :checked="smaExcludeSensitive" @change="$emit('update:smaExcludeSensitive', ($event.target as HTMLInputElement).checked)" />
            <span>Exclude sensitive data</span>
          </label>
        </div>
        <div class="field-help">SMA exports are compressed archives with integrity data suitable for full content migration.</div>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input :value="exportOutputDir" @input="on('exportOutputDir', $event)" class="form-input" placeholder="./data/exports" />
      </div>
    </div>
    <div class="form-actions">
      <button class="primary-button" :disabled="submitting" @click="$emit('submit')">
        {{ submitting ? 'Creating...' : 'Create' }}
      </button>
      <span v-if="error" class="form-error" style="margin-left:12px"><strong>Error:</strong> {{ error }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
const props = defineProps<{ 
  runMode: 'now' | 'schedule'
  createType: 'backup' | 'json' | 'yaml' | 'sma'
  createName: string
  createDesc: string
  createCompression: 'none' | 'gzip' | 'zip'
  createOutputDir: string
  schedulePreset: string
  scheduleInterval: string
  scheduleEnabled: boolean
  jsonPretty: boolean
  jsonIncludeDiscovered: boolean
  jsonCompression: 'none' | 'gzip' | 'zip'
  yamlIncludeDiscovered: boolean
  yamlCompression: 'none' | 'gzip' | 'zip'
  smaCompressionLevel: number
  smaIncludeDiscovered: boolean
  smaIncludeNetworkSettings: boolean
  smaIncludePluginConfigs: boolean
  smaIncludeSystemSettings: boolean
  smaExcludeSensitive: boolean
  exportOutputDir: string
  providerName?: string
  providerVersion?: string
  submitting: boolean
  error: string
}>()

const emit = defineEmits<{
  'update:runMode': ['now' | 'schedule']
  'update:createType': ['backup' | 'json' | 'yaml' | 'sma']
  'update:createName': [string]
  'update:createDesc': [string]
  'update:createCompression': ['none' | 'gzip' | 'zip']
  'update:createOutputDir': [string]
  'update:schedulePreset': [string]
  'update:scheduleInterval': [string]
  'update:scheduleEnabled': [boolean]
  'update:jsonPretty': [boolean]
  'update:jsonIncludeDiscovered': [boolean]
  'update:jsonCompression': ['none' | 'gzip' | 'zip']
  'update:yamlIncludeDiscovered': [boolean]
  'update:yamlCompression': ['none' | 'gzip' | 'zip']
  'update:smaCompressionLevel': [number]
  'update:smaIncludeDiscovered': [boolean]
  'update:smaIncludeNetworkSettings': [boolean]
  'update:smaIncludePluginConfigs': [boolean]
  'update:smaIncludeSystemSettings': [boolean]
  'update:smaExcludeSensitive': [boolean]
  'update:exportOutputDir': [string]
  'apply-preset': []
  'submit': []
}>()

function on(key: string, e: Event) {
  const target = e.target as HTMLInputElement | HTMLSelectElement
  const value = target.type === 'checkbox' ? (target as HTMLInputElement).checked : target.value
  emit(`update:${key}` as any, value as any)
}
</script>

<style scoped>
.create-section { margin: 16px 0; display: grid; gap: 10px }
.grid-2 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px }
.form-field { display: grid; gap: 6px }
.field-label { font-size: .875rem; color: #4b5563 }
.form-select, .form-input { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px }
.checkbox-label { display: flex; align-items: center; gap: 8px; margin: 4px 0 }
.field-help { color: #64748b; font-size: .875rem; margin-top: 6px }
.primary-button { background: #0ea5e9; color: #fff; border: 1px solid #0ea5e9; border-radius: 6px; padding: 6px 12px }
.form-actions { display: flex; align-items: center }
.form-error { color: #b91c1c }
@media (max-width: 720px) { .grid-2 { grid-template-columns: 1fr } }
</style>

