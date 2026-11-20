import api from './client'
import { errorEnvelopeDetails, errorEnvelopeMessage } from './errorBody'
import type { APIResponse, Metadata } from './types'

// Backend uses snake_case; these interfaces mirror `internal/configuration/models.go`.

export interface DriftSchedule {
  id: number
  name: string
  description?: string
  enabled: boolean
  cron_spec: string
  device_ids?: number[] | null
  // Backend stores this as a raw JSON value (json.RawMessage): a DriftDeviceFilter for a schedule saved
  // through the API, anything for an older row. The form reads `type` and `status` from it.
  device_filter?: unknown
  last_run?: string | null
  next_run?: string | null
  run_count: number
  created_at: string
  updated_at: string
}

// What a caller may set on a schedule (`ScheduleInput`, internal/configuration/schedule_crud.go). A
// schedule checks the devices in `device_ids`, or the devices `device_filter` selects, or every device
// when neither is given; both at once is refused with a 400. An update replaces every field, and an
// `enabled` that is left out stays as it is on update and is true on create.
export interface DriftDeviceFilter {
  type?: string
  status?: string
}

export interface DriftScheduleInput {
  name: string
  description?: string
  cron_spec: string
  enabled?: boolean
  device_ids?: number[]
  device_filter?: DriftDeviceFilter | null
}

// One run of a schedule, as the run list answers it: newest first, without its `results` document.
// `duration` is nanoseconds; `status` is "running", "completed", "failed", "interrupted" or "skipped".
export interface DriftRun {
  id: number
  schedule_id: number
  status: string
  started_at: string
  completed_at?: string | null
  duration?: number | null
  error?: string
  created_at: string
}

// Aggregate report (summary + devices + recommendations). Backend model;
// the UI currently does not render these directly — see DriftTrend for
// per-path drift items shown on the "Drift Reports" page.
export interface DriftReport {
  id: number
  report_type: string
  device_id?: number | null
  schedule_id?: number | null
  summary: Record<string, unknown>
  devices: Array<Record<string, unknown>>
  recommendations: Array<Record<string, unknown>>
  generated_at: string
  created_at: string
}

// Per-path drift pattern tracked over time. Backend entity backing the
// UI's "Drift Reports" table (one row per drifting config path per device)
// and resolvable via POST /config/drift-trends/{id}/resolve.
export interface DriftTrend {
  id: number
  device_id: number
  path: string
  category: string
  severity: 'low' | 'medium' | 'high' | 'critical' | string
  first_seen: string
  last_seen: string
  occurrences: number
  resolved: boolean
  resolved_at?: string | null
  created_at: string
  updated_at: string
}

// Daily aggregate derived client-side from a list of DriftTrend records;
// fuels the "Drift Trends" chart page. Backend has no equivalent endpoint.
export interface DriftDailyAggregate {
  date: string
  totalDrifts: number
  resolvedDrifts: number
  unresolvedDrifts: number
  deviceCount: number
}

export interface ListDriftSchedulesResult {
  items: DriftSchedule[]
  meta?: Metadata
}

// List drift schedules. Backend returns a raw array at `data`.
export async function listDriftSchedules(page = 1, pageSize = 25): Promise<ListDriftSchedulesResult> {
  const res = await api.get<APIResponse<DriftSchedule[]>>('/config/drift-schedules', {
    params: { page, page_size: pageSize }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load drift schedules'
    throw new Error(msg)
  }
  return {
    items: res.data.data || [],
    meta: res.data.meta
  }
}

export async function getDriftSchedule(id: number | string): Promise<DriftSchedule> {
  const res = await api.get<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// A stored schedule runs only where the server has drift.scheduler_enabled set, and the status route
// says whether the server that answers has. The status is the state of one server process, so replicas
// configured differently answer differently.
export async function getDriftSchedulerStatus(): Promise<boolean> {
  const res = await api.get<APIResponse<{ enabled: boolean }>>('/config/drift-scheduler')
  if (!res.data.success || typeof res.data.data?.enabled !== 'boolean') {
    const msg = res.data.error?.message || 'Failed to read the drift scheduler state'
    throw new Error(msg)
  }
  return res.data.data.enabled
}

export async function deleteDriftSchedule(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/config/drift-schedules/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete drift schedule'
    throw new Error(msg)
  }
}

// axios rejects a 4xx or 5xx answer before the envelope is read. A refused schedule input is a 400 whose
// reason is the envelope's `details` (its `message` only says "Validation failed"), and a missing
// schedule is a 404 with a `message`: throw the server's own text, or the original error when it sent none.
async function throwServerReason(e: any): Promise<never> {
  const data = e?.response?.data
  const reason = (await errorEnvelopeDetails(data)) ?? (await errorEnvelopeMessage(data))
  if (reason) throw new Error(reason)
  throw e
}

export async function createDriftSchedule(input: DriftScheduleInput): Promise<DriftSchedule> {
  const res = await api.post<APIResponse<DriftSchedule>>('/config/drift-schedules', input).catch(throwServerReason)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// Replaces every field of the schedule with the input.
export async function updateDriftSchedule(id: number | string, input: DriftScheduleInput): Promise<DriftSchedule> {
  const res = await api
    .put<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}`, input)
    .catch(throwServerReason)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

export async function toggleDriftSchedule(id: number | string): Promise<DriftSchedule> {
  const res = await api
    .post<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}/toggle`, {})
    .catch(throwServerReason)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to toggle drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// The newest runs of one schedule, `limit` 1 to 100 (the server's default is 20 when it is left out).
export async function listDriftScheduleRuns(id: number | string, limit?: number): Promise<DriftRun[]> {
  const res = await api
    .get<APIResponse<DriftRun[]>>(`/config/drift-schedules/${id}/runs`, limit === undefined ? undefined : { params: { limit } })
    .catch(throwServerReason)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load the runs of the drift schedule'
    throw new Error(msg)
  }
  return res.data.data || []
}

// Aggregate drift reports. Returned as a raw array. Rarely used by the UI today;
// the Drift Reports page now renders DriftTrend rows (via getDriftTrends).
export async function getDriftReports(limit = 50, deviceId?: number, reportType?: string): Promise<DriftReport[]> {
  const res = await api.get<APIResponse<DriftReport[]>>('/config/drift-reports', {
    params: { limit, device_id: deviceId, type: reportType }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load drift reports'
    throw new Error(msg)
  }
  return res.data.data || []
}

// Per-path drift patterns. Backend returns a raw array; filters are
// `device_id`, `resolved`, `limit`. There is no `days` filter on the server —
// the caller passes `limit` and derives date-range views client-side.
export async function getDriftTrends(limit = 500, resolved?: boolean, deviceId?: number): Promise<DriftTrend[]> {
  const res = await api.get<APIResponse<DriftTrend[]>>('/config/drift-trends', {
    params: { limit, resolved, device_id: deviceId }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load drift trends'
    throw new Error(msg)
  }
  return res.data.data || []
}

// Mark a DriftTrend (per-path pattern) as resolved. The backend handler
// ignores the request body but the Content-Type middleware still requires
// a JSON payload, so we send {}.
export async function resolveDriftTrend(id: number | string): Promise<void> {
  const res = await api.post<APIResponse<{ status: string }>>(`/config/drift-trends/${id}/resolve`, {})
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to resolve drift trend'
    throw new Error(msg)
  }
}

export async function generateDeviceDriftReport(deviceId: number | string): Promise<DriftReport> {
  const res = await api.post<APIResponse<DriftReport>>(`/devices/${deviceId}/drift-report`, {})
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to generate device drift report'
    throw new Error(msg)
  }
  return res.data.data
}

// Compute day-bucketed drift totals from per-path trend records. Each trend
// contributes to the day in which it was last seen; resolution counts use
// resolved_at when present, otherwise last_seen.
export function computeDailyAggregates(trends: DriftTrend[], days = 30): DriftDailyAggregate[] {
  const bucket = new Map<string, { total: number; resolved: number; unresolved: number; devices: Set<number> }>()
  const cutoff = Date.now() - days * 86_400_000

  for (const t of trends) {
    const basis = t.resolved && t.resolved_at ? t.resolved_at : t.last_seen
    const time = new Date(basis).getTime()
    if (!Number.isFinite(time) || time < cutoff) continue

    const date = basis.slice(0, 10)
    let entry = bucket.get(date)
    if (!entry) {
      entry = { total: 0, resolved: 0, unresolved: 0, devices: new Set<number>() }
      bucket.set(date, entry)
    }
    entry.total += 1
    if (t.resolved) entry.resolved += 1
    else entry.unresolved += 1
    entry.devices.add(t.device_id)
  }

  return Array.from(bucket.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, e]) => ({
      date,
      totalDrifts: e.total,
      resolvedDrifts: e.resolved,
      unresolvedDrifts: e.unresolved,
      deviceCount: e.devices.size
    }))
}
