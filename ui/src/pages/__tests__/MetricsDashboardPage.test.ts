import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import MetricsDashboardPage from '../MetricsDashboardPage.vue'
import { useMetricsStore } from '@/stores/metrics'
import * as metricsApi from '@/api/metrics'
import { dismissNotice, notice } from '@/composables/useNotice'
import type { TestAlertResult } from '@/api/metrics'
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

  const mockTestAlert: TestAlertResult = {
    status: 'sent',
    alert_type: 'test',
    message: 'Test alert sent at 10:00:00',
    severity: 'info'
  }

  const panel = (name: string) => wrapper.find(`[data-testid="panel-${name}"]`)

  // Replaces the fixture mounted by beforeEach: the old component is unmounted first,
  // so its timers and store polling stop and cannot consume this test's mocks.
  function mountPage() {
    wrapper?.unmount()
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

      wrapper = mountPage()

      await flushPromises()

      expect(wrapper.text()).toContain(errorMessage)
    })

    it('clears error when retrying successful fetch', async () => {
      // First call fails
      vi.mocked(metricsApi.getDashboardSummary).mockRejectedValueOnce(new Error('Network error'))

      wrapper = mountPage()

      await flushPromises()
      expect(wrapper.text()).toContain('Network error')

      // Second call succeeds
      vi.mocked(metricsApi.getDashboardSummary).mockResolvedValue(mockDashboardSummary)
      await wrapper.vm.fetchAdvancedMetrics()
      await flushPromises()

      expect(wrapper.text()).not.toContain('Network error')
      expect(panel('summary').exists()).toBe(true)
    })

    it('renders the other two panels when one call fails', async () => {
      vi.mocked(metricsApi.getResolutionMetrics).mockRejectedValue(
        new Error('Request failed with status code 502')
      )

      wrapper = mountPage()
      await flushPromises()

      expect(panel('summary').exists()).toBe(true)
      expect(panel('notifications').exists()).toBe(true)
      expect(panel('resolution').exists()).toBe(false)
      expect(wrapper.text()).toContain('Resolution metrics: Request failed with status code 502')
    })

    it('names every failed panel and renders none when all three calls fail', async () => {
      vi.mocked(metricsApi.getDashboardSummary).mockRejectedValue(new Error('boom 1'))
      vi.mocked(metricsApi.getNotificationMetrics).mockRejectedValue(new Error('boom 2'))
      vi.mocked(metricsApi.getResolutionMetrics).mockRejectedValue(new Error('boom 3'))

      wrapper = mountPage()
      await flushPromises()

      for (const name of ['summary', 'notifications', 'resolution']) {
        expect(panel(name).exists()).toBe(false)
      }
      const text = wrapper.text()
      expect(text).toContain('Dashboard summary: boom 1')
      expect(text).toContain('Notification metrics: boom 2')
      expect(text).toContain('Resolution metrics: boom 3')
    })

    it("shows the server's own error message, not the bare axios text", async () => {
      // What axios rejects with for a 401 carrying the backend error envelope.
      const unauthorized = Object.assign(new Error('Request failed with status code 401'), {
        response: {
          status: 401,
          data: { success: false, error: { code: 'UNAUTHORIZED', message: 'Admin authorization required' } }
        }
      })
      vi.mocked(metricsApi.getNotificationMetrics).mockRejectedValue(unauthorized)

      wrapper = mountPage()
      await flushPromises()

      const text = wrapper.text()
      expect(text).toContain('Notification metrics: Admin authorization required')
      expect(text).not.toContain('status code 401')
    })

    it.each([
      ['summary', 'getDashboardSummary'],
      ['notifications', 'getNotificationMetrics'],
      ['resolution', 'getResolutionMetrics']
    ] as const)('empties the %s panel when its refresh fails instead of keeping stale numbers', async (name, call) => {
      await flushPromises()
      expect(panel(name).exists()).toBe(true)

      vi.mocked(metricsApi[call]).mockRejectedValue(new Error('gone'))
      await wrapper.vm.fetchAdvancedMetrics()
      await flushPromises()

      expect(panel(name).exists()).toBe(false)
      const others = ['summary', 'notifications', 'resolution'].filter(n => n !== name)
      for (const other of others) expect(panel(other).exists()).toBe(true)
    })

    it('fills the panel again once its call succeeds', async () => {
      vi.mocked(metricsApi.getResolutionMetrics).mockRejectedValueOnce(new Error('gone'))

      wrapper = mountPage()
      await flushPromises()
      expect(panel('resolution').exists()).toBe(false)

      await wrapper.vm.fetchAdvancedMetrics()
      await flushPromises()

      expect(panel('resolution').exists()).toBe(true)
      expect(wrapper.text()).not.toContain('gone')
    })
  })

  // ginsys/shelly-manager#876: the security monitor is removed, so the page shows no panel for it
  // (the mocked client module above has no security call, so a page that asked for one would fail).
  describe('No security metrics', () => {
    it('shows no security panel', async () => {
      await flushPromises()

      expect(panel('security').exists()).toBe(false)
      expect(panel('security-off').exists()).toBe(false)
      expect(wrapper.text()).not.toContain('Security Metrics')
      expect(wrapper.text()).not.toContain('Security metrics:')
      // The other three panels are unchanged.
      for (const name of ['summary', 'notifications', 'resolution']) {
        expect(panel(name).exists()).toBe(true)
      }
    })
  })

  // ginsys/shelly-manager#614: the controls show their outcome on the page, never in a browser alert.
  describe('Collection Controls', () => {
    let alertSpy: ReturnType<typeof vi.spyOn>

    beforeEach(async () => {
      await flushPromises()
      alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    })

    afterEach(() => {
      alertSpy.mockRestore()
    })

    const clickButton = async (label: string) => {
      const button = wrapper.findAll('button').find((b: any) => b.text().includes(label))
      await button?.trigger('click')
      await flushPromises()
    }
    const status = () => wrapper.find('[data-testid="metrics-status"]')
    const actionError = () => wrapper.find('[data-testid="metrics-action-error"]')

    it('enables metrics collection', async () => {
      await clickButton('Enable Collection')

      expect(metricsApi.enableMetrics).toHaveBeenCalled()
      expect(status().text()).toBe('Metrics collection enabled')
      expect(actionError().exists()).toBe(false)
      expect(alertSpy).not.toHaveBeenCalled()
    })

    it('disables metrics collection', async () => {
      await clickButton('Disable Collection')

      expect(metricsApi.disableMetrics).toHaveBeenCalled()
      expect(status().text()).toBe('Metrics collection disabled')
      expect(alertSpy).not.toHaveBeenCalled()
    })

    // ginsys/shelly-manager#701: the store swallows a failed status read, so the page asks for it.
    it.each([
      ['Enable Collection', 'Metrics collection enabled'],
      ['Disable Collection', 'Metrics collection disabled']
    ] as const)('says when "%s" worked but the status could not be refreshed', async (label, text) => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      vi.mocked(metricsApi.getMetricsStatus).mockRejectedValueOnce(new Error('status down'))

      await clickButton(label)

      expect(status().text()).toBe(`${text}, but its status could not be refreshed`)
      expect(actionError().exists()).toBe(false)
      warn.mockRestore()
    })

    it('shows the failed advanced metrics in the app notice when the page was left', async () => {
      dismissNotice()
      let failRefresh!: () => void
      vi.mocked(metricsApi.getDashboardSummary).mockImplementationOnce(
        () => new Promise((_, reject) => { failRefresh = () => reject(new Error('summary down')) }) as any
      )
      wrapper = mountPage()
      await flushPromises()

      wrapper.unmount()
      failRefresh()
      await flushPromises()

      expect(notice.value?.message).toContain('summary down')
      dismissNotice()
    })

    it('triggers manual collection', async () => {
      await clickButton('Trigger Collection')

      expect(metricsApi.collectMetrics).toHaveBeenCalled()
      expect(status().text()).toBe('Metrics collection triggered')
      expect(alertSpy).not.toHaveBeenCalled()
    })

    it('sends test alert', async () => {
      await clickButton('Send Test Alert')

      expect(metricsApi.sendTestAlert).toHaveBeenCalled()
      expect(status().text()).toBe('Test alert sent: Test alert sent at 10:00:00')
      expect(actionError().exists()).toBe(false)
      expect(alertSpy).not.toHaveBeenCalled()
    })

    it('reports a failure when the answer is not "sent"', async () => {
      vi.mocked(metricsApi.sendTestAlert).mockResolvedValue({
        ...mockTestAlert,
        status: 'error',
        message: 'No channel'
      })

      await clickButton('Send Test Alert')

      expect(actionError().text()).toContain('Test alert failed: No channel')
      expect(status().exists()).toBe(false)
      expect(alertSpy).not.toHaveBeenCalled()
    })

    it.each([
      ['Enable Collection', 'enableMetrics', 'Enable metrics'],
      ['Disable Collection', 'disableMetrics', 'Disable metrics'],
      ['Trigger Collection', 'collectMetrics', 'Trigger collection'],
      ['Send Test Alert', 'sendTestAlert', 'Send test alert']
    ] as const)('shows the failure of "%s" in a panel and no success line', async (label, api, action) => {
      vi.mocked(metricsApi[api]).mockRejectedValueOnce(new Error('backend said no'))

      await clickButton(label)

      expect(actionError().text()).toContain('backend said no')
      expect(actionError().text()).toContain(action)
      expect(status().exists()).toBe(false)
      expect(alertSpy).not.toHaveBeenCalled()
    })

    it('replaces an earlier outcome with the next one', async () => {
      vi.mocked(metricsApi.enableMetrics).mockRejectedValueOnce(new Error('backend said no'))
      await clickButton('Enable Collection')
      expect(actionError().exists()).toBe(true)

      await clickButton('Disable Collection')

      expect(actionError().exists()).toBe(false)
      expect(status().text()).toBe('Metrics collection disabled')
    })

    // Review round 4: the status refresh of Enable can settle after Disable has finished.
    it('keeps the outcome of the newest control when an older one settles later', async () => {
      let finishRefresh!: () => void
      vi.spyOn(store, 'fetchStatus').mockImplementationOnce(
        () => new Promise<boolean>(resolve => { finishRefresh = () => resolve(true) }) as any
      )
      await clickButton('Enable Collection')
      await clickButton('Disable Collection')
      expect(status().text()).toBe('Metrics collection disabled')

      finishRefresh()
      await flushPromises()

      expect(status().text()).toBe('Metrics collection disabled')
    })

    // Review round 5: a control that starts removes the outcome of the one before it.
    it('removes the earlier success while the next control is running', async () => {
      await clickButton('Enable Collection')
      expect(status().text()).toBe('Metrics collection enabled')
      vi.mocked(metricsApi.disableMetrics).mockImplementationOnce(() => new Promise<void>(() => {}) as any)

      await clickButton('Disable Collection')

      expect(status().exists()).toBe(false)
    })

    it('removes the earlier failure while the next control is running', async () => {
      vi.mocked(metricsApi.enableMetrics).mockRejectedValueOnce(new Error('backend said no'))
      await clickButton('Enable Collection')
      expect(actionError().exists()).toBe(true)
      vi.mocked(metricsApi.enableMetrics).mockImplementationOnce(() => new Promise<void>(() => {}) as any)

      await clickButton('Enable Collection')

      expect(actionError().exists()).toBe(false)
    })

    it('still shows the failure of an older control that settles after a newer success', async () => {
      let failEnable!: (e: Error) => void
      vi.mocked(metricsApi.enableMetrics).mockImplementationOnce(
        () => new Promise<void>((_resolve, reject) => { failEnable = reject }) as any
      )
      await clickButton('Enable Collection')
      await clickButton('Disable Collection')

      failEnable(new Error('backend said no'))
      await flushPromises()

      expect(actionError().text()).toContain('backend said no')
    })

    it('removes the failure panel with its dismiss button', async () => {
      vi.mocked(metricsApi.enableMetrics).mockRejectedValueOnce(new Error('backend said no'))
      await clickButton('Enable Collection')

      await wrapper.get('[data-testid="metrics-action-error"] .dismiss-button').trigger('click')

      expect(actionError().exists()).toBe(false)
    })
  })

  describe('Prometheus Export', () => {
    let alertSpy: ReturnType<typeof vi.spyOn>
    // The shared setup's createElement. A test that replaces it gives it back in afterEach.
    let sharedCreateElement: ((...args: any[]) => any) | undefined

    beforeEach(() => {
      alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
      sharedCreateElement = vi.mocked(document.createElement).getMockImplementation()
      global.URL.createObjectURL = vi.fn(() => 'blob:mock-url')
      global.URL.revokeObjectURL = vi.fn()
    })

    afterEach(() => {
      alertSpy.mockRestore()
      if (sharedCreateElement) vi.mocked(document.createElement).mockImplementation(sharedCreateElement)
    })

    const clickExport = async () => {
      const exportButton = wrapper.findAll('button').find((b: any) =>
        b.text().includes('Export Prometheus')
      )
      await exportButton?.trigger('click')
      await flushPromises()
    }

    it('shows the failure of the export in a panel', async () => {
      vi.mocked(metricsApi.getPrometheusMetrics).mockRejectedValueOnce(new Error('no metrics yet'))

      await clickExport()

      const panel = wrapper.get('[data-testid="metrics-action-error"]')
      expect(panel.text()).toContain('no metrics yet')
      expect(panel.text()).toContain('Export Prometheus metrics')
      expect(alertSpy).not.toHaveBeenCalled()
    })

    it('exports Prometheus metrics and downloads file', async () => {
      const mockClick = vi.fn()
      const mockCreateElement = vi.spyOn(document, 'createElement')
      mockCreateElement.mockReturnValue({
        click: mockClick,
        href: '',
        download: ''
      } as any)

      await clickExport()

      expect(metricsApi.getPrometheusMetrics).toHaveBeenCalled()
      expect(mockClick).toHaveBeenCalled()
      expect(global.URL.revokeObjectURL).toHaveBeenCalled()
    })

    // ginsys/shelly-manager#696: the test above replaces document.createElement for every tag. The
    // next test, and the page's own timer, need a createElement that still builds real elements.
    it('leaves document.createElement building real elements', () => {
      const div = document.createElement('div')
      expect(div).toBeInstanceOf(HTMLElement)
      expect(typeof div.setAttribute).toBe('function')
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
