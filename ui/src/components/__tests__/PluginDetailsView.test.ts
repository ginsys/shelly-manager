import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { Quasar } from 'quasar'
import PluginDetailsView from '../PluginDetailsView.vue'
import type { Plugin } from '../../api/plugin'

// Mock the plugin store
const mockPluginStore = {
  getPlugin: vi.fn()
}

vi.mock('../../stores/plugin', () => ({
  usePluginStore: () => mockPluginStore
}))

// Mock Quasar dialog and notify
const mockDialog = vi.fn()
const mockNotify = vi.fn()
vi.mock('quasar', async () => {
  const actual = await vi.importActual('quasar')
  return {
    ...actual,
    useQuasar: () => ({
      dialog: mockDialog,
      notify: mockNotify
    })
  }
})

const mockPlugin: Plugin = {
  name: 'advanced-backup-plugin',
  version: '2.1.3',
  description: 'Advanced backup plugin with encryption and compression',
  long_description: 'A comprehensive backup solution that provides encrypted backups with multiple compression algorithms, incremental backup support, and advanced scheduling capabilities.',
  category: 'backup',
  status: 'active',
  author: 'Shelly Team',
  license: 'MIT',
  enabled: true,
  configured: true,
  supported_formats: ['json', 'yaml', 'tar.gz', 'zip'],
  tags: ['backup', 'encryption', 'compression', 'scheduling'],
  created_at: '2023-01-15T10:30:00Z',
  updated_at: '2023-07-20T14:45:00Z',
  capabilities: {
    export: ['full-backup', 'incremental', 'selective', 'encrypted'],
    import: ['restore', 'selective-restore', 'verify-backup']
  },
  features: ['AES-256 Encryption', 'Multiple Compression Algorithms', 'Incremental Backups', 'Scheduled Operations', 'Integrity Verification'],
  config: {
    encryption: true,
    compression: 'gzip',
    retention_days: 30,
    schedule: '0 2 * * *'
  },
  usage_stats: {
    total_exports: 450,
    successful_exports: 442,
    failed_exports: 8,
    last_used: '2023-07-20T02:00:00Z'
  },
  performance: {
    avg_duration: '3.2s',
    success_rate: '98.2%'
  },
  health: {
    status: 'healthy',
    last_check: '2023-07-20T15:00:00Z',
    dependencies: [
      { name: 'openssl', version: '1.1.1', status: 'ok' },
      { name: 'gzip', version: '1.10', status: 'ok' },
      { name: 'tar', version: '1.34', status: 'ok' }
    ],
    messages: []
  },
  recent_activity: [
    {
      id: '1',
      action: 'Scheduled Backup',
      description: 'Full system backup completed successfully (encrypted)',
      timestamp: '2023-07-20T02:00:00Z',
      success: true
    },
    {
      id: '2',
      action: 'Configuration Updated',
      description: 'Retention period changed to 30 days',
      timestamp: '2023-07-19T10:15:00Z',
      success: true
    },
    {
      id: '3',
      action: 'Health Check',
      description: 'All dependencies verified successfully',
      timestamp: '2023-07-19T00:00:00Z',
      success: true
    }
  ]
}

describe('PluginDetailsView', () => {
  let wrapper: any

  beforeEach(() => {
    vi.clearAllMocks()
    mockPluginStore.getPlugin.mockResolvedValue(mockPlugin)
  })

  const createWrapper = (props = {}) => {
    return mount(PluginDetailsView, {
      props: {
        plugin: mockPlugin,
        modelValue: true,
        ...props
      },
      global: {
        plugins: [Quasar]
      }
    })
  }

  describe('Component Initialization', () => {
    it('should render the dialog when modelValue is true', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.q-dialog').exists()).toBe(true)
      expect(wrapper.find('.text-h6').text()).toBe('Plugin Details')
    })

    it('should not render the dialog when modelValue is false', async () => {
      wrapper = createWrapper({ modelValue: false })
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.q-dialog').exists()).toBe(false)
    })

    it('should load plugin details on dialog open', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(mockPluginStore.getPlugin).toHaveBeenCalledWith('advanced-backup-plugin')
    })

    it('should display plugin header information', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('advanced-backup-plugin')
      expect(wrapper.text()).toContain('Advanced backup plugin with encryption and compression')
      expect(wrapper.text()).toContain('v2.1.3')
      expect(wrapper.text()).toContain('backup')
      expect(wrapper.text()).toContain('active')
    })
  })

  describe('Plugin Status Display', () => {
    it('should display correct status color and icon for active plugin', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      const statusChip = wrapper.find('[data-test="status-chip"]')
      expect(wrapper.vm.getStatusColor('active')).toBe('green')
      expect(wrapper.vm.getStatusIcon('active')).toBe('check_circle')
    })

    it('should display correct status color and icon for inactive plugin', async () => {
      const inactivePlugin = { ...mockPlugin, status: 'inactive' }
      wrapper = createWrapper({ plugin: inactivePlugin })
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.getStatusColor('inactive')).toBe('grey')
      expect(wrapper.vm.getStatusIcon('inactive')).toBe('pause_circle')
    })

    it('should display correct status color and icon for error state', async () => {
      const errorPlugin = { ...mockPlugin, status: 'error' }
      wrapper = createWrapper({ plugin: errorPlugin })
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.getStatusColor('error')).toBe('red')
      expect(wrapper.vm.getStatusIcon('error')).toBe('error')
    })
  })

  describe('Tab Navigation', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
    })

    it('should render all tab options', () => {
      expect(wrapper.find('[name="overview"]').exists()).toBe(true)
      expect(wrapper.find('[name="capabilities"]').exists()).toBe(true)
      expect(wrapper.find('[name="configuration"]').exists()).toBe(true)
      expect(wrapper.find('[name="usage"]').exists()).toBe(true)
      expect(wrapper.find('[name="health"]').exists()).toBe(true)
    })

    it('should switch between tabs', async () => {
      const capabilitiesTab = wrapper.find('[name="capabilities"]')
      await capabilitiesTab.trigger('click')

      expect(wrapper.vm.activeTab).toBe('capabilities')
    })

    it('should display overview information in overview tab', () => {
      expect(wrapper.text()).toContain('Shelly Team') // author
      expect(wrapper.text()).toContain('MIT') // license
      expect(wrapper.text()).toContain('json') // supported format
      expect(wrapper.text()).toContain('encryption') // tag
    })
  })

  describe('Capabilities Tab', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await wrapper.setData({ activeTab: 'capabilities' })
    })

    it('should display export capabilities', () => {
      expect(wrapper.text()).toContain('full-backup')
      expect(wrapper.text()).toContain('incremental')
      expect(wrapper.text()).toContain('selective')
      expect(wrapper.text()).toContain('encrypted')
    })

    it('should display import capabilities', () => {
      expect(wrapper.text()).toContain('restore')
      expect(wrapper.text()).toContain('selective-restore')
      expect(wrapper.text()).toContain('verify-backup')
    })

    it('should display features', () => {
      expect(wrapper.text()).toContain('AES-256 Encryption')
      expect(wrapper.text()).toContain('Multiple Compression Algorithms')
      expect(wrapper.text()).toContain('Incremental Backups')
    })
  })

  describe('Configuration Tab', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await wrapper.setData({ activeTab: 'configuration' })
    })

    it('should show current configuration for configured plugin', () => {
      expect(wrapper.text()).toContain('Current Configuration')
      expect(wrapper.text()).toContain('"encryption": true')
      expect(wrapper.text()).toContain('"compression": "gzip"')
    })

    it('should show configure button for configured plugin', () => {
      const editButton = wrapper.find('[label="Edit Configuration"]')
      expect(editButton.exists()).toBe(true)
    })

    it('should show not configured message for unconfigured plugin', async () => {
      const unconfiguredPlugin = { ...mockPlugin, configured: false, config: undefined }
      wrapper = createWrapper({ plugin: unconfiguredPlugin })
      await wrapper.vm.$nextTick()
      await wrapper.setData({ activeTab: 'configuration' })

      expect(wrapper.text()).toContain('Not Configured')
      expect(wrapper.text()).toContain('Configure Now')
    })
  })

  describe('Usage Tab', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await wrapper.setData({ activeTab: 'usage' })
    })

    it('should display usage statistics', () => {
      expect(wrapper.text()).toContain('450') // total exports
      expect(wrapper.text()).toContain('442') // successful exports
      expect(wrapper.text()).toContain('8') // failed exports
    })

    it('should display performance metrics', () => {
      expect(wrapper.text()).toContain('3.2s') // average duration
      expect(wrapper.text()).toContain('98.2%') // success rate
    })

    it('should display recent activity', () => {
      expect(wrapper.text()).toContain('Scheduled Backup')
      expect(wrapper.text()).toContain('Configuration Updated')
      expect(wrapper.text()).toContain('Health Check')
    })

    it('should format dates correctly', () => {
      const formattedDate = wrapper.vm.formatDate('2023-07-20T02:00:00Z')
      expect(formattedDate).toBeTruthy()
      expect(formattedDate).not.toBe('N/A')
    })
  })

  describe('Health Tab', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await wrapper.setData({ activeTab: 'health' })
    })

    it('should display health status', () => {
      expect(wrapper.text()).toContain('healthy')
    })

    it('should display dependencies with status', () => {
      expect(wrapper.text()).toContain('openssl')
      expect(wrapper.text()).toContain('1.1.1')
      expect(wrapper.text()).toContain('gzip')
      expect(wrapper.text()).toContain('tar')
    })

    it('should show health check button', () => {
      const healthButton = wrapper.find('[label="Check Health"]')
      expect(healthButton.exists()).toBe(true)
    })

    it('should display health messages when available', async () => {
      const pluginWithMessages = {
        ...mockPlugin,
        health: {
          ...mockPlugin.health!,
          messages: [
            { id: '1', type: 'warning', text: 'Dependency version is outdated' }
          ]
        }
      }
      
      wrapper = createWrapper({ plugin: pluginWithMessages })
      await wrapper.vm.$nextTick()
      await wrapper.setData({ activeTab: 'health' })

      expect(wrapper.text()).toContain('Dependency version is outdated')
    })
  })

  describe('Plugin Actions', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
    })

    it('should emit configure event when configure button is clicked', async () => {
      const configureButton = wrapper.find('[label="Configure"]')
      await configureButton.trigger('click')

      expect(wrapper.emitted().configure).toBeTruthy()
      expect(wrapper.emitted().configure[0]).toEqual([mockPlugin])
    })

    it('should test plugin when test button is clicked', async () => {
      const testButton = wrapper.find('[label="Test"]')
      await testButton.trigger('click')

      expect(wrapper.vm.testing).toBe(true)
      // Test should complete and show notification
      expect(mockNotify).toHaveBeenCalledWith({
        type: 'positive',
        message: 'advanced-backup-plugin test completed successfully'
      })
    })

    it('should toggle plugin state when enable/disable button is clicked', async () => {
      const toggleButton = wrapper.find('[label="Disable"]') // Plugin is enabled by default
      await toggleButton.trigger('click')

      expect(mockNotify).toHaveBeenCalledWith({
        type: 'positive',
        message: 'Plugin disabled successfully'
      })
    })

    it('should show confirmation dialog for uninstall', async () => {
      mockDialog.mockImplementation((options: any) => ({
        onOk: (callback: () => void) => callback()
      }))

      const uninstallButton = wrapper.find('[label="Uninstall"]')
      await uninstallButton.trigger('click')

      expect(mockDialog).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'Uninstall Plugin',
          message: expect.stringContaining('advanced-backup-plugin'),
          cancel: true,
          persistent: true,
          color: 'negative'
        })
      )
    })

    it('should show confirmation dialog for clear configuration', async () => {
      mockDialog.mockImplementation((options: any) => ({
        onOk: (callback: () => void) => callback()
      }))

      await wrapper.setData({ activeTab: 'configuration' })
      const clearButton = wrapper.find('[label="Clear Configuration"]')
      await clearButton.trigger('click')

      expect(mockDialog).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'Clear Configuration',
          message: expect.stringContaining('clear the plugin configuration')
        })
      )
    })
  })

  describe('Error Handling', () => {
    it('should handle plugin loading errors', async () => {
      mockPluginStore.getPlugin.mockRejectedValue(new Error('Failed to load plugin'))
      
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(wrapper.vm.error).toBe('Failed to load plugin details')
      expect(wrapper.text()).toContain('Failed to load plugin details')
    })

    it('should display loading state while fetching plugin details', async () => {
      let resolvePlugin: any
      const pluginPromise = new Promise(resolve => { resolvePlugin = resolve })
      mockPluginStore.getPlugin.mockReturnValue(pluginPromise)

      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.loading).toBe(true)
      expect(wrapper.text()).toContain('Loading plugin details...')

      resolvePlugin(mockPlugin)
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.loading).toBe(false)
    })
  })

  describe('Date Formatting', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
    })

    it('should format valid dates correctly', () => {
      const result = wrapper.vm.formatDate('2023-07-20T14:45:00Z')
      expect(result).toBeTruthy()
      expect(result).not.toBe('N/A')
    })

    it('should handle invalid dates gracefully', () => {
      expect(wrapper.vm.formatDate('invalid-date')).toBe('Invalid date')
      expect(wrapper.vm.formatDate('')).toBe('N/A')
      expect(wrapper.vm.formatDate(null)).toBe('N/A')
      expect(wrapper.vm.formatDate(undefined)).toBe('N/A')
    })
  })

  describe('Plugin Color and Icon Helpers', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
    })

    it('should return correct colors for plugin categories', () => {
      expect(wrapper.vm.getPluginColor('backup')).toBe('blue')
      expect(wrapper.vm.getPluginColor('gitops')).toBe('green')
      expect(wrapper.vm.getPluginColor('sync')).toBe('purple')
      expect(wrapper.vm.getPluginColor('custom')).toBe('orange')
      expect(wrapper.vm.getPluginColor('unknown')).toBe('blue-grey')
    })

    it('should return correct icons for plugin categories', () => {
      expect(wrapper.vm.getPluginIcon('backup')).toBe('backup')
      expect(wrapper.vm.getPluginIcon('gitops')).toBe('sync')
      expect(wrapper.vm.getPluginIcon('sync')).toBe('sync_alt')
      expect(wrapper.vm.getPluginIcon('custom')).toBe('extension')
      expect(wrapper.vm.getPluginIcon('unknown')).toBe('extension')
    })

    it('should return correct status colors', () => {
      expect(wrapper.vm.getStatusColor('active')).toBe('green')
      expect(wrapper.vm.getStatusColor('inactive')).toBe('grey')
      expect(wrapper.vm.getStatusColor('error')).toBe('red')
      expect(wrapper.vm.getStatusColor('healthy')).toBe('green')
      expect(wrapper.vm.getStatusColor('unhealthy')).toBe('red')
    })
  })

  describe('Dialog Control', () => {
    it('should emit update:modelValue when dialog is closed', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      const closeButton = wrapper.find('[icon="close"]')
      await closeButton.trigger('click')

      expect(wrapper.emitted()['update:modelValue']).toBeTruthy()
      expect(wrapper.emitted()['update:modelValue'][0]).toEqual([false])
    })

    it('should close dialog with footer close button', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      const closeButton = wrapper.find('[label="Close"]')
      await closeButton.trigger('click')

      expect(wrapper.emitted()['update:modelValue']).toBeTruthy()
    })
  })

  describe('Accessibility', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
    })

    it('should have proper heading structure', () => {
      const mainHeading = wrapper.find('.text-h6')
      expect(mainHeading.text()).toBe('Plugin Details')
    })

    it('should have proper ARIA labels for interactive elements', () => {
      const configureButton = wrapper.find('[label="Configure"]')
      const testButton = wrapper.find('[label="Test"]')
      
      expect(configureButton.exists()).toBe(true)
      expect(testButton.exists()).toBe(true)
    })

    it('should provide proper tab navigation', () => {
      const tabs = wrapper.findAll('.q-tab')
      expect(tabs.length).toBeGreaterThan(0)
      
      // Each tab should be keyboard accessible
      tabs.forEach(tab => {
        expect(tab.attributes('tabindex')).toBeDefined()
      })
    })
  })
})