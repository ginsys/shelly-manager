import api from './client'
import type { APIResponse } from './types'
import type { ConfigTemplate } from './templates'

export interface DeviceConfiguration {
  [key: string]: any
}

export interface DeviceTemplatesData {
  templates: ConfigTemplate[]
  template_ids: number[]
}

export async function getDeviceTemplatesNew(deviceId: number | string): Promise<DeviceTemplatesData> {
  const res = await api.get<APIResponse<DeviceTemplatesData>>(`/devices/${deviceId}/templates/new`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load device templates'
    throw new Error(msg)
  }
  return res.data.data
}

export interface SetDeviceTemplatesNewResult {
  templates: ConfigTemplate[]
  template_ids: number[]
  desired_config?: DeviceConfiguration
  sources?: Record<string, string>
}

export async function setDeviceTemplatesNew(
  deviceId: number | string,
  templateIds: number[],
): Promise<SetDeviceTemplatesNewResult> {
  const res = await api.put<APIResponse<SetDeviceTemplatesNewResult>>(`/devices/${deviceId}/templates/new`, {
    template_ids: templateIds,
  })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to set device templates'
    throw new Error(msg)
  }
  return res.data.data
}

export async function addDeviceTemplateNew(params: {
  deviceId: number | string
  templateId: number | string
  position?: number
}): Promise<ConfigTemplate[]> {
  const res = await api.post<APIResponse<{ templates: ConfigTemplate[] }>>(
    `/devices/${params.deviceId}/templates/new/${params.templateId}`,
    null,
    {
      params: {
        position: params.position,
      },
    },
  )

  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to add template to device'
    throw new Error(msg)
  }

  return res.data.data?.templates || []
}

export async function removeDeviceTemplateNew(
  deviceId: number | string,
  templateId: number | string,
): Promise<ConfigTemplate[]> {
  const res = await api.delete<APIResponse<{ templates: ConfigTemplate[] }>>(
    `/devices/${deviceId}/templates/new/${templateId}`,
  )

  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to remove template from device'
    throw new Error(msg)
  }

  return res.data.data?.templates || []
}

export interface GetDeviceOverridesNewResult {
  overrides: DeviceConfiguration | null
}

export async function getDeviceOverridesNew(deviceId: number | string): Promise<GetDeviceOverridesNewResult> {
  const res = await api.get<APIResponse<GetDeviceOverridesNewResult>>(`/devices/${deviceId}/overrides/new`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load device overrides'
    throw new Error(msg)
  }
  return res.data.data
}

export interface SetDeviceOverridesNewResult {
  overrides: DeviceConfiguration | null
  desired_config?: DeviceConfiguration
}

export async function setDeviceOverridesNew(
  deviceId: number | string,
  overrides: DeviceConfiguration,
): Promise<SetDeviceOverridesNewResult> {
  const res = await api.put<APIResponse<SetDeviceOverridesNewResult>>(`/devices/${deviceId}/overrides/new`, overrides)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to save device overrides'
    throw new Error(msg)
  }
  return res.data.data
}

export async function patchDeviceOverridesNew(
  deviceId: number | string,
  patch: DeviceConfiguration,
): Promise<DeviceConfiguration | null> {
  const res = await api.patch<APIResponse<{ overrides: DeviceConfiguration | null }>>(
    `/devices/${deviceId}/overrides/new`,
    patch,
  )
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to patch device overrides'
    throw new Error(msg)
  }
  return res.data.data?.overrides ?? null
}

export async function deleteDeviceOverridesNew(deviceId: number | string): Promise<void> {
  const res = await api.delete(`/devices/${deviceId}/overrides/new`)
  if (res.status !== 204) {
    const msg = (res.data as any)?.error?.message || 'Failed to clear device overrides'
    throw new Error(msg)
  }
}

export interface DesiredConfigData {
  config: DeviceConfiguration | null
  sources: Record<string, string>
}

export async function getDesiredConfigNew(deviceId: number | string): Promise<DesiredConfigData> {
  const res = await api.get<APIResponse<DesiredConfigData>>(`/devices/${deviceId}/desired-config`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load desired config'
    throw new Error(msg)
  }
  return res.data.data
}

export interface ConfigStatusData {
  device_id: number
  config_applied: boolean
  has_overrides: boolean
  template_count: number
  last_applied?: string
  pending_changes: boolean
}

export async function getConfigStatusNew(deviceId: number | string): Promise<ConfigStatusData> {
  const res = await api.get<APIResponse<ConfigStatusData>>(`/devices/${deviceId}/config/new/status`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load config status'
    throw new Error(msg)
  }
  return res.data.data
}

export interface ConfigApplyData {
  success: boolean
  applied_count: number
  failed_count: number
  requires_reboot: boolean
  failures?: string[]
}

export async function applyDeviceConfigNew(deviceId: number | string): Promise<ConfigApplyData> {
  const res = await api.post<APIResponse<ConfigApplyData>>(`/devices/${deviceId}/config/new/apply`, null)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to apply config'
    throw new Error(msg)
  }
  return res.data.data
}

export interface ConfigDifference {
  path: string
  expected: any
  actual: any
}

export interface ConfigVerifyData {
  match: boolean
  differences?: ConfigDifference[]
}

export async function verifyDeviceConfigNew(deviceId: number | string): Promise<ConfigVerifyData> {
  const res = await api.post<APIResponse<ConfigVerifyData>>(`/devices/${deviceId}/config/new/verify`, null)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to verify config'
    throw new Error(msg)
  }
  return res.data.data
}
