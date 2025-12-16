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
  listDriftSchedules,
  getDriftSchedule,
  createDriftSchedule,
  updateDriftSchedule,
  deleteDriftSchedule,
  toggleDriftSchedule,
  getDriftScheduleRuns,
  getDriftReports,
  getDriftTrends,
  resolveDriftReport,
  generateDeviceDriftReport,
  type DriftSchedule,
  type DriftScheduleRun,
  type DriftReport,
  type DriftTrend,
  type CreateDriftScheduleRequest
} from '../drift'

describe('Drift Detection API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('listDriftSchedules', () => {
    it('returns list of drift schedules with default pagination', async () => {
      const schedules: DriftSchedule[] = [
        {
          id: 1,
          name: 'Daily Check',
          description: 'Check all devices daily',
          deviceIds: [1, 2, 3],
          checkInterval: '24h',
          enabled: true,
          lastRun: '2023-01-01T00:00:00Z',
          nextRun: '2023-01-02T00:00:00Z',
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        }
      ]
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { schedules },
          meta: { total: 1, page: 1, pageSize: 25 },
          timestamp: new Date().toISOString()
        }
      })

      const result = await listDriftSchedules()

      expect(result.items).toEqual(schedules)
      expect(result.meta).toEqual({ total: 1, page: 1, pageSize: 25 })
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules', {
        params: { page: 1, page_size: 25 }
      })
    })

    it('returns list with custom pagination', async () => {
      const schedules: DriftSchedule[] = []
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { schedules },
          timestamp: new Date().toISOString()
        }
      })

      const result = await listDriftSchedules(2, 50)

      expect(result.items).toEqual([])
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules', {
        params: { page: 2, page_size: 50 }
      })
    })

    it('throws error when request fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Database error', code: 'DB_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(listDriftSchedules()).rejects.toThrow('Database error')
    })

    it('throws default error when no error message provided', async () => {
      ;(api.get as any).mockResolvedValue({
        data: { success: false, timestamp: new Date().toISOString() }
      })

      await expect(listDriftSchedules()).rejects.toThrow('Failed to load drift schedules')
    })
  })

  describe('getDriftSchedule', () => {
    it('returns a single drift schedule', async () => {
      const schedule: DriftSchedule = {
        id: 1,
        name: 'Hourly Check',
        checkInterval: '1h',
        enabled: true,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: schedule, timestamp: new Date().toISOString() }
      })

      const result = await getDriftSchedule(1)

      expect(result).toEqual(schedule)
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/1')
    })

    it('accepts string ID', async () => {
      const schedule: DriftSchedule = {
        id: 123,
        name: 'Test',
        checkInterval: '1h',
        enabled: false,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }
      ;(api.get as any).mockResolvedValue({
        data: { success: true, data: schedule, timestamp: new Date().toISOString() }
      })

      const result = await getDriftSchedule('123')

      expect(result).toEqual(schedule)
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/123')
    })

    it('throws error when request fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Not found', code: 'NOT_FOUND' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getDriftSchedule(999)).rejects.toThrow('Not found')
    })

    it('throws error when data is missing', async () => {
      ;(api.get as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(getDriftSchedule(1)).rejects.toThrow('Failed to load drift schedule')
    })
  })

  describe('createDriftSchedule', () => {
    it('creates a new drift schedule', async () => {
      const request: CreateDriftScheduleRequest = {
        name: 'New Schedule',
        description: 'Test schedule',
        deviceIds: [1, 2],
        checkInterval: '6h',
        enabled: true
      }
      const created: DriftSchedule = {
        id: 5,
        ...request,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: created, timestamp: new Date().toISOString() }
      })

      const result = await createDriftSchedule(request)

      expect(result).toEqual(created)
      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules', request)
    })

    it('throws error when creation fails', async () => {
      const request: CreateDriftScheduleRequest = {
        name: 'Bad Schedule',
        checkInterval: 'invalid'
      }
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid interval', code: 'VALIDATION_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(createDriftSchedule(request)).rejects.toThrow('Invalid interval')
    })

    it('throws default error when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(createDriftSchedule({ name: 'Test', checkInterval: '1h' })).rejects.toThrow(
        'Failed to create drift schedule'
      )
    })
  })

  describe('updateDriftSchedule', () => {
    it('updates an existing drift schedule', async () => {
      const updates = { name: 'Updated Name', enabled: false }
      const updated: DriftSchedule = {
        id: 1,
        name: 'Updated Name',
        checkInterval: '1h',
        enabled: false,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-02T00:00:00Z'
      }
      ;(api.put as any).mockResolvedValue({
        data: { success: true, data: updated, timestamp: new Date().toISOString() }
      })

      const result = await updateDriftSchedule(1, updates)

      expect(result).toEqual(updated)
      expect(api.put).toHaveBeenCalledWith('/config/drift-schedules/1', updates)
    })

    it('accepts string ID', async () => {
      const updates = { checkInterval: '12h' }
      const updated: DriftSchedule = {
        id: 42,
        name: 'Test',
        checkInterval: '12h',
        enabled: true,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }
      ;(api.put as any).mockResolvedValue({
        data: { success: true, data: updated, timestamp: new Date().toISOString() }
      })

      const result = await updateDriftSchedule('42', updates)

      expect(result).toEqual(updated)
      expect(api.put).toHaveBeenCalledWith('/config/drift-schedules/42', updates)
    })

    it('throws error when update fails', async () => {
      ;(api.put as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Schedule not found', code: 'NOT_FOUND' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(updateDriftSchedule(999, { name: 'Test' })).rejects.toThrow(
        'Schedule not found'
      )
    })
  })

  describe('deleteDriftSchedule', () => {
    it('deletes a drift schedule', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await deleteDriftSchedule(1)

      expect(api.delete).toHaveBeenCalledWith('/config/drift-schedules/1')
    })

    it('accepts string ID', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await deleteDriftSchedule('42')

      expect(api.delete).toHaveBeenCalledWith('/config/drift-schedules/42')
    })

    it('throws error when deletion fails', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Cannot delete active schedule', code: 'CONFLICT' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(deleteDriftSchedule(1)).rejects.toThrow('Cannot delete active schedule')
    })

    it('throws default error when no error message provided', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: { success: false, timestamp: new Date().toISOString() }
      })

      await expect(deleteDriftSchedule(1)).rejects.toThrow('Failed to delete drift schedule')
    })
  })

  describe('toggleDriftSchedule', () => {
    it('toggles a drift schedule enabled status', async () => {
      const toggled: DriftSchedule = {
        id: 1,
        name: 'Test',
        checkInterval: '1h',
        enabled: false,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: toggled, timestamp: new Date().toISOString() }
      })

      const result = await toggleDriftSchedule(1)

      expect(result).toEqual(toggled)
      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules/1/toggle')
    })

    it('accepts string ID', async () => {
      const toggled: DriftSchedule = {
        id: 99,
        name: 'Test',
        checkInterval: '2h',
        enabled: true,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: toggled, timestamp: new Date().toISOString() }
      })

      const result = await toggleDriftSchedule('99')

      expect(result).toEqual(toggled)
      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules/99/toggle')
    })

    it('throws error when toggle fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Schedule locked', code: 'LOCKED' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(toggleDriftSchedule(1)).rejects.toThrow('Schedule locked')
    })
  })

  describe('getDriftScheduleRuns', () => {
    it('returns schedule run history with default pagination', async () => {
      const runs: DriftScheduleRun[] = [
        {
          id: 1,
          scheduleId: 1,
          startedAt: '2023-01-01T00:00:00Z',
          completedAt: '2023-01-01T00:05:00Z',
          status: 'completed',
          devicesChecked: 10,
          driftsDetected: 2
        }
      ]
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { runs },
          meta: { total: 1, page: 1, pageSize: 25 },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDriftScheduleRuns(1)

      expect(result.items).toEqual(runs)
      expect(result.meta).toEqual({ total: 1, page: 1, pageSize: 25 })
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/1/runs', {
        params: { page: 1, page_size: 25 }
      })
    })

    it('returns runs with custom pagination', async () => {
      const runs: DriftScheduleRun[] = []
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { runs },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDriftScheduleRuns('42', 3, 10)

      expect(result.items).toEqual([])
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/42/runs', {
        params: { page: 3, page_size: 10 }
      })
    })

    it('throws error when request fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Schedule not found', code: 'NOT_FOUND' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getDriftScheduleRuns(999)).rejects.toThrow('Schedule not found')
    })
  })

  describe('getDriftReports', () => {
    it('returns drift reports with default parameters', async () => {
      const reports: DriftReport[] = [
        {
          id: 1,
          deviceId: 1,
          deviceName: 'Device 1',
          detectedAt: '2023-01-01T00:00:00Z',
          driftType: 'config_changed',
          field: 'wifi.ssid',
          expectedValue: 'OldSSID',
          actualValue: 'NewSSID',
          severity: 'medium',
          resolved: false
        }
      ]
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { reports },
          meta: { total: 1, page: 1, pageSize: 25 },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDriftReports()

      expect(result.items).toEqual(reports)
      expect(result.meta).toEqual({ total: 1, page: 1, pageSize: 25 })
      expect(api.get).toHaveBeenCalledWith('/config/drift-reports', {
        params: { page: 1, page_size: 25, resolved: undefined }
      })
    })

    it('filters by resolved status', async () => {
      const reports: DriftReport[] = []
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { reports },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDriftReports(2, 50, true)

      expect(result.items).toEqual([])
      expect(api.get).toHaveBeenCalledWith('/config/drift-reports', {
        params: { page: 2, page_size: 50, resolved: true }
      })
    })

    it('throws error when request fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Database error', code: 'DB_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getDriftReports()).rejects.toThrow('Database error')
    })
  })

  describe('getDriftTrends', () => {
    it('returns drift trends with default days', async () => {
      const trends: DriftTrend[] = [
        {
          date: '2023-01-01',
          totalDrifts: 10,
          resolvedDrifts: 8,
          unresolvedDrifts: 2,
          deviceCount: 5
        }
      ]
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { trends },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDriftTrends()

      expect(result).toEqual(trends)
      expect(api.get).toHaveBeenCalledWith('/config/drift-trends', {
        params: { days: 30 }
      })
    })

    it('returns trends with custom days', async () => {
      const trends: DriftTrend[] = []
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { trends },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getDriftTrends(7)

      expect(result).toEqual([])
      expect(api.get).toHaveBeenCalledWith('/config/drift-trends', {
        params: { days: 7 }
      })
    })

    it('throws error when request fails', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid days parameter', code: 'VALIDATION_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(getDriftTrends(0)).rejects.toThrow('Invalid days parameter')
    })

    it('throws default error when data is missing', async () => {
      ;(api.get as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(getDriftTrends()).rejects.toThrow('Failed to load drift trends')
    })
  })

  describe('resolveDriftReport', () => {
    it('resolves a drift report without notes', async () => {
      const resolved: DriftReport = {
        id: 1,
        deviceId: 1,
        deviceName: 'Device 1',
        detectedAt: '2023-01-01T00:00:00Z',
        driftType: 'config_changed',
        field: 'wifi.ssid',
        expectedValue: 'OldSSID',
        actualValue: 'NewSSID',
        severity: 'medium',
        resolved: true,
        resolvedAt: '2023-01-02T00:00:00Z',
        resolvedBy: 'admin'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: resolved, timestamp: new Date().toISOString() }
      })

      const result = await resolveDriftReport(1)

      expect(result).toEqual(resolved)
      expect(api.post).toHaveBeenCalledWith('/config/drift-trends/1/resolve', { notes: undefined })
    })

    it('resolves a drift report with notes', async () => {
      const resolved: DriftReport = {
        id: 2,
        deviceId: 2,
        deviceName: 'Device 2',
        detectedAt: '2023-01-01T00:00:00Z',
        driftType: 'unexpected_value',
        field: 'power.default',
        expectedValue: 'off',
        actualValue: 'on',
        severity: 'high',
        resolved: true,
        resolvedAt: '2023-01-02T00:00:00Z',
        resolvedBy: 'admin',
        notes: 'Fixed manually'
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: resolved, timestamp: new Date().toISOString() }
      })

      const result = await resolveDriftReport('2', 'Fixed manually')

      expect(result).toEqual(resolved)
      expect(api.post).toHaveBeenCalledWith('/config/drift-trends/2/resolve', {
        notes: 'Fixed manually'
      })
    })

    it('throws error when resolution fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Report not found', code: 'NOT_FOUND' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(resolveDriftReport(999)).rejects.toThrow('Report not found')
    })
  })

  describe('generateDeviceDriftReport', () => {
    it('generates drift report for a device', async () => {
      const reports: DriftReport[] = [
        {
          id: 1,
          deviceId: 1,
          deviceName: 'Device 1',
          detectedAt: '2023-01-01T00:00:00Z',
          driftType: 'config_changed',
          field: 'wifi.ssid',
          expectedValue: 'OldSSID',
          actualValue: 'NewSSID',
          severity: 'medium',
          resolved: false
        },
        {
          id: 2,
          deviceId: 1,
          deviceName: 'Device 1',
          detectedAt: '2023-01-01T00:00:00Z',
          driftType: 'missing_config',
          field: 'power.schedule',
          expectedValue: '{"enabled":true}',
          actualValue: null,
          severity: 'low',
          resolved: false
        }
      ]
      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: { reports },
          timestamp: new Date().toISOString()
        }
      })

      const result = await generateDeviceDriftReport(1)

      expect(result).toEqual(reports)
      expect(api.post).toHaveBeenCalledWith('/devices/1/drift-report')
    })

    it('accepts string device ID', async () => {
      const reports: DriftReport[] = []
      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: { reports },
          timestamp: new Date().toISOString()
        }
      })

      const result = await generateDeviceDriftReport('42')

      expect(result).toEqual([])
      expect(api.post).toHaveBeenCalledWith('/devices/42/drift-report')
    })

    it('throws error when generation fails', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device offline', code: 'DEVICE_OFFLINE' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(generateDeviceDriftReport(1)).rejects.toThrow('Device offline')
    })

    it('throws default error when data is missing', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(generateDeviceDriftReport(1)).rejects.toThrow(
        'Failed to generate device drift report'
      )
    })
  })
})
