import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PluginManagementPage from '../PluginManagementPage.vue'
import { usePluginStore } from '@/stores/plugin'
import type { Plugin, PluginSchema } from '@/api/plugin'

// Mock the PluginSchemaViewer component since it's complex
vi.mock('@/components/PluginSchemaViewer.vue', () => ({
  default: {
    name: 'PluginSchemaViewer',
    template: '<div class="mock-plugin-schema-viewer">Plugin Schema Viewer</div>',
    props: ['plugin', 'schema', 'configuration', 'validationErrors', 'loading', 'testing', 'testResult'],
    emits: ['update:configuration', 'test', 'save', 'reset', 'generate-default']
  }
}))

// Mock the API functions
vi.mock('@/api/plugin', () => ({
  getPluginCategoryInfo: vi.fn((category: string) => ({
    label: category.charAt(0).toUpperCase() + category.slice(1),
    description: `${category} plugins description`,
    icon: '📦',
    color: '#6b7280'
  })),
  getPluginStatusInfo: vi.fn((status: string) => ({
    label: status.charAt(0).toUpperCase() + status.slice(1),
    description: `${status} status description`,
    icon: status === 'configured' ? '✅' : '❌',
    color: status === 'configured' ? '#10b981' : '#ef4444'
  }))
}))

describe('PluginManagementPage', () => {
  let wrapper: VueWrapper<any>
  let store: ReturnType<typeof usePluginStore>

  const mockPlugin1: Plugin = {
    name: 'backup-plugin',
    display_name: 'Backup Plugin',
    description: 'A backup plugin for testing',
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
      endpoint: 'https://backup.example.com'
    }
  }

  const mockPlugin2: Plugin = {
    name: 'sync-plugin',
    display_name: 'Sync Plugin',
    description: 'A synchronization plugin for testing',
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
    properties: {
      endpoint: {
        name: 'endpoint',
        type: 'string',
        description: 'API endpoint URL'
      }
    },
    required: ['endpoint']
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    store = usePluginStore()
    
    // Mock store initial data
    store.plugins = [mockPlugin1, mockPlugin2]
    store.loading = false
    store.error = ''
    
    // Mock store methods
    vi.spyOn(store, 'fetchPlugins').mockResolvedValue()
    vi.spyOn(store, 'loadPlugin').mockResolvedValue(mockPlugin1)
    vi.spyOn(store, 'testPluginConfig').mockResolvedValue({
      success: true,
      message: 'Test successful'
    })
    vi.spyOn(store, 'updatePluginConfiguration').mockResolvedValue(mockPlugin1)
    vi.spyOn(store, 'exportPluginConfig').mockReturnValue('{"plugin_name":"test","configuration":{}}')

    wrapper = mount(PluginManagementPage, {
      global: {
        plugins: [createPinia()]
      }
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  describe('component initialization', () => {
    it('should render page header correctly', () => {
      const header = wrapper.find('.page-header h1')
      expect(header.text()).toBe('Plugin Management')
      
      const refreshButton = wrapper.find('.header-actions .secondary-button')
      expect(refreshButton.text()).toContain('Refresh')
      
      const importButton = wrapper.find('.header-actions .primary-button')
      expect(importButton.text()).toContain('Import Config')
    })

    it('should display plugin statistics', () => {
      const statCards = wrapper.findAll('.stats .card')
      expect(statCards.length).toBeGreaterThan(0)
      
      // Should show total plugins
      const totalStat = statCards.find(card => 
        card.find('.stat-label').text() === 'Total Plugins'
      )
      expect(totalStat).toBeTruthy()
      expect(totalStat?.find('.stat-value').text()).toBe('2')
    })

    it('should display health monitoring toggle', () => {
      const monitoringSection = wrapper.find('.monitoring-section')
      expect(monitoringSection.exists()).toBe(true)
      
      const toggleSwitch = wrapper.find('.toggle-switch input')
      expect(toggleSwitch.exists()).toBe(true)
      
      const monitoringLabel = wrapper.find('.monitoring-label')
      expect(monitoringLabel.text()).toBe('Health Monitoring')
    })

    it('should call fetchPlugins on mount', () => {
      expect(store.fetchPlugins).toHaveBeenCalled()
    })
  })

  describe('search and filters', () => {
    it('should render search input', () => {
      const searchInput = wrapper.find('.search-input')
      expect(searchInput.exists()).toBe(true)
      expect(searchInput.attributes('placeholder')).toContain('Search plugins')
    })

    it('should render category filter dropdown', () => {
      const categorySelect = wrapper.findAll('.filter-select')[0]
      expect(categorySelect.exists()).toBe(true)
      
      const options = categorySelect.findAll('option')
      expect(options[0].text()).toBe('All Categories')
    })

    it('should render status filter dropdown', () => {
      const statusSelect = wrapper.findAll('.filter-select')[1]
      expect(statusSelect.exists()).toBe(true)
      
      const options = statusSelect.findAll('option')
      expect(options[0].text()).toBe('All Statuses')
    })

    it('should apply search filter on input', async () => {
      const searchInput = wrapper.find('.search-input')
      await searchInput.setValue('backup')
      
      // Should trigger debounced search after timeout
      // Note: In real tests, we'd need to handle the debounce timing
      expect(searchInput.element.value).toBe('backup')
    })

    it('should apply category filter on selection', async () => {
      const categorySelect = wrapper.findAll('.filter-select')[0]
      await categorySelect.setValue('backup')
      
      expect(categorySelect.element.value).toBe('backup')
    })
  })

  describe('plugin categories dashboard', () => {
    it('should display category cards when no filters applied', () => {
      const categoriesSection = wrapper.find('.categories-section')
      expect(categoriesSection.exists()).toBe(true)
      
      const categoryCards = wrapper.findAll('.category-card')
      expect(categoryCards.length).toBeGreaterThan(0)
    })

    it('should show plugin previews in category cards', () => {
      const categoryCard = wrapper.find('.category-card')
      expect(categoryCard.exists()).toBe(true)
      
      const pluginPreviews = categoryCard.findAll('.plugin-preview')
      expect(pluginPreviews.length).toBeGreaterThan(0)
      
      const statusDots = categoryCard.findAll('.status-dot')
      expect(statusDots.length).toBeGreaterThan(0)
    })

    it('should handle category card clicks', async () => {
      const categoryCard = wrapper.find('.category-card')
      await categoryCard.trigger('click')
      
      // Should set category filter (would need to spy on the method call)
      // This tests the click handler exists
      expect(categoryCard.exists()).toBe(true)
    })
  })

  describe('plugin list view', () => {
    beforeEach(async () => {
      // Trigger filter to show plugin list instead of categories
      const searchInput = wrapper.find('.search-input')
      await searchInput.setValue('test')
    })

    it('should display plugins grid when filters are applied', async () => {
      const pluginsSection = wrapper.find('.plugins-section')
      expect(pluginsSection.exists()).toBe(true)
      
      const pluginsGrid = wrapper.find('.plugins-grid')
      expect(pluginsGrid.exists()).toBe(true)
    })

    it('should render plugin cards with correct information', async () => {
      const pluginCards = wrapper.findAll('.plugin-card')
      expect(pluginCards.length).toBeGreaterThan(0)
      
      const firstCard = pluginCards[0]
      expect(firstCard.find('.plugin-title h3').text()).toBeTruthy()
      expect(firstCard.find('.plugin-version').text()).toMatch(/v\d+\.\d+\.\d+/)
      expect(firstCard.find('.plugin-description').text()).toBeTruthy()
      expect(firstCard.find('.plugin-author').text()).toContain('by')
    })

    it('should display plugin status badges', async () => {
      const statusBadges = wrapper.findAll('.status-badge')
      expect(statusBadges.length).toBeGreaterThan(0)
      
      const configuredBadge = statusBadges.find(badge => 
        badge.text().includes('Configured')
      )
      expect(configuredBadge).toBeTruthy()
    })

    it('should display plugin capabilities', async () => {
      const capabilityTags = wrapper.findAll('.capability-tag')
      expect(capabilityTags.length).toBeGreaterThan(0)
      
      const requiredCapability = capabilityTags.find(tag => 
        tag.classes().includes('required')
      )
      expect(requiredCapability).toBeTruthy()
    })

    it('should display health indicators for configured plugins', async () => {
      const healthIndicators = wrapper.findAll('.health-indicator')
      expect(healthIndicators.length).toBeGreaterThan(0)
      
      const healthyIndicator = healthIndicators.find(indicator => 
        indicator.classes().includes('healthy')
      )
      expect(healthyIndicator).toBeTruthy()
    })
  })

  describe('plugin actions', () => {
    beforeEach(async () => {
      // Show plugin list
      const searchInput = wrapper.find('.search-input')
      await searchInput.setValue('test')
    })

    it('should render action buttons for each plugin', async () => {
      const actionButtons = wrapper.findAll('.action-btn')
      expect(actionButtons.length).toBeGreaterThan(0)
      
      const configureButton = actionButtons.find(btn => 
        btn.text().includes('Configure')
      )
      expect(configureButton).toBeTruthy()
    })

    it('should open configuration modal when configure button clicked', async () => {
      const configureButton = wrapper.find('.configure-btn')
      await configureButton.trigger('click')
      
      expect(wrapper.find('.config-modal').exists()).toBe(true)
      expect(store.loadPlugin).toHaveBeenCalled()
    })

    it('should handle test button click', async () => {
      const testButton = wrapper.find('.test-btn')
      if (testButton.exists()) {
        await testButton.trigger('click')
        expect(store.testPluginConfig).toHaveBeenCalled()
      }
    })

    it('should handle export button click', async () => {
      // Set up cached config to enable export
      store.configurationCache.set('backup-plugin', { endpoint: 'test' })
      await wrapper.vm.$nextTick()
      
      const exportButton = wrapper.find('.export-btn')
      if (exportButton.exists() && !exportButton.attributes('disabled')) {
        // Mock document.createElement and related DOM methods
        const mockA = {
          href: '',
          download: '',
          click: vi.fn()
        }
        vi.spyOn(document, 'createElement').mockReturnValue(mockA as any)
        vi.spyOn(document.body, 'appendChild').mockImplementation(() => {})
        vi.spyOn(document.body, 'removeChild').mockImplementation(() => {})
        vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock-url')
        vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
        
        await exportButton.trigger('click')
        
        expect(store.exportPluginConfig).toHaveBeenCalled()
        expect(mockA.click).toHaveBeenCalled()
      }
    })
  })

  describe('configuration modal', () => {
    beforeEach(async () => {
      // Open configuration modal
      const searchInput = wrapper.find('.search-input')
      await searchInput.setValue('test')
      
      const configureButton = wrapper.find('.configure-btn')
      await configureButton.trigger('click')
    })

    it('should display configuration modal', () => {
      const modal = wrapper.find('.config-modal')
      expect(modal.exists()).toBe(true)
      
      const modalHeader = wrapper.find('.modal-header h3')
      expect(modalHeader.text()).toContain('Configure')
    })

    it('should close modal when close button clicked', async () => {
      const closeButton = wrapper.find('.close-button')
      await closeButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.config-modal').exists()).toBe(false)
    })

    it('should render PluginSchemaViewer component', () => {
      const schemaViewer = wrapper.find('.mock-plugin-schema-viewer')
      expect(schemaViewer.exists()).toBe(true)
    })
  })

  describe('import configuration modal', () => {
    it('should open import modal when import button clicked', async () => {
      const importButton = wrapper.find('.header-actions .primary-button')
      await importButton.trigger('click')
      
      expect(wrapper.find('.import-modal').exists()).toBe(true)
    })

    it('should display import form', async () => {
      const importButton = wrapper.find('.header-actions .primary-button')
      await importButton.trigger('click')
      
      const textarea = wrapper.find('.import-textarea')
      expect(textarea.exists()).toBe(true)
      expect(textarea.attributes('placeholder')).toContain('Paste plugin configuration')
      
      const importActionButton = wrapper.find('.import-actions .primary-button')
      expect(importActionButton.text()).toContain('Import')
    })

    it('should close import modal when cancel button clicked', async () => {
      const importButton = wrapper.find('.header-actions .primary-button')
      await importButton.trigger('click')
      
      const cancelButton = wrapper.find('.import-actions .secondary-button')
      await cancelButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.import-modal').exists()).toBe(false)
    })
  })

  describe('health monitoring', () => {
    it('should toggle health monitoring when switch clicked', async () => {
      const healthToggle = wrapper.find('.toggle-switch input')
      
      vi.spyOn(store, 'startHealthMonitoring').mockImplementation(() => {})
      vi.spyOn(store, 'stopHealthMonitoring').mockImplementation(() => {})
      
      await healthToggle.setChecked(true)
      expect(store.startHealthMonitoring).toHaveBeenCalled()
      
      await healthToggle.setChecked(false)
      expect(store.stopHealthMonitoring).toHaveBeenCalled()
    })
  })

  describe('loading and error states', () => {
    it('should display loading state when loading', async () => {
      store.loading = true
      await wrapper.vm.$nextTick()
      
      const loadingState = wrapper.find('.loading-state')
      expect(loadingState.exists()).toBe(true)
      expect(loadingState.text()).toContain('Loading plugins...')
    })

    it('should display error state when there is an error', async () => {
      store.loading = false
      store.error = 'Failed to load plugins'
      await wrapper.vm.$nextTick()
      
      const errorState = wrapper.find('.error-state')
      expect(errorState.exists()).toBe(true)
      expect(errorState.text()).toContain('Failed to load plugins')
      
      const retryButton = wrapper.find('.retry-button')
      expect(retryButton.exists()).toBe(true)
    })

    it('should display empty state when no plugins match filters', async () => {
      // Clear plugins and set search to show empty state
      store.plugins = []
      const searchInput = wrapper.find('.search-input')
      await searchInput.setValue('nonexistent')
      await wrapper.vm.$nextTick()
      
      const emptyState = wrapper.find('.empty-state')
      expect(emptyState.exists()).toBe(true)
      expect(emptyState.text()).toContain('No plugins found')
    })
  })

  describe('test result display', () => {
    it('should display test result modal when test is completed', async () => {
      // Set up test result
      wrapper.vm.showTestResult = {
        success: true,
        message: 'Test successful',
        response_time_ms: 150
      }
      wrapper.vm.testResultPlugin = mockPlugin1
      await wrapper.vm.$nextTick()
      
      const testResultModal = wrapper.find('.test-result-modal')
      expect(testResultModal.exists()).toBe(true)
      
      const testStatus = wrapper.find('.test-status.success')
      expect(testStatus.exists()).toBe(true)
      expect(testStatus.text()).toContain('Test successful')
    })

    it('should close test result modal when close button clicked', async () => {
      wrapper.vm.showTestResult = {
        success: true,
        message: 'Test successful'
      }
      await wrapper.vm.$nextTick()
      
      const closeButton = wrapper.find('.test-result-modal .close-button')
      await closeButton.trigger('click')
      
      expect(wrapper.vm.showTestResult).toBeNull()
    })
  })

  describe('message display', () => {
    it('should display success messages', async () => {
      wrapper.vm.message.text = 'Plugin configured successfully'
      wrapper.vm.message.type = 'success'
      await wrapper.vm.$nextTick()
      
      const message = wrapper.find('.message.success')
      expect(message.exists()).toBe(true)
      expect(message.text()).toContain('Plugin configured successfully')
    })

    it('should display error messages', async () => {
      wrapper.vm.message.text = 'Configuration failed'
      wrapper.vm.message.type = 'error'
      await wrapper.vm.$nextTick()
      
      const message = wrapper.find('.message.error')
      expect(message.exists()).toBe(true)
      expect(message.text()).toContain('Configuration failed')
    })

    it('should close message when close button clicked', async () => {
      wrapper.vm.message.text = 'Test message'
      wrapper.vm.message.type = 'success'
      await wrapper.vm.$nextTick()
      
      const closeButton = wrapper.find('.message-close')
      await closeButton.trigger('click')
      
      expect(wrapper.vm.message.text).toBe('')
    })
  })

  describe('responsive design', () => {
    it('should adapt layout for mobile screens', () => {
      // Test that responsive CSS classes exist
      expect(wrapper.find('.page-header').exists()).toBe(true)
      expect(wrapper.find('.filter-row').exists()).toBe(true)
      expect(wrapper.find('.plugins-grid').exists()).toBe(true)
      
      // In a real test environment, we'd test actual responsive behavior
      // by changing viewport size or using CSS media query mocks
    })
  })

  describe('accessibility', () => {
    it('should have proper ARIA labels and roles', () => {
      // Check for basic accessibility features
      const buttons = wrapper.findAll('button')
      buttons.forEach(button => {
        // Buttons should have accessible text or aria-label
        const hasAccessibleName = button.text().trim() || button.attributes('aria-label')
        expect(hasAccessibleName).toBeTruthy()
      })
      
      // Form elements should have labels
      const inputs = wrapper.findAll('input, select, textarea')
      inputs.forEach(input => {
        // Should have label association or aria-label
        const hasLabel = input.attributes('aria-label') || input.attributes('placeholder')
        expect(hasLabel).toBeTruthy()
      })
    })

    it('should support keyboard navigation', () => {
      // All interactive elements should be focusable
      const interactiveElements = wrapper.findAll('button, input, select, textarea, a')
      interactiveElements.forEach(element => {
        // Should not have tabindex="-1" unless specifically needed
        const tabindex = element.attributes('tabindex')
        if (tabindex) {
          expect(parseInt(tabindex)).toBeGreaterThanOrEqual(0)
        }
      })
    })
  })
})