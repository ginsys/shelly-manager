import { Inflate } from 'pako'
import { canonicalize } from './jcs'
import { sha256Hex } from './sha256'
import { parseStrictJSON } from './strict-json'

const DEFAULT_MAX_SIZE = 100 * 1024 * 1024
const FORMAT_VERSION = '2026.1'
const timestamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
const checksum = /^sha256:[0-9a-f]{64}$/

export interface SMAMetadata {
  export_id: string
  created_at: string
  created_by: string
  export_type: 'manual' | 'api'
  system_info: {
    version: string
    database_type: 'sqlite' | 'postgresql' | 'mysql'
    hostname: string
    total_size_bytes: 0
    compression_ratio: 0
  }
  integrity: {
    checksum: string
    record_count: number
    file_count: 1
  }
}

export interface SMADeviceConfiguration {
  device_id: number
  template_id?: number
  config: Record<string, unknown>
  last_synced?: string
  sync_status: string
  updated_at: string
}

export interface SMADevice {
  id: number
  mac: string
  ip: string
  type: string
  name: string
  model: string
  firmware: string
  status: string
  last_seen: string
  settings: Record<string, unknown>
  configuration?: SMADeviceConfiguration
  created_at: string
  updated_at: string
}

export interface SMATemplate {
  id: number
  generation: number
  name: string
  description: string
  device_type: string
  config: Record<string, unknown>
  variables: Record<string, unknown>
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface SMADiscoveredDevice {
  mac: string
  ssid: string
  model: string
  ip: string
  agent_id: string
  generation: number
  signal: number
  discovered: string
}

export interface SMANetworkSettings {
  wifi_networks: Array<{ ssid: string; security: string; priority: number }>
  ntp_servers: string[]
  mqtt_config?: {
    server: string
    username: string
    port: number
    retain: boolean
    qos: number
  }
}

export interface SMAPluginConfig {
  plugin_name: string
  version: string
  config: Record<string, unknown>
  enabled: boolean
}

export interface SMASystemSettings {
  log_level: string
  api_settings: Record<string, unknown>
  database_settings: Record<string, unknown>
}

export interface SMAArchive {
  format_version: '2026.1'
  metadata: SMAMetadata
  devices: SMADevice[]
  templates: SMATemplate[]
  discovered_devices: SMADiscoveredDevice[]
  network_settings: SMANetworkSettings
  plugin_configurations: SMAPluginConfig[]
  system_settings: SMASystemSettings
}

export interface SMAParseOptions {
  maxSizeBytes?: number
}

export interface SMAParseResult {
  success: boolean
  archive?: SMAArchive
  error?: string
  parseInfo: {
    originalSize: number
    compressedSize: number
    compressionRatio: number
    parseTimeMs: number
  }
}

export async function parseSMAFile(buffer: ArrayBuffer, options: SMAParseOptions = {}): Promise<SMAParseResult> {
  const started = performance.now()
  const input = new Uint8Array(buffer)
  const limit = options.maxSizeBytes ?? DEFAULT_MAX_SIZE
  let normalized: Uint8Array<ArrayBufferLike> = new Uint8Array()
  try {
    if (!Number.isSafeInteger(limit) || limit <= 0) throw new Error('maxSizeBytes must be a positive safe integer')
    normalized = isGzip(input) ? inflateBounded(input, limit) : input
    if (normalized.byteLength > limit) throw new Error('normalized SMA data exceeds the configured limit')
    const text = new TextDecoder('utf-8', { fatal: true }).decode(normalized)
    const parsed = parseStrictJSON(text)
    validateSMAArchive(parsed)
    const archive = parsed as SMAArchive
    const supplied = archive.metadata.integrity.checksum
    archive.metadata.integrity.checksum = ''
    const actual = `sha256:${await sha256Hex(canonicalize(archive))}`
    archive.metadata.integrity.checksum = supplied
    if (actual !== supplied) throw new Error('SMA checksum mismatch')
    return {
      success: true,
      archive,
      parseInfo: info(normalized.byteLength, input.byteLength, started),
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
      parseInfo: info(normalized.byteLength, input.byteLength, started),
    }
  }
}

export async function parseSMAFromFile(file: File, options: SMAParseOptions = {}): Promise<SMAParseResult> {
  try {
    return await parseSMAFile(await file.arrayBuffer(), options)
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
      parseInfo: { originalSize: 0, compressedSize: file.size, compressionRatio: 0, parseTimeMs: 0 },
    }
  }
}

function isGzip(data: Uint8Array): boolean {
  return data.byteLength >= 2 && data[0] === 0x1f && data[1] === 0x8b
}

function inflateBounded(data: Uint8Array, limit: number): Uint8Array {
  const inflate = new Inflate()
  const chunks: Uint8Array[] = []
  let total = 0
  inflate.onData = chunk => {
    if (total + chunk.byteLength > limit) {
      throw new Error('normalized SMA data exceeds the configured limit')
    }
    chunks.push(chunk)
    total += chunk.byteLength
  }
  inflate.push(data, true)
  if (inflate.err) throw new Error(inflate.msg || 'malformed gzip input')
  const output = new Uint8Array(total)
  let offset = 0
  for (const chunk of chunks) {
    output.set(chunk, offset)
    offset += chunk.byteLength
  }
  return output
}

export function validateSMAArchive(value: unknown): asserts value is SMAArchive {
  const root = object(value, 'root')
  if (root.format_version !== FORMAT_VERSION) throw new Error(`format_version must be ${FORMAT_VERSION}`)
  keys(root, [
    'format_version', 'metadata', 'devices', 'templates', 'discovered_devices',
    'network_settings', 'plugin_configurations', 'system_settings',
  ])
  const metadata = object(root.metadata, 'metadata')
  keys(metadata, ['export_id', 'created_at', 'created_by', 'export_type', 'system_info', 'integrity'])
  string(metadata.export_id, 'export_id', false)
  if (!uuid.test(metadata.export_id as string)) throw new Error('export_id must be a lowercase UUID')
  canonicalTimestamp(metadata.created_at, 'created_at')
  string(metadata.created_by, 'created_by', false)
  if (metadata.export_type !== 'manual' && metadata.export_type !== 'api') throw new Error('invalid export_type')
  const system = object(metadata.system_info, 'system_info')
  keys(system, ['version', 'database_type', 'hostname', 'total_size_bytes', 'compression_ratio'])
  string(system.version, 'version', false)
  string(system.hostname, 'hostname', false)
  if (!['sqlite', 'postgresql', 'mysql'].includes(system.database_type as string)) throw new Error('invalid database_type')
  if (system.total_size_bytes !== 0 || system.compression_ratio !== 0) throw new Error('system size values must be zero')
  const integrity = object(metadata.integrity, 'integrity')
  keys(integrity, ['checksum', 'record_count', 'file_count'])
  if (typeof integrity.checksum !== 'string' || !checksum.test(integrity.checksum)) throw new Error('invalid checksum')
  safeInteger(integrity.record_count, 'record_count')
  if (integrity.file_count !== 1) throw new Error('file_count must be 1')

  const devices = array(root.devices, 'devices')
  devices.forEach((entry, index) => validateDevice(object(entry, `devices[${index}]`)))
  const templates = array(root.templates, 'templates')
  templates.forEach((entry, index) => validateTemplate(object(entry, `templates[${index}]`)))
  const discovered = array(root.discovered_devices, 'discovered_devices')
  discovered.forEach((entry, index) => validateDiscovered(object(entry, `discovered_devices[${index}]`)))
  if (devices.length + templates.length + discovered.length === 0) throw new Error('SMA archive is empty')
  if (integrity.record_count !== devices.length + templates.length + discovered.length) {
    throw new Error('record_count does not match archive contents')
  }
  validateNetwork(object(root.network_settings, 'network_settings'))
  array(root.plugin_configurations, 'plugin_configurations').forEach(entry => {
    const plugin = object(entry, 'plugin configuration')
    keys(plugin, ['plugin_name', 'version', 'config', 'enabled'])
    string(plugin.plugin_name, 'plugin_name', true)
    string(plugin.version, 'version', true)
    object(plugin.config, 'config')
    if (typeof plugin.enabled !== 'boolean') throw new Error('enabled must be boolean')
  })
  const settings = object(root.system_settings, 'system_settings')
  keys(settings, ['log_level', 'api_settings', 'database_settings'])
  string(settings.log_level, 'log_level', false)
  object(settings.api_settings, 'api_settings')
  object(settings.database_settings, 'database_settings')
}

function validateDevice(device: Record<string, unknown>): void {
  keys(device, [
    'id', 'mac', 'ip', 'type', 'name', 'model', 'firmware', 'status', 'last_seen',
    'settings', 'created_at', 'updated_at',
  ], ['configuration'])
  safeInteger(device.id, 'id')
  for (const name of ['mac', 'ip', 'type', 'name', 'model', 'firmware', 'status']) string(device[name], name, true)
  for (const name of ['last_seen', 'created_at', 'updated_at']) canonicalTimestamp(device[name], name)
  object(device.settings, 'settings')
  if ('configuration' in device) {
    const config = object(device.configuration, 'configuration')
    keys(config, ['device_id', 'config', 'sync_status', 'updated_at'], ['template_id', 'last_synced'])
    safeInteger(config.device_id, 'device_id')
    if ('template_id' in config) safeInteger(config.template_id, 'template_id')
    object(config.config, 'config')
    string(config.sync_status, 'sync_status', true)
    canonicalTimestamp(config.updated_at, 'updated_at')
    if ('last_synced' in config) canonicalTimestamp(config.last_synced, 'last_synced')
  }
}

function validateTemplate(template: Record<string, unknown>): void {
  keys(template, [
    'id', 'generation', 'name', 'description', 'device_type', 'config', 'variables',
    'is_default', 'created_at', 'updated_at',
  ])
  safeInteger(template.id, 'id')
  safeInteger(template.generation, 'generation')
  for (const name of ['name', 'description', 'device_type']) string(template[name], name, true)
  object(template.config, 'config')
  object(template.variables, 'variables')
  if (typeof template.is_default !== 'boolean') throw new Error('is_default must be boolean')
  canonicalTimestamp(template.created_at, 'created_at')
  canonicalTimestamp(template.updated_at, 'updated_at')
}

function validateDiscovered(device: Record<string, unknown>): void {
  keys(device, ['mac', 'ssid', 'model', 'ip', 'agent_id', 'generation', 'signal', 'discovered'])
  for (const name of ['mac', 'ssid', 'model', 'ip', 'agent_id']) string(device[name], name, true)
  safeInteger(device.generation, 'generation')
  safeInteger(device.signal, 'signal', true)
  canonicalTimestamp(device.discovered, 'discovered')
}

function validateNetwork(network: Record<string, unknown>): void {
  keys(network, ['wifi_networks', 'ntp_servers'], ['mqtt_config'])
  array(network.wifi_networks, 'wifi_networks').forEach(entry => {
    const wifi = object(entry, 'wifi entry')
    keys(wifi, ['ssid', 'security', 'priority'])
    string(wifi.ssid, 'ssid', true)
    string(wifi.security, 'security', true)
    safeInteger(wifi.priority, 'priority')
  })
  array(network.ntp_servers, 'ntp_servers').forEach(server => string(server, 'ntp server', true))
  if ('mqtt_config' in network) {
    const mqtt = object(network.mqtt_config, 'mqtt_config')
    keys(mqtt, ['server', 'username', 'port', 'retain', 'qos'])
    string(mqtt.server, 'server', true)
    string(mqtt.username, 'username', true)
    safeInteger(mqtt.port, 'port')
    safeInteger(mqtt.qos, 'qos')
    if ((mqtt.port as number) < 1 || (mqtt.port as number) > 65535) throw new Error('invalid MQTT port')
    if ((mqtt.qos as number) < 0 || (mqtt.qos as number) > 2) throw new Error('invalid MQTT qos')
    if (typeof mqtt.retain !== 'boolean') throw new Error('retain must be boolean')
  }
}

function object(value: unknown, name: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${name} must be a non-null object`)
  return value as Record<string, unknown>
}

function array(value: unknown, name: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`${name} must be a non-null array`)
  return value
}

function keys(value: Record<string, unknown>, required: string[], optional: string[] = []): void {
  const allowed = new Set([...required, ...optional])
  for (const name of required) if (!Object.prototype.hasOwnProperty.call(value, name)) throw new Error(`missing required field ${name}`)
  for (const name of Object.keys(value)) if (!allowed.has(name)) throw new Error(`unknown field ${name}`)
}

function string(value: unknown, name: string, empty: boolean): asserts value is string {
  if (typeof value !== 'string' || (!empty && value.length === 0)) throw new Error(`${name} must be a string`)
}

function safeInteger(value: unknown, name: string, signed = false): asserts value is number {
  if (!Number.isSafeInteger(value) || (!signed && (value as number) < 0)) throw new Error(`${name} must be a safe integer`)
}

function canonicalTimestamp(value: unknown, name: string): void {
  string(value, name, false)
  const match = timestamp.exec(value)
  if (!match || Number.isNaN(Date.parse(value))) throw new Error(`${name} must be a canonical UTC timestamp`)
  const [date, clock] = value.split('T')
  const [year, month, day] = date.split('-').map(Number)
  const [hour, minute, second] = clock.slice(0, 8).split(':').map(Number)
  const maxDay = new Date(Date.UTC(year, month, 0)).getUTCDate()
  if (month < 1 || month > 12 || day < 1 || day > maxDay ||
      hour > 23 || minute > 59 || second > 59) {
    throw new Error(`${name} must be a real calendar instant`)
  }
}

function info(originalSize: number, compressedSize: number, started: number) {
  return {
    originalSize,
    compressedSize,
    compressionRatio: originalSize === 0 ? 0 : compressedSize / originalSize,
    parseTimeMs: performance.now() - started,
  }
}

export function isSMAParsingSupported(): boolean {
  return typeof TextDecoder !== 'undefined' && typeof Inflate === 'function'
}
