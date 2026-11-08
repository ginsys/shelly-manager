import api from './client'
import type { APIResponse } from './types'

// Typed configuration interfaces
export interface TypedConfig {
  deviceId: string
  deviceType: string
  config: Record<string, any>
  schema?: ConfigSchema
  validationErrors?: ValidationError[]
}

export interface ConfigSchema {
  type: string
  properties: Record<string, SchemaProperty>
  required?: string[]
  // Device-type schema only: the model it was built for, and whether apply supports the model.
  'x-device-type'?: string
  'x-apply-supported'?: boolean
}

export interface SchemaProperty {
  type: string
  // Present on section-level schemas: the backend GetConfigurationSchema emits a
  // human title and nested properties for each config section (JSON-Schema style,
  // internal/configuration/typed_models.go). The frontend ConfigEditor renders both.
  title?: string
  properties?: Record<string, SchemaProperty>
  description?: string
  enum?: any[]
  default?: any
  minimum?: number
  maximum?: number
  // Also emitted by the backend at property level (typed_models.go) and read by
  // ConfigEditor: string format hints (e.g. "password", "ipv4") and a max length.
  format?: string
  maxLength?: number
  pattern?: string
  // Object schemas list required child keys; array schemas give their item schema.
  required?: string[]
  // Section schemas (wifi, auth) require more keys when a switch is on: `then.required` applies
  // when every `if.properties.<key>.const` equals the value of that key.
  if?: { properties?: Record<string, { const?: any }> }
  then?: { required?: string[] }
  items?: SchemaProperty
  // Device-type schema only (GET /config/schema?device_id=...): false when apply cannot write
  // the field to the device yet. The generic schema omits it.
  'x-applicable'?: boolean
}

export interface DeviceCapabilities {
  deviceId: string
  deviceType: string
  capabilities: string[]
  supportedFeatures: Record<string, boolean>
  firmwareVersion?: string
}

export interface ValidationError {
  field: string
  message: string
  severity: 'error' | 'warning'
}

export interface ValidationResult {
  valid: boolean
  errors?: ValidationError[]
  warnings?: ValidationError[]
}

export interface ConversionRequest {
  config: Record<string, any>
  deviceType?: string
}

export interface BulkValidationRequest {
  configs: Array<{
    deviceId: string
    config: Record<string, any>
  }>
}

export interface BulkValidationResult {
  results: Array<{
    deviceId: string
    valid: boolean
    errors?: ValidationError[]
    warnings?: ValidationError[]
  }>
  summary: {
    total: number
    valid: number
    invalid: number
  }
}

// Get typed configuration for a device
export async function getTypedConfig(deviceId: number | string): Promise<TypedConfig> {
  const res = await api.get<APIResponse<TypedConfig>>(`/devices/${deviceId}/config/typed`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get typed configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Update typed configuration for a device
export async function updateTypedConfig(deviceId: number | string, config: Record<string, any>): Promise<TypedConfig> {
  const res = await api.put<APIResponse<TypedConfig>>(`/devices/${deviceId}/config/typed`, { config })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update typed configuration'
    throw new Error(msg)
  }
  return res.data.data
}

// Get device capabilities
export async function getDeviceCapabilities(deviceId: number | string): Promise<DeviceCapabilities> {
  const res = await api.get<APIResponse<DeviceCapabilities>>(`/devices/${deviceId}/capabilities`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get device capabilities'
    throw new Error(msg)
  }
  return res.data.data
}

export async function validateTypedConfig(request: ConversionRequest): Promise<ValidationResult> {
  const res = await api.post<APIResponse<ValidationResult>>('/config/validate-typed', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to validate typed configuration'
    throw new Error(msg)
  }
  return res.data.data
}

export async function convertToTyped(request: ConversionRequest): Promise<TypedConfig> {
  const res = await api.post<APIResponse<TypedConfig>>('/config/convert-to-typed', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to convert to typed configuration'
    throw new Error(msg)
  }
  return res.data.data
}

export async function convertToRaw(request: ConversionRequest): Promise<Record<string, any>> {
  const res = await api.post<APIResponse<{ config: Record<string, any> }>>('/config/convert-to-raw', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to convert to raw configuration'
    throw new Error(msg)
  }
  return res.data.data.config
}

// getConfigSchema returns the generic schema, or with a device type or a device id the schema of
// the configuration stored for that model. A device id resolves the model on the server, the way
// apply does.
// getConfigSchema asks for the schema of one device (deviceId), one model (deviceType), or a
// global or group template (scope). The first one given wins. With none it gets the generic schema.
export async function getConfigSchema(
  deviceType?: string,
  deviceId?: number | string,
  scope?: string,
): Promise<ConfigSchema> {
  let params: Record<string, unknown> = {}
  if (deviceId !== undefined && deviceId !== '') params = { device_id: deviceId }
  else if (deviceType) params = { device_type: deviceType }
  else if (scope) params = { scope }
  const res = await api.get<APIResponse<ConfigSchema>>('/config/schema', { params })
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to get configuration schema'
    throw new Error(msg)
  }
  return res.data.data
}

export async function bulkValidateConfigs(request: BulkValidationRequest): Promise<BulkValidationResult> {
  const res = await api.post<APIResponse<BulkValidationResult>>('/config/bulk-validate', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to bulk validate configurations'
    throw new Error(msg)
  }
  return res.data.data
}
