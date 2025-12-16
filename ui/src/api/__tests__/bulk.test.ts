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
  bulkExport,
  bulkDriftDetect,
  bulkDriftDetectEnhanced,
  type BulkImportRequest,
  type BulkExportRequest,
  type BulkDriftDetectRequest,
  type BulkDriftDetectEnhancedRequest,
  type BulkOperationResult
} from '../bulk'

describe('Bulk Operations API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('bulkImport', () => {
    it('executes bulk import successfully', async () => {
      const request: BulkImportRequest = {
        deviceIds: [1, 2, 3],
        configurations: [
          { wifi: { ssid: 'Network1' } },
          { wifi: { ssid: 'Network2' } },
          { wifi: { ssid: 'Network3' } }
        ],
        options: {
          stopOnError: false,
          validateOnly: false
        }
      }
      const result: BulkOperationResult = {
        operationId: 'op-123',
        totalDevices: 3,
        successCount: 3,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        completedAt: '2023-01-01T00:01:00Z',
        results: [
          { deviceId: 1, deviceName: 'Device 1', status: 'success' },
          { deviceId: 2, deviceName: 'Device 2', status: 'success' },
          { deviceId: 3, deviceName: 'Device 3', status: 'success' }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkImport(request)

      expect(response).toEqual(result)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-import', request)
    })

    it('handles partial success', async () => {
      const request: BulkImportRequest = {
        deviceIds: [1, 2, 3],
        configurations: [{}, {}, {}]
      }
      const result: BulkOperationResult = {
        operationId: 'op-456',
        totalDevices: 3,
        successCount: 2,
        failureCount: 1,
        skippedCount: 0,
        status: 'partial',
        startedAt: '2023-01-01T00:00:00Z',
        completedAt: '2023-01-01T00:01:00Z',
        results: [
          { deviceId: 1, deviceName: 'Device 1', status: 'success' },
          { deviceId: 2, deviceName: 'Device 2', status: 'failed', error: 'Connection timeout' },
          { deviceId: 3, deviceName: 'Device 3', status: 'success' }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkImport(request)

      expect(response.status).toBe('partial')
      expect(response.successCount).toBe(2)
      expect(response.failureCount).toBe(1)
    })

    it('throws error when import fails', async () => {
      const request: BulkImportRequest = {
        deviceIds: [1],
        configurations: [{}]
      }
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid configuration format', code: 'VALIDATION_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(bulkImport(request)).rejects.toThrow('Invalid configuration format')
    })

    it('throws default error when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(
        bulkImport({ deviceIds: [1], configurations: [{}] })
      ).rejects.toThrow('Failed to execute bulk import')
    })
  })

  describe('bulkExport', () => {
    it('executes bulk export successfully', async () => {
      const request: BulkExportRequest = {
        deviceIds: [1, 2],
        options: {
          format: 'json',
          includeSecrets: false,
          includeMetadata: true
        }
      }
      const result: BulkOperationResult = {
        operationId: 'exp-789',
        totalDevices: 2,
        successCount: 2,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        completedAt: '2023-01-01T00:00:30Z',
        results: [
          {
            deviceId: 1,
            deviceName: 'Device 1',
            status: 'success',
            data: { wifi: { ssid: 'Network1' } }
          },
          {
            deviceId: 2,
            deviceName: 'Device 2',
            status: 'success',
            data: { wifi: { ssid: 'Network2' } }
          }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkExport(request)

      expect(response).toEqual(result)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-export', request)
    })

    it('handles different export formats', async () => {
      const yamlRequest: BulkExportRequest = {
        deviceIds: [1],
        options: { format: 'yaml' }
      }
      const result: BulkOperationResult = {
        operationId: 'exp-yaml',
        totalDevices: 1,
        successCount: 1,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        results: [{ deviceId: 1, deviceName: 'Device 1', status: 'success' }]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkExport(yamlRequest)

      expect(response.successCount).toBe(1)
    })

    it('throws error when export fails', async () => {
      const request: BulkExportRequest = {
        deviceIds: [999]
      }
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Device not found', code: 'NOT_FOUND' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(bulkExport(request)).rejects.toThrow('Device not found')
    })
  })

  describe('bulkDriftDetect', () => {
    it('executes bulk drift detection successfully', async () => {
      const request: BulkDriftDetectRequest = {
        deviceIds: [1, 2, 3],
        options: {
          stopOnError: false,
          detailedReport: true
        }
      }
      const result: BulkOperationResult = {
        operationId: 'drift-001',
        totalDevices: 3,
        successCount: 3,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        completedAt: '2023-01-01T00:02:00Z',
        results: [
          {
            deviceId: 1,
            deviceName: 'Device 1',
            status: 'success',
            data: { driftsFound: 0 }
          },
          {
            deviceId: 2,
            deviceName: 'Device 2',
            status: 'success',
            data: { driftsFound: 2 }
          },
          {
            deviceId: 3,
            deviceName: 'Device 3',
            status: 'success',
            data: { driftsFound: 1 }
          }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkDriftDetect(request)

      expect(response).toEqual(result)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-drift-detect', request)
    })

    it('handles devices with drift detected', async () => {
      const request: BulkDriftDetectRequest = {
        deviceIds: [1, 2]
      }
      const result: BulkOperationResult = {
        operationId: 'drift-002',
        totalDevices: 2,
        successCount: 2,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        results: [
          {
            deviceId: 1,
            deviceName: 'Device 1',
            status: 'success',
            data: {
              driftsFound: 3,
              drifts: [
                { field: 'wifi.ssid', expected: 'Network1', actual: 'Network2' },
                { field: 'power.default', expected: 'off', actual: 'on' },
                { field: 'mqtt.enabled', expected: true, actual: false }
              ]
            }
          },
          { deviceId: 2, deviceName: 'Device 2', status: 'success', data: { driftsFound: 0 } }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkDriftDetect(request)

      expect(response.results[0].data?.driftsFound).toBe(3)
    })

    it('throws error when detection fails', async () => {
      const request: BulkDriftDetectRequest = {
        deviceIds: [1]
      }
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'No baseline configuration', code: 'MISSING_BASELINE' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(bulkDriftDetect(request)).rejects.toThrow('No baseline configuration')
    })
  })

  describe('bulkDriftDetectEnhanced', () => {
    it('executes enhanced drift detection successfully', async () => {
      const request: BulkDriftDetectEnhancedRequest = {
        deviceIds: [1, 2],
        options: {
          stopOnError: false,
          detailedReport: true,
          includeHistory: true,
          compareWith: 'template',
          threshold: 'strict'
        }
      }
      const result: BulkOperationResult = {
        operationId: 'drift-enh-001',
        totalDevices: 2,
        successCount: 2,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        completedAt: '2023-01-01T00:03:00Z',
        results: [
          {
            deviceId: 1,
            deviceName: 'Device 1',
            status: 'success',
            data: {
              driftsFound: 2,
              history: [
                { date: '2023-01-01', driftCount: 2 },
                { date: '2022-12-31', driftCount: 1 }
              ]
            }
          },
          {
            deviceId: 2,
            deviceName: 'Device 2',
            status: 'success',
            data: { driftsFound: 0 }
          }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkDriftDetectEnhanced(request)

      expect(response).toEqual(result)
      expect(api.post).toHaveBeenCalledWith('/config/bulk-drift-detect-enhanced', request)
    })

    it('handles different comparison modes', async () => {
      const baselineRequest: BulkDriftDetectEnhancedRequest = {
        deviceIds: [1],
        options: { compareWith: 'baseline', threshold: 'moderate' }
      }
      const result: BulkOperationResult = {
        operationId: 'drift-baseline',
        totalDevices: 1,
        successCount: 1,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        results: [{ deviceId: 1, deviceName: 'Device 1', status: 'success' }]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkDriftDetectEnhanced(baselineRequest)

      expect(response.successCount).toBe(1)
    })

    it('handles peer comparison mode', async () => {
      const peerRequest: BulkDriftDetectEnhancedRequest = {
        deviceIds: [1, 2, 3],
        options: { compareWith: 'peer', threshold: 'relaxed' }
      }
      const result: BulkOperationResult = {
        operationId: 'drift-peer',
        totalDevices: 3,
        successCount: 3,
        failureCount: 0,
        skippedCount: 0,
        status: 'completed',
        startedAt: '2023-01-01T00:00:00Z',
        results: [
          {
            deviceId: 1,
            deviceName: 'Device 1',
            status: 'success',
            data: { driftsFound: 1, peerDeviation: 0.2 }
          },
          { deviceId: 2, deviceName: 'Device 2', status: 'success', data: { driftsFound: 0 } },
          {
            deviceId: 3,
            deviceName: 'Device 3',
            status: 'success',
            data: { driftsFound: 2, peerDeviation: 0.5 }
          }
        ]
      }
      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: result, timestamp: new Date().toISOString() }
      })

      const response = await bulkDriftDetectEnhanced(peerRequest)

      expect(response.totalDevices).toBe(3)
    })

    it('throws error when enhanced detection fails', async () => {
      const request: BulkDriftDetectEnhancedRequest = {
        deviceIds: [1]
      }
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid threshold value', code: 'VALIDATION_ERROR' },
          timestamp: new Date().toISOString()
        }
      })

      await expect(bulkDriftDetectEnhanced(request)).rejects.toThrow('Invalid threshold value')
    })

    it('throws default error when no data returned', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: true, timestamp: new Date().toISOString() }
      })

      await expect(bulkDriftDetectEnhanced({ deviceIds: [1] })).rejects.toThrow(
        'Failed to execute enhanced bulk drift detection'
      )
    })
  })
})
