import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client
vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn()
    }
  }
})

import api from '../client'
import {
  createDevice,
  updateDevice,
  deleteDevice,
  controlDevice,
  getDeviceStatus,
  getDeviceEnergy,
  type DeviceStatus,
  type DeviceEnergy
} from '../devices'
import type { Device } from '../types'

describe('devices api - extended operations', () => {
  beforeEach(() => {
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
    ;(api.delete as any).mockReset()
    ;(api.get as any).mockReset()
  })

  describe('createDevice', () => {
    it('creates a new device', async () => {
      const newDevice: Partial<Device> = {
        name: 'Living Room Switch',
        type: 'shelly1',
        ipAddress: '192.168.1.100'
      }

      const createdDevice: Device = {
        id: 123,
        name: 'Living Room Switch',
        type: 'shelly1',
        ipAddress: '192.168.1.100',
        mac: 'AA:BB:CC:DD:EE:FF',
        online: false,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: createdDevice,
          timestamp: new Date().toISOString()
        }
      })

      const device = await createDevice(newDevice)
      expect(device).toEqual(createdDevice)
      expect(api.post).toHaveBeenCalledWith('/devices', newDevice)
    })

    it('throws error on API failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid device data' }
        }
      })

      await expect(createDevice({})).rejects.toThrow('Invalid device data')
    })
  })

  describe('updateDevice', () => {
    it('updates an existing device', async () => {
      const updates: Partial<Device> = {
        name: 'Updated Name'
      }

      const updatedDevice: Device = {
        id: 123,
        name: 'Updated Name',
        type: 'shelly1',
        ipAddress: '192.168.1.100',
        mac: 'AA:BB:CC:DD:EE:FF',
        online: true,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T01:00:00Z'
      }

      ;(api.put as any).mockResolvedValue({
        data: {
          success: true,
          data: updatedDevice,
          timestamp: new Date().toISOString()
        }
      })

      const device = await updateDevice(123, updates)
      expect(device).toEqual(updatedDevice)
      expect(api.put).toHaveBeenCalledWith('/devices/123', updates)
    })

    it('throws error when device not found', async () => {
      ;(api.put as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device not found' }
        }
      })

      await expect(updateDevice(999, {})).rejects.toThrow('Device not found')
    })
  })

  describe('deleteDevice', () => {
    it('deletes a device', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await deleteDevice(123)
      expect(api.delete).toHaveBeenCalledWith('/devices/123')
    })

    it('throws error on deletion failure', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Cannot delete device in use' }
        }
      })

      await expect(deleteDevice(123)).rejects.toThrow('Cannot delete device in use')
    })
  })

  describe('controlDevice', () => {
    it('sends control command to device', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await controlDevice(123, 'on')
      expect(api.post).toHaveBeenCalledWith('/devices/123/control', { action: 'on' })
    })

    it('supports different actions', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await controlDevice(123, 'off')
      expect(api.post).toHaveBeenCalledWith('/devices/123/control', { action: 'off' })

      await controlDevice(123, 'restart')
      expect(api.post).toHaveBeenCalledWith('/devices/123/control', { action: 'restart' })

      await controlDevice(123, 'toggle')
      expect(api.post).toHaveBeenCalledWith('/devices/123/control', { action: 'toggle' })
    })

    it('throws error when control fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device offline' }
        }
      })

      await expect(controlDevice(123, 'on')).rejects.toThrow('Device offline')
    })
  })

  describe('getDeviceStatus', () => {
    it('returns device status', async () => {
      const status: DeviceStatus = {
        online: true,
        lastSeen: '2023-01-01T12:00:00Z',
        uptime: 86400,
        temperature: 45,
        cloud: { enabled: true, connected: true },
        wifi: { ssid: 'MyNetwork', rssi: -55 },
        mqtt: { connected: false }
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: status,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDeviceStatus(123)
      expect(result).toEqual(status)
      expect(api.get).toHaveBeenCalledWith('/devices/123/status')
    })

    it('throws error when status unavailable', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Status unavailable' }
        }
      })

      await expect(getDeviceStatus(123)).rejects.toThrow('Status unavailable')
    })
  })

  describe('getDeviceEnergy', () => {
    it('returns energy metrics for device', async () => {
      const energy: DeviceEnergy = {
        power: 150.5,
        voltage: 230.2,
        current: 0.65,
        total: 1234.56,
        totalReturned: 0
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: energy,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDeviceEnergy(123)
      expect(result).toEqual(energy)
      expect(api.get).toHaveBeenCalledWith('/devices/123/energy')
    })

    it('handles partial energy data', async () => {
      const energy: DeviceEnergy = {
        power: 50.0
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: energy,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDeviceEnergy(123)
      expect(result.power).toBe(50.0)
      expect(result.voltage).toBeUndefined()
    })

    it('throws error when energy data unavailable', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device does not support power metering' }
        }
      })

      await expect(getDeviceEnergy(123)).rejects.toThrow('Device does not support power metering')
    })
  })
})
