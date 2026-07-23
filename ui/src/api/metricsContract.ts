/**
 * TypeScript mirror of the Go metrics WebSocket payloads
 * (internal/metrics/websocket.go) plus a runtime validator.
 *
 * The compile-time discriminated union (MetricsWsMessage) is enforced against
 * the backend by the manifest + assertNever machinery in ./metricsMessages.ts.
 * This module adds the *runtime* half: parseMetricsWsMessage rejects malformed
 * frames (wrong shape, unknown type, missing fields) so bad data is surfaced and
 * never applied — a typed union alone cannot vet bytes off the wire.
 */
import { METRICS_WS_MESSAGE_TYPES, type MetricsWsMessageType } from './metricsMessages'

// --- Payload shapes (json tags from the Go structs) ---

export interface SystemStatus {
  uptime_seconds: number
  metrics_enabled: boolean
  last_collection_time: string
  total_devices: number
  online_devices: number
  devices_with_drift: number
}

export interface DeviceMetric {
  id: string
  name: string
  type: string
  status: string
  config_synced: boolean
  last_seen: string
}

export interface DriftMetrics {
  total_drift_issues: number
  severity_distribution: Record<string, number>
  category_distribution: Record<string, number>
  trend_analysis: unknown[]
}

export interface NotificationMetrics {
  total_sent: number
  total_failed: number
  channel_breakdown: Record<string, number>
  alert_level_breakdown: Record<string, number>
  average_latency_seconds: number
}

export interface ResolutionMetrics {
  total_resolutions: number
  auto_fix_success_rate: Record<string, number>
  resolutions_by_category: Record<string, number>
  average_review_time_seconds: number
}

export interface DashboardMetrics {
  system_status: SystemStatus
  device_metrics: DeviceMetric[]
  drift_metrics: DriftMetrics
  notification_metrics: NotificationMetrics
  resolution_metrics: ResolutionMetrics
}

export interface AlertPayload {
  alert_type: string
  message: string
  severity: string
}

export interface DeviceStatusChangePayload {
  device_id: string
  device_name: string
  old_status: string
  new_status: string
  timestamp: string
}

export interface DriftDetectedPayload {
  device_id: string
  device_name: string
  drift_count: number
  severity: string
  timestamp: string
}

// --- Discriminated union envelope ---

interface Envelope<T extends MetricsWsMessageType, D> {
  type: T
  timestamp: string
  data: D
}

export type MetricsWsMessage =
  | Envelope<'initial_metrics', DashboardMetrics>
  | Envelope<'metrics_update', DashboardMetrics>
  | Envelope<'alert', AlertPayload>
  | Envelope<'device_status_change', DeviceStatusChangePayload>
  | Envelope<'drift_detected', DriftDetectedPayload>

/** A metrics snapshot message (initial hydrate or periodic update). */
export type DashboardMessage = Extract<MetricsWsMessage, { type: 'initial_metrics' | 'metrics_update' }>
/** A discrete event message (does not carry a full snapshot). */
export type EventMessage = Extract<
  MetricsWsMessage,
  { type: 'alert' | 'device_status_change' | 'drift_detected' }
>

export function isDashboardMessage(msg: MetricsWsMessage): msg is DashboardMessage {
  return msg.type === 'initial_metrics' || msg.type === 'metrics_update'
}

// --- Runtime validation ---

function isObj(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}
function isStr(v: unknown): v is string {
  return typeof v === 'string'
}
function isNum(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v)
}
function isBool(v: unknown): v is boolean {
  return typeof v === 'boolean'
}
function isNumberRecord(v: unknown): v is Record<string, number> {
  return isObj(v) && Object.values(v).every(isNum)
}

const KNOWN_TYPES = new Set<string>(METRICS_WS_MESSAGE_TYPES)

function validSystemStatus(v: unknown): v is SystemStatus {
  return (
    isObj(v) &&
    isNum(v.uptime_seconds) &&
    isBool(v.metrics_enabled) &&
    isStr(v.last_collection_time) &&
    isNum(v.total_devices) &&
    isNum(v.online_devices) &&
    isNum(v.devices_with_drift)
  )
}

function validDevice(v: unknown): v is DeviceMetric {
  return (
    isObj(v) &&
    isStr(v.id) &&
    isStr(v.name) &&
    isStr(v.type) &&
    isStr(v.status) &&
    isBool(v.config_synced) &&
    isStr(v.last_seen)
  )
}

function validDrift(v: unknown): boolean {
  return (
    isObj(v) &&
    isNum(v.total_drift_issues) &&
    isNumberRecord(v.severity_distribution) &&
    isNumberRecord(v.category_distribution) &&
    Array.isArray(v.trend_analysis)
  )
}

function validNotification(v: unknown): boolean {
  return (
    isObj(v) &&
    isNum(v.total_sent) &&
    isNum(v.total_failed) &&
    isNumberRecord(v.channel_breakdown) &&
    isNumberRecord(v.alert_level_breakdown) &&
    isNum(v.average_latency_seconds)
  )
}

function validResolution(v: unknown): boolean {
  return (
    isObj(v) &&
    isNum(v.total_resolutions) &&
    isNumberRecord(v.auto_fix_success_rate) &&
    isNumberRecord(v.resolutions_by_category) &&
    isNum(v.average_review_time_seconds)
  )
}

// Validate every documented DashboardMetrics field so a partial frame can't be
// cast to a complete snapshot and leave consumers reading undefined.
function validDashboard(v: unknown): boolean {
  if (!isObj(v)) return false
  if (!validSystemStatus(v.system_status)) return false
  if (!Array.isArray(v.device_metrics) || !v.device_metrics.every(validDevice)) return false
  if (!validDrift(v.drift_metrics)) return false
  if (!validNotification(v.notification_metrics)) return false
  if (!validResolution(v.resolution_metrics)) return false
  return true
}

function validAlert(v: unknown): boolean {
  return isObj(v) && isStr(v.alert_type) && isStr(v.message) && isStr(v.severity)
}

function validDeviceStatusChange(v: unknown): boolean {
  return (
    isObj(v) &&
    isStr(v.device_id) &&
    isStr(v.device_name) &&
    isStr(v.old_status) &&
    isStr(v.new_status)
  )
}

function validDriftDetected(v: unknown): boolean {
  return (
    isObj(v) &&
    isStr(v.device_id) &&
    isStr(v.device_name) &&
    isNum(v.drift_count) &&
    isStr(v.severity)
  )
}

export interface ParseOk {
  ok: true
  message: MetricsWsMessage
}
export interface ParseErr {
  ok: false
  reason: string
  type?: string
}
export type ParseResult = ParseOk | ParseErr

/**
 * Validate an already-JSON-parsed frame against the metrics WebSocket contract.
 * Returns a typed message on success, or a reason on failure. Callers must treat
 * a failure as "do not apply, surface it" — never as a live feed.
 */
export function parseMetricsWsMessage(raw: unknown): ParseResult {
  if (!isObj(raw)) return { ok: false, reason: 'frame is not an object' }
  const { type, data } = raw
  if (!isStr(type)) return { ok: false, reason: 'missing string "type"' }
  if (!KNOWN_TYPES.has(type)) return { ok: false, reason: `unknown message type "${type}"`, type }
  if (!isStr(raw.timestamp)) return { ok: false, reason: 'missing string "timestamp"', type }

  const t = type as MetricsWsMessageType
  switch (t) {
    case 'initial_metrics':
    case 'metrics_update':
      if (!validDashboard(data)) return { ok: false, reason: 'invalid dashboard payload', type }
      break
    case 'alert':
      if (!validAlert(data)) return { ok: false, reason: 'invalid alert payload', type }
      break
    case 'device_status_change':
      if (!validDeviceStatusChange(data)) {
        return { ok: false, reason: 'invalid device_status_change payload', type }
      }
      break
    case 'drift_detected':
      if (!validDriftDetected(data)) return { ok: false, reason: 'invalid drift_detected payload', type }
      break
  }
  return { ok: true, message: raw as unknown as MetricsWsMessage }
}
