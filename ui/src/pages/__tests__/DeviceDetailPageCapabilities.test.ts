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

// The model name says "relay" and "rgbw", and the status carries wifi, mqtt, cloud and a temperature.
// The page used to build the card from those guesses. It must now show only what the route lists.
const device = {
  id: 7,
  ip: '10.0.0.7',
  mac: 'AA',
  type: 'SHRGBW2 relay',
  name: 'Lamp',
  firmware: '',
  status: 'online',
  last_seen: '',
  settings: '{}',
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

describe('DeviceDetailPage capabilities card', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(getDevice).mockImplementation(async () => ({ ...device }))
    vi.mocked(getDeviceStatus).mockResolvedValue({
      wifi: { ssid: 'x' },
      mqtt: { connected: true },
      cloud: { connected: true },
      temperature: 41,
    } as never)
    vi.mocked(getDeviceEnergy).mockRejectedValue(new Error('offline'))
    vi.mocked(detectConfigDrift).mockRejectedValue(new Error('none'))
  })

  it('shows what GET /devices/{id}/capabilities lists, and nothing the page guessed', async () => {
    vi.mocked(getDeviceCapabilities).mockResolvedValue({
      device_id: 7,
      device_model: 'SHSW-25',
      generation: 1,
      capabilities: ['wifi', 'relay', 'roller'],
    })
    const wrapper = await openPage()

    const names = wrapper.findAll('.capability-name').map((n) => n.text())
    expect(names).toEqual(['WiFi', 'Relay Control', 'Roller/Shutter'])
    expect(vi.mocked(getDeviceCapabilities)).toHaveBeenCalledWith('7', expect.anything())
  })

  it('shows a name it does not know as it came', async () => {
    vi.mocked(getDeviceCapabilities).mockResolvedValue({
      device_id: 7,
      device_model: 'SHSW-1',
      generation: 1,
      capabilities: ['relay', 'something_new'],
    })
    const wrapper = await openPage()

    expect(wrapper.findAll('.capability-name').map((n) => n.text())).toEqual(['Relay Control', 'something_new'])
  })

  it('drops the old list when a refresh cannot read the capabilities', async () => {
    vi.mocked(getDeviceCapabilities).mockResolvedValueOnce({
      device_id: 7,
      device_model: 'SHSW-25',
      generation: 1,
      capabilities: ['wifi', 'roller'],
    })
    vi.mocked(getDeviceCapabilities).mockRejectedValueOnce(new Error('500'))
    const wrapper = await openPage()
    expect(wrapper.findAll('.capability-name')).toHaveLength(2)

    const refresh = wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!
    await refresh.trigger('click')
    await flushPromises()

    expect(vi.mocked(getDeviceCapabilities)).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="capabilities-card"]').exists()).toBe(false)
  })

  it('ignores a capabilities answer that arrives after a newer read', async () => {
    const wrapper = await openPage()

    // Two Refresh reads stay pending (one will answer late, one will fail late); a third answers first.
    let answerOld!: (value: Awaited<ReturnType<typeof getDeviceCapabilities>>) => void
    let failOld!: (reason: Error) => void
    vi.mocked(getDeviceCapabilities).mockImplementationOnce(
      () => new Promise((resolve) => { answerOld = resolve }),
    )
    vi.mocked(getDeviceCapabilities).mockImplementationOnce(
      () => new Promise((_resolve, reject) => { failOld = reject }),
    )
    const refresh = () => wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!
    await refresh().trigger('click')
    await flushPromises()
    await refresh().trigger('click')
    await flushPromises()
    vi.mocked(getDeviceCapabilities).mockResolvedValueOnce({
      device_id: 8,
      device_model: 'SHSW-1',
      generation: 1,
      capabilities: ['relay'],
    })
    await refresh().trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.capability-name').map((n) => n.text())).toEqual(['Relay Control'])

    // The older read now answers with another list, then another fails: neither may touch the card.
    answerOld({ device_id: 7, device_model: 'SHSW-25', generation: 1, capabilities: ['wifi', 'roller'] })
    await flushPromises()
    expect(wrapper.findAll('.capability-name').map((n) => n.text())).toEqual(['Relay Control'])
    failOld(new Error('500'))
    await flushPromises()
    expect(wrapper.findAll('.capability-name').map((n) => n.text())).toEqual(['Relay Control'])
  })

  it('hides the card when the route fails', async () => {
    vi.mocked(getDeviceCapabilities).mockRejectedValue(new Error('500'))
    const wrapper = await openPage()

    expect(wrapper.find('[data-testid="capabilities-card"]').exists()).toBe(false)
  })
})
