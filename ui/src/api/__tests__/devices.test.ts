import { describe, it, expect, vi, beforeEach } from 'vitest'
import api from '../client'

vi.mock('../client', () => {
  const axiosLike = { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn(), interceptors: { request: { use: vi.fn() } } }
  return { default: axiosLike, __esModule: true }
})
const mockedApi = api as unknown as { get: any; post: any; put: any; delete: any }

describe('devices api (extended)', () => {
  beforeEach(() => vi.resetAllMocks())

  it('creates a device', async () => {
    const { createDevice } = await import('../devices')
    mockedApi.post.mockResolvedValueOnce({ data: { success: true, data: { id: 1, name: 'Lamp', ip: '192.0.2.10', mac: '00:11:22:33:44:55', type: 'shelly1', firmware: '', status: 'online', last_seen: new Date().toISOString() }, timestamp: new Date().toISOString() } })
    const d = await createDevice({ name: 'Lamp', ip: '192.0.2.10' })
    expect(d.name).toBe('Lamp')
  })

  it('updates a device', async () => {
    const { updateDevice } = await import('../devices')
    mockedApi.put.mockResolvedValueOnce({ data: { success: true, data: { id: 1, name: 'Lamp-2' }, timestamp: new Date().toISOString() } })
    const d = await updateDevice(1, { name: 'Lamp-2' } as any)
    expect(d.name).toBe('Lamp-2')
  })

  it('deletes a device', async () => {
    const { deleteDevice } = await import('../devices')
    mockedApi.delete.mockResolvedValueOnce({ data: { success: true, timestamp: new Date().toISOString() } })
    await expect(deleteDevice(1)).resolves.toBeUndefined()
  })

  it('controls a device', async () => {
    const { controlDevice } = await import('../devices')
    mockedApi.post.mockResolvedValueOnce({ data: { success: true, data: { accepted: true }, timestamp: new Date().toISOString() } })
    const res = await controlDevice(1, 'restart')
    expect(res.accepted).toBe(true)
  })

  it('gets device status and energy', async () => {
    const { getDeviceStatus, getDeviceEnergy } = await import('../devices')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { status: 'online', last_seen: new Date().toISOString() }, timestamp: new Date().toISOString() } })
    const st = await getDeviceStatus(1)
    expect(st.status).toBe('online')

    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { power_w: 12.3, energy_wh: 100 }, timestamp: new Date().toISOString() } })
    const en = await getDeviceEnergy(1)
    expect(en.power_w).toBe(12.3)
  })
})

