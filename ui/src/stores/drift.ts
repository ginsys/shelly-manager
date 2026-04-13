import { ref, computed } from 'vue'
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
  resolveDriftTrend,
  generateDeviceDriftReport,
  computeDailyAggregates,
  type DriftSchedule,
  type DriftScheduleRun,
  type DriftReport,
  type DriftTrend,
  type DriftDailyAggregate,
  type CreateDriftScheduleRequest
} from '@/api/drift'
import type { Metadata } from '@/api/types'

export const useDriftStore = defineStore('drift', () => {
  const schedules = ref<DriftSchedule[]>([])
  const currentSchedule = ref<DriftSchedule | null>(null)
  const scheduleRuns = ref<DriftScheduleRun[]>([])
  const reports = ref<DriftReport[]>([])
  const trends = ref<DriftTrend[]>([])
  const trendAggregateDays = ref(30)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const scheduleMeta = ref<Metadata | undefined>(undefined)
  const reportMeta = ref<Metadata | undefined>(undefined)
  const runsMeta = ref<Metadata | undefined>(undefined)

  const trendAggregates = computed<DriftDailyAggregate[]>(() =>
    computeDailyAggregates(trends.value, trendAggregateDays.value)
  )

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
      if (index !== -1) schedules.value[index] = updated
      if (currentSchedule.value?.id === updated.id) currentSchedule.value = updated
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
      if (currentSchedule.value?.id === id) currentSchedule.value = null
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
      if (index !== -1) schedules.value[index] = toggled
      if (currentSchedule.value?.id === toggled.id) currentSchedule.value = toggled
      return toggled
    } catch (e: any) {
      error.value = e?.message || 'Failed to toggle drift schedule'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchScheduleRuns(scheduleId: number | string, limit = 50) {
    loading.value = true
    error.value = null
    try {
      const result = await getDriftScheduleRuns(scheduleId, limit)
      scheduleRuns.value = result.items
      runsMeta.value = result.meta
    } catch (e: any) {
      error.value = e?.message || 'Failed to load schedule runs'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchReports(limit = 50, deviceId?: number, reportType?: string) {
    loading.value = true
    error.value = null
    try {
      reports.value = await getDriftReports(limit, deviceId, reportType)
      reportMeta.value = undefined
    } catch (e: any) {
      error.value = e?.message || 'Failed to load drift reports'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Per-path drift items (DriftTrend rows). Populates `trends` which the
  // "Drift Reports" page renders and the "Drift Trends" page aggregates.
  async function fetchTrends(resolved?: boolean, limit = 500, deviceId?: number) {
    loading.value = true
    error.value = null
    try {
      trends.value = await getDriftTrends(limit, resolved, deviceId)
    } catch (e: any) {
      error.value = e?.message || 'Failed to load drift trends'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function resolveTrend(id: number) {
    loading.value = true
    error.value = null
    try {
      await resolveDriftTrend(id)
      const index = trends.value.findIndex(t => t.id === id)
      if (index !== -1) {
        trends.value[index] = {
          ...trends.value[index],
          resolved: true,
          resolved_at: new Date().toISOString()
        }
      }
    } catch (e: any) {
      error.value = e?.message || 'Failed to resolve drift item'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function generateReport(deviceId: number | string) {
    loading.value = true
    error.value = null
    try {
      const report = await generateDeviceDriftReport(deviceId)
      reports.value = [report, ...reports.value]
      return report
    } catch (e: any) {
      error.value = e?.message || 'Failed to generate device drift report'
      throw e
    } finally {
      loading.value = false
    }
  }

  function setTrendAggregateDays(days: number) {
    trendAggregateDays.value = days
  }

  function clearError() {
    error.value = null
  }

  function clearCurrentSchedule() {
    currentSchedule.value = null
  }

  return {
    schedules,
    currentSchedule,
    scheduleRuns,
    reports,
    trends,
    trendAggregates,
    trendAggregateDays,
    loading,
    error,
    scheduleMeta,
    reportMeta,
    runsMeta,
    fetchSchedules,
    fetchSchedule,
    create,
    update,
    remove,
    toggle,
    fetchScheduleRuns,
    fetchReports,
    fetchTrends,
    resolveTrend,
    generateReport,
    setTrendAggregateDays,
    clearError,
    clearCurrentSchedule
  }
})
