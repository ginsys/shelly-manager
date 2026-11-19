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
  bulkImport,
  bulkDriftDetect,
  bulkDriftDetectEnhanced,
  bulkDownload,
  viewOf,
  type BulkDownloadFile,
  type BulkRequest,
  type BulkConfigResult,
  type BulkDriftResult,
  type BulkDriftReport
} from '../bulk'

// The answers below are copies of what the backend handlers return for a request naming one stored
// device and one id that names no device (internal/api/bulk_contract_test.go in the backend, the
// answer bodies as the handlers wrote them, with the volatile fields shortened). If a backend field
// is renamed, these are the fixtures to refresh.

const importAnswer: BulkConfigResult = {
  total: 2,
  success: 1,
  failed: 1,
  skipped: 0,
  results: [
    {
      device_id: 1,
      device_name: 'Test Device',
      device_ip: '127.0.0.1:42185',
      config_id: 1,
      status: 'success'
    },
    {
      device_id: 9999,
      device_name: '',
      device_ip: '',
      status: 'failed',
      error: 'device not found: record not found'
    }
  ]
}

// The same import with stop_on_error: the first device fails and the rest are skipped.
const importStopAnswer: BulkConfigResult = {
  total: 2,
  success: 0,
  failed: 1,
  skipped: 1,
  results: [
    {
      device_id: 9999,
      device_name: '',
      device_ip: '',
      status: 'failed',
      error: 'device not found: record not found'
    },
    { device_id: 1, device_name: 'Test Device', device_ip: '127.0.0.1:37707', status: 'skipped' }
  ]
}

const driftAnswer: BulkDriftResult = {
  total: 2,
  in_sync: 0,
  drifted: 1,
  errors: 1,
  skipped: 0,
  results: [
    {
      device_id: 1,
      device_name: 'Test Device',
      device_ip: '127.0.0.1:40987',
      status: 'drift',
      drift_summary: '1 configuration differences detected',
      difference_count: 1,
      drift: { device_id: 1, differences: [{ path: 'name', expected: 'exported', actual: null }] }
    },
    {
      device_id: 9999,
      device_name: '',
      device_ip: '',
      status: 'error',
      error: 'Device not found: record not found'
    }
  ],
  started_at: '2026-11-18T20:00:12.556576346+01:00',
  completed_at: '2026-11-18T20:00:12.558350728+01:00',
  duration: 1774384
}

const reportAnswer: BulkDriftReport = {
  id: 1,
  report_type: 'bulk',
  summary: {
    total_devices: 2,
    devices_in_sync: 0,
    devices_drifted: 1,
    devices_errored: 1
  },
  devices: [
    {
      device_id: 1,
      device_name: 'Test Device',
      device_ip: '127.0.0.1:42589',
      status: 'drift',
      total_differences: 1
    },
    {
      device_id: 9999,
      device_name: '',
      device_ip: '',
      status: 'error',
      total_differences: 0,
      error: 'Device not found: record not found'
    }
  ]
}

function answer(data: unknown) {
  ;(api.post as any).mockResolvedValue({
    data: { success: true, data, timestamp: new Date().toISOString() }
  })
}

const request: BulkRequest = { device_ids: [1, 9999], options: { stop_on_error: false } }

describe('Bulk Operations API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('bulkImport', () => {
    it('posts the wire request and returns the answer', async () => {
      answer(importAnswer)

      const response = await bulkImport(request)

      expect(response).toEqual(importAnswer)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-import', {
        device_ids: [1, 9999],
        options: { stop_on_error: false }
      })
    })

    it('throws the backend message when the answer is not a success', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Something failed', code: 'INTERNAL_SERVER_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(bulkImport(request)).rejects.toThrow('Something failed')
    })

    it('throws the default message when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(bulkImport(request)).rejects.toThrow('Failed to execute bulk import')
    })

    it('throws the reason of a refused request', async () => {
      ;(api.post as any).mockRejectedValue({
        message: 'Request failed with status code 400',
        response: {
          data: {
            success: false,
            error: {
              code: 'VALIDATION_FAILED',
              message: 'Validation failed',
              details: 'device_ids must name at least one device'
            }
          }
        }
      })

      await expect(bulkImport({ device_ids: [] })).rejects.toThrow(
        'device_ids must name at least one device'
      )
    })
  })

  it('sends the stop-on-error option as given', async () => {
    answer(importStopAnswer)

    const response = await bulkImport({
      device_ids: [9999, 1],
      options: { stop_on_error: true }
    })

    expect(response).toEqual(importStopAnswer)
    expect(api.post).toHaveBeenCalledWith('/config/bulk-import', {
      device_ids: [9999, 1],
      options: { stop_on_error: true }
    })
  })

  describe('bulkDriftDetect', () => {
    it('posts to the drift route and returns the answer', async () => {
      answer(driftAnswer)

      const response = await bulkDriftDetect(request)

      expect(response).toEqual(driftAnswer)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-drift-detect', request)
    })

    it('throws the default message when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(bulkDriftDetect(request)).rejects.toThrow(
        'Failed to execute bulk drift detection'
      )
    })
  })

  describe('bulkDriftDetectEnhanced', () => {
    it('posts to the enhanced route and returns the report', async () => {
      answer(reportAnswer)

      const response = await bulkDriftDetectEnhanced(request)

      expect(response).toEqual(reportAnswer)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-drift-detect-enhanced', request)
    })

    it('throws the default message when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(bulkDriftDetectEnhanced(request)).rejects.toThrow(
        'Failed to execute enhanced bulk drift detection'
      )
    })
  })

  describe('viewOf', () => {
    it('shows an import answer with its failed and skipped counts', () => {
      const view = viewOf(importStopAnswer)

      expect(view.stats).toEqual([
        { label: 'Success', value: 0, tone: 'success' },
        { label: 'Failed', value: 1, tone: 'failure' },
        { label: 'Skipped', value: 1, tone: 'skipped' }
      ])
      expect(view.rows).toEqual([
        {
          deviceId: 9999,
          deviceName: '',
          status: 'failed',
          tone: 'failure',
          message: 'device not found: record not found'
        },
        { deviceId: 1, deviceName: 'Test Device', status: 'skipped', tone: 'skipped', message: '' }
      ])
    })

    it('shows a drift answer', () => {
      const view = viewOf(driftAnswer)

      expect(view.stats.map((s) => [s.label, s.value])).toEqual([
        ['In sync', 0],
        ['Drifted', 1],
        ['Errors', 1],
        ['Skipped', 0]
      ])
      expect(view.rows[0]).toMatchObject({
        deviceId: 1,
        status: 'drift',
        tone: 'warning',
        message: '1 configuration differences detected'
      })
      expect(view.rows[1]).toMatchObject({
        status: 'error',
        tone: 'failure',
        message: 'Device not found: record not found'
      })
    })

    it('shows the comprehensive report', () => {
      const view = viewOf(reportAnswer)

      expect(view.stats.map((s) => [s.label, s.value])).toEqual([
        ['In sync', 0],
        ['Drifted', 1],
        ['Errors', 1]
      ])
      expect(view.rows[0]).toMatchObject({ status: 'drift', tone: 'warning', message: '1 difference(s)' })
      expect(view.rows[1]).toMatchObject({ status: 'error', tone: 'failure' })
    })
  })
})

// The file the route answers: the body itself, not the {success, data} envelope. A copy of what the
// backend writes for one readable device and one that cannot be read (bulk_download.go), with the
// secret already masked by the route.
const downloadFile: BulkDownloadFile = {
  generated_at: '2026-11-20T00:20:00Z',
  devices: [
    {
      device_id: 4,
      device_name: 'Kitchen',
      mac: 'AA:BB:CC:DD:EE:01',
      status: 'ok',
      config: { mqtt: { pass: '********' } }
    },
    {
      device_id: 9,
      device_name: 'Garage',
      status: 'error',
      error: 'no desired configuration is set for this device'
    }
  ]
}

const blobOf = (value: unknown) => new Blob([typeof value === 'string' ? value : JSON.stringify(value)])

describe('bulkDownload', () => {
  beforeEach(() => {
    ;(api.post as any).mockReset()
  })

  it('posts the ids to the route as a blob request and returns the file with the name the server sent', async () => {
    const blob = blobOf(downloadFile)
    ;(api.post as any).mockResolvedValue({
      data: blob,
      headers: { 'content-disposition': 'attachment; filename="shelly-configs-20261120-002000.json"' }
    })

    const answer = await bulkDownload({ device_ids: [4, 9] })

    expect(api.post).toHaveBeenCalledWith('/config/bulk-download', { device_ids: [4, 9] }, { responseType: 'blob' })
    expect(answer.blob).toBe(blob)
    expect(answer.filename).toBe('shelly-configs-20261120-002000.json')
    expect(answer.file).toEqual(downloadFile)
  })

  it('shows why the route refused: the details of its envelope, delivered as a Blob', async () => {
    ;(api.post as any).mockRejectedValue(
      Object.assign(new Error('Request failed with status code 400'), {
        response: {
          status: 400,
          data: blobOf({
            success: false,
            error: {
              code: 'VALIDATION_FAILED',
              message: 'Validation failed',
              details: 'bulk-download takes at most 10 devices, got 12'
            }
          })
        }
      })
    )

    const err = await bulkDownload({ device_ids: [1] }).catch((e) => e)

    expect(err.message).toBe('bulk-download takes at most 10 devices, got 12')
  })

  it('rethrows the original error when the body is not an error envelope', async () => {
    const original = Object.assign(new Error('Network Error'), { response: { data: blobOf('not json') } })
    ;(api.post as any).mockRejectedValue(original)

    await expect(bulkDownload({ device_ids: [1] })).rejects.toBe(original)
  })

  it('refuses an answer that is not the file', async () => {
    ;(api.post as any).mockResolvedValue({ data: blobOf('<html>not the file</html>'), headers: {} })

    await expect(bulkDownload({ device_ids: [1] })).rejects.toThrow(/not the configuration file/)
  })
})
