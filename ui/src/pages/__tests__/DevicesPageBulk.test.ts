import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DevicesPage from '../DevicesPage.vue'
import { useDevicesStore } from '@/stores/devices'
import { listDevices, deleteDevice } from '@/api/devices'
import type { Device } from '@/api/types'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn(),
  createDevice: vi.fn(),
  updateDevice: vi.fn(),
  deleteDevice: vi.fn(),
  controlDevice: vi.fn(),
  isAbortError: () => false
}))

function device(id: number, name: string, ip: string): Device {
  return {
    id,
    name,
    ip,
    mac: `AA:BB:CC:DD:EE:0${id}`,
    type: 'SHSW-1',
    firmware: '1.0.0',
    status: 'online',
    last_seen: '2026-11-18T20:00:00Z'
  }
}

const devices = [
  device(1, 'Alpha', '10.0.0.1'),
  device(2, 'Bravo', '10.0.0.2'),
  device(3, 'Charlie', '10.0.0.3')
]

// The real dialog has its own tests; here it only has to show what the page hands it.
const DialogStub = defineComponent({
  name: 'BulkOperationsDialog',
  props: {
    isOpen: Boolean,
    selectedDevices: { type: Array, default: () => [] },
    checking: Boolean,
    removedCount: { type: Number, default: 0 }
  },
  emits: ['close', 'complete', 'remove'],
  setup(props) {
    return () => (props.isOpen ? h('div', { 'data-testid': 'bulk-dialog-stub' }) : null)
  }
})

const SortIconStub = defineComponent({
  name: 'SortIcon',
  props: { field: String, sort: Object },
  setup() {
    return () => null
  }
})

// A span, not an anchor: the global test setup replaces `document.createElement('a')` with a
// plain object, which Vue cannot mount children into.
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

async function mountPage() {
  vi.mocked(listDevices).mockResolvedValue({
    items: devices,
    meta: { pagination: { total_pages: 1, has_next: false } }
  } as any)
  const wrapper = mount(DevicesPage, {
    global: {
      stubs: {
        BulkOperationsDialog: DialogStub,
        // The page's inline SortIcon helper has a runtime template, which the test build of Vue
        // cannot compile.
        SortIcon: SortIconStub,
        RouterLink: RouterLinkStub
      }
    }
  })
  await flushPromises()
  return wrapper
}

const rowBoxes = (w: ReturnType<typeof mount>) => w.findAll('[data-testid="select-device"]')
const bulkButton = (w: ReturnType<typeof mount>) => w.get('[data-testid="bulk-ops-btn"]')

describe('DevicesPage bulk operations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('localStorage', memoryStorage())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('keeps the bulk button disabled while nothing is selected', async () => {
    const wrapper = await mountPage()

    expect(rowBoxes(wrapper)).toHaveLength(3)
    expect(bulkButton(wrapper).attributes('disabled')).toBeDefined()
    expect(bulkButton(wrapper).text()).toBe('Bulk Operations')
  })

  it('enables the button with the count once rows are ticked', async () => {
    const wrapper = await mountPage()

    await rowBoxes(wrapper)[0].setValue(true)
    await rowBoxes(wrapper)[2].setValue(true)

    expect(bulkButton(wrapper).attributes('disabled')).toBeUndefined()
    expect(bulkButton(wrapper).text()).toBe('Bulk Operations (2)')
  })

  it('selects and unselects the visible rows from the header checkbox', async () => {
    const wrapper = await mountPage()
    const store = useDevicesStore()
    const header = wrapper.get('[data-testid="select-all-devices"]')

    await header.setValue(true)
    expect(store.selectedIds).toEqual([1, 2, 3])

    await header.setValue(false)
    expect(store.selectedIds).toEqual([])
  })

  it('shows the header checkbox as indeterminate when only some rows are ticked', async () => {
    const wrapper = await mountPage()

    await rowBoxes(wrapper)[1].setValue(true)

    const header = wrapper.get('[data-testid="select-all-devices"]').element as HTMLInputElement
    expect(header.indeterminate).toBe(true)
    expect(header.checked).toBe(false)
  })

  it('hands the dialog exactly the selected devices, and nothing before the button is pressed', async () => {
    const wrapper = await mountPage()
    const dialog = wrapper.findComponent(DialogStub)
    expect(dialog.props('isOpen')).toBe(false)

    await rowBoxes(wrapper)[0].setValue(true)
    await rowBoxes(wrapper)[2].setValue(true)
    await bulkButton(wrapper).trigger('click')

    expect(dialog.props('isOpen')).toBe(true)
    expect(dialog.props('selectedDevices')).toEqual([
      { id: 1, name: 'Alpha', ip: '10.0.0.1' },
      { id: 3, name: 'Charlie', ip: '10.0.0.3' }
    ])
  })

  it('checks the selection when the button opens the dialog, and hands the dialog the outcome', async () => {
    const wrapper = await mountPage()
    const dialog = wrapper.findComponent(DialogStub)
    const store = useDevicesStore()
    // Device 9 was ticked on another page and was deleted since; 1 is on this page.
    store.toggleSelect(devices[0])
    store.toggleSelect(device(9, 'Elsewhere', '10.0.0.9'))
    await flushPromises() // the button enables once the store has a selection
    let answerList!: (r: unknown) => void
    vi.mocked(listDevices).mockClear()
    vi.mocked(listDevices).mockReturnValueOnce(new Promise((resolve) => (answerList = resolve)) as any)

    await bulkButton(wrapper).trigger('click')

    expect(listDevices).toHaveBeenCalledTimes(1)
    expect(listDevices).toHaveBeenCalledWith({ page: 1, pageSize: 0 })
    expect(dialog.props('isOpen')).toBe(true)
    expect(dialog.props('checking')).toBe(true)

    // The whole list no longer has device 9.
    answerList({ items: devices, meta: {} })
    await flushPromises()

    expect(dialog.props('checking')).toBe(false)
    expect(dialog.props('removedCount')).toBe(1)
    expect(dialog.props('selectedDevices')).toEqual([{ id: 1, name: 'Alpha', ip: '10.0.0.1' }])
    expect(bulkButton(wrapper).text()).toBe('Bulk Operations (1)')
  })

  it('asks for no whole list when every selected device is on the page', async () => {
    const wrapper = await mountPage()
    await rowBoxes(wrapper)[0].setValue(true)
    vi.mocked(listDevices).mockClear()

    await bulkButton(wrapper).trigger('click')
    await flushPromises()

    expect(listDevices).not.toHaveBeenCalled()
  })

  it('refetches the list when the dialog completes, and closes it on close', async () => {
    const wrapper = await mountPage()
    const dialog = wrapper.findComponent(DialogStub)
    await rowBoxes(wrapper)[0].setValue(true)
    await bulkButton(wrapper).trigger('click')
    vi.mocked(listDevices).mockClear()

    dialog.vm.$emit('complete', {})
    await flushPromises()
    expect(listDevices).toHaveBeenCalledTimes(1)

    dialog.vm.$emit('close')
    await flushPromises()
    expect(dialog.props('isOpen')).toBe(false)
  })

  it('drops a device from the selection when the dialog removes it', async () => {
    const wrapper = await mountPage()
    const dialog = wrapper.findComponent(DialogStub)
    const store = useDevicesStore()
    await rowBoxes(wrapper)[0].setValue(true)
    await rowBoxes(wrapper)[1].setValue(true)

    dialog.vm.$emit('remove', 1)
    await flushPromises()

    expect(store.selectedIds).toEqual([2])
  })

  it('drops a deleted device from the selection', async () => {
    vi.stubGlobal('confirm', () => true)
    vi.mocked(deleteDevice).mockResolvedValue(undefined as any)
    const wrapper = await mountPage()
    const store = useDevicesStore()
    await rowBoxes(wrapper)[0].setValue(true)
    await rowBoxes(wrapper)[1].setValue(true)

    await wrapper.findAll('.action-btn.danger')[0].trigger('click')
    await flushPromises()

    expect(deleteDevice).toHaveBeenCalledWith(1)
    expect(store.selectedIds).toEqual([2])
  })

  it('hides the checkboxes and disarms the button when the select column is switched off', async () => {
    const wrapper = await mountPage()
    const store = useDevicesStore()
    await rowBoxes(wrapper)[0].setValue(true)

    store.toggleColumn('select')
    await flushPromises()

    expect(rowBoxes(wrapper)).toHaveLength(0)
    expect(wrapper.find('[data-testid="select-all-devices"]').exists()).toBe(false)
    expect(bulkButton(wrapper).attributes('disabled')).toBeDefined()
  })
})
