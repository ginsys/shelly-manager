import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client
vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn()
    }
  }
})

import api from '../client'
import {
  listTemplates,
  getTemplate,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  previewTemplate,
  validateTemplate,
  saveTemplate,
  getTemplateExamples,
  type ConfigTemplate,
  type TemplateExample,
  type TemplatePreviewResult,
  type TemplateValidationResult
} from '../templates'

describe('templates API', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
    ;(api.delete as any).mockReset()
  })

  describe('getTemplate', () => {
    it('returns a single template', async () => {
      const template: ConfigTemplate = {
        id: 1,
        name: 'Basic WiFi Template',
        description: 'Standard WiFi configuration',
        deviceType: 'shelly1',
        templateContent: '{{ wifi.ssid }}',
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: template,
          timestamp: new Date().toISOString()
        }
      })

      const result = await getTemplate(1)
      expect(result).toEqual(template)
      expect(api.get).toHaveBeenCalledWith('/config/templates/1')
    })

    it('throws error when template not found', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Template not found' }
        }
      })

      await expect(getTemplate(999)).rejects.toThrow('Template not found')
    })
  })

  describe('listTemplates', () => {
    it('returns list of templates', async () => {
      const templates: ConfigTemplate[] = [
        {
          id: 1,
          name: 'Basic WiFi Template',
          description: 'Standard WiFi configuration',
          deviceType: 'shelly1',
          templateContent: '{{ wifi.ssid }}',
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        },
        {
          id: 2,
          name: 'MQTT Template',
          description: 'MQTT broker configuration',
          deviceType: 'shelly1pm',
          templateContent: '{{ mqtt.server }}',
          createdAt: '2023-01-02T00:00:00Z',
          updatedAt: '2023-01-02T00:00:00Z'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { templates },
          meta: { pagination: { page: 1, page_size: 25, total: 2 } },
          timestamp: new Date().toISOString()
        }
      })

      const result = await listTemplates()
      expect(result.items).toHaveLength(2)
      expect(result.items[0].name).toBe('Basic WiFi Template')
      expect(api.get).toHaveBeenCalledWith('/config/templates', {
        params: { page: 1, page_size: 25, device_type: undefined, search: undefined }
      })
    })

    it('supports filtering by device type', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { templates: [] },
          timestamp: new Date().toISOString()
        }
      })

      await listTemplates({ deviceType: 'shelly1pm' })
      expect(api.get).toHaveBeenCalledWith('/config/templates', {
        params: { page: 1, page_size: 25, device_type: 'shelly1pm', search: undefined }
      })
    })

    it('supports search filtering', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { templates: [] },
          timestamp: new Date().toISOString()
        }
      })

      await listTemplates({ search: 'wifi' })
      expect(api.get).toHaveBeenCalledWith('/config/templates', {
        params: { page: 1, page_size: 25, device_type: undefined, search: 'wifi' }
      })
    })

    it('throws error on failure', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Database error' }
        }
      })

      await expect(listTemplates()).rejects.toThrow('Database error')
    })
  })

  describe('createTemplate', () => {
    it('creates a new template', async () => {
      const newTemplate: Partial<ConfigTemplate> = {
        name: 'New Template',
        description: 'Test template',
        deviceType: 'shelly1',
        templateContent: '{{ test }}'
      }

      const createdTemplate: ConfigTemplate = {
        id: 123,
        name: 'New Template',
        description: 'Test template',
        deviceType: 'shelly1',
        templateContent: '{{ test }}',
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: createdTemplate,
          timestamp: new Date().toISOString()
        }
      })

      const result = await createTemplate(newTemplate)
      expect(result).toEqual(createdTemplate)
      expect(api.post).toHaveBeenCalledWith('/config/templates', newTemplate)
    })

    it('throws error on creation failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Invalid template content' }
        }
      })

      await expect(createTemplate({})).rejects.toThrow('Invalid template content')
    })
  })

  describe('updateTemplate', () => {
    it('updates an existing template', async () => {
      const updates: Partial<ConfigTemplate> = {
        name: 'Updated Template'
      }

      const updatedTemplate: ConfigTemplate = {
        id: 123,
        name: 'Updated Template',
        deviceType: 'shelly1',
        templateContent: '{{ test }}',
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-02T00:00:00Z'
      }

      ;(api.put as any).mockResolvedValue({
        data: {
          success: true,
          data: updatedTemplate,
          timestamp: new Date().toISOString()
        }
      })

      const result = await updateTemplate(123, updates)
      expect(result).toEqual(updatedTemplate)
      expect(api.put).toHaveBeenCalledWith('/config/templates/123', updates)
    })

    it('throws error when template not found', async () => {
      ;(api.put as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Template not found' }
        }
      })

      await expect(updateTemplate(999, {})).rejects.toThrow('Template not found')
    })
  })

  describe('deleteTemplate', () => {
    it('deletes a template', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await deleteTemplate(123)
      expect(api.delete).toHaveBeenCalledWith('/config/templates/123')
    })

    it('throws error on deletion failure', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Template in use by devices' }
        }
      })

      await expect(deleteTemplate(123)).rejects.toThrow('Template in use by devices')
    })
  })

  describe('previewTemplate', () => {
    it('previews template rendering', async () => {
      const request = {
        templateContent: '{ "wifi": { "ssid": "{{ wifi_ssid }}" }}',
        variables: { wifi_ssid: 'MyNetwork' }
      }

      const preview: TemplatePreviewResult = {
        renderedConfig: { wifi: { ssid: 'MyNetwork' } }
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: preview,
          timestamp: new Date().toISOString()
        }
      })

      const result = await previewTemplate(request)
      expect(result.renderedConfig).toEqual({ wifi: { ssid: 'MyNetwork' } })
      expect(api.post).toHaveBeenCalledWith('/configuration/preview-template', request)
    })

    it('returns errors in preview', async () => {
      const request = {
        templateContent: '{ "wifi": {{ invalid }}',
        variables: {}
      }

      const preview: TemplatePreviewResult = {
        renderedConfig: {},
        errors: ['Invalid template syntax']
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: preview,
          timestamp: new Date().toISOString()
        }
      })

      const result = await previewTemplate(request)
      expect(result.errors).toContain('Invalid template syntax')
    })

    it('throws error on preview failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Template engine error' }
        }
      })

      await expect(previewTemplate({ templateContent: '' })).rejects.toThrow('Template engine error')
    })
  })

  describe('validateTemplate', () => {
    it('validates template syntax successfully', async () => {
      const request = {
        templateContent: '{ "valid": "{{ template }}" }',
        deviceType: 'shelly1'
      }

      const validation: TemplateValidationResult = {
        valid: true
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: validation,
          timestamp: new Date().toISOString()
        }
      })

      const result = await validateTemplate(request)
      expect(result.valid).toBe(true)
      expect(result.errors).toBeUndefined()
      expect(api.post).toHaveBeenCalledWith('/configuration/validate-template', request)
    })

    it('returns validation errors', async () => {
      const request = {
        templateContent: '{ invalid json',
        deviceType: 'shelly1'
      }

      const validation: TemplateValidationResult = {
        valid: false,
        errors: ['Invalid JSON syntax', 'Unclosed brace']
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: validation,
          timestamp: new Date().toISOString()
        }
      })

      const result = await validateTemplate(request)
      expect(result.valid).toBe(false)
      expect(result.errors).toHaveLength(2)
    })

    it('returns warnings', async () => {
      const request = {
        templateContent: '{ "wifi": { "ssid": "{{ ssid }}" }}',
        deviceType: 'shelly1'
      }

      const validation: TemplateValidationResult = {
        valid: true,
        warnings: ['Variable "ssid" not defined']
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: validation,
          timestamp: new Date().toISOString()
        }
      })

      const result = await validateTemplate(request)
      expect(result.valid).toBe(true)
      expect(result.warnings).toContain('Variable "ssid" not defined')
    })
  })

  describe('saveTemplate', () => {
    it('saves template using alternate endpoint', async () => {
      const data: Partial<ConfigTemplate> = {
        name: 'Saved Template',
        deviceType: 'shelly1',
        templateContent: '{{ content }}'
      }

      const savedTemplate: ConfigTemplate = {
        id: 456,
        name: 'Saved Template',
        deviceType: 'shelly1',
        templateContent: '{{ content }}',
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: savedTemplate,
          timestamp: new Date().toISOString()
        }
      })

      const result = await saveTemplate(data)
      expect(result).toEqual(savedTemplate)
      expect(api.post).toHaveBeenCalledWith('/configuration/templates', data)
    })

    it('throws error on save failure', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Validation failed' }
        }
      })

      await expect(saveTemplate({})).rejects.toThrow('Validation failed')
    })
  })

  describe('getTemplateExamples', () => {
    it('returns list of example templates', async () => {
      const examples: TemplateExample[] = [
        {
          name: 'Basic WiFi',
          description: 'Simple WiFi configuration',
          deviceType: 'shelly1',
          content: '{ "wifi": { "ssid": "{{ ssid }}" }}',
          variables: { ssid: 'string' }
        },
        {
          name: 'MQTT Basic',
          description: 'Basic MQTT setup',
          deviceType: 'shelly1pm',
          content: '{ "mqtt": { "server": "{{ server }}" }}',
          variables: { server: 'string' }
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { examples },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getTemplateExamples()
      expect(result).toHaveLength(2)
      expect(result[0].name).toBe('Basic WiFi')
      expect(result[1].deviceType).toBe('shelly1pm')
      expect(api.get).toHaveBeenCalledWith('/configuration/template-examples')
    })

    it('handles empty examples list', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { examples: [] },
          timestamp: new Date().toISOString()
        }
      })

      const result = await getTemplateExamples()
      expect(result).toEqual([])
    })

    it('handles missing examples field', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: {},
          timestamp: new Date().toISOString()
        }
      })

      const result = await getTemplateExamples()
      expect(result).toEqual([])
    })

    it('throws error on failure', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Examples not available' }
        }
      })

      await expect(getTemplateExamples()).rejects.toThrow('Examples not available')
    })
  })
})
