import api from './client'
import { DEVICE_CALL_TIMEOUT_MS } from './deviceCallTimeout'
import { errorEnvelopeMessage } from './errorBody'
import type { APIResponse } from './types'

// Configuration interfaces
export interface DeviceConfig {
  [key: string]: any
}

export interface ConfigDrift {
  hasDrift: boolean
  // The device has no desired configuration, so nothing was compared. hasDrift is false then, and
  // that is not a "no drift" result.
  noDesiredConfig?: boolean
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
  const res = await api.get<APIResponse<DeviceConfig>>(`/devices/${id}/config/current`, { timeout: DEVICE_CALL_TIMEOUT_MS })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get current device configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// The drift endpoint answers a 200 in one of two shapes: a full report
// ({differences: [{path, ...}], ...}) when the device is compared, or
// {device_id, drift: false, message} when the check returns no report. A device with no desired
// configuration is not a 200: the server answers 404 with the sentence below, because there is
// nothing to compare the device with, and that is not "no drift".
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

// configuration.ErrNoDesiredConfig, as the server writes it into the 404.
const NO_DESIRED_CONFIG_MESSAGE =
  'no desired configuration is set for this device, so there is nothing to compare it with'

// Detect configuration drift
export async function detectConfigDrift(id: number | string, signal?: AbortSignal): Promise<ConfigDrift> {
  let res
  try {
    res = await api.get<APIResponse<DriftResponse>>(`/devices/${id}/config/drift`, {
      signal,
      timeout: DEVICE_CALL_TIMEOUT_MS,
    })
  } catch (e: any) {
    if (e?.response?.status === 404 && (await errorEnvelopeMessage(e.response.data)) === NO_DESIRED_CONFIG_MESSAGE) {
      return { hasDrift: false, noDesiredConfig: true }
    }
    throw e
  }
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
