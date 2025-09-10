import { vi, describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useScheduleStore } from '../schedule'
import * as scheduleApi from '@/api/schedule'
import type { ExportSchedule, ExportScheduleRequest } from '@/api/schedule'

// Mock the API
vi.mock('@/api/schedule', () => ({
  listSchedules: vi.fn(),
  createSchedule: vi.fn(),
  getSchedule: vi.fn(),
  updateSchedule: vi.fn(),
  deleteSchedule: vi.fn(),
  runSchedule: vi.fn()
}))

const mockApi = vi.mocked(scheduleApi)

describe('Schedule Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  const mockSchedule: ExportSchedule = {
    id: 'schedule-1',
    name: 'Test Schedule',
    interval_sec: 3600,
    enabled: true,
    request: { plugin_name: 'test', format: 'json' },
    created_at: '2023-01-01T00:00:00Z',
    updated_at: '2023-01-01T00:00:00Z'
  }

  describe('Initial state', () => {
    it('should have correct initial state', () => {
      const store = useScheduleStore()

      expect(store.schedules).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.error).toBe('')
      expect(store.page).toBe(1)
      expect(store.pageSize).toBe(20)
      expect(store.plugin).toBe('')
      expect(store.enabled).toBeUndefined()
      expect(store.currentSchedule).toBeNull()
      expect(store.runningSchedules.size).toBe(0)
    })
  })

  describe('Getters', () => {
    it('should filter schedules by plugin', () => {
      const store = useScheduleStore()
      
      store.schedules = [
        { ...mockSchedule, id: '1', request: { plugin_name: 'plugin1', format: 'json' } },
        { ...mockSchedule, id: '2', request: { plugin_name: 'plugin2', format: 'json' } },
        { ...mockSchedule, id: '3', request: { plugin_name: 'Plugin1', format: 'json' } }
      ]
      store.plugin = 'plugin1'

      const filtered = store.filteredSchedules
      expect(filtered).toHaveLength(2)
      expect(filtered[0].id).toBe('1')
      expect(filtered[1].id).toBe('3') // Case insensitive
    })

    it('should filter schedules by enabled status', () => {
      const store = useScheduleStore()
      
      store.schedules = [
        { ...mockSchedule, id: '1', enabled: true },
        { ...mockSchedule, id: '2', enabled: false },
        { ...mockSchedule, id: '3', enabled: true }
      ]
      store.enabled = true

      const filtered = store.filteredSchedules
      expect(filtered).toHaveLength(2)
      expect(filtered.every(s => s.enabled)).toBe(true)
    })

    it('should sort schedules correctly', () => {
      const store = useScheduleStore()
      
      store.schedules = [
        { ...mockSchedule, id: '1', name: 'Z Schedule', enabled: false },
        { ...mockSchedule, id: '2', name: 'A Schedule', enabled: true },
        { ...mockSchedule, id: '3', name: 'B Schedule', enabled: false }
      ]

      const sorted = store.schedulesSorted
      expect(sorted[0].name).toBe('A Schedule') // Enabled first
      expect(sorted[1].name).toBe('B Schedule') // Then by name
      expect(sorted[2].name).toBe('Z Schedule')
    })

    it('should calculate stats correctly', () => {
      const store = useScheduleStore()
      
      store.schedules = [
        { ...mockSchedule, id: '1', enabled: true, request: { plugin_name: 'plugin1', format: 'json' } },
        { ...mockSchedule, id: '2', enabled: false, request: { plugin_name: 'plugin1', format: 'json' } },
        { ...mockSchedule, id: '3', enabled: true, request: { plugin_name: 'plugin2', format: 'json' } }
      ]

      const stats = store.stats
      expect(stats.total).toBe(3)
      expect(stats.enabled).toBe(2)
      expect(stats.disabled).toBe(1)
      expect(stats.byPlugin).toEqual({
        plugin1: 2,
        plugin2: 1
      })
    })

    it('should check running schedule status', () => {
      const store = useScheduleStore()
      
      store.runningSchedules.add('schedule-1')

      expect(store.isScheduleRunning('schedule-1')).toBe(true)
      expect(store.isScheduleRunning('schedule-2')).toBe(false)
    })

    it('should get recent run results', () => {
      const store = useScheduleStore()
      const runResult = { export_id: 'export-1', plugin_name: 'test', format: 'json' }
      
      store.recentRuns.set('schedule-1', runResult)

      expect(store.getRecentRun('schedule-1')).toEqual(runResult)
      expect(store.getRecentRun('schedule-2')).toBeUndefined()
    })
  })

  describe('Actions', () => {
    describe('fetchSchedules', () => {
      it('should fetch schedules successfully', async () => {
        const store = useScheduleStore()
        const mockSchedules = [mockSchedule]
        const mockMeta = { pagination: { page: 1, page_size: 20, total_pages: 1, has_next: false, has_previous: false } }

        mockApi.listSchedules.mockResolvedValue({
          schedules: mockSchedules,
          meta: mockMeta
        })

        await store.fetchSchedules()

        expect(mockApi.listSchedules).toHaveBeenCalledWith({
          page: 1,
          pageSize: 20,
          plugin: undefined,
          enabled: undefined
        })
        expect(store.schedules).toEqual(mockSchedules)
        expect(store.meta).toEqual(mockMeta)
        expect(store.loading).toBe(false)
        expect(store.error).toBe('')
      })

      it('should handle fetch error', async () => {
        const store = useScheduleStore()
        
        mockApi.listSchedules.mockRejectedValue(new Error('Network error'))

        await store.fetchSchedules()

        expect(store.schedules).toEqual([])
        expect(store.error).toBe('Network error')
        expect(store.loading).toBe(false)
      })

      it('should apply filters when fetching', async () => {
        const store = useScheduleStore()
        store.plugin = 'test-plugin'
        store.enabled = true

        mockApi.listSchedules.mockResolvedValue({ schedules: [], meta: undefined })

        await store.fetchSchedules()

        expect(mockApi.listSchedules).toHaveBeenCalledWith({
          page: 1,
          pageSize: 20,
          plugin: 'test-plugin',
          enabled: true
        })
      })
    })

    describe('createSchedule', () => {
      it('should create schedule successfully', async () => {
        const store = useScheduleStore()
        const request: ExportScheduleRequest = {
          name: 'New Schedule',
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        mockApi.createSchedule.mockResolvedValue(mockSchedule)

        const result = await store.createSchedule(request)

        expect(mockApi.createSchedule).toHaveBeenCalledWith(request)
        expect(result).toEqual(mockSchedule)
        expect(store.schedules[0]).toEqual(mockSchedule) // Added to beginning
        expect(store.currentLoading).toBe(false)
        expect(store.currentError).toBe('')
      })

      it('should handle creation error', async () => {
        const store = useScheduleStore()
        const request: ExportScheduleRequest = {
          name: 'Invalid',
          interval_sec: 30,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        mockApi.createSchedule.mockRejectedValue(new Error('Validation failed'))

        await expect(store.createSchedule(request)).rejects.toThrow('Validation failed')
        expect(store.currentError).toBe('Validation failed')
        expect(store.currentLoading).toBe(false)
      })
    })

    describe('loadSchedule', () => {
      it('should load schedule successfully', async () => {
        const store = useScheduleStore()
        
        mockApi.getSchedule.mockResolvedValue(mockSchedule)

        const result = await store.loadSchedule('schedule-1')

        expect(mockApi.getSchedule).toHaveBeenCalledWith('schedule-1')
        expect(result).toEqual(mockSchedule)
        expect(store.currentSchedule).toEqual(mockSchedule)
        expect(store.currentLoading).toBe(false)
        expect(store.currentError).toBe('')
      })

      it('should handle load error', async () => {
        const store = useScheduleStore()
        
        mockApi.getSchedule.mockRejectedValue(new Error('Not found'))

        await expect(store.loadSchedule('nonexistent')).rejects.toThrow('Not found')
        expect(store.currentSchedule).toBeNull()
        expect(store.currentError).toBe('Not found')
      })
    })

    describe('updateSchedule', () => {
      it('should update schedule successfully', async () => {
        const store = useScheduleStore()
        const updatedSchedule = { ...mockSchedule, name: 'Updated Schedule' }
        const request: ExportScheduleRequest = {
          name: 'Updated Schedule',
          interval_sec: 3600,
          enabled: true,
          request: { plugin_name: 'test', format: 'json' }
        }

        store.schedules = [mockSchedule]
        store.currentSchedule = mockSchedule

        mockApi.updateSchedule.mockResolvedValue(updatedSchedule)

        const result = await store.updateSchedule('schedule-1', request)

        expect(mockApi.updateSchedule).toHaveBeenCalledWith('schedule-1', request)
        expect(result).toEqual(updatedSchedule)
        expect(store.schedules[0]).toEqual(updatedSchedule)
        expect(store.currentSchedule).toEqual(updatedSchedule)
      })
    })

    describe('deleteSchedule', () => {
      it('should delete schedule successfully', async () => {
        const store = useScheduleStore()
        
        store.schedules = [mockSchedule, { ...mockSchedule, id: 'schedule-2' }]
        store.currentSchedule = mockSchedule
        store.runningSchedules.add('schedule-1')
        store.recentRuns.set('schedule-1', { export_id: 'test', plugin_name: 'test', format: 'json' })

        mockApi.deleteSchedule.mockResolvedValue(undefined)

        await store.deleteSchedule('schedule-1')

        expect(mockApi.deleteSchedule).toHaveBeenCalledWith('schedule-1')
        expect(store.schedules).toHaveLength(1)
        expect(store.schedules[0].id).toBe('schedule-2')
        expect(store.currentSchedule).toBeNull()
        expect(store.runningSchedules.has('schedule-1')).toBe(false)
        expect(store.recentRuns.has('schedule-1')).toBe(false)
      })

      it('should handle delete error', async () => {
        const store = useScheduleStore()
        
        mockApi.deleteSchedule.mockRejectedValue(new Error('Delete failed'))

        await expect(store.deleteSchedule('schedule-1')).rejects.toThrow('Delete failed')
        expect(store.error).toBe('Delete failed')
      })
    })

    describe('runScheduleNow', () => {
      it('should run schedule successfully', async () => {
        const store = useScheduleStore()
        const runResult = { 
          export_id: 'export-123', 
          plugin_name: 'test', 
          format: 'json', 
          record_count: 100 
        }

        store.schedules = [mockSchedule]

        mockApi.runSchedule.mockResolvedValue(runResult)

        const result = await store.runScheduleNow('schedule-1')

        expect(mockApi.runSchedule).toHaveBeenCalledWith('schedule-1')
        expect(result).toEqual(runResult)
        expect(store.recentRuns.get('schedule-1')).toEqual(runResult)
        expect(store.schedules[0].last_run).toBeDefined()
        expect(store.runningSchedules.has('schedule-1')).toBe(false) // Cleaned up after completion
      })

      it('should handle run error', async () => {
        const store = useScheduleStore()
        
        mockApi.runSchedule.mockRejectedValue(new Error('Run failed'))

        await expect(store.runScheduleNow('schedule-1')).rejects.toThrow('Run failed')
        expect(store.error).toBe('Run failed')
        expect(store.runningSchedules.has('schedule-1')).toBe(false) // Cleaned up after error
      })
    })

    describe('Filter methods', () => {
      it('should set plugin filter and reset page', () => {
        const store = useScheduleStore()
        store.page = 3

        store.setPlugin('new-plugin')

        expect(store.plugin).toBe('new-plugin')
        expect(store.page).toBe(1)
      })

      it('should set enabled filter and reset page', () => {
        const store = useScheduleStore()
        store.page = 2

        store.setEnabled(false)

        expect(store.enabled).toBe(false)
        expect(store.page).toBe(1)
      })

      it('should set page', () => {
        const store = useScheduleStore()

        store.setPage(5)

        expect(store.page).toBe(5)
      })

      it('should set page size and reset page', () => {
        const store = useScheduleStore()
        store.page = 3

        store.setPageSize(50)

        expect(store.pageSize).toBe(50)
        expect(store.page).toBe(1)
      })
    })

    describe('toggleScheduleEnabled', () => {
      it('should toggle schedule enabled status', async () => {
        const store = useScheduleStore()
        const disabledSchedule = { ...mockSchedule, enabled: false }
        
        store.schedules = [mockSchedule]
        store.currentSchedule = mockSchedule

        mockApi.updateSchedule.mockResolvedValue(disabledSchedule)

        await store.toggleScheduleEnabled('schedule-1')

        expect(mockApi.updateSchedule).toHaveBeenCalledWith('schedule-1', {
          name: mockSchedule.name,
          interval_sec: mockSchedule.interval_sec,
          enabled: false, // Toggled
          request: mockSchedule.request
        })
        expect(store.schedules[0].enabled).toBe(false)
        expect(store.currentSchedule?.enabled).toBe(false)
      })

      it('should handle toggle error', async () => {
        const store = useScheduleStore()
        store.schedules = [mockSchedule]

        mockApi.updateSchedule.mockRejectedValue(new Error('Toggle failed'))

        await expect(store.toggleScheduleEnabled('schedule-1')).rejects.toThrow('Toggle failed')
        expect(store.error).toBe('Toggle failed')
      })
    })

    describe('Utility methods', () => {
      it('should clear current schedule', () => {
        const store = useScheduleStore()
        
        store.currentSchedule = mockSchedule
        store.currentError = 'Some error'

        store.clearCurrentSchedule()

        expect(store.currentSchedule).toBeNull()
        expect(store.currentError).toBe('')
      })

      it('should clear all errors', () => {
        const store = useScheduleStore()
        
        store.error = 'Main error'
        store.currentError = 'Current error'

        store.clearErrors()

        expect(store.error).toBe('')
        expect(store.currentError).toBe('')
      })
    })
  })
})