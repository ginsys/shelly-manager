import { vi, describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDevicesStore } from '../devices'
import { listDevices } from '@/api/devices'
import type { Device } from '@/api/types'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn()
}))

function device(id: number, name: string, ip = `10.0.0.${id}`): Device {
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

const answer = (items: Device[]) =>
  ({ items, meta: { pagination: { total_pages: 1, has_next: false } } }) as any

// An answer the test finishes by hand, so it can land in any order.
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

// The one request a check makes: the whole list, with no paging.
const wholeListCalls = () => vi.mocked(listDevices).mock.calls.filter((c) => c[0]?.pageSize === 0)

// The page shows devices 1 and 2; 7 and 8 are ticked on another page.
async function storeWithSelection() {
  const store = useDevicesStore()
  vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
  await store.fetchDevices()
  store.toggleSelect(device(1, 'Alpha'))
  store.toggleSelect(device(7, 'Gone', '10.0.0.7'))
  store.toggleSelect(device(8, 'Kept', '10.0.0.8'))
  vi.mocked(listDevices).mockClear()
  return store
}

// The server still has 1, 2 and 8; 7 was deleted.
const fleetWithout7 = () => answer([device(1, 'Alpha'), device(2, 'Bravo'), device(8, 'Kept')])

describe('devices store, selection reconciled with the loaded list', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('copies a rename from the loaded row into the selection, with no extra request', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha renamed', '10.9.9.1'), device(2, 'Bravo')]))

    await store.fetchDevices()

    expect(store.selectedDevices.find((d) => d.id === 1)).toEqual({ id: 1, name: 'Alpha renamed', ip: '10.9.9.1' })
    expect(listDevices).toHaveBeenCalledTimes(1)
    expect(store.loading).toBe(false)
  })

  it('leaves a selected id that is not on the loaded page alone: it may be on another page', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))

    await store.fetchDevices()

    expect(store.selectedIds).toEqual([1, 7, 8])
    expect(wholeListCalls()).toHaveLength(0)
  })
})

describe('devices store, checkSelection', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('sends no request when every selected id is on the loaded page', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(1, 'Alpha'))
    vi.mocked(listDevices).mockClear()

    await store.checkSelection()

    expect(listDevices).not.toHaveBeenCalled()
    expect(store.checkingSelection).toBe(false)
    expect(store.removedFromSelection).toBe(0)
  })

  it('sends exactly one request however many ticked ids are off the page: the server counts every request', async () => {
    const store = await storeWithSelection()
    for (const id of [9, 10, 11, 12, 13]) store.toggleSelect(device(id, `Far ${id}`))
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(7, 'Gone'), device(8, 'Kept')]))

    await store.checkSelection()

    expect(listDevices).toHaveBeenCalledTimes(1)
    expect(wholeListCalls()).toHaveLength(1)
    expect(wholeListCalls()[0][0]).toEqual({ page: 1, pageSize: 0 })
  })

  it('drops a ticked id the whole list no longer holds, and counts it', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(fleetWithout7())

    await store.checkSelection()

    expect(store.selectedIds).toEqual([1, 8])
    expect(store.removedFromSelection).toBe(1)
  })

  it('keeps an id that is still listed and takes its fresh name and address', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(
      answer([device(1, 'Alpha'), device(8, 'Kept renamed', '10.8.8.8')])
    )

    await store.checkSelection()

    expect(store.selectedDevices.find((d) => d.id === 8)).toEqual({ id: 8, name: 'Kept renamed', ip: '10.8.8.8' })
  })

  it('keeps every id when the request fails, and reports no error', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockRejectedValueOnce(new Error('Network Error'))

    await store.checkSelection()

    expect(store.selectedIds).toEqual([1, 7, 8])
    expect(store.removedFromSelection).toBe(0)
    expect(store.error).toBeNull()
    expect(store.checkingSelection).toBe(false)
  })

  it('is checking until the answer is in', async () => {
    const store = await storeWithSelection()
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)

    const run = store.checkSelection()
    expect(store.checkingSelection).toBe(true)
    list.resolve(fleetWithout7())
    await run

    expect(store.checkingSelection).toBe(false)
  })

  it('does not put back an id that was unticked while the check was pending', async () => {
    const store = await storeWithSelection()
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)

    const run = store.checkSelection()
    store.deselect([8]) // the dialog's remove button does the same
    list.resolve(fleetWithout7())
    await run

    expect(store.selectedIds).toEqual([1])
  })

  it('does not put back an id when the selection was cleared while the check was pending', async () => {
    const store = await storeWithSelection()
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)

    const run = store.checkSelection()
    store.clearSelection()
    list.resolve(fleetWithout7())
    await run

    expect(store.selectedIds).toEqual([])
    expect(store.removedFromSelection).toBe(0)
  })

  it('does not count an id that was unticked before the answer arrived', async () => {
    const store = await storeWithSelection()
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)

    const run = store.checkSelection()
    store.deselect([7])
    list.resolve(fleetWithout7())
    await run

    expect(store.selectedIds).toEqual([1, 8])
    expect(store.removedFromSelection).toBe(0)
  })

  it('shows nothing from an older check once a newer one has started, even when its answer lands last', async () => {
    const store = await storeWithSelection()
    const older = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices)
      .mockReturnValueOnce(older.promise as any)
      .mockResolvedValueOnce(answer([device(1, 'Alpha'), device(8, 'Kept')]))

    const first = store.checkSelection()
    const second = store.checkSelection()
    await second
    expect(store.selectedIds).toEqual([1, 8])
    expect(store.removedFromSelection).toBe(1)

    // The older answer lists 7 and a stale name for 8: it must change nothing.
    older.resolve(answer([device(1, 'Alpha'), device(7, 'Gone'), device(8, 'Stale name')]))
    await first

    expect(store.selectedDevices.find((d) => d.id === 8)?.name).toBe('Kept')
    expect(store.selectedIds).toEqual([1, 8])
    expect(store.removedFromSelection).toBe(1)
  })

  it('stays checking until the newest check answers, not the first to finish', async () => {
    const store = await storeWithSelection()
    const olderList = deferred<ReturnType<typeof answer>>()
    const newerList = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices)
      .mockReturnValueOnce(olderList.promise as any)
      .mockReturnValueOnce(newerList.promise as any)
    const older = store.checkSelection()
    const newer = store.checkSelection()

    olderList.resolve(fleetWithout7())
    await older
    expect(store.checkingSelection).toBe(true)

    newerList.resolve(fleetWithout7())
    await newer
    expect(store.checkingSelection).toBe(false)
  })

  it('clears the previous count when a check starts', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(fleetWithout7())
    await store.checkSelection()
    expect(store.removedFromSelection).toBe(1)

    const pending = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(pending.promise as any)
    const run = store.checkSelection()
    expect(store.removedFromSelection).toBe(0)
    pending.resolve(fleetWithout7())
    await run
  })

  it('clearSelection resets the count', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(fleetWithout7())
    await store.checkSelection()
    expect(store.removedFromSelection).toBe(1)

    store.clearSelection()

    expect(store.removedFromSelection).toBe(0)
  })

  it('a load sends no whole-list request and is not held by a running check', async () => {
    const store = await storeWithSelection()
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)
    const run = store.checkSelection()
    vi.mocked(listDevices).mockClear()

    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
    await store.fetchDevices()

    expect(wholeListCalls()).toHaveLength(0)
    expect(store.loading).toBe(false)
    expect(store.checkingSelection).toBe(true)
    list.resolve(fleetWithout7())
    await run
  })

  it('does not trust the old rows while a load is running: a device deleted meanwhile is still dropped', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(1, 'Alpha'))
    // Device 1 is deleted elsewhere; a refresh is on its way and the dialog is opened before it lands.
    const refresh = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(refresh.promise as any)
    const loading = store.fetchDevices()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))

    const check = store.checkSelection()
    refresh.resolve(answer([device(2, 'Bravo')]))
    await Promise.all([loading, check])

    expect(wholeListCalls()).toHaveLength(1)
    expect(store.selectedIds).toEqual([])
    expect(store.removedFromSelection).toBe(1)
  })

  it('still drops a deleted device when the refresh that was running fails and the old rows stay', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(1, 'Alpha'))
    const refresh = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(refresh.promise as any)
    const loading = store.fetchDevices()
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)
    const check = store.checkSelection()

    refresh.reject(new Error('boom')) // the old rows, which still list device 1, stay
    await loading
    list.resolve(answer([device(2, 'Bravo')]))
    await check

    expect(store.items.map((d) => d.id)).toEqual([1, 2])
    expect(store.selectedIds).toEqual([])
    expect(store.removedFromSelection).toBe(1)
  })

  it('does not let an answer to a check started before a load overwrite what the load put on the page', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(7, 'Old', '10.0.0.7')) // ticked on another page
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)
    const check = store.checkSelection()

    // A page change brings device 7 onto the page with a new name, then the older answer lands.
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(7, 'New', '10.7.7.7')]))
    await store.fetchDevices()
    expect(store.selectedDevices).toEqual([{ id: 7, name: 'New', ip: '10.7.7.7' }])
    list.resolve(answer([device(2, 'Bravo'), device(7, 'Old', '10.0.0.7')]))
    await check

    expect(store.selectedDevices).toEqual([{ id: 7, name: 'New', ip: '10.7.7.7' }])
  })

  it('does not drop a device that a load has since put on the page, whatever the older answer said', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(7, 'Seven', '10.0.0.7'))
    const list = deferred<ReturnType<typeof answer>>()
    vi.mocked(listDevices).mockReturnValueOnce(list.promise as any)
    const check = store.checkSelection()

    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(7, 'Seven', '10.0.0.7')]))
    await store.fetchDevices()
    list.resolve(answer([device(2, 'Bravo')])) // the older answer does not list 7
    await check

    expect(store.selectedIds).toEqual([7])
    expect(store.removedFromSelection).toBe(0)
  })
})
