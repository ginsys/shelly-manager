import { vi, describe, it, expect, beforeEach } from 'vitest'
import {
  listDevices,
  getDevice,
  createDevice,
  updateDevice,
  deleteDevice,
  controlDevice,
  getDeviceStatus,
  getDeviceEnergy,
} from '../devices'
import api from '../client'
import type { Device, DeviceStatus, DeviceEnergy } from '../types'

// Mock the API client
vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

const mockApi = vi.mocked(api)

describe('Devices API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  const mockDevice: Device = {
    id: 1,
    ip: '192.168.1.100',
    mac: 'AA:BB:CC:DD:EE:FF',
    type: 'SHSW-1',
    name: 'Test Device',
    firmware: '1.0.0',
    status: 'online',
    last_seen: '2023-01-01T00:00:00Z',
    created_at: '2023-01-01T00:00:00Z',
    updated_at: '2023-01-01T00:00:00Z',
  }

  describe('listDevices', () => {
    it('should fetch devices successfully', async () => {
      const mockDevices = [mockDevice]

      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: { devices: mockDevices },
          meta: {
            pagination: { page: 1, page_size: 25, total_pages: 1, has_next: false, has_previous: false },
          },
        },
      })

      const result = await listDevices({ page: 1, pageSize: 25 })

      expect(mockApi.get).toHaveBeenCalledWith('/devices', {
        params: { page: 1, page_size: 25 },
      })
      expect(result.items).toEqual(mockDevices)
      expect(result.meta).toBeDefined()
    })

    it('should handle API error', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Failed to load devices' },
        },
      })

      await expect(listDevices()).rejects.toThrow('Failed to load devices')
    })
  })

  describe('getDevice', () => {
    it('should fetch a single device successfully', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: mockDevice,
        },
      })

      const result = await getDevice(1)

      expect(mockApi.get).toHaveBeenCalledWith('/devices/1')
      expect(result).toEqual(mockDevice)
    })

    it('should handle device not found', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device not found' },
        },
      })

      await expect(getDevice(999)).rejects.toThrow('Device not found')
    })

    it('should accept string ID', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: mockDevice,
        },
      })

      await getDevice('1')

      expect(mockApi.get).toHaveBeenCalledWith('/devices/1')
    })
  })

  describe('createDevice', () => {
    it('should create a device successfully', async () => {
      const createRequest = {
        ip: '192.168.1.100',
        mac: 'AA:BB:CC:DD:EE:FF',
        name: 'New Device',
        type: 'SHSW-1',
      }

      mockApi.post.mockResolvedValue({
        data: {
          success: true,
          data: mockDevice,
        },
      })

      const result = await createDevice(createRequest)

      expect(mockApi.post).toHaveBeenCalledWith('/devices', createRequest)
      expect(result).toEqual(mockDevice)
    })

    it('should handle create error', async () => {
      const createRequest = {
        ip: '192.168.1.100',
        mac: 'AA:BB:CC:DD:EE:FF',
      }

      mockApi.post.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device with MAC already exists' },
        },
      })

      await expect(createDevice(createRequest)).rejects.toThrow('Device with MAC already exists')
    })
  })

  describe('updateDevice', () => {
    it('should update a device successfully', async () => {
      const updateRequest = {
        name: 'Updated Name',
        ip: '192.168.1.101',
      }

      const updatedDevice = { ...mockDevice, ...updateRequest }

      mockApi.put.mockResolvedValue({
        data: {
          success: true,
          data: updatedDevice,
        },
      })

      const result = await updateDevice(1, updateRequest)

      expect(mockApi.put).toHaveBeenCalledWith('/devices/1', updateRequest)
      expect(result).toEqual(updatedDevice)
    })

    it('should handle update error', async () => {
      mockApi.put.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Failed to update device' },
        },
      })

      await expect(updateDevice(1, { name: 'New Name' })).rejects.toThrow('Failed to update device')
    })
  })

  describe('deleteDevice', () => {
    it('should delete a device successfully', async () => {
      mockApi.delete.mockResolvedValue({
        data: {
          success: true,
        },
      })

      await deleteDevice(1)

      expect(mockApi.delete).toHaveBeenCalledWith('/devices/1')
    })

    it('should handle delete error', async () => {
      mockApi.delete.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device not found' },
        },
      })

      await expect(deleteDevice(999)).rejects.toThrow('Device not found')
    })

    it('should accept string ID', async () => {
      mockApi.delete.mockResolvedValue({
        data: {
          success: true,
        },
      })

      await deleteDevice('1')

      expect(mockApi.delete).toHaveBeenCalledWith('/devices/1')
    })
  })

  describe('controlDevice', () => {
    it('should send control command successfully - on', async () => {
      const controlResponse = {
        status: 'success',
        device_id: 1,
        action: 'on',
      }

      mockApi.post.mockResolvedValue({
        data: {
          success: true,
          data: controlResponse,
        },
      })

      const result = await controlDevice(1, { action: 'on' })

      expect(mockApi.post).toHaveBeenCalledWith('/devices/1/control', { action: 'on' })
      expect(result).toEqual(controlResponse)
    })

    it('should send control command successfully - off', async () => {
      const controlResponse = {
        status: 'success',
        device_id: 1,
        action: 'off',
      }

      mockApi.post.mockResolvedValue({
        data: {
          success: true,
          data: controlResponse,
        },
      })

      const result = await controlDevice(1, { action: 'off' })

      expect(mockApi.post).toHaveBeenCalledWith('/devices/1/control', { action: 'off' })
      expect(result).toEqual(controlResponse)
    })

    it('should send control command with channel parameter', async () => {
      const controlResponse = {
        status: 'success',
        device_id: 1,
        action: 'on',
      }

      mockApi.post.mockResolvedValue({
        data: {
          success: true,
          data: controlResponse,
        },
      })

      const result = await controlDevice(1, { action: 'on', params: { channel: 2 } })

      expect(mockApi.post).toHaveBeenCalledWith('/devices/1/control', {
        action: 'on',
        params: { channel: 2 },
      })
      expect(result).toEqual(controlResponse)
    })

    it('should handle control error - device offline', async () => {
      mockApi.post.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device is offline' },
        },
      })

      await expect(controlDevice(1, { action: 'on' })).rejects.toThrow('Device is offline')
    })

    it('should support all actions', async () => {
      const actions: Array<'on' | 'off' | 'toggle' | 'reboot'> = ['on', 'off', 'toggle', 'reboot']

      for (const action of actions) {
        mockApi.post.mockResolvedValue({
          data: {
            success: true,
            data: { status: 'success', device_id: 1, action },
          },
        })

        await controlDevice(1, { action })

        expect(mockApi.post).toHaveBeenCalledWith('/devices/1/control', { action })
      }
    })
  })

  describe('getDeviceStatus', () => {
    it('should fetch device status successfully', async () => {
      const mockStatus: DeviceStatus = {
        device_id: 1,
        ip: '192.168.1.100',
        temperature: 45.5,
        uptime: 86400,
        wifi: {
          connected: true,
          ssid: 'TestNetwork',
          ip: '192.168.1.100',
          rssi: -45,
        },
        switches: [
          {
            id: 0,
            output: true,
            apower: 125.5,
            voltage: 230,
            current: 0.55,
          },
        ],
      }

      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: mockStatus,
        },
      })

      const result = await getDeviceStatus(1)

      expect(mockApi.get).toHaveBeenCalledWith('/devices/1/status')
      expect(result).toEqual(mockStatus)
    })

    it('should handle status fetch error', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Failed to fetch device status' },
        },
      })

      await expect(getDeviceStatus(1)).rejects.toThrow('Failed to fetch device status')
    })
  })

  describe('getDeviceEnergy', () => {
    it('should fetch device energy successfully with default channel', async () => {
      const mockEnergy: DeviceEnergy = {
        timestamp: '2023-01-01T00:00:00Z',
        power: 125.5,
        total: 1500000,
        total_returned: 0,
        voltage: 230,
        current: 0.55,
        pf: 0.98,
      }

      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: mockEnergy,
        },
      })

      const result = await getDeviceEnergy(1)

      expect(mockApi.get).toHaveBeenCalledWith('/devices/1/energy', {
        params: { channel: 0 },
      })
      expect(result).toEqual(mockEnergy)
    })

    it('should fetch device energy with specified channel', async () => {
      const mockEnergy: DeviceEnergy = {
        timestamp: '2023-01-01T00:00:00Z',
        power: 75.2,
        total: 500000,
        total_returned: 0,
        voltage: 230,
        current: 0.33,
      }

      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: mockEnergy,
        },
      })

      const result = await getDeviceEnergy(1, 2)

      expect(mockApi.get).toHaveBeenCalledWith('/devices/1/energy', {
        params: { channel: 2 },
      })
      expect(result).toEqual(mockEnergy)
    })

    it('should handle energy fetch error', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device does not support energy metering' },
        },
      })

      await expect(getDeviceEnergy(1)).rejects.toThrow('Device does not support energy metering')
    })
  })
})
