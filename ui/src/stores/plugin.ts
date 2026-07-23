import { defineStore } from 'pinia'
import {
  listPlugins,
  getPluginSchema,
  type Plugin,
  type PluginSchema,
  type PluginCategory,
  type ListPluginsResult
} from '@/api/plugin'
import type { Metadata } from '@/api/types'

// Plugins are read-only in the UI: list, per-category grouping, and read-only
// schema inspection. Configuration editing, connection testing and enable/disable
// were removed in #264 because they hit backend routes that do not exist
// (`/config`, `/test`) and there is no server-side model for stored plugin
// config. A real product model is tracked as a separate design vertical.
export const usePluginStore = defineStore('plugin', {
  state: () => ({
    // Plugin list state
    plugins: [] as Plugin[],
    categories: [] as PluginCategory[],
    loading: false,
    error: '' as string | '',
    meta: undefined as Metadata | undefined,

    // Filter and search state
    selectedCategory: '' as string,
    searchQuery: '' as string,
  }),

  getters: {
    /**
     * Get plugins filtered by category and search query. Status is
     * backend-hardcoded, so there is no status filter (#266).
     */
    filteredPlugins: (state) => {
      let filtered = state.plugins

      if (state.selectedCategory) {
        filtered = filtered.filter(p => p.category === state.selectedCategory)
      }

      if (state.searchQuery) {
        const query = state.searchQuery.toLowerCase()
        filtered = filtered.filter(p =>
          p.name.toLowerCase().includes(query) ||
          p.display_name.toLowerCase().includes(query) ||
          p.description.toLowerCase().includes(query) ||
          p.capabilities.some(cap => cap.toLowerCase().includes(query))
        )
      }

      return filtered
    },

    /**
     * Get plugins grouped by category, sorted by display name.
     */
    pluginsByCategory(): Record<string, Plugin[]> {
      const groups: Record<string, Plugin[]> = {}

      const list = (this as any).filteredPlugins as Plugin[]
      for (const plugin of list) {
        if (!groups[plugin.category]) {
          groups[plugin.category] = []
        }
        groups[plugin.category].push(plugin)
      }

      for (const category in groups) {
        groups[category].sort((a, b) => a.display_name.localeCompare(b.display_name))
      }

      return groups
    },

    /**
     * Get plugin statistics. Only truthful counts: total registered plugins and
     * per-category counts. Configured/enabled/error tallies were fiction and
     * were removed in #266.
     */
    pluginStats: (state) => {
      const byCategory = state.categories.reduce((acc, cat) => {
        acc[cat.name] = cat.plugin_count
        return acc
      }, {} as Record<string, number>)

      return {
        total: state.plugins.length,
        byCategory
      }
    },
  },

  actions: {
    /**
     * Fetch all plugins and categories with optimized loading
     */
    async fetchPlugins(category?: string) {
      this.loading = true
      this.error = ''

      try {
        const timeoutPromise = new Promise((_, reject) =>
          setTimeout(() => reject(new Error('Plugin loading timeout')), 8000)
        )

        const apiPromise = listPlugins(category)

        const result: ListPluginsResult = await Promise.race([apiPromise, timeoutPromise]) as ListPluginsResult

        this.plugins = result.plugins
        this.categories = result.categories
        this.meta = result.meta
      } catch (e: any) {
        this.error = e?.message || 'Failed to load plugins'
        this.plugins = []
        this.categories = []
      } finally {
        this.loading = false
      }
    },

    /**
     * Fetch a plugin's configuration schema for read-only inspection. Not
     * cached: plugin schemas are compiled into the server and constant, so a
     * fetch per "View schema" click is cheap and always current. Callers guard
     * their own view against out-of-order responses (see the schema modal's
     * request token in PluginManagementPage). Throws on load failure so callers
     * can distinguish a fetch error from a plugin that publishes no schema.
     */
    async loadPluginSchema(name: string): Promise<PluginSchema> {
      return getPluginSchema(name)
    },

    setCategory(category: string) {
      this.selectedCategory = category
    },

    setSearchQuery(query: string) {
      this.searchQuery = query
    },

    clearErrors() {
      this.error = ''
    },

    /**
     * Refresh plugin list
     */
    async refresh() {
      await this.fetchPlugins(this.selectedCategory || undefined)
    },
  },
})
