import type { AxiosResponse } from 'axios'
import api from './client'
import { errorEnvelopeDetails, errorEnvelopeMessage } from './errorBody'
import type { APIResponse, Metadata } from './types'

export interface ExportHistoryItem {
  id: number
  export_id: string
  plugin_name: string
  format: string
  name?: string
  description?: string
  file_path?: string
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
  success: boolean
  export_id: string
  plugin_name: string
  format: string
  output_path?: string
  record_count?: number
  file_size?: number
  checksum?: string
  duration?: number // nanoseconds (a Go time.Duration)
  errors?: string[]
  warnings?: string[]
  created_at?: string
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
  config: Record<string, unknown>
  filters: ExportFilters
  output: ExportOutput
  options: ExportOptions
}

export interface ExportFilters {
  device_ids?: number[]
  device_types?: string[]
  device_status?: string[]
  last_seen_after?: string
  has_config?: boolean
  template_ids?: number[]
  tags?: string[]
}

export interface ExportOutput {
  type: 'response'
  destination?: string
  headers?: Record<string, string>
}

export interface ExportOptions {
  dry_run: boolean
  include_history: boolean
  validate_only: boolean
  compact_output: boolean
  include_metadata: boolean
}

export interface ExportPreview {
  success: boolean
  sample_data: string
  record_count: number
  estimated_size: number
  warnings?: string[]
}

export interface ExportPreviewResponse {
  preview: ExportPreview
  summary: {
    record_count: number
    estimated_size: number
  }
}

export async function previewExport(req: ExportRequest): Promise<ExportPreviewResponse> {
  const res = await api.post<APIResponse<ExportPreviewResponse>>('/export/preview', req)
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

// Content export: JSON
export async function createJSONExport(config: Record<string, any> = {}, filters: Record<string, any> = {}, options: Record<string, any> = {}) {
  const res = await api.post<APIResponse<any>>('/export/json', { config, filters, options })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create JSON export')
  }
  return res.data.data
}

// Content export: YAML (single file)
export async function createYAMLExport(config: Record<string, any> = {}, filters: Record<string, any> = {}, options: Record<string, any> = {}) {
  const res = await api.post<APIResponse<any>>('/export/yaml', { config, filters, options })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create YAML export')
  }
  return res.data.data
}

// Content export: SMA
export interface SMAExportCreationOptions {
  compression_level: number
  include_discovered: boolean
  exclude_sensitive: boolean
}

export function buildSMAExportConfig(
  outputPath: string,
  options: SMAExportCreationOptions,
): Record<string, unknown> {
  return {
    output_path: outputPath,
    compression_level: options.compression_level,
    include_discovered: options.include_discovered,
    exclude_sensitive: options.exclude_sensitive,
  }
}

export async function createSMAExport(config: Record<string, any> = {}, filters: Record<string, any> = {}, options: Record<string, any> = {}) {
  const res = await api.post<APIResponse<any>>('/export/sma', { config, filters, options })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create SMA export')
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

// A download with the file name the server sends. A blob request delivers the server's error envelope
// as a Blob, so a failure shows the envelope's text and not the status text: its details when they are a
// text (a validation refusal says why there, and its message only says it failed), else its message.
// The caller makes the request, so each route stays a literal the route contract test can read.
export async function downloadBlobWithName(
  request: () => Promise<AxiosResponse>,
): Promise<{ blob: Blob; filename?: string }> {
  let res
  try {
    res = await request()
  } catch (e: any) {
    const message = (await errorEnvelopeDetails(e?.response?.data)) ?? (await errorEnvelopeMessage(e?.response?.data))
    if (message) throw new Error(message)
    throw e
  }
  const cd = (res.headers?.['content-disposition'] || res.headers?.['Content-Disposition']) as string | undefined
  let filename: string | undefined
  if (cd) {
    const match = cd.match(/filename\*=UTF-8''([^;]+)|filename="?([^";]+)"?/i)
    filename = decodeURIComponent((match?.[1] || match?.[2] || '').trim()) || undefined
  }
  return { blob: res.data, filename }
}

export function downloadBackupWithName(id: string): Promise<{ blob: Blob; filename?: string }> {
  return downloadBlobWithName(() => api.get(`/export/backup/${id}/download`, { responseType: 'blob' }))
}

// Generic export download (content or backup)
export async function downloadExport(id: string): Promise<Blob> {
  const res = await api.get(`/export/${id}/download`, { responseType: 'blob' })
  return res.data
}

export function downloadExportWithName(id: string): Promise<{ blob: Blob; filename?: string }> {
  return downloadBlobWithName(() => api.get(`/export/${id}/download`, { responseType: 'blob' }))
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
  const res = await api.delete(`/export/backup/${id}`)
  // Accept 204 No Content or any 2xx
  if (res.status === 204) return
  const data = res.data as APIResponse<void> | undefined
  if (data && data.success === false) {
    throw new Error(data.error?.message || 'Failed to delete backup')
  }
  // If no body, assume success on 2xx
  if (res.status < 200 || res.status >= 300) {
    throw new Error('Failed to delete backup')
  }
}

// Restore interfaces and methods

// A backup is restored in two steps: the server stages a checked copy, and applies it when it next
// starts. Only the preview is a quick look at the stored file; staging runs the real checks.

export const RESTORE_SQLITE_ONLY = 'Restore is only available for SQLite'

export interface RestorePending {
  backup_id: string
  staged_at: string
  sha256: string
}

export interface RestoreResult {
  status: 'applied' | 'failed'
  backup_id?: string
  at: string
  error?: string
}

export interface RestoreStatus {
  supported: boolean
  pending: RestorePending | null
  lastResult: RestoreResult | null
}

export interface RestorePreview {
  backup_id: string
  name: string
  valid: boolean
  file_size: number
  record_count: number
  created_at: string
  errors?: string[]
  warnings?: string[]
}

export async function previewRestore(backupId: string): Promise<RestorePreview> {
  const res = await api.post<APIResponse<RestorePreview>>('/import/restore-preview', {
    backup_id: backupId,
  })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to check the backup')
  }
  return res.data.data
}

// The server's reason for a refused restore call: the SQLite-only sentence for a 501, else the
// message of its error envelope.
async function restoreFailure(e: any, fallback: string): Promise<Error> {
  if (e?.response?.status === 501) return new Error(RESTORE_SQLITE_ONLY)
  return new Error((await errorEnvelopeMessage(e?.response?.data)) || e?.message || fallback)
}

export async function stageRestore(backupId: string): Promise<void> {
  try {
    await api.post('/import/restore', { backup_id: backupId })
  } catch (e: any) {
    // `answered` is false when no response arrived: the server may have staged the restore.
    throw Object.assign(await restoreFailure(e, 'Failed to stage the restore'), { answered: !!e?.response })
  }
}

// A 501 is the answer of a database that is not SQLite, not a failure.
export async function getRestoreStatus(): Promise<RestoreStatus> {
  try {
    const res = await api.get<APIResponse<{ pending?: RestorePending | null; last_result?: RestoreResult | null }>>(
      '/import/restore',
    )
    return { supported: true, pending: res.data.data?.pending ?? null, lastResult: res.data.data?.last_result ?? null }
  } catch (e: any) {
    if (e?.response?.status === 501) return { supported: false, pending: null, lastResult: null }
    throw await restoreFailure(e, 'Failed to read the restore status')
  }
}

// A 404 means nothing is staged any more, which is what the caller wanted.
export async function cancelRestore(): Promise<void> {
  try {
    await api.delete('/import/restore')
  } catch (e: any) {
    if (e?.response?.status === 404) return
    throw await restoreFailure(e, 'Failed to cancel the restore')
  }
}

// GitOps exports. The server writes a directory of YAML files. Its history route lists them
// (listExportHistory with plugin 'gitops'), the generic result route reads one (getExportResult)
// and the generic preview route previews one (previewExport). Only create and delete have
// GitOps routes of their own.

export interface GitOpsExportOptions {
  name: string
  description?: string
  group_by: 'location' | 'type' // 'custom' needs a group_mapping the page has no input for
  include_common: boolean
  include_templates: boolean
  device_ids?: number[] // empty or missing: every device
}

function gitopsConfig(o: GitOpsExportOptions) {
  return {
    config: {
      name: o.name,
      description: o.description,
      group_by: o.group_by,
      include_common: o.include_common,
      include_templates: o.include_templates,
    },
    filters: { device_ids: o.device_ids?.length ? o.device_ids : undefined },
  }
}

export async function createGitOpsExport(o: GitOpsExportOptions): Promise<{ export_id: string }> {
  const res = await api.post<APIResponse<{ export_id: string }>>('/export/gitops', gitopsConfig(o))
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create GitOps export')
  }
  return res.data.data
}

// The preview's sample is the file tree as UTF-8 text, base64 encoded by the server.
export function decodePreviewSample(sample: string): string {
  return new TextDecoder().decode(Uint8Array.from(atob(sample), (c) => c.charCodeAt(0)))
}

export function previewGitOpsExport(o: GitOpsExportOptions): Promise<ExportPreviewResponse> {
  return previewExport({
    plugin_name: 'gitops',
    format: 'yaml',
    ...gitopsConfig(o),
    output: { type: 'response' },
    options: { dry_run: true, include_history: false, validate_only: false, compact_output: false, include_metadata: false },
  })
}

// Removes the history record only. The exported files stay where the export wrote them.
export async function deleteGitOpsExport(id: string): Promise<void> {
  await api.delete(`/export/gitops/${id}`) // the client throws on a non-2xx answer
}

// The history response has no total, but its page count at one item per page is the item count
// (the Go test TestHistoryPageCountWithPageSizeOneIsTheRowCount pins this).
export async function countGitOpsExports(success?: boolean): Promise<number> {
  const { meta } = await listExportHistory({ plugin: 'gitops', pageSize: 1, success })
  return meta?.pagination?.total_pages ?? 0
}

