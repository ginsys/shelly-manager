import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DevicesPage from '../DevicesPage.vue'
import { listDevices, createDevice, deleteDevice, controlDevice } from '@/api/devices'
import { notice, dismissNotice } from '@/composables/useNotice'
import type { Device } from '@/api/types'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn(),
  createDevice: vi.fn(),
  updateDevice: vi.fn(),
  deleteDevice: vi.fn(),
  controlDevice: vi.fn(),
  isAbortError: () => false
}))

function device(id: number, name: string): Device {
  return {
    id,
    name,
    ip: `10.0.0.${id}`,
    mac: `AA:BB:CC:DD:EE:0${id}`,
    type: 'SHSW-1',
    firmware: '1.0.0',
    status: 'online',
    last_seen: '2026-11-18T20:00:00Z'
  }
}

const answer = (items: Device[]) =>
  ({ items, meta: { pagination: { total_pages: 1, has_next: false } } }) as any

const quiet = (name: string, props: Record<string, unknown> = {}) =>
  defineComponent({ name, props, setup: () => () => null })
const RouterLinkStub = defineComponent({
  name: 'RouterLink',
  setup(_, { slots }) {
    return () => h('span', slots.default?.())
  }
})

function memoryStorage() {
  const data = new Map<string, string>()
  return {
    getItem: (k: string) => (data.has(k) ? data.get(k)! : null),
    setItem: (k: string, v: string) => void data.set(k, String(v)),
    removeItem: (k: string) => void data.delete(k),
    clear: () => data.clear()
  }
}

let mounted: VueWrapper | null = null

async function mountPage() {
  vi.mocked(listDevices).mockResolvedValue(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
  mounted = mount(DevicesPage, {
    global: {
      stubs: {
        BulkOperationsDialog: quiet('BulkOperationsDialog', { isOpen: Boolean, selectedDevices: Array }),
        SortIcon: quiet('SortIcon', { field: String, sort: Object }),
        RouterLink: RouterLinkStub
      }
    }
  })
  await flushPromises()
  return mounted
}

// ginsys/shelly-manager#614: a failed device action shows an error panel, never a browser dialog.
describe('DevicesPage shows a failed device action as an error panel', () => {
  let alertSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('localStorage', memoryStorage())
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    mounted?.unmount()
    mounted = null
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('on a failed control', async () => {
    vi.mocked(controlDevice).mockRejectedValue(new Error('device is asleep'))
    const wrapper = await mountPage()

    await wrapper.get('button[title="Turn On"]').trigger('click')
    await flushPromises()

    const panel = wrapper.get('[data-testid="devices-action-error"]')
    expect(panel.text()).toContain('device is asleep')
    expect(panel.text()).toContain('Control device')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('on a failed delete', async () => {
    vi.mocked(deleteDevice).mockRejectedValue(new Error('device is locked'))
    const wrapper = await mountPage()

    await wrapper.get('button[title="Delete"]').trigger('click')
    await flushPromises()

    const panel = wrapper.get('[data-testid="devices-action-error"]')
    expect(panel.text()).toContain('device is locked')
    expect(panel.text()).toContain('Delete device')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('and removes it with its dismiss button', async () => {
    vi.mocked(controlDevice).mockRejectedValue(new Error('device is asleep'))
    const wrapper = await mountPage()
    await wrapper.get('button[title="Turn On"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="devices-action-error"] .dismiss-button').trigger('click')

    expect(wrapper.find('[data-testid="devices-action-error"]').exists()).toBe(false)
  })
})

// ginsys/shelly-manager#648: a failure the page panel cannot show goes to the app notice.
describe('DevicesPage failures that the page panel cannot show', () => {
  // A request that stays open until the test fails it.
  function hold(fn: (...a: any[]) => any) {
    let fail: (e: Error) => void = () => {}
    vi.mocked(fn).mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          fail = reject
        }) as any
    )
    return (e: Error) => fail(e)
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('localStorage', memoryStorage())
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    dismissNotice()
  })

  afterEach(() => {
    mounted?.unmount()
    mounted = null
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('shows a delete that fails while the add-device dialog is open in the notice', async () => {
    const failDelete = hold(deleteDevice)
    const wrapper = await mountPage()

    await wrapper.get('button[title="Delete"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="add-device-btn"]').trigger('click')
    expect(wrapper.find('.modal-overlay').exists()).toBe(true)

    failDelete(new Error('device is locked'))
    await flushPromises()

    expect(notice.value?.message).toContain('device is locked')
    expect(notice.value?.context?.action).toBe('Delete device')
    expect(wrapper.find('[data-testid="devices-action-error"]').exists()).toBe(false)
  })

  it('shows a delete that fails after the page was left in the notice', async () => {
    const failDelete = hold(deleteDevice)
    const wrapper = await mountPage()

    await wrapper.get('button[title="Delete"]').trigger('click')
    await flushPromises()
    wrapper.unmount()
    mounted = null

    failDelete(new Error('device is locked'))
    await flushPromises()

    expect(notice.value?.message).toContain('device is locked')
    expect(notice.value?.context?.action).toBe('Delete device')
  })

  it('shows a save that fails after the page was left in the notice', async () => {
    const failCreate = hold(createDevice)
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="add-device-btn"]').trigger('click')
    wrapper.findComponent({ name: 'DeviceForm' }).vm.$emit('submit', { name: 'New' })
    await flushPromises()
    wrapper.unmount()
    mounted = null

    failCreate(new Error('name already used'))
    await flushPromises()

    expect(notice.value?.message).toContain('name already used')
    expect(notice.value?.context?.action).toBe('Save device')
  })

  it('shows a save that fails after its dialog was cancelled on the page panel', async () => {
    const failCreate = hold(createDevice)
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="add-device-btn"]').trigger('click')
    wrapper.findComponent({ name: 'DeviceForm' }).vm.$emit('submit', { name: 'New' })
    await flushPromises()
    wrapper.findComponent({ name: 'DeviceForm' }).vm.$emit('cancel')
    await flushPromises()

    failCreate(new Error('name already used'))
    await flushPromises()

    expect(wrapper.get('[data-testid="devices-action-error"]').text()).toContain('name already used')
    expect(notice.value).toBeNull()
  })

  it('keeps a save that fails while its dialog is open in the dialog', async () => {
    const failCreate = hold(createDevice)
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="add-device-btn"]').trigger('click')
    wrapper.findComponent({ name: 'DeviceForm' }).vm.$emit('submit', { name: 'New' })
    await flushPromises()

    failCreate(new Error('name already used'))
    await flushPromises()

    expect(wrapper.get('.error-message').text()).toContain('name already used')
    expect(notice.value).toBeNull()
    expect(wrapper.find('[data-testid="devices-action-error"]').exists()).toBe(false)
  })
})
