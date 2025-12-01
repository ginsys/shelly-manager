<template>
  <section class="bulk-operations">
    <h2>Bulk Provisioning</h2>
    <p style="color:#6b7280;font-size:14px;margin-bottom:16px">
      Apply configuration to multiple devices at once
    </p>

    <div v-if="!showForm" class="actions">
      <button class="primary-button" @click="showForm = true">Start Bulk Operation</button>
    </div>

    <form v-else class="bulk-form" @submit.prevent="handleSubmit">
      <div class="form-group">
        <label for="deviceIds">Device IDs (comma-separated)</label>
        <textarea
          id="deviceIds"
          v-model="deviceIdsInput"
          class="form-input"
          rows="3"
          placeholder="device-1, device-2, device-3"
          required
        ></textarea>
        <p class="help-text">Enter device IDs separated by commas</p>
      </div>

      <div class="form-group">
        <label for="config">Configuration (JSON)</label>
        <textarea
          id="config"
          v-model="configInput"
          class="form-input code"
          rows="8"
          placeholder='{"setting": "value"}'
          required
        ></textarea>
        <p class="help-text">Valid JSON configuration object</p>
      </div>

      <div class="form-actions">
        <button type="submit" class="primary-button" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating Tasks...' : 'Create Tasks' }}
        </button>
        <button type="button" class="secondary-button" @click="handleCancel">Cancel</button>
      </div>

      <div v-if="error" class="error-message">{{ error }}</div>
      <div v-if="success" class="success-message">
        Successfully created {{ success }} provisioning tasks
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useProvisioningStore } from '@/stores/provisioning'

const store = useProvisioningStore()
const showForm = ref(false)
const deviceIdsInput = ref('')
const configInput = ref('{\n  \n}')
const isSubmitting = ref(false)
const error = ref('')
const success = ref(0)

async function handleSubmit() {
  error.value = ''
  success.value = 0

  try {
    // Parse device IDs
    const deviceIds = deviceIdsInput.value
      .split(',')
      .map(id => id.trim())
      .filter(id => id.length > 0)

    if (deviceIds.length === 0) {
      error.value = 'Please enter at least one device ID'
      return
    }

    // Parse config
    let config: Record<string, unknown>
    try {
      config = JSON.parse(configInput.value)
    } catch (e) {
      error.value = 'Invalid JSON configuration: ' + (e as Error).message
      return
    }

    // Create bulk tasks
    isSubmitting.value = true
    const tasks = await store.createBulkTasks({ deviceIds, config })
    success.value = tasks.length

    // Reset form after delay
    setTimeout(() => {
      showForm.value = false
      deviceIdsInput.value = ''
      configInput.value = '{\n  \n}'
      success.value = 0
    }, 2000)

  } catch (e) {
    error.value = (e as Error).message
  } finally {
    isSubmitting.value = false
  }
}

function handleCancel() {
  showForm.value = false
  deviceIdsInput.value = ''
  configInput.value = '{\n  \n}'
  error.value = ''
  success.value = 0
}
</script>

<style scoped>
.bulk-operations { border: 1px solid #e5e7eb; padding: 20px; border-radius: 8px; background: white; }
.bulk-form { margin-top: 16px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-weight: 500; color: #374151; margin-bottom: 6px; }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-family: inherit; font-size: 14px; }
.form-input.code { font-family: 'Monaco', 'Courier New', monospace; font-size: 12px; }
.help-text { margin-top: 4px; font-size: 12px; color: #6b7280; }
.form-actions { display: flex; gap: 12px; margin-top: 20px; }
.primary-button { padding: 10px 20px; background: #2563eb; color: white; border: none; border-radius: 6px; cursor: pointer; font-weight: 500; }
.primary-button:disabled { opacity: 0.5; cursor: not-allowed; }
.secondary-button { padding: 10px 20px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; }
.error-message { margin-top: 12px; padding: 12px; background: #fee2e2; color: #991b1b; border-radius: 6px; }
.success-message { margin-top: 12px; padding: 12px; background: #d1fae5; color: #065f46; border-radius: 6px; }
</style>
