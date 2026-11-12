import api from './client'
import type { APIResponse } from './types'

// Configuration interfaces
export interface DeviceConfig {
  [key: string]: any
}

export interface ConfigDrift {
  hasDrift: boolean
  driftFields?: string[]
  storedConfig?: DeviceConfig
  liveConfig?: DeviceConfig
}

export interface ConfigHistoryEntry {
  id: number
  deviceId: number
  timestamp: string
  config: DeviceConfig
  source: string
  user?: string
}

// Update stored device configuration
export async function updateDeviceConfig(id: number | string, config: DeviceConfig): Promise<DeviceConfig> {
  const res = await api.put<APIResponse<DeviceConfig>>(`/devices/${id}/config`, config)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update device configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Get current live configuration from device
export async function getCurrentDeviceConfig(id: number | string): Promise<DeviceConfig> {
  const res = await api.get<APIResponse<DeviceConfig>>(`/devices/${id}/config/current`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get current device configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// The drift endpoint answers in one of two shapes: a full report
// ({differences: [{path, ...}], ...}) when the device is compared, or
// {device_id, drift: false, message} when no stored configuration exists.
interface DriftDifference {
  path?: string
}
interface DriftResponse {
  differences?: DriftDifference[] | null
  drift?: boolean
  message?: string
}

// Only the two shapes above mean "no drift". Any other payload (an empty object,
// drift: true without a differences list, a differences value that is not a list)
// is not a comparison result, so it must fail instead of showing "No Drift".
function mapDriftResponse(data: DriftResponse): ConfigDrift {
  let differences: DriftDifference[]
  if (Array.isArray(data.differences)) {
    differences = data.differences
  } else if (data.differences === null) {
    differences = []
  } else if (data.differences === undefined && data.drift === false) {
    differences = []
  } else {
    throw new Error('Unexpected configuration drift response')
  }
  if (differences.length === 0) {
    return { hasDrift: false }
  }
  return {
    hasDrift: true,
    driftFields: differences.map((d) => d.path).filter((p): p is string => !!p),
  }
}

// Detect configuration drift
export async function detectConfigDrift(id: number | string, signal?: AbortSignal): Promise<ConfigDrift> {
  const res = await api.get<APIResponse<DriftResponse>>(`/devices/${id}/config/drift`, { signal })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to detect configuration drift'
    throw new Error(msg)
  }
  return mapDriftResponse(res.data.data)
}

// Get configuration change history
export async function getConfigHistory(id: number | string): Promise<ConfigHistoryEntry[]> {
  const res = await api.get<APIResponse<{ history: ConfigHistoryEntry[] }>>(`/devices/${id}/config/history`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get configuration history'
    throw new Error(msg)
  }
  return res.data.data.history || []
}
