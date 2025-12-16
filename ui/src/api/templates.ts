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

export interface TemplateExample {
  name: string
  description: string
  deviceType: string
  content: string
  variables?: Record<string, any>
}

export interface TemplatePreviewRequest {
  templateContent: string
  variables?: Record<string, any>
}

export interface TemplatePreviewResult {
  renderedConfig: Record<string, any>
  errors?: string[]
}

export interface TemplateValidationRequest {
  templateContent: string
  deviceType?: string
}

export interface TemplateValidationResult {
  valid: boolean
  errors?: string[]
  warnings?: string[]
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

// Preview template rendering
export async function previewTemplate(request: TemplatePreviewRequest): Promise<TemplatePreviewResult> {
  const res = await api.post<APIResponse<TemplatePreviewResult>>('/configuration/preview-template', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to preview template'
    throw new Error(msg)
  }
  return res.data.data
}

// Validate template syntax
export async function validateTemplate(request: TemplateValidationRequest): Promise<TemplateValidationResult> {
  const res = await api.post<APIResponse<TemplateValidationResult>>('/configuration/validate-template', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to validate template'
    throw new Error(msg)
  }
  return res.data.data
}

// Save template (alternate endpoint)
export async function saveTemplate(data: Partial<ConfigTemplate>): Promise<ConfigTemplate> {
  const res = await api.post<APIResponse<ConfigTemplate>>('/configuration/templates', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to save template'
    throw new Error(msg)
  }
  return res.data.data
}

// Get example templates
export async function getTemplateExamples(): Promise<TemplateExample[]> {
  const res = await api.get<APIResponse<{ examples: TemplateExample[] }>>('/configuration/template-examples')
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load template examples'
    throw new Error(msg)
  }
  return res.data.data.examples || []
}
