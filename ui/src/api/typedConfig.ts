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
}

export interface SchemaProperty {
  type: string
  description?: string
  enum?: any[]
  default?: any
  minimum?: number
  maximum?: number
  pattern?: string
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

export async function getConfigSchema(deviceType?: string): Promise<ConfigSchema> {
  const res = await api.get<APIResponse<ConfigSchema>>('/config/schema', {
    params: { device_type: deviceType }
  })
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
