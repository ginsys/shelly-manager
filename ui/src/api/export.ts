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

// GitOps-specific interfaces and methods

export interface GitOpsExportRequest {
  name: string
  description?: string
  format: 'terraform' | 'ansible' | 'kubernetes' | 'docker-compose' | 'yaml'
  devices?: number[] // Selected device IDs, empty/null for all
  repository_structure: 'monorepo' | 'hierarchical' | 'per-device' | 'flat'
  template_options?: GitOpsTemplateOptions
  git_config?: GitOpsGitConfig
  variable_substitution?: Record<string, string>
  include_secrets?: boolean
  generate_readme?: boolean
}

export interface GitOpsTemplateOptions {
  terraform?: {
    provider_version?: string
    module_structure?: 'single' | 'per-device' | 'per-type'
    include_data_sources?: boolean
    variable_files?: boolean
  }
  ansible?: {
    playbook_structure?: 'single' | 'per-device' | 'roles'
    inventory_format?: 'ini' | 'yaml'
    include_vault?: boolean
    use_collections?: boolean
  }
  kubernetes?: {
    api_version?: string
    namespace?: string
    use_kustomize?: boolean
    include_rbac?: boolean
    config_map_structure?: 'single' | 'per-device'
  }
  docker_compose?: {
    version?: string
    network_mode?: 'bridge' | 'host' | 'custom'
    include_volumes?: boolean
    use_profiles?: boolean
  }
}

export interface GitOpsGitConfig {
  repository_url?: string
  branch?: string
  commit_message_template?: string
  author_name?: string
  author_email?: string
  use_webhooks?: boolean
  webhook_secret?: string
}

export interface GitOpsExportResult {
  export_id: string
  name: string
  format: string
  repository_structure: string
  device_count: number
  file_count: number
  total_size: number
  files: GitOpsExportFile[]
  git_integration?: GitOpsIntegrationStatus
  warnings?: string[]
  duration?: string
}

export interface GitOpsExportFile {
  path: string
  name: string
  size: number
  type: 'config' | 'template' | 'variable' | 'readme' | 'script'
  description?: string
}

export interface GitOpsIntegrationStatus {
  repository_connected: boolean
  branch_exists: boolean
  last_commit?: string
  webhook_configured: boolean
  ci_status?: 'passing' | 'failing' | 'unknown'
}

export interface GitOpsExportItem {
  id: number
  export_id: string
  name: string
  description?: string
  format: string
  repository_structure: string
  device_count: number
  file_count: number
  total_size: number
  success: boolean
  error_message?: string
  created_at: string
  created_by?: string
}

export interface ListGitOpsExportsParams {
  page?: number
  pageSize?: number
  format?: string
  success?: boolean
}

export interface ListGitOpsExportsResult {
  items: GitOpsExportItem[]
  meta?: Metadata
}

export interface GitOpsExportStatistics {
  total: number
  success: number
  failure: number
  by_format: Record<string, number>
  by_structure: Record<string, number>
  total_files: number
  total_size: number
  last_export?: string
}

export async function createGitOpsExport(req: GitOpsExportRequest): Promise<{ export_id: string }> {
  const res = await api.post<APIResponse<{ export_id: string }>>('/export/gitops', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create GitOps export')
  }
  return res.data.data
}

export async function getGitOpsExportResult(id: string): Promise<GitOpsExportResult> {
  const res = await api.get<APIResponse<GitOpsExportResult>>(`/export/gitops/${id}`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to get GitOps export result')
  }
  return res.data.data
}

export async function downloadGitOpsExport(id: string): Promise<Blob> {
  const res = await api.get(`/export/gitops/${id}/download`, {
    responseType: 'blob'
  })
  return res.data
}

export async function listGitOpsExports(params: ListGitOpsExportsParams = {}): Promise<ListGitOpsExportsResult> {
  const { page = 1, pageSize = 20, format, success } = params
  const res = await api.get<APIResponse<{ exports: GitOpsExportItem[] }>>('/export/gitops', {
    params: { page, page_size: pageSize, format, success },
  })
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load GitOps exports')
  }
  return { items: res.data.data?.exports || [], meta: res.data.meta }
}

export async function getGitOpsExportStatistics(): Promise<GitOpsExportStatistics> {
  const res = await api.get<APIResponse<GitOpsExportStatistics>>('/export/gitops-statistics')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load GitOps export statistics')
  }
  return res.data.data
}

export async function deleteGitOpsExport(id: string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/export/gitops/${id}`)
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to delete GitOps export')
  }
}

export interface GitOpsExportPreview {
  success: boolean
  file_count?: number
  estimated_size?: number
  structure_preview: string[]
  template_validation: GitOpsTemplateValidation
  warnings?: string[]
}

export interface GitOpsTemplateValidation {
  valid: boolean
  terraform?: { syntax_valid: boolean; provider_compatible: boolean; warnings?: string[] }
  ansible?: { syntax_valid: boolean; collection_available: boolean; warnings?: string[] }
  kubernetes?: { api_valid: boolean; rbac_valid: boolean; warnings?: string[] }
  docker_compose?: { syntax_valid: boolean; service_valid: boolean; warnings?: string[] }
}

export async function previewGitOpsExport(req: GitOpsExportRequest): Promise<{ preview: GitOpsExportPreview; summary: any }> {
  const res = await api.post<APIResponse<{ preview: GitOpsExportPreview; summary: any }>>('/export/gitops-preview', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'GitOps preview failed')
  }
  return res.data.data
}
