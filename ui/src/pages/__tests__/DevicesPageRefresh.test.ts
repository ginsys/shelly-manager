import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DevicesPage from '../DevicesPage.vue'
import { useDevicesStore } from '@/stores/devices'
import { listDevices } from '@/api/devices'
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

// The real dialog has its own tests. The global test setup also replaces `createElement('a')`
// with a plain object, so the router link stub renders a span.
const DialogStub = defineComponent({
  name: 'BulkOperationsDialog',
  props: { isOpen: Boolean, selectedDevices: { type: Array, default: () => [] } },
  setup() {
    return () => null
  }
})
const SortIconStub = defineComponent({
  name: 'SortIcon',
  props: { field: String, sort: Object },
  setup() {
    return () => null
  }
})
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

// Window listeners only see events from elements that are in the document. The global test setup
// replaces `document.body.appendChild` with a mock, so `attachTo: document.body` attaches nothing;
// `append` is not mocked.
let host: HTMLElement | null = null
async function mountPage() {
  vi.mocked(listDevices).mockResolvedValue(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
  host = document.createElement('div')
  document.body.append(host)
  mounted = mount(DevicesPage, {
    attachTo: host,
    global: {
      stubs: { BulkOperationsDialog: DialogStub, SortIcon: SortIconStub, RouterLink: RouterLinkStub }
    }
  })
  await flushPromises()
  vi.mocked(listDevices).mockClear()
  return mounted
}

function press(key: string, init: KeyboardEventInit = {}, target: EventTarget = window) {
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init }))
}

const refreshButton = (w: VueWrapper) => w.get('[data-testid="refresh-btn"]')

// A load that stays pending until the test lets it finish.
function holdNextLoad() {
  let finish!: () => void
  vi.mocked(listDevices).mockReturnValueOnce(
    new Promise((resolve) => {
      finish = () => resolve(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
    }) as any
  )
  return () => finish()
}

describe('DevicesPage refresh', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('localStorage', memoryStorage())
  })

  afterEach(() => {
    mounted?.unmount()
    mounted = null
    host?.remove()
    host = null
    vi.unstubAllGlobals()
  })

  it('reloads the list when the button is pressed', async () => {
    const wrapper = await mountPage()

    await refreshButton(wrapper).trigger('click')
    await flushPromises()

    expect(listDevices).toHaveBeenCalledTimes(1)
    expect(refreshButton(wrapper).text()).toBe('Refresh')
  })

  it('shows a loading label and disables the button while a load runs', async () => {
    const wrapper = await mountPage()
    const finish = holdNextLoad()

    await refreshButton(wrapper).trigger('click')

    expect(refreshButton(wrapper).attributes('disabled')).toBeDefined()
    expect(refreshButton(wrapper).text()).toBe('Refreshing...')

    // A second press while loading starts nothing.
    await refreshButton(wrapper).trigger('click')
    expect(listDevices).toHaveBeenCalledTimes(1)

    finish()
    await flushPromises()
    expect(refreshButton(wrapper).attributes('disabled')).toBeUndefined()
    expect(refreshButton(wrapper).text()).toBe('Refresh')
  })

  it('announces the shortcut on the button', async () => {
    const wrapper = await mountPage()

    expect(refreshButton(wrapper).attributes('aria-keyshortcuts')).toBe('r')
    expect(refreshButton(wrapper).attributes('title')).toContain('R')
  })

  it('reloads on a plain r, in either case', async () => {
    await mountPage()

    press('r')
    await flushPromises()
    press('R')
    await flushPromises()

    expect(listDevices).toHaveBeenCalledTimes(2)
  })

  it('reloads when the key comes from an ordinary element, so the field tests below mean something', async () => {
    const wrapper = await mountPage()

    press('r', {}, wrapper.get('[data-testid="devices-table"]').element)
    await flushPromises()

    expect(listDevices).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['ctrl', { ctrlKey: true }],
    ['meta', { metaKey: true }],
    ['alt', { altKey: true }],
    ['shift', { shiftKey: true }],
    ['a key repeat', { repeat: true }]
  ])('does not reload when r comes with %s', async (_name, init) => {
    await mountPage()

    press('r', init)
    await flushPromises()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('does not reload on another key', async () => {
    await mountPage()

    press('x')
    press('Enter')
    await flushPromises()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('does not reload while the focus is in the search box, a select or a text area', async () => {
    const wrapper = await mountPage()
    const textarea = document.createElement('textarea')
    document.body.append(textarea)

    press('r', {}, wrapper.get('[data-testid="device-search"]').element)
    press('r', {}, wrapper.get('[data-testid="page-size-select"]').element)
    press('r', {}, textarea)
    await flushPromises()
    textarea.remove()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('does not reload from a content-editable element', async () => {
    await mountPage()
    const editable = document.createElement('div')
    editable.contentEditable = 'true'
    // jsdom does not compute isContentEditable from the attribute.
    Object.defineProperty(editable, 'isContentEditable', { value: true })
    document.body.append(editable)

    press('r', {}, editable)
    await flushPromises()
    editable.remove()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('does not reload while a dialog of the page is open', async () => {
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="add-device-btn"]').trigger('click')

    press('r')
    await flushPromises()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('does not reload while the bulk dialog is open', async () => {
    const wrapper = await mountPage()
    const store = useDevicesStore()
    store.toggleSelect(device(1, 'Alpha'))
    await flushPromises()
    await wrapper.get('[data-testid="bulk-ops-btn"]').trigger('click')

    press('r')
    await flushPromises()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('does not start a second load while one runs', async () => {
    const wrapper = await mountPage()
    const finish = holdNextLoad()
    await refreshButton(wrapper).trigger('click')

    press('r')
    await flushPromises()
    expect(listDevices).toHaveBeenCalledTimes(1)

    finish()
    await flushPromises()
  })

  it('keeps the selection across a refresh', async () => {
    const wrapper = await mountPage()
    const store = useDevicesStore()
    store.toggleSelect(device(2, 'Bravo'))

    await refreshButton(wrapper).trigger('click')
    await flushPromises()

    expect(store.selectedIds).toEqual([2])
    expect(wrapper.get('[data-testid="bulk-ops-btn"]').text()).toBe('Bulk Operations (1)')
  })

  it('stops listening for the shortcut once the page is gone', async () => {
    const wrapper = await mountPage()
    wrapper.unmount()
    mounted = null

    press('r')
    await flushPromises()

    expect(listDevices).not.toHaveBeenCalled()
  })
})
