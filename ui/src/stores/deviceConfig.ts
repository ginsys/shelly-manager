import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getDeviceConfig,
  updateDeviceConfig,
  getCurrentDeviceConfig,
  getNormalizedCurrentConfig,
  getTypedNormalizedConfig,
  importDeviceConfig,
  getConfigImportStatus,
  exportDeviceConfig,
  detectConfigDrift,
  applyConfigTemplate,
  getConfigHistory,
  type DeviceConfig,
  type ConfigDrift,
  type ConfigHistoryEntry,
  type ConfigImportStatus
} from '@/api/deviceConfig'

export const useDeviceConfigStore = defineStore('deviceConfig', () => {
  // State
  const storedConfig = ref<DeviceConfig | null>(null)
  const liveConfig = ref<DeviceConfig | null>(null)
  const normalizedConfig = ref<DeviceConfig | null>(null)
  const typedConfig = ref<DeviceConfig | null>(null)
  const drift = ref<ConfigDrift | null>(null)
  const history = ref<ConfigHistoryEntry[]>([])
  const importStatus = ref<ConfigImportStatus | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Current device ID being managed
  const currentDeviceId = ref<number | string | null>(null)

  // Actions

  // Fetch stored configuration
  async function fetchStoredConfig(deviceId: number | string) {
    loading.value = true
    error.value = null
    currentDeviceId.value = deviceId
    try {
      storedConfig.value = await getDeviceConfig(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load stored configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Update stored configuration
  async function saveStoredConfig(deviceId: number | string, config: DeviceConfig) {
    loading.value = true
    error.value = null
    try {
      storedConfig.value = await updateDeviceConfig(deviceId, config)
    } catch (e: any) {
      error.value = e?.message || 'Failed to save configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Fetch live configuration from device
  async function fetchLiveConfig(deviceId: number | string, normalized = false) {
    loading.value = true
    error.value = null
    try {
      if (normalized) {
        liveConfig.value = await getNormalizedCurrentConfig(deviceId)
      } else {
        liveConfig.value = await getCurrentDeviceConfig(deviceId)
      }
    } catch (e: any) {
      error.value = e?.message || 'Failed to load live configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Fetch normalized configuration
  async function fetchNormalizedConfig(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      normalizedConfig.value = await getNormalizedCurrentConfig(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load normalized configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Fetch typed normalized configuration
  async function fetchTypedConfig(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      typedConfig.value = await getTypedNormalizedConfig(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load typed configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Import configuration to device
  async function importConfig(deviceId: number | string, config: DeviceConfig) {
    loading.value = true
    error.value = null
    try {
      await importDeviceConfig(deviceId, config)
      // Start polling for import status
      await pollImportStatus(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to import configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Poll import status
  async function pollImportStatus(deviceId: number | string, maxAttempts = 30) {
    let attempts = 0
    const poll = async () => {
      try {
        importStatus.value = await getConfigImportStatus(deviceId)
        if (importStatus.value.status === 'completed' || importStatus.value.status === 'failed') {
          return
        }
        if (attempts < maxAttempts) {
          attempts++
          setTimeout(poll, 1000) // Poll every second
        }
      } catch (e: any) {
        error.value = e?.message || 'Failed to get import status'
      }
    }
    await poll()
  }

  // Export configuration from device
  async function exportConfig(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      const config = await exportDeviceConfig(deviceId)
      storedConfig.value = config
      return config
    } catch (e: any) {
      error.value = e?.message || 'Failed to export configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Detect configuration drift
  async function checkDrift(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      drift.value = await detectConfigDrift(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to detect configuration drift'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Apply template to device
  async function applyTemplate(deviceId: number | string, templateId: number | string) {
    loading.value = true
    error.value = null
    try {
      await applyConfigTemplate(deviceId, templateId)
      // Refresh stored config after applying template
      await fetchStoredConfig(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to apply template'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Fetch configuration history
  async function fetchHistory(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      history.value = await getConfigHistory(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load configuration history'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Clear state
  function reset() {
    storedConfig.value = null
    liveConfig.value = null
    normalizedConfig.value = null
    typedConfig.value = null
    drift.value = null
    history.value = []
    importStatus.value = null
    loading.value = false
    error.value = null
    currentDeviceId.value = null
  }

  return {
    // State
    storedConfig,
    liveConfig,
    normalizedConfig,
    typedConfig,
    drift,
    history,
    importStatus,
    loading,
    error,
    currentDeviceId,

    // Actions
    fetchStoredConfig,
    saveStoredConfig,
    fetchLiveConfig,
    fetchNormalizedConfig,
    fetchTypedConfig,
    importConfig,
    pollImportStatus,
    exportConfig,
    checkDrift,
    applyTemplate,
    fetchHistory,
    reset
  }
})
