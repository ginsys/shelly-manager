import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePluginStore } from '../plugin'
import * as pluginApi from '@/api/plugin'
import type { Plugin, PluginSchema, PluginConfig, PluginTestResult, PluginCategory } from '@/api/plugin'

// Mock the API functions
vi.mock('@/api/plugin', () => ({
  listPlugins: vi.fn(),
  getPlugin: vi.fn(),
  getPluginSchema: vi.fn(),
  getPluginConfig: vi.fn(),
  updatePluginConfig: vi.fn(),
  testPlugin: vi.fn(),
  validatePluginConfig: vi.fn(),
  generateDefaultConfig: vi.fn(),
}))

describe('Plugin Store', () => {
  let store: ReturnType<typeof usePluginStore>

  const mockPluginStatus = {
    available: true,
    configured: true,
    enabled: true,
    error: undefined,
    last_error: undefined,
  }

  const mockPlugin: Plugin = {
    name: 'test-plugin',
    display_name: 'Test Plugin',
    description: 'A test plugin for unit testing',
    version: '1.0.0',
    author: 'Test Author',
    category: 'backup',
    capabilities: ['backup', 'restore'],
    status: mockPluginStatus,
  }

  const mockPlugin2: Plugin = {
    name: 'sync-plugin',
    display_name: 'Sync Plugin',
    description: 'A synchronization plugin',
    version: '2.0.0',
    author: 'Sync Author',
    category: 'sync',
    capabilities: ['sync'],
    status: {
      available: true,
      configured: false,
      enabled: false,
    },
  }

  const mockCategory: PluginCategory = {
    name: 'backup',
    display_name: 'Backup',
    description: 'Backup plugins',
    plugin_count: 1,
  }

  const mockCategory2: PluginCategory = {
    name: 'sync',
    display_name: 'Sync',
    description: 'Sync plugins',
    plugin_count: 1,
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

  const mockConfig: PluginConfig = {
    plugin_name: 'test-plugin',
    enabled: true,
    config: {
      endpoint: 'https://test.example.com',
      timeout: 30,
    }
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
      expect(store.selectedCategory).toBe('')
      expect(store.statusFilter).toBe('')
      expect(store.searchQuery).toBe('')
    })
  })

  describe('getters', () => {
    beforeEach(() => {
      store.plugins = [mockPlugin, mockPlugin2]
      store.categories = [mockCategory, mockCategory2]
    })

    describe('filteredPlugins', () => {
      it('should return all plugins when no filters applied', () => {
        expect(store.filteredPlugins).toEqual([mockPlugin, mockPlugin2])
      })

      it('should filter by category', () => {
        store.selectedCategory = 'backup'
        expect(store.filteredPlugins).toEqual([mockPlugin])
      })

      it('should filter by status - configured', () => {
        store.statusFilter = 'configured'
        expect(store.filteredPlugins).toEqual([mockPlugin])
      })

      it('should filter by status - available', () => {
        store.statusFilter = 'available'
        expect(store.filteredPlugins).toEqual([mockPlugin2])
      })

      it('should filter by search query', () => {
        store.searchQuery = 'sync'
        expect(store.filteredPlugins).toEqual([mockPlugin2])
      })

      it('should combine multiple filters', () => {
        store.selectedCategory = 'backup'
        store.statusFilter = 'configured'
        store.searchQuery = 'test'
        expect(store.filteredPlugins).toEqual([mockPlugin])
      })
    })

    describe('pluginsByCategory', () => {
      it('should group plugins by category', () => {
        const grouped = store.pluginsByCategory
        expect(grouped.backup).toBeDefined()
        expect(grouped.sync).toBeDefined()
        expect(grouped.backup.map(p => p.name)).toContain('test-plugin')
        expect(grouped.sync.map(p => p.name)).toContain('sync-plugin')
      })
    })

    describe('pluginStats', () => {
      it('should calculate statistics correctly', () => {
        const stats = store.pluginStats
        expect(stats.total).toBe(2)
        expect(stats.configured).toBe(1)
        expect(stats.available).toBe(1)
      })
    })

    describe('isPluginTesting', () => {
      it('should return testing state for plugin', () => {
        expect(store.isPluginTesting('test-plugin')).toBe(false)
        store.testingPlugins.add('test-plugin')
        expect(store.isPluginTesting('test-plugin')).toBe(true)
      })
    })

    describe('getTestResult', () => {
      it('should return test result for plugin', () => {
        const result: PluginTestResult = { success: true, message: 'OK' }
        store.testResults.set('test-plugin', result)
        expect(store.getTestResult('test-plugin')).toEqual(result)
      })
    })
  })

  describe('actions', () => {
    describe('fetchPlugins', () => {
      it('should fetch plugins successfully', async () => {
        const mockResponse = {
          plugins: [mockPlugin, mockPlugin2],
          categories: [mockCategory, mockCategory2],
          meta: { count: 2 }
        }
        vi.mocked(pluginApi.listPlugins).mockResolvedValue(mockResponse)

        await store.fetchPlugins()

        expect(store.plugins).toEqual([mockPlugin, mockPlugin2])
        expect(store.categories).toEqual([mockCategory, mockCategory2])
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

      it('should pass category filter to API', async () => {
        vi.mocked(pluginApi.listPlugins).mockResolvedValue({ plugins: [], categories: [], meta: undefined })

        await store.fetchPlugins('backup')

        expect(pluginApi.listPlugins).toHaveBeenCalledWith('backup')
      })
    })

    describe('loadPluginDetails', () => {
      it('should load plugin details successfully', async () => {
        vi.mocked(pluginApi.getPlugin).mockResolvedValue(mockPlugin)
        vi.mocked(pluginApi.getPluginSchema).mockResolvedValue(mockSchema)
        vi.mocked(pluginApi.getPluginConfig).mockResolvedValue(mockConfig)
        vi.mocked(pluginApi.generateDefaultConfig).mockReturnValue({ endpoint: '', timeout: 30 })

        const result = await store.loadPluginDetails('test-plugin')

        expect(result).toEqual(mockPlugin)
        expect(store.currentPlugin).toEqual(mockPlugin)
        expect(store.currentPluginSchema).toEqual(mockSchema)
        expect(store.currentPluginConfig).toEqual(mockConfig)
        expect(store.currentError).toBe('')
      })

      it('should handle plugin not found', async () => {
        vi.mocked(pluginApi.getPlugin).mockRejectedValue(new Error('Plugin not found'))

        await expect(store.loadPluginDetails('nonexistent')).rejects.toThrow('Plugin not found')
        expect(store.currentError).toBe('Plugin not found')
      })

      it('should continue if schema loading fails', async () => {
        vi.mocked(pluginApi.getPlugin).mockResolvedValue(mockPlugin)
        vi.mocked(pluginApi.getPluginSchema).mockRejectedValue(new Error('Schema not found'))
        vi.mocked(pluginApi.getPluginConfig).mockResolvedValue(mockConfig)

        const result = await store.loadPluginDetails('test-plugin')

        expect(result).toEqual(mockPlugin)
        expect(store.currentPlugin).toEqual(mockPlugin)
        expect(store.currentPluginSchema).toBeNull()
      })
    })

    describe('updateConfiguration', () => {
      const newConfig = {
        endpoint: 'https://updated.example.com',
        timeout: 60,
      }

      it('should update configuration successfully', async () => {
        const updatedConfig = { ...mockConfig, config: newConfig }
        store.plugins = [mockPlugin]
        store.currentPlugin = mockPlugin

        vi.mocked(pluginApi.updatePluginConfig).mockResolvedValue(updatedConfig)

        const result = await store.updateConfiguration('test-plugin', newConfig, true)

        expect(result).toEqual(updatedConfig)
        expect(store.configurationCache.get('test-plugin')).toEqual(updatedConfig)
      })

      it('should handle update errors', async () => {
        vi.mocked(pluginApi.updatePluginConfig).mockRejectedValue(new Error('Update failed'))

        await expect(store.updateConfiguration('test-plugin', newConfig, true))
          .rejects.toThrow('Update failed')
        expect(store.currentError).toBe('Update failed')
      })
    })

    describe('testPluginConfiguration', () => {
      const mockTestResult: PluginTestResult = {
        success: true,
        message: 'Test completed successfully',
      }

      it('should test plugin successfully', async () => {
        vi.mocked(pluginApi.testPlugin).mockResolvedValue(mockTestResult)

        const result = await store.testPluginConfiguration('test-plugin', { endpoint: 'https://test.com' })

        expect(result).toEqual(mockTestResult)
        expect(store.testResults.get('test-plugin')).toEqual(mockTestResult)
        expect(store.testingPlugins.has('test-plugin')).toBe(false)
      })

      it('should handle test failures', async () => {
        vi.mocked(pluginApi.testPlugin).mockRejectedValue(new Error('Test failed'))

        await expect(store.testPluginConfiguration('test-plugin')).rejects.toThrow('Test failed')

        expect(store.testResults.get('test-plugin')).toEqual({
          success: false,
          message: 'Test failed',
          errors: ['Test failed']
        })
        expect(store.testingPlugins.has('test-plugin')).toBe(false)
      })

      it('should track testing state', async () => {
        let resolveFn: (value: PluginTestResult) => void
        vi.mocked(pluginApi.testPlugin).mockImplementation(
          () => new Promise((resolve) => { resolveFn = resolve })
        )

        const testPromise = store.testPluginConfiguration('test-plugin')
        expect(store.testingPlugins.has('test-plugin')).toBe(true)

        resolveFn!(mockTestResult)
        await testPromise

        expect(store.testingPlugins.has('test-plugin')).toBe(false)
      })
    })

    describe('togglePlugin', () => {
      it('should toggle plugin state successfully', async () => {
        store.plugins = [{ ...mockPlugin }]
        const updatedConfig = { ...mockConfig, enabled: false }

        vi.mocked(pluginApi.getPluginConfig).mockResolvedValue(mockConfig)
        vi.mocked(pluginApi.updatePluginConfig).mockResolvedValue(updatedConfig)

        await store.togglePlugin('test-plugin')

        expect(pluginApi.updatePluginConfig).toHaveBeenCalledWith('test-plugin', mockConfig.config, false)
      })

      it('should handle toggle errors', async () => {
        store.plugins = [{ ...mockPlugin }]
        vi.mocked(pluginApi.getPluginConfig).mockResolvedValue(mockConfig)
        vi.mocked(pluginApi.updatePluginConfig).mockRejectedValue(new Error('Toggle failed'))

        await expect(store.togglePlugin('test-plugin'))
          .rejects.toThrow('Toggle failed')
        expect(store.error).toBe('Toggle failed')
      })
    })

    describe('validateConfiguration', () => {
      it('should validate configuration with schema', () => {
        store.currentPluginSchema = mockSchema
        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue(['Endpoint is required'])

        store.configFormData = {}
        const isValid = store.validateConfiguration()

        expect(isValid).toBe(false)
        expect(store.configValidationErrors).toEqual(['Endpoint is required'])
      })

      it('should return true when no schema available', () => {
        store.currentPluginSchema = null

        const isValid = store.validateConfiguration()

        expect(isValid).toBe(true)
        expect(store.configValidationErrors).toEqual([])
      })
    })

    describe('filter management', () => {
      it('should set category filter', () => {
        store.setCategory('backup')
        expect(store.selectedCategory).toBe('backup')
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
        store.configFormData = { endpoint: 'test' }

        store.clearCurrentPlugin()

        expect(store.currentPlugin).toBeNull()
        expect(store.currentPluginSchema).toBeNull()
        expect(store.currentError).toBe('')
        expect(store.configFormData).toEqual({})
      })

      it('should clear errors', () => {
        store.error = 'main error'
        store.currentError = 'current error'

        store.clearErrors()

        expect(store.error).toBe('')
        expect(store.currentError).toBe('')
      })

      it('should clear test results', () => {
        store.testResults.set('test', { success: true, message: 'test' })

        store.clearTestResults()

        expect(store.testResults.size).toBe(0)
      })
    })

    describe('import/export configuration', () => {
      it('should export plugin configuration', () => {
        store.configFormData = { endpoint: 'https://test.com', timeout: 60 }

        const exported = store.exportConfiguration()
        const parsed = JSON.parse(exported)

        expect(parsed.endpoint).toBe('https://test.com')
        expect(parsed.timeout).toBe(60)
      })

      it('should import plugin configuration', () => {
        const configJson = JSON.stringify({ endpoint: 'https://imported.com' })

        const result = store.importConfiguration(configJson)

        expect(result).toBe(true)
        expect(store.configFormData.endpoint).toBe('https://imported.com')
      })

      it('should handle invalid import data', () => {
        const result = store.importConfiguration('invalid json')
        expect(result).toBe(false)
        expect(store.configValidationErrors.length).toBeGreaterThan(0)
      })
    })

    describe('config modal management', () => {
      it('should open config modal', () => {
        store.openConfigModal('test-plugin')

        expect(store.showConfigModal).toBe(true)
        expect(store.configModalPlugin).toBe('test-plugin')
      })

      it('should close config modal', () => {
        store.showConfigModal = true
        store.configModalPlugin = 'test-plugin'
        store.configFormData = { endpoint: 'test' }

        store.closeConfigModal()

        expect(store.showConfigModal).toBe(false)
        expect(store.configModalPlugin).toBe('')
        expect(store.configFormData).toEqual({})
      })
    })

    describe('form field updates', () => {
      it('should update simple form field', () => {
        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue([])
        store.currentPluginSchema = mockSchema

        store.updateFormField('endpoint', 'https://new.example.com')

        expect(store.configFormData.endpoint).toBe('https://new.example.com')
      })

      it('should update nested form field', () => {
        vi.mocked(pluginApi.validatePluginConfig).mockReturnValue([])
        store.currentPluginSchema = mockSchema

        store.updateFormField('auth.username', 'testuser')

        expect(store.configFormData.auth.username).toBe('testuser')
      })
    })

    describe('refresh', () => {
      it('should clear caches and refetch', async () => {
        store.configurationCache.set('test', mockConfig)
        store.schemaCache.set('test', mockSchema)
        store.testResults.set('test', { success: true, message: 'test' })

        vi.mocked(pluginApi.listPlugins).mockResolvedValue({ plugins: [], categories: [], meta: undefined })

        await store.refresh()

        expect(store.configurationCache.size).toBe(0)
        expect(store.schemaCache.size).toBe(0)
        expect(store.testResults.size).toBe(0)
        expect(pluginApi.listPlugins).toHaveBeenCalled()
      })
    })
  })
})
