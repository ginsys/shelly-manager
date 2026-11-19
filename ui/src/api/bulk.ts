import api from './client'
import { blobText } from './errorBody'
import { downloadBlobWithName } from './export'
import type { APIResponse } from './types'

// The bulk configuration routes act on exactly the devices named in `device_ids`, in the direction
// of the per-device route of the same name, and answer synchronously with the shapes below
// (ginsys/shelly-manager#248). The names are the wire names of the backend. None of them writes
// a device; writing a configuration to a device is the per-device apply. The bulk-export route is
// not used: it is being retired in favour of the apply (ginsys/shelly-manager#592).
//
//   bulk-import   reads each device's live configuration into the manager
//   bulk-drift-*  compares stored and live configuration
//   bulk-download saves the desired configuration of the named devices as one file, secrets masked;
//                 it reads the database and contacts no device (ginsys/shelly-manager#595)

export interface BulkRequest {
  device_ids: number[]
  options?: {
    // After the first failed device the rest are reported `skipped` and are not contacted.
    stop_on_error?: boolean
  }
}

// bulk-import
export type BulkConfigStatus = 'success' | 'failed' | 'skipped'

export interface BulkConfigDeviceResult {
  device_id: number
  device_name: string
  device_ip: string
  config_id?: number
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

// bulk-download answers the file itself, not the {success, data} envelope. The route masks every secret in
// `config` (never omits it), so a masked value is what the file holds, not the device's real value. The
// route refuses more devices than its cap, and says the number in its refusal.
export interface BulkDownloadEntry {
  device_id: number
  device_name?: string
  mac?: string
  // `error` entries carry a fixed text in `error` and no `config`.
  status: 'ok' | 'error'
  config?: unknown
  error?: string
}

export interface BulkDownloadFile {
  generated_at: string
  devices: BulkDownloadEntry[]
}

// The request has no options: the route refuses `stop_on_error: true`, since a device that cannot be
// read is an entry and not a stop.
export interface BulkDownloadRequest {
  device_ids: number[]
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

// Download the desired configuration of the named devices. Returns the file as the server sent it (to be
// saved byte for byte), the name it gave it, and its parsed form for the summary. A refusal throws the
// route's own text, which names the cap.
export async function bulkDownload(
  request: BulkDownloadRequest
): Promise<{ blob: Blob; filename?: string; file: BulkDownloadFile }> {
  const { blob, filename } = await downloadBlobWithName(() =>
    api.post('/config/bulk-download', request, { responseType: 'blob' })
  )
  let file: BulkDownloadFile | undefined
  try {
    file = JSON.parse(await blobText(blob))
  } catch {
    // handled below: not JSON is not the file
  }
  if (!file || !Array.isArray(file.devices)) {
    throw new Error('The answer is not the configuration file')
  }
  return { blob, filename, file }
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

// Turn an answer of any of the three routes into the counts and rows the dialog shows.
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
