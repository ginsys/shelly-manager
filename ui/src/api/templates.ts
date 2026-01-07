import api from './client'
import type { APIResponse, Metadata } from './types'

// Template interfaces
export interface ConfigTemplate {
  id: number
  name: string
  description?: string
  deviceType: string
  templateContent: string
  variables?: Record<string, any>
  createdAt: string
  updatedAt: string
}

export interface ListTemplatesParams {
  page?: number
  pageSize?: number
  deviceType?: string
  search?: string
}

export interface ListTemplatesResult {
  items: ConfigTemplate[]
  meta?: Metadata
}

// List configuration templates
export async function listTemplates(params: ListTemplatesParams = {}): Promise<ListTemplatesResult> {
  const { page = 1, pageSize = 25, deviceType, search } = params
  const res = await api.get<APIResponse<{ templates: ConfigTemplate[] }>>('/config/templates', {
    params: {
      page,
      page_size: pageSize,
      device_type: deviceType,
      search
    }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load templates'
    throw new Error(msg)
  }
  return {
    items: res.data.data?.templates || [],
    meta: res.data.meta
  }
}

// Get single configuration template
export async function getTemplate(id: number | string): Promise<ConfigTemplate> {
  const res = await api.get<APIResponse<ConfigTemplate>>(`/config/templates/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load template'
    throw new Error(msg)
  }
  return res.data.data
}

// Create configuration template
export async function createTemplate(data: Partial<ConfigTemplate>): Promise<ConfigTemplate> {
  const res = await api.post<APIResponse<ConfigTemplate>>('/config/templates', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create template'
    throw new Error(msg)
  }
  return res.data.data
}

// Update configuration template
export async function updateTemplate(id: number | string, data: Partial<ConfigTemplate>): Promise<ConfigTemplate> {
  const res = await api.put<APIResponse<ConfigTemplate>>(`/config/templates/${id}`, data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update template'
    throw new Error(msg)
  }
  return res.data.data
}

// Delete configuration template
export async function deleteTemplate(id: number | string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/config/templates/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete template'
    throw new Error(msg)
  }
}
