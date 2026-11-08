import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
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

// Every read answers with data that names its device, so a test can tell whose data is on screen.
function answerReads() {
  api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
  api.getDeviceOverridesNew.mockImplementation(async (id: any) => ({ overrides: { owner: `device-${id}` } }) as any)
  api.getDesiredConfigNew.mockImplementation(async (id: any) => ({ config: { owner: `device-${id}` }, sources: {} }) as any)
  api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: true, pending_changes: false }) as any)
}

async function mountAt(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
  })
  router.push(path)
  await router.isReady()
  const wrapper = mount(DeviceConfigPage, {
    global: {
      plugins: [router],
      stubs: { ConfigEditor: true, ConfigView: true, RouterLink: true },
    },
  })
  await flushPromises()
  return { wrapper, router }
}

function buttonWithText(wrapper: ReturnType<typeof mount>, text: string) {
  const found = wrapper.findAll('button').find((b) => b.text().trim() === text)
  if (!found) throw new Error(`no button with text "${text}"`)
  return found
}

describe('DeviceConfigPage follows the route (ginsys/shelly-manager#400)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    answerReads()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('saves overrides to the device in the address bar after the route changes', async () => {
    api.setDeviceOverridesNew.mockResolvedValue({ overrides: {} } as any)
    const { wrapper, router } = await mountAt('/devices/7/config')
    const store = useDeviceConfigNewStore()
    expect(store.overrides).toEqual({ owner: 'device-7' })

    await router.push('/devices/8/config')
    await flushPromises()

    // The page loaded the new device and dropped the old one.
    expect(api.getDeviceOverridesNew).toHaveBeenLastCalledWith('8')
    expect(store.overrides).toEqual({ owner: 'device-8' })

    await buttonWithText(wrapper, 'Edit').trigger('click')
    await buttonWithText(wrapper, 'Save').trigger('click')
    await flushPromises()

    expect(api.setDeviceOverridesNew).toHaveBeenCalledTimes(1)
    expect(api.setDeviceOverridesNew.mock.calls[0][0]).toBe('8')
  })

  it('applies to the device in the address bar after the route changes', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'alert').mockImplementation(() => {})
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: false, pending_changes: true }) as any)
    api.applyDeviceConfigNew.mockResolvedValue({ success: true, applied_count: 1, failed_count: 0 } as any)
    const { wrapper, router } = await mountAt('/devices/7/config')

    await router.push('/devices/8/config')
    await flushPromises()
    await buttonWithText(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    expect(api.applyDeviceConfigNew).toHaveBeenCalledTimes(1)
    expect(api.applyDeviceConfigNew.mock.calls[0][0]).toBe('8')
  })

  it('ignores a late answer for the device it left', async () => {
    let releaseOld: (value: any) => void = () => {}
    api.getDeviceOverridesNew.mockImplementation((id: any) => {
      if (String(id) === '7') {
        return new Promise((resolve) => {
          releaseOld = resolve
        }) as any
      }
      return Promise.resolve({ overrides: { owner: `device-${id}` } }) as any
    })
    const { router } = await mountAt('/devices/7/config')
    const store = useDeviceConfigNewStore()

    await router.push('/devices/8/config')
    await flushPromises()
    expect(store.overrides).toEqual({ owner: 'device-8' })

    releaseOld({ overrides: { owner: 'device-7' } })
    await flushPromises()

    expect(store.overrides).toEqual({ owner: 'device-8' })
    expect(store.loading).toBe(false)
  })

  it('leaves the editor closed and empty for the new device', async () => {
    const { wrapper, router } = await mountAt('/devices/7/config')
    await buttonWithText(wrapper, 'Edit').trigger('click')
    expect(wrapper.findAll('button').some((b) => b.text().trim() === 'Save')).toBe(true)

    await router.push('/devices/8/config')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().trim() === 'Save')).toBe(false)
  })
})
