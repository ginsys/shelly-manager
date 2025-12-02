import api from './client'
import type { APIResponse } from './types'

// Bulk operation interfaces
export interface BulkImportRequest {
  deviceIds: number[]
  configurations: Record<string, any>[]
  options?: {
    stopOnError?: boolean
    validateOnly?: boolean
    timeout?: number
  }
}

export interface BulkExportRequest {
  deviceIds: number[]
  options?: {
    format?: 'json' | 'yaml' | 'sma'
    includeSecrets?: boolean
    includeMetadata?: boolean
  }
}

export interface BulkDriftDetectRequest {
  deviceIds: number[]
  options?: {
    stopOnError?: boolean
    detailedReport?: boolean
  }
}

export interface BulkDriftDetectEnhancedRequest {
  deviceIds: number[]
  options?: {
    stopOnError?: boolean
    detailedReport?: boolean
    includeHistory?: boolean
    compareWith?: 'template' | 'baseline' | 'peer'
    threshold?: 'strict' | 'moderate' | 'relaxed'
  }
}

export interface BulkOperationResult {
  operationId: string
  totalDevices: number
  successCount: number
  failureCount: number
  skippedCount: number
  status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'partial'
  startedAt: string
  completedAt?: string
  results: DeviceOperationResult[]
}

export interface DeviceOperationResult {
  deviceId: number
  deviceName: string
  status: 'success' | 'failed' | 'skipped'
  message?: string
  error?: string
  data?: any
}

// Bulk import configurations to multiple devices
export async function bulkImport(request: BulkImportRequest): Promise<BulkOperationResult> {
  const res = await api.post<APIResponse<BulkOperationResult>>('/config/bulk-import', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to execute bulk import'
    throw new Error(msg)
  }
  return res.data.data
}

// Bulk export configurations from multiple devices
export async function bulkExport(request: BulkExportRequest): Promise<BulkOperationResult> {
  const res = await api.post<APIResponse<BulkOperationResult>>('/config/bulk-export', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to execute bulk export'
    throw new Error(msg)
  }
  return res.data.data
}

// Bulk drift detection on multiple devices
export async function bulkDriftDetect(
  request: BulkDriftDetectRequest
): Promise<BulkOperationResult> {
  const res = await api.post<APIResponse<BulkOperationResult>>(
    '/config/bulk-drift-detect',
    request
  )
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to execute bulk drift detection'
    throw new Error(msg)
  }
  return res.data.data
}

// Enhanced bulk drift detection with advanced options
export async function bulkDriftDetectEnhanced(
  request: BulkDriftDetectEnhancedRequest
): Promise<BulkOperationResult> {
  const res = await api.post<APIResponse<BulkOperationResult>>(
    '/config/bulk-drift-detect-enhanced',
    request
  )
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to execute enhanced bulk drift detection'
    throw new Error(msg)
  }
  return res.data.data
}
