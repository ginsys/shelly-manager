import { createHash } from 'crypto-browserify'
import pako from 'pako'

/**
 * SMA Format Parser for Shelly Manager Archive files
 * Handles parsing, validation, and extraction of compressed SMA files
 */

export interface SMAMetadata {
  export_id: string
  created_at: string
  created_by?: string
  export_type: 'manual' | 'scheduled' | 'api'
  system_info: {
    version: string
    database_type: 'sqlite' | 'postgresql' | 'mysql'
    hostname: string
    total_size_bytes: number
    compression_ratio: number
  }
  integrity: {
    checksum: string
    record_count: number
    file_count: number
  }
}

export interface SMADevice {
  id: number
  mac: string
  ip: string
  type: string
  name?: string
  model?: string
  firmware?: string
  status: 'online' | 'offline' | 'unknown'
  last_seen: string
  settings?: Record<string, any>
  configuration?: {
    template_id?: number
    config?: Record<string, any>
    last_synced?: string
    sync_status?: 'synced' | 'pending' | 'failed'
  }
  created_at: string
  updated_at: string
}

export interface SMATemplate {
  id: number
  name: string
  description?: string
  device_type: string
  generation: number
  config: Record<string, any>
  variables?: Record<string, any>
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface SMADiscoveredDevice {
  mac: string
  ssid: string
  model: string
  generation: number
  ip: string
  signal: number
  agent_id: string
  discovered: string
}

export interface SMANetworkSettings {
  wifi_networks: Array<{
    ssid: string
    security: string
    priority: number
  }>
  mqtt_config: {
    server: string
    port: number
    username: string
    retain: boolean
    qos: number
  }
  ntp_servers: string[]
}

export interface SMAPluginConfig {
  plugin_name: string
  version: string
  config: Record<string, any>
  enabled: boolean
}

export interface SMASystemSettings {
  log_level: string
  api_settings: {
    rate_limit: number
    cors_enabled: boolean
  }
  database_settings: {
    connection_pool_size: number
    query_timeout: string
  }
}

export interface SMAArchive {
  sma_version: string
  format_version: string
  metadata: SMAMetadata
  devices: SMADevice[]
  templates: SMATemplate[]
  discovered_devices: SMADiscoveredDevice[]
  network_settings: SMANetworkSettings
  plugin_configurations: SMAPluginConfig[]
  system_settings: SMASystemSettings
}

export interface SMAParseOptions {
  validateChecksum?: boolean
  validateStructure?: boolean
  maxSizeBytes?: number
}

export interface SMAParseResult {
  success: boolean
  archive?: SMAArchive
  error?: string
  warnings?: string[]
  parseInfo: {
    originalSize: number
    compressedSize: number
    compressionRatio: number
    parseTimeMs: number
  }
}

export interface SMAValidationResult {
  valid: boolean
  errors: string[]
  warnings: string[]
  summary: {
    smaVersion: string
    formatVersion: string
    deviceCount: number
    templateCount: number
    discoveredDeviceCount: number
    pluginConfigCount: number
    estimatedDataIntegrity: number // 0-100%
  }
}

/**
 * Parse SMA file from ArrayBuffer
 */
export async function parseSMAFile(
  buffer: ArrayBuffer, 
  options: SMAParseOptions = {}
): Promise<SMAParseResult> {
  const startTime = performance.now()
  const compressedSize = buffer.byteLength
  const warnings: string[] = []

  try {
    // Check maximum size
    if (options.maxSizeBytes && compressedSize > options.maxSizeBytes) {
      return {
        success: false,
        error: `File size ${formatBytes(compressedSize)} exceeds maximum allowed size ${formatBytes(options.maxSizeBytes)}`,
        parseInfo: {
          originalSize: 0,
          compressedSize,
          compressionRatio: 0,
          parseTimeMs: performance.now() - startTime
        }
      }
    }

    // Decompress using Gzip
    let decompressedData: string
    let originalSize: number
    
    try {
      const uint8Array = new Uint8Array(buffer)
      const decompressed = pako.gunzip(uint8Array, { to: 'string' })
      decompressedData = decompressed
      originalSize = decompressed.length
    } catch (decompressError) {
      return {
        success: false,
        error: `Failed to decompress SMA file: ${decompressError}`,
        parseInfo: {
          originalSize: 0,
          compressedSize,
          compressionRatio: 0,
          parseTimeMs: performance.now() - startTime
        }
      }
    }

    // Parse JSON
    let archive: SMAArchive
    try {
      archive = JSON.parse(decompressedData) as SMAArchive
    } catch (jsonError) {
      return {
        success: false,
        error: `Failed to parse JSON content: ${jsonError}`,
        parseInfo: {
          originalSize,
          compressedSize,
          compressionRatio: compressedSize / originalSize,
          parseTimeMs: performance.now() - startTime
        }
      }
    }

    // Validate checksum if requested
    if (options.validateChecksum !== false) {
      const calculatedChecksum = calculateSHA256(decompressedData)
      const expectedChecksum = archive.metadata.integrity.checksum.replace('sha256:', '')
      
      if (calculatedChecksum !== expectedChecksum) {
        return {
          success: false,
          error: `Checksum validation failed. Expected: ${expectedChecksum}, Got: ${calculatedChecksum}`,
          parseInfo: {
            originalSize,
            compressedSize,
            compressionRatio: compressedSize / originalSize,
            parseTimeMs: performance.now() - startTime
          }
        }
      }
    }

    // Validate structure if requested
    if (options.validateStructure !== false) {
      const validation = validateSMAStructure(archive)
      if (!validation.valid) {
        return {
          success: false,
          error: `Structure validation failed: ${validation.errors.join(', ')}`,
          parseInfo: {
            originalSize,
            compressedSize,
            compressionRatio: compressedSize / originalSize,
            parseTimeMs: performance.now() - startTime
          }
        }
      }
      warnings.push(...validation.warnings)
    }

    // Success
    return {
      success: true,
      archive,
      warnings,
      parseInfo: {
        originalSize,
        compressedSize,
        compressionRatio: compressedSize / originalSize,
        parseTimeMs: performance.now() - startTime
      }
    }

  } catch (error) {
    return {
      success: false,
      error: `Unexpected error parsing SMA file: ${error}`,
      parseInfo: {
        originalSize: 0,
        compressedSize,
        compressionRatio: 0,
        parseTimeMs: performance.now() - startTime
      }
    }
  }
}

/**
 * Parse SMA file from File object
 */
export async function parseSMAFromFile(
  file: File, 
  options: SMAParseOptions = {}
): Promise<SMAParseResult> {
  try {
    const buffer = await file.arrayBuffer()
    return await parseSMAFile(buffer, options)
  } catch (error) {
    return {
      success: false,
      error: `Failed to read file: ${error}`,
      parseInfo: {
        originalSize: 0,
        compressedSize: file.size,
        compressionRatio: 0,
        parseTimeMs: 0
      }
    }
  }
}

/**
 * Validate SMA archive structure
 */
export function validateSMAStructure(archive: any): SMAValidationResult {
  const errors: string[] = []
  const warnings: string[] = []

  try {
    // Check required top-level fields
    if (!archive.sma_version) errors.push('Missing sma_version')
    if (!archive.format_version) errors.push('Missing format_version')
    if (!archive.metadata) errors.push('Missing metadata')
    if (!Array.isArray(archive.devices)) errors.push('Missing or invalid devices array')
    if (!Array.isArray(archive.templates)) errors.push('Missing or invalid templates array')

    // Validate SMA version compatibility
    if (archive.sma_version) {
      const version = parseFloat(archive.sma_version)
      if (version > 1.0) {
        warnings.push(`SMA version ${archive.sma_version} is newer than supported version 1.0`)
      } else if (version < 1.0) {
        warnings.push(`SMA version ${archive.sma_version} is older than current version 1.0`)
      }
    }

    // Validate metadata structure
    if (archive.metadata) {
      if (!archive.metadata.export_id) errors.push('Missing metadata.export_id')
      if (!archive.metadata.created_at) errors.push('Missing metadata.created_at')
      if (!archive.metadata.integrity) errors.push('Missing metadata.integrity')
      
      if (archive.metadata.integrity) {
        if (!archive.metadata.integrity.checksum) errors.push('Missing metadata.integrity.checksum')
        if (typeof archive.metadata.integrity.record_count !== 'number') {
          errors.push('Missing or invalid metadata.integrity.record_count')
        }
      }
    }

    // Validate device references
    const templateIds = new Set((archive.templates || []).map((t: any) => t.id))
    const deviceTemplateRefs = (archive.devices || [])
      .filter((d: any) => d.configuration?.template_id)
      .map((d: any) => d.configuration.template_id)

    for (const templateId of deviceTemplateRefs) {
      if (!templateIds.has(templateId)) {
        warnings.push(`Device references non-existent template ID: ${templateId}`)
      }
    }

    // Record count validation
    if (archive.metadata?.integrity?.record_count) {
      const actualCount = (archive.devices?.length || 0) + 
                         (archive.templates?.length || 0) + 
                         (archive.discovered_devices?.length || 0)
      const expectedCount = archive.metadata.integrity.record_count

      if (actualCount !== expectedCount) {
        warnings.push(`Record count mismatch: expected ${expectedCount}, found ${actualCount}`)
      }
    }

    // Calculate data integrity score
    let integrityScore = 100
    if (errors.length > 0) integrityScore -= errors.length * 20
    if (warnings.length > 0) integrityScore -= warnings.length * 5
    integrityScore = Math.max(0, integrityScore)

    return {
      valid: errors.length === 0,
      errors,
      warnings,
      summary: {
        smaVersion: archive.sma_version || 'unknown',
        formatVersion: archive.format_version || 'unknown',
        deviceCount: archive.devices?.length || 0,
        templateCount: archive.templates?.length || 0,
        discoveredDeviceCount: archive.discovered_devices?.length || 0,
        pluginConfigCount: archive.plugin_configurations?.length || 0,
        estimatedDataIntegrity: integrityScore
      }
    }

  } catch (error) {
    return {
      valid: false,
      errors: [`Structure validation error: ${error}`],
      warnings: [],
      summary: {
        smaVersion: 'unknown',
        formatVersion: 'unknown',
        deviceCount: 0,
        templateCount: 0,
        discoveredDeviceCount: 0,
        pluginConfigCount: 0,
        estimatedDataIntegrity: 0
      }
    }
  }
}

/**
 * Extract metadata from SMA file without full parsing
 */
export async function extractSMAMetadata(buffer: ArrayBuffer): Promise<{
  success: boolean
  metadata?: SMAMetadata
  error?: string
}> {
  try {
    const uint8Array = new Uint8Array(buffer)
    const decompressed = pako.gunzip(uint8Array, { to: 'string' })
    
    // Parse just enough to get metadata
    const partialParse = JSON.parse(decompressed)
    
    if (!partialParse.metadata) {
      return { success: false, error: 'No metadata found in SMA file' }
    }

    return { success: true, metadata: partialParse.metadata }
  } catch (error) {
    return { success: false, error: `Failed to extract metadata: ${error}` }
  }
}

/**
 * Get compatible import sections from SMA archive
 */
export function getSMAImportSections(archive: SMAArchive): string[] {
  const sections: string[] = []
  
  if (archive.devices && archive.devices.length > 0) sections.push('devices')
  if (archive.templates && archive.templates.length > 0) sections.push('templates')
  if (archive.discovered_devices && archive.discovered_devices.length > 0) sections.push('discovered_devices')
  if (archive.network_settings) sections.push('network_settings')
  if (archive.plugin_configurations && archive.plugin_configurations.length > 0) sections.push('plugin_configurations')
  if (archive.system_settings) sections.push('system_settings')
  
  return sections
}

/**
 * Filter SMA archive to specific sections
 */
export function filterSMAArchive(archive: SMAArchive, sections: string[]): Partial<SMAArchive> {
  const filtered: Partial<SMAArchive> = {
    sma_version: archive.sma_version,
    format_version: archive.format_version,
    metadata: {
      ...archive.metadata,
      integrity: {
        ...archive.metadata.integrity,
        // Recalculate record count for filtered data
        record_count: 0 // Will be calculated below
      }
    }
  }

  let recordCount = 0

  if (sections.includes('devices') && archive.devices) {
    filtered.devices = archive.devices
    recordCount += archive.devices.length
  }

  if (sections.includes('templates') && archive.templates) {
    filtered.templates = archive.templates
    recordCount += archive.templates.length
  }

  if (sections.includes('discovered_devices') && archive.discovered_devices) {
    filtered.discovered_devices = archive.discovered_devices
    recordCount += archive.discovered_devices.length
  }

  if (sections.includes('network_settings') && archive.network_settings) {
    filtered.network_settings = archive.network_settings
  }

  if (sections.includes('plugin_configurations') && archive.plugin_configurations) {
    filtered.plugin_configurations = archive.plugin_configurations
  }

  if (sections.includes('system_settings') && archive.system_settings) {
    filtered.system_settings = archive.system_settings
  }

  // Update record count
  if (filtered.metadata) {
    filtered.metadata.integrity.record_count = recordCount
  }

  return filtered
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
 * Format bytes to human readable string
 */
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

/**
 * Check if browser supports required features
 */
export function isSMAParsingSupported(): boolean {
  return typeof crypto !== 'undefined' && 
         typeof pako !== 'undefined' &&
         typeof performance !== 'undefined'
}