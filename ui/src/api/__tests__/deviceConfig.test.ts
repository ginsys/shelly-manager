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
import { DEVICE_CALL_TIMEOUT_MS } from '../deviceCallTimeout'
import {
  updateDeviceConfig,
  getCurrentDeviceConfig,
  detectConfigDrift,
  getConfigHistory,
  getStoredDeviceConfig,
  type DeviceConfig
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
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/current', { timeout: DEVICE_CALL_TIMEOUT_MS })
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
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/drift', { signal: undefined, timeout: DEVICE_CALL_TIMEOUT_MS })
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

    const refused = (status: number, message: string) =>
      (api.get as any).mockRejectedValue({
        isAxiosError: true,
        response: { status, data: { success: false, error: { code: 'NOT_FOUND', message } } }
      })

    it('reports a device with no desired configuration as such, never as no drift', async () => {
      refused(
        404,
        'no desired configuration is set for this device, so there is nothing to compare it with'
      )

      expect(await detectConfigDrift(123)).toEqual({ hasDrift: false, noDesiredConfig: true })
    })

    it('rethrows any other 404, so a missing device is not shown as a missing configuration', async () => {
      refused(404, 'device not found')

      await expect(detectConfigDrift(123)).rejects.toMatchObject({ response: { status: 404 } })
    })

    it('rethrows the same sentence under another status', async () => {
      refused(
        500,
        'no desired configuration is set for this device, so there is nothing to compare it with'
      )

      await expect(detectConfigDrift(123)).rejects.toMatchObject({ response: { status: 500 } })
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


  // The server answers the stored rows as a bare list (internal/configuration ConfigHistory), newest
  // first, with the configuration as an object under old_config and new_config.
  describe('getConfigHistory', () => {
    const row = (over: Record<string, unknown> = {}) => ({
      id: 7,
      device_id: 123,
      config_id: 1,
      action: 'manual',
      old_config: { mqtt: { user: 'iot' } },
      new_config: { mqtt: { user: 'iot2' } },
      changes: [{ path: 'mqtt.user', expected: 'iot', actual: 'iot2', type: 'modified' }],
      changed_by: 'user',
      created_at: '2026-11-20T10:00:00Z',
      ...over
    })
    const respond = (data: unknown) =>
      (api.get as any).mockResolvedValue({ data: { success: true, data, timestamp: new Date().toISOString() } })

    it('maps the stored rows to history entries', async () => {
      respond([row(), row({ id: 6, action: 'updated', old_config: null, changes: null, changed_by: 'api' })])

      const result = await getConfigHistory(123)

      expect(api.get).toHaveBeenCalledWith('/devices/123/config/history')
      expect(result).toHaveLength(2)
      expect(result[0]).toEqual({
        id: 7,
        deviceId: 123,
        timestamp: '2026-11-20T10:00:00Z',
        source: 'manual',
        user: 'user',
        config: { mqtt: { user: 'iot2' } },
        oldConfig: { mqtt: { user: 'iot' } },
        changes: [{ path: 'mqtt.user', expected: 'iot', actual: 'iot2', type: 'modified' }]
      })
      expect(result[1].source).toBe('updated')
      expect(result[1].oldConfig).toBeNull()
      expect(result[1].changes).toBeNull()
    })

    it('returns empty history when none exists', async () => {
      respond([])

      expect(await getConfigHistory(123)).toEqual([])
    })

    it('keeps a configuration the server sent as text, so it is shown and never dropped', async () => {
      respond([row({ new_config: '********' })])

      expect((await getConfigHistory(123))[0].config).toBe('********')
    })

    it.each([
      ['the old wrapped shape', { history: [] }],
      ['an empty object', {}],
      ['null', null]
    ])('rejects %s instead of showing an empty history', async (_label, payload) => {
      respond(payload)

      await expect(getConfigHistory(123)).rejects.toThrow('Unexpected configuration history response')
    })

    it('throws the backend message when the request fails', async () => {
      ;(api.get as any).mockResolvedValue({ data: { success: false, error: { message: 'Forbidden' } } })

      await expect(getConfigHistory(123)).rejects.toThrow('Forbidden')
    })
  })

  describe('getStoredDeviceConfig', () => {
    const stored = {
      id: 3,
      device_id: 123,
      template_id: null,
      config: { mqtt: { user: 'iot', pass: '********' } },
      last_synced: '2026-11-20T09:00:00Z',
      sync_status: 'synced',
      created_at: '2026-11-19T09:00:00Z',
      updated_at: '2026-11-20T09:00:00Z'
    }

    it('returns the stored import', async () => {
      ;(api.get as any).mockResolvedValue({ data: { success: true, data: stored } })

      expect(await getStoredDeviceConfig(123)).toEqual(stored)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config')
    })

    it('answers null for a device that was never imported (404)', async () => {
      ;(api.get as any).mockRejectedValue({
        isAxiosError: true,
        response: { status: 404, data: { success: false, error: { code: 'NOT_FOUND', message: 'not found' } } }
      })

      expect(await getStoredDeviceConfig(123)).toBeNull()
    })

    it('rethrows any other failure, so a broken store is not shown as nothing imported', async () => {
      ;(api.get as any).mockRejectedValue({ isAxiosError: true, response: { status: 500, data: {} } })

      await expect(getStoredDeviceConfig(123)).rejects.toMatchObject({ response: { status: 500 } })
    })

    it('throws the backend message on an unsuccessful answer', async () => {
      ;(api.get as any).mockResolvedValue({ data: { success: false, error: { message: 'Broken' } } })

      await expect(getStoredDeviceConfig(123)).rejects.toThrow('Broken')
    })
  })
})
