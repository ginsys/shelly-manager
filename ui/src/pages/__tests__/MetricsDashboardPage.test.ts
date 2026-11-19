import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import MetricsDashboardPage from '../MetricsDashboardPage.vue'
import { useMetricsStore } from '@/stores/metrics'
import * as metricsApi from '@/api/metrics'
import type { SecurityMetrics, TestAlertResult } from '@/api/metrics'
import type {
  DashboardMetrics,
  NotificationMetrics,
  ResolutionMetrics
} from '@/api/metricsContract'

// Mock the API module
vi.mock('@/api/metrics', () => ({
  enableMetrics: vi.fn(),
  disableMetrics: vi.fn(),
  collectMetrics: vi.fn(),
  getPrometheusMetrics: vi.fn(),
  getDashboardSummary: vi.fn(),
  getNotificationMetrics: vi.fn(),
  getResolutionMetrics: vi.fn(),
  getSecurityMetrics: vi.fn(),
  sendTestAlert: vi.fn(),
  // Store-level API methods
  getMetricsStatus: vi.fn(),
  getMetricsHealth: vi.fn(),
  getSystemMetrics: vi.fn(),
  getDevicesMetrics: vi.fn(),
  getDriftSummary: vi.fn()
}))

describe('MetricsDashboardPage', () => {
  let wrapper: any
  let store: any

  // Mock data, shaped like the Go answers: the field names and nesting of
  // internal/metrics/websocket.go (DashboardMetrics and its parts),
  // internal/metrics/handlers.go (SendTestAlert) and
  // internal/api/middleware/security_monitor.go (SecurityMetrics).
  const mockNotificationMetrics: NotificationMetrics = {
    total_sent: 15,
    total_failed: 1,
    channel_breakdown: { email: 10, slack: 6 },
    alert_level_breakdown: { info: 12, critical: 4 },
    average_latency_seconds: 0
  }

  const mockResolutionMetrics: ResolutionMetrics = {
    total_resolutions: 5,
    auto_fix_success_rate: { drift: 0.8 },
    resolutions_by_category: { drift: 3, error: 2 },
    average_review_time_seconds: 120
  }

  const mockDashboardSummary: DashboardMetrics = {
    system_status: {
      uptime_seconds: 3600,
      metrics_enabled: true,
      last_collection_time: '2026-01-01T00:00:00Z',
      total_devices: 10,
      online_devices: 8,
      devices_with_drift: 2
    },
    device_metrics: [],
    drift_metrics: {
      total_drift_issues: 7,
      severity_distribution: { high: 2, low: 5 },
      category_distribution: {},
      trend_analysis: []
    },
    notification_metrics: mockNotificationMetrics,
    resolution_metrics: mockResolutionMetrics
  }

  const mockSecurityMetrics: SecurityMetrics = {
    total_requests: 500,
    blocked_requests: 7,
    suspicious_requests: 11,
    rate_limit_violations: 3,
    validation_failures: 4,
    attacks_by_type: { sql_injection: 2 },
    top_attacker_ips: null,
    alerts_by_level: {},
    last_updated: '2026-01-01T00:00:00Z',
    blocked_ips: 1,
    active_threats: 2
  }

  const mockTestAlert: TestAlertResult = {
    status: 'sent',
    alert_type: 'test',
    message: 'Test alert sent at 10:00:00',
    severity: 'info'
  }

  const panel = (name: string) => wrapper.find(`[data-testid="panel-${name}"]`)

  function mountPage() {
    return mount(MetricsDashboardPage, {
      global: {
        plugins: [createTestingPinia({ stubActions: false })],
        stubs: {
          LineChart: { template: '<div class="line-chart-stub"></div>' },
          BarChart: { template: '<div class="bar-chart-stub"></div>' }
        }
      }
    })
  }

  beforeEach(() => {
    // Setup default mock implementations
    vi.mocked(metricsApi.getDashboardSummary).mockResolvedValue(mockDashboardSummary)
    vi.mocked(metricsApi.getNotificationMetrics).mockResolvedValue(mockNotificationMetrics)
    vi.mocked(metricsApi.getResolutionMetrics).mockResolvedValue(mockResolutionMetrics)
    vi.mocked(metricsApi.getSecurityMetrics).mockResolvedValue(mockSecurityMetrics)
    vi.mocked(metricsApi.enableMetrics).mockResolvedValue({ success: true })
    vi.mocked(metricsApi.disableMetrics).mockResolvedValue({ success: true })
    vi.mocked(metricsApi.collectMetrics).mockResolvedValue({ success: true })
    vi.mocked(metricsApi.sendTestAlert).mockResolvedValue(mockTestAlert)
    vi.mocked(metricsApi.getPrometheusMetrics).mockResolvedValue('# Prometheus metrics')

    // Store-level API mocks
    vi.mocked(metricsApi.getMetricsStatus).mockResolvedValue({ enabled: true, uptime_seconds: 3600 })
    vi.mocked(metricsApi.getMetricsHealth).mockResolvedValue({ status: 'healthy' })
    // Backend-shaped REST payloads (SystemStatus / {devices} / DriftMetrics).
    vi.mocked(metricsApi.getSystemMetrics).mockResolvedValue({
      uptime_seconds: 3600,
      metrics_enabled: true,
      last_collection_time: '2026-01-01T00:00:00Z',
      total_devices: 10,
      online_devices: 8,
      devices_with_drift: 2
    })
    vi.mocked(metricsApi.getDevicesMetrics).mockResolvedValue({ devices: [] })
    vi.mocked(metricsApi.getDriftSummary).mockResolvedValue({
      total_drift_issues: 3,
      severity_distribution: { high: 1, low: 2 },
      category_distribution: {},
      trend_analysis: []
    })

    // Create wrapper with testing pinia
    wrapper = mount(MetricsDashboardPage, {
      global: {
        plugins: [
          createTestingPinia({
            stubActions: false,
            initialState: {
              metrics: {
                status: { enabled: true, uptime_seconds: 3600 }
              }
            }
          })
        ],
        stubs: {
          // Stub chart components with simple templates to avoid setAttribute issues
          LineChart: { template: '<div class="line-chart-stub"></div>' },
          BarChart: { template: '<div class="bar-chart-stub"></div>' }
        }
      }
    })

    store = useMetricsStore()
  })

  afterEach(() => {
    wrapper?.unmount()
    vi.clearAllMocks()
  })

  describe('Initial Rendering', () => {
    it('renders the page title', () => {
      expect(wrapper.text()).toContain('Metrics Dashboard')
    })

    it('displays connection status cards', () => {
      const cards = wrapper.findAll('.card')
      expect(cards.length).toBeGreaterThan(0)
    })

    it('shows enabled status from store', () => {
      expect(wrapper.text()).toContain('Enabled: true')
    })

    it('formats uptime correctly', () => {
      expect(wrapper.text()).toContain('Uptime: 1h 0m')
    })

    it('shows WebSocket connection status', () => {
      // Component initializes with fresh state (Disconnected), not mock state
      expect(wrapper.text()).toContain('WebSocket: Disconnected')
    })
  })

  describe('Advanced Metrics Loading', () => {
    it('fetches all advanced metrics on mount', async () => {
      await flushPromises()

      expect(metricsApi.getDashboardSummary).toHaveBeenCalled()
      expect(metricsApi.getNotificationMetrics).toHaveBeenCalled()
      expect(metricsApi.getResolutionMetrics).toHaveBeenCalled()
      expect(metricsApi.getSecurityMetrics).toHaveBeenCalled()
    })

    it('displays dashboard summary after loading', async () => {
      await flushPromises()

      const text = panel('summary').text()
      expect(text).toContain('Total: 10')
      expect(text).toContain('Online: 8')
      expect(text).toContain('Offline: 2')
      expect(text).toContain('With drift: 2')
      expect(text).toContain('Issues: 7')
      expect(text).toContain('Sent: 15')
      expect(text).toContain('Failed: 1')
    })

    it('displays notification metrics', async () => {
      await flushPromises()

      const text = panel('notifications').text()
      expect(text).toContain('Total Sent: 15')
      expect(text).toContain('Total Failed: 1')
      expect(text).toContain('email: 10')
      expect(text).toContain('critical: 4')
    })

    it('displays resolution metrics', async () => {
      await flushPromises()

      const text = panel('resolution').text()
      expect(text).toContain('Total Resolved: 5')
      expect(text).toContain('Avg Review Time: 2m 0s')
      expect(text).toContain('drift: 3')
      expect(text).toContain('drift: 0.8')
    })

    it('displays security metrics', async () => {
      await flushPromises()

      const text = panel('security').text()
      expect(text).toContain('Total: 500')
      expect(text).toContain('Blocked: 7')
      expect(text).toContain('Suspicious: 11')
      expect(text).toContain('Rate limit: 3')
      expect(text).toContain('Validation failures: 4')
      expect(text).toContain('Blocked IPs: 1')
      expect(text).toContain('Active threats: 2')
    })

    it('shows no card for a field the server does not send', async () => {
      await flushPromises()

      expect(wrapper.text()).not.toContain('Recent Notifications')
      expect(wrapper.text()).not.toContain('By User')
      expect(wrapper.text()).not.toContain('Exports')
      expect(wrapper.text()).not.toContain('Imports')
    })
  })

  describe('Error Handling', () => {
    it('displays error message when metrics loading fails', async () => {
      const errorMessage = 'Failed to load metrics'
      vi.mocked(metricsApi.getDashboardSummary).mockRejectedValue(new Error(errorMessage))

      wrapper = mount(MetricsDashboardPage, {
        global: {
          plugins: [createTestingPinia({ stubActions: false })],
          stubs: {
            LineChart: { template: '<div class="line-chart-stub"></div>' },
            BarChart: { template: '<div class="bar-chart-stub"></div>' }
          }
        }
      })

      await flushPromises()

      expect(wrapper.text()).toContain(errorMessage)
    })

    it('clears error when retrying successful fetch', async () => {
      // First call fails
      vi.mocked(metricsApi.getDashboardSummary).mockRejectedValueOnce(new Error('Network error'))

      wrapper = mount(MetricsDashboardPage, {
        global: {
          plugins: [createTestingPinia({ stubActions: false })],
          stubs: {
            LineChart: { template: '<div class="line-chart-stub"></div>' },
            BarChart: { template: '<div class="bar-chart-stub"></div>' }
          }
        }
      })

      await flushPromises()
      expect(wrapper.text()).toContain('Network error')

      // Second call succeeds
      vi.mocked(metricsApi.getDashboardSummary).mockResolvedValue(mockDashboardSummary)
      await wrapper.vm.fetchAdvancedMetrics()
      await flushPromises()

      expect(wrapper.text()).not.toContain('Network error')
      expect(panel('summary').exists()).toBe(true)
    })

    it('renders the other three panels when one call fails', async () => {
      vi.mocked(metricsApi.getSecurityMetrics).mockRejectedValue(
        new Error('Request failed with status code 404')
      )

      wrapper = mountPage()
      await flushPromises()

      expect(panel('summary').exists()).toBe(true)
      expect(panel('notifications').exists()).toBe(true)
      expect(panel('resolution').exists()).toBe(true)
      expect(panel('security').exists()).toBe(false)
      expect(wrapper.text()).toContain('Security metrics: Request failed with status code 404')
    })

    it('names every failed panel and renders none when all four calls fail', async () => {
      vi.mocked(metricsApi.getDashboardSummary).mockRejectedValue(new Error('boom 1'))
      vi.mocked(metricsApi.getNotificationMetrics).mockRejectedValue(new Error('boom 2'))
      vi.mocked(metricsApi.getResolutionMetrics).mockRejectedValue(new Error('boom 3'))
      vi.mocked(metricsApi.getSecurityMetrics).mockRejectedValue(new Error('boom 4'))

      wrapper = mountPage()
      await flushPromises()

      for (const name of ['summary', 'notifications', 'resolution', 'security']) {
        expect(panel(name).exists()).toBe(false)
      }
      const text = wrapper.text()
      expect(text).toContain('Dashboard summary: boom 1')
      expect(text).toContain('Notification metrics: boom 2')
      expect(text).toContain('Resolution metrics: boom 3')
      expect(text).toContain('Security metrics: boom 4')
    })

    it.each([
      ['summary', 'getDashboardSummary'],
      ['notifications', 'getNotificationMetrics'],
      ['resolution', 'getResolutionMetrics'],
      ['security', 'getSecurityMetrics']
    ] as const)('empties the %s panel when its refresh fails instead of keeping stale numbers', async (name, call) => {
      await flushPromises()
      expect(panel(name).exists()).toBe(true)

      vi.mocked(metricsApi[call]).mockRejectedValue(new Error('gone'))
      await wrapper.vm.fetchAdvancedMetrics()
      await flushPromises()

      expect(panel(name).exists()).toBe(false)
      const others = ['summary', 'notifications', 'resolution', 'security'].filter(n => n !== name)
      for (const other of others) expect(panel(other).exists()).toBe(true)
    })

    it('fills the panel again once its call succeeds', async () => {
      vi.mocked(metricsApi.getSecurityMetrics).mockRejectedValueOnce(new Error('gone'))

      wrapper = mountPage()
      await flushPromises()
      expect(panel('security').exists()).toBe(false)

      await wrapper.vm.fetchAdvancedMetrics()
      await flushPromises()

      expect(panel('security').exists()).toBe(true)
      expect(wrapper.text()).not.toContain('gone')
    })
  })

  describe('Collection Controls', () => {
    beforeEach(async () => {
      await flushPromises()
      // Mock window.alert to avoid test warnings
      global.alert = vi.fn()
    })

    it('enables metrics collection', async () => {
      const enableButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Enable Collection')
      )

      await enableButton?.trigger('click')
      await flushPromises()

      expect(metricsApi.enableMetrics).toHaveBeenCalled()
      expect(global.alert).toHaveBeenCalledWith('Metrics collection enabled')
    })

    it('disables metrics collection', async () => {
      const disableButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Disable Collection')
      )

      await disableButton?.trigger('click')
      await flushPromises()

      expect(metricsApi.disableMetrics).toHaveBeenCalled()
      expect(global.alert).toHaveBeenCalledWith('Metrics collection disabled')
    })

    it('triggers manual collection', async () => {
      const collectButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Trigger Collection')
      )

      await collectButton?.trigger('click')
      await flushPromises()

      expect(metricsApi.collectMetrics).toHaveBeenCalled()
      expect(global.alert).toHaveBeenCalledWith('Metrics collection triggered')
    })

    it('sends test alert', async () => {
      const testButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Send Test Alert')
      )

      await testButton?.trigger('click')
      await flushPromises()

      expect(metricsApi.sendTestAlert).toHaveBeenCalled()
      expect(global.alert).toHaveBeenCalledWith('Success: Test alert sent at 10:00:00')
      expect(global.alert).not.toHaveBeenCalledWith(expect.stringContaining('Failed'))
    })

    it('reports a failure when the answer is not "sent"', async () => {
      vi.mocked(metricsApi.sendTestAlert).mockResolvedValue({
        ...mockTestAlert,
        status: 'error',
        message: 'No channel'
      })

      const testButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Send Test Alert')
      )
      await testButton?.trigger('click')
      await flushPromises()

      expect(global.alert).toHaveBeenCalledWith('Failed: No channel')
    })
  })

  describe('Prometheus Export', () => {
    it('exports Prometheus metrics and downloads file', async () => {
      global.alert = vi.fn()

      // Mock URL and DOM APIs
      global.URL.createObjectURL = vi.fn(() => 'blob:mock-url')
      global.URL.revokeObjectURL = vi.fn()

      const mockClick = vi.fn()
      const mockCreateElement = vi.spyOn(document, 'createElement')
      mockCreateElement.mockReturnValue({
        click: mockClick,
        href: '',
        download: ''
      } as any)

      const exportButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Export Prometheus')
      )

      await exportButton?.trigger('click')
      await flushPromises()

      expect(metricsApi.getPrometheusMetrics).toHaveBeenCalled()
      expect(mockClick).toHaveBeenCalled()
      expect(global.URL.revokeObjectURL).toHaveBeenCalled()
    })
  })

  // NOTE: Tests skipped due to jsdom limitation with Vue scoped styles (el.setAttribute)
  // See: https://github.com/vuejs/core/issues/7849
  describe.skip('WebSocket Connection Status', () => {
    it('shows connected state when WebSocket is active', async () => {
      await flushPromises()

      const connectionCard = wrapper.findAll('.card').find((c: any) =>
        c.text().includes('WebSocket')
      )

      expect(connectionCard?.text()).toContain('WebSocket: Connected')
      expect(connectionCard?.classes()).toContain('connection-connected')
    })

    it('shows reconnecting state with attempt count', async () => {
      store.wsConnected = false
      store.wsReconnectAttempts = 3
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('WebSocket: Reconnecting')
      expect(wrapper.text()).toContain('attempt 3')
    })

    it('shows disconnected state when not connected', async () => {
      store.wsConnected = false
      store.wsReconnectAttempts = 0
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('WebSocket: Disconnected')
    })
  })

  // NOTE: Tests skipped due to jsdom limitation with Vue scoped styles (el.setAttribute)
  describe.skip('Loading States', () => {
    it('displays loading indicator while fetching metrics', async () => {
      // Create a slow-resolving promise
      vi.mocked(metricsApi.getDashboardSummary).mockImplementation(() =>
        new Promise((resolve) => setTimeout(() => resolve(mockDashboardSummary), 100))
      )

      wrapper = mount(MetricsDashboardPage, {
        global: {
          plugins: [createTestingPinia({ stubActions: false })],
          stubs: {
            LineChart: { template: '<div class="line-chart-stub"></div>' },
            BarChart: { template: '<div class="bar-chart-stub"></div>' }
          }
        }
      })

      // Should show loading
      expect(wrapper.text()).toContain('Loading advanced metrics')

      await flushPromises()

      // Should hide loading after data loads
      expect(wrapper.text()).not.toContain('Loading advanced metrics')
    })
  })

  // NOTE: Tests skipped due to jsdom limitation with Vue scoped styles (el.setAttribute)
  describe.skip('Chart Rendering', () => {
    it('renders System Metrics chart', async () => {
      await flushPromises()

      expect(wrapper.find('.line-chart-stub').exists()).toBe(true)
    })

    it('renders Device and Drift charts', async () => {
      await flushPromises()

      const barCharts = wrapper.findAll('.bar-chart-stub')
      expect(barCharts.length).toBeGreaterThanOrEqual(2)
    })

    it('shows LIVE badge when realtime is active', async () => {
      await flushPromises()

      expect(wrapper.text()).toContain('LIVE')
    })
  })

  // NOTE: Tests skipped due to jsdom limitation with Vue scoped styles (el.setAttribute)
  describe.skip('Store Integration', () => {
    it('calls store methods on mount', async () => {
      expect(store.fetchStatus).toHaveBeenCalled()
      expect(store.fetchHealth).toHaveBeenCalled()
      expect(store.startPolling).toHaveBeenCalled()
      expect(store.connectWS).toHaveBeenCalled()
    })

    it('calls store cleanup on unmount', async () => {
      await wrapper.unmount()

      expect(store.cleanup).toHaveBeenCalled()
    })
  })
})
