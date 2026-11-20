import { describe, it, expect, beforeEach, vi } from 'vitest'
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

import { getDevice, getDeviceStatus, getDeviceEnergy } from '../../api/devices'
import { detectConfigDrift } from '../../api/deviceConfig'
import { getDeviceCapabilities } from '../../api/typedConfig'

// A device that does not answer (ginsys/shelly-manager#755): the page must show what the manager
// stores at once and fill each live read in as it arrives.
const device = {
  id: 7,
  ip: '10.0.0.7',
  mac: 'AA',
  type: 'SHSW-1',
  name: 'Garage',
  firmware: '',
  status: 'online',
  last_seen: new Date().toISOString(),
  settings: '{}',
}

const unanswered = (message: string) => ({
  isAxiosError: true,
  message: 'Request failed with status code 504',
  response: { status: 504, data: { success: false, error: { code: 'DEVICE_UNREACHABLE', message } } },
})

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

const refreshButton = (wrapper: Awaited<ReturnType<typeof openPage>>) =>
  wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!

describe('DeviceDetailPage with a device that does not answer', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(getDevice).mockImplementation(async () => ({ ...device }))
    vi.mocked(getDeviceEnergy).mockRejectedValue(new Error('no meter'))
    vi.mocked(detectConfigDrift).mockRejectedValue(new Error('none'))
    vi.mocked(getDeviceCapabilities).mockRejectedValue(new Error('none'))
  })

  it('shows the stored device while the status read is still pending', async () => {
    vi.mocked(getDeviceStatus).mockImplementation(() => new Promise(() => {}))
    const wrapper = await openPage()

    expect(wrapper.text()).not.toContain('Loading...')
    expect(wrapper.text()).toContain('Garage')
    expect(wrapper.text()).toContain('10.0.0.7')
    expect(wrapper.find('[data-testid="status-pending"]').exists()).toBe(true)
  })

  it('says the device did not answer, with the server text that names it', async () => {
    const text = 'device "Garage" (10.0.0.7) did not answer'
    vi.mocked(getDeviceStatus).mockRejectedValue(unanswered(text))
    const wrapper = await openPage()

    expect(wrapper.get('[data-testid="status-failed"]').text()).toContain(text)
    expect(wrapper.text()).toContain('Garage')
  })

  it('says it for the energy read too, and stays quiet for a device without a meter', async () => {
    vi.mocked(getDeviceStatus).mockResolvedValue({ online: true } as never)
    vi.mocked(getDeviceEnergy).mockRejectedValue(unanswered('device "Garage" (10.0.0.7) did not answer'))
    const wrapper = await openPage()
    expect(wrapper.get('[data-testid="energy-failed"]').text()).toContain('did not answer')

    vi.mocked(getDeviceEnergy).mockRejectedValue(new Error('no meter'))
    await refreshButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="energy-failed"]').exists()).toBe(false)
  })

  it('keeps the stored device on screen while Refresh waits for the device', async () => {
    vi.mocked(getDeviceStatus).mockResolvedValue({ online: true } as never)
    const wrapper = await openPage()

    vi.mocked(getDeviceStatus).mockImplementation(() => new Promise(() => {}))
    await refreshButton(wrapper).trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('Loading...')
    expect(wrapper.text()).toContain('Garage')
  })

  it('does not let a late status answer replace a newer one', async () => {
    vi.mocked(getDeviceStatus).mockResolvedValue({ online: true, temperature: 20 } as never)
    const wrapper = await openPage()

    let answerOld!: (value: unknown) => void
    vi.mocked(getDeviceStatus).mockImplementationOnce(() => new Promise((resolve) => { answerOld = resolve }) as never)
    await refreshButton(wrapper).trigger('click')
    await flushPromises()
    vi.mocked(getDeviceStatus).mockResolvedValueOnce({ online: true, temperature: 33 } as never)
    await refreshButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('33°C')

    answerOld({ online: true, temperature: 99 })
    await flushPromises()
    expect(wrapper.text()).toContain('33°C')
    expect(wrapper.text()).not.toContain('99°C')
  })

  it('starts no new periodic read while the last one is still running, so its timeout is shown', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
    try {
      let failFirst!: (reason: unknown) => void
      vi.mocked(getDeviceStatus).mockImplementationOnce(() => new Promise((_resolve, reject) => { failFirst = reject }))
      const wrapper = await openPage()
      expect(vi.mocked(getDeviceStatus)).toHaveBeenCalledTimes(1)

      // Two polling periods pass with the first read still running: nothing new starts.
      await vi.advanceTimersByTimeAsync(60_000)
      expect(vi.mocked(getDeviceStatus)).toHaveBeenCalledTimes(1)

      // The browser gives up on it: its failure is shown, and the next period reads again.
      failFirst(unanswered('device "Garage" (10.0.0.7) did not answer'))
      await flushPromises()
      expect(wrapper.get('[data-testid="status-failed"]').text()).toContain('did not answer')
      vi.mocked(getDeviceStatus).mockResolvedValue({ online: true } as never)
      await vi.advanceTimersByTimeAsync(30_000)
      await flushPromises()
      expect(vi.mocked(getDeviceStatus)).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows a device last seen two days ago as not seen recently, never online', async () => {
    vi.mocked(getDevice).mockImplementation(async () => ({
      ...device,
      last_seen: new Date(Date.now() - 2 * 24 * 3600 * 1000).toISOString(),
    }))
    vi.mocked(getDeviceStatus).mockImplementation(() => new Promise(() => {}))
    const wrapper = await openPage()

    expect(wrapper.get('[data-testid="device-status"]').text()).toBe('not seen recently')
  })
})
