import api from './client'
import type { APIResponse } from './types'

// The four bulk configuration routes act on exactly the devices named in `device_ids`, in the
// direction of the per-device route of the same name, and answer synchronously with the shapes
// below (ginsys/shelly-manager#248). The names are the wire names of the backend.
//
//   bulk-import   reads each device's live configuration into the manager (never writes a device)
//   bulk-export   writes each device's stored import/export configuration to the device
//   bulk-drift-*  compares stored and live configuration (never writes a device)

export interface BulkRequest {
  device_ids: number[]
  options?: {
    // After the first failed device the rest are reported `skipped` and are not contacted.
    stop_on_error?: boolean
  }
}

// bulk-import and bulk-export
export type BulkConfigStatus = 'success' | 'failed' | 'skipped'

export interface BulkConfigDeviceResult {
  device_id: number
  device_name: string
  device_ip: string
  config_id?: number // import only
  status: BulkConfigStatus
  error?: string
}

export interface BulkConfigResult {
  total: number
  success: number
  failed: number
  skipped: number
  results: BulkConfigDeviceResult[]
}

// bulk-drift-detect
export type BulkDriftStatus = 'synced' | 'drift' | 'error' | 'skipped'

export interface BulkDriftDeviceResult {
  device_id: number
  device_name: string
  device_ip: string
  status: BulkDriftStatus
  error?: string
  drift_summary?: string
  difference_count?: number
  drift?: unknown
}

export interface BulkDriftResult {
  total: number
  in_sync: number
  drifted: number
  errors: number
  skipped: number
  results: BulkDriftDeviceResult[]
  started_at: string
  completed_at: string
  duration: number // nanoseconds
}

// bulk-drift-detect-enhanced: the comprehensive report of the devices that ran. Only the fields
// the dialog reads are typed. If the report cannot be stored the route answers a BulkDriftResult.
export interface BulkDriftReport {
  id: number
  report_type: string
  summary: {
    total_devices: number
    devices_in_sync: number
    devices_drifted: number
    devices_errored: number
  }
  devices: {
    device_id: number
    device_name: string
    device_ip: string
    status: 'synced' | 'drift' | 'error'
    total_differences: number
    error?: string
  }[]
}

// What the dialog shows for any of the answers.
export type BulkTone = 'success' | 'failure' | 'skipped' | 'warning'

export interface BulkView {
  stats: { label: string; value: number; tone: BulkTone }[]
  rows: { deviceId: number; deviceName: string; status: string; tone: BulkTone; message: string }[]
}

export type BulkAnswer = BulkConfigResult | BulkDriftResult | BulkDriftReport

// Unwrap the envelope of a bulk call. Each route below keeps its own literal `api.post` URL, which
// the route contract test reads.
async function unwrap<T>(call: Promise<{ data: APIResponse<T> }>, failure: string): Promise<T> {
  let body: APIResponse<T>
  try {
    body = (await call).data
  } catch (e: any) {
    // A refused request is a 400 whose `details` says why.
    const error = e?.response?.data?.error
    const details = typeof error?.details === 'string' ? error.details : undefined
    throw new Error(details || error?.message || e?.message || failure)
  }
  if (!body.success || !body.data) {
    throw new Error(body.error?.message || failure)
  }
  return body.data
}

// Read each device's live configuration into the manager.
export function bulkImport(request: BulkRequest): Promise<BulkConfigResult> {
  return unwrap(
    api.post<APIResponse<BulkConfigResult>>('/config/bulk-import', request),
    'Failed to execute bulk import'
  )
}

// Write each device's stored import/export configuration to the device.
export function bulkExport(request: BulkRequest): Promise<BulkConfigResult> {
  return unwrap(
    api.post<APIResponse<BulkConfigResult>>('/config/bulk-export', request),
    'Failed to execute bulk export'
  )
}

// Compare stored and live configuration.
export function bulkDriftDetect(request: BulkRequest): Promise<BulkDriftResult> {
  return unwrap(
    api.post<APIResponse<BulkDriftResult>>('/config/bulk-drift-detect', request),
    'Failed to execute bulk drift detection'
  )
}

// The same selection, answered with the comprehensive report.
export function bulkDriftDetectEnhanced(
  request: BulkRequest
): Promise<BulkDriftReport | BulkDriftResult> {
  return unwrap(
    api.post<APIResponse<BulkDriftReport | BulkDriftResult>>(
      '/config/bulk-drift-detect-enhanced',
      request
    ),
    'Failed to execute enhanced bulk drift detection'
  )
}

const configTone: Record<BulkConfigStatus, BulkTone> = {
  success: 'success',
  failed: 'failure',
  skipped: 'skipped'
}

const driftTone: Record<BulkDriftStatus, BulkTone> = {
  synced: 'success',
  drift: 'warning',
  error: 'failure',
  skipped: 'skipped'
}

// Turn an answer of any of the four routes into the counts and rows the dialog shows.
export function viewOf(answer: BulkAnswer): BulkView {
  if ('summary' in answer) {
    return {
      stats: [
        { label: 'In sync', value: answer.summary.devices_in_sync, tone: 'success' },
        { label: 'Drifted', value: answer.summary.devices_drifted, tone: 'warning' },
        { label: 'Errors', value: answer.summary.devices_errored, tone: 'failure' }
      ],
      rows: answer.devices.map((d) => ({
        deviceId: d.device_id,
        deviceName: d.device_name,
        status: d.status,
        tone: driftTone[d.status] ?? 'warning',
        message: d.error || (d.total_differences ? `${d.total_differences} difference(s)` : '')
      }))
    }
  }
  if ('in_sync' in answer) {
    return {
      stats: [
        { label: 'In sync', value: answer.in_sync, tone: 'success' },
        { label: 'Drifted', value: answer.drifted, tone: 'warning' },
        { label: 'Errors', value: answer.errors, tone: 'failure' },
        { label: 'Skipped', value: answer.skipped, tone: 'skipped' }
      ],
      rows: answer.results.map((d) => ({
        deviceId: d.device_id,
        deviceName: d.device_name,
        status: d.status,
        tone: driftTone[d.status] ?? 'warning',
        message: d.error || d.drift_summary || ''
      }))
    }
  }
  return {
    stats: [
      { label: 'Success', value: answer.success, tone: 'success' },
      { label: 'Failed', value: answer.failed, tone: 'failure' },
      { label: 'Skipped', value: answer.skipped, tone: 'skipped' }
    ],
    rows: answer.results.map((d) => ({
      deviceId: d.device_id,
      deviceName: d.device_name,
      status: d.status,
      tone: configTone[d.status] ?? 'failure',
      message: d.error || ''
    }))
  }
}
