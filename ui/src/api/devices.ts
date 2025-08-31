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
