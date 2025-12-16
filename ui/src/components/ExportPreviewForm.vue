<template>
  <div class="preview-form-container">
    <form class="preview-form" @submit.prevent="onPreview">
      <!-- Plugin Selection -->
      <div class="form-section">
        <label class="form-label">Export Plugin *</label>
        <select v-model="formData.plugin_name" @change="onPluginChange" required class="form-select">
          <option value="">Select a plugin...</option>
          <option v-for="plugin in availablePlugins" :key="plugin.id" :value="plugin.id">
            {{ plugin.name }} ({{ plugin.formats.join(', ') }})
          </option>
        </select>
        <div v-if="pluginError" class="error-message">{{ pluginError }}</div>
      </div>

      <!-- Format Selection -->
      <div class="form-section" v-if="formData.plugin_name">
        <label class="form-label">Output Format *</label>
        <select v-model="formData.format" @change="onFormatChange" required class="form-select">
          <option value="">Select format...</option>
          <option v-for="format in availableFormats" :key="format" :value="format">
            {{ format.toUpperCase() }}
          </option>
        </select>
        <div v-if="formatError" class="error-message">{{ formatError }}</div>
      </div>

      <!-- Dynamic Plugin Configuration -->
      <div class="form-section" v-if="pluginSchema?.config">
        <h3 class="section-title">Plugin Configuration</h3>
        <SchemaForm :schema="pluginSchema.config" v-model="formData.config" />
      </div>

      <!-- Filters Section -->
      <div class="form-section" v-if="pluginSchema?.filters">
        <h3 class="section-title">Export Filters</h3>
        <SchemaForm :schema="pluginSchema.filters" v-model="formData.filters" />
      </div>

      <!-- JSON Configuration Editor -->
      <div class="form-section" v-if="showAdvanced">
        <h3 class="section-title">
          Advanced Configuration (JSON)
          <button type="button" @click="showAdvanced = false" class="toggle-button">Hide</button>
        </h3>
        <div class="json-editor-container">
          <textarea 
            v-model="jsonConfig" 
            @input="validateJson"
            class="json-editor"
            placeholder="{ &#10;  &quot;config&quot;: {},&#10;  &quot;filters&quot;: {},&#10;  &quot;options&quot;: {}&#10;}"
            rows="8"
          ></textarea>
          <div v-if="jsonError" class="json-error">
            <strong>JSON Error:</strong> {{ jsonError }}
          </div>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="form-actions">
        <button type="button" 
                @click="showAdvanced = !showAdvanced"
                class="secondary-button">
          {{ showAdvanced ? 'Hide' : 'Show' }} JSON Editor
        </button>
        <button type="button" 
                @click="loadFromStorage"
                :disabled="!hasStoredConfig"
                class="secondary-button">
          Load Last Config
        </button>
        <button type="submit" 
                :disabled="!isFormValid || isLoading"
                class="primary-button">
          {{ isLoading ? 'Previewing...' : 'Preview Export' }}
        </button>
      </div>
    </form>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading-overlay">
      <div class="spinner"></div>
      <span>Generating preview...</span>
    </div>

    <!-- Preview Results -->
    <div v-if="previewResult" class="preview-results">
      <div class="preview-header">
        <h3>Preview Results</h3>
        <div class="preview-actions">
          <button @click="copyToClipboard" class="action-button">📋 Copy JSON</button>
          <button @click="downloadJson" class="action-button">💾 Download</button>
          <button @click="clearPreview" class="action-button">✖ Clear</button>
        </div>
      </div>

      <!-- Summary Information -->
      <div class="preview-summary">
        <div class="summary-item" :class="{ success: previewResult.preview?.success }">
          <strong>Status:</strong> 
          {{ previewResult.preview?.success ? 'Success' : 'Warning' }}
        </div>
        <div class="summary-item" v-if="previewResult.preview?.record_count">
          <strong>Records:</strong> {{ previewResult.preview.record_count.toLocaleString() }}
        </div>
        <div class="summary-item" v-if="previewResult.preview?.estimated_size">
          <strong>Size:</strong> {{ formatFileSize(previewResult.preview.estimated_size) }}
        </div>
      </div>

      <!-- Warnings -->
      <div v-if="previewResult.preview?.warnings?.length" class="warnings-section">
        <h4>⚠️ Warnings</h4>
        <ul class="warnings-list">
          <li v-for="warning in previewResult.preview.warnings" :key="warning" class="warning-item">
            {{ warning }}
          </li>
        </ul>
      </div>

      <!-- Raw JSON Preview -->
      <details class="json-details">
        <summary>Raw Response Data</summary>
        <pre class="json-preview">{{ JSON.stringify(previewResult, null, 2) }}</pre>
      </details>
    </div>

    <!-- Error Display -->
    <div v-if="errorMessage" class="error-display">
      <strong>Error:</strong> {{ errorMessage }}
      <button @click="errorMessage = ''" class="error-close">✖</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { previewExport, type ExportRequest } from '@/api/export'
import SchemaForm from '@/components/shared/SchemaForm.vue'

// Plugin schema type for form generation
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
  filters?: Record<string, PluginField>
  options?: Record<string, PluginField>
}

interface Plugin {
  id: string
  name: string
  formats: string[]
  schema?: PluginSchema
}

// Reactive state
const formData = reactive<ExportRequest>({
  plugin_name: '',
  format: '',
  config: {},
  filters: {},
  options: {}
})

const jsonConfig = ref('')
const showAdvanced = ref(false)
const isLoading = ref(false)
const previewResult = ref<any>(null)
const errorMessage = ref('')
const pluginError = ref('')
const formatError = ref('')
const jsonError = ref('')

// Available plugins with schemas
const availablePlugins = ref<Plugin[]>([
  {
    id: 'mockfile',
    name: 'Mock File Export',
    formats: ['txt', 'csv', 'json'],
    schema: {
      config: {
        include_headers: { type: 'boolean', label: 'Include Headers', description: 'Add column headers to output' },
        max_records: { type: 'number', label: 'Max Records', placeholder: '1000', min: 1, max: 100000 },
        encoding: { 
          type: 'select', 
          label: 'Character Encoding',
          options: [
            { value: 'utf-8', label: 'UTF-8' },
            { value: 'ascii', label: 'ASCII' },
            { value: 'latin-1', label: 'Latin-1' }
          ]
        }
      },
      filters: {
        date_from: { type: 'string', label: 'Date From', placeholder: '2023-01-01' },
        date_to: { type: 'string', label: 'Date To', placeholder: '2023-12-31' },
        device_type: { type: 'string', label: 'Device Type Filter', placeholder: 'shelly1' }
      }
    }
  },
  {
    id: 'gitops',
    name: 'GitOps Configuration',
    formats: ['yaml', 'json'],
    schema: {
      config: {
        namespace: { type: 'string', label: 'Kubernetes Namespace', placeholder: 'shelly-manager', required: true },
        include_secrets: { type: 'boolean', label: 'Include Secrets', description: 'Export sensitive configuration' },
        format_version: { 
          type: 'select', 
          label: 'Format Version',
          options: [
            { value: 'v1', label: 'Version 1.0' },
            { value: 'v2', label: 'Version 2.0 (Beta)' }
          ]
        }
      }
    }
  }
])

// Computed properties
const availableFormats = computed(() => {
  const plugin = availablePlugins.value.find(p => p.id === formData.plugin_name)
  return plugin?.formats || []
})

const pluginSchema = computed(() => {
  const plugin = availablePlugins.value.find(p => p.id === formData.plugin_name)
  return plugin?.schema
})

const isFormValid = computed(() => {
  return formData.plugin_name && 
         formData.format && 
         !pluginError.value && 
         !formatError.value && 
         !jsonError.value
})

const hasStoredConfig = computed(() => {
  return localStorage.getItem(`export-config-${formData.plugin_name}`) !== null
})

// Storage key for persistence
const storageKey = computed(() => `export-config-${formData.plugin_name}-${formData.format}`)

// Methods
function onPluginChange() {
  formData.format = ''
  formData.config = {}
  formData.filters = {}
  formData.options = {}
  pluginError.value = ''
  formatError.value = ''
  previewResult.value = null
  errorMessage.value = ''
}

function onFormatChange() {
  previewResult.value = null
  errorMessage.value = ''
}

function validateJson() {
  if (!jsonConfig.value.trim()) {
    jsonError.value = ''
    return
  }
  
  try {
    const parsed = JSON.parse(jsonConfig.value)
    
    // Merge JSON config into form data
    if (parsed.config) Object.assign(formData.config, parsed.config)
    if (parsed.filters) Object.assign(formData.filters, parsed.filters)
    if (parsed.options) Object.assign(formData.options, parsed.options)
    
    jsonError.value = ''
  } catch (error) {
    jsonError.value = error instanceof Error ? error.message : 'Invalid JSON'
  }
}

async function onPreview() {
  if (!isFormValid.value) return
  
  isLoading.value = true
  errorMessage.value = ''
  previewResult.value = null
  
  try {
    const result = await previewExport(formData)
    previewResult.value = result
    
    // Save successful config to localStorage
    saveToStorage()
    
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Preview failed'
  } finally {
    isLoading.value = false
  }
}

function saveToStorage() {
  try {
    localStorage.setItem(storageKey.value, JSON.stringify({
      config: formData.config,
      filters: formData.filters,
      options: formData.options,
      timestamp: Date.now()
    }))
  } catch (error) {
    console.warn('Failed to save config to localStorage:', error)
  }
}

function loadFromStorage() {
  try {
    const stored = localStorage.getItem(storageKey.value)
    if (stored) {
      const parsed = JSON.parse(stored)
      Object.assign(formData.config, parsed.config || {})
      Object.assign(formData.filters, parsed.filters || {})
      Object.assign(formData.options, parsed.options || {})
    }
  } catch (error) {
    console.warn('Failed to load config from localStorage:', error)
  }
}

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(JSON.stringify(previewResult.value, null, 2))
    // Could show a toast notification here
  } catch (error) {
    console.error('Failed to copy to clipboard:', error)
  }
}

function downloadJson() {
  const blob = new Blob([JSON.stringify(previewResult.value, null, 2)], { 
    type: 'application/json' 
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `export-preview-${formData.plugin_name}-${Date.now()}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function clearPreview() {
  previewResult.value = null
  errorMessage.value = ''
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// Watch for form changes to sync with JSON editor
watch(formData, () => {
  if (!showAdvanced.value) return
  
  const jsonData = {
    config: formData.config,
    filters: formData.filters,
    options: formData.options
  }
  
  // Only update if different to avoid cursor jumping
  const currentJson = JSON.stringify(jsonData, null, 2)
  if (jsonConfig.value !== currentJson) {
    jsonConfig.value = currentJson
  }
}, { deep: true })

// Load saved config on mount if available
onMounted(() => {
  // Could load from URL params or localStorage here
})
</script>

<style scoped>
.preview-form-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.preview-form {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 20px;
}

.form-section {
  margin-bottom: 24px;
}

.section-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin: 0 0 16px 0;
  color: #1f2937;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}

.form-label {
  display: block;
  font-weight: 500;
  color: #374151;
  margin-bottom: 6px;
  font-size: 0.875rem;
}

.field-description {
  display: block;
  font-weight: 400;
  color: #6b7280;
  font-size: 0.75rem;
  margin-top: 2px;
}

.required {
  color: #dc2626;
  margin-left: 2px;
}

.form-input, .form-select {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-input:focus, .form-select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-select {
  background-color: #ffffff;
  cursor: pointer;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 400;
}

.checkbox-label input[type="checkbox"] {
  width: auto;
}

.config-field {
  margin-bottom: 16px;
}

.json-editor-container {
  position: relative;
}

.json-editor {
  width: 100%;
  min-height: 200px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.875rem;
  padding: 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background-color: #f8fafc;
  resize: vertical;
}

.json-editor:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.json-error {
  margin-top: 8px;
  padding: 8px 12px;
  background-color: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  color: #dc2626;
  font-size: 0.875rem;
}

.error-message {
  margin-top: 4px;
  color: #dc2626;
  font-size: 0.75rem;
}

.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
  flex-wrap: wrap;
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

.secondary-button:hover:not(:disabled) {
  background-color: #e5e7eb;
  border-color: #9ca3af;
}

.secondary-button:disabled {
  background-color: #f9fafb;
  color: #9ca3af;
  cursor: not-allowed;
}

.toggle-button {
  background: none;
  border: none;
  color: #3b82f6;
  font-size: 0.875rem;
  cursor: pointer;
  margin-left: 12px;
}

.toggle-button:hover {
  text-decoration: underline;
}

.loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 16px;
  z-index: 1000;
  color: white;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid rgba(255, 255, 255, 0.3);
  border-radius: 50%;
  border-top-color: #ffffff;
  animation: spin 1s ease-in-out infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.preview-results {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 20px;
}

.preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid #e5e7eb;
}

.preview-header h3 {
  margin: 0;
  color: #1f2937;
}

.preview-actions {
  display: flex;
  gap: 8px;
}

.action-button {
  background: #f3f4f6;
  border: 1px solid #d1d5db;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 0.875rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.action-button:hover {
  background: #e5e7eb;
}

.preview-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.summary-item {
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #f9fafb;
}

.summary-item.success {
  border-color: #10b981;
  background: #ecfdf5;
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
}

.json-details {
  margin-top: 20px;
}

.json-details summary {
  cursor: pointer;
  font-weight: 500;
  color: #374151;
  padding: 8px 0;
}

.json-details summary:hover {
  color: #1f2937;
}

.json-preview {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 16px;
  overflow: auto;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.875rem;
  line-height: 1.4;
  white-space: pre-wrap;
  max-height: 400px;
}

.error-display {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  padding: 16px;
  color: #dc2626;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.error-close {
  background: none;
  border: none;
  color: #dc2626;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 0;
  line-height: 1;
}

.error-close:hover {
  color: #991b1b;
}

/* Responsive design */
@media (max-width: 768px) {
  .preview-form-container {
    padding: 12px;
  }
  
  .preview-form {
    padding: 16px;
  }
  
  .form-actions {
    flex-direction: column;
  }
  
  .preview-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  
  .preview-actions {
    width: 100%;
    justify-content: flex-start;
  }
  
  .preview-summary {
    grid-template-columns: 1fr;
  }
}
</style>
