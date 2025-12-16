<template>
  <div class="device-form">
    <div class="form-header">
      <h2>{{ isEdit ? 'Edit Device' : 'Add Device' }}</h2>
      <button class="close-button" @click="$emit('cancel')" type="button">✖</button>
    </div>

    <form @submit.prevent="onSubmit" class="form-content">
      <!-- Basic Information -->
      <div class="form-section">
        <h3>Device Information</h3>

        <div class="form-field">
          <label class="field-label">
            Device Name
            <span class="field-help">A friendly name for this device</span>
          </label>
          <input
            v-model="formData.name"
            type="text"
            placeholder="e.g. Living Room Light"
            class="form-input"
            :class="{ error: errors.name }"
          />
          <div v-if="errors.name" class="field-error">{{ errors.name }}</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            IP Address *
            <span class="field-help">The device's IP address on your network</span>
          </label>
          <input
            v-model="formData.ip"
            type="text"
            required
            placeholder="e.g. 192.168.1.100"
            class="form-input"
            :class="{ error: errors.ip }"
          />
          <div v-if="errors.ip" class="field-error">{{ errors.ip }}</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            MAC Address *
            <span class="field-help">The device's MAC address (format: XX:XX:XX:XX:XX:XX)</span>
          </label>
          <input
            v-model="formData.mac"
            type="text"
            required
            placeholder="e.g. AA:BB:CC:DD:EE:FF"
            class="form-input"
            :class="{ error: errors.mac }"
            :disabled="isEdit"
          />
          <div v-if="errors.mac" class="field-error">{{ errors.mac }}</div>
          <div v-if="isEdit" class="field-help">MAC address cannot be changed</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            Device Type
            <span class="field-help">Model or type identifier (e.g. SHSW-1, SHPLG-S)</span>
          </label>
          <input
            v-model="formData.type"
            type="text"
            placeholder="e.g. SHSW-1"
            class="form-input"
            :class="{ error: errors.type }"
          />
          <div v-if="errors.type" class="field-error">{{ errors.type }}</div>
        </div>
      </div>

      <!-- Error Display -->
      <div v-if="error" class="form-error">{{ error }}</div>

      <!-- Form Actions -->
      <div class="form-actions">
        <button type="button" @click="$emit('cancel')" class="secondary-button">
          Cancel
        </button>
        <button type="submit" :disabled="!isFormValid || loading" class="primary-button">
          {{ loading ? (isEdit ? 'Updating...' : 'Creating...') : (isEdit ? 'Update Device' : 'Create Device') }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import type { Device, CreateDeviceRequest, UpdateDeviceRequest } from '@/api/types'

const props = defineProps<{
  existingDevice?: Device | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  submit: [CreateDeviceRequest | UpdateDeviceRequest]
  cancel: []
}>()

const isEdit = computed(() => !!props.existingDevice)

// Form data
const formData = reactive<CreateDeviceRequest>({
  ip: '',
  mac: '',
  name: '',
  type: '',
})

// Validation errors
const errors = reactive<Record<string, string>>({})

// IP address validation regex
const IP_REGEX = /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/

// MAC address validation regex
const MAC_REGEX = /^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/

// Computed validity
const isFormValid = computed(() => {
  return (
    formData.ip &&
    formData.mac &&
    Object.keys(errors).length === 0
  )
})

// Validation function
function validateForm() {
  // Clear previous errors
  Object.keys(errors).forEach(key => delete errors[key])

  // Validate IP address
  if (!formData.ip) {
    errors.ip = 'IP address is required'
  } else if (!IP_REGEX.test(formData.ip)) {
    errors.ip = 'Invalid IP address format (e.g. 192.168.1.100)'
  }

  // Validate MAC address
  if (!formData.mac) {
    errors.mac = 'MAC address is required'
  } else if (!MAC_REGEX.test(formData.mac)) {
    errors.mac = 'Invalid MAC address format (e.g. AA:BB:CC:DD:EE:FF)'
  }

  // Validate name (if provided)
  if (formData.name && formData.name.length > 100) {
    errors.name = 'Name must be 100 characters or less'
  }

  // Validate type (if provided)
  if (formData.type && formData.type.length > 50) {
    errors.type = 'Type must be 50 characters or less'
  }
}

// Submit handler
function onSubmit() {
  validateForm()
  if (!isFormValid.value) return

  // Prepare submission data
  const submitData: CreateDeviceRequest | UpdateDeviceRequest = {
    ip: formData.ip,
    mac: formData.mac,
  }

  // Add optional fields only if they have values
  if (formData.name) submitData.name = formData.name
  if (formData.type) submitData.type = formData.type

  emit('submit', submitData)
}

// Initialize for edit mode
onMounted(() => {
  if (props.existingDevice) {
    formData.ip = props.existingDevice.ip
    formData.mac = props.existingDevice.mac
    formData.name = props.existingDevice.name || ''
    formData.type = props.existingDevice.type || ''
  }
})

// Watch for real-time validation
watch(formData, validateForm, { deep: true })
</script>

<style scoped>
.device-form {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  max-width: 600px;
  margin: 0 auto;
}

.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid #e5e7eb;
}

.form-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #111827;
}

.close-button {
  background: none;
  border: none;
  font-size: 20px;
  color: #6b7280;
  cursor: pointer;
  padding: 4px 8px;
  line-height: 1;
}

.close-button:hover {
  color: #111827;
}

.form-content {
  padding: 24px;
}

.form-section {
  margin-bottom: 24px;
}

.form-section:last-of-type {
  margin-bottom: 0;
}

.form-section h3 {
  margin: 0 0 16px 0;
  font-size: 16px;
  font-weight: 600;
  color: #374151;
}

.form-field {
  margin-bottom: 16px;
}

.field-label {
  display: block;
  margin-bottom: 6px;
  font-weight: 500;
  color: #374151;
  font-size: 14px;
}

.field-help {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: #6b7280;
  font-weight: 400;
}

.form-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 14px;
  transition: border-color 0.2s;
}

.form-input:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input.error {
  border-color: #ef4444;
}

.form-input:disabled {
  background-color: #f3f4f6;
  cursor: not-allowed;
  color: #6b7280;
}

.field-error {
  margin-top: 4px;
  font-size: 12px;
  color: #ef4444;
}

.form-error {
  padding: 12px 16px;
  background-color: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  color: #dc2626;
  font-size: 14px;
  margin-bottom: 16px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid #e5e7eb;
}

.primary-button,
.secondary-button {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.primary-button {
  background-color: #3b82f6;
  color: white;
}

.primary-button:hover:not(:disabled) {
  background-color: #2563eb;
}

.primary-button:disabled {
  background-color: #93c5fd;
  cursor: not-allowed;
}

.secondary-button {
  background-color: white;
  color: #374151;
  border: 1px solid #d1d5db;
}

.secondary-button:hover {
  background-color: #f9fafb;
}
</style>
