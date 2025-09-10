import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePluginStore } from '../plugin'
import * as pluginApi from '@/api/plugin'
import type { Plugin, PluginSchema, PluginConfig, PluginTestResult, PluginHealthStatus } from '@/api/plugin'

// Mock the API functions
vi.mock('@/api/plugin', () => ({
  listPlugins: vi.fn(),
  getPlugin: vi.fn(),
  getPluginSchema: vi.fn(),
  updatePluginConfig: vi.fn(),
  testPlugin: vi.fn(),
  togglePlugin: vi.fn(),
  getPluginHealth: vi.fn(),
  resetPluginConfig: vi.fn(),
  validatePluginConfig: vi.fn(),
  generateDefaultConfig: vi.fn(),
}))

describe('Plugin Store', () => {
  let store: ReturnType<typeof usePluginStore>

  const mockPlugin: Plugin = {
    name: 'test-plugin',
    display_name: 'Test Plugin',
    description: 'A test plugin for unit testing',
    version: '1.0.0',
    author: 'Test Author',
    category: 'backup',
    capabilities: [
      { name: 'backup', description: 'Full system backup', required: true }
    ],
    status: 'configured',
    health: {
      healthy: true,
      last_check: '2024-01-01T12:00:00Z',
      response_time_ms: 150
    },
    configuration: {
      endpoint: 'https://test.example.com',
      timeout: 30,
      enabled: true
    }
  }

  const mockPlugin2: Plugin = {
    name: 'sync-plugin',
    display_name: 'Sync Plugin',
    description: 'A synchronization plugin',
    version: '2.0.0',
    author: 'Sync Author',
    category: 'sync',
    capabilities: [
      { name: 'sync', description: 'Data synchronization', required: true }
    ],
    status: 'available'
  }

  const mockSchema: PluginSchema = {
    type: 'object',
    title: 'Test Plugin Configuration',
    properties: {
      endpoint: {
        name: 'endpoint',
        type: 'string',
        description: 'API endpoint URL',
        required: true,
        format: 'url'
      },
      timeout: {
        name: 'timeout',
        type: 'number',
        description: 'Request timeout in seconds',
        default: 30,
        minimum: 5,
        maximum: 300
      },
      enabled: {
        name: 'enabled',
        type: 'boolean',
        description: 'Enable the plugin',
        default: true
      }
    },
    required: ['endpoint']
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    store = usePluginStore()
    vi.clearAllMocks()
  })

  describe('state initialization', () => {
    it('should initialize with empty state', () => {
      expect(store.plugins).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.error).toBe('')
      expect(store.currentPlugin).toBeNull()
      expect(store.currentPluginSchema).toBeNull()
      expect(store.categoryFilter).toBe('')
      expect(store.statusFilter).toBe('')
      expect(store.searchQuery).toBe('')
    })
  })

  describe('getters', () => {
    beforeEach(() => {
      store.plugins = [mockPlugin, mockPlugin2]
    })

    describe('filteredPlugins', () => {
      it('should return all plugins when no filters applied', () => {
        expect(store.filteredPlugins).toEqual([mockPlugin, mockPlugin2])
      })

      it('should filter by category', () => {
        store.categoryFilter = 'backup'
        expect(store.filteredPlugins).toEqual([mockPlugin])
      })

      it('should filter by status', () => {
        store.statusFilter = 'available'
        expect(store.filteredPlugins).toEqual([mockPlugin2])
      })

      it('should filter by search query', () => {
        store.searchQuery = 'sync'
        expect(store.filteredPlugins).toEqual([mockPlugin2])
      })

      it('should combine multiple filters', () => {
        store.categoryFilter = 'backup'
        store.statusFilter = 'configured'
        store.searchQuery = 'test'
        expect(store.filteredPlugins).toEqual([mockPlugin])
      })
    })

    describe('pluginsByCategory', () => {
      it('should group plugins by category', () => {
        const grouped = store.pluginsByCategory
        expect(grouped.backup).toEqual([mockPlugin])
        expect(grouped.sync).toEqual([mockPlugin2])
      })

      it('should sort plugins within categories', () => {
        const plugin3: Plugin = {
          ...mockPlugin2,
          name: 'another-plugin',
          display_name: 'Another Plugin'
        }
        store.plugins = [mockPlugin, plugin3, mockPlugin2]

        const grouped = store.pluginsByCategory
        expect(grouped.sync[0].display_name).toBe('Another Plugin')
        expect(grouped.sync[1].display_name).toBe('Sync Plugin')
      })
    })

    describe('stats', () => {
      it('should calculate statistics correctly', () => {
        const stats = store.stats
        expect(stats.total).toBe(2)
        expect(stats.byCategory.backup).toBe(1)
        expect(stats.byCategory.sync).toBe(1)
        expect(stats.byStatus.configured).toBe(1)
        expect(stats.byStatus.available).toBe(1)
      })
    })

    describe('availableCategories', () => {
      it('should return unique categories with counts', () => {
        const categories = store.availableCategories
        expect(categories).toHaveLength(2)
        expect(categories.find(c => c.value === 'backup')?.count).toBe(1)
        expect(categories.find(c => c.value === 'sync')?.count).toBe(1)
      })
    })

    describe('availableStatuses', () => {
      it('should return unique statuses with counts', () => {
        const statuses = store.availableStatuses
        expect(statuses).toHaveLength(2)
        expect(statuses.find(s => s.value === 'configured')?.count).toBe(1)
        expect(statuses.find(s => s.value === 'available')?.count).toBe(1)
      })
    })
  })

  describe('actions', () => {
    describe('fetchPlugins', () => {
      it('should fetch plugins successfully', async () => {
        const mockResponse = {
          plugins: [mockPlugin, mockPlugin2],
          meta: { count: 2 }
        }
        vi.mocked(pluginApi.listPlugins).mockResolvedValue(mockResponse)

        await store.fetchPlugins()

        expect(store.plugins).toEqual([mockPlugin, mockPlugin2])
        expect(store.meta).toEqual({ count: 2 })
        expect(store.loading).toBe(false)
        expect(store.error).toBe('')
      })

      it('should handle fetch errors', async () => {
        vi.mocked(pluginApi.listPlugins).mockRejectedValue(new Error('Network error'))

        await store.fetchPlugins()

        expect(store.plugins).toEqual([])
        expect(store.loading).toBe(false)
        expect(store.error).toBe('Network error')
      })

      it('should pass filters to API', async () => {
        store.categoryFilter = 'backup'
        store.statusFilter = 'configured'
        store.searchQuery = 'test'

        vi.mocked(pluginApi.listPlugins).mockResolvedValue({ plugins: [], meta: undefined })

        await store.fetchPlugins()

        expect(pluginApi.listPlugins).toHaveBeenCalledWith({
          category: 'backup',
          status: 'configured',
          search: 'test'
        })
      })
    })

    describe('loadPlugin', () => {
      it('should load plugin and schema successfully', async () => {
        vi.mocked(pluginApi.getPlugin).mockResolvedValue(mockPlugin)
        vi.mocked(pluginApi.getPluginSchema).mockResolvedValue(mockSchema)

        const result = await store.loadPlugin('test-plugin')

        expect(result).toEqual(mockPlugin)
        expect(store.currentPlugin).toEqual(mockPlugin)
        expect(store.currentPluginSchema).toEqual(mockSchema)
        expect(store.currentError).toBe('')
      })

      it('should handle plugin not found', async () => {
        vi.mocked(pluginApi.getPlugin).mockRejectedValue(new Error('Plugin not found'))

        await expect(store.loadPlugin('nonexistent')).rejects.toThrow('Plugin not found')
        expect(store.currentError).toBe('Plugin not found')
      })

      it('should continue if schema loading fails', async () => {
        vi.mocked(pluginApi.getPlugin).mockResolvedValue(mockPlugin)
        vi.mocked(pluginApi.getPluginSchema).mockRejectedValue(new Error('Schema not found'))

        const result = await store.loadPlugin('test-plugin')

        expect(result).toEqual(mockPlugin)
        expect(store.currentPlugin).toEqual(mockPlugin)
        expect(store.currentPluginSchema).toBeNull()
      })
    })

    describe('updatePluginConfiguration', () => {
      const mockConfig: PluginConfig = {
        plugin_name: 'test-plugin',
        configuration: {
          endpoint: 'https://updated.example.com',
          timeout: 60,
          enabled: true
        },
        enabled: true
      }

      it('should update configuration successfully', async () => {
        const updatedPlugin = { ...mockPlugin, configuration: mockConfig.configuration }
        store.plugins = [mockPlugin]
        store.currentPlugin = mockPlugin
        store.schemaCache.set('test-plugin', mockSchema)

        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue([])
        vi.mocked(pluginApi.updatePluginConfig).mockResolvedValue(updatedPlugin)

        const result = await store.updatePluginConfiguration('test-plugin', mockConfig)

        expect(result.configuration).toEqual(mockConfig.configuration)
        expect(store.plugins[0].configuration).toEqual(mockConfig.configuration)
        expect(store.currentPlugin?.configuration).toEqual(mockConfig.configuration)
      })

      it('should handle validation errors', async () => {
        store.schemaCache.set('test-plugin', mockSchema)
        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue(['Endpoint is required'])

        await expect(store.updatePluginConfiguration('test-plugin', mockConfig))
          .rejects.toThrow('Configuration validation failed: Endpoint is required')
        expect(store.validationErrors.get('test-plugin')).toEqual(['Endpoint is required'])
      })

      it('should clear validation errors on success', async () => {
        const updatedPlugin = { ...mockPlugin, configuration: mockConfig.configuration }
        store.schemaCache.set('test-plugin', mockSchema)
        store.validationErrors.set('test-plugin', ['Old error'])

        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue([])
        vi.mocked(pluginApi.updatePluginConfig).mockResolvedValue(updatedPlugin)

        await store.updatePluginConfiguration('test-plugin', mockConfig)

        expect(store.validationErrors.has('test-plugin')).toBe(false)
      })
    })

    describe('testPluginConfig', () => {
      const mockTestResult: PluginTestResult = {
        success: true,
        message: 'Test completed successfully',
        response_time_ms: 150
      }

      it('should test plugin successfully', async () => {
        vi.mocked(pluginApi.testPlugin).mockResolvedValue(mockTestResult)

        const result = await store.testPluginConfig('test-plugin', { endpoint: 'https://test.com' })

        expect(result).toEqual(mockTestResult)
        expect(store.testResults.get('test-plugin')).toEqual(mockTestResult)
        expect(store.testing.has('test-plugin')).toBe(false)
      })

      it('should handle test failures', async () => {
        const failedResult: PluginTestResult = {
          success: false,
          message: 'Connection failed',
          error: 'Timeout'
        }
        vi.mocked(pluginApi.testPlugin).mockRejectedValue(new Error('Test failed'))

        await expect(store.testPluginConfig('test-plugin')).rejects.toThrow('Test failed')

        expect(store.testResults.get('test-plugin')).toEqual({
          success: false,
          message: 'Test failed',
          error: 'Test failed'
        })
        expect(store.testing.has('test-plugin')).toBe(false)
      })

      it('should track testing state', async () => {
        let resolveFn: (value: PluginTestResult) => void
        vi.mocked(pluginApi.testPlugin).mockImplementation(
          () => new Promise((resolve) => { resolveFn = resolve })
        )

        const testPromise = store.testPluginConfig('test-plugin')
        expect(store.testing.has('test-plugin')).toBe(true)

        resolveFn(mockTestResult)
        await testPromise

        expect(store.testing.has('test-plugin')).toBe(false)
      })
    })

    describe('togglePluginEnabled', () => {
      it('should toggle plugin state successfully', async () => {
        const disabledPlugin = { ...mockPlugin, status: 'disabled' as const }
        store.plugins = [mockPlugin]
        store.currentPlugin = mockPlugin

        vi.mocked(pluginApi.togglePlugin).mockResolvedValue(disabledPlugin)

        const result = await store.togglePluginEnabled('test-plugin', false)

        expect(result.status).toBe('disabled')
        expect(store.plugins[0].status).toBe('disabled')
        expect(store.currentPlugin?.status).toBe('disabled')
      })

      it('should handle toggle errors', async () => {
        vi.mocked(pluginApi.togglePlugin).mockRejectedValue(new Error('Toggle failed'))

        await expect(store.togglePluginEnabled('test-plugin', false))
          .rejects.toThrow('Toggle failed')
        expect(store.error).toBe('Toggle failed')
      })
    })

    describe('refreshPluginHealth', () => {
      const mockHealth: PluginHealthStatus = {
        healthy: true,
        last_check: '2024-01-01T12:00:00Z',
        response_time_ms: 150
      }

      it('should refresh health status successfully', async () => {
        vi.mocked(pluginApi.getPluginHealth).mockResolvedValue(mockHealth)

        const result = await store.refreshPluginHealth('test-plugin')

        expect(result).toEqual(mockHealth)
        expect(store.healthStatus.get('test-plugin')).toEqual(mockHealth)
      })

      it('should handle health check failures', async () => {
        vi.mocked(pluginApi.getPluginHealth).mockRejectedValue(new Error('Health check failed'))

        const result = await store.refreshPluginHealth('test-plugin')

        expect(result.healthy).toBe(false)
        expect(result.error_message).toBe('Health check failed')
        expect(store.healthStatus.get('test-plugin')?.healthy).toBe(false)
      })
    })

    describe('resetPluginConfiguration', () => {
      it('should reset configuration successfully', async () => {
        const resetPlugin = { ...mockPlugin, configuration: {} }
        store.plugins = [mockPlugin]
        store.currentPlugin = mockPlugin
        store.configurationCache.set('test-plugin', { some: 'config' })
        store.testResults.set('test-plugin', { success: true, message: 'test' })

        vi.mocked(pluginApi.resetPluginConfig).mockResolvedValue(resetPlugin)

        const result = await store.resetPluginConfiguration('test-plugin')

        expect(result.configuration).toEqual({})
        expect(store.configurationCache.has('test-plugin')).toBe(false)
        expect(store.testResults.has('test-plugin')).toBe(false)
        expect(store.plugins[0].configuration).toEqual({})
      })
    })

    describe('generatePluginDefaultConfig', () => {
      it('should generate default configuration', () => {
        store.schemaCache.set('test-plugin', mockSchema)
        vi.mocked(pluginApi.generateDefaultConfig).mockReturnValue({
          endpoint: '',
          timeout: 30,
          enabled: true
        })

        const config = store.generatePluginDefaultConfig('test-plugin')

        expect(config).toEqual({
          endpoint: '',
          timeout: 30,
          enabled: true
        })
        expect(store.configurationCache.get('test-plugin')).toEqual(config)
      })

      it('should return null if no schema available', () => {
        const config = store.generatePluginDefaultConfig('nonexistent')
        expect(config).toBeNull()
      })
    })

    describe('validatePluginConfiguration', () => {
      it('should validate configuration and store errors', () => {
        store.schemaCache.set('test-plugin', mockSchema)
        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue(['Endpoint is required'])

        const errors = store.validatePluginConfiguration('test-plugin', {})

        expect(errors).toEqual(['Endpoint is required'])
        expect(store.validationErrors.get('test-plugin')).toEqual(['Endpoint is required'])
      })
    })

    describe('health monitoring', () => {
      beforeEach(() => {
        vi.useFakeTimers()
      })

      afterEach(() => {
        vi.useRealTimers()
      })

      it('should start health monitoring', () => {
        store.startHealthMonitoring()
        expect(store.healthMonitoring).toBe(true)
      })

      it('should stop health monitoring', () => {
        store.healthMonitoring = true
        store.stopHealthMonitoring()
        expect(store.healthMonitoring).toBe(false)
      })

      it('should monitor configured plugins', async () => {
        store.plugins = [mockPlugin, mockPlugin2] // Only mockPlugin is configured
        store.healthMonitoring = true
        vi.mocked(pluginApi.getPluginHealth).mockResolvedValue({
          healthy: true,
          last_check: '2024-01-01T12:00:00Z'
        })

        await store.monitorPluginHealth()

        // Should only check configured plugins
        expect(pluginApi.getPluginHealth).toHaveBeenCalledTimes(1)
        expect(pluginApi.getPluginHealth).toHaveBeenCalledWith('test-plugin')
      })
    })

    describe('import/export configuration', () => {
      it('should export plugin configuration', () => {
        const config = { endpoint: 'https://test.com', timeout: 60 }
        store.configurationCache.set('test-plugin', config)
        store.plugins = [mockPlugin]

        const exported = store.exportPluginConfig('test-plugin')
        const parsed = JSON.parse(exported!)

        expect(parsed.plugin_name).toBe('test-plugin')
        expect(parsed.plugin_version).toBe('1.0.0')
        expect(parsed.configuration).toEqual(config)
        expect(parsed.exported_at).toBeDefined()
      })

      it('should import plugin configuration', async () => {
        store.plugins = [mockPlugin]
        store.schemaCache.set('test-plugin', mockSchema)
        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue([])

        const configData = {
          plugin_name: 'test-plugin',
          plugin_version: '1.0.0',
          configuration: { endpoint: 'https://imported.com' },
          exported_at: '2024-01-01T12:00:00Z'
        }

        await store.importPluginConfig(JSON.stringify(configData))

        expect(store.configurationCache.get('test-plugin')).toEqual(configData.configuration)
      })

      it('should handle invalid import data', async () => {
        await expect(store.importPluginConfig('invalid json')).rejects.toThrow()
        await expect(store.importPluginConfig('{}')).rejects.toThrow('Invalid configuration format')
      })
    })

    describe('filter management', () => {
      it('should set category filter', () => {
        store.setCategoryFilter('backup')
        expect(store.categoryFilter).toBe('backup')
      })

      it('should set status filter', () => {
        store.setStatusFilter('configured')
        expect(store.statusFilter).toBe('configured')
      })

      it('should set search query', () => {
        store.setSearchQuery('test query')
        expect(store.searchQuery).toBe('test query')
      })
    })

    describe('cleanup methods', () => {
      it('should clear current plugin', () => {
        store.currentPlugin = mockPlugin
        store.currentPluginSchema = mockSchema
        store.currentError = 'some error'

        store.clearCurrentPlugin()

        expect(store.currentPlugin).toBeNull()
        expect(store.currentPluginSchema).toBeNull()
        expect(store.currentError).toBe('')
      })

      it('should clear errors', () => {
        store.error = 'main error'
        store.currentError = 'current error'

        store.clearErrors()

        expect(store.error).toBe('')
        expect(store.currentError).toBe('')
      })

      it('should clear all caches', () => {
        store.configurationCache.set('test', {})
        store.schemaCache.set('test', mockSchema)
        store.validationErrors.set('test', ['error'])
        store.testResults.set('test', { success: true, message: 'test' })
        store.healthStatus.set('test', { healthy: true, last_check: '2024-01-01' })

        store.clearCaches()

        expect(store.configurationCache.size).toBe(0)
        expect(store.schemaCache.size).toBe(0)
        expect(store.validationErrors.size).toBe(0)
        expect(store.testResults.size).toBe(0)
        expect(store.healthStatus.size).toBe(0)
      })
    })
  })
})