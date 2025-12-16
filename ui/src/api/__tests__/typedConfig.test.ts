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
  getTypedConfig,
  updateTypedConfig,
  getDeviceCapabilities,
  validateTypedConfig,
  convertToTyped,
  convertToRaw,
  getConfigSchema,
  bulkValidateConfigs,
  type TypedConfig,
  type DeviceCapabilities,
  type ValidationResult,
  type ConfigSchema,
  type BulkValidationResult
} from '../typedConfig'

describe('typedConfig API', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
  })

  describe('getTypedConfig', () => {
    it('returns typed configuration for a device', async () => {
      const typedConfig: TypedConfig = {
        deviceId: '123',
        deviceType: 'shelly1',
        config: {
          wifi: { ssid: 'MyNetwork' },
          mqtt: { enabled: true }
        }
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: typedConfig,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getTypedConfig(123)
      expect(result).toEqual(typedConfig)
      expect(api.get).toHaveBeenCalledWith('/devices/123/config/typed')
    })

    it('throws error when config not available', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device not found' }
        }
      })

      await expect(getTypedConfig(999)).rejects.toThrow('Device not found')
    })
  })

  describe('updateTypedConfig', () => {
    it('updates typed configuration successfully', async () => {
      const newConfig = { wifi: { ssid: 'NewNetwork' } }
      const updatedConfig: TypedConfig = {
        deviceId: '123',
        deviceType: 'shelly1',
        config: newConfig
      }

      ;(api.put as any).mockResolvedValue({
        data: {
          success: true,
          data: updatedConfig,
          timestamp: new Date().toISOString()
        }
      })

      const result = await updateTypedConfig(123, newConfig)
      expect(result).toEqual(updatedConfig)
      expect(api.put).toHaveBeenCalledWith('/devices/123/config/typed', { config: newConfig })
    })

    it('throws error on update failure', async () => {
      ;(api.put as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid configuration' }
        }
      })

      await expect(updateTypedConfig(123, {})).rejects.toThrow('Invalid configuration')
    })
  })

  describe('getDeviceCapabilities', () => {
    it('returns device capabilities', async () => {
      const capabilities: DeviceCapabilities = {
        deviceId: '123',
        deviceType: 'shelly1',
        capabilities: ['relay', 'wifi', 'mqtt'],
        supportedFeatures: {
          relay: true,
          dimmer: false,
          roller: false
        },
        firmwareVersion: '1.11.0'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: capabilities,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDeviceCapabilities(123)
      expect(result).toEqual(capabilities)
      expect(result.capabilities).toContain('relay')
      expect(result.supportedFeatures.relay).toBe(true)
      expect(api.get).toHaveBeenCalledWith('/devices/123/capabilities')
    })

    it('throws error when capabilities not available', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Capabilities not available' }
        }
      })

      await expect(getDeviceCapabilities(123)).rejects.toThrow('Capabilities not available')
    })
  })

  describe('validateTypedConfig', () => {
    it('validates configuration successfully', async () => {
      const validation: ValidationResult = {
        valid: true
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: validation,
          timestamp: new Date().toISOString()
        }
      })

      const result = await validateTypedConfig({
        config: { wifi: { ssid: 'Test' } },
        deviceType: 'shelly1'
      })
      expect(result.valid).toBe(true)
      expect(result.errors).toBeUndefined()
    })

    it('returns validation errors', async () => {
      const validation: ValidationResult = {
        valid: false,
        errors: [
          { field: 'wifi.ssid', message: 'SSID is required', severity: 'error' }
        ],
        warnings: [
          { field: 'mqtt.enabled', message: 'MQTT not configured', severity: 'warning' }
        ]
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: validation,
          timestamp: new Date().toISOString()
        }
      })

      const result = await validateTypedConfig({
        config: {},
        deviceType: 'shelly1'
      })
      expect(result.valid).toBe(false)
      expect(result.errors).toHaveLength(1)
      expect(result.warnings).toHaveLength(1)
    })
  })

  describe('convertToTyped', () => {
    it('converts raw config to typed format', async () => {
      const typedConfig: TypedConfig = {
        deviceId: '123',
        deviceType: 'shelly1',
        config: { wifi: { ssid: 'Test' } }
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: typedConfig,
          timestamp: new Date().toISOString()
        }
      })

      const result = await convertToTyped({
        config: { 'wifi.ssid': 'Test' },
        deviceType: 'shelly1'
      })
      expect(result).toEqual(typedConfig)
    })

    it('throws error on conversion failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid config format' }
        }
      })

      await expect(convertToTyped({ config: {} })).rejects.toThrow('Invalid config format')
    })
  })

  describe('convertToRaw', () => {
    it('converts typed config to raw format', async () => {
      const rawConfig = { 'wifi.ssid': 'Test', 'mqtt.enabled': 'true' }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: { config: rawConfig },
          timestamp: new Date().toISOString()
        }
      })

      const result = await convertToRaw({
        config: { wifi: { ssid: 'Test' }, mqtt: { enabled: true } },
        deviceType: 'shelly1'
      })
      expect(result).toEqual(rawConfig)
    })

    it('throws error on conversion failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Conversion failed' }
        }
      })

      await expect(convertToRaw({ config: {} })).rejects.toThrow('Conversion failed')
    })
  })

  describe('getConfigSchema', () => {
    it('returns configuration schema', async () => {
      const schema: ConfigSchema = {
        type: 'object',
        properties: {
          wifi: {
            type: 'object',
            description: 'WiFi configuration'
          },
          mqtt: {
            type: 'object',
            description: 'MQTT configuration'
          }
        },
        required: ['wifi']
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: schema,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getConfigSchema('shelly1')
      expect(result).toEqual(schema)
      expect(result.required).toContain('wifi')
      expect(api.get).toHaveBeenCalledWith('/configuration/schema', {
        params: { device_type: 'shelly1' }
      })
    })

    it('fetches schema without device type', async () => {
      const schema: ConfigSchema = {
        type: 'object',
        properties: {}
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: schema,
          timestamp: new Date().toISOString()
        }
      })

      await getConfigSchema()
      expect(api.get).toHaveBeenCalledWith('/configuration/schema', {
        params: { device_type: undefined }
      })
    })

    it('throws error when schema not available', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Schema not found' }
        }
      })

      await expect(getConfigSchema('invalid')).rejects.toThrow('Schema not found')
    })
  })

  describe('bulkValidateConfigs', () => {
    it('validates multiple configurations', async () => {
      const bulkResult: BulkValidationResult = {
        results: [
          {
            deviceId: '1',
            valid: true
          },
          {
            deviceId: '2',
            valid: false,
            errors: [{ field: 'wifi', message: 'Invalid WiFi', severity: 'error' }]
          }
        ],
        summary: {
          total: 2,
          valid: 1,
          invalid: 1
        }
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: bulkResult,
          timestamp: new Date().toISOString()
        }
      })

      const result = await bulkValidateConfigs({
        configs: [
          { deviceId: '1', config: { wifi: { ssid: 'Test1' } } },
          { deviceId: '2', config: {} }
        ]
      })
      expect(result.summary.total).toBe(2)
      expect(result.summary.valid).toBe(1)
      expect(result.summary.invalid).toBe(1)
      expect(result.results[1].valid).toBe(false)
    })

    it('handles empty bulk validation', async () => {
      const bulkResult: BulkValidationResult = {
        results: [],
        summary: {
          total: 0,
          valid: 0,
          invalid: 0
        }
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: bulkResult,
          timestamp: new Date().toISOString()
        }
      })

      const result = await bulkValidateConfigs({ configs: [] })
      expect(result.summary.total).toBe(0)
    })

    it('throws error on bulk validation failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Bulk validation failed' }
        }
      })

      await expect(bulkValidateConfigs({ configs: [] })).rejects.toThrow('Bulk validation failed')
    })
  })
})
