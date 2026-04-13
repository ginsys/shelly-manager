import api from './client'
import type { APIResponse, Metadata } from './types'

// Backend uses snake_case; these interfaces mirror `internal/configuration/models.go`.

export interface DriftSchedule {
  id: number
  name: string
  description?: string
  enabled: boolean
  cron_spec: string
  device_ids?: number[] | null
  device_filter?: string | null
  last_run?: string | null
  next_run?: string | null
  run_count: number
  created_at: string
  updated_at: string
}

export interface DriftScheduleRun {
  id: number
  schedule_id: number
  status: 'running' | 'completed' | 'failed'
  started_at: string
  completed_at?: string | null
  duration?: number | null
  results?: string
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

export interface CreateDriftScheduleRequest {
  name: string
  description?: string
  enabled?: boolean
  cron_spec: string
  device_ids?: number[]
  device_filter?: string
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

export async function createDriftSchedule(data: CreateDriftScheduleRequest): Promise<DriftSchedule> {
  const res = await api.post<APIResponse<DriftSchedule>>('/config/drift-schedules', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

export async function updateDriftSchedule(id: number | string, data: Partial<CreateDriftScheduleRequest>): Promise<DriftSchedule> {
  const res = await api.put<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}`, data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

export async function deleteDriftSchedule(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/config/drift-schedules/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete drift schedule'
    throw new Error(msg)
  }
}

export async function toggleDriftSchedule(id: number | string): Promise<DriftSchedule> {
  // Empty-body POST: pass {} so axios emits a Content-Type header
  // (required by the backend's ValidateContentTypeMiddleware).
  const res = await api.post<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}/toggle`, {})
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to toggle drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// Backend returns a raw array; `limit` is the only filter the handler reads.
export async function getDriftScheduleRuns(id: number | string, limit = 50): Promise<{ items: DriftScheduleRun[]; meta?: Metadata }> {
  const res = await api.get<APIResponse<DriftScheduleRun[]>>(`/config/drift-schedules/${id}/runs`, {
    params: { limit }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load schedule runs'
    throw new Error(msg)
  }
  return {
    items: res.data.data || [],
    meta: res.data.meta
  }
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
