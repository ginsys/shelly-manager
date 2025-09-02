import api from './client'
import type { APIResponse, Metadata } from './types'

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

