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

