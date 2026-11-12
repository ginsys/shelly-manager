import api from './client'
import type { APIResponse } from './types'

// Typed configuration interfaces
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

// The body of GET /devices/{id}/capabilities. The server sends it bare, with no
// {success, data} envelope, and the names are snake_case.
export interface DeviceCapabilities {
  device_id: number
  device_model: string
  generation: number
  capabilities: string[]
}

// Get device capabilities
export async function getDeviceCapabilities(deviceId: number | string, signal?: AbortSignal): Promise<DeviceCapabilities> {
  const res = await api.get<DeviceCapabilities>(`/devices/${deviceId}/capabilities`, { signal })
  if (!Array.isArray(res.data?.capabilities)) {
    throw new Error('Failed to get device capabilities')
  }
  return res.data
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
