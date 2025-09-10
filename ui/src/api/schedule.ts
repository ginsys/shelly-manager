import api from './client'
import type { APIResponse, Metadata } from './types'
import type { ExportRequest } from './export'

export interface ExportSchedule {
  id: string
  name: string
  interval_sec: number
  enabled: boolean
  request: ExportRequest
  last_run?: string
  next_run?: string
  created_at: string
  updated_at: string
  metadata?: Record<string, any>
}

export interface ExportScheduleRequest {
  name: string
  interval_sec: number
  enabled: boolean
  request: ExportRequest
}

export interface ListSchedulesParams {
  page?: number
  pageSize?: number
  plugin?: string
  enabled?: boolean
}

export interface ListSchedulesResult {
  schedules: ExportSchedule[]
  meta?: Metadata
}

export interface ScheduleRunResult {
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

/**
 * List all export schedules with optional filtering and pagination
 */
export async function listSchedules(params: ListSchedulesParams = {}): Promise<ListSchedulesResult> {
  const { page = 1, pageSize = 20, plugin, enabled } = params
  const res = await api.get<APIResponse<{ schedules: ExportSchedule[] }>>('/export/schedules', {
    params: { 
      page, 
      page_size: pageSize, 
      plugin: plugin || undefined,
      enabled: enabled !== undefined ? enabled : undefined
    },
  })
  
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load export schedules')
  }
  
  return { 
    schedules: res.data.data?.schedules || [], 
    meta: res.data.meta 
  }
}

/**
 * Create a new export schedule
 */
export async function createSchedule(request: ExportScheduleRequest): Promise<ExportSchedule> {
  const res = await api.post<APIResponse<ExportSchedule>>('/export/schedules', request)
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create schedule')
  }
  
  return res.data.data
}

/**
 * Get a specific export schedule by ID
 */
export async function getSchedule(id: string): Promise<ExportSchedule> {
  const res = await api.get<APIResponse<ExportSchedule>>(`/export/schedules/${id}`)
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load schedule')
  }
  
  return res.data.data
}

/**
 * Update an existing export schedule
 */
export async function updateSchedule(id: string, request: ExportScheduleRequest): Promise<ExportSchedule> {
  const res = await api.put<APIResponse<ExportSchedule>>(`/export/schedules/${id}`, request)
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to update schedule')
  }
  
  return res.data.data
}

/**
 * Delete an export schedule
 */
export async function deleteSchedule(id: string): Promise<void> {
  const res = await api.delete<APIResponse<{ status: string }>>(`/export/schedules/${id}`)
  
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to delete schedule')
  }
}

/**
 * Run a schedule immediately (manual execution)
 */
export async function runSchedule(id: string): Promise<ScheduleRunResult> {
  const res = await api.post<APIResponse<ScheduleRunResult>>(`/export/schedules/${id}/run`)
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to run schedule')
  }
  
  return res.data.data
}

/**
 * Calculate next run time for a given interval in seconds
 */
export function calculateNextRun(intervalSec: number, lastRun?: string): Date {
  const now = new Date()
  const lastRunDate = lastRun ? new Date(lastRun) : now
  
  return new Date(lastRunDate.getTime() + (intervalSec * 1000))
}

/**
 * Format interval seconds into human readable string
 */
export function formatInterval(intervalSec: number): string {
  if (intervalSec < 60) {
    return `${intervalSec} second${intervalSec !== 1 ? 's' : ''}`
  } else if (intervalSec < 3600) {
    const minutes = Math.floor(intervalSec / 60)
    return `${minutes} minute${minutes !== 1 ? 's' : ''}`
  } else if (intervalSec < 86400) {
    const hours = Math.floor(intervalSec / 3600)
    return `${hours} hour${hours !== 1 ? 's' : ''}`
  } else {
    const days = Math.floor(intervalSec / 86400)
    return `${days} day${days !== 1 ? 's' : ''}`
  }
}

/**
 * Parse interval string into seconds
 */
export function parseInterval(interval: string): number {
  const match = interval.match(/^(\d+)\s*(second|minute|hour|day)s?$/i)
  if (!match) {
    throw new Error('Invalid interval format. Use format like "5 minutes" or "1 hour"')
  }
  
  const [, value, unit] = match
  const num = parseInt(value, 10)
  
  switch (unit.toLowerCase()) {
    case 'second':
      return num
    case 'minute':
      return num * 60
    case 'hour':
      return num * 3600
    case 'day':
      return num * 86400
    default:
      throw new Error(`Unknown time unit: ${unit}`)
  }
}

/**
 * Validate schedule request
 */
export function validateScheduleRequest(request: ExportScheduleRequest): string[] {
  const errors: string[] = []
  
  if (!request.name || request.name.trim().length === 0) {
    errors.push('Name is required')
  }
  
  if (request.name && request.name.length > 100) {
    errors.push('Name must be less than 100 characters')
  }
  
  if (!request.interval_sec || request.interval_sec < 60) {
    errors.push('Interval must be at least 60 seconds')
  }
  
  if (request.interval_sec && request.interval_sec > 86400 * 30) {
    errors.push('Interval must be less than 30 days')
  }
  
  if (!request.request?.plugin_name) {
    errors.push('Plugin name is required')
  }
  
  if (!request.request?.format) {
    errors.push('Format is required')
  }
  
  return errors
}