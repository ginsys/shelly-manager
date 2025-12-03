import { shallowMount, flushPromises } from '@vue/test-utils'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
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
    const wrapper = shallowMount(ExportHistoryPage, { attachTo: document.body })
    await flushPromises()
    expect(wrapper.findComponent(ErrorDisplay).exists()).toBe(true)
    // Emit retry from stubbed component
    const ed = wrapper.findComponent(ErrorDisplay)
    ed.vm.$emit('retry')
    await flushPromises()
    expect(fetchHistory).toHaveBeenCalled()
  })
})
