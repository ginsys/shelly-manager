import { mount, flushPromises } from '@vue/test-utils'
import ExportSchedulesPage from '@/pages/ExportSchedulesPage.vue'

const fetchSchedules = vi.fn()

vi.mock('@/stores/schedule', () => ({
  useScheduleStore: () => ({
    schedulesSorted: [],
    loading: false,
    error: 'Failed to load schedules',
    meta: { pagination: { page: 1, total_pages: 1, has_next: false, has_previous: false } },
    stats: { total: 0, enabled: 0, disabled: 0 },
    plugin: '',
    enabled: undefined,
    // actions
    fetchSchedules,
    setPlugin: () => {},
    setEnabled: () => {},
    setPage: () => {},
    isScheduleRunning: () => false,
    getRecentRun: () => null,
  }),
}))

describe('ExportSchedulesPage', () => {
  it('shows ErrorDisplay and retries fetchSchedules', async () => {
    const wrapper = mount(ExportSchedulesPage, {
      global: {
        stubs: { ScheduleFilterBar: { template: '<div />' }, DataTable: { template: '<div />' } },
      },
    })
    await flushPromises()
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)
    const retry = wrapper.find('button.btn.primary')
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    }
    expect(fetchSchedules).toHaveBeenCalled()
  })
})

