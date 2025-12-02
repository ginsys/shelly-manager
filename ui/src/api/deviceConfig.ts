import api from './client'
import type { APIResponse, Metadata } from './types'

export interface DeviceConfigPayload {
  [key: string]: any
}

export interface ConfigHistoryItem {
  id: string
  timestamp: string
  user?: string
  diff?: any
}

export interface DriftStatus {
  has_drift: boolean
  diff?: any
}

export interface ImportStatus {
  status: 'pending' | 'running' | 'completed' | 'failed'
  error?: string
}

export interface ListHistoryResult {
  items: ConfigHistoryItem[]
  meta?: Metadata
}

export async function getStoredConfig(deviceId: number | string): Promise<DeviceConfigPayload> {
  const res = await api.get<APIResponse<DeviceConfigPayload>>(`/devices/${deviceId}/config`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to get stored config')
  return res.data.data
}

export async function updateStoredConfig(deviceId: number | string, payload: DeviceConfigPayload): Promise<DeviceConfigPayload> {
  const res = await api.put<APIResponse<DeviceConfigPayload>>(`/devices/${deviceId}/config`, payload)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to update config')
  return res.data.data
}

export async function getLiveConfig(deviceId: number | string): Promise<DeviceConfigPayload> {
  const res = await api.get<APIResponse<DeviceConfigPayload>>(`/devices/${deviceId}/config/current`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to get live config')
  return res.data.data
}

export async function getLiveConfigNormalized(deviceId: number | string): Promise<DeviceConfigPayload> {
  const res = await api.get<APIResponse<DeviceConfigPayload>>(`/devices/${deviceId}/config/current/normalized`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to get normalized live config')
  return res.data.data
}

export async function getTypedNormalizedConfig(deviceId: number | string): Promise<DeviceConfigPayload> {
  const res = await api.get<APIResponse<DeviceConfigPayload>>(`/devices/${deviceId}/config/typed/normalized`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to get typed normalized config')
  return res.data.data
}

export async function importConfig(deviceId: number | string, payload: DeviceConfigPayload): Promise<{ accepted: boolean }> {
  const res = await api.post<APIResponse<{ accepted: boolean }>>(`/devices/${deviceId}/config/import`, payload)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Import failed')
  return res.data.data
}

export async function getImportStatus(deviceId: number | string): Promise<ImportStatus> {
  const res = await api.get<APIResponse<ImportStatus>>(`/devices/${deviceId}/config/status`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to get import status')
  return res.data.data
}

export async function exportConfig(deviceId: number | string, opts: { format?: 'json' | 'yaml' } = {}): Promise<{ export_id: string }> {
  const res = await api.post<APIResponse<{ export_id: string }>>(`/devices/${deviceId}/config/export`, opts)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Export failed')
  return res.data.data
}

export async function detectDrift(deviceId: number | string): Promise<DriftStatus> {
  const res = await api.get<APIResponse<DriftStatus>>(`/devices/${deviceId}/config/drift`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to detect drift')
  return res.data.data
}

export async function applyTemplate(deviceId: number | string, templateId: number | string, vars: Record<string, any> = {}): Promise<{ applied: boolean }> {
  const res = await api.post<APIResponse<{ applied: boolean }>>(`/devices/${deviceId}/config/apply-template`, { template_id: templateId, variables: vars })
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to apply template')
  return res.data.data
}

export async function getConfigHistory(deviceId: number | string, params: { page?: number; pageSize?: number } = {}): Promise<ListHistoryResult> {
  const { page = 1, pageSize = 20 } = params
  const res = await api.get<APIResponse<{ history: ConfigHistoryItem[] }>>(`/devices/${deviceId}/config/history`, { params: { page, page_size: pageSize } })
  if (!res.data.success) throw new Error(res.data.error?.message || 'Failed to load config history')
  return { items: res.data.data?.history || [], meta: res.data.meta }
}

