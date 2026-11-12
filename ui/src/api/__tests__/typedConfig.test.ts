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
  getDeviceCapabilities,
  getConfigSchema,
  type DeviceCapabilities,
  type ConfigSchema
} from '../typedConfig'

describe('typedConfig API', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
  })

  describe('getDeviceCapabilities', () => {
    it('reads the bare body the server sends', async () => {
      // GET /devices/{id}/capabilities answers with the object itself, no {success, data} envelope.
      const capabilities: DeviceCapabilities = {
        device_id: 123,
        device_model: 'SHSW-25',
        generation: 1,
        capabilities: ['wifi', 'relay', 'power_metering', 'roller']
      }

      ;(api.get as any).mockResolvedValue({ data: capabilities })

      const result = await getDeviceCapabilities(123)
      expect(result).toEqual(capabilities)
      expect(result.capabilities).toContain('roller')
      expect(api.get).toHaveBeenCalledWith('/devices/123/capabilities', { signal: undefined })
    })

    it('throws when the body carries no capability list', async () => {
      ;(api.get as any).mockResolvedValue({ data: {} })

      await expect(getDeviceCapabilities(123)).rejects.toThrow('Failed to get device capabilities')
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
      expect(api.get).toHaveBeenCalledWith('/config/schema', {
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
      expect(api.get).toHaveBeenCalledWith('/config/schema', { params: {} })
    })

    it('asks for the template schema of a global or group scope', async () => {
      ;(api.get as any).mockResolvedValue({ data: { success: true, data: { type: 'object', properties: {} } } })
      await getConfigSchema(undefined, undefined, 'group')
      expect(api.get).toHaveBeenCalledWith('/config/schema', { params: { scope: 'group' } })
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
})
