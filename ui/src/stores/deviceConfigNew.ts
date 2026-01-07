import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  addDeviceTemplateNew,
  applyDeviceConfigNew,
  deleteDeviceOverridesNew,
  getConfigStatusNew,
  getDesiredConfigNew,
  getDeviceOverridesNew,
  getDeviceTemplatesNew,
  patchDeviceOverridesNew,
  removeDeviceTemplateNew,
  setDeviceOverridesNew,
  setDeviceTemplatesNew,
  type ConfigApplyData,
  type ConfigStatusData,
  type DesiredConfigData,
  type DeviceConfiguration,
  type DeviceTemplatesData,
  type ConfigVerifyData,
  verifyDeviceConfigNew,
} from '@/api/configNew'
import type { ConfigTemplate } from '@/api/templates'

export const useDeviceConfigNewStore = defineStore('deviceConfigNew', () => {
  const templates = ref<ConfigTemplate[]>([])
  const templateIds = ref<number[]>([])
  const overrides = ref<DeviceConfiguration | null>(null)
  const desiredConfig = ref<DeviceConfiguration | null>(null)
  const sources = ref<Record<string, string>>({})
  const status = ref<ConfigStatusData | null>(null)
  const lastApply = ref<ConfigApplyData | null>(null)
  const lastVerify = ref<ConfigVerifyData | null>(null)

  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh(deviceId: number | string) {
    loading.value = true
    error.value = null

    try {
      const [t, o, d, s] = await Promise.all([
        getDeviceTemplatesNew(deviceId),
        getDeviceOverridesNew(deviceId),
        getDesiredConfigNew(deviceId),
        getConfigStatusNew(deviceId),
      ])

      templates.value = t.templates
      templateIds.value = t.template_ids
      overrides.value = o.overrides
      desiredConfig.value = d.config
      sources.value = d.sources || {}
      status.value = s
    } catch (e: any) {
      error.value = e?.message || 'Failed to load new config state'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function addTemplate(deviceId: number | string, templateId: number | string, position?: number) {
    loading.value = true
    error.value = null

    try {
      const updated = await addDeviceTemplateNew({ deviceId, templateId, position })
      templates.value = updated
      templateIds.value = updated.map((t) => t.id)
    } catch (e: any) {
      error.value = e?.message || 'Failed to add template'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function removeTemplate(deviceId: number | string, templateId: number | string) {
    loading.value = true
    error.value = null

    try {
      const updated = await removeDeviceTemplateNew(deviceId, templateId)
      templates.value = updated
      templateIds.value = updated.map((t) => t.id)
    } catch (e: any) {
      error.value = e?.message || 'Failed to remove template'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function setTemplates(deviceId: number | string, templateIdsNew: number[]) {
    loading.value = true
    error.value = null

    try {
      const result = await setDeviceTemplatesNew(deviceId, templateIdsNew)
      templates.value = result.templates
      templateIds.value = result.template_ids
      if (result.desired_config !== undefined) desiredConfig.value = result.desired_config
      if (result.sources) sources.value = result.sources
    } catch (e: any) {
      error.value = e?.message || 'Failed to set templates'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function saveOverrides(deviceId: number | string, overridesNew: DeviceConfiguration) {
    loading.value = true
    error.value = null

    try {
      const result = await setDeviceOverridesNew(deviceId, overridesNew)
      overrides.value = result.overrides
      if (result.desired_config !== undefined) desiredConfig.value = result.desired_config
    } catch (e: any) {
      error.value = e?.message || 'Failed to save overrides'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function patchOverrides(deviceId: number | string, patch: DeviceConfiguration) {
    loading.value = true
    error.value = null

    try {
      overrides.value = await patchDeviceOverridesNew(deviceId, patch)
    } catch (e: any) {
      error.value = e?.message || 'Failed to patch overrides'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function clearOverrides(deviceId: number | string) {
    loading.value = true
    error.value = null

    try {
      await deleteDeviceOverridesNew(deviceId)
      overrides.value = null
    } catch (e: any) {
      error.value = e?.message || 'Failed to clear overrides'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchDesired(deviceId: number | string): Promise<DesiredConfigData> {
    loading.value = true
    error.value = null

    try {
      const result = await getDesiredConfigNew(deviceId)
      desiredConfig.value = result.config
      sources.value = result.sources || {}
      return result
    } catch (e: any) {
      error.value = e?.message || 'Failed to load desired config'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchStatus(deviceId: number | string) {
    loading.value = true
    error.value = null

    try {
      status.value = await getConfigStatusNew(deviceId)
      return status.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to load status'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function apply(deviceId: number | string) {
    loading.value = true
    error.value = null

    try {
      lastApply.value = await applyDeviceConfigNew(deviceId)
      await fetchStatus(deviceId)
      return lastApply.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to apply config'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function verify(deviceId: number | string) {
    loading.value = true
    error.value = null

    try {
      lastVerify.value = await verifyDeviceConfigNew(deviceId)
      return lastVerify.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to verify config'
      throw e
    } finally {
      loading.value = false
    }
  }

  function reset() {
    templates.value = []
    templateIds.value = []
    overrides.value = null
    desiredConfig.value = null
    sources.value = {}
    status.value = null
    lastApply.value = null
    lastVerify.value = null
    loading.value = false
    error.value = null
  }

  return {
    templates,
    templateIds,
    overrides,
    desiredConfig,
    sources,
    status,
    lastApply,
    lastVerify,
    loading,
    error,

    refresh,
    addTemplate,
    removeTemplate,
    setTemplates,
    saveOverrides,
    patchOverrides,
    clearOverrides,
    fetchDesired,
    fetchStatus,
    apply,
    verify,
    reset,
  }
})
