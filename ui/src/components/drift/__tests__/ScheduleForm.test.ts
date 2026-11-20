import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import ScheduleForm from '../ScheduleForm.vue'
import { useDriftStore } from '@/stores/drift'
import type { DriftSchedule } from '@/api/drift'
import type { Device } from '@/api/types'

const devices = [
  { id: 1, name: 'Porch plug', mac: 'AA', type: 'SHPLG-S', status: 'online' },
  { id: 2, name: 'Desk light', mac: 'BB', type: 'SHDM-2', status: 'offline' },
  { id: 3, name: 'Hall plug', mac: 'CC', type: 'SHPLG-S', status: 'online' }
] as Device[]

const stored: DriftSchedule = {
  id: 7,
  name: 'Nightly',
  description: 'every night',
  enabled: false,
  cron_spec: '0 3 * * *',
  device_ids: [2],
  device_filter: null,
  last_run: null,
  next_run: null,
  run_count: 4,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z'
}

function mountForm(props: { schedule?: DriftSchedule | null; devicesError?: string } = {}) {
  const pinia = createTestingPinia({ createSpy: vi.fn })
  const store = useDriftStore(pinia)
  const wrapper = mount(ScheduleForm, { props: { devices, ...props }, global: { plugins: [pinia] } })
  return { wrapper, store }
}

async function fillName(wrapper: ReturnType<typeof mountForm>['wrapper'], value = 'Check') {
  await wrapper.find('[data-testid="schedule-name"]').setValue(value)
}

describe('ScheduleForm', () => {
  it('says what saving does to a run that is due and to the timer', () => {
    const { wrapper } = mountForm()
    expect(wrapper.find('[data-testid="save-note"]').text()).toBe(
      'Saving can skip a run that is already due. A changed schedule restarts its timer.'
    )
  })

  it('a new schedule starts enabled, daily, for all devices, and sends neither device field', async () => {
    const { wrapper, store } = mountForm()
    vi.mocked(store.create).mockResolvedValue({ ...stored, id: 9 })
    await fillName(wrapper)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(store.create).toHaveBeenCalledTimes(1)
    const sent = vi.mocked(store.create).mock.calls[0][0]
    expect(sent).toEqual({ name: 'Check', description: '', cron_spec: '0 3 * * *', enabled: true })
    expect('device_ids' in sent).toBe(false)
    expect('device_filter' in sent).toBe(false)
    expect(wrapper.emitted('saved')?.[0]).toEqual([{ ...stored, id: 9 }])
  })

  it.each([
    ['hourly', '0 * * * *'],
    ['daily', '0 3 * * *'],
    ['weekly', '0 3 * * 1']
  ])('the %s preset sends %s', async (preset, spec) => {
    const { wrapper, store } = mountForm()
    vi.mocked(store.create).mockResolvedValue(stored)
    await fillName(wrapper)
    await wrapper.find('[data-testid="schedule-preset"]').setValue(preset)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(vi.mocked(store.create).mock.calls[0][0].cron_spec).toBe(spec)
  })

  it('Custom shows a box for the spec and sends what was typed, trimmed', async () => {
    const { wrapper, store } = mountForm()
    vi.mocked(store.create).mockResolvedValue(stored)
    expect(wrapper.find('[data-testid="schedule-custom-spec"]').exists()).toBe(false)
    await fillName(wrapper)
    await wrapper.find('[data-testid="schedule-preset"]').setValue('custom')
    await wrapper.find('[data-testid="schedule-custom-spec"]').setValue('  */15 * * * *  ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(vi.mocked(store.create).mock.calls[0][0].cron_spec).toBe('*/15 * * * *')
  })

  it('sends at most one of device_ids and device_filter, and neither for all devices', async () => {
    const { wrapper, store } = mountForm()
    vi.mocked(store.create).mockResolvedValue(stored)
    await fillName(wrapper)

    // Chosen devices: device_ids only.
    await wrapper.find('[data-testid="mode-devices"]').setValue(true)
    await wrapper.find('[data-testid="device-1"]').setValue(true)
    await wrapper.find('[data-testid="device-3"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const byIds = vi.mocked(store.create).mock.calls[0][0]
    expect(byIds.device_ids).toEqual([1, 3])
    expect('device_filter' in byIds).toBe(false)

    // Filter: device_filter only, and only the boxes that were filled.
    await wrapper.find('[data-testid="mode-filter"]').setValue(true)
    await wrapper.find('[data-testid="filter-type"]').setValue(' SHPLG-S ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const byFilter = vi.mocked(store.create).mock.calls[1][0]
    expect(byFilter.device_filter).toEqual({ type: 'SHPLG-S' })
    expect('device_ids' in byFilter).toBe(false)

    // Both boxes: both keys.
    await wrapper.find('[data-testid="filter-status"]').setValue('online')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(vi.mocked(store.create).mock.calls[2][0].device_filter).toEqual({ type: 'SHPLG-S', status: 'online' })

    // All devices: neither.
    await wrapper.find('[data-testid="mode-all"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const all = vi.mocked(store.create).mock.calls[3][0]
    expect('device_ids' in all).toBe(false)
    expect('device_filter' in all).toBe(false)
  })

  it('withholds Save with one line when Chosen devices has nothing checked, and sends nothing', async () => {
    const { wrapper, store } = mountForm()
    await fillName(wrapper)
    await wrapper.find('[data-testid="mode-devices"]').setValue(true)

    expect(wrapper.find('[data-testid="save-blocked"]').text()).toBe('Choose at least one device')
    expect(wrapper.find('[data-testid="schedule-save"]').attributes('disabled')).toBeDefined()
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(store.create).not.toHaveBeenCalled()

    await wrapper.find('[data-testid="device-2"]').setValue(true)
    expect(wrapper.find('[data-testid="save-blocked"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="schedule-save"]').attributes('disabled')).toBeUndefined()
  })

  it('withholds Save with one line when Filter has both boxes empty, and sends nothing', async () => {
    const { wrapper, store } = mountForm()
    await fillName(wrapper)
    await wrapper.find('[data-testid="mode-filter"]').setValue(true)
    await wrapper.find('[data-testid="filter-type"]').setValue('   ')

    expect(wrapper.find('[data-testid="save-blocked"]').text()).toBe('Give a type or a status')
    expect(wrapper.find('[data-testid="schedule-save"]').attributes('disabled')).toBeDefined()
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(store.create).not.toHaveBeenCalled()

    await wrapper.find('[data-testid="filter-status"]').setValue('online')
    expect(wrapper.find('[data-testid="save-blocked"]').exists()).toBe(false)
  })

  it('offers the distinct device types for the filter', async () => {
    const { wrapper } = mountForm()
    await wrapper.find('[data-testid="mode-filter"]').setValue(true)
    const options = wrapper.findAll('#schedule-device-types option').map(o => o.attributes('value'))
    expect(options).toEqual(['SHDM-2', 'SHPLG-S'])
  })

  it('edit fills every field from the stored schedule and updates by id', async () => {
    const { wrapper, store } = mountForm({ schedule: stored })
    vi.mocked(store.update).mockResolvedValue(stored)

    expect(wrapper.find('h2').text()).toBe('Edit drift schedule')
    expect((wrapper.find('[data-testid="schedule-name"]').element as HTMLInputElement).value).toBe('Nightly')
    expect((wrapper.find('[data-testid="schedule-description"]').element as HTMLInputElement).value).toBe('every night')
    expect((wrapper.find('[data-testid="schedule-preset"]').element as HTMLSelectElement).value).toBe('daily')
    expect((wrapper.find('[data-testid="schedule-enabled"]').element as HTMLInputElement).checked).toBe(false)
    expect((wrapper.find('[data-testid="mode-devices"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.find('[data-testid="device-2"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.find('[data-testid="device-1"]').element as HTMLInputElement).checked).toBe(false)

    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(store.update).toHaveBeenCalledWith(7, {
      name: 'Nightly',
      description: 'every night',
      cron_spec: '0 3 * * *',
      enabled: false,
      device_ids: [2]
    })
    expect(store.create).not.toHaveBeenCalled()
  })

  it('edit opens Custom for a spec that is no preset, and Filter for a stored filter', () => {
    const { wrapper } = mountForm({
      schedule: { ...stored, cron_spec: '*/5 * * * *', device_ids: null, device_filter: { type: 'SHDM-2', status: 'online' } }
    })
    expect((wrapper.find('[data-testid="schedule-preset"]').element as HTMLSelectElement).value).toBe('custom')
    expect((wrapper.find('[data-testid="schedule-custom-spec"]').element as HTMLInputElement).value).toBe('*/5 * * * *')
    expect((wrapper.find('[data-testid="mode-filter"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.find('[data-testid="filter-type"]').element as HTMLInputElement).value).toBe('SHDM-2')
    expect((wrapper.find('[data-testid="filter-status"]').element as HTMLInputElement).value).toBe('online')
    expect(wrapper.find('[data-testid="filter-extra-keys"]').exists()).toBe(false)
  })

  it('names the fields of a stored filter that this form cannot edit', () => {
    const { wrapper } = mountForm({
      schedule: { ...stored, device_ids: null, device_filter: { type: 'SHDM-2', generation: 1, group: 'x' } }
    })
    expect(wrapper.find('[data-testid="filter-extra-keys"]').text()).toBe(
      'This filter has fields this form cannot edit (generation, group); saving replaces it.'
    )
  })

  it('shows the server text above the form and stays open when the save fails', async () => {
    const { wrapper, store } = mountForm()
    vi.mocked(store.create).mockRejectedValue(new Error('invalid cron spec: expected 5 fields'))
    await fillName(wrapper)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const alert = wrapper.find('[data-testid="schedule-form-error"]')
    expect(alert.attributes('role')).toBe('alert')
    expect(alert.text()).toBe('invalid cron spec: expected 5 fields')
    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.emitted('saved')).toBeUndefined()
    // The form can be saved again once the input changed.
    expect(wrapper.find('[data-testid="schedule-save"]').attributes('disabled')).toBeUndefined()
  })

  it('Cancel emits cancel and saves nothing', async () => {
    const { wrapper, store } = mountForm()
    await wrapper.find('[data-testid="schedule-cancel"]').trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    expect(store.create).not.toHaveBeenCalled()
  })

  it('lists a chosen device that the list does not show, so it can be unchecked', async () => {
    // The server keeps the id of a deleted device in the schedule and refuses every save that still
    // names it, so the form must let the operator remove it.
    const { wrapper, store } = mountForm({ schedule: { ...stored, device_ids: [2, 99] } })
    vi.mocked(store.update).mockResolvedValue(stored)
    const row = wrapper.find('[data-testid="device-99"]')
    expect(row.exists()).toBe(true)
    expect((row.element as HTMLInputElement).checked).toBe(true)
    expect(wrapper.find('[data-testid="unlisted-devices"]').text()).toContain('Device 99')

    await row.setValue(false)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(vi.mocked(store.update).mock.calls[0][1]).toMatchObject({ device_ids: [2] })
  })

  it('shows the device list failure in the picker', async () => {
    const { wrapper } = mountForm({ devicesError: 'Devices could not be loaded' })
    await wrapper.find('[data-testid="mode-devices"]').setValue(true)
    expect(wrapper.find('[data-testid="device-picker"]').text()).toContain('Devices could not be loaded')
  })
})
