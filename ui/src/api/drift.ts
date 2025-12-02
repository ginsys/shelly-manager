import api from './client'
import type { APIResponse, Metadata } from './types'

// Drift detection interfaces
export interface DriftSchedule {
  id: number
  name: string
  description?: string
  deviceIds?: number[]
  deviceFilter?: string
  checkInterval: string // e.g., "1h", "24h"
  enabled: boolean
  lastRun?: string
  nextRun?: string
  createdAt: string
  updatedAt: string
}

export interface DriftScheduleRun {
  id: number
  scheduleId: number
  startedAt: string
  completedAt?: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  devicesChecked: number
  driftsDetected: number
  error?: string
}

export interface DriftReport {
  id: number
  deviceId: number
  deviceName: string
  detectedAt: string
  driftType: 'config_changed' | 'unexpected_value' | 'missing_config'
  field: string
  expectedValue: any
  actualValue: any
  severity: 'low' | 'medium' | 'high' | 'critical'
  resolved: boolean
  resolvedAt?: string
  resolvedBy?: string
  notes?: string
}

export interface DriftTrend {
  date: string
  totalDrifts: number
  resolvedDrifts: number
  unresolvedDrifts: number
  deviceCount: number
}

export interface CreateDriftScheduleRequest {
  name: string
  description?: string
  deviceIds?: number[]
  deviceFilter?: string
  checkInterval: string
  enabled?: boolean
}

export interface ListDriftSchedulesResult {
  items: DriftSchedule[]
  meta?: Metadata
}

export interface ListDriftReportsResult {
  items: DriftReport[]
  meta?: Metadata
}

// List drift schedules
export async function listDriftSchedules(page = 1, pageSize = 25): Promise<ListDriftSchedulesResult> {
  const res = await api.get<APIResponse<{ schedules: DriftSchedule[] }>>('/config/drift-schedules', {
    params: { page, page_size: pageSize }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load drift schedules'
    throw new Error(msg)
  }
  return {
    items: res.data.data?.schedules || [],
    meta: res.data.meta
  }
}

// Get single drift schedule
export async function getDriftSchedule(id: number | string): Promise<DriftSchedule> {
  const res = await api.get<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// Create drift schedule
export async function createDriftSchedule(data: CreateDriftScheduleRequest): Promise<DriftSchedule> {
  const res = await api.post<APIResponse<DriftSchedule>>('/config/drift-schedules', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// Update drift schedule
export async function updateDriftSchedule(id: number | string, data: Partial<CreateDriftScheduleRequest>): Promise<DriftSchedule> {
  const res = await api.put<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}`, data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// Delete drift schedule
export async function deleteDriftSchedule(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/config/drift-schedules/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete drift schedule'
    throw new Error(msg)
  }
}

// Toggle drift schedule enabled status
export async function toggleDriftSchedule(id: number | string): Promise<DriftSchedule> {
  const res = await api.post<APIResponse<DriftSchedule>>(`/config/drift-schedules/${id}/toggle`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to toggle drift schedule'
    throw new Error(msg)
  }
  return res.data.data
}

// Get drift schedule run history
export async function getDriftScheduleRuns(id: number | string, page = 1, pageSize = 25): Promise<{ items: DriftScheduleRun[]; meta?: Metadata }> {
  const res = await api.get<APIResponse<{ runs: DriftScheduleRun[] }>>(`/config/drift-schedules/${id}/runs`, {
    params: { page, page_size: pageSize }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load schedule runs'
    throw new Error(msg)
  }
  return {
    items: res.data.data?.runs || [],
    meta: res.data.meta
  }
}

// Get drift reports
export async function getDriftReports(page = 1, pageSize = 25, resolved?: boolean): Promise<ListDriftReportsResult> {
  const res = await api.get<APIResponse<{ reports: DriftReport[] }>>('/config/drift-reports', {
    params: { page, page_size: pageSize, resolved }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load drift reports'
    throw new Error(msg)
  }
  return {
    items: res.data.data?.reports || [],
    meta: res.data.meta
  }
}

// Get drift trends
export async function getDriftTrends(days = 30): Promise<DriftTrend[]> {
  const res = await api.get<APIResponse<{ trends: DriftTrend[] }>>('/config/drift-trends', {
    params: { days }
  })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load drift trends'
    throw new Error(msg)
  }
  return res.data.data.trends || []
}

// Resolve drift report
export async function resolveDriftReport(id: number | string, notes?: string): Promise<DriftReport> {
  const res = await api.post<APIResponse<DriftReport>>(`/config/drift-trends/${id}/resolve`, { notes })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to resolve drift report'
    throw new Error(msg)
  }
  return res.data.data
}

// Generate drift report for a device
export async function generateDeviceDriftReport(deviceId: number | string): Promise<DriftReport[]> {
  const res = await api.post<APIResponse<{ reports: DriftReport[] }>>(`/devices/${deviceId}/drift-report`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to generate device drift report'
    throw new Error(msg)
  }
  return res.data.data.reports || []
}
