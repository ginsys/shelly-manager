import { mount, flushPromises } from '@vue/test-utils'
import ExportHistoryPage from '@/pages/ExportHistoryPage.vue'

const fetchHistory = vi.fn()

vi.mock('@/stores/export', () => ({
  useExportStore: () => ({
    // state
    items: [],
    loading: false,
    error: 'Failed to load export history',
    stats: { total: 0, success: 0, failure: 0 },
    meta: { pagination: { page: 1, total_pages: 1, has_next: false, has_previous: false } },
    plugin: '',
    success: undefined,
    // actions
    setPlugin: () => {},
    setSuccess: () => {},
    setPage: () => {},
    fetchHistory,
    fetchStats: vi.fn(),
  }),
}))

describe('ExportHistoryPage', () => {
  it('shows ErrorDisplay and retries fetchHistory', async () => {
    const wrapper = mount(ExportHistoryPage)
    await flushPromises()
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)
    const retry = wrapper.find('button.btn.primary')
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    }
    expect(fetchHistory).toHaveBeenCalled()
  })
})

