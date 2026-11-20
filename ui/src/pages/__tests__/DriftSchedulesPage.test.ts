import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import DriftSchedulesPage from '../DriftSchedulesPage.vue'
import { useDriftStore } from '@/stores/drift'
import type { DriftSchedule, DriftRun } from '@/api/drift'
import { listDevices } from '@/api/devices'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn()
}))

// An enabled schedule with a JSON device_filter.
const storedSchedule: DriftSchedule = {
  id: 1,
  name: 'Nightly drift',
  description: 'every night',
  enabled: true,
  cron_spec: '0 0 * * *',
  device_ids: [1, 2],
  device_filter: { device_type: 'shelly1', generation: 1 },
  last_run: null,
  next_run: null,
  run_count: 0,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z'
}

interface MountOptions {
  schedulerEnabled?: boolean | null
  statusFails?: boolean
  schedules?: DriftSchedule[]
}

function mountPage({ schedulerEnabled = true, statusFails = false, schedules = [storedSchedule] }: MountOptions = {}) {
  const pinia = createTestingPinia({
    createSpy: vi.fn,
    initialState: {
      drift: { schedules, loading: false, error: null, schedulerEnabled }
    }
  })
  if (statusFails) {
    vi.mocked(useDriftStore(pinia).fetchSchedulerStatus).mockRejectedValue(new Error('status is down'))
  }
  return mount(DriftSchedulesPage, { global: { plugins: [pinia] } })
}

describe('DriftSchedulesPage', () => {
  beforeEach(() => {
    vi.mocked(listDevices).mockReset()
    vi.mocked(listDevices).mockResolvedValue({
      items: [{ id: 1, name: 'Porch plug', mac: 'AA', type: 'SHPLG-S', status: 'online' } as any]
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('reads the scheduler state when it opens', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(useDriftStore().fetchSchedulerStatus).toHaveBeenCalledTimes(1)
    expect(wrapper.exists()).toBe(true)
  })

  it('shows no banner and an Enabled badge while the scheduler runs', async () => {
    const wrapper = mountPage({ schedulerEnabled: true })
    await flushPromises()
    expect(wrapper.find('[data-testid="scheduler-banner"]').exists()).toBe(false)
    expect(wrapper.find('.status-badge').text()).toBe('Enabled')
  })

  it('says the scheduler is off, and badges an enabled schedule "Scheduler off"', async () => {
    const wrapper = mountPage({ schedulerEnabled: false })
    await flushPromises()
    const banner = wrapper.find('[data-testid="scheduler-banner"]')
    expect(banner.text()).toContain('The drift scheduler is off on this server')
    expect(banner.text()).toContain('drift.scheduler_enabled')
    expect(wrapper.find('.status-badge').text()).toBe('Scheduler off')
  })

  it('badges a disabled schedule "Disabled" whatever the scheduler state', async () => {
    for (const schedulerEnabled of [true, false]) {
      const wrapper = mountPage({ schedulerEnabled, schedules: [{ ...storedSchedule, enabled: false }] })
      await flushPromises()
      expect(wrapper.find('.status-badge').text()).toBe('Disabled')
    }
  })

  it('says the state is unknown when the read failed, never that the scheduler is off', async () => {
    const wrapper = mountPage({ schedulerEnabled: null, statusFails: true })
    await flushPromises()
    const banner = wrapper.find('[data-testid="scheduler-banner"]')
    expect(banner.text()).toContain('The scheduler state could not be read')
    expect(wrapper.text()).not.toContain('is off on this server')
    expect(wrapper.find('.status-badge').text()).toBe('Enabled')
  })

  it('no longer says schedules are not executed or that run history is unavailable', async () => {
    const wrapper = mountPage({ schedulerEnabled: false })
    await flushPromises()
    expect(wrapper.text()).not.toContain('not executed')
    expect(wrapper.text()).not.toContain('Run history: unavailable')
    expect(wrapper.text()).not.toContain('Persisted values')
    expect(wrapper.text()).toContain('Run count: 0')
  })

  it('renders device_filter read-only as JSON (never [object Object])', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const filter = wrapper.find('.filter-json')
    expect(filter.exists()).toBe(true)
    expect(filter.text()).toContain('device_type')
    expect(wrapper.text()).not.toContain('[object Object]')
  })

  it('opens with no form, and loads the schedules without a page', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('[data-testid="schedule-form"]').exists()).toBe(false)
    expect(useDriftStore().fetchSchedules).toHaveBeenCalledWith()
    expect(listDevices).not.toHaveBeenCalled()
  })

  it('has no pager, even when the store holds paging meta', async () => {
    const pinia = createTestingPinia({
      createSpy: vi.fn,
      initialState: {
        drift: {
          schedules: [storedSchedule],
          scheduleMeta: { pagination: { page: 1, page_size: 1, total_pages: 9, has_next: true, has_previous: false } },
          loading: false,
          error: null,
          schedulerEnabled: true
        }
      }
    })
    const wrapper = mount(DriftSchedulesPage, { global: { plugins: [pinia] } })
    await flushPromises()
    expect(wrapper.find('.pagination').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Next')
    expect(wrapper.text()).not.toContain('Previous')
  })

  it('New asks for every device, then opens an empty form', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('[data-testid="new-schedule"]').trigger('click')
    await flushPromises()

    expect(listDevices).toHaveBeenCalledWith({ all: true })
    expect(wrapper.find('[data-testid="schedule-form"]').exists()).toBe(true)
    expect(wrapper.find('h2').text()).toBe('New drift schedule')
    expect((wrapper.find('[data-testid="schedule-name"]').element as HTMLInputElement).value).toBe('')
  })

  it('a second New or Edit while the devices load is ignored: the form is the one first asked for', async () => {
    let release: (v: { items: any[] }) => void = () => {}
    vi.mocked(listDevices).mockReset()
    vi.mocked(listDevices).mockReturnValueOnce(new Promise(resolve => { release = resolve }))
    const wrapper = mountPage({ schedules: [{ ...storedSchedule, device_filter: null }] })
    await flushPromises()

    await wrapper.find('[data-testid="new-schedule"]').trigger('click')
    await wrapper.find('[data-testid="edit-1"]').trigger('click')
    expect(listDevices).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="edit-1"]').attributes('disabled')).toBeDefined()

    release({ items: [] })
    await flushPromises()
    expect(wrapper.find('h2').text()).toBe('New drift schedule')
    expect((wrapper.find('[data-testid="schedule-name"]').element as HTMLInputElement).value).toBe('')
  })

  it('Edit opens the form filled from that schedule', async () => {
    const wrapper = mountPage({ schedules: [{ ...storedSchedule, device_filter: null }] })
    await flushPromises()
    await wrapper.find('[data-testid="edit-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('h2').text()).toBe('Edit drift schedule')
    expect((wrapper.find('[data-testid="schedule-name"]').element as HTMLInputElement).value).toBe('Nightly drift')
  })

  it('the form closes on cancel, and when the device list fails it still opens and says so', async () => {
    vi.mocked(listDevices).mockRejectedValueOnce(new Error('devices are down'))
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('[data-testid="new-schedule"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-testid="mode-devices"]').setValue(true)
    expect(wrapper.find('[data-testid="device-picker"]').text()).toContain('Devices could not be loaded: devices are down')
    await wrapper.find('[data-testid="schedule-cancel"]').trigger('click')
    expect(wrapper.find('[data-testid="schedule-form"]').exists()).toBe(false)
  })

  it('the toggle button says what it will do and calls store.toggle', async () => {
    const wrapper = mountPage({ schedules: [storedSchedule, { ...storedSchedule, id: 2, enabled: false }] })
    await flushPromises()
    expect(wrapper.find('[data-testid="toggle-1"]').text()).toBe('Disable')
    expect(wrapper.find('[data-testid="toggle-2"]').text()).toBe('Enable')

    await wrapper.find('[data-testid="toggle-2"]').trigger('click')
    expect(useDriftStore().toggle).toHaveBeenCalledWith(2)
  })

  it('a failed toggle shows its failure above the list and keeps the page', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vi.mocked(useDriftStore().toggle).mockRejectedValueOnce(new Error('toggle refused'))
    await wrapper.find('[data-testid="toggle-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="schedule-action-error"]').text()).toContain('toggle refused')
    expect(wrapper.find('.schedule-card').exists()).toBe(true)
  })

  describe('recent runs', () => {
    const runs: DriftRun[] = [
      { id: 12, schedule_id: 1, status: 'skipped', started_at: '2026-07-02T03:00:00Z', duration: 4_000_000, created_at: 'x' },
      { id: 11, schedule_id: 1, status: 'failed', started_at: '2026-07-01T03:00:00Z', duration: 2_500_000_000, error: 'device unreachable', created_at: 'x' },
      { id: 10, schedule_id: 1, status: 'completed', started_at: '2026-06-30T03:00:00Z', duration: 90_000_000_000, created_at: 'x' }
    ]

    it('loads when the card is first expanded, newest first, with durations from nanoseconds', async () => {
      const wrapper = mountPage()
      await flushPromises()
      const store = useDriftStore()
      vi.mocked(store.fetchRuns).mockResolvedValue(runs)
      expect(store.fetchRuns).not.toHaveBeenCalled()

      await wrapper.find('[data-testid="runs-toggle-1"]').trigger('click')
      await flushPromises()

      expect(store.fetchRuns).toHaveBeenCalledWith(1)
      const rows = wrapper.findAll('[data-testid="runs-1"] .run-row').map(r => r.findAll('td').map(td => td.text()))
      expect(rows.map(r => r[1])).toEqual(['skipped', 'failed', 'completed'])
      expect(rows.map(r => r[2])).toEqual(['4 ms', '2.5 s', '1 min 30 s'])
      expect(rows[1][3]).toBe('device unreachable')
    })

    it('collapses and re-expands without a second read when runs were found', async () => {
      const wrapper = mountPage()
      await flushPromises()
      const store = useDriftStore()
      vi.mocked(store.fetchRuns).mockResolvedValue(runs)

      const toggle = wrapper.find('[data-testid="runs-toggle-1"]')
      await toggle.trigger('click')
      await flushPromises()
      await toggle.trigger('click')
      expect(wrapper.find('[data-testid="runs-1"]').exists()).toBe(false)
      await toggle.trigger('click')
      await flushPromises()
      expect(wrapper.find('[data-testid="runs-1"]').exists()).toBe(true)
      expect(store.fetchRuns).toHaveBeenCalledTimes(1)
    })

    it('says "No runs yet" for a schedule that never ran', async () => {
      const wrapper = mountPage()
      await flushPromises()
      vi.mocked(useDriftStore().fetchRuns).mockResolvedValue([])
      await wrapper.find('[data-testid="runs-toggle-1"]').trigger('click')
      await flushPromises()
      expect(wrapper.find('[data-testid="runs-1"]').text()).toContain('No runs yet')
      expect(wrapper.find('.runs-table').exists()).toBe(false)
    })

    it('shows a failed load with a retry that reads again', async () => {
      const wrapper = mountPage()
      await flushPromises()
      const store = useDriftStore()
      vi.mocked(store.fetchRuns).mockRejectedValueOnce(new Error('runs are down')).mockResolvedValueOnce(runs)

      await wrapper.find('[data-testid="runs-toggle-1"]').trigger('click')
      await flushPromises()
      const panel = wrapper.find('[data-testid="runs-1"]')
      expect(panel.find('[role="alert"]').text()).toContain('Runs could not be loaded: runs are down')
      expect(panel.text()).not.toContain('No runs yet')

      await wrapper.find('[data-testid="runs-retry-1"]').trigger('click')
      await flushPromises()
      expect(store.fetchRuns).toHaveBeenCalledTimes(2)
      expect(wrapper.findAll('[data-testid="runs-1"] .run-row')).toHaveLength(3)
    })
  })

  it('Delete calls the retained store.remove action', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const store = useDriftStore()
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    await wrapper.find('.btn-icon.danger').trigger('click')

    expect(store.remove).toHaveBeenCalledWith(1)
  })
})
