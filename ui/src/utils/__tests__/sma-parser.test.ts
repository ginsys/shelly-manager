import { describe, it, expect, vi, beforeEach } from 'vitest'
import pako from 'pako'
import {
  parseSMAFile,
  parseSMAFromFile,
  validateSMAStructure,
  extractSMAMetadata,
  getSMAImportSections,
  filterSMAArchive,
  isSMAParsingSupported,
  type SMAArchive
} from '../sma-parser'

// Mock crypto-browserify
vi.mock('crypto-browserify', () => ({
  createHash: vi.fn(() => ({
    update: vi.fn(),
    digest: vi.fn(() => 'mockedchecksum')
  }))
}))

// Mock pako
vi.mock('pako', () => ({
  default: {
    gunzip: vi.fn()
  }
}))

const mockPako = vi.mocked(pako)

describe('SMA Parser', () => {
  const mockSMAArchive: SMAArchive = {
    sma_version: '1.0',
    format_version: '2024.1',
    metadata: {
      export_id: 'test-export-123',
      created_at: '2024-01-15T10:30:00Z',
      created_by: 'test@example.com',
      export_type: 'manual',
      system_info: {
        version: 'v0.5.4-alpha',
        database_type: 'sqlite',
        hostname: 'test-host',
        total_size_bytes: 12345,
        compression_ratio: 0.35
      },
      integrity: {
        checksum: 'sha256:mockedchecksum',
        record_count: 3,
        file_count: 5
      }
    },
    devices: [
      {
        id: 1,
        mac: 'AA:BB:CC:DD:EE:01',
        ip: '192.168.1.100',
        type: 'shelly1',
        name: 'Test Device 1',
        status: 'online',
        last_seen: '2024-01-15T10:25:00Z',
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-15T10:00:00Z'
      }
    ],
    templates: [
      {
        id: 1,
        name: 'Test Template',
        description: 'Test description',
        device_type: 'shelly1',
        generation: 1,
        config: { test: 'config' },
        is_default: true,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-15T10:00:00Z'
      }
    ],
    discovered_devices: [
      {
        mac: 'AA:BB:CC:DD:EE:02',
        ssid: 'ShellyTest-123',
        model: 'SHSW-1',
        generation: 1,
        ip: '192.168.1.101',
        signal: -45,
        agent_id: 'agent-001',
        discovered: '2024-01-15T10:20:00Z'
      }
    ],
    network_settings: {
      wifi_networks: [
        { ssid: 'TestNetwork', security: 'WPA2', priority: 1 }
      ],
      mqtt_config: {
        server: 'mqtt.test.local',
        port: 1883,
        username: 'test',
        retain: false,
        qos: 0
      },
      ntp_servers: ['pool.ntp.org']
    },
    plugin_configurations: [
      {
        plugin_name: 'test-plugin',
        version: '1.0.0',
        config: { enabled: true },
        enabled: true
      }
    ],
    system_settings: {
      log_level: 'info',
      api_settings: {
        rate_limit: 100,
        cors_enabled: true
      },
      database_settings: {
        connection_pool_size: 10,
        query_timeout: '30s'
      }
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
    // Setup default pako mock behavior
    mockPako.gunzip.mockReturnValue(JSON.stringify(mockSMAArchive))
  })

  describe('parseSMAFile', () => {
    it('should parse valid SMA file successfully', async () => {
      const buffer = new ArrayBuffer(100)
      
      const result = await parseSMAFile(buffer)

      expect(result.success).toBe(true)
      expect(result.archive).toEqual(mockSMAArchive)
      expect(result.parseInfo.compressedSize).toBe(100)
      expect(result.parseInfo.parseTimeMs).toBeGreaterThan(0)
    })

    it('should fail when file exceeds maximum size', async () => {
      const buffer = new ArrayBuffer(1000)
      
      const result = await parseSMAFile(buffer, { maxSizeBytes: 500 })

      expect(result.success).toBe(false)
      expect(result.error).toContain('exceeds maximum allowed size')
    })

    it('should fail when decompression fails', async () => {
      mockPako.gunzip.mockImplementation(() => {
        throw new Error('Invalid gzip data')
      })
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Failed to decompress SMA file')
    })

    it('should fail when JSON parsing fails', async () => {
      mockPako.gunzip.mockReturnValue('invalid json {')
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Failed to parse JSON content')
    })

    it('should validate checksum when enabled', async () => {
      // Mock wrong checksum
      const wrongArchive = { ...mockSMAArchive }
      wrongArchive.metadata.integrity.checksum = 'sha256:wrongchecksum'
      mockPako.gunzip.mockReturnValue(JSON.stringify(wrongArchive))
      
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer, { validateChecksum: true })

      expect(result.success).toBe(false)
      expect(result.error).toContain('Checksum validation failed')
    })

    it('should skip checksum validation when disabled', async () => {
      const wrongArchive = { ...mockSMAArchive }
      wrongArchive.metadata.integrity.checksum = 'sha256:wrongchecksum'
      mockPako.gunzip.mockReturnValue(JSON.stringify(wrongArchive))
      
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer, { validateChecksum: false })

      expect(result.success).toBe(true)
    })

    it('should validate structure when enabled', async () => {
      const invalidArchive = { ...mockSMAArchive }
      delete (invalidArchive as any).sma_version
      mockPako.gunzip.mockReturnValue(JSON.stringify(invalidArchive))
      
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer, { 
        validateStructure: true, 
        validateChecksum: false // Disable checksum to test structure only
      })

      expect(result.success).toBe(false)
      expect(result.error).toContain('Structure validation failed')
    })
  })

  describe('parseSMAFromFile', () => {
    it('should parse File object successfully', async () => {
      // Reset the mock to ensure clean test
      mockPako.gunzip.mockReturnValue(JSON.stringify(mockSMAArchive))
      
      const mockFile = {
        arrayBuffer: vi.fn().mockResolvedValue(new ArrayBuffer(100)),
        size: 100
      } as unknown as File

      const result = await parseSMAFromFile(mockFile, { validateChecksum: false })

      expect(result.success).toBe(true)
      expect(mockFile.arrayBuffer).toHaveBeenCalled()
    })

    it('should handle file reading error', async () => {
      const mockFile = {
        arrayBuffer: vi.fn().mockRejectedValue(new Error('File read error')),
        size: 100
      } as unknown as File

      const result = await parseSMAFromFile(mockFile)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Failed to read file')
    })
  })

  describe('validateSMAStructure', () => {
    it('should validate valid structure', () => {
      const result = validateSMAStructure(mockSMAArchive)

      expect(result.valid).toBe(true)
      expect(result.errors).toHaveLength(0)
      expect(result.summary.deviceCount).toBe(1)
      expect(result.summary.templateCount).toBe(1)
      expect(result.summary.discoveredDeviceCount).toBe(1)
      expect(result.summary.estimatedDataIntegrity).toBeGreaterThan(0)
    })

    it('should detect missing required fields', () => {
      const invalidArchive = { ...mockSMAArchive }
      delete (invalidArchive as any).sma_version
      delete (invalidArchive as any).metadata

      const result = validateSMAStructure(invalidArchive)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('Missing sma_version')
      expect(result.errors).toContain('Missing metadata')
    })

    it('should warn about version compatibility', () => {
      const newerArchive = { ...mockSMAArchive }
      newerArchive.sma_version = '2.0'

      const result = validateSMAStructure(newerArchive)

      expect(result.valid).toBe(true)
      expect(result.warnings.some(w => w.includes('newer than supported version'))).toBe(true)
    })

    it('should validate device-template references', () => {
      const archiveWithBadRef = { ...mockSMAArchive }
      archiveWithBadRef.devices[0].configuration = {
        template_id: 999, // Non-existent template
        sync_status: 'synced'
      }

      const result = validateSMAStructure(archiveWithBadRef)

      expect(result.valid).toBe(true)
      expect(result.warnings.some(w => w.includes('non-existent template ID: 999'))).toBe(true)
    })

    it('should validate record count', () => {
      const archiveWithWrongCount = { ...mockSMAArchive }
      archiveWithWrongCount.metadata.integrity.record_count = 10 // Should be 3

      const result = validateSMAStructure(archiveWithWrongCount)

      expect(result.valid).toBe(true)
      expect(result.warnings.some(w => w.includes('Record count mismatch'))).toBe(true)
    })
  })

  describe('extractSMAMetadata', () => {
    it('should extract metadata successfully', async () => {
      const buffer = new ArrayBuffer(100)

      const result = await extractSMAMetadata(buffer)

      expect(result.success).toBe(true)
      expect(result.metadata).toEqual(mockSMAArchive.metadata)
    })

    it('should handle missing metadata', async () => {
      const archiveWithoutMetadata = { ...mockSMAArchive }
      delete (archiveWithoutMetadata as any).metadata
      mockPako.gunzip.mockReturnValue(JSON.stringify(archiveWithoutMetadata))
      
      const buffer = new ArrayBuffer(100)

      const result = await extractSMAMetadata(buffer)

      expect(result.success).toBe(false)
      expect(result.error).toContain('No metadata found')
    })

    it('should handle decompression error', async () => {
      mockPako.gunzip.mockImplementation(() => {
        throw new Error('Decompression failed')
      })
      const buffer = new ArrayBuffer(100)

      const result = await extractSMAMetadata(buffer)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Failed to extract metadata')
    })
  })

  describe('getSMAImportSections', () => {
    it('should identify all available sections', () => {
      const sections = getSMAImportSections(mockSMAArchive)

      expect(sections).toContain('devices')
      expect(sections).toContain('templates')
      expect(sections).toContain('discovered_devices')
      expect(sections).toContain('network_settings')
      expect(sections).toContain('plugin_configurations')
      expect(sections).toContain('system_settings')
    })

    it('should only include sections with data', () => {
      const emptyArchive = { ...mockSMAArchive }
      emptyArchive.devices = []
      emptyArchive.templates = []

      const sections = getSMAImportSections(emptyArchive)

      expect(sections).not.toContain('devices')
      expect(sections).not.toContain('templates')
      expect(sections).toContain('discovered_devices') // Still has data
    })
  })

  describe('filterSMAArchive', () => {
    it('should filter archive to selected sections', () => {
      const filtered = filterSMAArchive(mockSMAArchive, ['devices', 'templates'])

      expect(filtered.devices).toEqual(mockSMAArchive.devices)
      expect(filtered.templates).toEqual(mockSMAArchive.templates)
      expect(filtered.discovered_devices).toBeUndefined()
      expect(filtered.network_settings).toBeUndefined()
    })

    it('should preserve metadata and version info', () => {
      const filtered = filterSMAArchive(mockSMAArchive, ['devices'])

      expect(filtered.sma_version).toBe(mockSMAArchive.sma_version)
      expect(filtered.format_version).toBe(mockSMAArchive.format_version)
      expect(filtered.metadata?.export_id).toBe(mockSMAArchive.metadata.export_id)
    })

    it('should recalculate record count for filtered data', () => {
      const filtered = filterSMAArchive(mockSMAArchive, ['devices']) // Only 1 device

      expect(filtered.metadata?.integrity.record_count).toBe(1)
    })
  })

  describe('isSMAParsingSupported', () => {
    it('should return true when all dependencies are available', () => {
      // Dependencies are already available in test environment
      const supported = isSMAParsingSupported()

      expect(supported).toBe(true)
    })

    it('should return false when dependencies are missing', () => {
      // Use vi.stubGlobal to mock missing dependencies
      vi.stubGlobal('crypto', undefined)
      
      const supported = isSMAParsingSupported()

      expect(supported).toBe(false)
      
      // Restore
      vi.unstubAllGlobals()
    })
  })

  describe('Error handling', () => {
    it('should handle unexpected errors gracefully', async () => {
      // Make pako.gunzip throw an unexpected error
      mockPako.gunzip.mockImplementation(() => {
        throw new Error('Unexpected decompression error')
      })
      
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Failed to decompress SMA file')
    })

    it('should handle malformed archive structure', () => {
      const result = validateSMAStructure(null as any)

      expect(result.valid).toBe(false)
      expect(result.errors.some(e => e.includes('Structure validation error'))).toBe(true)
    })
  })

  describe('Performance and size tracking', () => {
    it('should track parsing time', async () => {
      // Reset mock to return valid data
      mockPako.gunzip.mockReturnValue(JSON.stringify(mockSMAArchive))
      const buffer = new ArrayBuffer(100)

      const result = await parseSMAFile(buffer, { validateChecksum: false })

      expect(result.success).toBe(true)
      expect(result.parseInfo.parseTimeMs).toBeGreaterThan(0)
    })

    it('should calculate compression ratio', async () => {
      const buffer = new ArrayBuffer(50) // Compressed size
      mockPako.gunzip.mockReturnValue(JSON.stringify(mockSMAArchive)) // Valid JSON

      const result = await parseSMAFile(buffer, { validateChecksum: false })

      expect(result.success).toBe(true)
      expect(result.parseInfo.originalSize).toBeGreaterThan(0)
      expect(result.parseInfo.compressedSize).toBe(50)
      expect(result.parseInfo.compressionRatio).toBeGreaterThan(0)
    })
  })
})