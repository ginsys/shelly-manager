import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getTypedConfig,
  updateTypedConfig,
  getDeviceCapabilities,
  validateTypedConfig,
  convertToTyped,
  convertToRaw,
  getConfigSchema,
  bulkValidateConfigs,
  type TypedConfig,
  type DeviceCapabilities,
  type ValidationResult,
  type ConfigSchema,
  type BulkValidationResult
} from '@/api/typedConfig'

export const useTypedConfigStore = defineStore('typedConfig', () => {
  // State
  const typedConfig = ref<TypedConfig | null>(null)
  const capabilities = ref<DeviceCapabilities | null>(null)
  const schema = ref<ConfigSchema | null>(null)
  const validationResult = ref<ValidationResult | null>(null)
  const bulkValidationResult = ref<BulkValidationResult | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Schema cache: deviceType -> schema
  const schemaCache = ref<Map<string, ConfigSchema>>(new Map())

  // Actions

  // Fetch typed configuration for a device
  async function fetchTypedConfig(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      typedConfig.value = await getTypedConfig(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load typed configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Update typed configuration
  async function updateConfig(deviceId: number | string, config: Record<string, any>) {
    loading.value = true
    error.value = null
    try {
      typedConfig.value = await updateTypedConfig(deviceId, config)
      return typedConfig.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to update typed configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Fetch device capabilities
  async function fetchCapabilities(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      capabilities.value = await getDeviceCapabilities(deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load device capabilities'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Validate typed configuration
  async function validate(config: Record<string, any>, deviceType?: string) {
    loading.value = true
    error.value = null
    try {
      validationResult.value = await validateTypedConfig({ config, deviceType })
      return validationResult.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to validate configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Convert raw to typed
  async function toTyped(config: Record<string, any>, deviceType?: string) {
    loading.value = true
    error.value = null
    try {
      const result = await convertToTyped({ config, deviceType })
      return result
    } catch (e: any) {
      error.value = e?.message || 'Failed to convert to typed configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Convert typed to raw
  async function toRaw(config: Record<string, any>, deviceType?: string) {
    loading.value = true
    error.value = null
    try {
      const result = await convertToRaw({ config, deviceType })
      return result
    } catch (e: any) {
      error.value = e?.message || 'Failed to convert to raw configuration'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Fetch configuration schema (with caching)
  async function fetchSchema(deviceType?: string) {
    const cacheKey = deviceType || 'default'

    // Check cache first
    if (schemaCache.value.has(cacheKey)) {
      schema.value = schemaCache.value.get(cacheKey)!
      return schema.value
    }

    loading.value = true
    error.value = null
    try {
      schema.value = await getConfigSchema(deviceType)
      // Cache the schema
      schemaCache.value.set(cacheKey, schema.value)
      return schema.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to load configuration schema'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Bulk validate configurations
  async function bulkValidate(configs: Array<{ deviceId: string; config: Record<string, any> }>) {
    loading.value = true
    error.value = null
    try {
      bulkValidationResult.value = await bulkValidateConfigs({ configs })
      return bulkValidationResult.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to bulk validate configurations'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Clear cached schemas
  function clearSchemaCache() {
    schemaCache.value.clear()
  }

  // Clear validation results
  function clearValidation() {
    validationResult.value = null
    bulkValidationResult.value = null
  }

  // Clear all state
  function reset() {
    typedConfig.value = null
    capabilities.value = null
    schema.value = null
    validationResult.value = null
    bulkValidationResult.value = null
    loading.value = false
    error.value = null
    schemaCache.value.clear()
  }

  return {
    // State
    typedConfig,
    capabilities,
    schema,
    validationResult,
    bulkValidationResult,
    loading,
    error,

    // Actions
    fetchTypedConfig,
    updateConfig,
    fetchCapabilities,
    validate,
    toTyped,
    toRaw,
    fetchSchema,
    bulkValidate,
    clearSchemaCache,
    clearValidation,
    reset
  }
})
