import api from './client'
import type { APIResponse } from './types'

export interface MetricsStatus { enabled: boolean; last_collection_time?: string; uptime_seconds?: number }

export async function getMetricsStatus(): Promise<MetricsStatus> {
  const res = await api.get<APIResponse<MetricsStatus>>('/metrics/status')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load metrics status')
  return res.data.data
}

export async function getMetricsHealth(): Promise<any> {
  const res = await api.get<APIResponse<any>>('/metrics/health')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load health')
  return res.data.data
}

export function openMetricsWebSocket(onMessage: (data:any)=>void): WebSocket {
  const base = (window as any).__API_BASE__ || '/api/v1'
  const loc = window.location
  const proto = loc.protocol === 'https:' ? 'wss' : 'ws'
  const token = (window as any).__ADMIN_KEY__
  const url = `${proto}://${loc.host}${base.replace('/api/v1','')}/metrics/ws${token?`?token=${encodeURIComponent(token)}`:''}`
  const ws = new WebSocket(url)
  ws.onmessage = (ev) => {
    try { onMessage(JSON.parse(ev.data)) } catch { /* ignore */ }
  }
  return ws
}

export async function getSystemMetrics(): Promise<any> {
  const res = await api.get<APIResponse<any>>('/metrics/system')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load system metrics')
  return res.data.data
}

export async function getDevicesMetrics(): Promise<any> {
  const res = await api.get<APIResponse<any>>('/metrics/devices')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load devices metrics')
  return res.data.data
}

export async function getDriftSummary(): Promise<any> {
  const res = await api.get<APIResponse<any>>('/metrics/drift')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load drift summary')
  return res.data.data
}

// Advanced Metrics Interfaces
export interface DashboardSummary {
  devices: { total: number; online: number; offline: number }
  exports: { total: number; recent: number }
  imports: { total: number; recent: number }
  drifts: { total: number; unresolved: number }
  notifications: { sent: number; failed: number }
}

export interface NotificationMetrics {
  totalSent: number
  totalFailed: number
  byChannel: Record<string, { sent: number; failed: number }>
  recentNotifications: Array<{ timestamp: string; channel: string; status: string }>
}

export interface ResolutionMetrics {
  totalResolved: number
  averageResolutionTime: number
  byType: Record<string, number>
  byUser: Record<string, number>
}

export interface SecurityMetrics {
  authAttempts: { successful: number; failed: number }
  apiCalls: { total: number; errors: number }
  rateLimit: { triggered: number; blocked: number }
}

export interface TestAlertResult {
  success: boolean
  message: string
  timestamp: string
}

// Get Prometheus-formatted metrics
export async function getPrometheusMetrics(): Promise<string> {
  const res = await api.get('/metrics/prometheus', { responseType: 'text' })
  if (typeof res.data === 'string') {
    return res.data
  }
  throw new Error('Failed to load Prometheus metrics')
}

// Enable metrics collection
export async function enableMetrics(): Promise<MetricsStatus> {
  const res = await api.post<APIResponse<MetricsStatus>>('/metrics/enable')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to enable metrics collection')
  }
  return res.data.data
}

// Disable metrics collection
export async function disableMetrics(): Promise<MetricsStatus> {
  const res = await api.post<APIResponse<MetricsStatus>>('/metrics/disable')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to disable metrics collection')
  }
  return res.data.data
}

// Trigger metrics collection
export async function collectMetrics(): Promise<MetricsStatus> {
  const res = await api.post<APIResponse<MetricsStatus>>('/metrics/collect')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to trigger metrics collection')
  }
  return res.data.data
}

// Get dashboard summary
export async function getDashboardSummary(): Promise<DashboardSummary> {
  const res = await api.get<APIResponse<DashboardSummary>>('/metrics/dashboard')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load dashboard summary')
  }
  return res.data.data
}

// Send test alert
export async function sendTestAlert(): Promise<TestAlertResult> {
  const res = await api.post<APIResponse<TestAlertResult>>('/metrics/test-alert')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to send test alert')
  }
  return res.data.data
}

// Get notification metrics
export async function getNotificationMetrics(): Promise<NotificationMetrics> {
  const res = await api.get<APIResponse<NotificationMetrics>>('/metrics/notifications')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load notification metrics')
  }
  return res.data.data
}

// Get resolution metrics
export async function getResolutionMetrics(): Promise<ResolutionMetrics> {
  const res = await api.get<APIResponse<ResolutionMetrics>>('/metrics/resolution')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load resolution metrics')
  }
  return res.data.data
}

// Get security metrics
export async function getSecurityMetrics(): Promise<SecurityMetrics> {
  const res = await api.get<APIResponse<SecurityMetrics>>('/metrics/security')
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load security metrics')
  }
  return res.data.data
}
