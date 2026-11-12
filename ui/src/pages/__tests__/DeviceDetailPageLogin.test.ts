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

import { getDevice, getDeviceStatus, getDeviceEnergy, updateDeviceAuth } from '../../api/devices'
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

  it('drops a typed login when the page comes to show another device', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/devices/:id', component: DeviceDetailPage }],
    })
    router.push('/devices/7')
    await router.isReady()
    const wrapper = mount(RouterView, { global: { plugins: [router], stubs: { RouterLink: true } } })
    await flushPromises()
    await wrapper.get('[data-testid="device-login-username"]').setValue('typed-for-7')
    await wrapper.get('[data-testid="device-login-password"]').setValue('secret-for-7')

    // The same page instance now shows device 8 after a Refresh.
    vi.mocked(getDevice).mockImplementation(async () => ({
      ...device,
      id: 8,
      settings: JSON.stringify({ auth_user: 'admin8', auth_pass: '********' }),
    }))
    await router.push('/devices/8')
    const refresh = wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!
    await refresh.trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="device-login-username"]').element as HTMLInputElement).value).toBe('admin8')
    expect((wrapper.get('[data-testid="device-login-password"]').element as HTMLInputElement).value).toBe('********')
  })

  it('ignores a read that started before a save and arrives after it', async () => {
    const wrapper = await openPage()

    // Refresh starts a read that stays pending and will carry the login from before the save.
    let answerStale!: (value: typeof device) => void
    vi.mocked(getDevice).mockImplementationOnce(
      () => new Promise<typeof device>((resolve) => { answerStale = resolve }),
    )
    const refresh = wrapper.findAll('button').find((b) => b.text().trim() === 'Refresh')!
    await refresh.trigger('click')
    await flushPromises()

    // The save succeeds and the read it triggers returns the new login.
    vi.mocked(updateDeviceAuth).mockResolvedValue(undefined as never)
    vi.mocked(getDevice).mockImplementation(async () => ({
      ...device,
      settings: JSON.stringify({ auth_user: 'newuser', auth_pass: '********' }),
    }))
    await wrapper.get('[data-testid="device-login-username"]').setValue('newuser')
    await wrapper.get('[data-testid="device-login-password"]').setValue('secret')
    await wrapper.get('[data-testid="device-login-save"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="device-login-username"]').element as HTMLInputElement).value).toBe('newuser')

    // The older read now arrives with the login from before the save.
    answerStale({ ...device })
    await flushPromises()

    expect((wrapper.get('[data-testid="device-login-username"]').element as HTMLInputElement).value).toBe('newuser')
  })
})
