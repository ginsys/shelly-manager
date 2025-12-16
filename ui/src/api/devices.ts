import api from './client'
import type {
  APIResponse,
  Device,
  Metadata,
  CreateDeviceRequest,
  UpdateDeviceRequest,
  ControlDeviceRequest,
  ControlDeviceResponse,
  DeviceStatus,
  DeviceEnergy,
} from './types'

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

export async function createDevice(data: CreateDeviceRequest): Promise<Device> {
  const res = await api.post<APIResponse<Device>>('/devices', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create device'
    throw new Error(msg)
  }
  return res.data.data
}

export async function updateDevice(id: number | string, data: UpdateDeviceRequest): Promise<Device> {
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

export async function controlDevice(
  id: number | string,
  request: ControlDeviceRequest,
): Promise<ControlDeviceResponse> {
  const res = await api.post<APIResponse<ControlDeviceResponse>>(`/devices/${id}/control`, request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to control device'
    throw new Error(msg)
  }
  return res.data.data
}

export async function getDeviceStatus(id: number | string): Promise<DeviceStatus> {
  const res = await api.get<APIResponse<DeviceStatus>>(`/devices/${id}/status`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get device status'
    throw new Error(msg)
  }
  return res.data.data
}

export async function getDeviceEnergy(id: number | string, channel = 0): Promise<DeviceEnergy> {
  const res = await api.get<APIResponse<DeviceEnergy>>(`/devices/${id}/energy`, {
    params: { channel },
  })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get device energy data'
    throw new Error(msg)
  }
  return res.data.data
}
