import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDevicesStore } from '../devices'
import { listDevices } from '@/api/devices'
import type { Device } from '@/api/types'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn()
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

const a = device(1, 'Alpha', '10.0.0.1')
const b = device(2, 'Bravo', '10.0.0.2')
const c = device(3, 'Charlie', '10.0.0.3')

// An in-memory storage of our own: newer Node versions expose an experimental global
// `localStorage` that has no file behind it and shadows the one jsdom provides.
function memoryStorage() {
  const data = new Map<string, string>()
  return {
    getItem: (k: string) => (data.has(k) ? data.get(k)! : null),
    setItem: (k: string, v: string) => void data.set(k, String(v)),
    removeItem: (k: string) => void data.delete(k),
    clear: () => data.clear()
  }
}

describe('devices store selection', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('localStorage', memoryStorage())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('starts with nothing selected', () => {
    const store = useDevicesStore()

    expect(store.selectedCount).toBe(0)
    expect(store.selectedIds).toEqual([])
    expect(store.selectedDevices).toEqual([])
  })

  it('toggles a device on and off by id', () => {
    const store = useDevicesStore()

    store.toggleSelect(a)
    expect(store.selectedIds).toEqual([1])
    expect(store.isSelected(1)).toBe(true)

    store.toggleSelect(a)
    expect(store.selectedIds).toEqual([])
    expect(store.isSelected(1)).toBe(false)
  })

  it('keeps the name and address from the moment of the tick', () => {
    const store = useDevicesStore()

    store.toggleSelect(a)
    store.toggleSelect(b)

    expect(store.selectedDevices).toEqual([
      { id: 1, name: 'Alpha', ip: '10.0.0.1' },
      { id: 2, name: 'Bravo', ip: '10.0.0.2' }
    ])
  })

  it('select all adds the given devices once and leaves the others alone', () => {
    const store = useDevicesStore()
    store.toggleSelect(a)

    store.selectAll([a, b])
    store.selectAll([b])

    expect(store.selectedIds).toEqual([1, 2])
    expect(store.selectedCount).toBe(2)
  })

  it('deselect removes only the named ids', () => {
    const store = useDevicesStore()
    store.selectAll([a, b, c])

    store.deselect([1, 3])

    expect(store.selectedIds).toEqual([2])
  })

  it('clears the selection', () => {
    const store = useDevicesStore()
    store.selectAll([a, b])

    store.clearSelection()

    expect(store.selectedCount).toBe(0)
  })

  it('does not change what is selected when the list is sorted or searched', () => {
    const store = useDevicesStore()
    store.items = [a, b, c]
    store.toggleSelect(b)

    store.toggleSort('name')
    store.toggleSort('name')
    store.setSearch('charlie')

    expect(store.sortedItems.map((d) => d.id)).toEqual([3])
    expect(store.selectedIds).toEqual([2])
  })

  it('keeps a device selected on another page, with its name, after the page changes', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValue({
      items: [c],
      meta: { pagination: { total_pages: 2, has_next: false } }
    } as any)
    store.items = [a, b]
    store.toggleSelect(a)

    store.setPage(2)
    await store.fetchDevices()

    expect(store.items.map((d) => d.id)).toEqual([3])
    expect(store.selectedDevices).toEqual([{ id: 1, name: 'Alpha', ip: '10.0.0.1' }])
  })

  it('hiding the select column clears the selection', () => {
    const store = useDevicesStore()
    store.selectAll([a, b])

    store.toggleColumn('select')

    expect(store.visibleColumns.select).toBe(false)
    expect(store.selectedCount).toBe(0)
  })

  it('showing the select column again selects nothing', () => {
    const store = useDevicesStore()
    store.toggleColumn('select')

    store.toggleColumn('select')

    expect(store.visibleColumns.select).toBe(true)
    expect(store.selectedCount).toBe(0)
  })

  it('shows the select column by default, also for a saved set that predates it', () => {
    localStorage.setItem('devices.visibleColumns', JSON.stringify({ name: true, ip: false }))
    const store = useDevicesStore()

    store.loadPreferences()

    expect(store.visibleColumns.select).toBe(true)
    expect(store.visibleColumns.ip).toBe(false)
  })
})
