import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import { defineComponent, h } from 'vue'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'
import * as configNewApi from '@/api/configNew'

vi.mock('@/api/configNew', () => ({
  getDeviceTemplatesNew: vi.fn(),
  setDeviceTemplatesNew: vi.fn(),
  addDeviceTemplateNew: vi.fn(),
  removeDeviceTemplateNew: vi.fn(),
  getDeviceOverridesNew: vi.fn(),
  setDeviceOverridesNew: vi.fn(),
  patchDeviceOverridesNew: vi.fn(),
  deleteDeviceOverridesNew: vi.fn(),
  getDesiredConfigNew: vi.fn(),
  getConfigStatusNew: vi.fn(),
  applyDeviceConfigNew: vi.fn(),
  verifyDeviceConfigNew: vi.fn(),
}))

const api = vi.mocked(configNewApi)

// ginsys/shelly-manager#402: the config page is left through the device page, so the id of the
// second config page differs from the first without any id change on the page itself.
const DeviceDetailStub = defineComponent({ render: () => h('div', 'device detail') })

async function mountApp(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/devices/:id', component: DeviceDetailStub },
      { path: '/devices/:id/config', component: DeviceConfigPage },
    ],
  })
  router.push(path)
  await router.isReady()
  const wrapper = mount(RouterView, {
    global: { plugins: [router], stubs: { ConfigEditor: true, ConfigView: true, RouterLink: true } },
  })
  await flushPromises()
  return { wrapper, router }
}

function buttonWithText(wrapper: ReturnType<typeof mount>, text: string) {
  const found = wrapper.findAll('button').find((b) => b.text().trim() === text)
  if (!found) throw new Error(`no button with text "${text}"`)
  return found
}

describe('DeviceConfigPage drops work when it is left (ginsys/shelly-manager#402)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
    api.getDeviceOverridesNew.mockImplementation(async (id: any) => ({ overrides: { owner: `device-${id}` } }) as any)
    api.getDesiredConfigNew.mockImplementation(async (id: any) => ({ config: { owner: `device-${id}` }, sources: {} }) as any)
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: true, pending_changes: false }) as any)
  })

  it('does not show the verify answer of the old device on the page of the next device', async () => {
    let releaseVerify: (value: any) => void = () => {}
    api.verifyDeviceConfigNew.mockImplementation(
      () =>
        new Promise((resolve) => {
          releaseVerify = resolve
        }) as any,
    )
    const { wrapper, router } = await mountApp('/devices/7/config')
    const store = useDeviceConfigNewStore()

    await buttonWithText(wrapper, 'Verify Device').trigger('click') // the verify of device 7 stays open
    await router.push('/devices/7')
    await flushPromises()
    await router.push('/devices/8/config')
    await flushPromises()
    expect(store.overrides).toEqual({ owner: 'device-8' })

    releaseVerify({ match: false, differences: [{ path: 'owner', expected: 'device-7', actual: 'x' }] })
    await flushPromises()

    expect(store.lastVerify).toBeNull()
    expect(store.loading).toBe(false)
    expect(wrapper.text()).not.toContain('device-7')
  })

  it('does not apply the status answer of the old device to the next device', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'alert').mockImplementation(() => {})
    let releaseApply: (value: any) => void = () => {}
    api.getConfigStatusNew.mockImplementation(
      async (id: any) => ({ config_applied: false, pending_changes: String(id) === '7', owner: `device-${id}` }) as any,
    )
    api.applyDeviceConfigNew.mockImplementation(
      () =>
        new Promise((resolve) => {
          releaseApply = resolve
        }) as any,
    )
    const { wrapper, router } = await mountApp('/devices/7/config')
    const store = useDeviceConfigNewStore()

    await buttonWithText(wrapper, 'Apply Config').trigger('click') // the apply of device 7 stays open
    await router.push('/devices/7')
    await flushPromises()
    await router.push('/devices/8/config')
    await flushPromises()

    releaseApply({ success: true, applied_count: 1, failed_count: 0 })
    await flushPromises()

    expect(store.lastApply).toBeNull()
    expect((store.status as any)?.owner).toBe('device-8')
  })

  it('does not reload the old device after its save ends on a page that was left', async () => {
    let releaseSave: (value: any) => void = () => {}
    api.setDeviceOverridesNew.mockImplementation(
      () =>
        new Promise((resolve) => {
          releaseSave = resolve
        }) as any,
    )
    const { wrapper, router } = await mountApp('/devices/7/config')
    const store = useDeviceConfigNewStore()

    await buttonWithText(wrapper, 'Edit').trigger('click')
    await buttonWithText(wrapper, 'Save').trigger('click') // the save of device 7 stays open
    await router.push('/devices/7')
    await flushPromises()
    await router.push('/devices/8/config')
    await flushPromises()
    api.getDeviceOverridesNew.mockClear()

    releaseSave({ overrides: { owner: 'device-7' } })
    await flushPromises()

    expect(api.getDeviceOverridesNew).not.toHaveBeenCalled()
    expect(store.overrides).toEqual({ owner: 'device-8' })
  })
})
