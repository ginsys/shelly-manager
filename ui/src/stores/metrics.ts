import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  getMetricsStatus,
  getMetricsHealth,
  getSystemMetrics,
  getDevicesMetrics,
  getDriftSummary,
  type MetricsStatus,
} from '@/api/metrics'
import { useWebSocket } from '@/composables/useWebSocket'
import { assertNever } from '@/api/metricsMessages'
import {
  parseMetricsWsMessage,
  type SystemStatus,
  type DeviceMetric,
  type DriftMetrics,
  type NotificationMetrics,
  type ResolutionMetrics,
  type DashboardMetrics,
  type EventMessage,
} from '@/api/metricsContract'

// Freshness state machine. Exactly one is active at a time:
//   idle    – nothing started yet
//   polling – REST fallback is the source of truth (no fresh WS metrics)
//   live    – WS metrics applied within STALE_MS; the "LIVE" badge is truthful
//   stale   – was live but metrics stopped arriving; REST resumes
// "live" is the ONLY state that reports real-time; connection state alone never
// does (a socket can be open with no data flowing).
export type FeedState = 'idle' | 'polling' | 'live' | 'stale'

// Backend broadcasts a snapshot every 5s; tolerate a few missed ticks before
// declaring the feed stale.
const STALE_MS = 20000
const WATCHDOG_INTERVAL_MS = 5000
const POLL_INTERVAL_MS = 30000
const SERIES_MAX = 50
const EVENTS_MAX = 100

export interface LiveEvent {
  id: number
  kind: EventMessage['type']
  severity: string
  message: string
  timestamp: string
}

export interface CountSeries {
  timestamps: string[]
  online: number[]
  total: number[]
  drift: number[]
  maxLength: number
}

export const useMetricsStore = defineStore('metrics', () => {
  // --- REST-only panels ---
  const status = ref<MetricsStatus | null>(null)
  const health = ref<Record<string, unknown> | null>(null)

  // --- Metrics snapshot state (WS or REST) ---
  const systemStatus = ref<SystemStatus | null>(null)
  const deviceMetrics = ref<DeviceMetric[]>([])
  const drift = ref<DriftMetrics | null>(null)
  const notification = ref<NotificationMetrics | null>(null)
  const resolution = ref<ResolutionMetrics | null>(null)

  // Repurposed "system" chart: device-count trend from system_status (the
  // backend has no CPU/memory/disk telemetry — see #247). Bounded ring buffer.
  const series = ref<CountSeries>({
    timestamps: [],
    online: [],
    total: [],
    drift: [],
    maxLength: SERIES_MAX,
  })

  // Live events feed (alert / device_status_change / drift_detected). Never
  // coalesced or dropped.
  const events = ref<LiveEvent[]>([])
  let eventSeq = 0

  // Freshness / staleness.
  const feedState = ref<FeedState>('idle')
  const lastAppliedMetricsAt = ref<number | null>(null)
  // Monotonic counter bumped on every applied WS snapshot; used to detect that
  // WS data landed while a REST request was in flight (stale-REST protection).
  let metricsApplySeq = 0

  // Diagnostics for invalid frames (surfaced, not swallowed).
  const lastInvalidReason = ref<string | null>(null)
  const invalidMessageCount = ref(0)

  const _timer = ref<ReturnType<typeof setInterval> | null>(null)
  const _watchdog = ref<ReturnType<typeof setInterval> | null>(null)

  // WebSocket URL generator
  function getWebSocketUrl(): string {
    const base = (window as unknown as { __API_BASE__?: string }).__API_BASE__ || '/api/v1'
    const loc = window.location
    const proto = loc.protocol === 'https:' ? 'wss' : 'ws'
    const token = (window as unknown as { __ADMIN_KEY__?: string }).__ADMIN_KEY__
    return `${proto}://${loc.host}${base.replace('/api/v1', '')}/metrics/ws${
      token ? `?token=${encodeURIComponent(token)}` : ''
    }`
  }

  const ws = useWebSocket<unknown>({
    url: getWebSocketUrl,
    autoConnect: false,
    reconnect: true,
    reconnectInterval: 1000,
    heartbeatInterval: 15000,
    heartbeatTimeout: 45000,
    onMessage: (raw) => handleWSMessage(raw),
    onOpen: () => {
      // Deliberately NOT stopping polling here: an open socket is not live data.
      // REST keeps running until the first valid snapshot is applied (#247).
    },
    onClose: () => {
      // Fall back to polling; the feed is no longer live.
      if (feedState.value !== 'idle') feedState.value = 'polling'
    },
    onError: (error) => {
      console.error('WebSocket error:', error)
    },
  })

  // --- Getters ---
  const wsConnected = computed(() => ws.isConnected.value)
  const wsReconnectAttempts = computed(() => ws.reconnectAttempts.value)
  const lastMessageAt = computed(() => ws.lastMessageAt.value)

  // The feed is live only when a fresh metrics snapshot has been APPLIED — never
  // from connection state or transport-level lastMessageAt.
  const isRealtimeActive = computed(() => feedState.value === 'live')

  // Device status distribution for the devices bar chart.
  const deviceStatusCounts = computed<Record<string, number>>(() => {
    const counts: Record<string, number> = {}
    for (const d of deviceMetrics.value) {
      const key = d.status || 'unknown'
      counts[key] = (counts[key] || 0) + 1
    }
    return counts
  })

  // --- Metrics application ---
  function pushSeries(s: SystemStatus, timestamp: string) {
    const buf = series.value
    buf.timestamps.push(timestamp)
    buf.online.push(s.online_devices)
    buf.total.push(s.total_devices)
    buf.drift.push(s.devices_with_drift)
    const overflow = buf.timestamps.length - buf.maxLength
    if (overflow > 0) {
      buf.timestamps.splice(0, overflow)
      buf.online.splice(0, overflow)
      buf.total.splice(0, overflow)
      buf.drift.splice(0, overflow)
    }
  }

  function applyDashboard(d: DashboardMetrics, timestamp: string) {
    systemStatus.value = d.system_status
    deviceMetrics.value = d.device_metrics ?? []
    drift.value = d.drift_metrics
    notification.value = d.notification_metrics
    resolution.value = d.resolution_metrics
    pushSeries(d.system_status, timestamp)

    lastAppliedMetricsAt.value = Date.now()
    metricsApplySeq++
    feedState.value = 'live'
  }

  function appendEvent(msg: EventMessage) {
    eventSeq++
    let severity = 'info'
    let message = ''
    switch (msg.type) {
      case 'alert':
        severity = msg.data.severity
        message = msg.data.message
        break
      case 'device_status_change':
        severity = msg.data.new_status === 'offline' ? 'warning' : 'info'
        message = `${msg.data.device_name}: ${msg.data.old_status} → ${msg.data.new_status}`
        break
      case 'drift_detected':
        severity = msg.data.severity
        message = `Drift on ${msg.data.device_name} (${msg.data.drift_count} issue${
          msg.data.drift_count === 1 ? '' : 's'
        })`
        break
      default:
        return assertNever(msg)
    }
    events.value.push({ id: eventSeq, kind: msg.type, severity, message, timestamp: msg.timestamp })
    const overflow = events.value.length - EVENTS_MAX
    if (overflow > 0) events.value.splice(0, overflow)
  }

  // Message handling. Invalid frames are surfaced and never applied; events are
  // appended immediately (never coalesced/dropped); snapshots hydrate state.
  function handleWSMessage(raw: unknown) {
    const result = parseMetricsWsMessage(raw)
    if (!result.ok) {
      lastInvalidReason.value = result.reason
      invalidMessageCount.value++
      console.error('Discarded invalid metrics WebSocket message:', result.reason)
      return
    }

    const msg = result.message
    switch (msg.type) {
      case 'initial_metrics':
      case 'metrics_update':
        applyDashboard(msg.data, msg.timestamp)
        break
      case 'alert':
      case 'device_status_change':
      case 'drift_detected':
        appendEvent(msg)
        break
      default:
        assertNever(msg)
    }
  }

  // --- REST fallback ---
  async function fetchStatus() {
    try {
      status.value = await getMetricsStatus()
    } catch (e) {
      console.warn('Failed to fetch metrics status:', e)
    }
  }

  async function fetchHealth() {
    try {
      health.value = await getMetricsHealth()
    } catch (e) {
      console.warn('Failed to fetch metrics health:', e)
    }
  }

  async function fetchSummaries() {
    // Stale-REST protection: capture the apply sequence before awaiting. If a WS
    // snapshot lands (or the feed goes live) while these requests are in flight,
    // discard the REST results so we never overwrite fresher WS data.
    const seqAtStart = metricsApplySeq

    const [systemRes, devicesRes, driftRes] = await Promise.allSettled([
      getSystemMetrics(),
      getDevicesMetrics(),
      getDriftSummary(),
    ])

    if (feedState.value === 'live' || metricsApplySeq !== seqAtStart) {
      return // superseded by WS while awaiting
    }

    if (systemRes.status === 'fulfilled' && systemRes.value) {
      const s = systemRes.value as SystemStatus
      systemStatus.value = s
      pushSeries(s, new Date().toISOString())
    } else if (systemRes.status === 'rejected') {
      console.warn('Failed to fetch system metrics:', systemRes.reason)
    }

    if (devicesRes.status === 'fulfilled' && devicesRes.value) {
      const payload = devicesRes.value as { devices?: DeviceMetric[] }
      deviceMetrics.value = payload.devices ?? []
    } else if (devicesRes.status === 'rejected') {
      console.warn('Failed to fetch devices metrics:', devicesRes.reason)
    }

    if (driftRes.status === 'fulfilled' && driftRes.value) {
      drift.value = driftRes.value as DriftMetrics
    } else if (driftRes.status === 'rejected') {
      console.warn('Failed to fetch drift summary:', driftRes.reason)
    }
  }

  // --- Polling + watchdog ---
  function startPolling(intervalMs = POLL_INTERVAL_MS) {
    if (feedState.value === 'idle') feedState.value = 'polling'
    startWatchdog()
    if (_timer.value) return
    _timer.value = setInterval(() => {
      if (feedState.value !== 'live') fetchSummaries()
    }, intervalMs)
    if (feedState.value !== 'live') fetchSummaries()
  }

  function stopPolling() {
    if (_timer.value) {
      clearInterval(_timer.value)
      _timer.value = null
    }
  }

  function startWatchdog() {
    if (_watchdog.value) return
    _watchdog.value = setInterval(() => {
      if (
        feedState.value === 'live' &&
        lastAppliedMetricsAt.value != null &&
        Date.now() - lastAppliedMetricsAt.value > STALE_MS
      ) {
        feedState.value = 'stale'
        // Resume REST immediately rather than waiting for the next poll tick.
        fetchSummaries()
      }
    }, WATCHDOG_INTERVAL_MS)
  }

  function stopWatchdog() {
    if (_watchdog.value) {
      clearInterval(_watchdog.value)
      _watchdog.value = null
    }
  }

  // --- WebSocket control ---
  function connectWS() {
    ws.connect()
  }

  function disconnectWS() {
    ws.disconnect()
  }

  function cleanup() {
    stopPolling()
    stopWatchdog()
    disconnectWS()
  }

  return {
    // State
    status,
    health,
    systemStatus,
    series,
    deviceMetrics,
    drift,
    notification,
    resolution,
    events,
    feedState,
    lastAppliedMetricsAt,
    lastInvalidReason,
    invalidMessageCount,

    // WebSocket state
    wsConnected,
    wsReconnectAttempts,
    lastMessageAt,

    // Getters
    isRealtimeActive,
    deviceStatusCounts,

    // Actions
    fetchStatus,
    fetchHealth,
    fetchSummaries,
    startPolling,
    stopPolling,
    connectWS,
    disconnectWS,
    cleanup,

    // Exposed for testing
    _ws: ws,
    _timer,
    _watchdog,
    handleWSMessage,
  }
})
