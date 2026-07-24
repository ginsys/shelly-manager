import api from './client'
import type { APIResponse, Metadata } from './types'

export const BROWSER_DATA_IMPORT_PLUGINS = ['sma'] as const

export interface ImportHistoryItem {
  id: number
  import_id: string
  plugin_name: string
  format: string
  requested_by?: string
  success: boolean
  records_imported?: number
  records_skipped?: number
  duration_ms?: number
  error_message?: string
  created_at: string
}

export interface ListImportHistoryParams {
  page?: number
  pageSize?: number
  plugin?: string
  success?: boolean
}

export interface ListImportHistoryResult {
  items: ImportHistoryItem[]
  meta?: Metadata
}

export async function listImportHistory(params: ListImportHistoryParams = {}): Promise<ListImportHistoryResult> {
  const { page = 1, pageSize = 20, plugin, success } = params
  const res = await api.get<APIResponse<{ history: ImportHistoryItem[] }>>('/import/history', {
    params: { page, page_size: pageSize, plugin, success },
  })
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load import history')
  }
  return { items: res.data.data?.history || [], meta: res.data.meta }
}

export interface ImportStatistics {
  total: number
  success: number
  failure: number
  by_plugin: Record<string, number>
}

export async function getImportStatistics(): Promise<ImportStatistics> {
  const res = await api.get<APIResponse<ImportStatistics>>('/import/statistics')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load import statistics')
  }
  return res.data.data
}

export interface ImportResult {
  success: boolean
  import_id: string
  plugin_name: string
  format: string
  records_imported?: number
  records_skipped?: number
  duration?: string
  changes?: ImportChange[]
  errors?: string[]
  warnings?: string[]
}

export interface ImportChange {
  type: 'create' | 'update' | 'delete'
  resource: string
  resource_id: string
  old_value?: unknown
  new_value: unknown
  field?: string
}

export async function getImportResult(id: string): Promise<ImportResult> {
  const res = await api.get<APIResponse<ImportResult>>(`/import/${id}`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load import result')
  }
  return res.data.data
}

export type ImportSource =
  | { type: 'data'; data: string }
  | { type: 'file'; path: string }
  | { type: 'url'; url: string }

export interface ImportOptions {
  dry_run: boolean
  validate_only: boolean
}

export interface ImportRequest {
  plugin_name: string
  format: string
  source: ImportSource
  config: Record<string, unknown>
  options: ImportOptions
}

export interface ImportPreviewResponse {
  preview: ImportResult
  changes_count: number
  summary: {
    will_create: number
    will_update: number
    will_delete: number
  }
}

export async function previewImport(req: ImportRequest): Promise<ImportPreviewResponse> {
  const res = await api.post<APIResponse<ImportPreviewResponse>>('/import/preview', req)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Preview failed')
  }
  return res.data.data
}
