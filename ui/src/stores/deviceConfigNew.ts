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

  // Counts refresh calls and resets. A refresh that finishes after a newer refresh or a reset
  // belongs to an earlier device and must not write its answer into the store.
  let refreshSeq = 0

  // Counts resets. An action that started for one device and finishes after a reset must hand its
  // answer back to its caller but must not write it into the store, which now holds another device.
  let deviceEpoch = 0

  async function refresh(deviceId: number | string) {
    const seq = ++refreshSeq
    loading.value = true
    error.value = null

    try {
      const [t, o, d, s] = await Promise.all([
        getDeviceTemplatesNew(deviceId),
        getDeviceOverridesNew(deviceId),
        getDesiredConfigNew(deviceId),
        getConfigStatusNew(deviceId),
      ])
      if (seq !== refreshSeq) return

      templates.value = t.templates
      templateIds.value = t.template_ids
      overrides.value = o.overrides
      desiredConfig.value = d.config
      sources.value = d.sources || {}
      status.value = s
    } catch (e: any) {
      // The error of a refresh that was replaced is not the error of the device on screen.
      if (seq !== refreshSeq) return
      error.value = e?.message || 'Failed to load new config state'
      throw e
    } finally {
      if (seq === refreshSeq) loading.value = false
    }
  }

  // The actions below share one shape. They remember the epoch when they start. If a reset ran
  // meanwhile, the store holds another device: the caller still gets the answer, or the error, but
  // the store keeps its own state.
  async function addTemplate(deviceId: number | string, templateId: number | string, position?: number) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const updated = await addDeviceTemplateNew({ deviceId, templateId, position })
      if (epoch !== deviceEpoch) return
      templates.value = updated
      templateIds.value = updated.map((t) => t.id)
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to add template'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function removeTemplate(deviceId: number | string, templateId: number | string) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const updated = await removeDeviceTemplateNew(deviceId, templateId)
      if (epoch !== deviceEpoch) return
      templates.value = updated
      templateIds.value = updated.map((t) => t.id)
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to remove template'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function setTemplates(deviceId: number | string, templateIdsNew: number[]) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const result = await setDeviceTemplatesNew(deviceId, templateIdsNew)
      if (epoch !== deviceEpoch) return
      templates.value = result.templates
      templateIds.value = result.template_ids
      if (result.desired_config !== undefined) desiredConfig.value = result.desired_config
      if (result.sources) sources.value = result.sources
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to set templates'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function saveOverrides(deviceId: number | string, overridesNew: DeviceConfiguration) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const result = await setDeviceOverridesNew(deviceId, overridesNew)
      if (epoch !== deviceEpoch) return
      overrides.value = result.overrides
      if (result.desired_config !== undefined) desiredConfig.value = result.desired_config
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to save overrides'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function patchOverrides(deviceId: number | string, patch: DeviceConfiguration) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const result = await patchDeviceOverridesNew(deviceId, patch)
      if (epoch !== deviceEpoch) return
      overrides.value = result
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to patch overrides'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function clearOverrides(deviceId: number | string) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      await deleteDeviceOverridesNew(deviceId)
      if (epoch !== deviceEpoch) return
      overrides.value = null
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to clear overrides'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function fetchDesired(deviceId: number | string): Promise<DesiredConfigData> {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const result = await getDesiredConfigNew(deviceId)
      if (epoch !== deviceEpoch) return result
      desiredConfig.value = result.config
      sources.value = result.sources || {}
      return result
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to load desired config'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function fetchStatus(deviceId: number | string) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const result = await getConfigStatusNew(deviceId)
      if (epoch === deviceEpoch) status.value = result
      return result
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to load status'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function apply(deviceId: number | string) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      // The caller gets the answer of this apply, whatever happens to the store while the status loads.
      const result = await applyDeviceConfigNew(deviceId)
      if (epoch === deviceEpoch) {
        lastApply.value = result
        await fetchStatus(deviceId)
      }
      return result
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to apply config'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  async function verify(deviceId: number | string) {
    const epoch = deviceEpoch
    loading.value = true
    error.value = null

    try {
      const result = await verifyDeviceConfigNew(deviceId)
      if (epoch === deviceEpoch) lastVerify.value = result
      return result
    } catch (e: any) {
      if (epoch === deviceEpoch) error.value = e?.message || 'Failed to verify config'
      throw e
    } finally {
      if (epoch === deviceEpoch) loading.value = false
    }
  }

  function reset() {
    refreshSeq++ // a refresh still in flight now belongs to an earlier device
    deviceEpoch++
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
