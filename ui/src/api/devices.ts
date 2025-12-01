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

export async function createDevice(data: Partial<Device>): Promise<Device> {
  const res = await api.post<APIResponse<Device>>('/devices', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create device'
    throw new Error(msg)
  }
  return res.data.data
}

export async function updateDevice(id: number | string, data: Partial<Device>): Promise<Device> {
  const res = await api.put<APIResponse<Device>>(`/devices/${id}`, data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update device'
    throw new Error(msg)
  }
  return res.data.data
}

export async function deleteDevice(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/devices/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete device'
    throw new Error(msg)
  }
}

export interface DeviceControlRequest {
  action: 'on' | 'off' | 'restart' | 'toggle'
}

export async function controlDevice(id: number | string, action: string): Promise<void> {
  const res = await api.post<APIResponse<void>>(`/devices/${id}/control`, { action })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to control device'
    throw new Error(msg)
  }
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

export async function getDeviceStatus(id: number | string): Promise<DeviceStatus> {
  const res = await api.get<APIResponse<DeviceStatus>>(`/devices/${id}/status`)
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

export async function getDeviceEnergy(id: number | string): Promise<DeviceEnergy> {
  const res = await api.get<APIResponse<DeviceEnergy>>(`/devices/${id}/energy`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get energy metrics'
    throw new Error(msg)
  }
  return res.data.data
}
