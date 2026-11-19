import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceDetailPage from '../DeviceDetailPage.vue'

vi.mock('../../api/devices', () => ({
  getDevice: vi.fn(),
  getDeviceStatus: vi.fn(),
  getDeviceEnergy: vi.fn(),
  controlDevice: vi.fn(),
  updateDevice: vi.fn(),
  isAbortError: () => false,
  updateDeviceAuth: vi.fn(),
}))
vi.mock('../../api/deviceConfig', () => ({ detectConfigDrift: vi.fn() }))
vi.mock('../../api/typedConfig', () => ({ getDeviceCapabilities: vi.fn() }))

import { getDevice, getDeviceStatus, getDeviceEnergy, controlDevice, updateDevice } from '../../api/devices'
import { detectConfigDrift } from '../../api/deviceConfig'
import { getDeviceCapabilities } from '../../api/typedConfig'

const device = {
  id: 7,
  ip: '10.0.0.7',
  mac: 'AA',
  type: 'SHSW-1',
  name: 'Plug',
  firmware: '',
  status: 'online',
  last_seen: '',
  settings: '',
}

async function openPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id', component: DeviceDetailPage }],
  })
  router.push('/devices/7')
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs: { RouterLink: true } } })
  await flushPromises()
  return wrapper
}

// ginsys/shelly-manager#614: a failed control or save shows an error panel, never a browser dialog.
describe('DeviceDetailPage shows a failed action as an error panel', () => {
  let alertSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.resetAllMocks()
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    vi.mocked(getDevice).mockImplementation(async () => ({ ...device }))
    vi.mocked(getDeviceStatus).mockRejectedValue(new Error('offline'))
    vi.mocked(getDeviceEnergy).mockRejectedValue(new Error('offline'))
    vi.mocked(detectConfigDrift).mockRejectedValue(new Error('none'))
    vi.mocked(getDeviceCapabilities).mockRejectedValue(new Error('none'))
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('under the toolbar when a control command fails', async () => {
    vi.mocked(controlDevice).mockRejectedValue(new Error('device is asleep'))
    const wrapper = await openPage()

    await wrapper.get('button[title="Turn On"]').trigger('click')
    await flushPromises()

    const panel = wrapper.get('[data-testid="device-action-error"]')
    expect(panel.text()).toContain('device is asleep')
    expect(panel.text()).toContain('Control device')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('inside the open edit dialog when a save fails, and keeps the dialog open', async () => {
    vi.mocked(updateDevice).mockRejectedValue(new Error('the device changed meanwhile'))
    const wrapper = await openPage()

    await wrapper.get('button[title="Edit Device"]').trigger('click')
    await wrapper.get('.modal-dialog .btn-primary').trigger('click')
    await flushPromises()

    const panel = wrapper.get('.modal-dialog [data-testid="device-save-error"]')
    expect(panel.text()).toContain('the device changed meanwhile')
    expect(panel.text()).toContain('Save device')
    expect(wrapper.find('.modal-dialog').exists()).toBe(true)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('under the toolbar when the dialog was closed while the save was still open', async () => {
    let rejectSave: (reason: any) => void = () => {}
    vi.mocked(updateDevice).mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectSave = reject
        }) as any,
    )
    const wrapper = await openPage()
    await wrapper.get('button[title="Edit Device"]').trigger('click')
    await wrapper.get('.modal-dialog .btn-primary').trigger('click')
    await flushPromises() // the save is open
    await wrapper.get('.modal-dialog .btn-secondary').trigger('click')
    expect(wrapper.find('.modal-dialog').exists()).toBe(false)

    rejectSave(new Error('the device changed meanwhile'))
    await flushPromises()

    const panel = wrapper.get('[data-testid="device-action-error"]')
    expect(panel.text()).toContain('the device changed meanwhile')
    expect(panel.text()).toContain('Save device')
    expect(wrapper.find('[data-testid="device-save-error"]').exists()).toBe(false)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('clears the save message when the dialog is opened again', async () => {
    vi.mocked(updateDevice).mockRejectedValue(new Error('the device changed meanwhile'))
    const wrapper = await openPage()
    await wrapper.get('button[title="Edit Device"]').trigger('click')
    await wrapper.get('.modal-dialog .btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="device-save-error"]').exists()).toBe(true)
    await wrapper.get('.modal-dialog .btn-secondary').trigger('click')

    await wrapper.get('button[title="Edit Device"]').trigger('click')

    expect(wrapper.find('[data-testid="device-save-error"]').exists()).toBe(false)
  })
})
