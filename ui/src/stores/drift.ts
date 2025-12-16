import { ref } from 'vue'
import { defineStore } from 'pinia'
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
} from '@/api/drift'
import type { Metadata } from '@/api/types'

export const useDriftStore = defineStore('drift', () => {
  // State
  const schedules = ref<DriftSchedule[]>([])
  const currentSchedule = ref<DriftSchedule | null>(null)
  const scheduleRuns = ref<DriftScheduleRun[]>([])
  const reports = ref<DriftReport[]>([])
  const trends = ref<DriftTrend[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const scheduleMeta = ref<Metadata | undefined>(undefined)
  const reportMeta = ref<Metadata | undefined>(undefined)
  const runsMeta = ref<Metadata | undefined>(undefined)

  // Actions - Drift Schedules
  async function fetchSchedules(page = 1, pageSize = 25) {
    loading.value = true
    error.value = null
    try {
      const result = await listDriftSchedules(page, pageSize)
      schedules.value = result.items
      scheduleMeta.value = result.meta
    } catch (e: any) {
      error.value = e?.message || 'Failed to load drift schedules'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchSchedule(id: number | string) {
    loading.value = true
    error.value = null
    try {
      currentSchedule.value = await getDriftSchedule(id)
      return currentSchedule.value
    } catch (e: any) {
      error.value = e?.message || 'Failed to load drift schedule'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function create(data: CreateDriftScheduleRequest) {
    loading.value = true
    error.value = null
    try {
      const newSchedule = await createDriftSchedule(data)
      schedules.value.unshift(newSchedule)
      return newSchedule
    } catch (e: any) {
      error.value = e?.message || 'Failed to create drift schedule'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function update(id: number | string, data: Partial<CreateDriftScheduleRequest>) {
    loading.value = true
    error.value = null
    try {
      const updated = await updateDriftSchedule(id, data)
      const index = schedules.value.findIndex(s => s.id === updated.id)
      if (index !== -1) {
        schedules.value[index] = updated
      }
      if (currentSchedule.value?.id === updated.id) {
        currentSchedule.value = updated
      }
      return updated
    } catch (e: any) {
      error.value = e?.message || 'Failed to update drift schedule'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function remove(id: number | string) {
    loading.value = true
    error.value = null
    try {
      await deleteDriftSchedule(id)
      schedules.value = schedules.value.filter(s => s.id !== id)
      if (currentSchedule.value?.id === id) {
        currentSchedule.value = null
      }
    } catch (e: any) {
      error.value = e?.message || 'Failed to delete drift schedule'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function toggle(id: number | string) {
    loading.value = true
    error.value = null
    try {
      const toggled = await toggleDriftSchedule(id)
      const index = schedules.value.findIndex(s => s.id === toggled.id)
      if (index !== -1) {
        schedules.value[index] = toggled
      }
      if (currentSchedule.value?.id === toggled.id) {
        currentSchedule.value = toggled
      }
      return toggled
    } catch (e: any) {
      error.value = e?.message || 'Failed to toggle drift schedule'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Actions - Schedule Runs
  async function fetchScheduleRuns(scheduleId: number | string, page = 1, pageSize = 25) {
    loading.value = true
    error.value = null
    try {
      const result = await getDriftScheduleRuns(scheduleId, page, pageSize)
      scheduleRuns.value = result.items
      runsMeta.value = result.meta
    } catch (e: any) {
      error.value = e?.message || 'Failed to load schedule runs'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Actions - Drift Reports
  async function fetchReports(page = 1, pageSize = 25, resolved?: boolean) {
    loading.value = true
    error.value = null
    try {
      const result = await getDriftReports(page, pageSize, resolved)
      reports.value = result.items
      reportMeta.value = result.meta
    } catch (e: any) {
      error.value = e?.message || 'Failed to load drift reports'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function resolveReport(id: number | string, notes?: string) {
    loading.value = true
    error.value = null
    try {
      const resolved = await resolveDriftReport(id, notes)
      const index = reports.value.findIndex(r => r.id === resolved.id)
      if (index !== -1) {
        reports.value[index] = resolved
      }
      return resolved
    } catch (e: any) {
      error.value = e?.message || 'Failed to resolve drift report'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function generateReport(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      const deviceReports = await generateDeviceDriftReport(deviceId)
      // Add new reports to the list
      reports.value = [...deviceReports, ...reports.value]
      return deviceReports
    } catch (e: any) {
      error.value = e?.message || 'Failed to generate device drift report'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Actions - Drift Trends
  async function fetchTrends(days = 30) {
    loading.value = true
    error.value = null
    try {
      trends.value = await getDriftTrends(days)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load drift trends'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Utility actions
  function clearError() {
    error.value = null
  }

  function clearCurrentSchedule() {
    currentSchedule.value = null
  }

  return {
    // State
    schedules,
    currentSchedule,
    scheduleRuns,
    reports,
    trends,
    loading,
    error,
    scheduleMeta,
    reportMeta,
    runsMeta,
    // Actions
    fetchSchedules,
    fetchSchedule,
    create,
    update,
    remove,
    toggle,
    fetchScheduleRuns,
    fetchReports,
    resolveReport,
    generateReport,
    fetchTrends,
    clearError,
    clearCurrentSchedule
  }
})
