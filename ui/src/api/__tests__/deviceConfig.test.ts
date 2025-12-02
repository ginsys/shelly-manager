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
  getDeviceConfig,
  updateDeviceConfig,
  getCurrentDeviceConfig,
  getNormalizedCurrentConfig,
  getTypedNormalizedConfig,
  importDeviceConfig,
  getConfigImportStatus,
  exportDeviceConfig,
  detectConfigDrift,
  applyConfigTemplate,
  getConfigHistory,
  type DeviceConfig,
  type ConfigDrift,
  type ConfigHistoryEntry,
  type ConfigImportStatus
} from '../deviceConfig'

describe('deviceConfig API', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
  })

  describe('getDeviceConfig', () => {
    it('returns stored device configuration', async () => {
      const config: DeviceConfig = {
        wifi: { ssid: 'MyNetwork', password: '***' },
        mqtt: { enabled: true, server: 'mqtt.example.com' }
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: config,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDeviceConfig(123)
      expect(result).toEqual(config)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config')
    })

    it('throws error when config not found', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Configuration not found' }
        }
      })

      await expect(getDeviceConfig(123)).rejects.toThrow('Configuration not found')
    })
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

  describe('getNormalizedCurrentConfig', () => {
    it('returns normalized live configuration', async () => {
      const config: DeviceConfig = {
        network: { wifi_ssid: 'NormalizedNetwork' },
        power: { relay_state: true }
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: config,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getNormalizedCurrentConfig(123)
      expect(result).toEqual(config)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/current/normalized')
    })
  })

  describe('getTypedNormalizedConfig', () => {
    it('returns typed normalized configuration', async () => {
      const config: DeviceConfig = {
        network: { wifi_ssid: 'TypedNetwork' },
        deviceType: 'shelly1pm'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: config,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getTypedNormalizedConfig(123)
      expect(result).toEqual(config)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/typed/normalized')
    })
  })

  describe('importDeviceConfig', () => {
    it('imports configuration to device', async () => {
      const config: DeviceConfig = {
        wifi: { ssid: 'ImportedNetwork' }
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await importDeviceConfig(123, config)
      expect(api.post).toHaveBeenCalledWith('/devices/123/config/import', config)
    })

    it('throws error on import failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Import failed: device busy' }
        }
      })

      await expect(importDeviceConfig(123, {})).rejects.toThrow('Import failed: device busy')
    })
  })

  describe('getConfigImportStatus', () => {
    it('returns import status', async () => {
      const status: ConfigImportStatus = {
        status: 'in_progress',
        progress: 50,
        message: 'Applying configuration...'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: status,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getConfigImportStatus(123)
      expect(result).toEqual(status)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/status')
    })

    it('returns completed status', async () => {
      const status: ConfigImportStatus = {
        status: 'completed',
        progress: 100,
        completedAt: '2023-01-01T12:00:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: status,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getConfigImportStatus(123)
      expect(result.status).toBe('completed')
      expect(result.progress).toBe(100)
    })
  })

  describe('exportDeviceConfig', () => {
    it('exports configuration from device', async () => {
      const config: DeviceConfig = {
        wifi: { ssid: 'ExportedNetwork' },
        mqtt: { enabled: false }
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: config,
          timestamp: new Date().toISOString()
        }
      })

      const result = await exportDeviceConfig(123)
      expect(result).toEqual(config)
      expect(api.post).toHaveBeenCalledWith('/devices/123/config/export')
    })

    it('throws error when export fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device not responding' }
        }
      })

      await expect(exportDeviceConfig(123)).rejects.toThrow('Device not responding')
    })
  })

  describe('detectConfigDrift', () => {
    it('detects configuration drift', async () => {
      const drift: ConfigDrift = {
        hasDrift: true,
        driftFields: ['wifi.ssid', 'mqtt.enabled'],
        storedConfig: { wifi: { ssid: 'OldNetwork' }, mqtt: { enabled: true } },
        liveConfig: { wifi: { ssid: 'NewNetwork' }, mqtt: { enabled: false } }
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: drift,
          timestamp: new Date().toISOString()
        }
      })

      const result = await detectConfigDrift(123)
      expect(result.hasDrift).toBe(true)
      expect(result.driftFields).toHaveLength(2)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/drift')
    })

    it('returns no drift when configurations match', async () => {
      const drift: ConfigDrift = {
        hasDrift: false
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: drift,
          timestamp: new Date().toISOString()
        }
      })

      const result = await detectConfigDrift(123)
      expect(result.hasDrift).toBe(false)
      expect(result.driftFields).toBeUndefined()
    })
  })

  describe('applyConfigTemplate', () => {
    it('applies template to device configuration', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await applyConfigTemplate(123, 456)
      expect(api.post).toHaveBeenCalledWith('/devices/123/config/apply-template', { templateId: 456 })
    })

    it('throws error when template not found', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Template not found' }
        }
      })

      await expect(applyConfigTemplate(123, 999)).rejects.toThrow('Template not found')
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
