import api from './client'
import type { APIResponse, Metadata } from './types'

export interface ExportHistoryItem {
  id: number
  export_id: string
  plugin_name: string
  format: string
  requested_by?: string
  success: boolean
  record_count?: number
  file_size?: number
  duration_ms?: number
  error_message?: string
  created_at: string
}

export interface ListExportHistoryParams {
  page?: number
  pageSize?: number
  plugin?: string
  success?: boolean
}

export interface ListExportHistoryResult {
  items: ExportHistoryItem[]
  meta?: Metadata
}

export async function listExportHistory(params: ListExportHistoryParams = {}): Promise<ListExportHistoryResult> {
  const { page = 1, pageSize = 20, plugin, success } = params
  const res = await api.get<APIResponse<{ history: ExportHistoryItem[] }>>('/export/history', {
    params: { page, page_size: pageSize, plugin, success },
  })
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load export history')
  }
  return { items: res.data.data?.history || [], meta: res.data.meta }
}

export interface ExportStatistics {
  total: number
  success: number
  failure: number
  by_plugin: Record<string, number>
}

export async function getExportStatistics(): Promise<ExportStatistics> {
  const res = await api.get<APIResponse<ExportStatistics>>('/export/statistics')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load export statistics')
  }
  return res.data.data
}

export interface ExportResult {
  export_id: string
  plugin_name: string
  format: string
  output_path?: string
  record_count?: number
  file_size?: number
  checksum?: string
  duration?: string
  warnings?: string[]
}

export async function getExportResult(id: string): Promise<ExportResult> {
  const res = await api.get<APIResponse<ExportResult>>(`/export/${id}`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load export result')
  }
  return res.data.data
}

export interface ExportRequest {
  plugin_name: string
  format: string
  config?: Record<string, any>
  filters?: Record<string, any>
  options?: Record<string, any>
}

export interface ExportPreview {
  success: boolean
  record_count?: number
  estimated_size?: number
  warnings?: string[]
}

export async function previewExport(req: ExportRequest): Promise<{ preview: ExportPreview; summary: any }> {
  const res = await api.post<APIResponse<{ preview: ExportPreview; summary: any }>>('/export/preview', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Preview failed')
  }
  return res.data.data
}

// Backup-specific interfaces and methods

export interface BackupRequest {
  name: string
  description?: string
  devices?: number[] // Selected device IDs, empty/null for all
  format: string // 'json', 'sma', etc.
  include_settings?: boolean
  include_schedules?: boolean
  include_metrics?: boolean
  encrypt?: boolean
  encryption_password?: string
}

export interface BackupItem {
  id: number
  backup_id: string
  name: string
  description?: string
  format: string
  device_count: number
  file_size?: number
  checksum?: string
  encrypted: boolean
  success: boolean
  error_message?: string
  created_at: string
  created_by?: string
}

export interface BackupResult {
  backup_id: string
  name: string
  format: string
  device_count: number
  file_size?: number
  checksum?: string
  file_path?: string
  encrypted: boolean
  warnings?: string[]
  duration?: string
}

export interface ListBackupsParams {
  page?: number
  pageSize?: number
  success?: boolean
  format?: string
}

export interface ListBackupsResult {
  items: BackupItem[]
  meta?: Metadata
}

export interface BackupStatistics {
  total: number
  success: number
  failure: number
  total_size: number
  by_format: Record<string, number>
  last_backup?: string
}

export async function createBackup(req: BackupRequest): Promise<{ backup_id: string }> {
  const res = await api.post<APIResponse<{ backup_id: string }>>('/export/backup', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create backup')
  }
  return res.data.data
}

export async function getBackupResult(id: string): Promise<BackupResult> {
  const res = await api.get<APIResponse<BackupResult>>(`/export/backup/${id}`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to get backup result')
  }
  return res.data.data
}

export async function downloadBackup(id: string): Promise<Blob> {
  const res = await api.get(`/export/backup/${id}/download`, {
    responseType: 'blob'
  })
  return res.data
}

export async function listBackups(params: ListBackupsParams = {}): Promise<ListBackupsResult> {
  const { page = 1, pageSize = 20, success, format } = params
  const res = await api.get<APIResponse<{ backups: BackupItem[] }>>('/export/backups', {
    params: { page, page_size: pageSize, success, format },
  })
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load backups')
  }
  return { items: res.data.data?.backups || [], meta: res.data.meta }
}

export async function getBackupStatistics(): Promise<BackupStatistics> {
  const res = await api.get<APIResponse<BackupStatistics>>('/export/backup-statistics')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load backup statistics')
  }
  return res.data.data
}

export async function deleteBackup(id: string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/export/backup/${id}`)
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to delete backup')
  }
}

// Restore interfaces and methods

export interface RestoreRequest {
  backup_id: string
  devices?: number[] // Target devices, empty/null for all
  include_settings?: boolean
  include_schedules?: boolean
  include_metrics?: boolean
  dry_run?: boolean
  force?: boolean
}

export interface RestorePreview {
  device_count: number
  settings_count: number
  schedules_count: number
  metrics_count: number
  conflicts: string[]
  warnings: string[]
}

export interface RestoreResult {
  restore_id: string
  backup_id: string
  success: boolean
  device_count: number
  applied_settings: number
  applied_schedules: number
  applied_metrics: number
  warnings?: string[]
  errors?: string[]
  duration?: string
}

export async function previewRestore(req: RestoreRequest): Promise<RestorePreview> {
  const res = await api.post<APIResponse<RestorePreview>>('/import/restore-preview', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to preview restore')
  }
  return res.data.data
}

export async function executeRestore(req: RestoreRequest): Promise<{ restore_id: string }> {
  const res = await api.post<APIResponse<{ restore_id: string }>>('/import/restore', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to execute restore')
  }
  return res.data.data
}

export async function getRestoreResult(id: string): Promise<RestoreResult> {
  const res = await api.get<APIResponse<RestoreResult>>(`/import/restore/${id}`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to get restore result')
  }
  return res.data.data
}
