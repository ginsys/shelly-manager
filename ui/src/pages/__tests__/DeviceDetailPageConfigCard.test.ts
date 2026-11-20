import { describe, it, expect, beforeEach, vi } from 'vitest'
import { defineComponent, h } from 'vue'
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

// ginsys/shelly-manager#756: the Configuration card had two controls for one page, and an empty
// drift status that gave no reason.
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

// A link that renders its target and its text, which `stubs: { RouterLink: true }` does not. It is a
// span: the global test setup replaces document.createElement('a') with a mock element.
const link = defineComponent({
  props: { to: { type: String, default: '' } },
  setup(props, { slots }) {
    return () => h('span', { 'data-to': props.to }, slots.default?.())
  },
})

async function openPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id', component: DeviceDetailPage }],
  })
  router.push('/devices/7')
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs: { RouterLink: link } } })
  await flushPromises()
  return wrapper
}

describe('DeviceDetailPage Configuration card', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(getDevice).mockImplementation(async () => ({ ...device }))
    vi.mocked(getDeviceStatus).mockResolvedValue({ online: true } as never)
    vi.mocked(getDeviceEnergy).mockRejectedValue(new Error('no meter'))
    vi.mocked(getDeviceCapabilities).mockRejectedValue(new Error('none'))
  })

  it('has one way to the configuration page, plus the history', async () => {
    vi.mocked(detectConfigDrift).mockResolvedValue({ hasDrift: false })
    const wrapper = await openPage()

    const links = wrapper.findAll('[data-to]').map((a) => ({ text: a.text(), href: a.attributes('data-to') }))
    expect(links.filter((l) => l.href === '/devices/7/config')).toEqual([
      { text: 'Edit Configuration', href: '/devices/7/config' },
    ])
    expect(links.map((l) => l.text)).toContain('View History')
    expect(wrapper.text()).not.toContain('View Full Config')
  })

  it('says why there is no drift status when the device has no desired configuration', async () => {
    vi.mocked(detectConfigDrift).mockResolvedValue({ hasDrift: false, noDesiredConfig: true })
    const wrapper = await openPage()

    expect(wrapper.get('[data-testid="drift-no-desired"]').text()).toBe(
      'No desired configuration yet: add a template or overrides',
    )
    expect(wrapper.text()).not.toContain('No Drift')
  })

  it('says why the check failed, with the reason', async () => {
    vi.mocked(detectConfigDrift).mockRejectedValue(new Error('Internal server error'))
    const wrapper = await openPage()

    expect(wrapper.get('[data-testid="drift-failed"]').text()).toBe('Could not check drift: Internal server error')
  })

  it('does not let an older check that fails replace a newer one that succeeded', async () => {
    let failOld!: (reason: Error) => void
    vi.mocked(detectConfigDrift).mockImplementationOnce(
      () => new Promise((_resolve, reject) => { failOld = reject }),
    )
    const wrapper = await openPage()
    expect(wrapper.text()).toContain('Checking...')

    vi.mocked(detectConfigDrift).mockResolvedValueOnce({ hasDrift: false })
    const refresh = wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!
    await refresh.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('No Drift')

    failOld(new Error('late failure'))
    await flushPromises()
    expect(wrapper.text()).toContain('No Drift')
    expect(wrapper.find('[data-testid="drift-failed"]').exists()).toBe(false)
  })

  it('still shows a real comparison as before', async () => {
    vi.mocked(detectConfigDrift).mockResolvedValue({ hasDrift: true, driftFields: ['wifi.ssid'] })
    const wrapper = await openPage()

    expect(wrapper.text()).toContain('Configuration Drift Detected')
    expect(wrapper.text()).toContain('wifi.ssid')
  })
})
