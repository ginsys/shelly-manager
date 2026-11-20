import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  listDriftSchedules,
  getDriftSchedule,
  deleteDriftSchedule,
  createDriftSchedule,
  updateDriftSchedule,
  toggleDriftSchedule,
  listDriftScheduleRuns,
  getDriftSchedulerStatus,
  getDriftReports,
  getDriftTrends,
  resolveDriftTrend,
  generateDeviceDriftReport,
  computeDailyAggregates,
  type DriftSchedule,
  type DriftScheduleInput,
  type DriftRun,
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
  // Whether the server runs the scheduler: null until it was read, and after a read that failed.
  const schedulerEnabled = ref<boolean | null>(null)

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

  // Rethrows a failed read and leaves `error` alone: the page shows its own banner for it.
  async function fetchSchedulerStatus() {
    try {
      schedulerEnabled.value = await getDriftSchedulerStatus()
    } catch (e) {
      schedulerEnabled.value = null
      throw e
    }
  }

  // An action (remove, resolveTrend, generateReport) rethrows its failure and leaves `error` and the lists
  // alone: `error` is the failure of the last list load, and the page that acted shows the failure of its
  // own action (ginsys/shelly-manager#701).
  async function remove(id: number | string) {
    loading.value = true
    try {
      await deleteDriftSchedule(id)
      schedules.value = schedules.value.filter(s => s.id !== id)
      if (currentSchedule.value?.id === id) currentSchedule.value = null
    } finally {
      loading.value = false
    }
  }

  // Put the schedule the server answered into the list: replace its row, or add it at the end.
  function putSchedule(schedule: DriftSchedule) {
    const index = schedules.value.findIndex(s => s.id === schedule.id)
    if (index === -1) schedules.value = [...schedules.value, schedule]
    else schedules.value = schedules.value.map((s, i) => (i === index ? schedule : s))
    if (currentSchedule.value?.id === schedule.id) currentSchedule.value = schedule
    return schedule
  }

  // create, update and toggle follow the same convention as remove: a failure is rethrown with the
  // server's text and `error` and the list stay as they were; a success replaces the one row.
  async function create(input: DriftScheduleInput) {
    return putSchedule(await createDriftSchedule(input))
  }

  async function update(id: number | string, input: DriftScheduleInput) {
    return putSchedule(await updateDriftSchedule(id, input))
  }

  async function toggle(id: number | string) {
    return putSchedule(await toggleDriftSchedule(id))
  }

  // The runs of one schedule are not kept here: the page holds them per card.
  async function fetchRuns(id: number | string, limit?: number): Promise<DriftRun[]> {
    return listDriftScheduleRuns(id, limit)
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
    } finally {
      loading.value = false
    }
  }

  async function generateReport(deviceId: number | string) {
    loading.value = true
    try {
      const report = await generateDeviceDriftReport(deviceId)
      reports.value = [report, ...reports.value]
      return report
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
    schedulerEnabled,
    fetchSchedules,
    fetchSchedule,
    fetchSchedulerStatus,
    remove,
    create,
    update,
    toggle,
    fetchRuns,
    fetchReports,
    fetchTrends,
    resolveTrend,
    generateReport,
    setTrendAggregateDays,
    clearError,
    clearCurrentSchedule
  }
})
