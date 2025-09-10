<template>
  <div class="sma-import-form">
    <div class="form-header">
      <h2>Import SMA File</h2>
      <button class="close-button" @click="$emit('cancel')" type="button">✖</button>
    </div>

    <div class="form-content">
      <!-- File Selection -->
      <div class="form-section">
        <h3>Select SMA File</h3>
        
        <div class="file-upload" :class="{ 'drag-over': isDragOver, 'has-file': selectedFile }">
          <input
            ref="fileInput"
            type="file"
            accept=".sma"
            @change="handleFileSelect"
            class="file-input"
            hidden
          />
          
          <div 
            class="upload-area"
            @click="$refs.fileInput?.click()"
            @dragover.prevent="isDragOver = true"
            @dragleave.prevent="isDragOver = false"
            @drop.prevent="handleFileDrop"
          >
            <div v-if="!selectedFile" class="upload-placeholder">
              <div class="upload-icon">📁</div>
              <h4>Choose SMA File</h4>
              <p>Click to browse or drag and drop your .sma file here</p>
              <div class="file-format-info">
                Supports: Shelly Management Archive (.sma) files
              </div>
            </div>
            
            <div v-else class="file-info">
              <div class="file-icon">📄</div>
              <div class="file-details">
                <h4>{{ selectedFile.name }}</h4>
                <p>{{ formatFileSize(selectedFile.size) }} • {{ selectedFile.type || 'application/octet-stream' }}</p>
                <div class="file-actions">
                  <button type="button" @click.stop="clearFile" class="clear-file-btn">
                    Remove File
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- File Preview -->
      <div v-if="previewData" class="form-section">
        <h3>File Preview</h3>
        
        <div class="preview-content">
          <!-- Validation Status -->
          <div class="validation-status" :class="{ 
            'valid': previewData.validation.valid, 
            'invalid': !previewData.validation.valid 
          }">
            <div class="status-header">
              <span class="status-icon">
                {{ previewData.validation.valid ? '✅' : '❌' }}
              </span>
              <h4>{{ previewData.validation.valid ? 'Valid SMA File' : 'Invalid SMA File' }}</h4>
            </div>
            
            <div class="validation-details">
              <div class="detail-item">
                <strong>SMA Version:</strong> {{ previewData.validation.sma_version }}
              </div>
              <div class="detail-item">
                <strong>Format Version:</strong> {{ previewData.validation.format_version }}
              </div>
              <div class="detail-item">
                <strong>Data Integrity:</strong> {{ previewData.validation.data_integrity }}%
              </div>
            </div>
          </div>

          <!-- Errors -->
          <div v-if="previewData.validation.errors.length > 0" class="errors-section">
            <h4>❌ Validation Errors</h4>
            <ul>
              <li v-for="error in previewData.validation.errors" :key="error">
                {{ error }}
              </li>
            </ul>
          </div>

          <!-- Warnings -->
          <div v-if="previewData.validation.warnings.length > 0" class="warnings-section">
            <h4>⚠️ Warnings</h4>
            <ul>
              <li v-for="warning in previewData.validation.warnings" :key="warning">
                {{ warning }}
              </li>
            </ul>
          </div>

          <!-- Summary -->
          <div class="summary-section">
            <h4>📊 Content Summary</h4>
            <div class="summary-grid">
              <div class="summary-item">
                <span class="summary-label">Devices:</span>
                <span class="summary-value">{{ previewData.summary.device_count }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Templates:</span>
                <span class="summary-value">{{ previewData.summary.template_count }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Discovered Devices:</span>
                <span class="summary-value">{{ previewData.summary.discovered_device_count }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Plugin Configs:</span>
                <span class="summary-value">{{ previewData.summary.plugin_config_count }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">File Size:</span>
                <span class="summary-value">{{ formatFileSize(previewData.summary.file_size) }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Compression Ratio:</span>
                <span class="summary-value">{{ Math.round((1 - previewData.summary.compression_ratio) * 100) }}%</span>
              </div>
            </div>
          </div>

          <!-- Available Sections -->
          <div class="sections-section">
            <h4>📋 Available Sections</h4>
            <div class="section-tags">
              <span 
                v-for="section in previewData.summary.sections" 
                :key="section"
                class="section-tag"
              >
                {{ formatSectionName(section) }}
              </span>
            </div>
          </div>

          <!-- Conflicts -->
          <div v-if="previewData.conflicts && previewData.conflicts.length > 0" class="conflicts-section">
            <h4>⚠️ Potential Conflicts</h4>
            <ul>
              <li v-for="conflict in previewData.conflicts" :key="conflict">
                {{ conflict }}
              </li>
            </ul>
          </div>

          <!-- Import Preview -->
          <div v-if="previewData.import_preview" class="import-preview-section">
            <h4>🔍 Import Preview</h4>
            <div class="import-stats">
              <div class="stat-group">
                <h5>Devices</h5>
                <div class="stat-item">New: {{ previewData.import_preview.devices_to_add }}</div>
                <div class="stat-item">Updates: {{ previewData.import_preview.devices_to_update }}</div>
              </div>
              <div class="stat-group">
                <h5>Templates</h5>
                <div class="stat-item">New: {{ previewData.import_preview.templates_to_add }}</div>
                <div class="stat-item">Updates: {{ previewData.import_preview.templates_to_update }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Import Options -->
      <div v-if="selectedFile && previewData" class="form-section">
        <h3>Import Options</h3>
        
        <div class="import-options">
          <div class="option-group">
            <h4>Validation Options</h4>
            
            <label class="checkbox-label">
              <input
                v-model="importOptions.validate_checksums"
                type="checkbox"
                class="form-checkbox"
              />
              <span>Validate checksums</span>
              <span class="field-help">Verify data integrity using SHA-256 checksums</span>
            </label>

            <label class="checkbox-label">
              <input
                v-model="importOptions.validate_structure"
                type="checkbox"
                class="form-checkbox"
              />
              <span>Validate structure</span>
              <span class="field-help">Verify SMA format structure and compatibility</span>
            </label>
          </div>

          <div class="option-group">
            <h4>Import Behavior</h4>
            
            <label class="checkbox-label">
              <input
                v-model="importOptions.dry_run"
                type="checkbox"
                class="form-checkbox"
              />
              <span>Dry run (preview only)</span>
              <span class="field-help">Preview changes without applying them</span>
            </label>

            <label class="checkbox-label">
              <input
                v-model="importOptions.backup_before"
                type="checkbox"
                class="form-checkbox"
              />
              <span>Create backup before import</span>
              <span class="field-help">Automatically create backup before applying changes</span>
            </label>
          </div>

          <div class="option-group">
            <h4>Merge Strategy</h4>
            <p class="group-description">How to handle conflicts with existing data:</p>
            
            <div class="radio-group">
              <label class="radio-label">
                <input
                  v-model="importOptions.merge_strategy"
                  type="radio"
                  value="overwrite"
                  class="form-radio"
                />
                <span>Overwrite existing data</span>
                <span class="field-help">Replace existing devices/templates with imported ones</span>
              </label>

              <label class="radio-label">
                <input
                  v-model="importOptions.merge_strategy"
                  type="radio"
                  value="merge"
                  class="form-radio"
                />
                <span>Merge with existing data</span>
                <span class="field-help">Combine imported data with existing data</span>
              </label>

              <label class="radio-label">
                <input
                  v-model="importOptions.merge_strategy"
                  type="radio"
                  value="skip"
                  class="form-radio"
                />
                <span>Skip conflicts</span>
                <span class="field-help">Only import data that doesn't conflict</span>
              </label>
            </div>
          </div>

          <div class="option-group" v-if="previewData.summary.sections.length > 1">
            <h4>Import Sections</h4>
            <p class="group-description">Select which sections to import:</p>
            
            <div class="section-checkboxes">
              <label 
                v-for="section in previewData.summary.sections" 
                :key="section"
                class="checkbox-label"
              >
                <input
                  v-model="importOptions.import_sections"
                  type="checkbox"
                  :value="section"
                  class="form-checkbox"
                />
                <span>{{ formatSectionName(section) }}</span>
                <span class="field-help">{{ getSectionDescription(section) }}</span>
              </label>
            </div>
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
          v-if="selectedFile && !previewData && !previewLoading"
          type="button"
          @click="previewFile"
          class="secondary-button"
        >
          Preview File
        </button>
        <button
          v-if="selectedFile && previewData"
          type="button"
          @click="executeImport"
          :disabled="!canImport || importLoading"
          class="primary-button"
        >
          <span v-if="importLoading">{{ importOptions.dry_run ? 'Running Preview...' : 'Importing...' }}</span>
          <span v-else>{{ importOptions.dry_run ? 'Run Dry Run' : 'Import Data' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useImportStore } from '@/stores/import'
import type { SMAImportRequest, SMAPreview } from '@/api/export'

const emit = defineEmits<{
  cancel: []
  success: [{ importId: string; preview: boolean }]
}>()

// Store
const importStore = useImportStore()

// Form state
const selectedFile = ref<File | null>(null)
const isDragOver = ref(false)
const previewData = ref<SMAPreview | null>(null)
const previewLoading = ref(false)
const importLoading = ref(false)
const error = ref('')

const importOptions = reactive({
  validate_checksums: true,
  validate_structure: true,
  dry_run: true,
  merge_strategy: 'merge' as 'overwrite' | 'merge' | 'skip',
  backup_before: true,
  import_sections: [] as string[]
})

// Computed properties
const canImport = computed(() => {
  return selectedFile.value && 
         previewData.value && 
         previewData.value.validation.valid &&
         importOptions.import_sections.length > 0
})

// Methods
function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    selectFile(input.files[0])
  }
}

function handleFileDrop(event: DragEvent) {
  isDragOver.value = false
  
  if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
    selectFile(event.dataTransfer.files[0])
  }
}

function selectFile(file: File) {
  // Validate file type
  if (!file.name.toLowerCase().endsWith('.sma')) {
    error.value = 'Please select a valid .sma file'
    return
  }

  // Validate file size (max 100MB)
  if (file.size > 100 * 1024 * 1024) {
    error.value = 'File size too large. Maximum size is 100MB.'
    return
  }

  selectedFile.value = file
  previewData.value = null
  error.value = ''
  importOptions.import_sections = []
}

function clearFile() {
  selectedFile.value = null
  previewData.value = null
  error.value = ''
  importOptions.import_sections = []
}

async function previewFile() {
  if (!selectedFile.value) return

  previewLoading.value = true
  error.value = ''

  try {
    const preview = await importStore.previewSMAFile(selectedFile.value, {
      validate_checksums: importOptions.validate_checksums,
      validate_structure: importOptions.validate_structure
    })

    previewData.value = preview
    
    // Auto-select all available sections
    importOptions.import_sections = [...preview.summary.sections]

  } catch (err: any) {
    error.value = err.message || 'Failed to preview SMA file'
  } finally {
    previewLoading.value = false
  }
}

async function executeImport() {
  if (!selectedFile.value || !canImport.value) return

  importLoading.value = true
  error.value = ''

  try {
    const request: SMAImportRequest = {
      file: selectedFile.value,
      options: {
        validate_checksums: importOptions.validate_checksums,
        validate_structure: importOptions.validate_structure,
        dry_run: importOptions.dry_run,
        merge_strategy: importOptions.merge_strategy,
        backup_before: importOptions.backup_before,
        import_sections: importOptions.import_sections
      }
    }

    const importId = await importStore.importSMAFile(request)
    
    emit('success', { 
      importId, 
      preview: importOptions.dry_run 
    })

  } catch (err: any) {
    error.value = err.message || 'Failed to import SMA file'
  } finally {
    importLoading.value = false
  }
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatSectionName(section: string): string {
  const names: Record<string, string> = {
    'devices': 'Devices',
    'templates': 'Templates', 
    'discovered_devices': 'Discovered Devices',
    'network_settings': 'Network Settings',
    'plugin_configurations': 'Plugin Configurations',
    'system_settings': 'System Settings'
  }
  return names[section] || section
}

function getSectionDescription(section: string): string {
  const descriptions: Record<string, string> = {
    'devices': 'Managed device configurations and settings',
    'templates': 'Device configuration templates',
    'discovered_devices': 'Unmanaged devices found during discovery',
    'network_settings': 'WiFi networks and MQTT configuration',
    'plugin_configurations': 'Plugin settings and configurations',
    'system_settings': 'Application-level system settings'
  }
  return descriptions[section] || 'Data section'
}
</script>

<style scoped>
.sma-import-form {
  background: white;
  border-radius: 8px;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-width: 800px;
  width: 100%;
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
  margin-bottom: 24px;
}

.form-section h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
  font-size: 1.125rem;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}

.file-upload {
  border: 2px dashed #d1d5db;
  border-radius: 8px;
  transition: all 0.2s;
  overflow: hidden;
}

.file-upload.drag-over {
  border-color: #3b82f6;
  background: #eff6ff;
}

.file-upload.has-file {
  border-color: #10b981;
  border-style: solid;
}

.upload-area {
  padding: 32px;
  text-align: center;
  cursor: pointer;
  transition: background-color 0.2s;
}

.upload-area:hover {
  background: #f9fafb;
}

.upload-placeholder .upload-icon {
  font-size: 3rem;
  margin-bottom: 16px;
}

.upload-placeholder h4 {
  margin: 0 0 8px 0;
  color: #1f2937;
  font-size: 1.125rem;
}

.upload-placeholder p {
  margin: 0 0 16px 0;
  color: #6b7280;
}

.file-format-info {
  font-size: 0.75rem;
  color: #9ca3af;
  font-style: italic;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 16px;
  text-align: left;
}

.file-icon {
  font-size: 2rem;
}

.file-details h4 {
  margin: 0 0 4px 0;
  color: #1f2937;
  font-size: 1rem;
}

.file-details p {
  margin: 0 0 8px 0;
  color: #6b7280;
  font-size: 0.875rem;
}

.clear-file-btn {
  background: #ef4444;
  color: white;
  border: none;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.clear-file-btn:hover {
  background: #dc2626;
}

.preview-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.validation-status {
  padding: 16px;
  border-radius: 6px;
  border: 1px solid;
}

.validation-status.valid {
  background: #f0fdf4;
  border-color: #bbf7d0;
  color: #166534;
}

.validation-status.invalid {
  background: #fef2f2;
  border-color: #fecaca;
  color: #991b1b;
}

.status-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.status-header h4 {
  margin: 0;
  font-size: 1rem;
}

.validation-details {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 8px;
}

.detail-item {
  font-size: 0.875rem;
}

.errors-section, .warnings-section, .conflicts-section {
  padding: 12px 16px;
  border-radius: 4px;
}

.errors-section {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.warnings-section {
  background: #fffbeb;
  border: 1px solid #fed7aa;
  color: #92400e;
}

.conflicts-section {
  background: #fdf4ff;
  border: 1px solid #e9d5ff;
  color: #7c3aed;
}

.errors-section h4, .warnings-section h4, .conflicts-section h4 {
  margin: 0 0 8px 0;
  font-size: 0.875rem;
  font-weight: 600;
}

.errors-section ul, .warnings-section ul, .conflicts-section ul {
  margin: 0;
  padding-left: 16px;
  font-size: 0.875rem;
}

.summary-section {
  padding: 16px;
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
}

.summary-section h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
  font-size: 1rem;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px;
}

.summary-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.875rem;
}

.summary-label {
  color: #6b7280;
}

.summary-value {
  color: #1f2937;
  font-weight: 600;
}

.sections-section {
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.sections-section h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
  font-size: 1rem;
}

.section-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.section-tag {
  background: #dbeafe;
  color: #1e40af;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}

.import-preview-section {
  padding: 16px;
  background: #fefce8;
  border: 1px solid #fde047;
  border-radius: 6px;
}

.import-preview-section h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
  font-size: 1rem;
}

.import-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 16px;
}

.stat-group h5 {
  margin: 0 0 8px 0;
  color: #374151;
  font-size: 0.875rem;
  font-weight: 600;
}

.stat-item {
  font-size: 0.875rem;
  color: #6b7280;
  margin-bottom: 4px;
}

.import-options {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.option-group {
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.option-group h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
  font-size: 1rem;
  font-weight: 600;
}

.group-description {
  margin: 0 0 12px 0;
  color: #6b7280;
  font-size: 0.875rem;
}

.checkbox-label, .radio-label {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  cursor: pointer;
  margin-bottom: 12px;
}

.checkbox-label span, .radio-label span {
  font-weight: 500;
  color: #374151;
}

.field-help {
  display: block;
  font-weight: 400 !important;
  color: #6b7280 !important;
  font-size: 0.75rem !important;
  margin-top: 2px;
}

.form-checkbox, .form-radio {
  width: auto;
  margin: 0;
}

.radio-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-checkboxes {
  display: flex;
  flex-direction: column;
  gap: 8px;
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

  .upload-area {
    padding: 24px 16px;
  }

  .summary-grid {
    grid-template-columns: 1fr;
  }

  .import-stats {
    grid-template-columns: 1fr;
  }

  .form-actions {
    flex-direction: column;
  }

  .validation-details {
    grid-template-columns: 1fr;
  }
}
</style>