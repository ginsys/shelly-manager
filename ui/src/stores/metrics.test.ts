import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useMetricsStore } from './metrics'
import { getSystemMetrics, getDevicesMetrics, getDriftSummary } from '@/api/metrics'

// Backend REST /metrics/system returns a SystemStatus (counts, no cpu/mem/disk).
vi.mock('@/api/metrics', () => ({
  getMetricsStatus: vi.fn().mockResolvedValue({ enabled: true, uptime_seconds: 3600 }),
  getMetricsHealth: vi.fn().mockResolvedValue({ status: 'healthy' }),
  getSystemMetrics: vi.fn(),
  getDevicesMetrics: vi.fn(),
  getDriftSummary: vi.fn(),
}))

// Minimal WebSocket stub; these tests drive the store's message handler directly
// rather than through a live socket (the composable has its own tests).
class StubWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  readyState = StubWebSocket.CONNECTING
  onopen: ((e: Event) => void) | null = null
  onclose: ((e: CloseEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  constructor(public url: string) {}
  close() {}
  send() {}
}
global.WebSocket = StubWebSocket as any

// --- Backend-shaped fixtures ---

function systemStatusFixture(overrides: Record<string, unknown> = {}) {
  return {
    uptime_seconds: 3600,
    metrics_enabled: true,
    last_collection_time: '2026-01-01T00:00:00Z',
    total_devices: 10,
    online_devices: 7,
    devices_with_drift: 2,
    ...overrides,
  }
}

function dashboardFixture(overrides: Record<string, unknown> = {}) {
  return {
    system_status: systemStatusFixture(),
    device_metrics: [
      { id: '1', name: 'A', type: 'shelly1', status: 'online', config_synced: true, last_seen: '2026-01-01T00:00:00Z' },
      { id: '2', name: 'B', type: 'shelly1', status: 'offline', config_synced: false, last_seen: '2026-01-01T00:00:00Z' },
    ],
    drift_metrics: {
      total_drift_issues: 3,
      severity_distribution: { high: 1, low: 2 },
      category_distribution: {},
      trend_analysis: [],
    },
    notification_metrics: {
      total_sent: 0,
      total_failed: 0,
      channel_breakdown: {},
      alert_level_breakdown: {},
      average_latency_seconds: 0,
    },
    resolution_metrics: {
      total_resolutions: 0,
      auto_fix_success_rate: {},
      resolutions_by_category: {},
      average_review_time_seconds: 0,
    },
    ...overrides,
  }
}

function msg(type: string, data: unknown, timestamp = '2026-01-01T00:00:00Z') {
  return { type, timestamp, data }
}

describe('useMetricsStore', () => {
  let store: ReturnType<typeof useMetricsStore>

  beforeEach(() => {
    setActivePinia(createPinia())
    store = useMetricsStore()
    vi.mocked(getSystemMetrics).mockResolvedValue(systemStatusFixture({ total_devices: 5, online_devices: 3, devices_with_drift: 1 }))
    vi.mocked(getDevicesMetrics).mockResolvedValue({ devices: [] })
    vi.mocked(getDriftSummary).mockResolvedValue(dashboardFixture().drift_metrics)
  })

  afterEach(() => {
    store.cleanup()
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  describe('initial state', () => {
    it('is not live and not connected', () => {
      expect(store.feedState).toBe('idle')
      expect(store.isRealtimeActive).toBe(false)
      expect(store.wsConnected).toBe(false)
      expect(store.lastAppliedMetricsAt).toBe(null)
    })

    it('resets freshness on cleanup so a remount is not falsely live', () => {
      store.handleWSMessage(msg('metrics_update', dashboardFixture()))
      expect(store.feedState).toBe('live')

      store.cleanup()
      expect(store.feedState).toBe('idle')
      expect(store.lastAppliedMetricsAt).toBe(null)
    })
  })

  describe('snapshot hydration', () => {
    it('hydrates from initial_metrics and marks the feed live', () => {
      store.handleWSMessage(msg('initial_metrics', dashboardFixture()))

      expect(store.systemStatus?.online_devices).toBe(7)
      expect(store.deviceMetrics.length).toBe(2)
      expect(store.drift?.severity_distribution).toEqual({ high: 1, low: 2 })
      expect(store.series.online).toEqual([7])
      expect(store.series.total).toEqual([10])
      expect(store.feedState).toBe('live')
      expect(store.isRealtimeActive).toBe(true)
      expect(store.lastAppliedMetricsAt).not.toBe(null)
    })

    it('also hydrates from metrics_update', () => {
      store.handleWSMessage(msg('metrics_update', dashboardFixture({ system_status: systemStatusFixture({ online_devices: 4 }) })))
      expect(store.systemStatus?.online_devices).toBe(4)
      expect(store.isRealtimeActive).toBe(true)
    })

    it('derives device status distribution', () => {
      store.handleWSMessage(msg('initial_metrics', dashboardFixture()))
      expect(store.deviceStatusCounts).toEqual({ online: 1, offline: 1 })
    })
  })

  describe('invalid frames are surfaced, never applied', () => {
    it('rejects an unknown message type without marking the feed live', () => {
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      store.handleWSMessage(msg('totally_unknown', { anything: true }))

      expect(store.isRealtimeActive).toBe(false)
      expect(store.feedState).toBe('idle')
      expect(store.invalidMessageCount).toBe(1)
      expect(store.lastInvalidReason).toContain('unknown message type')
      expect(spy).toHaveBeenCalled()
      spy.mockRestore()
    })

    it('rejects a metrics message with a malformed payload', () => {
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      // Missing system_status counts.
      store.handleWSMessage(msg('metrics_update', { device_metrics: [], drift_metrics: {} }))

      expect(store.isRealtimeActive).toBe(false)
      expect(store.systemStatus).toBe(null)
      expect(store.invalidMessageCount).toBe(1)
      expect(store.lastInvalidReason).toContain('invalid dashboard payload')
      spy.mockRestore()
    })

    it('rejects a non-object frame', () => {
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      store.handleWSMessage('not an object')
      expect(store.invalidMessageCount).toBe(1)
      spy.mockRestore()
    })

    it('rejects a partial dashboard missing notification/resolution metrics', () => {
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const partial = dashboardFixture()
      delete (partial as Record<string, unknown>).notification_metrics
      store.handleWSMessage(msg('metrics_update', partial))

      expect(store.isRealtimeActive).toBe(false)
      expect(store.systemStatus).toBe(null)
      expect(store.invalidMessageCount).toBe(1)
      spy.mockRestore()
    })
  })

  describe('events are never dropped and never mark the feed live', () => {
    it('appends every event in order', () => {
      store.handleWSMessage(msg('alert', { alert_type: 'test', message: 'boom', severity: 'warning' }))
      store.handleWSMessage(msg('device_status_change', {
        device_id: '1', device_name: 'Living Room', old_status: 'online', new_status: 'offline',
      }))
      store.handleWSMessage(msg('drift_detected', {
        device_id: '1', device_name: 'Living Room', drift_count: 3, severity: 'high',
      }))

      expect(store.events.map((e) => e.kind)).toEqual(['alert', 'device_status_change', 'drift_detected'])
      expect(store.events[1].message).toBe('Living Room: online → offline')
      expect(store.events[1].severity).toBe('warning')
      expect(store.events[2].message).toContain('3 issues')
      // Events do not, by themselves, make the feed "live".
      expect(store.isRealtimeActive).toBe(false)
    })

    it('does not coalesce a burst of events', () => {
      for (let i = 0; i < 5; i++) {
        store.handleWSMessage(msg('alert', { alert_type: 'a', message: `m${i}`, severity: 'info' }))
      }
      expect(store.events.length).toBe(5)
      expect(store.events.map((e) => e.message)).toEqual(['m0', 'm1', 'm2', 'm3', 'm4'])
    })
  })

  describe('REST fallback and stale-REST protection', () => {
    it('polls until the first snapshot is applied, then pauses while live', async () => {
      vi.useFakeTimers()
      const spy = vi.mocked(getSystemMetrics)

      store.startPolling(1000)
      expect(spy).toHaveBeenCalledTimes(1) // immediate fetch on start

      store.handleWSMessage(msg('metrics_update', dashboardFixture()))
      expect(store.feedState).toBe('live')

      spy.mockClear()
      await vi.advanceTimersByTimeAsync(1100)
      expect(spy).not.toHaveBeenCalled() // poll tick skipped because live
    })

    it('discards a REST response when the feed is already live', async () => {
      store.handleWSMessage(msg('metrics_update', dashboardFixture()))
      expect(store.systemStatus?.online_devices).toBe(7)

      await store.fetchSummaries() // REST would report online_devices: 3
      expect(store.systemStatus?.online_devices).toBe(7) // unchanged
    })

    it('discards a REST response if a WS snapshot lands mid-flight', async () => {
      // While the REST request is in flight, a WS snapshot is applied.
      vi.mocked(getSystemMetrics).mockImplementationOnce(async () => {
        store.handleWSMessage(msg('metrics_update', dashboardFixture({ system_status: systemStatusFixture({ online_devices: 7 }) })))
        return systemStatusFixture({ online_devices: 999 })
      })

      await store.fetchSummaries()
      // The WS value (7) must win over the stale REST value (999).
      expect(store.systemStatus?.online_devices).toBe(7)
    })

    it('applies REST data when not live', async () => {
      await store.fetchSummaries()
      expect(store.systemStatus?.online_devices).toBe(3) // from REST fixture
      expect(store.isRealtimeActive).toBe(false) // REST never marks live
    })
  })

  describe('watchdog freshness', () => {
    it('transitions live -> stale when snapshots stop arriving', () => {
      vi.useFakeTimers()
      store.startPolling(60000) // also starts the watchdog
      store.handleWSMessage(msg('metrics_update', dashboardFixture()))
      expect(store.feedState).toBe('live')

      // Past STALE_MS (20s); watchdog ticks every 5s.
      vi.advanceTimersByTime(26000)
      expect(store.feedState).toBe('stale')
      expect(store.isRealtimeActive).toBe(false)
    })

    it('returns to live when snapshots resume', () => {
      vi.useFakeTimers()
      store.startPolling(60000)
      store.handleWSMessage(msg('metrics_update', dashboardFixture()))
      vi.advanceTimersByTime(26000)
      expect(store.feedState).toBe('stale')

      store.handleWSMessage(msg('metrics_update', dashboardFixture()))
      expect(store.feedState).toBe('live')
    })
  })

  describe('series ring buffer', () => {
    it('bounds the device-count series to maxLength', () => {
      for (let i = 0; i < 60; i++) {
        store.handleWSMessage(msg('metrics_update', dashboardFixture({
          system_status: systemStatusFixture({ online_devices: i }),
        }), `2026-01-01T00:${String(i).padStart(2, '0')}:00Z`))
      }
      expect(store.series.online.length).toBe(50)
      expect(store.series.online[49]).toBe(59)
    })
  })
})
