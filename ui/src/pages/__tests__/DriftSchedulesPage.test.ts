import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import DriftSchedulesPage from '../DriftSchedulesPage.vue'
import { useDriftStore } from '@/stores/drift'
import type { DriftSchedule } from '@/api/drift'

// A stored-but-inert schedule with enabled=true and a JSON device_filter.
const storedSchedule: DriftSchedule = {
  id: 1,
  name: 'Nightly drift',
  description: 'stored only',
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

function mountPage() {
  const wrapper = mount(DriftSchedulesPage, {
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            drift: { schedules: [storedSchedule], loading: false, error: null }
          }
        })
      ]
    }
  })
  return wrapper
}

describe('DriftSchedulesPage (fail-closed #270)', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows the not-executed notice', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('Drift schedules are not executed in this release.')
  })

  it('renders an enabled schedule as stored/inactive, not operational', async () => {
    const wrapper = mountPage()
    await flushPromises()
    // Neutral "Stored" badge, never an "Enabled"/operational badge.
    expect(wrapper.text()).toContain('Stored (inactive)')
    expect(wrapper.find('.status-badge').text()).not.toContain('Enabled')
    // Run history is labelled unavailable, not empty.
    expect(wrapper.text()).toContain('Run history: unavailable')
  })

  it('renders device_filter read-only as JSON (never [object Object])', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const filter = wrapper.find('.filter-json')
    expect(filter.exists()).toBe(true)
    expect(filter.text()).toContain('device_type')
    expect(wrapper.text()).not.toContain('[object Object]')
  })

  it('exposes only Delete — no create, edit, toggle or runs controls', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const buttonText = wrapper.findAll('button').map(b => b.text().toLowerCase())
    expect(buttonText.some(t => t.includes('delete'))).toBe(true)
    expect(buttonText.some(t => t.includes('create'))).toBe(false)
    expect(buttonText.some(t => t.includes('edit'))).toBe(false)
    expect(buttonText.some(t => t.includes('toggle'))).toBe(false)
    expect(buttonText.some(t => t.includes('runs') || t.includes('view runs'))).toBe(false)

    // No create/edit modal or form inputs exist at all.
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
