import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePluginStore } from '../plugin'
import * as pluginApi from '@/api/plugin'
import type { Plugin, PluginSchema, PluginCategory } from '@/api/plugin'

// Plugins are read-only (#264): the store exposes list/filter/grouping and
// read-only schema inspection only. Configuration, testing and enable/disable
// were removed as unbacked, so this suite covers just the surviving surface.
vi.mock('@/api/plugin', () => ({
  listPlugins: vi.fn(),
  getPluginSchema: vi.fn(),
}))

describe('Plugin Store', () => {
  let store: ReturnType<typeof usePluginStore>

  const mockPlugin: Plugin = {
    name: 'test-plugin',
    display_name: 'Test Plugin',
    description: 'A test plugin for unit testing',
    version: '1.0.0',
    category: 'backup',
    capabilities: ['backup', 'restore'],
    status: { available: true, configured: true, enabled: true },
  }

  const mockPlugin2: Plugin = {
    name: 'sync-plugin',
    display_name: 'Sync Plugin',
    description: 'A synchronization plugin',
    version: '2.0.0',
    category: 'sync',
    capabilities: ['sync'],
    status: { available: true, configured: false, enabled: false },
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
      endpoint: { type: 'string', description: 'API endpoint URL' },
      timeout: { type: 'number', description: 'Request timeout in seconds', default: 30 },
    },
    required: ['endpoint'],
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
      expect(store.selectedCategory).toBe('')
      expect(store.searchQuery).toBe('')
      expect(store.schemaCache.size).toBe(0)
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

      it('should filter by search query', () => {
        store.searchQuery = 'sync'
        expect(store.filteredPlugins).toEqual([mockPlugin2])
      })

      it('should combine category and search filters', () => {
        store.selectedCategory = 'backup'
        store.searchQuery = 'test'
        expect(store.filteredPlugins).toEqual([mockPlugin])
      })
    })

    describe('pluginsByCategory', () => {
      it('should group plugins by category', () => {
        const grouped = store.pluginsByCategory
        expect(grouped.backup.map(p => p.name)).toContain('test-plugin')
        expect(grouped.sync.map(p => p.name)).toContain('sync-plugin')
      })
    })

    describe('pluginStats', () => {
      it('reports only truthful counts: total and per-category', () => {
        const stats = store.pluginStats
        expect(stats.total).toBe(2)
        expect(stats.byCategory).toEqual({ backup: 1, sync: 1 })
        expect('configured' in stats).toBe(false)
        expect('error' in stats).toBe(false)
      })
    })
  })

  describe('actions', () => {
    describe('fetchPlugins', () => {
      it('should fetch plugins successfully', async () => {
        vi.mocked(pluginApi.listPlugins).mockResolvedValue({
          plugins: [mockPlugin, mockPlugin2],
          categories: [mockCategory, mockCategory2],
          meta: { count: 2 } as any,
        })

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

    describe('loadPluginSchema', () => {
      it('loads and caches the schema for read-only inspection', async () => {
        vi.mocked(pluginApi.getPluginSchema).mockResolvedValue(mockSchema)

        const result = await store.loadPluginSchema('test-plugin')

        expect(result).toEqual(mockSchema)
        expect(store.getCachedSchema('test-plugin')).toEqual(mockSchema)
      })

      it('returns the cached schema without a second request', async () => {
        vi.mocked(pluginApi.getPluginSchema).mockResolvedValue(mockSchema)

        await store.loadPluginSchema('test-plugin')
        await store.loadPluginSchema('test-plugin')

        expect(pluginApi.getPluginSchema).toHaveBeenCalledTimes(1)
      })

      it('propagates load failures (so callers can show an error)', async () => {
        vi.mocked(pluginApi.getPluginSchema).mockRejectedValue(new Error('Schema not available'))

        await expect(store.loadPluginSchema('test-plugin')).rejects.toThrow('Schema not available')
      })

      it('cache is first-writer-wins: a late response does not overwrite a cached one', async () => {
        const resolvers: Array<(s: PluginSchema) => void> = []
        vi.mocked(pluginApi.getPluginSchema).mockImplementation(
          () => new Promise<PluginSchema>(res => { resolvers.push(res) })
        )
        const first: PluginSchema = { ...mockSchema, title: 'first' }
        const second: PluginSchema = { ...mockSchema, title: 'second' }

        // Two concurrent loads for the same plugin (cache empty for both).
        const p1 = store.loadPluginSchema('x')
        const p2 = store.loadPluginSchema('x')
        expect(pluginApi.getPluginSchema).toHaveBeenCalledTimes(2)

        // The second request resolves first and populates the cache.
        resolvers[1](second)
        await expect(p2).resolves.toEqual(second)
        expect(store.getCachedSchema('x')).toEqual(second)

        // The first request resolves last: it must NOT overwrite the cache, and
        // it returns the already-cached value.
        resolvers[0](first)
        await expect(p1).resolves.toEqual(second)
        expect(store.getCachedSchema('x')).toEqual(second)
      })

      it('a response resolving after refresh() does not repopulate the cleared cache', async () => {
        const resolvers: Array<(s: PluginSchema) => void> = []
        vi.mocked(pluginApi.getPluginSchema).mockImplementation(
          () => new Promise<PluginSchema>(res => { resolvers.push(res) })
        )
        vi.mocked(pluginApi.listPlugins).mockResolvedValue({ plugins: [], categories: [], meta: undefined })
        const stale: PluginSchema = { ...mockSchema, title: 'stale' }

        // Schema load in flight, then a refresh clears + invalidates.
        const p = store.loadPluginSchema('x')
        await store.refresh()

        // The pre-refresh response resolves late: cache must stay empty.
        resolvers[0](stale)
        await p
        expect(store.getCachedSchema('x')).toBeUndefined()
      })
    })

    describe('filter management', () => {
      it('should set category filter', () => {
        store.setCategory('backup')
        expect(store.selectedCategory).toBe('backup')
      })

      it('should set search query', () => {
        store.setSearchQuery('test query')
        expect(store.searchQuery).toBe('test query')
      })
    })

    describe('clearErrors', () => {
      it('should clear the error', () => {
        store.error = 'main error'
        store.clearErrors()
        expect(store.error).toBe('')
      })
    })

    describe('refresh', () => {
      it('should clear the schema cache and refetch', async () => {
        store.schemaCache.set('test', mockSchema)
        vi.mocked(pluginApi.listPlugins).mockResolvedValue({ plugins: [], categories: [], meta: undefined })

        await store.refresh()

        expect(store.schemaCache.size).toBe(0)
        expect(pluginApi.listPlugins).toHaveBeenCalled()
      })
    })
  })
})
