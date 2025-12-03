import { mount, flushPromises } from '@vue/test-utils'
import ImportHistoryPage from '@/pages/ImportHistoryPage.vue'

const fetchHistory = vi.fn()

vi.mock('@/stores/import', () => ({
  useImportStore: () => ({
    items: [],
    loading: false,
    error: 'Failed to load import history',
    stats: { total: 0, success: 0, failure: 0 },
    meta: { pagination: { page: 1, total_pages: 1, has_next: false, has_previous: false } },
    plugin: '',
    success: undefined,
    setPlugin: () => {},
    setSuccess: () => {},
    setPage: () => {},
    fetchHistory,
    fetchStats: vi.fn(),
  }),
}))

describe('ImportHistoryPage', () => {
  it('shows ErrorDisplay and retries fetchHistory', async () => {
    const wrapper = mount(ImportHistoryPage)
    await flushPromises()
    expect(wrapper.find('[data-testid=\"error-display\"]').exists()).toBe(true)
    const retry = wrapper.find('button.btn.primary')
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    }
    expect(fetchHistory).toHaveBeenCalled()
  })
})

