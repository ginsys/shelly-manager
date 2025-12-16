<template>
  <form @submit.prevent="handleSubmit" class="device-form">
    <div class="form-group">
      <label for="name">Device Name *</label>
      <input
        id="name"
        v-model="formData.name"
        type="text"
        class="form-input"
        required
        placeholder="Living Room Light"
      />
    </div>

    <div class="form-group">
      <label for="type">Device Type *</label>
      <select id="type" v-model="formData.type" class="form-input" required>
        <option value="">Select type...</option>
        <option value="shelly1">Shelly 1</option>
        <option value="shelly1pm">Shelly 1PM</option>
        <option value="shelly25">Shelly 2.5</option>
        <option value="shellyplug">Shelly Plug</option>
        <option value="shellyem">Shelly EM</option>
        <option value="shelly3em">Shelly 3EM</option>
        <option value="shellydimmer">Shelly Dimmer</option>
        <option value="shellyrgbw2">Shelly RGBW2</option>
      </select>
    </div>

    <div class="form-group">
      <label for="ipAddress">IP Address *</label>
      <input
        id="ipAddress"
        v-model="formData.ipAddress"
        type="text"
        class="form-input"
        required
        placeholder="192.168.1.100"
        pattern="^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$"
      />
    </div>

    <div class="form-group">
      <label for="mac">MAC Address</label>
      <input
        id="mac"
        v-model="formData.mac"
        type="text"
        class="form-input"
        placeholder="AA:BB:CC:DD:EE:FF"
        pattern="^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$"
      />
    </div>

    <div class="form-actions">
      <button type="submit" class="primary-button" :disabled="isSubmitting">
        {{ isSubmitting ? 'Saving...' : (isEdit ? 'Update Device' : 'Create Device') }}
      </button>
      <button type="button" class="secondary-button" @click="$emit('cancel')">
        Cancel
      </button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>
  </form>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import type { Device } from '@/api/types'

interface Props {
  device?: Device | null
  isEdit?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  device: null,
  isEdit: false
})

const emit = defineEmits<{
  submit: [data: Partial<Device>]
  cancel: []
}>()

const formData = reactive({
  name: '',
  type: '',
  ipAddress: '',
  mac: ''
})

const isSubmitting = ref(false)
const error = ref('')

// Populate form if editing
watch(() => props.device, (device) => {
  if (device) {
    formData.name = device.name || ''
    formData.type = device.type || ''
    formData.ipAddress = device.ipAddress || ''
    formData.mac = device.mac || ''
  }
}, { immediate: true })

function handleSubmit() {
  error.value = ''
  isSubmitting.value = true

  const data: Partial<Device> = {
    name: formData.name,
    type: formData.type,
    ipAddress: formData.ipAddress
  }

  if (formData.mac) {
    data.mac = formData.mac
  }

  emit('submit', data)
}

defineExpose({
  reset: () => {
    formData.name = ''
    formData.type = ''
    formData.ipAddress = ''
    formData.mac = ''
    error.value = ''
    isSubmitting.value = false
  },
  setError: (msg: string) => {
    error.value = msg
    isSubmitting.value = false
  }
})
</script>

<style scoped>
.device-form { max-width: 500px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-weight: 500; color: #374151; margin-bottom: 6px; }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-family: inherit; font-size: 14px; }
.form-input:focus { outline: none; border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1); }
.form-actions { display: flex; gap: 12px; margin-top: 20px; }
.primary-button { padding: 10px 20px; background: #2563eb; color: white; border: none; border-radius: 6px; cursor: pointer; font-weight: 500; }
.primary-button:disabled { opacity: 0.5; cursor: not-allowed; }
.secondary-button { padding: 10px 20px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; }
.error-message { margin-top: 12px; padding: 12px; background: #fee2e2; color: #991b1b; border-radius: 6px; }
</style>
