<template>
  <div class="schedule-form">
    <div class="form-header">
      <h2>{{ isEdit ? 'Edit Schedule' : 'Create Schedule' }}</h2>
      <button class="close-button" @click="$emit('cancel')" type="button">✖</button>
    </div>

    <form @submit.prevent="onSubmit" class="form-content">
      <!-- Basic Information -->
      <div class="form-section">
        <h3>Basic Information</h3>
        
        <div class="form-field">
          <label class="field-label">
            Schedule Name *
            <span class="field-help">A descriptive name for this schedule</span>
          </label>
          <input
            v-model="formData.name"
            type="text"
            required
            maxlength="100"
            placeholder="e.g. Daily Device Backup"
            class="form-input"
            :class="{ error: errors.name }"
          />
          <div v-if="errors.name" class="field-error">{{ errors.name }}</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            Run Interval *
            <span class="field-help">How often should this schedule run?</span>
          </label>
          <div class="interval-inputs">
            <input
              v-model.number="intervalValue"
              type="number"
              min="1"
              required
              class="form-input interval-value"
              :class="{ error: errors.interval }"
            />
            <select v-model="intervalUnit" class="form-select interval-unit">
              <option value="minutes">Minutes</option>
              <option value="hours">Hours</option>
              <option value="days">Days</option>
            </select>
          </div>
          <div class="interval-preview">
            Every {{ intervalValue }} {{ intervalUnit.slice(0, -1) }}{{ intervalValue !== 1 ? intervalUnit.slice(-1) : '' }}
            ({{ formatInterval(formData.interval_sec) }})
          </div>
          <div v-if="errors.interval" class="field-error">{{ errors.interval }}</div>
        </div>

        <div class="form-field">
          <label class="checkbox-label">
            <input
              v-model="formData.enabled"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Enable schedule immediately</span>
            <span class="field-help">If disabled, the schedule won't run automatically</span>
          </label>
        </div>
      </div>

      <!-- Export Configuration -->
      <div class="form-section">
        <h3>Export Configuration</h3>
        
        <div class="form-field">
          <label class="field-label">
            Export Plugin *
            <span class="field-help">The plugin to use for exporting data</span>
          </label>
          <select
            v-model="formData.request.plugin_name"
            @change="onPluginChange"
            required
            class="form-select"
            :class="{ error: errors.plugin }"
          >
            <option value="">Select a plugin...</option>
            <option v-for="plugin in availablePlugins" :key="plugin.id" :value="plugin.id">
              {{ plugin.name }} ({{ plugin.formats.join(', ') }})
            </option>
          </select>
          <div v-if="errors.plugin" class="field-error">{{ errors.plugin }}</div>
        </div>

        <div v-if="formData.request.plugin_name" class="form-field">
          <label class="field-label">
            Output Format *
            <span class="field-help">The format for exported data</span>
          </label>
          <select
            v-model="formData.request.format"
            required
            class="form-select"
            :class="{ error: errors.format }"
          >
            <option value="">Select format...</option>
            <option v-for="format in availableFormats" :key="format" :value="format">
              {{ format.toUpperCase() }}
            </option>
          </select>
          <div v-if="errors.format" class="field-error">{{ errors.format }}</div>
        </div>

        <!-- Plugin-specific Configuration -->
        <div v-if="pluginSchema?.config" class="form-field">
          <label class="field-label">
            Plugin Configuration
            <span class="field-help">Plugin-specific settings</span>
          </label>
          <div class="config-fields">
            <div v-for="(field, key) in pluginSchema.config" :key="key" class="config-field">
              <label class="config-label">
                {{ field.label || key }}
                <span v-if="field.required" class="required">*</span>
              </label>
              
              <!-- Text input -->
              <input
                v-if="field.type === 'string' || !field.type"
                v-model="formData.request.config[key]"
                :placeholder="field.placeholder"
                :required="field.required"
                class="form-input config-input"
              />
              
              <!-- Number input -->
              <input
                v-else-if="field.type === 'number'"
                v-model.number="formData.request.config[key]"
                type="number"
                :min="field.min"
                :max="field.max"
                :placeholder="field.placeholder"
                :required="field.required"
                class="form-input config-input"
              />
              
              <!-- Boolean checkbox -->
              <label v-else-if="field.type === 'boolean'" class="config-checkbox-label">
                <input
                  type="checkbox"
                  v-model="formData.request.config[key]"
                  class="form-checkbox"
                />
                <span>{{ field.label || key }}</span>
              </label>
              
              <!-- Select dropdown -->
              <select
                v-else-if="field.type === 'select'"
                v-model="formData.request.config[key]"
                :required="field.required"
                class="form-select config-select"
              >
                <option value="">Select...</option>
                <option
                  v-for="option in field.options"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label || option.value }}
                </option>
              </select>

              <div v-if="field.description" class="config-help">{{ field.description }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Schedule Preview -->
      <div v-if="formData.name && formData.interval_sec > 0" class="form-section preview-section">
        <h3>Schedule Preview</h3>
        <div class="preview-info">
          <div class="preview-item">
            <strong>Name:</strong> {{ formData.name }}
          </div>
          <div class="preview-item">
            <strong>Plugin:</strong> {{ formData.request.plugin_name }} ({{ formData.request.format.toUpperCase() }})
          </div>
          <div class="preview-item">
            <strong>Interval:</strong> {{ formatInterval(formData.interval_sec) }}
          </div>
          <div class="preview-item">
            <strong>Status:</strong> 
            <span :class="['status-preview', formData.enabled ? 'enabled' : 'disabled']">
              {{ formData.enabled ? 'Enabled' : 'Disabled' }}
            </span>
          </div>
          <div class="preview-item">
            <strong>Next Run:</strong> {{ formatNextRun() }}
          </div>
        </div>
      </div>

      <!-- Error Display -->
      <div v-if="error" class="form-error">
        <strong>Error:</strong> {{ error }}
      </div>

      <!-- Form Actions -->
      <div class="form-actions">
        <button type="button" @click="$emit('cancel')" class="secondary-button">
          Cancel
        </button>
        <button
          type="submit"
          :disabled="!isFormValid || loading"
          class="primary-button"
        >
          <span v-if="loading">{{ isEdit ? 'Updating...' : 'Creating...' }}</span>
          <span v-else>{{ isEdit ? 'Update Schedule' : 'Create Schedule' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { validateScheduleRequest, formatInterval, calculateNextRun } from '@/api/schedule'
import type { ExportSchedule, ExportScheduleRequest } from '@/api/schedule'

interface PluginField {
  type?: 'string' | 'number' | 'boolean' | 'select'
  label?: string
  description?: string
  placeholder?: string
  required?: boolean
  min?: number
  max?: number
  options?: { value: string; label?: string }[]
}

interface PluginSchema {
  config?: Record<string, PluginField>
}

interface Plugin {
  id: string
  name: string
  formats: string[]
  schema?: PluginSchema
}

const props = defineProps<{
  schedule?: ExportSchedule | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  submit: [ExportScheduleRequest]
  cancel: []
}>()

const isEdit = computed(() => !!props.schedule)

// Form data
const formData = reactive<ExportScheduleRequest>({
  name: '',
  interval_sec: 3600, // Default: 1 hour
  enabled: true,
  request: {
    plugin_name: '',
    format: '',
    config: {},
    filters: {},
    options: {}
  }
})

// Interval helpers
const intervalValue = ref(1)
const intervalUnit = ref('hours')

// Validation errors
const errors = reactive<Record<string, string>>({})

// Available plugins (mock data for now - in real app would fetch from API)
const availablePlugins = ref<Plugin[]>([
  {
    id: 'mockfile',
    name: 'Mock File Export',
    formats: ['txt', 'csv', 'json'],
    schema: {
      config: {
        include_headers: { 
          type: 'boolean', 
          label: 'Include Headers', 
          description: 'Add column headers to output' 
        },
        max_records: { 
          type: 'number', 
          label: 'Max Records', 
          placeholder: '1000', 
          min: 1, 
          max: 100000 
        },
        encoding: { 
          type: 'select', 
          label: 'Character Encoding',
          options: [
            { value: 'utf-8', label: 'UTF-8' },
            { value: 'ascii', label: 'ASCII' },
            { value: 'latin-1', label: 'Latin-1' }
          ]
        }
      }
    }
  },
  {
    id: 'gitops',
    name: 'GitOps Configuration',
    formats: ['yaml', 'json'],
    schema: {
      config: {
        namespace: { 
          type: 'string', 
          label: 'Kubernetes Namespace', 
          placeholder: 'shelly-manager', 
          required: true 
        },
        include_secrets: { 
          type: 'boolean', 
          label: 'Include Secrets', 
          description: 'Export sensitive configuration' 
        }
      }
    }
  }
])

// Computed properties
const availableFormats = computed(() => {
  const plugin = availablePlugins.value.find(p => p.id === formData.request.plugin_name)
  return plugin?.formats || []
})

const pluginSchema = computed(() => {
  const plugin = availablePlugins.value.find(p => p.id === formData.request.plugin_name)
  return plugin?.schema
})

const isFormValid = computed(() => {
  return formData.name && 
         formData.interval_sec > 0 && 
         formData.request.plugin_name && 
         formData.request.format &&
         Object.keys(errors).length === 0
})

// Methods
function updateIntervalSec() {
  let multiplier = 1
  switch (intervalUnit.value) {
    case 'minutes': multiplier = 60; break
    case 'hours': multiplier = 3600; break
    case 'days': multiplier = 86400; break
  }
  formData.interval_sec = intervalValue.value * multiplier
}

function setIntervalFromSec(seconds: number) {
  if (seconds % 86400 === 0) {
    intervalValue.value = seconds / 86400
    intervalUnit.value = 'days'
  } else if (seconds % 3600 === 0) {
    intervalValue.value = seconds / 3600
    intervalUnit.value = 'hours'
  } else {
    intervalValue.value = seconds / 60
    intervalUnit.value = 'minutes'
  }
}

function onPluginChange() {
  // Reset format when plugin changes
  formData.request.format = ''
  formData.request.config = {}
  formData.request.filters = {}
  formData.request.options = {}
}

function validateForm() {
  const validationErrors = validateScheduleRequest(formData)
  
  // Clear previous errors
  Object.keys(errors).forEach(key => delete errors[key])
  
  // Set new errors
  validationErrors.forEach(error => {
    if (error.includes('Name')) errors.name = error
    else if (error.includes('Interval')) errors.interval = error
    else if (error.includes('Plugin')) errors.plugin = error
    else if (error.includes('Format')) errors.format = error
  })
}

function formatNextRun(): string {
  if (!formData.enabled || formData.interval_sec <= 0) {
    return 'Not scheduled'
  }
  
  const nextRun = calculateNextRun(formData.interval_sec)
  return nextRun.toLocaleString()
}

function onSubmit() {
  validateForm()
  
  if (!isFormValid.value) {
    return
  }
  
  emit('submit', { ...formData })
}

// Initialize form when editing
onMounted(() => {
  if (props.schedule) {
    formData.name = props.schedule.name
    formData.interval_sec = props.schedule.interval_sec
    formData.enabled = props.schedule.enabled
    formData.request = { ...props.schedule.request }
    
    setIntervalFromSec(props.schedule.interval_sec)
  }
})

// Watch for interval changes
watch([intervalValue, intervalUnit], updateIntervalSec)

// Watch for form changes to validate
watch(formData, validateForm, { deep: true })
</script>

<style scoped>
.schedule-form {
  background: white;
  border-radius: 8px;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid #e5e7eb;
}

.form-header h2 {
  margin: 0;
  color: #1f2937;
  font-size: 1.5rem;
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

.form-content {
  padding: 24px;
  overflow-y: auto;
}

.form-section {
  margin-bottom: 32px;
}

.form-section h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
  font-size: 1.125rem;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}

.form-field {
  margin-bottom: 20px;
}

.field-label {
  display: block;
  font-weight: 500;
  color: #374151;
  margin-bottom: 6px;
  font-size: 0.875rem;
}

.field-help {
  display: block;
  font-weight: 400;
  color: #6b7280;
  font-size: 0.75rem;
  margin-top: 2px;
}

.form-input, .form-select {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
  background: white;
}

.form-input:focus, .form-select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input.error, .form-select.error {
  border-color: #dc2626;
}

.checkbox-label {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  cursor: pointer;
}

.form-checkbox {
  width: auto;
  margin: 0;
}

.checkbox-label span {
  font-weight: 500;
  color: #374151;
}

.interval-inputs {
  display: flex;
  gap: 8px;
  align-items: center;
}

.interval-value {
  width: 100px;
}

.interval-unit {
  width: 120px;
}

.interval-preview {
  margin-top: 8px;
  font-size: 0.875rem;
  color: #6b7280;
  font-style: italic;
}

.config-fields {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 16px;
  background: #f9fafb;
}

.config-field {
  margin-bottom: 16px;
}

.config-field:last-child {
  margin-bottom: 0;
}

.config-label {
  display: block;
  font-weight: 500;
  color: #374151;
  margin-bottom: 4px;
  font-size: 0.875rem;
}

.required {
  color: #dc2626;
  margin-left: 2px;
}

.config-input, .config-select {
  width: 100%;
  padding: 8px 10px;
  font-size: 0.875rem;
}

.config-checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 400;
}

.config-help {
  margin-top: 4px;
  font-size: 0.75rem;
  color: #6b7280;
}

.preview-section {
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
  padding: 16px;
}

.preview-info {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
}

.preview-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.875rem;
}

.status-preview.enabled {
  color: #10b981;
  font-weight: 500;
}

.status-preview.disabled {
  color: #f59e0b;
  font-weight: 500;
}

.field-error {
  margin-top: 4px;
  color: #dc2626;
  font-size: 0.75rem;
}

.form-error {
  margin-bottom: 20px;
  padding: 12px;
  background: #fee2e2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  color: #dc2626;
  font-size: 0.875rem;
}

.form-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding-top: 20px;
  border-top: 1px solid #e5e7eb;
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

.secondary-button {
  background-color: #f3f4f6;
  color: #374151;
  border: 1px solid #d1d5db;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s, border-color 0.2s;
}

.secondary-button:hover {
  background-color: #e5e7eb;
  border-color: #9ca3af;
}

/* Responsive design */
@media (max-width: 768px) {
  .form-header {
    padding: 16px;
  }

  .form-content {
    padding: 16px;
  }

  .interval-inputs {
    flex-direction: column;
    align-items: stretch;
  }

  .interval-value,
  .interval-unit {
    width: 100%;
  }

  .form-actions {
    flex-direction: column;
  }

  .preview-info {
    grid-template-columns: 1fr;
  }
}
</style>