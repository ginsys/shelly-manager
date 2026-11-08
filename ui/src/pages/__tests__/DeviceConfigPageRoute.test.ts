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

  it('reports an apply as done when the route changes while its status loads', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    void confirmSpy
    let releaseOldStatus: (value: any) => void = () => {}
    let oldStatusCalls = 0
    api.getConfigStatusNew.mockImplementation((id: any) => {
      if (String(id) === '7') {
        oldStatusCalls++
        if (oldStatusCalls === 1) return Promise.resolve({ config_applied: false, pending_changes: true }) as any
        return new Promise((resolve) => {
          releaseOldStatus = resolve
        }) as any
      }
      return Promise.resolve({ config_applied: true, pending_changes: false, owner: 'device-8' }) as any
    })
    api.applyDeviceConfigNew.mockResolvedValue({ success: true, applied_count: 1, failed_count: 0 } as any)
    const { wrapper, router } = await mountAt('/devices/7/config')
    const store = useDeviceConfigNewStore()

    await buttonWithText(wrapper, 'Apply Config').trigger('click')
    await flushPromises() // the apply is done, the status of device 7 is still loading

    await router.push('/devices/8/config')
    await flushPromises()
    releaseOldStatus({ config_applied: false, pending_changes: true, owner: 'device-7' })
    await flushPromises()

    expect(alertSpy).toHaveBeenCalledTimes(1)
    expect(alertSpy.mock.calls[0][0]).toContain('Applied successfully')
    expect((store.status as any)?.owner).toBe('device-8')
  })

  it('does not load the status of the old device when its apply ends after the route changed', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'alert').mockImplementation(() => {})
    let releaseApply: (value: any) => void = () => {}
    api.applyDeviceConfigNew.mockImplementation(
      () =>
        new Promise((resolve) => {
          releaseApply = resolve
        }) as any,
    )
    api.getConfigStatusNew.mockImplementation(async (id: any) => ({
      config_applied: false,
      pending_changes: String(id) === '7',
      owner: `device-${id}`,
    }) as any)
    const { wrapper, router } = await mountAt('/devices/7/config')
    const store = useDeviceConfigNewStore()

    await buttonWithText(wrapper, 'Apply Config').trigger('click')
    await flushPromises() // the apply request of device 7 is still open

    await router.push('/devices/8/config')
    await flushPromises()
    releaseApply({ success: true, applied_count: 1, failed_count: 0 })
    await flushPromises()

    const statusReads = api.getConfigStatusNew.mock.calls.map((c) => String(c[0]))
    expect(statusReads.filter((id) => id === '7')).toHaveLength(1) // the read at mount only
    expect((store.status as any)?.owner).toBe('device-8')
    expect(store.loading).toBe(false)
  })

  it('keeps the add-template dialog of the new device when the old add ends late', async () => {
    let releaseOld: (value: any) => void = () => {}
    let oldTemplateReads = 0
    api.addDeviceTemplateNew.mockResolvedValue([] as any)
    api.getDeviceTemplatesNew.mockImplementation((id: any) => {
      if (String(id) === '7' && ++oldTemplateReads === 2) {
        return new Promise((resolve) => {
          releaseOld = resolve
        }) as any
      }
      return Promise.resolve({ templates: [], template_ids: [] }) as any
    })
    const { wrapper, router } = await mountAt('/devices/7/config')
    const vm = wrapper.vm as any

    vm.newTemplateId = 5
    void vm.handleAddTemplate()
    await flushPromises() // the add is done, the refresh of device 7 is still open

    await router.push('/devices/8/config')
    await flushPromises()
    vm.showAddTemplate = true
    vm.newTemplateId = 9

    releaseOld({ templates: [], template_ids: [] })
    await flushPromises()

    expect(vm.showAddTemplate).toBe(true)
    expect(vm.newTemplateId).toBe(9)
  })

  it('keeps the edits after the route returns to the first device and its old refresh ends', async () => {
    let releaseFirst: (value: any) => void = () => {}
    let reads = 0
    api.getDeviceOverridesNew.mockImplementation((id: any) => {
      if (String(id) === '7' && ++reads === 1) {
        return new Promise((resolve) => {
          releaseFirst = resolve
        }) as any
      }
      return Promise.resolve({ overrides: { owner: `device-${id}` } }) as any
    })
    const { wrapper, router } = await mountAt('/devices/7/config')

    await router.push('/devices/8/config')
    await flushPromises()
    await router.push('/devices/7/config')
    await flushPromises()
    ;(wrapper.vm as any).overridesData = { draft: true }

    releaseFirst({ overrides: { owner: 'device-7' } })
    await flushPromises()

    expect((wrapper.vm as any).overridesData).toEqual({ draft: true })
  })

  it('keeps the edits of the new device when the old refresh fails late', async () => {
    let rejectOld: (reason: any) => void = () => {}
    api.getDeviceOverridesNew.mockImplementation((id: any) => {
      if (String(id) === '7') {
        return new Promise((_resolve, reject) => {
          rejectOld = reject
        }) as any
      }
      return Promise.resolve({ overrides: { owner: `device-${id}` } }) as any
    })
    const { wrapper, router } = await mountAt('/devices/7/config')

    await router.push('/devices/8/config')
    await flushPromises()
    ;(wrapper.vm as any).overridesData = { draft: true }

    rejectOld(new Error('device 7 is gone'))
    await flushPromises()

    expect((wrapper.vm as any).overridesData).toEqual({ draft: true })
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
