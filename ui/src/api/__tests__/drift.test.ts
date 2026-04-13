import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn()
  }
}))

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
  resolveDriftTrend,
  generateDeviceDriftReport,
  computeDailyAggregates,
  type DriftSchedule,
  type DriftScheduleRun,
  type DriftTrend,
  type DriftReport,
  type CreateDriftScheduleRequest
} from '../drift'

function ok<T>(data: T) {
  return { data: { success: true, data, meta: { version: 'v1' }, timestamp: '2026-04-13T00:00:00Z' } }
}

function fail(message: string) {
  return { data: { success: false, error: { code: 'ERR', message }, meta: { version: 'v1' }, timestamp: '2026-04-13T00:00:00Z' } }
}

describe('Drift API client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('listDriftSchedules', () => {
    it('unwraps raw array at data (no schedules wrapper)', async () => {
      const schedules: DriftSchedule[] = [{
        id: 1, name: 'Daily', enabled: true, cron_spec: '0 0 * * *',
        device_ids: [1, 2], device_filter: null, last_run: null, next_run: null,
        run_count: 0, created_at: '2026-04-13T00:00:00Z', updated_at: '2026-04-13T00:00:00Z'
      }]
      vi.mocked(api.get).mockResolvedValueOnce(ok(schedules))

      const result = await listDriftSchedules()

      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules', {
        params: { page: 1, page_size: 25 }
      })
      expect(result.items).toEqual(schedules)
      expect(result.items[0].cron_spec).toBe('0 0 * * *')
    })

    it('returns empty items when data is absent', async () => {
      vi.mocked(api.get).mockResolvedValueOnce(ok(undefined))
      const result = await listDriftSchedules()
      expect(result.items).toEqual([])
    })

    it('throws when success is false', async () => {
      vi.mocked(api.get).mockResolvedValueOnce(fail('boom'))
      await expect(listDriftSchedules()).rejects.toThrow('boom')
    })
  })

  describe('getDriftSchedule', () => {
    it('fetches one schedule by id', async () => {
      const schedule: DriftSchedule = {
        id: 7, name: 'Hourly', enabled: true, cron_spec: '0 * * * *',
        run_count: 3, created_at: '2026-04-13T00:00:00Z', updated_at: '2026-04-13T00:00:00Z'
      }
      vi.mocked(api.get).mockResolvedValueOnce(ok(schedule))

      const result = await getDriftSchedule(7)

      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/7')
      expect(result.cron_spec).toBe('0 * * * *')
    })
  })

  describe('createDriftSchedule', () => {
    it('posts snake_case fields and returns created schedule', async () => {
      const req: CreateDriftScheduleRequest = {
        name: 'Every 6h', cron_spec: '0 */6 * * *', device_ids: [1, 2], enabled: true
      }
      vi.mocked(api.post).mockResolvedValueOnce(ok({ id: 42, ...req, run_count: 0, created_at: 'x', updated_at: 'x' }))

      const result = await createDriftSchedule(req)

      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules', req)
      expect(result.id).toBe(42)
      expect(result.cron_spec).toBe('0 */6 * * *')
    })

    it('throws when backend rejects', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(fail('invalid cron'))
      await expect(createDriftSchedule({ name: 'X', cron_spec: 'bad' })).rejects.toThrow('invalid cron')
    })
  })

  describe('updateDriftSchedule', () => {
    it('puts partial update', async () => {
      vi.mocked(api.put).mockResolvedValueOnce(ok({
        id: 1, name: 'Renamed', enabled: true, cron_spec: '0 0 * * *',
        run_count: 0, created_at: 'x', updated_at: 'x'
      }))

      const result = await updateDriftSchedule(1, { name: 'Renamed' })

      expect(api.put).toHaveBeenCalledWith('/config/drift-schedules/1', { name: 'Renamed' })
      expect(result.name).toBe('Renamed')
    })
  })

  describe('deleteDriftSchedule', () => {
    it('deletes and returns void', async () => {
      vi.mocked(api.delete).mockResolvedValueOnce(ok({ status: 'deleted' }))
      await expect(deleteDriftSchedule(5)).resolves.toBeUndefined()
      expect(api.delete).toHaveBeenCalledWith('/config/drift-schedules/5')
    })
  })

  describe('toggleDriftSchedule', () => {
    it('posts and returns toggled schedule', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(ok({
        id: 1, name: 'X', enabled: false, cron_spec: '0 0 * * *',
        run_count: 0, created_at: 'x', updated_at: 'x'
      }))
      const result = await toggleDriftSchedule(1)
      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules/1/toggle', {})
      expect(result.enabled).toBe(false)
    })
  })

  describe('getDriftScheduleRuns', () => {
    it('returns raw array unwrapped', async () => {
      const runs: DriftScheduleRun[] = [{
        id: 1, schedule_id: 1, status: 'completed',
        started_at: 'x', completed_at: 'y', created_at: 'x'
      }]
      vi.mocked(api.get).mockResolvedValueOnce(ok(runs))

      const result = await getDriftScheduleRuns(1, 10)

      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/1/runs', {
        params: { limit: 10 }
      })
      expect(result.items).toEqual(runs)
    })
  })

  describe('getDriftReports', () => {
    it('returns raw array of aggregate reports', async () => {
      const reports: DriftReport[] = [{
        id: 1, report_type: 'device', device_id: 5,
        summary: { total_devices: 1 }, devices: [], recommendations: [],
        generated_at: 'x', created_at: 'x'
      }]
      vi.mocked(api.get).mockResolvedValueOnce(ok(reports))

      const result = await getDriftReports(50, 5, 'device')

      expect(api.get).toHaveBeenCalledWith('/config/drift-reports', {
        params: { limit: 50, device_id: 5, type: 'device' }
      })
      expect(result).toEqual(reports)
    })
  })

  describe('getDriftTrends', () => {
    it('returns raw array of per-path trends', async () => {
      const trends: DriftTrend[] = [{
        id: 1, device_id: 5, path: 'wifi.ssid', category: 'network', severity: 'info',
        first_seen: '2026-04-10T00:00:00Z', last_seen: '2026-04-13T00:00:00Z',
        occurrences: 3, resolved: false, created_at: 'x', updated_at: 'x'
      }]
      vi.mocked(api.get).mockResolvedValueOnce(ok(trends))

      const result = await getDriftTrends(200, false, 5)

      expect(api.get).toHaveBeenCalledWith('/config/drift-trends', {
        params: { limit: 200, resolved: false, device_id: 5 }
      })
      expect(result[0].path).toBe('wifi.ssid')
    })
  })

  describe('resolveDriftTrend', () => {
    it('posts to drift-trends/{id}/resolve with empty body for Content-Type middleware', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(ok({ status: 'resolved' }))
      await resolveDriftTrend(42)
      expect(api.post).toHaveBeenCalledWith('/config/drift-trends/42/resolve', {})
    })

    it('throws on failure', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(fail('not found'))
      await expect(resolveDriftTrend(99)).rejects.toThrow('not found')
    })
  })

  describe('generateDeviceDriftReport', () => {
    it('posts and returns single aggregate report', async () => {
      const report: DriftReport = {
        id: 10, report_type: 'device', device_id: 5,
        summary: {}, devices: [], recommendations: [],
        generated_at: 'x', created_at: 'x'
      }
      vi.mocked(api.post).mockResolvedValueOnce(ok(report))

      const result = await generateDeviceDriftReport(5)

      expect(api.post).toHaveBeenCalledWith('/devices/5/drift-report', {})
      expect(result.id).toBe(10)
    })
  })

  describe('computeDailyAggregates', () => {
    const now = new Date('2026-04-13T12:00:00Z')
    beforeEach(() => vi.setSystemTime(now))

    it('buckets by last_seen date and counts resolved/unresolved', () => {
      const trends: DriftTrend[] = [
        { id: 1, device_id: 1, path: 'a', category: 'x', severity: 'low',
          first_seen: '2026-04-12T00:00:00Z', last_seen: '2026-04-12T09:00:00Z',
          occurrences: 1, resolved: false, created_at: 'x', updated_at: 'x' },
        { id: 2, device_id: 2, path: 'b', category: 'x', severity: 'high',
          first_seen: '2026-04-12T00:00:00Z', last_seen: '2026-04-12T10:00:00Z',
          occurrences: 2, resolved: true, resolved_at: '2026-04-12T11:00:00Z',
          created_at: 'x', updated_at: 'x' },
        { id: 3, device_id: 1, path: 'c', category: 'x', severity: 'medium',
          first_seen: '2026-04-13T00:00:00Z', last_seen: '2026-04-13T11:00:00Z',
          occurrences: 5, resolved: false, created_at: 'x', updated_at: 'x' }
      ]

      const result = computeDailyAggregates(trends, 7)

      expect(result).toHaveLength(2)
      expect(result[0]).toEqual({
        date: '2026-04-12',
        totalDrifts: 2,
        resolvedDrifts: 1,
        unresolvedDrifts: 1,
        deviceCount: 2
      })
      expect(result[1]).toEqual({
        date: '2026-04-13',
        totalDrifts: 1,
        resolvedDrifts: 0,
        unresolvedDrifts: 1,
        deviceCount: 1
      })
    })

    it('drops trends outside the cutoff window', () => {
      const trends: DriftTrend[] = [
        { id: 1, device_id: 1, path: 'old', category: 'x', severity: 'low',
          first_seen: '2026-01-01T00:00:00Z', last_seen: '2026-01-01T00:00:00Z',
          occurrences: 1, resolved: false, created_at: 'x', updated_at: 'x' },
        { id: 2, device_id: 1, path: 'new', category: 'x', severity: 'low',
          first_seen: '2026-04-13T00:00:00Z', last_seen: '2026-04-13T00:00:00Z',
          occurrences: 1, resolved: false, created_at: 'x', updated_at: 'x' }
      ]
      const result = computeDailyAggregates(trends, 7)
      expect(result.map(r => r.date)).toEqual(['2026-04-13'])
    })

    it('returns empty array when trends list is empty', () => {
      expect(computeDailyAggregates([], 30)).toEqual([])
    })
  })
})
