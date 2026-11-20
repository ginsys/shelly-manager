import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client used by export API
vi.mock('./client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import api from './client'
import { 
  createBackup, 
  getBackupResult, 
  downloadBackup, 
  listBackups, 
  getBackupStatistics, 
  deleteBackup,
  previewRestore,
  buildSMAExportConfig,
  type BackupRequest 
} from './export'

describe('backup api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('builds SMA plugin config without removed or inert options', () => {
    expect(buildSMAExportConfig('/exports', {
      compression_level: 7,
      include_discovered: false,
      exclude_sensitive: true,
    })).toEqual({
      output_path: '/exports',
      compression_level: 7,
      include_discovered: false,
      exclude_sensitive: true,
    })
  })

  describe('createBackup', () => {
    it('should create a backup successfully', async () => {
      const mockRequest: BackupRequest = {
        name: 'Test Backup',
        description: 'Test description',
        format: 'json',
        include_settings: true,
        include_schedules: true,
        include_metrics: false
      }

      const mockResponse = {
        data: {
          success: true,
          data: { backup_id: 'backup-123' },
          timestamp: new Date().toISOString()
        }
      }

      ;(api.post as any).mockResolvedValue(mockResponse)

      const result = await createBackup(mockRequest)
      
      expect(result.backup_id).toBe('backup-123')
      expect(api.post).toHaveBeenCalledWith('/export/backup', mockRequest)
    })

    it('should handle backup creation errors', async () => {
      const mockRequest: BackupRequest = {
        name: 'Test Backup',
        format: 'json'
      }

      const mockResponse = {
        data: {
          success: false,
          error: { message: 'Backup creation failed' }
        }
      }

      ;(api.post as any).mockResolvedValue(mockResponse)

      await expect(createBackup(mockRequest)).rejects.toThrow('Backup creation failed')
    })
  })

  describe('getBackupResult', () => {
    it('should return backup result', async () => {
      const mockResult = {
        backup_id: 'backup-123',
        name: 'Test Backup',
        format: 'json',
        device_count: 5,
        file_size: 1024,
        encrypted: false,
        checksum: 'abc123'
      }

      const mockResponse = {
        data: {
          success: true,
          data: mockResult,
          timestamp: new Date().toISOString()
        }
      }

      ;(api.get as any).mockResolvedValue(mockResponse)

      const result = await getBackupResult('backup-123')
      
      expect(result).toEqual(mockResult)
      expect(api.get).toHaveBeenCalledWith('/export/backup/backup-123')
    })
  })

  describe('downloadBackup', () => {
    it('should download backup as blob', async () => {
      const mockBlob = new Blob(['test data'], { type: 'application/zip' })
      
      ;(api.get as any).mockResolvedValue({ data: mockBlob })

      const result = await downloadBackup('backup-123')
      
      expect(result).toBe(mockBlob)
      expect(api.get).toHaveBeenCalledWith('/export/backup/backup-123/download', {
        responseType: 'blob'
      })
    })
  })

  describe('listBackups', () => {
    it('should list backups with filters', async () => {
      const mockBackups = [
        {
          id: 1,
          backup_id: 'backup-123',
          name: 'Test Backup',
          format: 'json',
          device_count: 5,
          file_size: 1024,
          encrypted: false,
          success: true,
          created_at: new Date().toISOString()
        }
      ]

      const mockResponse = {
        data: {
          success: true,
          data: { backups: mockBackups },
          meta: { pagination: { page: 1, page_size: 20, total_pages: 1, has_next: false, has_previous: false } },
          timestamp: new Date().toISOString()
        }
      }

      ;(api.get as any).mockResolvedValue(mockResponse)

      const result = await listBackups({ 
        page: 1, 
        pageSize: 20, 
        format: 'json', 
        success: true 
      })
      
      expect(result.items).toEqual(mockBackups)
      expect(result.meta?.pagination?.page).toBe(1)
      expect(api.get).toHaveBeenCalledWith('/export/backups', {
        params: { page: 1, page_size: 20, success: true, format: 'json' }
      })
    })

    it('should handle empty backup list', async () => {
      const mockResponse = {
        data: {
          success: true,
          data: { backups: [] },
          timestamp: new Date().toISOString()
        }
      }

      ;(api.get as any).mockResolvedValue(mockResponse)

      const result = await listBackups()
      
      expect(result.items).toEqual([])
    })
  })

  describe('getBackupStatistics', () => {
    it('should return backup statistics', async () => {
      const mockStats = {
        total: 10,
        success: 8,
        failure: 2,
        total_size: 10485760,
        by_format: { json: 5, sma: 3, yaml: 2 },
        last_backup: new Date().toISOString()
      }

      const mockResponse = {
        data: {
          success: true,
          data: mockStats,
          timestamp: new Date().toISOString()
        }
      }

      ;(api.get as any).mockResolvedValue(mockResponse)

      const result = await getBackupStatistics()
      
      expect(result).toEqual(mockStats)
      expect(api.get).toHaveBeenCalledWith('/export/backup-statistics')
    })
  })

  describe('deleteBackup', () => {
    it('should delete backup successfully', async () => {
      const mockResponse = {
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      }

      ;(api.delete as any).mockResolvedValue(mockResponse)

      await deleteBackup('backup-123')
      
      expect(api.delete).toHaveBeenCalledWith('/export/backup/backup-123')
    })

    it('should handle delete errors', async () => {
      const mockResponse = {
        data: {
          success: false,
          error: { message: 'Backup not found' }
        }
      }

      ;(api.delete as any).mockResolvedValue(mockResponse)

      await expect(deleteBackup('backup-123')).rejects.toThrow('Backup not found')
    })
  })

  describe('previewRestore', () => {
    it('posts only the backup id and returns the check result', async () => {
      const mockPreview = {
        backup_id: 'backup-123',
        name: 'nightly',
        valid: true,
        file_size: 4096,
        record_count: 12,
        created_at: '2026-11-20T08:15:00Z',
        warnings: ["Only the file's presence and size were checked; its contents were not read."]
      }

      ;(api.post as any).mockResolvedValue({
        data: { success: true, data: mockPreview, timestamp: new Date().toISOString() }
      })

      const result = await previewRestore('backup-123')

      expect(result).toEqual(mockPreview)
      expect(api.post).toHaveBeenCalledWith('/import/restore-preview', { backup_id: 'backup-123' })
    })

    it('throws the server message when the check is refused', async () => {
      ;(api.post as any).mockResolvedValue({
        data: { success: false, error: { message: 'The backup file is not available' } }
      })

      await expect(previewRestore('backup-123')).rejects.toThrow('The backup file is not available')
    })
  })

  it('offers no restore execution', async () => {
    const mod = await import('./export')
    expect('executeRestore' in mod).toBe(false)
    expect('getRestoreResult' in mod).toBe(false)
  })
})
