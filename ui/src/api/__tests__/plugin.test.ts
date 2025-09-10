import { describe, it, expect, beforeEach, vi } from 'vitest'
import type { AxiosResponse } from 'axios'
import {
  listPlugins,
  getPlugin,
  getPluginSchema,
  validatePluginConfig,
  type Plugin,
  type PluginSchema,
  type APIResponse
} from '../plugin'

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
        version: '1.0.0',
        description: 'System backup plugin',
        category: 'backup',
        status: 'active',
        author: 'Shelly Team',
        enabled: true,
        configured: true,
        supported_formats: ['json', 'yaml'],
        tags: ['backup', 'export'],
        created_at: '2023-01-01T00:00:00Z',
        updated_at: '2023-06-01T00:00:00Z'
      },
      {
        name: 'gitops-plugin',
        version: '2.1.0',
        description: 'GitOps export plugin',
        category: 'gitops',
        status: 'active',
        author: 'Shelly Team',
        enabled: true,
        configured: false,
        supported_formats: ['terraform', 'ansible', 'kubernetes'],
        tags: ['gitops', 'iac'],
        created_at: '2023-02-01T00:00:00Z',
        updated_at: '2023-07-01T00:00:00Z'
      }
    ]

    it('should fetch all plugins successfully', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[] }>> = {
        data: {
          success: true,
          data: { plugins: mockPlugins },
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

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins')
      expect(result.plugins).toEqual(mockPlugins)
      expect(result.plugins).toHaveLength(2)
    })

    it('should filter plugins by category', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[] }>> = {
        data: {
          success: true,
          data: { plugins: [mockPlugins[0]] },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listPlugins({ category: 'backup' })

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins', {
        params: { category: 'backup' }
      })
      expect(result.plugins).toEqual([mockPlugins[0]])
    })

    it('should filter plugins by status', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[] }>> = {
        data: {
          success: true,
          data: { plugins: mockPlugins },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listPlugins({ status: 'active' })

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins', {
        params: { status: 'active' }
      })
      expect(result.plugins).toHaveLength(2)
    })

    it('should filter plugins by enabled status', async () => {
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[] }>> = {
        data: {
          success: true,
          data: { plugins: mockPlugins.filter(p => p.enabled) },
          timestamp: '2023-01-01T00:00:00Z',
          request_id: 'req-123'
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as any
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listPlugins({ enabled: true })

      expect(mockApi.get).toHaveBeenCalledWith('/export/plugins', {
        params: { enabled: true }
      })
      expect(result.plugins).toHaveLength(2)
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
      const mockResponse: AxiosResponse<APIResponse<{ plugins: Plugin[] }>> = {
        data: {
          success: true,
          data: null,
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
      version: '1.0.0',
      description: 'System backup plugin',
      long_description: 'A comprehensive system backup plugin that supports multiple formats and compression options.',
      category: 'backup',
      status: 'active',
      author: 'Shelly Team',
      enabled: true,
      configured: true,
      supported_formats: ['json', 'yaml', 'tar.gz'],
      tags: ['backup', 'export', 'compression'],
      license: 'MIT',
      created_at: '2023-01-01T00:00:00Z',
      updated_at: '2023-06-01T00:00:00Z',
      capabilities: {
        export: ['full-backup', 'incremental', 'selective'],
        import: ['restore', 'selective-restore']
      },
      features: ['Compression', 'Encryption', 'Scheduling', 'Versioning'],
      config: {
        compression: 'gzip',
        encryption: true,
        retention_days: 30
      },
      usage_stats: {
        total_exports: 150,
        successful_exports: 145,
        failed_exports: 5,
        last_used: '2023-07-15T10:30:00Z'
      },
      performance: {
        avg_duration: '2.5s',
        success_rate: '96.7%'
      },
      health: {
        status: 'healthy',
        last_check: '2023-07-15T12:00:00Z',
        dependencies: [
          { name: 'gzip', version: '1.10', status: 'ok' },
          { name: 'tar', version: '1.34', status: 'ok' }
        ],
        messages: []
      },
      recent_activity: [
        {
          id: '1',
          action: 'Backup Created',
          description: 'Full system backup completed successfully',
          timestamp: '2023-07-15T10:30:00Z',
          success: true
        }
      ]
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
      expect(result.configured).toBe(true)
      expect(result.usage_stats?.total_exports).toBe(150)
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

    it('should validate plugin name parameter', async () => {
      await expect(getPlugin('')).rejects.toThrow('Plugin name is required')
      await expect(getPlugin('  ')).rejects.toThrow('Plugin name is required')
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
        },
        exclude_patterns: {
          type: 'array',
          title: 'Exclude Patterns',
          description: 'File patterns to exclude from backup',
          items: {
            type: 'string'
          },
          default: ['*.tmp', '*.log']
        },
        advanced_config: {
          type: 'object',
          title: 'Advanced Configuration',
          description: 'Advanced configuration options',
          properties: {
            buffer_size: {
              type: 'integer',
              minimum: 1024,
              maximum: 1048576,
              default: 65536
            }
          }
        }
      },
      required: ['compression', 'retention_days'],
      additionalProperties: false
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

    it('should validate plugin name parameter', async () => {
      await expect(getPluginSchema('')).rejects.toThrow('Plugin name is required')
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

      const result = validatePluginConfig(config, mockSchema)

      expect(result.valid).toBe(true)
      expect(result.errors).toEqual([])
    })

    it('should detect missing required fields', () => {
      const config = {
        enabled: true
      }

      const result = validatePluginConfig(config, mockSchema)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('name is required')
      expect(result.errors).toContain('count is required')
    })

    it('should detect type mismatches', () => {
      const config = {
        name: 123, // should be string
        count: 'invalid', // should be number
        enabled: 'yes' // should be boolean
      }

      const result = validatePluginConfig(config, mockSchema)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('name must be a string')
      expect(result.errors).toContain('count must be a number')
      expect(result.errors).toContain('enabled must be a boolean')
    })

    it('should validate string constraints', () => {
      const config = {
        name: '', // violates minLength
        count: 5
      }

      const result = validatePluginConfig(config, mockSchema)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('name must be at least 1 characters')
    })

    it('should validate number constraints', () => {
      const config = {
        name: 'valid',
        count: -1 // violates minimum
      }

      const result = validatePluginConfig(config, mockSchema)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('count must be at least 0')
    })

    it('should validate array types', () => {
      const config = {
        name: 'valid',
        count: 5,
        tags: [123, 'valid'] // mixed types in array
      }

      const result = validatePluginConfig(config, mockSchema)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('tags[0] must be a string')
    })

    it('should handle empty schema', () => {
      const config = { any: 'value' }
      const emptySchema: PluginSchema = { type: 'object' }

      const result = validatePluginConfig(config, emptySchema)

      expect(result.valid).toBe(true)
      expect(result.errors).toEqual([])
    })

    it('should handle null or undefined config', () => {
      const result1 = validatePluginConfig(null as any, mockSchema)
      const result2 = validatePluginConfig(undefined as any, mockSchema)

      expect(result1.valid).toBe(false)
      expect(result2.valid).toBe(false)
      expect(result1.errors).toContain('Configuration is required')
      expect(result2.errors).toContain('Configuration is required')
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