import { createHash } from 'crypto-browserify'
import pako from 'pako'
import type { 
  SMAArchive, 
  SMAMetadata, 
  SMADevice, 
  SMATemplate,
  SMADiscoveredDevice,
  SMANetworkSettings,
  SMAPluginConfig,
  SMASystemSettings
} from './sma-parser'

/**
 * SMA Format Generator for Shelly Manager Archive files
 * Handles generation, compression, and export of SMA format files
 */

export interface SMAGenerateOptions {
  compression?: boolean
  compressionLevel?: number // 1-9, 6 is default
  includeMetadata?: boolean
  calculateChecksum?: boolean
  exportId?: string
  createdBy?: string
  exportType?: 'manual' | 'scheduled' | 'api'
  systemInfo?: {
    version?: string
    databaseType?: 'sqlite' | 'postgresql' | 'mysql'
    hostname?: string
  }
}

export interface SMAGenerateResult {
  success: boolean
  blob?: Blob
  filename?: string
  error?: string
  metadata: {
    originalSize: number
    compressedSize: number
    compressionRatio: number
    checksum?: string
    generateTimeMs: number
    recordCount: number
  }
}

export interface SMADataSources {
  devices?: SMADevice[]
  templates?: SMATemplate[]
  discoveredDevices?: SMADiscoveredDevice[]
  networkSettings?: SMANetworkSettings
  pluginConfigurations?: SMAPluginConfig[]
  systemSettings?: SMASystemSettings
}

export interface SMAExportConfig {
  sections: string[] // Which sections to include
  deviceIds?: number[] // Specific devices to export (empty = all)
  templateIds?: number[] // Specific templates to export (empty = all)
  includeDiscovered?: boolean
  includeNetworkSettings?: boolean
  includePluginConfigs?: boolean
  includeSystemSettings?: boolean
}

/**
 * Generate SMA file from data sources
 */
export async function generateSMAFile(
  dataSources: SMADataSources,
  options: SMAGenerateOptions = {}
): Promise<SMAGenerateResult> {
  const startTime = performance.now()

  try {
    // Build archive structure
    const archive = await buildSMAArchive(dataSources, options)
    
    // Convert to JSON
    const jsonData = JSON.stringify(archive, null, 2)
    const originalSize = new TextEncoder().encode(jsonData).length
    let recordCount = 0

    // Count records
    if (archive.devices) recordCount += archive.devices.length
    if (archive.templates) recordCount += archive.templates.length
    if (archive.discovered_devices) recordCount += archive.discovered_devices.length

    // Calculate checksum if requested
    let checksum: string | undefined
    if (options.calculateChecksum !== false) {
      checksum = calculateSHA256(jsonData)
      
      // Update archive with checksum
      archive.metadata.integrity.checksum = `sha256:${checksum}`
      archive.metadata.integrity.record_count = recordCount
      
      // Regenerate JSON with updated metadata
      const finalJsonData = JSON.stringify(archive, null, 2)
      
      // Compress if requested
      let finalData: Uint8Array
      if (options.compression !== false) {
        try {
          finalData = pako.gzip(finalJsonData, { 
            level: options.compressionLevel || 6 
          })
        } catch (compressionError) {
          return {
            success: false,
            error: `Compression failed: ${compressionError}`,
            metadata: {
              originalSize,
              compressedSize: 0,
              compressionRatio: 0,
              generateTimeMs: performance.now() - startTime,
              recordCount
            }
          }
        }
      } else {
        finalData = new TextEncoder().encode(finalJsonData)
      }

      const compressedSize = finalData.length
      const blob = new Blob([finalData], { type: 'application/octet-stream' })
      const filename = generateSMAFilename(archive.metadata)

      return {
        success: true,
        blob,
        filename,
        metadata: {
          originalSize,
          compressedSize,
          compressionRatio: compressedSize / originalSize,
          checksum,
          generateTimeMs: performance.now() - startTime,
          recordCount
        }
      }
    }

    // Simple generation without checksum recalculation
    let finalData: Uint8Array
    if (options.compression !== false) {
      try {
        finalData = pako.gzip(jsonData, { 
          level: options.compressionLevel || 6 
        })
      } catch (compressionError) {
        return {
          success: false,
          error: `Compression failed: ${compressionError}`,
          metadata: {
            originalSize,
            compressedSize: 0,
            compressionRatio: 0,
            generateTimeMs: performance.now() - startTime,
            recordCount
          }
        }
      }
    } else {
      finalData = new TextEncoder().encode(jsonData)
    }

    const compressedSize = finalData.length
    const blob = new Blob([finalData], { type: 'application/octet-stream' })
    const filename = generateSMAFilename(archive.metadata)

    return {
      success: true,
      blob,
      filename,
      metadata: {
        originalSize,
        compressedSize,
        compressionRatio: compressedSize / originalSize,
        checksum,
        generateTimeMs: performance.now() - startTime,
        recordCount
      }
    }

  } catch (error) {
    return {
      success: false,
      error: `Failed to generate SMA file: ${error}`,
      metadata: {
        originalSize: 0,
        compressedSize: 0,
        compressionRatio: 0,
        generateTimeMs: performance.now() - startTime,
        recordCount: 0
      }
    }
  }
}

/**
 * Generate SMA file with export configuration
 */
export async function generateSMAWithConfig(
  dataSources: SMADataSources,
  exportConfig: SMAExportConfig,
  options: SMAGenerateOptions = {}
): Promise<SMAGenerateResult> {
  // Filter data sources based on configuration
  const filteredSources: SMADataSources = {}

  // Filter devices
  if (exportConfig.sections.includes('devices') && dataSources.devices) {
    if (exportConfig.deviceIds && exportConfig.deviceIds.length > 0) {
      filteredSources.devices = dataSources.devices.filter(d => 
        exportConfig.deviceIds!.includes(d.id)
      )
    } else {
      filteredSources.devices = dataSources.devices
    }
  }

  // Filter templates
  if (exportConfig.sections.includes('templates') && dataSources.templates) {
    if (exportConfig.templateIds && exportConfig.templateIds.length > 0) {
      filteredSources.templates = dataSources.templates.filter(t => 
        exportConfig.templateIds!.includes(t.id)
      )
    } else {
      filteredSources.templates = dataSources.templates
    }
  }

  // Include other sections based on configuration
  if (exportConfig.includeDiscovered && dataSources.discoveredDevices) {
    filteredSources.discoveredDevices = dataSources.discoveredDevices
  }

  if (exportConfig.includeNetworkSettings && dataSources.networkSettings) {
    filteredSources.networkSettings = dataSources.networkSettings
  }

  if (exportConfig.includePluginConfigs && dataSources.pluginConfigurations) {
    filteredSources.pluginConfigurations = dataSources.pluginConfigurations
  }

  if (exportConfig.includeSystemSettings && dataSources.systemSettings) {
    filteredSources.systemSettings = dataSources.systemSettings
  }

  return await generateSMAFile(filteredSources, options)
}

/**
 * Build SMA archive structure from data sources
 */
async function buildSMAArchive(
  dataSources: SMADataSources,
  options: SMAGenerateOptions
): Promise<SMAArchive> {
  const now = new Date().toISOString()
  const exportId = options.exportId || generateUUID()

  // Calculate total size estimation
  let totalSizeBytes = 0
  const sections = Object.keys(dataSources).length
  
  if (dataSources.devices) totalSizeBytes += dataSources.devices.length * 2048
  if (dataSources.templates) totalSizeBytes += dataSources.templates.length * 1024
  if (dataSources.discoveredDevices) totalSizeBytes += dataSources.discoveredDevices.length * 512

  const metadata: SMAMetadata = {
    export_id: exportId,
    created_at: now,
    created_by: options.createdBy,
    export_type: options.exportType || 'manual',
    system_info: {
      version: options.systemInfo?.version || 'v0.5.4-alpha',
      database_type: options.systemInfo?.databaseType || 'sqlite',
      hostname: options.systemInfo?.hostname || 'shelly-manager',
      total_size_bytes: totalSizeBytes,
      compression_ratio: 0.7 // Estimated, will be updated after compression
    },
    integrity: {
      checksum: '', // Will be calculated later
      record_count: 0, // Will be calculated later
      file_count: sections
    }
  }

  const archive: SMAArchive = {
    sma_version: '1.0',
    format_version: '2024.1',
    metadata,
    devices: dataSources.devices || [],
    templates: dataSources.templates || [],
    discovered_devices: dataSources.discoveredDevices || [],
    network_settings: dataSources.networkSettings || {
      wifi_networks: [],
      mqtt_config: {
        server: '',
        port: 1883,
        username: '',
        retain: false,
        qos: 0
      },
      ntp_servers: []
    },
    plugin_configurations: dataSources.pluginConfigurations || [],
    system_settings: dataSources.systemSettings || {
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

  return archive
}

/**
 * Generate SMA filename based on metadata
 */
function generateSMAFilename(metadata: SMAMetadata): string {
  const timestamp = new Date(metadata.created_at)
  const dateStr = timestamp.toISOString().split('T')[0].replace(/-/g, '')
  const timeStr = timestamp.toTimeString().split(' ')[0].replace(/:/g, '')
  
  return `shelly-manager-backup-${dateStr}-${timeStr}.sma`
}

/**
 * Download SMA file to user's device
 */
export async function downloadSMAFile(
  dataSources: SMADataSources,
  options: SMAGenerateOptions = {}
): Promise<SMAGenerateResult> {
  const result = await generateSMAFile(dataSources, options)
  
  if (result.success && result.blob && result.filename) {
    // Create download link
    const url = URL.createObjectURL(result.blob)
    const a = document.createElement('a')
    a.href = url
    a.download = result.filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
  
  return result
}

/**
 * Create SMA file for API upload
 */
export async function createSMAForUpload(
  dataSources: SMADataSources,
  options: SMAGenerateOptions = {}
): Promise<{ file?: File; error?: string; metadata?: any }> {
  const result = await generateSMAFile(dataSources, options)
  
  if (!result.success || !result.blob || !result.filename) {
    return { error: result.error || 'Failed to generate SMA file' }
  }
  
  const file = new File([result.blob], result.filename, { 
    type: 'application/octet-stream' 
  })
  
  return { file, metadata: result.metadata }
}

/**
 * Estimate SMA file size without generating
 */
export function estimateSMASize(
  dataSources: SMADataSources,
  options: SMAGenerateOptions = {}
): {
  estimatedOriginalSize: number
  estimatedCompressedSize: number
  estimatedCompressionRatio: number
} {
  let baseSize = 2048 // Base JSON structure overhead

  // Estimate device data size
  if (dataSources.devices) {
    baseSize += dataSources.devices.length * 1500 // ~1.5KB per device
  }

  // Estimate template data size  
  if (dataSources.templates) {
    baseSize += dataSources.templates.length * 800 // ~800B per template
  }

  // Estimate discovered devices size
  if (dataSources.discoveredDevices) {
    baseSize += dataSources.discoveredDevices.length * 300 // ~300B per discovered device
  }

  // Add other sections
  if (dataSources.networkSettings) baseSize += 1024
  if (dataSources.pluginConfigurations) {
    baseSize += dataSources.pluginConfigurations.length * 400
  }
  if (dataSources.systemSettings) baseSize += 512

  const compressionRatio = options.compression !== false ? 0.7 : 1.0
  const compressedSize = Math.round(baseSize * compressionRatio)

  return {
    estimatedOriginalSize: baseSize,
    estimatedCompressedSize: compressedSize,
    estimatedCompressionRatio: compressionRatio
  }
}

/**
 * Validate data sources before generation
 */
export function validateSMADataSources(dataSources: SMADataSources): {
  valid: boolean
  errors: string[]
  warnings: string[]
} {
  const errors: string[] = []
  const warnings: string[] = []

  // Check if we have any data to export
  const hasData = Object.values(dataSources).some(value => 
    Array.isArray(value) ? value.length > 0 : !!value
  )

  if (!hasData) {
    errors.push('No data provided for export')
    return { valid: false, errors, warnings }
  }

  // Validate device data
  if (dataSources.devices) {
    const deviceIds = new Set()
    for (const device of dataSources.devices) {
      if (!device.id) errors.push('Device missing ID')
      if (!device.mac) errors.push(`Device ${device.id} missing MAC address`)
      if (!device.ip) errors.push(`Device ${device.id} missing IP address`)
      
      if (deviceIds.has(device.id)) {
        errors.push(`Duplicate device ID: ${device.id}`)
      }
      deviceIds.add(device.id)
    }
  }

  // Validate template data
  if (dataSources.templates) {
    const templateIds = new Set()
    for (const template of dataSources.templates) {
      if (!template.id) errors.push('Template missing ID')
      if (!template.name) errors.push(`Template ${template.id} missing name`)
      if (!template.device_type) errors.push(`Template ${template.id} missing device_type`)
      
      if (templateIds.has(template.id)) {
        errors.push(`Duplicate template ID: ${template.id}`)
      }
      templateIds.add(template.id)
    }

    // Check for template references in devices
    if (dataSources.devices) {
      const templateRefs = dataSources.devices
        .filter(d => d.configuration?.template_id)
        .map(d => d.configuration!.template_id!)
      
      for (const templateId of templateRefs) {
        if (!templateIds.has(templateId)) {
          warnings.push(`Device references non-existent template ID: ${templateId}`)
        }
      }
    }
  }

  // Validate network settings
  if (dataSources.networkSettings) {
    if (!dataSources.networkSettings.wifi_networks) {
      warnings.push('Network settings missing wifi_networks array')
    }
    if (!dataSources.networkSettings.mqtt_config) {
      warnings.push('Network settings missing mqtt_config')
    }
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings
  }
}

// Helper functions

/**
 * Calculate SHA-256 hash of string
 */
function calculateSHA256(data: string): string {
  const hash = createHash('sha256')
  hash.update(data)
  return hash.digest('hex')
}

/**
 * Generate UUID v4
 */
function generateUUID(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
}

/**
 * Check if browser supports SMA generation
 */
export function isSMAGenerationSupported(): boolean {
  return typeof crypto !== 'undefined' && 
         typeof pako !== 'undefined' &&
         typeof Blob !== 'undefined' &&
         typeof URL !== 'undefined'
}