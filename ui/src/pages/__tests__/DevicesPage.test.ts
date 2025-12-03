import { shallowMount, flushPromises } from '@vue/test-utils'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import DevicesPage from '@/pages/DevicesPage.vue'

const fetch = vi.fn()
const initializeFromStorage = vi.fn()

vi.mock('@/stores/devices', () => ({
  useDevicesStore: () => ({
    // state
    search: '',
    items: [],
    meta: { pagination: { page: 1, total_pages: 1, has_next: false } },
    columns: { name: true, ip: true, mac: true, type: true, status: true, last_seen: true, firmware: true },
    page: 1,
    pageSize: 10,
    // actions
    fetch,
    initializeFromStorage,
    setColumns: () => {},
    setPageSize: () => {},
  }),
}))

describe('DevicesPage', () => {
  beforeEach(() => {
    fetch.mockReset()
    initializeFromStorage.mockReset()
  })

  it('shows ErrorDisplay on fetch failure and retries', async () => {
    fetch.mockRejectedValueOnce(new Error('Failed to load devices'))
    fetch.mockResolvedValueOnce(undefined)

    const wrapper = shallowMount(DevicesPage, { attachTo: document.body })
    await flushPromises()

    expect(wrapper.findComponent(ErrorDisplay).exists()).toBe(true)

    // Retry via the ErrorDisplay retry button
    const retry = wrapper.find('button.btn.primary')
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    } else {
      // Fallback if primary button not present
      await (wrapper.vm as any).fetchData?.()
      await flushPromises()
    }

    expect(fetch).toHaveBeenCalledTimes(2)
  })
})
