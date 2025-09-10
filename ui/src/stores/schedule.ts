import { defineStore } from 'pinia'
import { 
  listSchedules, 
  createSchedule, 
  getSchedule, 
  updateSchedule, 
  deleteSchedule, 
  runSchedule,
  type ExportSchedule,
  type ExportScheduleRequest,
  type ScheduleRunResult 
} from '@/api/schedule'
import type { Metadata } from '@/api/types'

export const useScheduleStore = defineStore('schedule', {
  state: () => ({
    // List state
    schedules: [] as ExportSchedule[],
    loading: false,
    error: '' as string | '',
    
    // Pagination and filtering
    page: 1,
    pageSize: 20,
    plugin: '' as string,
    enabled: undefined as boolean | undefined,
    meta: undefined as Metadata | undefined,
    
    // Current schedule state (for editing)
    currentSchedule: null as ExportSchedule | null,
    currentLoading: false,
    currentError: '' as string | '',
    
    // Running schedules state
    runningSchedules: new Set<string>(),
    recentRuns: new Map<string, ScheduleRunResult>(),
  }),

  getters: {
    /**
     * Get filtered schedules based on current filters
     */
    filteredSchedules: (state) => {
      let filtered = state.schedules
      
      if (state.plugin) {
        filtered = filtered.filter(s => 
          s.request.plugin_name.toLowerCase().includes(state.plugin.toLowerCase())
        )
      }
      
      if (state.enabled !== undefined) {
        filtered = filtered.filter(s => s.enabled === state.enabled)
      }
      
      return filtered
    },

    /**
     * Get schedules sorted by various criteria
     */
    schedulesSorted: (state) => {
      return [...state.schedules].sort((a, b) => {
        // First by enabled status (enabled first)
        if (a.enabled !== b.enabled) {
          return a.enabled ? -1 : 1
        }
        // Then by name
        return a.name.localeCompare(b.name)
      })
    },

    /**
     * Get schedule statistics
     */
    stats: (state) => {
      const total = state.schedules.length
      const enabled = state.schedules.filter(s => s.enabled).length
      const disabled = total - enabled
      const byPlugin = state.schedules.reduce((acc, s) => {
        const plugin = s.request.plugin_name
        acc[plugin] = (acc[plugin] || 0) + 1
        return acc
      }, {} as Record<string, number>)
      
      return { total, enabled, disabled, byPlugin }
    },

    /**
     * Check if a schedule is currently running
     */
    isScheduleRunning: (state) => (id: string) => {
      return state.runningSchedules.has(id)
    },

    /**
     * Get recent run result for a schedule
     */
    getRecentRun: (state) => (id: string) => {
      return state.recentRuns.get(id)
    },
  },

  actions: {
    /**
     * Fetch schedules list with current filters and pagination
     */
    async fetchSchedules() {
      this.loading = true
      this.error = ''
      
      try {
        const { schedules, meta } = await listSchedules({
          page: this.page,
          pageSize: this.pageSize,
          plugin: this.plugin || undefined,
          enabled: this.enabled,
        })
        
        this.schedules = schedules
        this.meta = meta
      } catch (e: any) {
        this.error = e?.message || 'Failed to load schedules'
        this.schedules = []
      } finally {
        this.loading = false
      }
    },

    /**
     * Create a new schedule
     */
    async createSchedule(request: ExportScheduleRequest): Promise<ExportSchedule> {
      this.currentLoading = true
      this.currentError = ''
      
      try {
        const schedule = await createSchedule(request)
        
        // Add to local list if it matches current filters
        this.schedules.unshift(schedule)
        
        return schedule
      } catch (e: any) {
        this.currentError = e?.message || 'Failed to create schedule'
        throw e
      } finally {
        this.currentLoading = false
      }
    },

    /**
     * Load a specific schedule for editing
     */
    async loadSchedule(id: string) {
      this.currentLoading = true
      this.currentError = ''
      this.currentSchedule = null
      
      try {
        const schedule = await getSchedule(id)
        this.currentSchedule = schedule
        return schedule
      } catch (e: any) {
        this.currentError = e?.message || 'Failed to load schedule'
        throw e
      } finally {
        this.currentLoading = false
      }
    },

    /**
     * Update an existing schedule
     */
    async updateSchedule(id: string, request: ExportScheduleRequest): Promise<ExportSchedule> {
      this.currentLoading = true
      this.currentError = ''
      
      try {
        const schedule = await updateSchedule(id, request)
        
        // Update in local list
        const index = this.schedules.findIndex(s => s.id === id)
        if (index !== -1) {
          this.schedules[index] = schedule
        }
        
        // Update current schedule if it matches
        if (this.currentSchedule?.id === id) {
          this.currentSchedule = schedule
        }
        
        return schedule
      } catch (e: any) {
        this.currentError = e?.message || 'Failed to update schedule'
        throw e
      } finally {
        this.currentLoading = false
      }
    },

    /**
     * Delete a schedule
     */
    async deleteSchedule(id: string) {
      try {
        await deleteSchedule(id)
        
        // Remove from local list
        this.schedules = this.schedules.filter(s => s.id !== id)
        
        // Clear current schedule if it matches
        if (this.currentSchedule?.id === id) {
          this.currentSchedule = null
        }
        
        // Clear any running state
        this.runningSchedules.delete(id)
        this.recentRuns.delete(id)
        
      } catch (e: any) {
        this.error = e?.message || 'Failed to delete schedule'
        throw e
      }
    },

    /**
     * Run a schedule immediately
     */
    async runScheduleNow(id: string): Promise<ScheduleRunResult> {
      this.runningSchedules.add(id)
      
      try {
        const result = await runSchedule(id)
        
        // Store the result for display
        this.recentRuns.set(id, result)
        
        // Update the schedule's last run time
        const schedule = this.schedules.find(s => s.id === id)
        if (schedule) {
          schedule.last_run = new Date().toISOString()
        }
        
        if (this.currentSchedule?.id === id) {
          this.currentSchedule.last_run = new Date().toISOString()
        }
        
        return result
      } catch (e: any) {
        this.error = e?.message || 'Failed to run schedule'
        throw e
      } finally {
        this.runningSchedules.delete(id)
      }
    },

    /**
     * Set filter values and reset pagination
     */
    setPlugin(value: string) {
      this.plugin = value
      this.page = 1
    },

    setEnabled(value: boolean | undefined) {
      this.enabled = value
      this.page = 1
    },

    setPage(page: number) {
      this.page = page
    },

    setPageSize(size: number) {
      this.pageSize = size
      this.page = 1
    },

    /**
     * Clear current schedule
     */
    clearCurrentSchedule() {
      this.currentSchedule = null
      this.currentError = ''
    },

    /**
     * Clear all errors
     */
    clearErrors() {
      this.error = ''
      this.currentError = ''
    },

    /**
     * Toggle schedule enabled state
     */
    async toggleScheduleEnabled(id: string) {
      const schedule = this.schedules.find(s => s.id === id)
      if (!schedule) return
      
      try {
        const updated = await updateSchedule(id, {
          name: schedule.name,
          interval_sec: schedule.interval_sec,
          enabled: !schedule.enabled,
          request: schedule.request,
        })
        
        // Update in local list
        const index = this.schedules.findIndex(s => s.id === id)
        if (index !== -1) {
          this.schedules[index] = updated
        }
        
        if (this.currentSchedule?.id === id) {
          this.currentSchedule = updated
        }
        
      } catch (e: any) {
        this.error = e?.message || 'Failed to toggle schedule'
        throw e
      }
    },
  },
})