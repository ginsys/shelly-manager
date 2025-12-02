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

export async function getDevice(id: number | string): Promise<Device> {
  const res = await api.get<APIResponse<Device>>(`/devices/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Device not found'
    throw new Error(msg)
  }
  return res.data.data
}

// Create a device
export async function createDevice(payload: Partial<Device> & { ip?: string; mac?: string; name?: string; type?: string }): Promise<Device> {
  const res = await api.post<APIResponse<Device>>('/devices', payload)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create device')
  }
  return res.data.data
}

// Update a device
export async function updateDevice(id: number | string, payload: Partial<Device>): Promise<Device> {
  const res = await api.put<APIResponse<Device>>(`/devices/${id}`, payload)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to update device')
  }
  return res.data.data
}

// Delete a device
export async function deleteDevice(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<unknown>>(`/devices/${id}`)
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to delete device')
  }
}

// Control device (on/off/restart)
export type DeviceAction = 'on' | 'off' | 'restart'
export async function controlDevice(id: number | string, action: DeviceAction): Promise<{ accepted: boolean }> {
  const res = await api.post<APIResponse<{ accepted: boolean }>>(`/devices/${id}/control`, { action })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to control device')
  }
  return res.data.data
}

// Device status and energy types
export interface DeviceStatus { status: string; last_seen?: string }
export interface DeviceEnergyMetrics { power_w?: number; energy_wh?: number; voltage_v?: number; current_a?: number; timestamp?: string }

export async function getDeviceStatus(id: number | string): Promise<DeviceStatus> {
  const res = await api.get<APIResponse<DeviceStatus>>(`/devices/${id}/status`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to get device status')
  }
  return res.data.data
}

export async function getDeviceEnergy(id: number | string): Promise<DeviceEnergyMetrics> {
  const res = await api.get<APIResponse<DeviceEnergyMetrics>>(`/devices/${id}/energy`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to get energy metrics')
  }
  return res.data.data
}

// Device capabilities
export type DeviceCapabilities = Record<string, boolean>
export async function getDeviceCapabilities(id: number | string): Promise<DeviceCapabilities> {
  const res = await api.get<APIResponse<DeviceCapabilities>>(`/devices/${id}/capabilities`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to get device capabilities')
  }
  return res.data.data
}
