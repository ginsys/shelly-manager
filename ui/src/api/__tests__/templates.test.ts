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
  type ConfigTemplate
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
})
