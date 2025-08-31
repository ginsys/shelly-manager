import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client used by devices API
vi.mock('./client', () => {
  return {
    default: {
      get: vi.fn(),
    },
  }
})

import api from './client'
import { listDevices, getDevice } from './devices'

describe('devices api', () => {
  beforeEach(() => {
    (api.get as any).mockReset()
  })

  it('listDevices returns items and meta', async () => {
    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: { devices: [{ id: 1, ip: '1.2.3.4', mac: 'AA', type: 'plug', name: 'Dev', firmware: '', status: 'online', last_seen: new Date().toISOString() }] },
        meta: { pagination: { page: 1, page_size: 25, total_pages: 1, has_next: false, has_previous: false }, version: 'v1' },
        timestamp: new Date().toISOString(),
      },
    })

    const res = await listDevices({ page: 1, pageSize: 25 })
    expect(res.items.length).toBe(1)
    expect(res.meta?.pagination?.page).toBe(1)
    expect(api.get).toHaveBeenCalledWith('/devices', { params: { page: 1, page_size: 25 } })
  })

  it('getDevice returns a device', async () => {
    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: { id: 2, ip: '2.2.2.2', mac: 'BB', type: 'switch', name: 'Dev2', firmware: '', status: 'offline', last_seen: new Date().toISOString() },
        meta: { version: 'v1' },
        timestamp: new Date().toISOString(),
      },
    })

    const d = await getDevice(2)
    expect(d.id).toBe(2)
    expect(api.get).toHaveBeenCalledWith('/devices/2')
  })
})

