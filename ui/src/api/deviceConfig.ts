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

export interface ConfigImportStatus {
  status: 'pending' | 'in_progress' | 'completed' | 'failed'
  progress?: number
  message?: string
  completedAt?: string
}

// Get stored device configuration
export async function getDeviceConfig(id: number | string): Promise<DeviceConfig> {
  const res = await api.get<APIResponse<DeviceConfig>>(`/devices/${id}/config`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get device configuration'
    throw new Error(msg)
  }
  return res.data.data
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

// Get normalized live configuration from device
export async function getNormalizedCurrentConfig(id: number | string): Promise<DeviceConfig> {
  const res = await api.get<APIResponse<DeviceConfig>>(`/devices/${id}/config/current/normalized`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get normalized configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Get typed normalized configuration
export async function getTypedNormalizedConfig(id: number | string): Promise<DeviceConfig> {
  const res = await api.get<APIResponse<DeviceConfig>>(`/devices/${id}/config/typed/normalized`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get typed normalized configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Import configuration to device
export async function importDeviceConfig(id: number | string, config: DeviceConfig): Promise<void> {
  const res = await api.post<APIResponse<void>>(`/devices/${id}/config/import`, config)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to import device configuration'
    throw new Error(msg)
  }
}

// Get configuration import status
export async function getConfigImportStatus(id: number | string): Promise<ConfigImportStatus> {
  const res = await api.get<APIResponse<ConfigImportStatus>>(`/devices/${id}/config/status`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get import status'
    throw new Error(msg)
  }
  return res.data.data
}

// Export configuration from device
export async function exportDeviceConfig(id: number | string): Promise<DeviceConfig> {
  const res = await api.post<APIResponse<DeviceConfig>>(`/devices/${id}/config/export`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to export device configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Detect configuration drift
export async function detectConfigDrift(id: number | string): Promise<ConfigDrift> {
  const res = await api.get<APIResponse<ConfigDrift>>(`/devices/${id}/config/drift`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to detect configuration drift'
    throw new Error(msg)
  }
  return res.data.data
}

// Apply template to device configuration
export async function applyConfigTemplate(id: number | string, templateId: number | string): Promise<void> {
  const res = await api.post<APIResponse<void>>(`/devices/${id}/config/apply-template`, { templateId })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to apply configuration template'
    throw new Error(msg)
  }
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
