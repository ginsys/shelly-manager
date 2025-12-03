import { shallowMount, flushPromises } from '@vue/test-utils'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
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
    const wrapper = shallowMount(ExportSchedulesPage, {
      global: {
        stubs: { ScheduleFilterBar: { template: '<div />' }, DataTable: { template: '<div />' } },
      },
      attachTo: document.body,
    })
    await flushPromises()
    expect(wrapper.findComponent(ErrorDisplay).exists()).toBe(true)
    const ed = wrapper.findComponent(ErrorDisplay)
    ed.vm.$emit('retry')
    await flushPromises()
    expect(fetchSchedules).toHaveBeenCalled()
  })
})
