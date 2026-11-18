import { vi, describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDevicesStore } from '../devices'
import { listDevices } from '@/api/devices'
import type { Device } from '@/api/types'

vi.mock('@/api/devices', () => ({
  listDevices: vi.fn()
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

// A load that the test finishes by hand, so the answers can arrive in either order.
function deferred() {
  let resolve!: (items: Device[]) => void
  let reject!: (error: Error) => void
  const promise = new Promise((res, rej) => {
    resolve = (items) => res({ items, meta: { pagination: { total_pages: 1, has_next: false } } })
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('devices store, overlapping loads', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('drops an older answer that arrives after a newer one', async () => {
    const store = useDevicesStore()
    const older = deferred()
    const newer = deferred()
    vi.mocked(listDevices)
      .mockReturnValueOnce(older.promise as any)
      .mockReturnValueOnce(newer.promise as any)

    const first = store.fetchDevices()
    const second = store.fetchDevices()
    newer.resolve([device(2, 'Newer')])
    await second
    older.resolve([device(1, 'Older')])
    await first

    expect(store.items.map((d) => d.name)).toEqual(['Newer'])
    expect(store.loading).toBe(false)
  })

  it('stays loading until the newest load answers, not the first to finish', async () => {
    const store = useDevicesStore()
    const older = deferred()
    const newer = deferred()
    vi.mocked(listDevices)
      .mockReturnValueOnce(older.promise as any)
      .mockReturnValueOnce(newer.promise as any)

    const first = store.fetchDevices()
    const second = store.fetchDevices()
    older.resolve([device(1, 'Older')])
    await first
    expect(store.loading).toBe(true)

    newer.resolve([device(2, 'Newer')])
    await second
    expect(store.loading).toBe(false)
    expect(store.items.map((d) => d.name)).toEqual(['Newer'])
  })

  it('does not let an older failure replace a newer answer', async () => {
    const store = useDevicesStore()
    const older = deferred()
    const newer = deferred()
    vi.mocked(listDevices)
      .mockReturnValueOnce(older.promise as any)
      .mockReturnValueOnce(newer.promise as any)

    const first = store.fetchDevices()
    const second = store.fetchDevices()
    newer.resolve([device(2, 'Newer')])
    await second
    older.reject(new Error('boom'))
    await first

    expect(store.error).toBeNull()
    expect(store.items.map((d) => d.name)).toEqual(['Newer'])
  })

  it('still reports a failure of the only load', async () => {
    const store = useDevicesStore()
    vi.mocked(listDevices).mockRejectedValueOnce(new Error('boom'))

    await store.fetchDevices()

    expect(store.error).toBe('boom')
    expect(store.loading).toBe(false)
  })
})
