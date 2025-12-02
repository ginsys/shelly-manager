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
import {
  getPrometheusMetrics,
  enableMetrics,
  disableMetrics,
  collectMetrics,
  getDashboardSummary,
  sendTestAlert,
  getNotificationMetrics,
  getResolutionMetrics,
  getSecurityMetrics,
  type MetricsStatus,
  type DashboardSummary,
  type NotificationMetrics,
  type ResolutionMetrics,
  type SecurityMetrics,
  type TestAlertResult
} from '../metrics'

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
      const summary: DashboardSummary = {
        devices: { total: 100, online: 85, offline: 15 },
        exports: { total: 250, recent: 10 },
        imports: { total: 180, recent: 5 },
        drifts: { total: 30, unresolved: 8 },
        notifications: { sent: 500, failed: 12 }
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: summary, timestamp: new Date().toISOString() }
      })

      const result = await getDashboardSummary()

      expect(result).toEqual(summary)
      expect(result.devices.total).toBe(100)
      expect(result.devices.online).toBe(85)
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
      const result: TestAlertResult = {
        success: true,
        message: 'Test alert sent successfully',
        timestamp: '2023-01-01T00:00:00Z'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await sendTestAlert()

      expect(response).toEqual(result)
      expect(response.success).toBe(true)
      expect(api.post).toHaveBeenCalledWith('/metrics/test-alert')
    })

    it('returns failure result when alert fails', async () => {
      const result: TestAlertResult = {
        success: false,
        message: 'No notification channels configured',
        timestamp: '2023-01-01T00:00:00Z'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await sendTestAlert()

      expect(response.success).toBe(false)
      expect(response.message).toContain('No notification channels')
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
        totalSent: 1500,
        totalFailed: 45,
        byChannel: {
          email: { sent: 800, failed: 20 },
          slack: { sent: 500, failed: 15 },
          webhook: { sent: 200, failed: 10 }
        },
        recentNotifications: [
          { timestamp: '2023-01-01T00:00:00Z', channel: 'email', status: 'sent' },
          { timestamp: '2023-01-01T00:01:00Z', channel: 'slack', status: 'failed' }
        ]
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getNotificationMetrics()

      expect(result).toEqual(metrics)
      expect(result.totalSent).toBe(1500)
      expect(result.byChannel.email.sent).toBe(800)
      expect(api.get).toHaveBeenCalledWith('/metrics/notifications')
    })

    it('handles empty metrics', async () => {
      const metrics: NotificationMetrics = {
        totalSent: 0,
        totalFailed: 0,
        byChannel: {},
        recentNotifications: []
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getNotificationMetrics()

      expect(result.totalSent).toBe(0)
      expect(Object.keys(result.byChannel)).toHaveLength(0)
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
        totalResolved: 250,
        averageResolutionTime: 3600,
        byType: {
          drift: 150,
          config: 80,
          error: 20
        },
        byUser: {
          admin: 180,
          operator: 70
        }
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getResolutionMetrics()

      expect(result).toEqual(metrics)
      expect(result.totalResolved).toBe(250)
      expect(result.averageResolutionTime).toBe(3600)
      expect(result.byType.drift).toBe(150)
      expect(api.get).toHaveBeenCalledWith('/metrics/resolution')
    })

    it('handles zero resolution time', async () => {
      const metrics: ResolutionMetrics = {
        totalResolved: 0,
        averageResolutionTime: 0,
        byType: {},
        byUser: {}
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getResolutionMetrics()

      expect(result.totalResolved).toBe(0)
      expect(result.averageResolutionTime).toBe(0)
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

  describe('getSecurityMetrics', () => {
    it('returns security metrics', async () => {
      const metrics: SecurityMetrics = {
        authAttempts: { successful: 5000, failed: 120 },
        apiCalls: { total: 50000, errors: 250 },
        rateLimit: { triggered: 15, blocked: 8 }
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getSecurityMetrics()

      expect(result).toEqual(metrics)
      expect(result.authAttempts.successful).toBe(5000)
      expect(result.authAttempts.failed).toBe(120)
      expect(result.apiCalls.total).toBe(50000)
      expect(result.rateLimit.triggered).toBe(15)
      expect(api.get).toHaveBeenCalledWith('/metrics/security')
    })

    it('handles zero security events', async () => {
      const metrics: SecurityMetrics = {
        authAttempts: { successful: 0, failed: 0 },
        apiCalls: { total: 0, errors: 0 },
        rateLimit: { triggered: 0, blocked: 0 }
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: metrics, timestamp: new Date().toISOString() }
      })

      const result = await getSecurityMetrics()

      expect(result.authAttempts.failed).toBe(0)
      expect(result.rateLimit.triggered).toBe(0)
    })

    it('throws error when metrics fail', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Permission denied', code: 'FORBIDDEN' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getSecurityMetrics()).rejects.toThrow('Permission denied')
    })

    it('throws default error when no data returned', async () => {
      ;(api.get as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(getSecurityMetrics()).rejects.toThrow('Failed to load security metrics')
    })
  })
})
