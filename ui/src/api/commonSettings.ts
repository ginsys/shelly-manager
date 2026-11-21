import api from './client'
import type { APIResponse } from './types'

// What the devices' stored imports have in common (POST /config/common-settings), in the typed shape a
// template holds. The server already left out every secret and every per-device value, and names the
// devices it could not compare.
export interface CommonSettings {
  compared: number[]
  skipped: { deviceId: number; deviceName?: string; reason: string }[]
  // The one model of every compared device; empty when they are not all one model.
  deviceType: string
  common: Record<string, any>
  // Per section, how many values were left out because the devices differ on them.
  differing: Record<string, number>
}

interface CommonSettingsAnswer {
  compared?: number[]
  skipped?: { device_id: number; device_name?: string; reason: string }[]
  device_type?: string
  common?: Record<string, any>
  differing?: Record<string, number>
}

export async function getCommonSettings(deviceIds: number[]): Promise<CommonSettings> {
  const res = await api.post<APIResponse<CommonSettingsAnswer>>('/config/common-settings', { device_ids: deviceIds })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to compare the devices')
  }
  const answer = res.data.data
  return {
    compared: answer.compared ?? [],
    skipped: (answer.skipped ?? []).map((s) => ({ deviceId: s.device_id, deviceName: s.device_name, reason: s.reason })),
    deviceType: answer.device_type ?? '',
    common: answer.common ?? {},
    differing: answer.differing ?? {},
  }
}
