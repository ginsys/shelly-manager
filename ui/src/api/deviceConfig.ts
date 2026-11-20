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

// A stored configuration that the server could not read as settings arrives as text (the mask).
export type StoredConfigBody = DeviceConfig | string | null

// The configuration an import stored for a device (GET /devices/{id}/config), secrets masked.
export interface StoredDeviceConfig {
  id: number
  device_id: number
  template_id: number | null
  config: StoredConfigBody
  last_synced: string | null
  sync_status: string
  created_at: string
  updated_at: string
}

// One change of the stored configuration. source is the server's action ("manual", "updated", ...)
// and user is who made it; config is the configuration after the change.
export interface ConfigHistoryEntry {
  id: number
  deviceId: number
  timestamp: string
  config: StoredConfigBody
  oldConfig: StoredConfigBody
  changes: unknown
  source: string
  user?: string
}

// Update stored device configuration. No page calls it today: the history page's Restore was its only
// caller and is gone. It stays for a real restore, ginsys/shelly-manager#775.
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

// Get the configuration an import stored for a device. A device that was never imported answers 404,
// which is "nothing stored" and gives null; any other failure is thrown.
export async function getStoredDeviceConfig(id: number | string): Promise<StoredDeviceConfig | null> {
  let res
  try {
    res = await api.get<APIResponse<StoredDeviceConfig>>(`/devices/${id}/config`)
  } catch (e: any) {
    if (e?.response?.status === 404) return null
    throw e
  }
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get stored device configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Get the stored import in its typed form (GET /devices/{id}/config/typed): the sections a template has,
// secrets masked. The server answers 500 when nothing is stored, so every failure is thrown and none is
// read as "nothing stored".
export async function getTypedDeviceConfig(id: number | string): Promise<Record<string, any>> {
  const res = await api.get<APIResponse<{ configuration: Record<string, any> }>>(`/devices/${id}/config/typed`)
  if (!res.data.success || !res.data.data?.configuration) {
    const msg = res.data.error?.message || 'Failed to get the typed device configuration'
    throw new Error(msg)
  }
  return res.data.data.configuration
}

// A row of the history answer (internal/configuration ConfigHistory).
interface ConfigHistoryRow {
  id: number
  device_id: number
  action: string
  old_config: StoredConfigBody
  new_config: StoredConfigBody
  changes: unknown
  changed_by?: string
  created_at: string
}

// Get configuration change history. The answer is a list of stored rows, newest first.
export async function getConfigHistory(id: number | string): Promise<ConfigHistoryEntry[]> {
  const res = await api.get<APIResponse<ConfigHistoryRow[]>>(`/devices/${id}/config/history`)
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to get configuration history')
  }
  const rows = res.data.data
  if (!Array.isArray(rows)) {
    throw new Error('Unexpected configuration history response')
  }
  return rows.map((row) => ({
    id: row.id,
    deviceId: row.device_id,
    timestamp: row.created_at,
    source: row.action,
    user: row.changed_by,
    config: row.new_config,
    oldConfig: row.old_config,
    changes: row.changes,
  }))
}
