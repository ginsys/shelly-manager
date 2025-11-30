import { defineStore } from 'pinia'
import { 
  listPlugins,
  getPlugin,
  getPluginSchema,
  testPlugin,
  getPluginConfig,
  updatePluginConfig,
  validatePluginConfig,
  generateDefaultConfig,
  type Plugin,
  type PluginSchema,
  type PluginConfig,
  type PluginCategory,
  type PluginTestResult,
  type ListPluginsResult
} from '@/api/plugin'
import type { Metadata } from '@/api/types'

export const usePluginStore = defineStore('plugin', {
  state: () => ({
    // Plugin list state
    plugins: [] as Plugin[],
    categories: [] as PluginCategory[],
    loading: false,
    error: '' as string | '',
    meta: undefined as Metadata | undefined,
    
    // Current plugin state (for detailed view)
    currentPlugin: null as Plugin | null,
    currentPluginSchema: null as PluginSchema | null,
    currentPluginConfig: null as PluginConfig | null,
    currentLoading: false,
    currentError: '' as string | '',
    
    // Configuration state
    configurationCache: new Map<string, PluginConfig>(),
    schemaCache: new Map<string, PluginSchema>(),
    
    // Filter and search state
    selectedCategory: '' as string,
    searchQuery: '' as string,
    statusFilter: '' as string, // 'all', 'configured', 'available', 'error'
    
    // Testing state
    testingPlugins: new Set<string>(),
    testResults: new Map<string, PluginTestResult>(),
    
    // UI state
    showConfigModal: false,
    configModalPlugin: '' as string,
    configFormData: {} as Record<string, any>,
    configValidationErrors: [] as string[],
  }),

  getters: {
    /**
     * Get plugins filtered by current filters
     */
    filteredPlugins: (state) => {
      let filtered = state.plugins
      
      // Filter by category
      if (state.selectedCategory) {
        filtered = filtered.filter(p => p.category === state.selectedCategory)
      }
      
      // Filter by search query
      if (state.searchQuery) {
        const query = state.searchQuery.toLowerCase()
        filtered = filtered.filter(p =>
          p.name.toLowerCase().includes(query) ||
          p.display_name.toLowerCase().includes(query) ||
          p.description.toLowerCase().includes(query) ||
          p.capabilities.some(cap => cap.toLowerCase().includes(query))
        )
      }
      
      // Filter by status
      if (state.statusFilter) {
        switch (state.statusFilter) {
          case 'configured':
            filtered = filtered.filter(p => p.status.configured && p.status.enabled)
            break
          case 'available':
            filtered = filtered.filter(p => p.status.available && !p.status.configured)
            break
          case 'error':
            filtered = filtered.filter(p => p.status.error)
            break
          case 'disabled':
            filtered = filtered.filter(p => p.status.configured && !p.status.enabled)
            break
        }
      }
      
      return filtered
    },

    /**
     * Get plugins grouped by category
     */
    pluginsByCategory(state): Record<string, Plugin[]> {
      const groups: Record<string, Plugin[]> = {}

      // Use the filteredPlugins getter to populate categories
      const list = (this as any).filteredPlugins as Plugin[]
      for (const plugin of list) {
        if (!groups[plugin.category]) {
          groups[plugin.category] = []
        }
        groups[plugin.category].push(plugin)
      }

      // Sort plugins within each category
      for (const category in groups) {
        groups[category].sort((a, b) => {
          // Sort by status first (configured > available > error > unavailable)
          const statusOrder = { ready: 0, 'not-configured': 1, disabled: 2, error: 3, unavailable: 4 }
          const aStatus = (this as any).getPluginStatusClass(a.status) as string
          const bStatus = (this as any).getPluginStatusClass(b.status) as string

          if (statusOrder[aStatus as keyof typeof statusOrder] !== statusOrder[bStatus as keyof typeof statusOrder]) {
            return (statusOrder[aStatus as keyof typeof statusOrder] || 5) - (statusOrder[bStatus as keyof typeof statusOrder] || 5)
          }

          // Then by display name
          return a.display_name.localeCompare(b.display_name)
        })
      }

      return groups
    },

    /**
     * Get plugin statistics
     */
    pluginStats: (state) => {
      const total = state.plugins.length
      const configured = state.plugins.filter(p => p.status.configured && p.status.enabled).length
      const available = state.plugins.filter(p => p.status.available && !p.status.configured).length
      const disabled = state.plugins.filter(p => p.status.configured && !p.status.enabled).length
      const error = state.plugins.filter(p => p.status.error).length
      const unavailable = state.plugins.filter(p => !p.status.available).length
      
      const byCategory = state.categories.reduce((acc, cat) => {
        acc[cat.name] = cat.plugin_count
        return acc
      }, {} as Record<string, number>)
      
      return {
        total,
        configured,
        available,
        disabled,
        error,
        unavailable,
        byCategory
      }
    },

    /**
     * Check if a plugin is currently being tested
     */
    isPluginTesting: (state) => (name: string) => {
      return state.testingPlugins.has(name)
    },

    /**
     * Get test result for a plugin
     */
    getTestResult: (state) => (name: string) => {
      return state.testResults.get(name)
    },

    /**
     * Get plugin status display class
     */
    getPluginStatusClass: () => (status: Plugin['status']) => {
      if (!status.available) return 'unavailable'
      if (status.error) return 'error'
      if (!status.configured) return 'not-configured'
      if (!status.enabled) return 'disabled'
      return 'ready'
    },

    /**
     * Get cached plugin configuration
     */
    getCachedConfig: (state) => (name: string) => {
      return state.configurationCache.get(name)
    },

    /**
     * Get cached plugin schema
     */
    getCachedSchema: (state) => (name: string) => {
      return state.schemaCache.get(name)
    },

    /**
     * Check if configuration form has unsaved changes
     */
    hasConfigChanges: (state) => {
      if (!state.currentPluginConfig) return false
      
      return JSON.stringify(state.configFormData) !== JSON.stringify(state.currentPluginConfig.config)
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
        // Set timeout for API call to prevent hanging
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
     * Load detailed information for a specific plugin
     */
    async loadPluginDetails(name: string) {
      this.currentLoading = true
      this.currentError = ''
      
      try {
        // Load plugin details
        this.currentPlugin = await getPlugin(name)
        
        // Load schema and configuration in parallel
        const [schema, config] = await Promise.all([
          this.loadPluginSchema(name),
          this.loadPluginConfig(name)
        ])
        
        this.currentPluginSchema = schema
        this.currentPluginConfig = config
        
        // Initialize form data with current config or defaults
        if (config?.config) {
          this.configFormData = { ...config.config }
        } else if (schema) {
          this.configFormData = generateDefaultConfig(schema)
        } else {
          this.configFormData = {}
        }
        
        this.configValidationErrors = []
        
        return this.currentPlugin
      } catch (e: any) {
        this.currentError = e?.message || 'Failed to load plugin details'
        throw e
      } finally {
        this.currentLoading = false
      }
    },

    /**
     * Load plugin schema with caching
     */
    async loadPluginSchema(name: string): Promise<PluginSchema | null> {
      // Check cache first
      if (this.schemaCache.has(name)) {
        return this.schemaCache.get(name)!
      }
      
      try {
        const schema = await getPluginSchema(name)
        this.schemaCache.set(name, schema)
        return schema
      } catch (e) {
        // Schema is optional, don't throw
        console.warn(`Failed to load schema for plugin ${name}:`, e)
        return null
      }
    },

    /**
     * Load plugin configuration with caching
     */
    async loadPluginConfig(name: string): Promise<PluginConfig | null> {
      // Check cache first
      if (this.configurationCache.has(name)) {
        return this.configurationCache.get(name)!
      }
      
      try {
        const config = await getPluginConfig(name)
        this.configurationCache.set(name, config)
        return config
      } catch (e) {
        // Configuration might not exist yet
        return null
      }
    },

    /**
     * Update plugin configuration
     */
    async updateConfiguration(name: string, config: Record<string, any>, enabled: boolean = true) {
      this.currentLoading = true
      this.currentError = ''
      
      try {
        const updatedConfig = await updatePluginConfig(name, config, enabled)
        
        // Update caches
        this.configurationCache.set(name, updatedConfig)
        
        // Update current config if it matches
        if (this.currentPlugin?.name === name) {
          this.currentPluginConfig = updatedConfig
          this.configFormData = { ...config }
        }
        
        // Update plugin status in the list
        const pluginIndex = this.plugins.findIndex(p => p.name === name)
        if (pluginIndex !== -1) {
          this.plugins[pluginIndex].status.configured = true
          this.plugins[pluginIndex].status.enabled = enabled
          this.plugins[pluginIndex].status.error = undefined
        }
        
        return updatedConfig
      } catch (e: any) {
        this.currentError = e?.message || 'Failed to update plugin configuration'
        throw e
      } finally {
        this.currentLoading = false
      }
    },

    /**
     * Test plugin configuration
     */
    async testPluginConfiguration(name: string, config?: Record<string, any>): Promise<PluginTestResult> {
      this.testingPlugins.add(name)
      
      try {
        const testConfig = config || this.configFormData
        const result = await testPlugin(name, testConfig)
        
        // Store result for display
        this.testResults.set(name, result)
        
        return result
      } catch (e: any) {
        const errorResult: PluginTestResult = {
          success: false,
          message: e?.message || 'Test failed',
          errors: [e?.message || 'Test failed']
        }
        this.testResults.set(name, errorResult)
        throw e
      } finally {
        this.testingPlugins.delete(name)
      }
    },

    /**
     * Validate configuration form data
     */
    validateConfiguration(schema?: PluginSchema) {
      const schemaToUse = schema || this.currentPluginSchema
      if (!schemaToUse) {
        this.configValidationErrors = []
        return true
      }
      
      this.configValidationErrors = validatePluginConfig(this.configFormData, schemaToUse)
      return this.configValidationErrors.length === 0
    },

    /**
     * Reset configuration form to defaults
     */
    resetConfigurationForm() {
      if (this.currentPluginConfig?.config) {
        this.configFormData = { ...this.currentPluginConfig.config }
      } else if (this.currentPluginSchema) {
        this.configFormData = generateDefaultConfig(this.currentPluginSchema)
      } else {
        this.configFormData = {}
      }
      
      this.configValidationErrors = []
    },

    /**
     * Set search and filter parameters
     */
    setCategory(category: string) {
      this.selectedCategory = category
    },

    setSearchQuery(query: string) {
      this.searchQuery = query
    },

    setStatusFilter(filter: string) {
      this.statusFilter = filter
    },

    /**
     * Open configuration modal
     */
    openConfigModal(pluginName: string) {
      this.configModalPlugin = pluginName
      this.showConfigModal = true
    },

    /**
     * Close configuration modal
     */
    closeConfigModal() {
      this.showConfigModal = false
      this.configModalPlugin = ''
      this.configFormData = {}
      this.configValidationErrors = []
    },

    /**
     * Clear current plugin details
     */
    clearCurrentPlugin() {
      this.currentPlugin = null
      this.currentPluginSchema = null
      this.currentPluginConfig = null
      this.currentError = ''
      this.configFormData = {}
      this.configValidationErrors = []
    },

    /**
     * Clear all errors
     */
    clearErrors() {
      this.error = ''
      this.currentError = ''
    },

    /**
     * Clear test results
     */
    clearTestResults() {
      this.testResults.clear()
    },

    /**
     * Refresh plugin list
     */
    async refresh() {
      // Clear caches to force fresh data
      this.configurationCache.clear()
      this.schemaCache.clear()
      this.testResults.clear()
      
      await this.fetchPlugins(this.selectedCategory || undefined)
    },

    /**
     * Update form field value
     */
    updateFormField(field: string, value: any) {
      // Security: Block prototype pollution attacks
      // Reject dangerous property names that could pollute Object.prototype
      const dangerousKeys = ['__proto__', 'constructor', 'prototype']
      const fieldParts = field.split('.')

      // Check if any part of the field path is dangerous
      if (fieldParts.some(part => dangerousKeys.includes(part))) {
        console.warn('Blocked potential prototype pollution attempt:', field)
        return
      }

      if (fieldParts.length === 1) {
        this.configFormData[field] = value
      } else {
        // Create nested object structure
        let current = this.configFormData
        for (let i = 0; i < fieldParts.length - 1; i++) {
          const part = fieldParts[i]
          if (!Object.prototype.hasOwnProperty.call(current, part)) {
            current[part] = {}
          }
          current = current[part]
        }
        current[fieldParts[fieldParts.length - 1]] = value
      }

      // Revalidate configuration
      this.validateConfiguration()
    },

    /**
     * Import configuration from JSON
     */
    importConfiguration(configJson: string): boolean {
      try {
        const config = JSON.parse(configJson)
        if (typeof config !== 'object' || Array.isArray(config)) {
          throw new Error('Configuration must be an object')
        }
        
        this.configFormData = { ...config }
        this.validateConfiguration()
        return true
      } catch (e: any) {
        this.configValidationErrors = [`Invalid JSON: ${e.message}`]
        return false
      }
    },

    /**
     * Export current configuration as JSON
     */
    exportConfiguration(): string {
      return JSON.stringify(this.configFormData, null, 2)
    },

    /**
     * Toggle plugin enabled state
     */
    async togglePlugin(name: string) {
      const plugin = this.plugins.find(p => p.name === name)
      if (!plugin || !plugin.status.configured) return
      
      try {
        const config = await this.loadPluginConfig(name)
        if (!config) return
        
        await this.updateConfiguration(name, config.config, !plugin.status.enabled)
      } catch (e: any) {
        this.error = e?.message || `Failed to toggle plugin ${name}`
        throw e
      }
    },
  },
})
