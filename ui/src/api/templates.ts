import api from './client'
import type { APIResponse } from './types'

export type TemplateScope = 'global' | 'group' | 'device_type'

export interface DeviceConfiguration {
  [key: string]: any
}

export interface ConfigTemplate {
  id: number
  name: string
  description?: string
  scope: TemplateScope | string
  device_type?: string
  config: DeviceConfiguration
  created_at: string
  updated_at: string
  has_wifi_password?: boolean
  has_mqtt_password?: boolean
  has_auth_password?: boolean
  // Schema paths of the secrets the template holds, for example "wifi.ap.pass". The API never
  // returns their values. A save that leaves one out keeps it; an empty string clears it.
  stored_secrets?: string[]
}

export interface DeviceTypeInfo {
  device_type: string
  name: string
  generation: number
}

// listDeviceTypes returns the models a device-type template can name.
export async function listDeviceTypes(): Promise<DeviceTypeInfo[]> {
  const res = await api.get<APIResponse<{ device_types: DeviceTypeInfo[] }>>('/config/device-types')
  if (!res.data.success || !res.data.data?.device_types) {
    throw new Error(res.data.error?.message || 'Failed to load device types')
  }
  return res.data.data.device_types
}

export interface ListTemplatesParams {
  scope?: TemplateScope | string
}

export async function listTemplates(params: ListTemplatesParams = {}): Promise<ConfigTemplate[]> {
  const res = await api.get<APIResponse<{ templates: ConfigTemplate[] }>>('/config/templates/new', {
    params: {
      scope: params.scope,
    },
  })

  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load templates'
    throw new Error(msg)
  }

  return res.data.data?.templates || []
}

export async function getTemplate(id: number | string): Promise<ConfigTemplate> {
  const res = await api.get<APIResponse<{ template: ConfigTemplate }>>(`/config/templates/new/${id}`)

  if (!res.data.success || !res.data.data?.template) {
    const msg = res.data.error?.message || 'Failed to load template'
    throw new Error(msg)
  }

  return res.data.data.template
}

// FieldError is one config field the server refused (internal/configuration FieldError).
export interface FieldError {
  path: string
  message: string
}

// TemplateSaveError carries the field errors of a refused template body, so the page can show
// each one next to its path.
export class TemplateSaveError extends Error {
  fields: FieldError[]
  constructor(message: string, fields: FieldError[]) {
    super(message)
    this.name = 'TemplateSaveError'
    this.fields = fields
  }
}

// toSaveError turns a refused create or update into a TemplateSaveError. The server answers a
// schema failure with 400 and error.details.fields. The device override routes answer the same
// way, so they use it too.
export function toSaveError(e: any, fallback: string): Error {
  const error = e?.response?.data?.error
  const fields = error?.details?.fields
  if (Array.isArray(fields)) return new TemplateSaveError(error?.message || fallback, fields)
  return new Error(error?.message || e?.message || fallback)
}

export interface CreateTemplateRequest {
  name: string
  description?: string
  scope: TemplateScope | string
  device_type?: string
  config: DeviceConfiguration
}

export async function createTemplate(data: CreateTemplateRequest): Promise<ConfigTemplate> {
  let res
  try {
    res = await api.post<APIResponse<{ template: ConfigTemplate }>>('/config/templates/new', data)
  } catch (e) {
    throw toSaveError(e, 'Failed to create template')
  }

  if (!res.data.success || !res.data.data?.template) {
    const msg = res.data.error?.message || 'Failed to create template'
    throw new Error(msg)
  }

  return res.data.data.template
}

export interface UpdateTemplateRequest {
  name?: string
  description?: string
  config?: DeviceConfiguration
}

export interface UpdateTemplateResult {
  template: ConfigTemplate
  affected_devices?: number
}

export async function updateTemplate(id: number | string, data: UpdateTemplateRequest): Promise<UpdateTemplateResult> {
  let res
  try {
    res = await api.put<APIResponse<UpdateTemplateResult>>(`/config/templates/new/${id}`, data)
  } catch (e) {
    throw toSaveError(e, 'Failed to update template')
  }

  if (!res.data.success || !res.data.data?.template) {
    const msg = res.data.error?.message || 'Failed to update template'
    throw new Error(msg)
  }

  return res.data.data
}

export async function deleteTemplate(id: number | string): Promise<void> {
  const res = await api.delete(`/config/templates/new/${id}`)
  if (res.status !== 204) {
    const msg = (res.data as any)?.error?.message || 'Failed to delete template'
    throw new Error(msg)
  }
}

export interface AssignTemplateToDeviceParams {
  deviceId: number | string
  templateId: number | string
  position?: number
}

export async function assignTemplateToDevice(params: AssignTemplateToDeviceParams): Promise<void> {
  const res = await api.post<APIResponse<{ templates: ConfigTemplate[] }>>(
    `/devices/${params.deviceId}/templates/new/${params.templateId}`,
    null,
    {
      params: {
        position: params.position,
      },
    },
  )

  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to assign template to device'
    throw new Error(msg)
  }
}
