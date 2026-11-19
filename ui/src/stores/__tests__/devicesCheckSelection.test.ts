import { vi, describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDevicesStore } from '../devices'
import { listDevices, getDevice } from '@/api/devices'
import type { Device } from '@/api/types'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn(),
  getDevice: vi.fn()
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

const notFound = () => Object.assign(new Error('Request failed with status code 404'), { response: { status: 404 } })
const serverError = () => Object.assign(new Error('Request failed with status code 500'), { response: { status: 500 } })

// A probe the test answers by hand.
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

// The page shows devices 1 and 2; 7 and 8 are ticked on another page.
async function storeWithSelection() {
  const store = useDevicesStore()
  vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
  await store.fetchDevices()
  store.toggleSelect(device(1, 'Alpha'))
  store.toggleSelect(device(7, 'Gone', '10.0.0.7'))
  store.toggleSelect(device(8, 'Kept', '10.0.0.8'))
  return store
}

describe('devices store, selection reconciled with the loaded list', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('copies a rename from the loaded row into the selection, with no request', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha renamed', '10.9.9.1'), device(2, 'Bravo')]))

    await store.fetchDevices()

    expect(store.selectedDevices.find((d) => d.id === 1)).toEqual({ id: 1, name: 'Alpha renamed', ip: '10.9.9.1' })
    expect(getDevice).not.toHaveBeenCalled()
    expect(store.loading).toBe(false)
  })

  it('leaves a selected id that is not on the loaded page alone: it may be on another page', async () => {
    const store = await storeWithSelection()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))

    await store.fetchDevices()

    expect(store.selectedIds).toEqual([1, 7, 8])
    expect(getDevice).not.toHaveBeenCalled()
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

    await store.checkSelection()

    expect(getDevice).not.toHaveBeenCalled()
    expect(store.checkingSelection).toBe(false)
    expect(store.removedFromSelection).toBe(0)
  })

  it('asks only for the ids that are not on the loaded page', async () => {
    const store = await storeWithSelection()
    vi.mocked(getDevice).mockResolvedValue(device(8, 'Kept'))
    vi.mocked(getDevice).mockImplementation(async (id) => {
      if (id === 7) throw notFound()
      return device(8, 'Kept')
    })

    await store.checkSelection()

    expect(vi.mocked(getDevice).mock.calls.map((c) => c[0]).sort()).toEqual([7, 8])
  })

  it('drops an id the server answers 404 for and counts it', async () => {
    const store = await storeWithSelection()
    vi.mocked(getDevice).mockImplementation(async (id) => {
      if (id === 7) throw notFound()
      return device(8, 'Kept')
    })

    await store.checkSelection()

    expect(store.selectedIds).toEqual([1, 8])
    expect(store.removedFromSelection).toBe(1)
  })

  it('keeps an id that exists and takes its fresh name and address', async () => {
    const store = await storeWithSelection()
    vi.mocked(getDevice).mockImplementation(async (id) => {
      if (id === 7) throw notFound()
      return device(8, 'Kept renamed', '10.8.8.8')
    })

    await store.checkSelection()

    expect(store.selectedDevices.find((d) => d.id === 8)).toEqual({ id: 8, name: 'Kept renamed', ip: '10.8.8.8' })
  })

  it('keeps an id whose check failed for any reason other than a 404', async () => {
    const store = await storeWithSelection()
    vi.mocked(getDevice).mockImplementation(async (id) => {
      if (id === 7) throw serverError()
      throw new Error('Network Error')
    })

    await store.checkSelection()

    expect(store.selectedIds).toEqual([1, 7, 8])
    expect(store.removedFromSelection).toBe(0)
    expect(store.error).toBeNull()
  })

  it('is checking until the answers are in', async () => {
    const store = await storeWithSelection()
    const gone = deferred<Device>()
    vi.mocked(getDevice).mockImplementation((id) => (id === 7 ? gone.promise : Promise.resolve(device(8, 'Kept'))))

    const run = store.checkSelection()
    expect(store.checkingSelection).toBe(true)
    gone.reject(notFound())
    await run

    expect(store.checkingSelection).toBe(false)
  })

  it('does not put back an id that was unticked while its check was pending', async () => {
    const store = await storeWithSelection()
    const kept = deferred<Device>()
    vi.mocked(getDevice).mockImplementation((id) => {
      if (id === 7) return Promise.reject(notFound())
      return kept.promise
    })

    const run = store.checkSelection()
    store.deselect([8]) // the dialog's remove button does the same
    kept.resolve(device(8, 'Kept'))
    await run

    expect(store.selectedIds).toEqual([1])
  })

  it('does not put back an id when the selection was cleared while its check was pending', async () => {
    const store = await storeWithSelection()
    const kept = deferred<Device>()
    vi.mocked(getDevice).mockImplementation((id) => {
      if (id === 7) return Promise.reject(notFound())
      return kept.promise
    })

    const run = store.checkSelection()
    store.clearSelection()
    kept.resolve(device(8, 'Kept'))
    await run

    expect(store.selectedIds).toEqual([])
    expect(store.removedFromSelection).toBe(0)
  })

  it('does not count an id that was unticked before its 404 arrived', async () => {
    const store = await storeWithSelection()
    const gone = deferred<Device>()
    vi.mocked(getDevice).mockImplementation((id) => (id === 7 ? gone.promise : Promise.resolve(device(8, 'Kept'))))

    const run = store.checkSelection()
    store.deselect([7])
    gone.reject(notFound())
    await run

    expect(store.selectedIds).toEqual([1, 8])
    expect(store.removedFromSelection).toBe(0)
  })

  it('shows nothing from an older check once a newer one has started', async () => {
    const store = await storeWithSelection()
    const first7 = deferred<Device>()
    const first8 = deferred<Device>()
    vi.mocked(getDevice)
      .mockImplementationOnce(() => first7.promise)
      .mockImplementationOnce(() => first8.promise)
      .mockImplementation(async (id) => {
        if (id === 7) throw notFound()
        return device(8, 'Kept')
      })

    const older = store.checkSelection()
    const newer = store.checkSelection()
    await newer
    expect(store.removedFromSelection).toBe(1)

    first7.resolve(device(7, 'Gone'))
    first8.resolve(device(8, 'Stale name'))
    await older

    expect(store.selectedDevices.find((d) => d.id === 8)?.name).toBe('Kept')
    expect(store.removedFromSelection).toBe(1)
  })

  it('stays checking until the newest check answers, not the first to finish', async () => {
    const store = await storeWithSelection()
    const slow7 = deferred<Device>()
    vi.mocked(getDevice)
      .mockImplementationOnce(() => slow7.promise) // older check, id 7
      .mockImplementationOnce(() => Promise.resolve(device(8, 'Kept'))) // older check, id 8
    const older = store.checkSelection()

    const slowNewer = deferred<Device>()
    vi.mocked(getDevice)
      .mockImplementationOnce(() => slowNewer.promise) // newer check, id 7
      .mockImplementationOnce(() => Promise.resolve(device(8, 'Kept')))
    const newer = store.checkSelection()

    slow7.reject(notFound())
    await older
    expect(store.checkingSelection).toBe(true)

    slowNewer.reject(notFound())
    await newer
    expect(store.checkingSelection).toBe(false)
  })

  it('clears the previous count when a check starts', async () => {
    const store = await storeWithSelection()
    vi.mocked(getDevice).mockImplementation(async (id) => {
      if (id === 7) throw notFound()
      return device(8, 'Kept')
    })
    await store.checkSelection()
    expect(store.removedFromSelection).toBe(1)

    const pending = deferred<Device>()
    vi.mocked(getDevice).mockImplementation(() => pending.promise)
    const run = store.checkSelection()
    expect(store.removedFromSelection).toBe(0)
    pending.resolve(device(8, 'Kept'))
    await run
  })

  it('clearSelection resets the count', async () => {
    const store = await storeWithSelection()
    vi.mocked(getDevice).mockImplementation(async (id) => {
      if (id === 7) throw notFound()
      return device(8, 'Kept')
    })
    await store.checkSelection()
    expect(store.removedFromSelection).toBe(1)

    store.clearSelection()

    expect(store.removedFromSelection).toBe(0)
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
    vi.mocked(getDevice).mockRejectedValue(notFound())

    const check = store.checkSelection()
    refresh.resolve(answer([device(2, 'Bravo')]))
    await Promise.all([loading, check])

    expect(getDevice).toHaveBeenCalledWith(1)
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
    const probe = deferred<Device>()
    vi.mocked(getDevice).mockReturnValue(probe.promise)
    const check = store.checkSelection()

    refresh.reject(new Error('boom')) // the old rows, which still list device 1, stay
    await loading
    probe.reject(notFound())
    await check

    expect(store.items.map((d) => d.id)).toEqual([1, 2])
    expect(store.selectedIds).toEqual([])
    expect(store.removedFromSelection).toBe(1)
  })

  it('does not let a probe started before a load overwrite what the load put on the page', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(7, 'Old', '10.0.0.7')) // ticked on another page
    const probe = deferred<Device>()
    vi.mocked(getDevice).mockReturnValue(probe.promise)
    const check = store.checkSelection()

    // A page change brings device 7 onto the page with a new name, then the older probe answers.
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(7, 'New', '10.7.7.7')]))
    await store.fetchDevices()
    expect(store.selectedDevices).toEqual([{ id: 7, name: 'New', ip: '10.7.7.7' }])
    probe.resolve(device(7, 'Old', '10.0.0.7'))
    await check

    expect(store.selectedDevices).toEqual([{ id: 7, name: 'New', ip: '10.7.7.7' }])
  })

  it('does not drop a device that a load has since put on the page, whatever the older probe said', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(2, 'Bravo')]))
    await store.fetchDevices()
    store.toggleSelect(device(7, 'Seven', '10.0.0.7'))
    const probe = deferred<Device>()
    vi.mocked(getDevice).mockReturnValue(probe.promise)
    const check = store.checkSelection()

    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(7, 'Seven', '10.0.0.7')]))
    await store.fetchDevices()
    probe.reject(notFound())
    await check

    expect(store.selectedIds).toEqual([7])
    expect(store.removedFromSelection).toBe(0)
  })

  it('a load sends no probe and is not held by a running check', async () => {
    const store = await storeWithSelection()
    const gone = deferred<Device>()
    vi.mocked(getDevice).mockImplementation(() => gone.promise)
    const run = store.checkSelection()
    vi.mocked(getDevice).mockClear()

    vi.mocked(listDevices).mockResolvedValueOnce(answer([device(1, 'Alpha'), device(2, 'Bravo')]))
    await store.fetchDevices()

    expect(getDevice).not.toHaveBeenCalled()
    expect(store.loading).toBe(false)
    expect(store.checkingSelection).toBe(true)
    gone.reject(notFound())
    await run
  })
})
