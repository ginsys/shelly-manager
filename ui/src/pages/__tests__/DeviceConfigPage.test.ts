import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'

vi.mock('@/api/deviceConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/deviceConfig')>()
  return { ...actual, getCurrentDeviceConfig: vi.fn() }
})

import { getCurrentDeviceConfig } from '@/api/deviceConfig'

const stubs = {
  'router-link': true,
  ConfigEditor: true,
  ConfigView: {
    props: ['config'],
    template: '<pre class="stub-view">{{ JSON.stringify(config) }}</pre>',
  },
}

// The page follows the route, so the tests use a real router and can move it to another device.
async function mountPage() {
  const store = useDeviceConfigNewStore()
  // The page refreshes the desired-config store on mount; keep that offline.
  vi.spyOn(store, 'refresh').mockResolvedValue(undefined as never)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
  })
  router.push('/devices/7/config')
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs } })
  await flushPromises()
  return { wrapper, router }
}

describe('DeviceConfigPage live panel (#388)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('does not contact the device on page load', async () => {
    await mountPage()

    expect(getCurrentDeviceConfig).not.toHaveBeenCalled()
  })

  it('shows the device settings after the button is clicked', async () => {
    vi.mocked(getCurrentDeviceConfig).mockResolvedValue({
      id: 1,
      device_id: 7,
      config: { name: 'Live Name' },
    })
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()

    expect(getCurrentDeviceConfig).toHaveBeenCalledWith('7')
    expect(wrapper.get('[data-testid="live-config"]').text()).toContain('Live Name')
    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
  })

  it('shows the error text when the device cannot be reached, without failing the page', async () => {
    vi.mocked(getCurrentDeviceConfig).mockRejectedValue(new Error('Device not responding'))
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="live-error"]').text()).toBe('Device not responding')
    expect(wrapper.find('[data-testid="live-config"]').exists()).toBe(false)
    // The rest of the page is still there.
    expect(wrapper.text()).toContain('Desired Configuration')
  })

  it('clears an earlier error when a later read succeeds', async () => {
    vi.mocked(getCurrentDeviceConfig)
      .mockRejectedValueOnce(new Error('Device not responding'))
      .mockResolvedValueOnce({ config: { name: 'Back Online' } })
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="live-config"]').text()).toContain('Back Online')
  })

  it('drops what device 7 reported when the page moves to device 8', async () => {
    vi.mocked(getCurrentDeviceConfig).mockResolvedValue({ config: { name: 'Device Seven' } })
    const { wrapper, router } = await mountPage()
    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="live-config"]').text()).toContain('Device Seven')

    await router.push('/devices/8/config')
    await flushPromises()

    expect(wrapper.find('[data-testid="live-config"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Device Seven')
    expect(wrapper.get('[data-testid="live-load"]').text()).toBe('Read from device')
  })

  it('drops the error of device 7 when the page moves to device 8', async () => {
    vi.mocked(getCurrentDeviceConfig).mockRejectedValue(new Error('Device 7 not responding'))
    const { wrapper, router } = await mountPage()
    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="live-error"]').text()).toBe('Device 7 not responding')

    await router.push('/devices/8/config')
    await flushPromises()

    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
  })

  // Starts a read of device 7 that stays pending, moves the page to device 8, and hands back the
  // two ends of the pending read so a test can settle it after the move.
  async function startReadThenMoveToDevice8() {
    let resolveRead: (value: unknown) => void = () => {}
    let rejectRead: (reason: Error) => void = () => {}
    vi.mocked(getCurrentDeviceConfig).mockImplementation(
      () => new Promise((resolve, reject) => {
        resolveRead = resolve
        rejectRead = reject
      }) as never,
    )
    const { wrapper, router } = await mountPage()
    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="live-load"]').attributes('disabled')).toBeDefined()

    await router.push('/devices/8/config')
    await flushPromises()
    // The device change frees the button for device 8 while the read of device 7 is still out.
    expect(wrapper.get('[data-testid="live-load"]').attributes('disabled')).toBeUndefined()
    return { wrapper, resolveRead, rejectRead }
  }

  it('ignores a read of device 7 that answers after the page moved to device 8', async () => {
    const { wrapper, resolveRead } = await startReadThenMoveToDevice8()

    resolveRead({ config: { name: 'Device Seven' } })
    await flushPromises()

    expect(wrapper.find('[data-testid="live-config"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Device Seven')
    expect(wrapper.get('[data-testid="live-load"]').attributes('disabled')).toBeUndefined()
  })

  it('ignores a read of device 7 that fails after the page moved to device 8', async () => {
    const { wrapper, rejectRead } = await startReadThenMoveToDevice8()

    rejectRead(new Error('Device 7 not responding'))
    await flushPromises()

    expect(wrapper.find('[data-testid="live-config"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="live-load"]').attributes('disabled')).toBeUndefined()
  })
})
