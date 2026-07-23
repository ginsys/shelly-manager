import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  listDriftSchedules,
  getDriftSchedule,
  deleteDriftSchedule,
  getDriftReports,
  getDriftTrends,
  resolveDriftTrend,
  generateDeviceDriftReport,
  computeDailyAggregates,
  type DriftSchedule,
  type DriftReport,
  type DriftTrend,
  type DriftDailyAggregate
} from '@/api/drift'
import type { Metadata } from '@/api/types'

export const useDriftStore = defineStore('drift', () => {
  const schedules = ref<DriftSchedule[]>([])
  const currentSchedule = ref<DriftSchedule | null>(null)
  const reports = ref<DriftReport[]>([])
  const trends = ref<DriftTrend[]>([])
  const trendAggregateDays = ref(30)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const scheduleMeta = ref<Metadata | undefined>(undefined)
  const reportMeta = ref<Metadata | undefined>(undefined)

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
    reports,
    trends,
    trendAggregates,
    trendAggregateDays,
    loading,
    error,
    scheduleMeta,
    reportMeta,
    fetchSchedules,
    fetchSchedule,
    remove,
    fetchReports,
    fetchTrends,
    resolveTrend,
    generateReport,
    setTrendAggregateDays,
    clearError,
    clearCurrentSchedule
  }
})
