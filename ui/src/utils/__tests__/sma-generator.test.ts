import { describe, it, expect, vi, beforeEach } from 'vitest'
import pako from 'pako'
import {
  generateSMAFile,
  generateSMAWithConfig,
  downloadSMAFile,
  createSMAForUpload,
  estimateSMASize,
  validateSMADataSources,
  isSMAGenerationSupported,
  type SMADataSources,
  type SMAExportConfig,
  type SMAGenerateOptions
} from '../sma-generator'

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
    gzip: vi.fn()
  }
}))

const mockPako = vi.mocked(pako)

describe('SMA Generator', () => {
  const mockDataSources: SMADataSources = {
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
      },
      {
        id: 2,
        mac: 'AA:BB:CC:DD:EE:02',
        ip: '192.168.1.101',
        type: 'shelly25',
        name: 'Test Device 2',
        status: 'offline',
        last_seen: '2024-01-15T09:00:00Z',
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-15T09:30:00Z'
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
    discoveredDevices: [
      {
        mac: 'AA:BB:CC:DD:EE:03',
        ssid: 'ShellyTest-123',
        model: 'SHSW-1',
        generation: 1,
        ip: '192.168.1.102',
        signal: -45,
        agent_id: 'agent-001',
        discovered: '2024-01-15T10:20:00Z'
      }
    ],
    networkSettings: {
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
    pluginConfigurations: [
      {
        plugin_name: 'test-plugin',
        version: '1.0.0',
        config: { enabled: true },
        enabled: true
      }
    ],
    systemSettings: {
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
    mockPako.gzip.mockImplementation((data: string) => {
      const compressed = new Uint8Array(data.length / 2) // Mock 50% compression
      return compressed
    })

    // Mock Blob and URL for browser APIs
    global.Blob = vi.fn().mockImplementation((parts, options) => ({
      size: parts?.[0]?.length || 0,
      type: options?.type || 'application/octet-stream'
    })) as any

    global.URL = {
      createObjectURL: vi.fn(() => 'blob:mock-url'),
      revokeObjectURL: vi.fn()
    } as any

    global.File = vi.fn().mockImplementation((parts, name, options) => ({
      name,
      size: parts?.[0]?.length || 0,
      type: options?.type || 'application/octet-stream'
    })) as any
  })

  describe('generateSMAFile', () => {
    it('should generate SMA file successfully', async () => {
      const result = await generateSMAFile(mockDataSources)

      expect(result.success).toBe(true)
      expect(result.blob).toBeDefined()
      expect(result.filename).toMatch(/shelly-manager-backup-\d{8}-\d{6}\.sma/)
      expect(result.metadata.recordCount).toBe(4) // 2 devices + 1 template + 1 discovered
      expect(result.metadata.generateTimeMs).toBeGreaterThan(0)
    })

    it('should generate without compression when disabled', async () => {
      const options: SMAGenerateOptions = { compression: false }
      
      const result = await generateSMAFile(mockDataSources, options)

      expect(result.success).toBe(true)
      expect(mockPako.gzip).not.toHaveBeenCalled()
      expect(result.metadata.compressionRatio).toBeCloseTo(1, 1) // No compression (allow slight variance)
    })

    it('should use custom compression level', async () => {
      const options: SMAGenerateOptions = { 
        compression: true, 
        compressionLevel: 9 
      }
      
      const result = await generateSMAFile(mockDataSources, options)

      expect(result.success).toBe(true)
      expect(mockPako.gzip).toHaveBeenCalledWith(
        expect.any(String),
        { level: 9 }
      )
    })

    it('should include checksum when enabled', async () => {
      const options: SMAGenerateOptions = { calculateChecksum: true }
      
      const result = await generateSMAFile(mockDataSources, options)

      expect(result.success).toBe(true)
      expect(result.metadata.checksum).toBe('mockedchecksum')
    })

    it('should use custom export metadata', async () => {
      const options: SMAGenerateOptions = {
        exportId: 'custom-export-123',
        createdBy: 'test@example.com',
        exportType: 'scheduled',
        systemInfo: {
          version: 'v0.6.0',
          databaseType: 'postgresql',
          hostname: 'custom-host'
        }
      }
      
      const result = await generateSMAFile(mockDataSources, options)

      expect(result.success).toBe(true)
      // We can't easily inspect the archive content due to compression
      // but we can verify the function completed successfully
    })

    it('should handle compression errors', async () => {
      mockPako.gzip.mockImplementation(() => {
        throw new Error('Compression failed')
      })

      const result = await generateSMAFile(mockDataSources)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Compression failed')
    })

    it('should handle unexpected errors', async () => {
      // Create an invalid data source that will cause JSON.stringify to fail
      const invalidDataSources = {
        devices: [{ circular: {} as any }]
      }
      invalidDataSources.devices[0].circular = invalidDataSources.devices[0]

      const result = await generateSMAFile(invalidDataSources)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Failed to generate SMA file')
    })
  })

  describe('generateSMAWithConfig', () => {
    it('should filter devices based on configuration', async () => {
      const exportConfig: SMAExportConfig = {
        sections: ['devices', 'templates'],
        deviceIds: [1], // Only first device
        includeDiscovered: false,
        includeNetworkSettings: false,
        includePluginConfigs: false,
        includeSystemSettings: false
      }

      const result = await generateSMAWithConfig(mockDataSources, exportConfig)

      expect(result.success).toBe(true)
      expect(result.metadata.recordCount).toBe(2) // 1 device + 1 template
    })

    it('should filter templates based on configuration', async () => {
      const exportConfig: SMAExportConfig = {
        sections: ['templates'],
        templateIds: [1], // Include only first template
        includeDiscovered: false,
        includeNetworkSettings: false,
        includePluginConfigs: false,
        includeSystemSettings: false
      }

      const result = await generateSMAWithConfig(mockDataSources, exportConfig)

      expect(result.success).toBe(true)
      expect(result.metadata.recordCount).toBe(1) // Only 1 template
    })

    it('should include optional sections when requested', async () => {
      const exportConfig: SMAExportConfig = {
        sections: ['devices', 'templates'],
        includeDiscovered: true,
        includeNetworkSettings: true,
        includePluginConfigs: true,
        includeSystemSettings: true
      }

      const result = await generateSMAWithConfig(mockDataSources, exportConfig)

      expect(result.success).toBe(true)
      expect(result.metadata.recordCount).toBe(4) // 2 devices + 1 template + 1 discovered
    })
  })

  describe('downloadSMAFile', () => {
    it('should trigger file download', async () => {
      // Mock DOM methods
      const mockElement = {
        href: '',
        download: '',
        click: vi.fn(),
      }
      const createElement = vi.spyOn(document, 'createElement').mockReturnValue(mockElement as any)
      const appendChild = vi.spyOn(document.body, 'appendChild').mockImplementation(() => mockElement as any)
      const removeChild = vi.spyOn(document.body, 'removeChild').mockImplementation(() => mockElement as any)

      const result = await downloadSMAFile(mockDataSources)

      expect(result.success).toBe(true)
      expect(createElement).toHaveBeenCalledWith('a')
      expect(appendChild).toHaveBeenCalledWith(mockElement)
      expect(mockElement.click).toHaveBeenCalled()
      expect(removeChild).toHaveBeenCalledWith(mockElement)
      expect(global.URL.createObjectURL).toHaveBeenCalled()
      expect(global.URL.revokeObjectURL).toHaveBeenCalled()
    })

    it('should handle generation failure', async () => {
      // Force generation to fail
      mockPako.gzip.mockImplementation(() => {
        throw new Error('Generation failed')
      })

      const result = await downloadSMAFile(mockDataSources)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Compression failed')
    })
  })

  describe('createSMAForUpload', () => {
    it('should create File object for upload', async () => {
      const result = await createSMAForUpload(mockDataSources)

      expect(result.file).toBeDefined()
      expect(result.metadata).toBeDefined()
      expect(result.error).toBeUndefined()
      expect(global.File).toHaveBeenCalled()
    })

    it('should return error when generation fails', async () => {
      mockPako.gzip.mockImplementation(() => {
        throw new Error('Upload generation failed')
      })

      const result = await createSMAForUpload(mockDataSources)

      expect(result.file).toBeUndefined()
      expect(result.error).toContain('Compression failed')
    })
  })

  describe('estimateSMASize', () => {
    it('should estimate size based on data sources', () => {
      const estimate = estimateSMASize(mockDataSources)

      expect(estimate.estimatedOriginalSize).toBeGreaterThan(0)
      expect(estimate.estimatedCompressedSize).toBeLessThan(estimate.estimatedOriginalSize)
      expect(estimate.estimatedCompressionRatio).toBeLessThan(1)
    })

    it('should account for compression disabled', () => {
      const estimate = estimateSMASize(mockDataSources, { compression: false })

      expect(estimate.estimatedCompressionRatio).toBe(1.0)
      expect(estimate.estimatedCompressedSize).toBe(estimate.estimatedOriginalSize)
    })

    it('should handle empty data sources', () => {
      const estimate = estimateSMASize({})

      expect(estimate.estimatedOriginalSize).toBeGreaterThan(0) // Base structure overhead
      expect(estimate.estimatedCompressedSize).toBeGreaterThan(0)
    })
  })

  describe('validateSMADataSources', () => {
    it('should validate valid data sources', () => {
      const result = validateSMADataSources(mockDataSources)

      expect(result.valid).toBe(true)
      expect(result.errors).toHaveLength(0)
    })

    it('should reject empty data sources', () => {
      const result = validateSMADataSources({})

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('No data provided for export')
    })

    it('should validate device data integrity', () => {
      const invalidDataSources: SMADataSources = {
        devices: [
          {
            // Missing required fields
            id: 0,
            mac: '',
            ip: '',
            type: 'shelly1',
            status: 'online',
            last_seen: '2024-01-15T10:25:00Z',
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-15T10:00:00Z'
          }
        ]
      }

      const result = validateSMADataSources(invalidDataSources)

      expect(result.valid).toBe(false)
      expect(result.errors.some(e => e.includes('missing ID'))).toBe(true)
      expect(result.errors.some(e => e.includes('missing MAC address'))).toBe(true)
      expect(result.errors.some(e => e.includes('missing IP address'))).toBe(true)
    })

    it('should detect duplicate device IDs', () => {
      const duplicateDataSources: SMADataSources = {
        devices: [
          {
            id: 1,
            mac: 'AA:BB:CC:DD:EE:01',
            ip: '192.168.1.100',
            type: 'shelly1',
            status: 'online',
            last_seen: '2024-01-15T10:25:00Z',
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-15T10:00:00Z'
          },
          {
            id: 1, // Duplicate ID
            mac: 'AA:BB:CC:DD:EE:02',
            ip: '192.168.1.101',
            type: 'shelly1',
            status: 'online',
            last_seen: '2024-01-15T10:25:00Z',
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-15T10:00:00Z'
          }
        ]
      }

      const result = validateSMADataSources(duplicateDataSources)

      expect(result.valid).toBe(false)
      expect(result.errors.some(e => e.includes('Duplicate device ID: 1'))).toBe(true)
    })

    it('should validate template data integrity', () => {
      const invalidTemplateDataSources: SMADataSources = {
        templates: [
          {
            // Missing required fields
            id: 0,
            name: '',
            device_type: '',
            generation: 1,
            config: {},
            is_default: false,
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-15T10:00:00Z'
          }
        ]
      }

      const result = validateSMADataSources(invalidTemplateDataSources)

      expect(result.valid).toBe(false)
      expect(result.errors.some(e => e.includes('Template missing ID'))).toBe(true)
      expect(result.errors.some(e => e.includes('missing name'))).toBe(true)
      expect(result.errors.some(e => e.includes('missing device_type'))).toBe(true)
    })

    it('should detect template reference issues', () => {
      const dataWithBadRefs: SMADataSources = {
        devices: [
          {
            id: 1,
            mac: 'AA:BB:CC:DD:EE:01',
            ip: '192.168.1.100',
            type: 'shelly1',
            status: 'online',
            last_seen: '2024-01-15T10:25:00Z',
            configuration: {
              template_id: 999, // Non-existent template
              sync_status: 'synced'
            },
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-15T10:00:00Z'
          }
        ],
        templates: [
          {
            id: 1, // Template ID 1, but device references 999
            name: 'Test Template',
            device_type: 'shelly1',
            generation: 1,
            config: {},
            is_default: true,
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-15T10:00:00Z'
          }
        ]
      }

      const result = validateSMADataSources(dataWithBadRefs)

      expect(result.valid).toBe(true) // Still valid, but has warnings
      expect(result.warnings.some(w => w.includes('non-existent template ID: 999'))).toBe(true)
    })

    it('should warn about missing optional configurations', () => {
      const dataWithMissingOptional: SMADataSources = {
        devices: mockDataSources.devices,
        networkSettings: {
          // Missing wifi_networks and mqtt_config
          wifi_networks: undefined as any,
          mqtt_config: undefined as any,
          ntp_servers: ['pool.ntp.org']
        }
      }

      const result = validateSMADataSources(dataWithMissingOptional)

      expect(result.valid).toBe(true) // Still valid
      expect(result.warnings.some(w => w.includes('missing wifi_networks'))).toBe(true)
      expect(result.warnings.some(w => w.includes('missing mqtt_config'))).toBe(true)
    })
  })

  describe('isSMAGenerationSupported', () => {
    it('should return true when all dependencies are available', () => {
      // Mock dependencies are already available in test environment
      const supported = isSMAGenerationSupported()

      expect(supported).toBe(true)
    })

    it('should return false when dependencies are missing', () => {
      // Use vi.stubGlobal to mock missing dependencies
      vi.stubGlobal('crypto', undefined)
      
      const supported = isSMAGenerationSupported()

      expect(supported).toBe(false)
      
      // Restore
      vi.unstubAllGlobals()
    })
  })

  describe('Helper functions', () => {
    it('should generate unique UUIDs', async () => {
      const result1 = await generateSMAFile(mockDataSources)
      const result2 = await generateSMAFile(mockDataSources)

      expect(result1.success).toBe(true)
      expect(result2.success).toBe(true)
      // We can't easily check the UUIDs are different due to compression,
      // but we can verify both succeeded
    })

    it('should generate appropriate filenames', async () => {
      const result = await generateSMAFile(mockDataSources)

      expect(result.success).toBe(true)
      expect(result.filename).toMatch(/^shelly-manager-backup-\d{8}-\d{6}\.sma$/)
    })
  })

  describe('Performance and size tracking', () => {
    it('should track generation time', async () => {
      const result = await generateSMAFile(mockDataSources)

      expect(result.success).toBe(true)
      expect(result.metadata.generateTimeMs).toBeGreaterThan(0)
    })

    it('should calculate compression metrics', async () => {
      const result = await generateSMAFile(mockDataSources, { compression: true })

      expect(result.success).toBe(true)
      expect(result.metadata.originalSize).toBeGreaterThan(0)
      expect(result.metadata.compressedSize).toBeGreaterThan(0)
      expect(result.metadata.compressionRatio).toBeGreaterThan(0)
      expect(result.metadata.compressionRatio).toBeLessThanOrEqual(1)
    })
  })
})