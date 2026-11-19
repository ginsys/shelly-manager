import api from './client'
import type { APIResponse } from './types'
import type { DashboardMetrics, NotificationMetrics, ResolutionMetrics } from './metricsContract'

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

// Advanced metrics answers. The dashboard, notification and resolution shapes
// are the Go structs the WebSocket contract already mirrors (metricsContract.ts),
// so they are imported, not redeclared.

// Mirrors middleware.IPAttackSummary (internal/api/middleware/security_monitor.go).
export interface IPAttackSummary {
  ip: string
  total_requests: number
  suspicious_requests: number
  attack_types: Record<string, number>
  first_seen: string
  last_seen: string
  blocked: boolean
}

// Mirrors middleware.SecurityMetrics (internal/api/middleware/security_monitor.go).
// A Go nil slice marshals to null, hence the nullable attacker list.
export interface SecurityMetrics {
  total_requests: number
  blocked_requests: number
  suspicious_requests: number
  rate_limit_violations: number
  validation_failures: number
  attacks_by_type: Record<string, number>
  top_attacker_ips: IPAttackSummary[] | null
  alerts_by_level: Record<string, number>
  last_updated: string
  blocked_ips: number
  active_threats: number
}

// What SendTestAlert (internal/metrics/handlers.go) sends inside the envelope.
export interface TestAlertResult {
  status: string
  alert_type: string
  message: string
  severity: string
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
export async function getDashboardSummary(): Promise<DashboardMetrics> {
  const res = await api.get<APIResponse<DashboardMetrics>>('/metrics/dashboard')
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
