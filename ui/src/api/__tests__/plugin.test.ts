import { describe, it, expect, beforeEach, vi } from 'vitest'
import type { AxiosResponse } from 'axios'
import {
  listPlugins,
  getPlugin,
  getPluginSchema,
  validatePluginConfig,
  type Plugin,
  type PluginSchema,
} from '../plugin'
import type { APIResponse } from '../types'

// Mock the API client
vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn()
  }
}))

import api from '../client'
const mockApi = vi.mocked(api)

describe('Plugin API Client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('listPlugins', () => {
    const mockPlugins: Plugin[] = [
      {
        name: 'backup-plugin',
        display_name: 'Backup Plugin',
        version: '1.0.0',
        description: 'System backup plugin',
        category: 'backup',
        capabilities: ['backup', 'export'],
        status: {
          available: true,
          configured: true,
          enabled: true,
        }
      },
      {
        name: 'gitops-plugin',
        display_name: 'GitOps Plugin',
        version: '2.1.0',
        description: 'GitOps export plugin',
        category: 'gitops',
        capabilities: ['gitops', 'iac'],
        status: {
          available: true,
          configured: false,
          enabled: false,
        }
      }
    ]

    it('should fetch all plugins successfully', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[]; categories: any[] }>> = {
        data: {
          success: true,
          data: { plugins: mockPlugins, categories: [] },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listPlugins()

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins', {
        params: { category: undefined }
      })
      expect(result.plugins).toEqual(mockPlugins)
      expect(result.plugins).toHaveLength(2)
    })

    it('should filter plugins by category', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[]; categories: any[] }>> = {
        data: {
          success: true,
          data: { plugins: [mockPlugins[0]], categories: [] },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listPlugins('backup')

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins', {
        params: { category: 'backup' }
      })
      expect(result.plugins).toEqual([mockPlugins[0]])
    })

    it('should handle API errors gracefully', async () => {
      const mockResponse: AxiosResponse<APIResponse<any>> = {
        data: {
          success: false,
          error: { message: 'Failed to fetch plugins', code: 'FETCH_ERROR' },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 500,
        statusText: 'Internal Server Error',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      await expect(listPlugins()).rejects.toThrow('Failed to fetch plugins')
    })

    it('should handle network errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Network error'))

      await expect(listPlugins()).rejects.toThrow('Network error')
    })

    it('should return empty array when no data', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[]; categories: any[] }>> = {
        data: {
          success: true,
          data: null as any,
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listPlugins()

      expect(result.plugins).toEqual([])
    })
  })

  describe('getPlugin', () => {
    const mockPlugin: Plugin = {
      name: 'backup-plugin',
      display_name: 'Backup Plugin',
      version: '1.0.0',
      description: 'System backup plugin',
      category: 'backup',
      capabilities: ['backup', 'export', 'restore'],
      status: {
        available: true,
        configured: true,
        enabled: true,
      }
    }

    it('should fetch plugin details successfully', async () => {
      const mockResponse: AxiosResponse<APIResponse<Plugin>> = {
        data: {
          success: true,
          data: mockPlugin,
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await getPlugin('backup-plugin')

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins/backup-plugin')
      expect(result).toEqual(mockPlugin)
      expect(result.name).toBe('backup-plugin')
      expect(result.status.configured).toBe(true)
    })

    it('should handle plugin not found', async () => {
      const mockResponse: AxiosResponse<APIResponse<any>> = {
        data: {
          success: false,
          error: { message: 'Plugin not found', code: 'NOT_FOUND' },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 404,
        statusText: 'Not Found',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      await expect(getPlugin('nonexistent-plugin')).rejects.toThrow('Plugin not found')
    })
  })

  describe('getPluginSchema', () => {
    const mockSchema: PluginSchema = {
      type: 'object',
      properties: {
        compression: {
          type: 'string',
          title: 'Compression Type',
          description: 'Type of compression to use',
          enum: ['none', 'gzip', 'bzip2', 'xz'],
          default: 'gzip'
        },
        encryption: {
          type: 'boolean',
          title: 'Enable Encryption',
          description: 'Whether to encrypt backup files',
          default: false
        },
        retention_days: {
          type: 'integer',
          title: 'Retention Days',
          description: 'Number of days to retain backups',
          minimum: 1,
          maximum: 365,
          default: 30
        }
      },
      required: ['compression', 'retention_days']
    }

    it('should fetch plugin schema successfully', async () => {
      const mockResponse: AxiosResponse<APIResponse<PluginSchema>> = {
        data: {
          success: true,
          data: mockSchema,
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await getPluginSchema('backup-plugin')

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins/backup-plugin/schema')
      expect(result).toEqual(mockSchema)
      expect(result.properties?.compression?.enum).toContain('gzip')
      expect(result.required).toContain('compression')
    })

    it('should handle schema not available', async () => {
      const mockResponse: AxiosResponse<APIResponse<any>> = {
        data: {
          success: false,
          error: { message: 'Schema not available', code: 'SCHEMA_NOT_FOUND' },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 404,
        statusText: 'Not Found',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      await expect(getPluginSchema('plugin-without-schema')).rejects.toThrow('Schema not available')
    })
  })

  describe('validatePluginConfig', () => {
    const mockSchema: PluginSchema = {
      type: 'object',
      properties: {
        name: {
          type: 'string',
          title: 'Name',
          minLength: 1,
          maxLength: 50
        },
        count: {
          type: 'integer',
          minimum: 0,
          maximum: 100
        },
        enabled: {
          type: 'boolean'
        },
        tags: {
          type: 'array',
          items: { type: 'string' }
        }
      },
      required: ['name', 'count']
    }

    it('should validate valid configuration', () => {
      const config = {
        name: 'test-config',
        count: 5,
        enabled: true,
        tags: ['tag1', 'tag2']
      }

      const errors = validatePluginConfig(config, mockSchema)

      expect(errors).toEqual([])
    })

    it('should detect missing required fields', () => {
      const config = {
        enabled: true
      }

      const errors = validatePluginConfig(config, mockSchema)

      expect(errors.length).toBeGreaterThan(0)
      expect(errors.some(e => e.includes("'name'") && e.includes('required'))).toBe(true)
      expect(errors.some(e => e.includes("'count'") && e.includes('required'))).toBe(true)
    })

    it('should detect type mismatches', () => {
      const config = {
        name: 123, // should be string
        count: 'invalid', // should be number
        enabled: 'yes' // should be boolean
      }

      const errors = validatePluginConfig(config, mockSchema)

      expect(errors.length).toBeGreaterThan(0)
      expect(errors.some(e => e.includes("'name'") && e.includes('string'))).toBe(true)
      expect(errors.some(e => e.includes("'count'") && e.includes('number'))).toBe(true)
      expect(errors.some(e => e.includes("'enabled'") && e.includes('boolean'))).toBe(true)
    })

    it('should validate string constraints', () => {
      const config = {
        name: '', // violates minLength
        count: 5
      }

      const errors = validatePluginConfig(config, mockSchema)

      // Empty string should trigger required validation
      expect(errors.some(e => e.includes("'name'") && e.includes('required'))).toBe(true)
    })

    it('should validate number constraints', () => {
      const config = {
        name: 'valid',
        count: -1 // violates minimum
      }

      const errors = validatePluginConfig(config, mockSchema)

      expect(errors.some(e => e.includes("'count'") && e.includes('at least 0'))).toBe(true)
    })

    it('should validate array types', () => {
      const config = {
        name: 'valid',
        count: 5,
        tags: [123, 'valid'] // mixed types in array
      }

      const errors = validatePluginConfig(config, mockSchema)

      expect(errors.some(e => e.includes('tags[0]') && e.includes('string'))).toBe(true)
    })

    it('should handle empty schema', () => {
      const config = { any: 'value' }
      const emptySchema: PluginSchema = { type: 'object', properties: {} }

      const errors = validatePluginConfig(config, emptySchema)

      expect(errors).toEqual([])
    })
  })

  describe('Edge Cases and Error Handling', () => {
    it('should handle malformed API responses', async () => {
      const mockResponse: AxiosResponse<any> = {
        data: null,
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      await expect(listPlugins()).rejects.toThrow()
    })

    it('should handle timeout errors', async () => {
      const timeoutError = new Error('timeout of 5000ms exceeded')
      timeoutError.name = 'TimeoutError'

      mockApi.get.mockRejectedValue(timeoutError)

      await expect(listPlugins()).rejects.toThrow('timeout of 5000ms exceeded')
    })

    it('should handle rate limiting', async () => {
      const mockResponse: AxiosResponse<APIResponse<any>> = {
        data: {
          success: false,
          error: { message: 'Rate limit exceeded', code: 'RATE_LIMITED' },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 429,
        statusText: 'Too Many Requests',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      await expect(listPlugins()).rejects.toThrow('Rate limit exceeded')
    })
  })
})
