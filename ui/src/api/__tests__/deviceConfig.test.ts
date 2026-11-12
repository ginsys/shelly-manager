import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client
vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn()
    }
  }
})

import api from '../client'
import {
  updateDeviceConfig,
  getCurrentDeviceConfig,
  detectConfigDrift,
  getConfigHistory,
  type DeviceConfig,
  type ConfigHistoryEntry
} from '../deviceConfig'

describe('deviceConfig API', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
  })

  describe('updateDeviceConfig', () => {
    it('updates stored device configuration', async () => {
      const config: DeviceConfig = {
        wifi: { ssid: 'UpdatedNetwork' }
      }

      ;(api.put as any).mockResolvedValue({
        data: {
          success: true,
          data: config,
          timestamp: new Date().toISOString()
        }
      })

      const result = await updateDeviceConfig(123, config)
      expect(result).toEqual(config)
      expect(api.put).toHaveBeenCalledWith('/devices/123/config', config)
    })

    it('throws error on update failure', async () => {
      ;(api.put as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid configuration' }
        }
      })

      await expect(updateDeviceConfig(123, {})).rejects.toThrow('Invalid configuration')
    })
  })

  describe('getCurrentDeviceConfig', () => {
    it('returns current live configuration from device', async () => {
      const config: DeviceConfig = {
        wifi: { ssid: 'LiveNetwork', connected: true },
        uptime: 86400
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: config,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getCurrentDeviceConfig(123)
      expect(result).toEqual(config)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/current')
    })

    it('throws error when device offline', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device offline' }
        }
      })

      await expect(getCurrentDeviceConfig(123)).rejects.toThrow('Device offline')
    })
  })

  describe('detectConfigDrift', () => {
    const respond = (data: unknown) =>
      (api.get as any).mockResolvedValue({
        data: { success: true, data, timestamp: new Date().toISOString() }
      })

    it('maps a drift report to hasDrift and the changed paths', async () => {
      respond({
        device_id: 123,
        device_name: 'Living room',
        differences: [
          { path: 'wifi.ssid', expected: 'Old', actual: 'New', type: 'modified' },
          { path: 'mqtt.enabled', expected: true, actual: false, type: 'modified' }
        ],
        requires_action: true
      })

      const result = await detectConfigDrift(123)
      expect(result).toEqual({ hasDrift: true, driftFields: ['wifi.ssid', 'mqtt.enabled'] })
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/drift', { signal: undefined })
    })

    it('reports no drift for an empty differences list', async () => {
      respond({ device_id: 123, differences: [], requires_action: false })

      const result = await detectConfigDrift(123)
      expect(result).toEqual({ hasDrift: false })
    })

    it('reports no drift for a null differences list', async () => {
      respond({ device_id: 123, differences: null })

      expect((await detectConfigDrift(123)).hasDrift).toBe(false)
    })

    it('maps the no-stored-configuration shape to no drift', async () => {
      respond({ device_id: 123, drift: false, message: 'No stored configuration to compare' })

      const result = await detectConfigDrift(123)
      expect(result).toEqual({ hasDrift: false })
    })

    it('throws the backend message when the request fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: { success: false, error: { message: 'Device not responding' } }
      })

      await expect(detectConfigDrift(123)).rejects.toThrow('Device not responding')
    })

    it.each([
      ['an empty object', {}],
      ['drift true without a differences list', { drift: true }],
      ['a differences value that is not a list', { differences: 'invalid' }],
      ['a differences object', { differences: { path: 'wifi.ssid' } }],
      ['a message without a verdict', { device_id: 123, message: 'No stored configuration to compare' }]
    ])('rejects %s instead of showing no drift', async (_label, payload) => {
      respond(payload)

      await expect(detectConfigDrift(123)).rejects.toThrow('Unexpected configuration drift response')
    })
  })

  describe('getConfigHistory', () => {
    it('returns configuration change history', async () => {
      const history: ConfigHistoryEntry[] = [
        {
          id: 1,
          deviceId: 123,
          timestamp: '2023-01-01T12:00:00Z',
          config: { wifi: { ssid: 'Network1' } },
          source: 'manual',
          user: 'admin'
        },
        {
          id: 2,
          deviceId: 123,
          timestamp: '2023-01-02T12:00:00Z',
          config: { wifi: { ssid: 'Network2' } },
          source: 'template',
          user: 'admin'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { history },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getConfigHistory(123)
      expect(result).toHaveLength(2)
      expect(result[0].source).toBe('manual')
      expect(result[1].source).toBe('template')
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/history')
    })

    it('returns empty history when none exists', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { history: [] },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getConfigHistory(123)
      expect(result).toEqual([])
    })

    it('handles missing history field gracefully', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: {},
          timestamp: new Date().toISOString()
        }
      })

      const result = await getConfigHistory(123)
      expect(result).toEqual([])
    })
  })
})
