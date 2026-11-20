import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import DriftSchedulesPage from '../DriftSchedulesPage.vue'
import { useDriftStore } from '@/stores/drift'
import type { DriftSchedule } from '@/api/drift'

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

  it('exposes only Delete — no create, edit, toggle or runs controls yet', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const buttonText = wrapper.findAll('button').map(b => b.text().toLowerCase())
    expect(buttonText.some(t => t.includes('delete'))).toBe(true)
    expect(buttonText.some(t => t.includes('create'))).toBe(false)
    expect(buttonText.some(t => t.includes('edit'))).toBe(false)
    expect(buttonText.some(t => t.includes('toggle'))).toBe(false)
    expect(buttonText.some(t => t.includes('runs') || t.includes('view runs'))).toBe(false)

    expect(wrapper.find('.modal').exists()).toBe(false)
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.findAll('input').length).toBe(0)
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
