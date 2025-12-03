import { shallowMount, flushPromises } from '@vue/test-utils'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
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
    const wrapper = shallowMount(ImportHistoryPage, { attachTo: document.body })
    await flushPromises()
    expect(wrapper.findComponent(ErrorDisplay).exists()).toBe(true)
    const ed = wrapper.findComponent(ErrorDisplay)
    ed.vm.$emit('retry')
    await flushPromises()
    expect(fetchHistory).toHaveBeenCalled()
  })
})
