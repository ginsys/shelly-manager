<template>
  <div class="backup-form">
    <div class="form-header">
      <h2>{{ isEdit ? 'Edit Backup' : 'Create Backup' }}</h2>
      <button class="close-button" @click="$emit('cancel')" type="button">✖</button>
    </div>

    <form @submit.prevent="onSubmit" class="form-content">
      <!-- Basic Information -->
      <div class="form-section">
        <h3>Basic Information</h3>
        
        <div class="form-field">
          <label class="field-label">
            Backup Name *
            <span class="field-help">A descriptive name for this backup</span>
          </label>
          <input
            v-model="formData.name"
            type="text"
            required
            maxlength="100"
            placeholder="e.g. Production Devices Backup"
            class="form-input"
            :class="{ error: errors.name }"
          />
          <div v-if="errors.name" class="field-error">{{ errors.name }}</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            Description
            <span class="field-help">Optional description of the backup purpose</span>
          </label>
          <textarea
            v-model="formData.description"
            maxlength="500"
            placeholder="e.g. Weekly backup before maintenance window"
            class="form-textarea"
            rows="2"
          ></textarea>
        </div>

        <div class="form-field">
          <label class="field-label">
            Output Format *
            <span class="field-help">The format for backup data</span>
          </label>
          <select
            v-model="formData.format"
            required
            class="form-select"
            :class="{ error: errors.format }"
          >
            <option value="">Select format...</option>
            <option value="json">JSON - Full configuration data</option>
            <option value="sma">SMA - Shelly Manager Archive</option>
            <option value="yaml">YAML - Human readable format</option>
          </select>
          <div v-if="errors.format" class="field-error">{{ errors.format }}</div>
        </div>
      </div>

      <!-- SMA-specific configuration -->
      <SMAConfigForm
        v-if="formData.format === 'sma'"
        :device-count="estimatedDeviceCount"
        :template-count="availableDevices.length"
        @update:config="handleSMAConfigUpdate"
        @update:sizeEstimate="handleSMASize"
      />

      <!-- Device Selection -->
      <div class="form-section">
        <h3>Device Selection</h3>
        
        <div class="device-selection">
          <label class="checkbox-label">
            <input
              type="radio"
              :value="true"
              v-model="selectAllDevices"
              class="form-radio"
            />
            <span>All devices ({{ availableDevices.length }} devices)</span>
            <span class="field-help">Include all discovered devices in backup</span>
          </label>
          
          <label class="checkbox-label">
            <input
              type="radio"
              :value="false"
              v-model="selectAllDevices"
              class="form-radio"
            />
            <span>Select specific devices</span>
            <span class="field-help">Choose individual devices to backup</span>
          </label>
        </div>

        <div v-if="!selectAllDevices" class="device-list">
          <div class="device-list-header">
            <div class="device-count">{{ selectedDevices.length }} of {{ availableDevices.length }} selected</div>
            <div class="device-actions">
              <button type="button" @click="selectAllInList" class="select-all-btn">Select All</button>
              <button type="button" @click="clearSelection" class="clear-all-btn">Clear All</button>
            </div>
          </div>
          
          <div class="device-checkboxes" v-if="availableDevices.length > 0">
            <label 
              v-for="device in availableDevices" 
              :key="device.id"
              class="device-checkbox"
            >
              <input
                type="checkbox"
                :value="device.id"
                v-model="selectedDevices"
                class="device-checkbox-input"
              />
              <div class="device-info">
                <div class="device-name">{{ device.name || device.ip }}</div>
                <div class="device-details">
                  {{ device.type }} • {{ device.ip }} • {{ device.status }}
                </div>
              </div>
            </label>
          </div>
          
          <div v-else class="no-devices">
            No devices available. Please discover devices first.
          </div>
        </div>
      </div>

      <!-- Content Options -->
      <div class="form-section">
        <h3>Content Options</h3>
        
        <div class="content-options">
          <label class="checkbox-label">
            <input
              v-model="formData.include_settings"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Device Settings</span>
            <span class="field-help">Include device configuration settings</span>
          </label>

          <label class="checkbox-label">
            <input
              v-model="formData.include_schedules"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Schedules</span>
            <span class="field-help">Include export schedules and automation</span>
          </label>

          <label class="checkbox-label">
            <input
              v-model="formData.include_metrics"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Historical Metrics</span>
            <span class="field-help">Include historical metrics and statistics</span>
          </label>
        </div>
      </div>

      <!-- Security Options -->
      <div class="form-section">
        <h3>Security Options</h3>
        
        <div class="security-options">
          <label class="checkbox-label">
            <input
              v-model="formData.encrypt"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Encrypt backup file</span>
            <span class="field-help">Protect backup with password encryption</span>
          </label>

          <div v-if="formData.encrypt" class="encryption-password">
            <div class="form-field">
              <label class="field-label">
                Encryption Password *
                <span class="field-help">Strong password to encrypt the backup file</span>
              </label>
              <input
                v-model="formData.encryption_password"
                type="password"
                required
                minlength="8"
                placeholder="Enter strong password"
                class="form-input"
                :class="{ error: errors.password }"
              />
              <div v-if="errors.password" class="field-error">{{ errors.password }}</div>
            </div>

            <div class="form-field">
              <label class="field-label">
                Confirm Password *
              </label>
              <input
                v-model="confirmPassword"
                type="password"
                required
                placeholder="Confirm password"
                class="form-input"
                :class="{ error: errors.confirmPassword }"
              />
              <div v-if="errors.confirmPassword" class="field-error">{{ errors.confirmPassword }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Size Estimation -->
      <div class="form-section" v-if="sizeEstimate">
        <h3>Estimated Size</h3>
        <div class="size-estimate">
          <div class="estimate-item">
            <strong>Devices:</strong> {{ estimatedDeviceCount }} devices
          </div>
          <div class="estimate-item">
            <strong>Estimated Size:</strong> {{ formatFileSize(sizeEstimate) }}
          </div>
          <div class="estimate-item" v-if="formData.encrypt">
            <strong>Note:</strong> Encrypted files may be slightly larger
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
          <span v-if="loading">Creating Backup...</span>
          <span v-else>Create Backup</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import type { BackupRequest, BackupItem, SMAExportRequest } from '@/api/export'
import type { Device } from '@/api/types'
import SMAConfigForm from './SMAConfigForm.vue'

const props = defineProps<{
  backup?: BackupItem | null
  loading?: boolean
  error?: string
  availableDevices?: Device[]
}>()

const emit = defineEmits<{
  submit: [BackupRequest]
  cancel: []
}>()

const isEdit = computed(() => !!props.backup)

// Form data
const formData = reactive<BackupRequest>({
  name: '',
  description: '',
  format: 'json',
  devices: [],
  include_settings: true,
  include_schedules: true,
  include_metrics: false,
  encrypt: false,
  encryption_password: ''
})

// Form state
const selectedDevices = ref<number[]>([])
const selectAllDevices = ref(true)
const confirmPassword = ref('')
const sizeEstimate = ref(0)

// SMA-specific state
const smaConfig = ref<Partial<SMAExportRequest> | null>(null)
const smaSize = ref<{ originalSize: number; compressedSize: number; compressionRatio: number; recordCount: number } | null>(null)

// Validation errors
const errors = reactive<Record<string, string>>({})

// Computed properties
const availableDevices = computed(() => props.availableDevices || [])
const estimatedDeviceCount = computed(() => 
  selectAllDevices.value ? availableDevices.value.length : selectedDevices.value.length
)

const isFormValid = computed(() => {
  const hasName = formData.name.trim().length > 0
  const hasFormat = formData.format.length > 0
  const hasDevices = selectAllDevices.value || selectedDevices.value.length > 0
  const passwordValid = !formData.encrypt || (
    formData.encryption_password.length >= 8 &&
    formData.encryption_password === confirmPassword.value
  )
  const noErrors = Object.keys(errors).length === 0
  
  return hasName && hasFormat && hasDevices && passwordValid && noErrors
})

// Methods
function selectAllInList() {
  selectedDevices.value = availableDevices.value.map(d => d.id)
}

function clearSelection() {
  selectedDevices.value = []
}

function validateForm() {
  // Clear previous errors
  Object.keys(errors).forEach(key => delete errors[key])

  // Name validation
  if (!formData.name.trim()) {
    errors.name = 'Backup name is required'
  } else if (formData.name.length > 100) {
    errors.name = 'Name must be 100 characters or less'
  }

  // Format validation
  if (!formData.format) {
    errors.format = 'Format is required'
  }

  // Password validation
  if (formData.encrypt) {
    if (!formData.encryption_password) {
      errors.password = 'Password is required for encrypted backups'
    } else if (formData.encryption_password.length < 8) {
      errors.password = 'Password must be at least 8 characters'
    }

    if (formData.encryption_password !== confirmPassword.value) {
      errors.confirmPassword = 'Passwords do not match'
    }
  }

  // Device validation
  if (!selectAllDevices.value && selectedDevices.value.length === 0) {
    errors.devices = 'At least one device must be selected'
  }
}

function calculateSizeEstimate() {
  // Use SMA size estimate if available and format is SMA
  if (formData.format === 'sma' && smaSize.value) {
    sizeEstimate.value = smaSize.value.compressedSize
    return
  }

  // Simple size estimation based on device count and content options
  let baseSize = estimatedDeviceCount.value * 2048 // ~2KB per device base

  if (formData.include_settings) baseSize += estimatedDeviceCount.value * 1024
  if (formData.include_schedules) baseSize += estimatedDeviceCount.value * 512
  if (formData.include_metrics) baseSize += estimatedDeviceCount.value * 4096

  // Add format overhead
  if (formData.format === 'yaml') baseSize *= 1.3
  else if (formData.format === 'sma') baseSize *= 0.7

  sizeEstimate.value = Math.max(baseSize, 1024) // Minimum 1KB
}

// SMA-specific handlers
function handleSMAConfigUpdate(config: Partial<SMAExportRequest>) {
  smaConfig.value = config
}

function handleSMASize(size: { originalSize: number; compressedSize: number; compressionRatio: number; recordCount: number }) {
  smaSize.value = size
  calculateSizeEstimate()
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function onSubmit() {
  validateForm()
  
  if (!isFormValid.value) {
    return
  }
  
  // Prepare the base request
  let request: BackupRequest = {
    ...formData,
    devices: selectAllDevices.value ? undefined : selectedDevices.value
  }

  // Don't send password if not encrypting
  if (!request.encrypt) {
    delete request.encryption_password
  }

  // If SMA format, merge SMA configuration
  if (formData.format === 'sma' && smaConfig.value) {
    // Convert BackupRequest to SMA-compatible format
    const smaRequest = {
      plugin_name: 'sma' as const,
      format: 'sma' as const,
      config: smaConfig.value.config,
      filters: {
        ...smaConfig.value.filters,
        device_ids: selectAllDevices.value ? undefined : selectedDevices.value
      },
      options: {
        ...smaConfig.value.options,
        export_type: smaConfig.value.options?.export_type || 'manual'
      }
    }
    
    // Emit SMA request with backup metadata
    const enrichedRequest = {
      ...request,
      smaConfig: smaRequest
    }
    
    emit('submit', enrichedRequest)
  } else {
    emit('submit', request)
  }
}

// Watchers
watch([selectAllDevices, selectedDevices, formData], calculateSizeEstimate, { deep: true })
watch([formData, confirmPassword], validateForm, { deep: true })

// Clear password when encryption is disabled
watch(() => formData.encrypt, (encrypt) => {
  if (!encrypt) {
    formData.encryption_password = ''
    confirmPassword.value = ''
  }
})

// Clear SMA config when format changes away from SMA
watch(() => formData.format, (newFormat) => {
  if (newFormat !== 'sma') {
    smaConfig.value = null
    smaSize.value = null
    calculateSizeEstimate()
  }
})

// Initialize form when editing
onMounted(() => {
  if (props.backup) {
    // Initialize form with existing backup data
    formData.name = props.backup.name
    formData.description = props.backup.description || ''
    formData.format = props.backup.format
    formData.encrypt = props.backup.encrypted
    // Note: Can't restore device selection or passwords from existing backup
  }
  calculateSizeEstimate()
})
</script>

<style scoped>
.backup-form {
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

.form-input, .form-select, .form-textarea {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
  background: white;
}

.form-input:focus, .form-select:focus, .form-textarea:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input.error, .form-select.error, .form-textarea.error {
  border-color: #dc2626;
}

.form-textarea {
  resize: vertical;
  min-height: 60px;
}

.checkbox-label {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  cursor: pointer;
  margin-bottom: 12px;
}

.form-checkbox, .form-radio {
  width: auto;
  margin: 0;
}

.checkbox-label span {
  font-weight: 500;
  color: #374151;
}

.device-selection {
  margin-bottom: 16px;
}

.device-list {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #f9fafb;
  padding: 16px;
}

.device-list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.device-count {
  font-weight: 500;
  color: #374151;
}

.device-actions {
  display: flex;
  gap: 8px;
}

.select-all-btn, .clear-all-btn {
  background: none;
  border: 1px solid #d1d5db;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.select-all-btn:hover, .clear-all-btn:hover {
  background: #e5e7eb;
}

.device-checkboxes {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
  max-height: 200px;
  overflow-y: auto;
}

.device-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  background: white;
  cursor: pointer;
  transition: background-color 0.2s;
}

.device-checkbox:hover {
  background: #f3f4f6;
}

.device-checkbox-input {
  width: auto;
  margin: 0;
}

.device-info {
  flex: 1;
}

.device-name {
  font-weight: 500;
  color: #1f2937;
  font-size: 0.875rem;
}

.device-details {
  font-size: 0.75rem;
  color: #6b7280;
}

.no-devices {
  text-align: center;
  color: #9ca3af;
  font-style: italic;
  padding: 32px;
}

.content-options, .security-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.encryption-password {
  margin-top: 16px;
  padding: 16px;
  background: #fef3c7;
  border: 1px solid #fcd34d;
  border-radius: 6px;
}

.size-estimate {
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
  padding: 16px;
}

.estimate-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 0.875rem;
}

.estimate-item:last-child {
  margin-bottom: 0;
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

  .device-list-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .device-checkboxes {
    grid-template-columns: 1fr;
  }

  .form-actions {
    flex-direction: column;
  }

  .estimate-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
}
</style>