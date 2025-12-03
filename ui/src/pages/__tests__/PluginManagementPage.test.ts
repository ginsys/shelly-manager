import { mount, flushPromises } from '@vue/test-utils'
import PluginManagementPage from '@/pages/PluginManagementPage.vue'

const refresh = vi.fn()

vi.mock('@/stores/plugin', () => ({
  usePluginStore: () => ({
    // state
    plugins: [],
    categories: [],
    loading: false,
    error: '',
    currentLoading: false,
    // getters used by page
    filteredPlugins: [],
    pluginsByCategory: {},
    pluginStats: { total: 0, configured: 0, available: 0, disabled: 0, error: 0 },
    // methods
    refresh,
    setCategory: () => {},
    setStatusFilter: () => {},
    setSearchQuery: () => {},
    getPluginStatusClass: () => 'ready',
    clearErrors: () => {},
    isPluginTesting: () => false,
    getTestResult: () => null,
    getPlugin: () => {},
  }),
}))

describe('PluginManagementPage', () => {
  beforeEach(() => {
    refresh.mockReset()
  })

  it('renders ErrorDisplay when refresh fails on mount', async () => {
    refresh.mockRejectedValueOnce(new Error('Failed to load plugins'))

    const wrapper = mount(PluginManagementPage, {
      global: {
        stubs: {
          PluginFilters: { template: '<div />' },
          PluginCategorySection: { template: '<div />' },
          PluginConfigModal: { template: '<div />' },
          PluginDetailsModal: { template: '<div />' },
          MessageBanner: { template: '<div />' },
        },
      },
    })

    await flushPromises()
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)
  })
})

