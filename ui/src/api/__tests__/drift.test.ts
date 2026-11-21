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
  deleteDriftSchedule,
  createDriftSchedule,
  updateDriftSchedule,
  StaleScheduleError,
  toggleDriftSchedule,
  listDriftScheduleRuns,
  getDriftSchedulerStatus,
  getDriftReports,
  getDriftTrends,
  resolveDriftTrend,
  generateDeviceDriftReport,
  computeDailyAggregates,
  type DriftSchedule,
  type DriftRun,
  type DriftTrend,
  type DriftReport
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
        run_count: 0, revision: 0, created_at: '2026-04-13T00:00:00Z', updated_at: '2026-04-13T00:00:00Z'
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
        run_count: 3, revision: 0, created_at: '2026-04-13T00:00:00Z', updated_at: '2026-04-13T00:00:00Z'
      }
      vi.mocked(api.get).mockResolvedValueOnce(ok(schedule))

      const result = await getDriftSchedule(7)

      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/7')
      expect(result.cron_spec).toBe('0 * * * *')
    })
  })

  describe('deleteDriftSchedule', () => {
    it('deletes and returns void', async () => {
      vi.mocked(api.delete).mockResolvedValueOnce(ok({ status: 'deleted' }))
      await expect(deleteDriftSchedule(5)).resolves.toBeUndefined()
      expect(api.delete).toHaveBeenCalledWith('/config/drift-schedules/5')
    })
  })

  describe('createDriftSchedule', () => {
    const input = { name: 'Nightly', cron_spec: '0 3 * * *', device_ids: [1, 2] }
    const stored: DriftSchedule = {
      id: 9, name: 'Nightly', enabled: true, cron_spec: '0 3 * * *', device_ids: [1, 2],
      run_count: 0, revision: 0, created_at: 'x', updated_at: 'x'
    }

    it('posts the input and returns the stored schedule', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(ok(stored))
      await expect(createDriftSchedule(input)).resolves.toEqual(stored)
      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules', input)
    })

    it('throws the server message, and a default when there is none or no data', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(fail('invalid cron spec'))
      await expect(createDriftSchedule(input)).rejects.toThrow('invalid cron spec')
      vi.mocked(api.post).mockResolvedValueOnce(ok(undefined))
      await expect(createDriftSchedule(input)).rejects.toThrow('Failed to create drift schedule')
    })
  })

  // axios rejects a 4xx answer before the envelope is read: the server's reason is in the error's response.
  describe('a refused write', () => {
    const refused = (status: number, error: Record<string, unknown>) =>
      Object.assign(new Error(`Request failed with status code ${status}`), {
        response: { status, data: { success: false, error, timestamp: 'x' } }
      })
    const input = { name: 'N', cron_spec: '0 * * * *' }

    it('throws the details of a 400 validation refusal, not "Validation failed"', async () => {
      vi.mocked(api.post).mockRejectedValueOnce(
        refused(400, { code: 'VALIDATION_FAILED', message: 'Validation failed', details: 'device 99 does not exist' })
      )
      await expect(createDriftSchedule(input)).rejects.toThrow('device 99 does not exist')
    })

    it('throws the message of a 404 on update, toggle and runs', async () => {
      vi.mocked(api.put).mockRejectedValueOnce(refused(404, { code: 'NOT_FOUND', message: 'Schedule not found' }))
      await expect(updateDriftSchedule(4, input, 0)).rejects.toThrow('Schedule not found')
      vi.mocked(api.post).mockRejectedValueOnce(refused(404, { code: 'NOT_FOUND', message: 'Schedule not found' }))
      await expect(toggleDriftSchedule(4)).rejects.toThrow('Schedule not found')
      vi.mocked(api.get).mockRejectedValueOnce(refused(404, { code: 'NOT_FOUND', message: 'Schedule not found' }))
      await expect(listDriftScheduleRuns(4)).rejects.toThrow('Schedule not found')
    })

    it('rethrows the original error when the answer carries no envelope', async () => {
      const original = Object.assign(new Error('Network Error'), { response: undefined })
      vi.mocked(api.post).mockRejectedValueOnce(original)
      await expect(createDriftSchedule(input)).rejects.toBe(original)
    })
  })

  describe('updateDriftSchedule', () => {
    const input = { name: 'Nightly', cron_spec: '0 3 * * *', device_filter: { type: 'plug' } }

    it('puts the whole input and the revision read to the schedule and returns the answer', async () => {
      const stored = { id: 4, ...input, enabled: true, run_count: 1, revision: 6, created_at: 'x', updated_at: 'y' }
      vi.mocked(api.put).mockResolvedValueOnce(ok(stored))
      await expect(updateDriftSchedule(4, input, 5)).resolves.toEqual(stored)
      expect(api.put).toHaveBeenCalledWith('/config/drift-schedules/4', { ...input, revision: 5 })
    })

    it('throws the server message, and a default when there is no data', async () => {
      vi.mocked(api.put).mockResolvedValueOnce(fail('schedule not found'))
      await expect(updateDriftSchedule(4, input, 0)).rejects.toThrow('schedule not found')
      vi.mocked(api.put).mockResolvedValueOnce(ok(undefined))
      await expect(updateDriftSchedule(4, input, 0)).rejects.toThrow('Failed to update drift schedule')
    })

    it('throws a StaleScheduleError with the current schedule on a 409', async () => {
      const current = { id: 4, name: 'Changed elsewhere', revision: 7 }
      vi.mocked(api.put).mockRejectedValueOnce(
        Object.assign(new Error('Request failed with status code 409'), {
          response: {
            status: 409,
            data: { success: false, error: { code: 'CONFLICT', message: 'drift schedule changed since it was read', details: current } }
          }
        })
      )
      const error = await updateDriftSchedule(4, input, 5).catch(e => e)
      expect(error).toBeInstanceOf(StaleScheduleError)
      expect(error.current).toEqual(current)
      expect(error.message).toBe('drift schedule changed since it was read')
    })

    it('treats a 409 with no schedule in it as an ordinary refusal', async () => {
      vi.mocked(api.put).mockRejectedValueOnce(
        Object.assign(new Error('x'), {
          response: { status: 409, data: { success: false, error: { code: 'CONFLICT', message: 'busy' } } }
        })
      )
      const error = await updateDriftSchedule(4, input, 5).catch(e => e)
      expect(error).not.toBeInstanceOf(StaleScheduleError)
      expect(error.message).toBe('busy')
    })

    it('treats a 409 whose details are not a schedule as an ordinary refusal', async () => {
      vi.mocked(api.put).mockRejectedValueOnce(
        Object.assign(new Error('x'), {
          response: { status: 409, data: { success: false, error: { code: 'CONFLICT', message: 'busy', details: { reason: 'locked' } } } }
        })
      )
      const error = await updateDriftSchedule(4, input, 5).catch(e => e)
      expect(error).not.toBeInstanceOf(StaleScheduleError)
      expect(error.message).toBe('busy')
    })
  })

  describe('toggleDriftSchedule', () => {
    it('posts an empty body to the toggle route and returns the schedule', async () => {
      const stored = { id: 4, name: 'N', enabled: false, cron_spec: '0 * * * *', run_count: 0, created_at: 'x', updated_at: 'y' }
      vi.mocked(api.post).mockResolvedValueOnce(ok(stored))
      await expect(toggleDriftSchedule(4)).resolves.toEqual(stored)
      expect(api.post).toHaveBeenCalledWith('/config/drift-schedules/4/toggle', {})
    })

    it('throws the server message, and a default when there is no data', async () => {
      vi.mocked(api.post).mockResolvedValueOnce(fail('gone'))
      await expect(toggleDriftSchedule(4)).rejects.toThrow('gone')
      vi.mocked(api.post).mockResolvedValueOnce(ok(undefined))
      await expect(toggleDriftSchedule(4)).rejects.toThrow('Failed to toggle drift schedule')
    })
  })

  describe('listDriftScheduleRuns', () => {
    const runs: DriftRun[] = [
      { id: 2, schedule_id: 4, status: 'skipped', started_at: 'b', created_at: 'b' },
      { id: 1, schedule_id: 4, status: 'completed', started_at: 'a', duration: 1_500_000_000, created_at: 'a' }
    ]

    it('asks for the server default when no limit is given', async () => {
      vi.mocked(api.get).mockResolvedValueOnce(ok(runs))
      await expect(listDriftScheduleRuns(4)).resolves.toEqual(runs)
      // axios leaves an undefined param out of the query string
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/4/runs', { params: { limit: undefined } })
    })

    it('sends the limit when one is given', async () => {
      vi.mocked(api.get).mockResolvedValueOnce(ok(runs))
      await listDriftScheduleRuns(4, 10)
      expect(api.get).toHaveBeenCalledWith('/config/drift-schedules/4/runs', { params: { limit: 10 } })
    })

    it('returns an empty list when data is absent and throws on a failed answer', async () => {
      vi.mocked(api.get).mockResolvedValueOnce(ok(undefined))
      await expect(listDriftScheduleRuns(4)).resolves.toEqual([])
      vi.mocked(api.get).mockResolvedValueOnce(fail('schedule not found'))
      await expect(listDriftScheduleRuns(4)).rejects.toThrow('schedule not found')
    })
  })

  describe('getDriftSchedulerStatus', () => {
    it.each([true, false])('returns enabled=%s from the status route', async (enabled) => {
      vi.mocked(api.get).mockResolvedValueOnce(ok({ enabled }))
      await expect(getDriftSchedulerStatus()).resolves.toBe(enabled)
      expect(api.get).toHaveBeenCalledWith('/config/drift-scheduler')
    })

    it('throws on a failed answer and on a body with no boolean', async () => {
      vi.mocked(api.get).mockResolvedValueOnce(fail('nope'))
      await expect(getDriftSchedulerStatus()).rejects.toThrow('nope')
      vi.mocked(api.get).mockResolvedValueOnce(ok({}))
      await expect(getDriftSchedulerStatus()).rejects.toThrow('Failed to read the drift scheduler state')
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
