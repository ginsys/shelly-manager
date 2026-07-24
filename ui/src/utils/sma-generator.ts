import { gzip } from 'pako'
import { canonicalize } from './jcs'
import { sha256Hex } from './sha256'
import { validateSMAArchive } from './sma-parser'
import type {
  SMAArchive,
  SMADevice,
  SMADiscoveredDevice,
  SMANetworkSettings,
  SMAPluginConfig,
  SMASystemSettings,
  SMATemplate,
} from './sma-parser'

export interface SMAGenerateOptions {
  compressionLevel?: number
}

export interface SMADataSources {
  devices?: SMADevice[]
  templates?: SMATemplate[]
  discoveredDevices?: SMADiscoveredDevice[]
  networkSettings?: SMANetworkSettings
  pluginConfigurations?: SMAPluginConfig[]
  systemSettings?: SMASystemSettings
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

export async function generateSMAFile(
  sources: SMADataSources,
  options: SMAGenerateOptions = {},
): Promise<SMAGenerateResult> {
  const started = performance.now()
  try {
    const level = options.compressionLevel ?? 6
    if (!Number.isInteger(level) || level < 1 || level > 9) {
      throw new Error('compressionLevel must be an integer from 1 through 9')
    }
    const devices = normalizeArray(sources.devices) as SMADevice[]
    const templates = normalizeArray(sources.templates) as SMATemplate[]
    const discovered = normalizeArray(sources.discoveredDevices) as SMADiscoveredDevice[]
    const recordCount = devices.length + templates.length + discovered.length
    if (recordCount === 0) throw new Error('SMA archive must not be empty')

    const createdAt = new Date().toISOString()
    const archive = Object.assign(Object.create(null), {
      format_version: '2026.1',
      metadata: Object.assign(Object.create(null), {
        export_id: crypto.randomUUID().toLowerCase(),
        created_at: createdAt,
        created_by: 'shelly-manager-ui',
        export_type: 'manual',
        system_info: Object.assign(Object.create(null), {
          version: 'unknown',
          database_type: 'sqlite',
          hostname: 'shelly-manager-ui',
          total_size_bytes: 0,
          compression_ratio: 0,
        }),
        integrity: Object.assign(Object.create(null), {
          checksum: '',
          record_count: recordCount,
          file_count: 1,
        }),
      }),
      devices: devices.map(buildDevice),
      templates: templates.map(buildTemplate),
      discovered_devices: discovered.map(buildDiscovered),
      network_settings: buildNetwork(sources.networkSettings),
      plugin_configurations: (sources.pluginConfigurations ?? []).map(plugin => Object.assign(Object.create(null), {
        plugin_name: plugin.plugin_name,
        version: plugin.version,
        config: normalizeJSON(plugin.config ?? {}),
        enabled: plugin.enabled,
      })),
      system_settings: Object.assign(Object.create(null), {
        log_level: sources.systemSettings?.log_level ?? 'info',
        api_settings: normalizeJSON(sources.systemSettings?.api_settings ?? {}),
        database_settings: normalizeJSON(sources.systemSettings?.database_settings ?? {}),
      }),
    }) as SMAArchive

    const digest = await sha256Hex(canonicalize(archive))
    const checksum = `sha256:${digest}`
    archive.metadata.integrity.checksum = checksum
    validateGeneratedArchive(archive)
    const canonical = canonicalize(archive)
    const encoded = new TextEncoder().encode(canonical)
    const compressed = gzip(encoded, { level })
    const blob = new Blob([compressed], { type: 'application/gzip' })
    return {
      success: true,
      blob,
      filename: `shelly-manager-${createdAt.replace(/[-:.]/g, '').replace('Z', 'Z')}.sma`,
      metadata: {
        originalSize: encoded.byteLength,
        compressedSize: compressed.byteLength,
        compressionRatio: compressed.byteLength / encoded.byteLength,
        checksum,
        generateTimeMs: performance.now() - started,
        recordCount,
      },
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
      metadata: {
        originalSize: 0,
        compressedSize: 0,
        compressionRatio: 0,
        generateTimeMs: performance.now() - started,
        recordCount: 0,
      },
    }
  }
}

export async function downloadSMAFile(
  sources: SMADataSources,
  options: SMAGenerateOptions = {},
): Promise<SMAGenerateResult> {
  const result = await generateSMAFile(sources, options)
  if (result.success && result.blob && result.filename) {
    const url = URL.createObjectURL(result.blob)
    const link = document.createElement('a')
    link.href = url
    link.download = result.filename
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }
  return result
}

export async function createSMAForUpload(
  sources: SMADataSources,
  options: SMAGenerateOptions = {},
): Promise<{ file?: File; error?: string; metadata?: SMAGenerateResult['metadata'] }> {
  const result = await generateSMAFile(sources, options)
  if (!result.success || !result.blob || !result.filename) return { error: result.error }
  return {
    file: new File([result.blob], result.filename, { type: 'application/gzip' }),
    metadata: result.metadata,
  }
}

function normalizeArray<T>(value: T[] | undefined): T[] {
  return value ?? []
}

function buildDevice(device: SMADevice): Record<string, unknown> {
  const result = Object.assign(Object.create(null), {
    id: device.id,
    mac: device.mac ?? '',
    ip: device.ip ?? '',
    type: device.type ?? '',
    name: device.name ?? '',
    model: device.model ?? '',
    firmware: device.firmware ?? '',
    status: device.status ?? '',
    last_seen: normalizeTimestamp(device.last_seen),
    settings: normalizeJSON(device.settings ?? {}),
    created_at: normalizeTimestamp(device.created_at),
    updated_at: normalizeTimestamp(device.updated_at),
  }) as Record<string, unknown>
  if (device.configuration != null) {
    const configuration = Object.assign(Object.create(null), {
      device_id: device.configuration.device_id,
      config: normalizeJSON(device.configuration.config ?? {}),
      sync_status: device.configuration.sync_status ?? '',
      updated_at: normalizeTimestamp(device.configuration.updated_at),
    }) as Record<string, unknown>
    if (device.configuration.template_id != null) configuration.template_id = device.configuration.template_id
    if (device.configuration.last_synced != null) {
      configuration.last_synced = normalizeTimestamp(device.configuration.last_synced)
    }
    result.configuration = configuration
  }
  return result
}

function buildTemplate(template: SMATemplate): Record<string, unknown> {
  return Object.assign(Object.create(null), {
    id: template.id,
    generation: template.generation,
    name: template.name ?? '',
    description: template.description ?? '',
    device_type: template.device_type ?? '',
    config: normalizeJSON(template.config ?? {}),
    variables: normalizeJSON(template.variables ?? {}),
    is_default: template.is_default,
    created_at: normalizeTimestamp(template.created_at),
    updated_at: normalizeTimestamp(template.updated_at),
  })
}

function buildDiscovered(device: SMADiscoveredDevice): Record<string, unknown> {
  return Object.assign(Object.create(null), {
    mac: device.mac ?? '',
    ssid: device.ssid ?? '',
    model: device.model ?? '',
    ip: device.ip ?? '',
    agent_id: device.agent_id ?? '',
    generation: device.generation,
    signal: device.signal,
    discovered: normalizeTimestamp(device.discovered),
  })
}

function buildNetwork(network?: SMANetworkSettings): Record<string, unknown> {
  const result = Object.assign(Object.create(null), {
    wifi_networks: (network?.wifi_networks ?? []).map(entry => Object.assign(Object.create(null), {
      ssid: entry.ssid ?? '',
      security: entry.security ?? '',
      priority: entry.priority,
    })),
    ntp_servers: [...(network?.ntp_servers ?? [])],
  }) as Record<string, unknown>
  if (network?.mqtt_config != null) {
    result.mqtt_config = Object.assign(Object.create(null), {
      server: network.mqtt_config.server ?? '',
      username: network.mqtt_config.username ?? '',
      port: network.mqtt_config.port,
      retain: network.mqtt_config.retain,
      qos: network.mqtt_config.qos,
    })
  }
  return result
}

function normalizeTimestamp(value: unknown): string {
  if (value instanceof Date) return value.toISOString()
  if (typeof value !== 'string') throw new Error('timestamp must be a string or Date')
  return value
}

function normalizeJSON(value: unknown, active = new Set<object>(), depth = 1): unknown {
  if (value instanceof Date) return value.toISOString()
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return value
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) throw new Error('numbers must be finite')
    if (Number.isInteger(value) && !Number.isSafeInteger(value)) throw new Error('integer is outside the safe range')
    return value
  }
  if (typeof value !== 'object') throw new Error(`unsupported JSON value: ${typeof value}`)
  if (depth > 64) throw new Error('maximum JSON depth 64 exceeded')
  if (active.has(value)) throw new Error('cycle detected')
  active.add(value)
  try {
    if (Array.isArray(value)) return value.map(item => normalizeJSON(item, active, depth + 1))
    const result = Object.create(null) as Record<string, unknown>
    for (const [key, child] of Object.entries(value)) {
      result[key] = normalizeJSON(child, active, depth + 1)
    }
    return result
  } finally {
    active.delete(value)
  }
}

function validateGeneratedArchive(archive: SMAArchive): void {
  validateSMAArchive(archive)
}
