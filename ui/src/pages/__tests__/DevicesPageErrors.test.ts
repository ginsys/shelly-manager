import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DevicesPage from '../DevicesPage.vue'
import { listDevices, deleteDevice, controlDevice } from '@/api/devices'
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
