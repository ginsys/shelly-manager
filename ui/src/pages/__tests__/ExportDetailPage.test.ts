import { mount, flushPromises } from '@vue/test-utils'
import ExportDetailPage from '@/pages/ExportDetailPage.vue'

const getExportResult = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'exp-123' } }),
}))

vi.mock('@/api/export', () => ({
  getExportResult: (...args: any[]) => getExportResult(...args),
}))

describe('ExportDetailPage', () => {
  beforeEach(() => {
    getExportResult.mockReset()
  })

  it('renders ErrorDisplay when loading fails and retries', async () => {
    getExportResult.mockRejectedValueOnce(new Error('Network fail'))
    // On retry, succeed
    getExportResult.mockResolvedValueOnce({ export_id: 'exp-123', plugin_name: 'json', format: 'json' })

    const wrapper = mount(ExportDetailPage, { attachTo: document.body })
    await flushPromises()

    // Error display is shown
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)

    // Click Retry
    const retry = wrapper.find('button.btn.primary')
    expect(retry.exists()).toBe(true)
    await retry.trigger('click')
    await flushPromises()

    expect(getExportResult).toHaveBeenCalledTimes(2)
    // After success, error display should disappear
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(false)
  })
})
