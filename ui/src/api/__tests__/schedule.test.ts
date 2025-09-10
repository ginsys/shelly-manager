import { vi, describe, it, expect, beforeEach } from 'vitest'
import {
  listSchedules,
  createSchedule,
  getSchedule,
  updateSchedule,
  deleteSchedule,
  runSchedule,
  formatInterval,
  parseInterval,
  calculateNextRun,
  validateScheduleRequest,
  type ExportSchedule,
  type ExportScheduleRequest
} from '../schedule'
import api from '../client'

// Mock the API client
vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn()
  }
}))

const mockApi = vi.mocked(api)

describe('Schedule API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  describe('listSchedules', () => {
    it('should fetch schedules successfully', async () => {
      const mockSchedules = [
        {
          id: 'schedule-1',
          name: 'Test Schedule',
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' },
          created_at: '2023-01-01T00:00:00Z',
          updated_at: '2023-01-01T00:00:00Z'
        }
      ]

      mockApi.get.mockResolvedValue({
        data: {
          success: true,
          data: { schedules: mockSchedules },
          meta: { pagination: { page: 1, page_size: 20, total_pages: 1, has_next: false, has_previous: false } }
        }
      })

      const result = await listSchedules({ page: 1, pageSize: 20 })

      expect(mockApi.get).toHaveBeenCalledWith('/export/schedules', {
        params: { page: 1, page_size: 20, plugin: undefined, enabled: undefined }
      })
      expect(result.schedules).toEqual(mockSchedules)
      expect(result.meta).toBeDefined()
    })

    it('should handle API error', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Server error' }
        }
      })

      await expect(listSchedules()).rejects.toThrow('Server error')
    })

    it('should filter by plugin and enabled status', async () => {
      mockApi.get.mockResolvedValue({
        data: { success: true, data: { schedules: [] } }
      })

      await listSchedules({ plugin: 'test-plugin', enabled: true })

      expect(mockApi.get).toHaveBeenCalledWith('/export/schedules', {
        params: { page: 1, page_size: 20, plugin: 'test-plugin', enabled: true }
      })
    })
  })

  describe('createSchedule', () => {
    it('should create schedule successfully', async () => {
      const request: ExportScheduleRequest = {
        name: 'New Schedule',
        interval_sec: 7200,
        enabled: true,
        request: { plugin_name: 'test', format: 'json' }
      }

      const mockSchedule: ExportSchedule = {
        id: 'new-schedule',
        ...request,
        created_at: '2023-01-01T00:00:00Z',
        updated_at: '2023-01-01T00:00:00Z'
      }

      mockApi.post.mockResolvedValue({
        data: { success: true, data: mockSchedule }
      })

      const result = await createSchedule(request)

      expect(mockApi.post).toHaveBeenCalledWith('/export/schedules', request)
      expect(result).toEqual(mockSchedule)
    })

    it('should handle creation error', async () => {
      mockApi.post.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Validation failed' }
        }
      })

      const request: ExportScheduleRequest = {
        name: 'Invalid',
        interval_sec: 30, // Too short
        enabled: true,
        request: { plugin_name: 'test', format: 'json' }
      }

      await expect(createSchedule(request)).rejects.toThrow('Validation failed')
    })
  })

  describe('getSchedule', () => {
    it('should fetch schedule by ID', async () => {
      const mockSchedule: ExportSchedule = {
        id: 'schedule-1',
        name: 'Test Schedule',
        interval_sec: 3600,
        enabled: true,
        request: { plugin_name: 'test', format: 'json' },
        created_at: '2023-01-01T00:00:00Z',
        updated_at: '2023-01-01T00:00:00Z'
      }

      mockApi.get.mockResolvedValue({
        data: { success: true, data: mockSchedule }
      })

      const result = await getSchedule('schedule-1')

      expect(mockApi.get).toHaveBeenCalledWith('/export/schedules/schedule-1')
      expect(result).toEqual(mockSchedule)
    })

    it('should handle not found error', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Schedule not found' }
        }
      })

      await expect(getSchedule('nonexistent')).rejects.toThrow('Schedule not found')
    })
  })

  describe('updateSchedule', () => {
    it('should update schedule successfully', async () => {
      const request: ExportScheduleRequest = {
        name: 'Updated Schedule',
        interval_sec: 7200,
        enabled: false,
        request: { plugin_name: 'updated', format: 'yaml' }
      }

      const mockSchedule: ExportSchedule = {
        id: 'schedule-1',
        ...request,
        created_at: '2023-01-01T00:00:00Z',
        updated_at: '2023-01-01T01:00:00Z'
      }

      mockApi.put.mockResolvedValue({
        data: { success: true, data: mockSchedule }
      })

      const result = await updateSchedule('schedule-1', request)

      expect(mockApi.put).toHaveBeenCalledWith('/export/schedules/schedule-1', request)
      expect(result).toEqual(mockSchedule)
    })
  })

  describe('deleteSchedule', () => {
    it('should delete schedule successfully', async () => {
      mockApi.delete.mockResolvedValue({
        data: { success: true, data: { status: 'deleted' } }
      })

      await deleteSchedule('schedule-1')

      expect(mockApi.delete).toHaveBeenCalledWith('/export/schedules/schedule-1')
    })

    it('should handle delete error', async () => {
      mockApi.delete.mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Schedule not found' }
        }
      })

      await expect(deleteSchedule('nonexistent')).rejects.toThrow('Schedule not found')
    })
  })

  describe('runSchedule', () => {
    it('should run schedule successfully', async () => {
      const mockResult = {
        export_id: 'export-123',
        plugin_name: 'test',
        format: 'json',
        record_count: 100,
        file_size: 1024
      }

      mockApi.post.mockResolvedValue({
        data: { success: true, data: mockResult }
      })

      const result = await runSchedule('schedule-1')

      expect(mockApi.post).toHaveBeenCalledWith('/export/schedules/schedule-1/run')
      expect(result).toEqual(mockResult)
    })
  })

  describe('Helper functions', () => {
    describe('formatInterval', () => {
      it('should format seconds correctly', () => {
        expect(formatInterval(30)).toBe('30 seconds')
        expect(formatInterval(1)).toBe('1 second')
      })

      it('should format minutes correctly', () => {
        expect(formatInterval(60)).toBe('1 minute')
        expect(formatInterval(120)).toBe('2 minutes')
      })

      it('should format hours correctly', () => {
        expect(formatInterval(3600)).toBe('1 hour')
        expect(formatInterval(7200)).toBe('2 hours')
      })

      it('should format days correctly', () => {
        expect(formatInterval(86400)).toBe('1 day')
        expect(formatInterval(172800)).toBe('2 days')
      })
    })

    describe('parseInterval', () => {
      it('should parse interval strings correctly', () => {
        expect(parseInterval('30 seconds')).toBe(30)
        expect(parseInterval('5 minutes')).toBe(300)
        expect(parseInterval('1 hour')).toBe(3600)
        expect(parseInterval('2 days')).toBe(172800)
      })

      it('should handle singular forms', () => {
        expect(parseInterval('1 second')).toBe(1)
        expect(parseInterval('1 minute')).toBe(60)
        expect(parseInterval('1 hour')).toBe(3600)
        expect(parseInterval('1 day')).toBe(86400)
      })

      it('should throw error for invalid format', () => {
        expect(() => parseInterval('invalid')).toThrow('Invalid interval format')
        expect(() => parseInterval('5 weeks')).toThrow('Invalid interval format')
      })
    })

    describe('calculateNextRun', () => {
      it('should calculate next run time', () => {
        const now = new Date('2023-01-01T12:00:00Z')
        const result = calculateNextRun(3600, now.toISOString()) // 1 hour

        expect(result.getTime()).toBe(now.getTime() + 3600000)
      })

      it('should use current time if no last run provided', () => {
        const beforeCall = Date.now()
        const result = calculateNextRun(3600)
        const afterCall = Date.now()

        expect(result.getTime()).toBeGreaterThanOrEqual(beforeCall + 3600000)
        expect(result.getTime()).toBeLessThanOrEqual(afterCall + 3600000)
      })
    })

    describe('validateScheduleRequest', () => {
      it('should validate correct request', () => {
        const request: ExportScheduleRequest = {
          name: 'Valid Schedule',
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        const errors = validateScheduleRequest(request)
        expect(errors).toEqual([])
      })

      it('should detect missing name', () => {
        const request: ExportScheduleRequest = {
          name: '',
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        const errors = validateScheduleRequest(request)
        expect(errors).toContain('Name is required')
      })

      it('should detect short interval', () => {
        const request: ExportScheduleRequest = {
          name: 'Test',
          interval_sec: 30, // Too short
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        const errors = validateScheduleRequest(request)
        expect(errors).toContain('Interval must be at least 60 seconds')
      })

      it('should detect long interval', () => {
        const request: ExportScheduleRequest = {
          name: 'Test',
          interval_sec: 86400 * 31, // Too long
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        const errors = validateScheduleRequest(request)
        expect(errors).toContain('Interval must be less than 30 days')
      })

      it('should detect missing plugin and format', () => {
        const request: ExportScheduleRequest = {
          name: 'Test',
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: '', format: '' }
        }

        const errors = validateScheduleRequest(request)
        expect(errors).toContain('Plugin name is required')
        expect(errors).toContain('Format is required')
      })

      it('should detect long name', () => {
        const request: ExportScheduleRequest = {
          name: 'A'.repeat(101), // Too long
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        const errors = validateScheduleRequest(request)
        expect(errors).toContain('Name must be less than 100 characters')
      })
    })
  })
})