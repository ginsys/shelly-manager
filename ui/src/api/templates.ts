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

export interface CreateTemplateRequest {
  name: string
  description?: string
  scope: TemplateScope | string
  device_type?: string
  config: DeviceConfiguration
}

export async function createTemplate(data: CreateTemplateRequest): Promise<ConfigTemplate> {
  const res = await api.post<APIResponse<{ template: ConfigTemplate }>>('/config/templates/new', data)

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
  const res = await api.put<APIResponse<UpdateTemplateResult>>(`/config/templates/new/${id}`, data)

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
