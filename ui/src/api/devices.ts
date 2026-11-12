import api from './client'
import type { APIResponse, Device, Metadata } from './types'

export interface ListDevicesParams {
  page?: number
  pageSize?: number
}

export interface ListDevicesResult {
  items: Device[]
  meta?: Metadata
}

export async function listDevices(params: ListDevicesParams = {}): Promise<ListDevicesResult> {
  const { page = 1, pageSize = 25 } = params
  const res = await api.get<APIResponse<{ devices: Device[] }>>('/devices', {
    params: { page, page_size: pageSize },
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load devices'
    throw new Error(msg)
  }
  return {
    items: res.data.data?.devices || [],
    meta: res.data.meta,
  }
}

export async function getDevice(id: number | string, signal?: AbortSignal): Promise<Device> {
  const res = await api.get<APIResponse<Device>>(`/devices/${id}`, { signal })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Device not found'
    throw new Error(msg)
  }
  return res.data.data
}

export async function createDevice(data: Partial<Device>): Promise<Device> {
  const res = await api.post<APIResponse<Device>>('/devices', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create device'
    throw new Error(msg)
  }
  return res.data.data
}

// Send the updated_at that the device was read with. The server refuses the save with a 409 when
// the device changed after that read, so a save never puts old data over a newer one.
export async function updateDevice(id: number | string, data: Partial<Device>): Promise<Device> {
  try {
    const res = await api.put<APIResponse<Device>>(`/devices/${id}`, data)
    if (!res.data.success || !res.data.data) {
      const msg = res.data.error?.message || 'Failed to update device'
      throw new Error(msg)
    }
    return res.data.data
  } catch (e: any) {
    // A 409 carries the server's reason in the body; show it rather than the status line.
    const message = e?.response?.status === 409 ? e?.response?.data?.error?.message : undefined
    if (message) throw new Error(message)
    throw e
  }
}

// The login the manager uses to reach the device. An empty username turns device auth off. A
// password equal to the mask the device routes show keeps the stored one.
export interface DeviceLogin {
  username: string
  password: string
}

export async function updateDeviceAuth(id: number | string, login: DeviceLogin): Promise<void> {
  try {
    const res = await api.put<APIResponse<{ status: string }>>(`/devices/${id}/config/auth`, login)
    if (!res.data.success) {
      throw new Error(res.data.error?.message || 'Failed to save the device login')
    }
  } catch (e: any) {
    // A 400 or 404 carries the server's reason in the body; show it rather than the status line.
    const message = e?.response?.data?.error?.message
    if (message) throw new Error(message)
    throw e
  }
}

export async function deleteDevice(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/devices/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete device'
    throw new Error(msg)
  }
}

export interface DeviceControlRequest {
  action: 'on' | 'off' | 'restart' | 'toggle' | 'reboot'
  params?: Record<string, any>
}

export async function controlDevice(id: number | string, request: DeviceControlRequest, signal?: AbortSignal): Promise<any> {
  const res = await api.post<APIResponse<any>>(`/devices/${id}/control`, request, { signal })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to control device'
    throw new Error(msg)
  }
  return res.data.data
}

export interface DeviceStatus {
  online: boolean
  lastSeen?: string
  uptime?: number
  temperature?: number
  cloud?: { enabled: boolean; connected: boolean }
  wifi?: { ssid: string; rssi: number }
  mqtt?: { connected: boolean }
}

export async function getDeviceStatus(id: number | string, signal?: AbortSignal): Promise<DeviceStatus> {
  const res = await api.get<APIResponse<DeviceStatus>>(`/devices/${id}/status`, { signal })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get device status'
    throw new Error(msg)
  }
  return res.data.data
}

export interface DeviceEnergy {
  power?: number
  voltage?: number
  current?: number
  total?: number
  totalReturned?: number
}

export async function getDeviceEnergy(id: number | string, channel: number = 0, signal?: AbortSignal): Promise<DeviceEnergy> {
  const res = await api.get<APIResponse<DeviceEnergy>>(`/devices/${id}/energy`, {
    params: { channel },
    signal,
  })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get energy metrics'
    throw new Error(msg)
  }
  return res.data.data
}

export function isAbortError(err: unknown): boolean {
  if (err instanceof DOMException && err.name === 'AbortError') return true
  if (err instanceof Error && err.name === 'CanceledError') return true
  return false
}
