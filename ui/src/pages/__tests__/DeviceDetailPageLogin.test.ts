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

import { getDevice, getDeviceStatus, getDeviceEnergy } from '../../api/devices'
import { detectConfigDrift } from '../../api/deviceConfig'

const device = {
  id: 7,
  ip: '10.0.0.7',
  mac: 'AA',
  type: 'SHSW-1',
  name: 'Plug',
  firmware: '',
  status: 'online',
  last_seen: '',
  settings: JSON.stringify({ auth_user: 'admin', auth_pass: '********' }),
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

// Refresh sets the page's loading flag, which unmounts everything inside the loading conditional.
describe('DeviceDetailPage Device Login card', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(getDevice).mockImplementation(async () => ({ ...device }))
    vi.mocked(getDeviceStatus).mockRejectedValue(new Error('offline'))
    vi.mocked(getDeviceEnergy).mockRejectedValue(new Error('offline'))
    vi.mocked(detectConfigDrift).mockRejectedValue(new Error('none'))
  })

  it('shows the stored login', async () => {
    const wrapper = await openPage()
    expect((wrapper.get('[data-testid="device-login-username"]').element as HTMLInputElement).value).toBe('admin')
  })

  it('keeps what the user typed when Refresh reloads the device', async () => {
    const wrapper = await openPage()
    await wrapper.get('[data-testid="device-login-username"]').setValue('typing')
    await wrapper.get('[data-testid="device-login-password"]').setValue('half-typed')

    const refresh = wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!
    await refresh.trigger('click')
    await flushPromises()

    expect(vi.mocked(getDevice)).toHaveBeenCalledTimes(2)
    expect((wrapper.get('[data-testid="device-login-username"]').element as HTMLInputElement).value).toBe('typing')
    expect((wrapper.get('[data-testid="device-login-password"]').element as HTMLInputElement).value).toBe('half-typed')
  })
})
