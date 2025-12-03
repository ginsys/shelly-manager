<template>
  <div class="preview-form-container">
    <form class="preview-form" @submit.prevent="onPreview">
      <!-- Plugin Selection -->
      <div class="form-section">
        <label class="form-label">Import Plugin *</label>
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
        <label class="form-label">Input Format *</label>
        <select v-model="formData.format" @change="onFormatChange" required class="form-select">
          <option value="">Select format...</option>
          <option v-for="format in availableFormats" :key="format" :value="format">
            {{ format.toUpperCase() }}
          </option>
        </select>
        <div v-if="formatError" class="error-message">{{ formatError }}</div>
      </div>

      <!-- File Upload -->
      <div class="form-section" v-if="formData.plugin_name && formData.format">
        <label class="form-label">Import Data</label>
        <div class="file-input-container">
          <input type="file" 
                 @change="onFileChange" 
                 :accept="getAcceptedTypes()"
                 class="file-input"
                 id="import-file" />
          <label for="import-file" class="file-input-label">
            {{ selectedFile ? selectedFile.name : 'Choose file...' }}
          </label>
        </div>
        <div v-if="selectedFile" class="file-info">
          Size: {{ formatFileSize(selectedFile.size) }} | 
          Type: {{ selectedFile.type || 'Unknown' }}
        </div>
        
        <!-- Text input alternative -->
        <div class="text-input-toggle">
          <button type="button" 
                  @click="showTextInput = !showTextInput"
                  class="toggle-button">
            {{ showTextInput ? 'Hide' : 'Show' }} Text Input
          </button>
        </div>
        
        <div v-if="showTextInput" class="text-input-container">
          <textarea v-model="textData" 
                    class="text-input"
                    :placeholder="`Paste your ${formData.format.toUpperCase()} data here...`"
                    rows="6"></textarea>
        </div>
      </div>

      <!-- Dynamic Plugin Configuration -->
      <div class="form-section" v-if="pluginSchema?.config">
        <h3 class="section-title">Import Configuration</h3>
        <SchemaForm :schema="pluginSchema.config" v-model="formData.config" />
      </div>

      <!-- Import Options -->
      <div class="form-section" v-if="pluginSchema?.options">
        <h3 class="section-title">Import Options</h3>
        <SchemaForm :schema="pluginSchema.options" v-model="formData.options" />
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
            placeholder="{ &#10;  &quot;config&quot;: {},&#10;  &quot;options&quot;: {}&#10;}"
            rows="6"
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
          {{ isLoading ? 'Analyzing...' : 'Preview Import' }}
        </button>
      </div>
    </form>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading-overlay">
      <div class="spinner"></div>
      <span>Analyzing import data...</span>
    </div>

    <!-- Preview Results -->
    <div v-if="previewResult" class="preview-results">
      <div class="preview-header">
        <h3>Import Preview</h3>
        <div class="preview-actions">
          <button @click="copyToClipboard" class="action-button">📋 Copy JSON</button>
          <button @click="downloadJson" class="action-button">💾 Download</button>
          <button @click="clearPreview" class="action-button">✖ Clear</button>
        </div>
      </div>

      <!-- Summary Information -->
      <div class="preview-summary">
        <div class="summary-item" :class="{ success: previewResult.success }">
          <strong>Status:</strong> 
          {{ previewResult.success ? 'Ready to Import' : 'Issues Found' }}
        </div>
        <div class="summary-item" v-if="previewResult.will_create">
          <strong>Will Create:</strong> {{ previewResult.will_create.toLocaleString() }} records
        </div>
        <div class="summary-item" v-if="previewResult.will_update">
          <strong>Will Update:</strong> {{ previewResult.will_update.toLocaleString() }} records
        </div>
        <div class="summary-item" v-if="previewResult.will_skip">
          <strong>Will Skip:</strong> {{ previewResult.will_skip.toLocaleString() }} records
        </div>
      </div>

      <!-- Changes Preview -->
      <div v-if="previewResult.changes" class="changes-section">
        <h4>📝 Proposed Changes</h4>
        <div class="changes-list">
          <div v-for="change in previewResult.changes.slice(0, 5)" :key="change.id" class="change-item">
            <span class="change-type" :class="change.type">{{ change.type.toUpperCase() }}</span>
            <span class="change-target">{{ change.target }}</span>
            <span class="change-description">{{ change.description }}</span>
          </div>
          <div v-if="previewResult.changes.length > 5" class="more-changes">
            ... and {{ previewResult.changes.length - 5 }} more changes
          </div>
        </div>
      </div>

      <!-- Warnings -->
      <div v-if="previewResult.warnings?.length" class="warnings-section">
        <h4>⚠️ Warnings</h4>
        <ul class="warnings-list">
          <li v-for="warning in previewResult.warnings" :key="warning" class="warning-item">
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
import { previewImport } from '@/api/import'
import SchemaForm from '@/components/shared/SchemaForm.vue'

// Plugin schema type for form generation (same as export)
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
  options?: Record<string, PluginField>
}

interface Plugin {
  id: string
  name: string
  formats: string[]
  schema?: PluginSchema
  acceptedTypes?: string[]
}

interface ImportRequest {
  plugin_name: string
  format: string
  config?: Record<string, any>
  options?: Record<string, any>
  data?: string
}

// Reactive state
const formData = reactive<ImportRequest>({
  plugin_name: '',
  format: '',
  config: {},
  options: {}
})

const jsonConfig = ref('')
const showAdvanced = ref(false)
const showTextInput = ref(false)
const textData = ref('')
const selectedFile = ref<File | null>(null)
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
    name: 'Mock File Import',
    formats: ['txt', 'csv', 'json'],
    acceptedTypes: ['.txt', '.csv', '.json'],
    schema: {
      config: {
        skip_duplicates: { type: 'boolean', label: 'Skip Duplicates', description: 'Skip records that already exist' },
        validate_format: { type: 'boolean', label: 'Validate Format', description: 'Validate data format before import' },
        batch_size: { type: 'number', label: 'Batch Size', placeholder: '100', min: 1, max: 10000 },
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
      options: {
        dry_run: { type: 'boolean', label: 'Dry Run', description: 'Preview changes without applying them' },
        backup: { type: 'boolean', label: 'Create Backup', description: 'Create backup before import' }
      }
    }
  },
  {
    id: 'gitops',
    name: 'GitOps Configuration Import',
    formats: ['yaml', 'json'],
    acceptedTypes: ['.yaml', '.yml', '.json'],
    schema: {
      config: {
        namespace: { type: 'string', label: 'Target Namespace', placeholder: 'shelly-manager', required: true },
        merge_strategy: { 
          type: 'select', 
          label: 'Merge Strategy',
          options: [
            { value: 'replace', label: 'Replace Existing' },
            { value: 'merge', label: 'Merge with Existing' },
            { value: 'append', label: 'Append Only' }
          ]
        },
        validate_secrets: { type: 'boolean', label: 'Validate Secrets', description: 'Validate secret references' }
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
         (selectedFile.value || textData.value.trim()) &&
         !pluginError.value && 
         !formatError.value && 
         !jsonError.value
})

const hasStoredConfig = computed(() => {
  return localStorage.getItem(`import-config-${formData.plugin_name}`) !== null
})

// Storage key for persistence
const storageKey = computed(() => `import-config-${formData.plugin_name}-${formData.format}`)

// Methods
function onPluginChange() {
  formData.format = ''
  formData.config = {}
  formData.options = {}
  selectedFile.value = null
  textData.value = ''
  pluginError.value = ''
  formatError.value = ''
  previewResult.value = null
  errorMessage.value = ''
}

function onFormatChange() {
  previewResult.value = null
  errorMessage.value = ''
}

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  selectedFile.value = target.files?.[0] || null
  textData.value = '' // Clear text input when file selected
}

function getAcceptedTypes(): string {
  const plugin = availablePlugins.value.find(p => p.id === formData.plugin_name)
  return plugin?.acceptedTypes?.join(',') || '*'
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
    // Prepare data from file or text input
    let data = textData.value
    if (selectedFile.value && !data) {
      data = await readFileAsText(selectedFile.value)
    }
    
    const requestData: ImportRequest = {
      ...formData,
      data
    }
    
    const result = await previewImport(requestData)
    previewResult.value = result.summary || result
    
    // Save successful config to localStorage
    saveToStorage()
    
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Preview failed'
  } finally {
    isLoading.value = false
  }
}

function readFileAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsText(file)
  })
}

function saveToStorage() {
  try {
    localStorage.setItem(storageKey.value, JSON.stringify({
      config: formData.config,
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
  a.download = `import-preview-${formData.plugin_name}-${Date.now()}.json`
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
/* Import the same styles as ExportPreviewForm with some additions */
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

/* File input specific styles */
.file-input-container {
  position: relative;
}

.file-input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.file-input-label {
  display: block;
  width: 100%;
  padding: 8px 12px;
  border: 2px dashed #d1d5db;
  border-radius: 6px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.2s, background-color 0.2s;
  background-color: #f9fafb;
}

.file-input-label:hover {
  border-color: #3b82f6;
  background-color: #f3f4f6;
}

.file-info {
  margin-top: 8px;
  font-size: 0.875rem;
  color: #6b7280;
}

.text-input-toggle {
  margin: 12px 0;
}

.text-input-container {
  margin-top: 12px;
}

.text-input {
  width: 100%;
  min-height: 150px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.875rem;
  padding: 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background-color: #f8fafc;
  resize: vertical;
}

.text-input:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.json-editor-container {
  position: relative;
}

.json-editor {
  width: 100%;
  min-height: 150px;
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

/* Changes section for imports */
.changes-section {
  margin-bottom: 20px;
  padding: 16px;
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
}

.changes-section h4 {
  margin: 0 0 12px 0;
  color: #0369a1;
}

.changes-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.change-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px;
  background: #ffffff;
  border-radius: 4px;
  font-size: 0.875rem;
}

.change-type {
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
  font-size: 0.75rem;
  min-width: 60px;
  text-align: center;
}

.change-type.create {
  background: #dcfce7;
  color: #166534;
}

.change-type.update {
  background: #fef3c7;
  color: #92400e;
}

.change-type.skip {
  background: #f3f4f6;
  color: #374151;
}

.change-target {
  font-weight: 500;
  color: #1f2937;
  min-width: 120px;
}

.change-description {
  color: #6b7280;
  flex: 1;
}

.more-changes {
  padding: 8px;
  text-align: center;
  color: #6b7280;
  font-style: italic;
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
  
  .change-item {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
