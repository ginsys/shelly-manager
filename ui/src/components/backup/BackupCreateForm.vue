<template>
  <section id="create-backup" class="create-section">
    <h2>Create Backup or Content Export</h2>
    <div class="grid-2">
      <div class="form-field">
        <label class="field-label">Create Type</label>
        <select v-model="localCreateType" class="form-select">
          <option value="backup">Backup (Provider Snapshot)</option>
          <option value="json">Content Export: JSON</option>
          <option value="yaml">Content Export: YAML</option>
          <option value="sma">Content Export: SMA</option>
        </select>
      </div>
      <div class="form-field">
        <label class="field-label">Run Mode</label>
        <select v-model="localRunMode" class="form-select">
          <option value="now">Run Now</option>
          <option value="schedule">Schedule</option>
        </select>
      </div>
    </div>
    <div class="grid-2">
      <div class="form-field">
        <label class="field-label">Name</label>
        <input v-model="localCreateName" class="form-input" placeholder="e.g. Pre-maintenance snapshot" />
      </div>
      <div class="form-field">
        <label class="field-label">Description</label>
        <input v-model="localCreateDesc" class="form-input" placeholder="Optional description" />
      </div>
    </div>

    <!-- Backup options -->
    <div class="grid-2" v-if="localCreateType === 'backup'">
      <div class="form-field">
        <label class="field-label">Compression</label>
        <select v-model="localCreateCompression" class="form-select">
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
        <input v-model="localCreateOutputDir" class="form-input" placeholder="./data/backups" />
      </div>
    </div>

    <!-- Schedule options -->
    <div class="grid-2" v-if="localRunMode === 'schedule'">
      <div class="form-field">
        <label class="field-label">Schedule Interval</label>
        <select v-model="localSchedulePreset" class="form-select" @change="applyIntervalPreset">
          <option value="">Custom…</option>
          <option value="15 minutes">Every 15 minutes</option>
          <option value="1 hour">Every hour</option>
          <option value="6 hours">Every 6 hours</option>
          <option value="24 hours">Daily</option>
        </select>
        <input v-model="localScheduleInterval" class="form-input" placeholder="e.g. 1 hour, 24 hours" style="margin-top:8px" />
        <div class="field-help">Use format like "15 minutes", "1 hour", or "1 day".</div>
      </div>
      <div class="form-field">
        <label class="field-label">Enabled</label>
        <select v-model="localScheduleEnabled" class="form-select">
          <option :value="true">Enabled</option>
          <option :value="false">Disabled</option>
        </select>
      </div>
    </div>

    <!-- Content export options -->
    <div class="grid-2" v-if="localCreateType === 'json'">
      <div class="form-field">
        <label class="field-label">JSON Options</label>
        <label class="checkbox-label">
          <input type="checkbox" v-model="localJsonOptions.pretty" />
          <span>Pretty-print JSON</span>
        </label>
        <label class="checkbox-label">
          <input type="checkbox" v-model="localJsonOptions.include_discovered" />
          <span>Include discovered devices</span>
        </label>
        <label class="field-label" style="margin-top:8px">Compression</label>
        <select v-model="localJsonCompression" class="form-select">
          <option value="none">None</option>
          <option value="gzip">Gzip</option>
          <option value="zip">Zip</option>
        </select>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input v-model="localExportOutputDir" class="form-input" placeholder="./data/exports" />
      </div>
    </div>

    <div class="grid-2" v-if="localCreateType === 'yaml'">
      <div class="form-field">
        <label class="field-label">YAML Options</label>
        <label class="checkbox-label">
          <input type="checkbox" v-model="localYamlOptions.include_discovered" />
          <span>Include discovered devices</span>
        </label>
        <label class="field-label" style="margin-top:8px">Compression</label>
        <select v-model="localYamlCompression" class="form-select">
          <option value="none">None</option>
          <option value="gzip">Gzip</option>
          <option value="zip">Zip</option>
        </select>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input v-model="localExportOutputDir" class="form-input" placeholder="./data/exports" />
      </div>
    </div>

    <div class="grid-2" v-if="localCreateType === 'sma'">
      <div class="form-field">
        <label class="field-label">SMA Options</label>
        <div class="grid-2">
          <div>
            <label class="field-label">Compression level (1-9)</label>
            <input class="form-input" type="number" min="1" max="9" v-model.number="localSmaOptions.compression_level" />
          </div>
        </div>
        <div class="grid-2">
          <label class="checkbox-label">
            <input type="checkbox" v-model="localSmaOptions.include_discovered" />
            <span>Include discovered devices</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="localSmaOptions.include_network_settings" />
            <span>Include network settings</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="localSmaOptions.include_plugin_configs" />
            <span>Include plugin configurations</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="localSmaOptions.include_system_settings" />
            <span>Include system settings</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="localSmaOptions.exclude_sensitive" />
            <span>Exclude sensitive data</span>
          </label>
        </div>
        <div class="field-help">SMA exports are compressed archives with integrity data suitable for full content migration.</div>
      </div>
      <div class="form-field">
        <label class="field-label">Output Directory</label>
        <input v-model="localExportOutputDir" class="form-input" placeholder="./data/exports" />
      </div>
    </div>

    <div class="form-actions">
      <button class="primary-button" :disabled="submitting" @click="emit('submit')">
        {{ submitting ? 'Creating...' : 'Create' }}
      </button>
      <span v-if="error" class="form-error" style="margin-left:12px"><strong>Error:</strong> {{ error }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'

interface JsonOptions {
  pretty: boolean
  include_discovered: boolean
}

interface YamlOptions {
  include_discovered: boolean
}

interface SmaOptions {
  compression_level: number
  include_discovered: boolean
  include_network_settings: boolean
  include_plugin_configs: boolean
  include_system_settings: boolean
  exclude_sensitive: boolean
}

interface Props {
  createType: string
  runMode: string
  createName: string
  createDesc: string
  createCompression: string
  createOutputDir: string
  exportOutputDir: string
  scheduleEnabled: boolean
  scheduleInterval: string
  schedulePreset: string
  jsonOptions: JsonOptions
  yamlOptions: YamlOptions
  jsonCompression: string
  yamlCompression: string
  smaOptions: SmaOptions
  submitting: boolean
  error: string
  providerName?: string
  providerVersion?: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:createType': [value: string]
  'update:runMode': [value: string]
  'update:createName': [value: string]
  'update:createDesc': [value: string]
  'update:createCompression': [value: string]
  'update:createOutputDir': [value: string]
  'update:exportOutputDir': [value: string]
  'update:scheduleEnabled': [value: boolean]
  'update:scheduleInterval': [value: string]
  'update:schedulePreset': [value: string]
  'update:jsonOptions': [value: JsonOptions]
  'update:yamlOptions': [value: YamlOptions]
  'update:jsonCompression': [value: string]
  'update:yamlCompression': [value: string]
  'update:smaOptions': [value: SmaOptions]
  submit: []
}>()

// Local state with watchers for two-way binding
const localCreateType = ref(props.createType)
const localRunMode = ref(props.runMode)
const localCreateName = ref(props.createName)
const localCreateDesc = ref(props.createDesc)
const localCreateCompression = ref(props.createCompression)
const localCreateOutputDir = ref(props.createOutputDir)
const localExportOutputDir = ref(props.exportOutputDir)
const localScheduleEnabled = ref(props.scheduleEnabled)
const localScheduleInterval = ref(props.scheduleInterval)
const localSchedulePreset = ref(props.schedulePreset)
const localJsonOptions = reactive({ ...props.jsonOptions })
const localYamlOptions = reactive({ ...props.yamlOptions })
const localJsonCompression = ref(props.jsonCompression)
const localYamlCompression = ref(props.yamlCompression)
const localSmaOptions = reactive({ ...props.smaOptions })

// Watch local changes and emit
watch(localCreateType, (val) => emit('update:createType', val))
watch(localRunMode, (val) => emit('update:runMode', val))
watch(localCreateName, (val) => emit('update:createName', val))
watch(localCreateDesc, (val) => emit('update:createDesc', val))
watch(localCreateCompression, (val) => emit('update:createCompression', val))
watch(localCreateOutputDir, (val) => emit('update:createOutputDir', val))
watch(localExportOutputDir, (val) => emit('update:exportOutputDir', val))
watch(localScheduleEnabled, (val) => emit('update:scheduleEnabled', val))
watch(localScheduleInterval, (val) => emit('update:scheduleInterval', val))
watch(localSchedulePreset, (val) => emit('update:schedulePreset', val))
watch(localJsonOptions, (val) => emit('update:jsonOptions', { ...val }), { deep: true })
watch(localYamlOptions, (val) => emit('update:yamlOptions', { ...val }), { deep: true })
watch(localJsonCompression, (val) => emit('update:jsonCompression', val))
watch(localYamlCompression, (val) => emit('update:yamlCompression', val))
watch(localSmaOptions, (val) => emit('update:smaOptions', { ...val }), { deep: true })

function applyIntervalPreset() {
  if (localSchedulePreset.value) {
    localScheduleInterval.value = localSchedulePreset.value
  }
}
</script>

<style scoped>
.create-section {
  margin-bottom: 24px;
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.grid-2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.field-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #374151;
}

.field-help {
  font-size: 0.75rem;
  color: #6b7280;
  margin-top: 4px;
}

.form-select, .form-input {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: white;
  font-size: 0.875rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  cursor: pointer;
}

.form-actions {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.primary-button {
  background-color: #3b82f6;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.primary-button:hover:not(:disabled) {
  background-color: #2563eb;
}

.primary-button:disabled {
  background-color: #9ca3af;
  cursor: not-allowed;
}

.form-error {
  color: #dc2626;
  font-size: 0.875rem;
}
</style>
