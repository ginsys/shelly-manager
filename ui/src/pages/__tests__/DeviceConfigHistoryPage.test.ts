import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigHistoryPage from '../DeviceConfigHistoryPage.vue'

vi.mock('@/api/deviceConfig', () => ({
  getConfigHistory: vi.fn(),
  updateDeviceConfig: vi.fn(),
}))

import { getConfigHistory, updateDeviceConfig } from '@/api/deviceConfig'

const entry = {
  id: 1,
  device_id: 7,
  config: { mqtt: { port: 1883 } },
  source: 'manual',
  user: 'serge',
  timestamp: '2026-11-18T20:00:00Z',
}

// ginsys/shelly-manager#614: a failed restore shows an error panel and a restore that worked
// leaves the page for the configuration page. Neither opens a browser dialog.
describe('DeviceConfigHistoryPage restore', () => {
  let alertSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.resetAllMocks()
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.mocked(getConfigHistory).mockResolvedValue([entry] as any)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  async function openPage() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/devices/:id/config/history', component: DeviceConfigHistoryPage },
        { path: '/devices/:id/config', component: { template: '<div>config page</div>' } },
      ],
    })
    router.push('/devices/7/config/history')
    await router.isReady()
    const wrapper = mount(RouterView, { global: { plugins: [router], stubs: { RouterLink: true } } })
    await flushPromises()
    return { wrapper, router }
  }

  it('shows a failed restore as an error panel under the toolbar', async () => {
    vi.mocked(updateDeviceConfig).mockRejectedValue(new Error('device refused the configuration'))
    const { wrapper } = await openPage()

    const restore = wrapper.findAll('button').find((b) => b.text().trim() === 'Restore')!
    await restore.trigger('click')
    await flushPromises()

    const panel = wrapper.get('[data-testid="restore-error"]')
    expect(panel.text()).toContain('device refused the configuration')
    expect(panel.text()).toContain('Restore configuration')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('shows the failure inside the viewer when the restore started from the viewer', async () => {
    vi.mocked(updateDeviceConfig).mockRejectedValue(new Error('device refused the configuration'))
    const { wrapper } = await openPage()
    await wrapper.findAll('button').find((b) => b.text().trim() === 'View')!.trigger('click')

    await wrapper.findAll('button').find((b) => b.text().trim() === 'Restore This Version')!.trigger('click')
    await flushPromises()

    expect(wrapper.get('.modal [data-testid="restore-error"]').text()).toContain('device refused the configuration')
    expect(wrapper.findAll('[data-testid="restore-error"]')).toHaveLength(1)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('goes to the configuration page after a restore that worked, with no dialog', async () => {
    vi.mocked(updateDeviceConfig).mockResolvedValue(undefined as any)
    const { wrapper, router } = await openPage()

    const restore = wrapper.findAll('button').find((b) => b.text().trim() === 'Restore')!
    await restore.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/devices/7/config')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('clears the failure as soon as the next restore starts, before it finishes', async () => {
    vi.mocked(updateDeviceConfig).mockRejectedValueOnce(new Error('device refused the configuration'))
    let finishSecond: (value: any) => void = () => {}
    vi.mocked(updateDeviceConfig).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishSecond = resolve
        }) as any,
    )
    const { wrapper, router } = await openPage()
    const restore = () => wrapper.findAll('button').find((b) => b.text().trim() === 'Restore')!

    await restore().trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="restore-error"]').exists()).toBe(true)

    await restore().trigger('click')
    await flushPromises() // the second restore is still open
    expect(vi.mocked(updateDeviceConfig)).toHaveBeenCalledTimes(2)
    expect(router.currentRoute.value.path).toBe('/devices/7/config/history')
    expect(wrapper.find('[data-testid="restore-error"]').exists()).toBe(false)

    finishSecond(undefined)
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/devices/7/config')
  })

  it('dismisses the failure with its Dismiss button', async () => {
    vi.mocked(updateDeviceConfig).mockRejectedValue(new Error('device refused the configuration'))
    const { wrapper } = await openPage()

    await wrapper.findAll('button').find((b) => b.text().trim() === 'Restore')!.trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="restore-error"] .dismiss-button').trigger('click')

    expect(wrapper.find('[data-testid="restore-error"]').exists()).toBe(false)
  })

  it('shows no Retry button on a retryable failure, since nothing handles it', async () => {
    vi.mocked(updateDeviceConfig).mockRejectedValue(
      Object.assign(new Error('Network Error'), { code: 'ERR_NETWORK', isAxiosError: true }),
    )
    const { wrapper } = await openPage()

    await wrapper.findAll('button').find((b) => b.text().trim() === 'Restore')!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="restore-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="restore-error"] .retry-button').exists()).toBe(false)
  })
})
