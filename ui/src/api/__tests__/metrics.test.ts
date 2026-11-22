import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client
vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn()
    }
  }
})

import api from '../client'
import * as metricsModule from '../metrics'
import {
  getPrometheusMetrics,
  enableMetrics,
  disableMetrics,
  collectMetrics,
  getDashboardSummary,
  sendTestAlert,
  getNotificationMetrics,
  getResolutionMetrics,
  type MetricsStatus,
  type TestAlertResult
} from '../metrics'
import type {
  DashboardMetrics,
  NotificationMetrics,
  ResolutionMetrics
} from '../metricsContract'

// The fixtures below copy the Go answers (the field names and nesting of
// internal/metrics/websocket.go and internal/metrics/handlers.go), typed as the
// contract types so vue-tsc rejects a field the server does not send.

describe('Advanced Metrics API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getPrometheusMetrics', () => {
    it('returns Prometheus-formatted metrics', async () => {
      const prometheusData = `# HELP shelly_devices_total Total number of devices
# TYPE shelly_devices_total gauge
shelly_devices_total 42
# HELP shelly_exports_total Total exports
# TYPE shelly_exports_total counter
shelly_exports_total 123`
      ;(api.get as any).mockResolvedValue({
        data: prometheusData
      })

      const result = await getPrometheusMetrics()

      expect(result).toBe(prometheusData)
      expect(api.get).toHaveBeenCalledWith('/metrics/prometheus', { responseType: 'text' })
    })

    it('throws error when response is not a string', async () => {
      ;(api.get as any).mockResolvedValue({
        data: { error: 'Invalid response' }
      })

      await expect(getPrometheusMetrics()).rejects.toThrow('Failed to load Prometheus metrics')
    })
  })

  describe('enableMetrics', () => {
    it('enables metrics collection', async () => {
      const status: MetricsStatus = {
        enabled: true,
        last_collection_time: '2023-01-01T00:00:00Z',
        uptime_seconds: 3600
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: status, timestamp: new Date().toISOString() }
      })

      const result = await enableMetrics()

      expect(result).toEqual(status)
      expect(api.post).toHaveBeenCalledWith('/metrics/enable')
    })

    it('throws error when enable fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Already enabled', code: 'ALREADY_ENABLED' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(enableMetrics()).rejects.toThrow('Already enabled')
    })

    it('throws default error when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(enableMetrics()).rejects.toThrow('Failed to enable metrics collection')
    })
  })

  describe('disableMetrics', () => {
    it('disables metrics collection', async () => {
      const status: MetricsStatus = {
        enabled: false,
        last_collection_time: '2023-01-01T00:00:00Z',
        uptime_seconds: 3600
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: status, timestamp: new Date().toISOString() }
      })

      const result = await disableMetrics()

      expect(result).toEqual(status)
      expect(result.enabled).toBe(false)
      expect(api.post).toHaveBeenCalledWith('/metrics/disable')
    })

    it('throws error when disable fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Already disabled', code: 'ALREADY_DISABLED' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(disableMetrics()).rejects.toThrow('Already disabled')
    })
  })

  describe('collectMetrics', () => {
    it('triggers metrics collection', async () => {
      const status: MetricsStatus = {
        enabled: true,
        last_collection_time: '2023-01-01T00:05:00Z',
        uptime_seconds: 3900
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: status, timestamp: new Date().toISOString() }
      })

      const result = await collectMetrics()

      expect(result).toEqual(status)
      expect(api.post).toHaveBeenCalledWith('/metrics/collect')
    })

    it('throws error when collection fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Collection disabled', code: 'DISABLED' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(collectMetrics()).rejects.toThrow('Collection disabled')
    })
  })

  describe('getDashboardSummary', () => {
    it('returns dashboard summary', async () => {
      const summary: DashboardMetrics = {
        system_status: {
          uptime_seconds: 7200,
          metrics_enabled: true,
          last_collection_time: '2023-01-01T00:00:00Z',
          total_devices: 100,
          online_devices: 85,
          devices_with_drift: 8
        },
        device_metrics: [
          {
            id: '1',
            name: 'Kitchen',
            type: 'SHSW-1',
            status: 'online',
            config_synced: true,
            last_seen: '2023-01-01T00:00:00Z'
          }
        ],
        drift_metrics: {
          total_drift_issues: 30,
          severity_distribution: { high: 10, low: 20 },
          category_distribution: { wifi: 30 },
          trend_analysis: []
        },
        notification_metrics: {
          total_sent: 500,
          total_failed: 12,
          channel_breakdown: { email: 500 },
          alert_level_breakdown: { info: 500 },
          average_latency_seconds: 0
        },
        resolution_metrics: {
          total_resolutions: 4,
          auto_fix_success_rate: {},
          resolutions_by_category: { drift: 4 },
          average_review_time_seconds: 0
        }
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: summary, timestamp: new Date().toISOString() }
      })

      const result = await getDashboardSummary()

      expect(result).toEqual(summary)
      expect(result.system_status.total_devices).toBe(100)
      expect(result.system_status.online_devices).toBe(85)
      expect(api.get).toHaveBeenCalledWith('/metrics/dashboard')
    })

    it('throws error when summary fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Database error', code: 'DB_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getDashboardSummary()).rejects.toThrow('Database error')
    })
  })

  describe('sendTestAlert', () => {
    it('sends test alert successfully', async () => {
      // What SendTestAlert in handlers.go answers: status "sent" inside the envelope.
      const result: TestAlertResult = {
        status: 'sent',
        alert_type: 'test',
        message: 'Test alert sent at 10:00:00',
        severity: 'info'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await sendTestAlert()

      expect(response).toEqual(result)
      expect(response.status).toBe('sent')
      expect(api.post).toHaveBeenCalledWith('/metrics/test-alert')
    })

    it('throws error when API call fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Service unavailable', code: 'UNAVAILABLE' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(sendTestAlert()).rejects.toThrow('Service unavailable')
    })
  })

  describe('getNotificationMetrics', () => {
    it('returns notification metrics', async () => {
      const metrics: NotificationMetrics = {
        total_sent: 1500,
        total_failed: 45,
        channel_breakdown: { email: 800, slack: 500, webhook: 200 },
        alert_level_breakdown: { info: 1400, critical: 100 },
        average_latency_seconds: 0
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getNotificationMetrics()

      expect(result).toEqual(metrics)
      expect(result.total_sent).toBe(1500)
      expect(result.channel_breakdown.email).toBe(800)
      expect(api.get).toHaveBeenCalledWith('/metrics/notifications')
    })

    it('handles empty metrics', async () => {
      // The Go collector makes every map, so an empty one arrives as {}.
      const metrics: NotificationMetrics = {
        total_sent: 0,
        total_failed: 0,
        channel_breakdown: {},
        alert_level_breakdown: {},
        average_latency_seconds: 0
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getNotificationMetrics()

      expect(result.total_sent).toBe(0)
      expect(Object.keys(result.channel_breakdown)).toHaveLength(0)
    })

    it('throws error when metrics fail', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Not found', code: 'NOT_FOUND' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getNotificationMetrics()).rejects.toThrow('Not found')
    })
  })

  describe('getResolutionMetrics', () => {
    it('returns resolution metrics', async () => {
      const metrics: ResolutionMetrics = {
        total_resolutions: 250,
        auto_fix_success_rate: { drift: 0.9 },
        resolutions_by_category: { drift: 150, config: 80, error: 20 },
        average_review_time_seconds: 3600
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getResolutionMetrics()

      expect(result).toEqual(metrics)
      expect(result.total_resolutions).toBe(250)
      expect(result.average_review_time_seconds).toBe(3600)
      expect(result.resolutions_by_category.drift).toBe(150)
      expect(api.get).toHaveBeenCalledWith('/metrics/resolution')
    })

    it('handles zero review time', async () => {
      const metrics: ResolutionMetrics = {
        total_resolutions: 0,
        auto_fix_success_rate: {},
        resolutions_by_category: {},
        average_review_time_seconds: 0
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getResolutionMetrics()

      expect(result.total_resolutions).toBe(0)
      expect(result.average_review_time_seconds).toBe(0)
    })

    it('throws error when metrics fail', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Access denied', code: 'ACCESS_DENIED' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getResolutionMetrics()).rejects.toThrow('Access denied')
    })
  })

  // ginsys/shelly-manager#876: the server route is gone, so the client has no call for it.
  describe('security metrics client', () => {
    it('is gone: the module exports no security call', () => {
      expect(Object.keys(metricsModule).filter(k => /security/i.test(k))).toEqual([])
    })
  })
})
