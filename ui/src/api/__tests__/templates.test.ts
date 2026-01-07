import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import api from '../client'
import {
  listTemplates,
  getTemplate,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  type ConfigTemplate,
} from '../templates'

describe('templates API (new config templates)', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
    ;(api.delete as any).mockReset()
  })

  it('listTemplates calls /config/templates/new', async () => {
    const templates: ConfigTemplate[] = [
      {
        id: 1,
        name: 'Global MQTT',
        scope: 'global',
        config: { mqtt: { enable: true } },
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]

    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: { templates },
        timestamp: new Date().toISOString(),
      },
    })

    const result = await listTemplates()
    expect(result).toEqual(templates)
    expect(api.get).toHaveBeenCalledWith('/config/templates/new', { params: { scope: undefined } })
  })

  it('getTemplate calls /config/templates/new/:id', async () => {
    const template: ConfigTemplate = {
      id: 2,
      name: 'Plug Defaults',
      scope: 'device_type',
      device_type: 'SHPLG-S',
      config: { system: { name: 'Plug' } },
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-02T00:00:00Z',
    }

    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: { template },
        timestamp: new Date().toISOString(),
      },
    })

    const result = await getTemplate(2)
    expect(result).toEqual(template)
    expect(api.get).toHaveBeenCalledWith('/config/templates/new/2')
  })

  it('createTemplate posts to /config/templates/new', async () => {
    const created: ConfigTemplate = {
      id: 3,
      name: 'Office Settings',
      scope: 'group',
      config: { mqtt: { server: 'mqtt.local' } },
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }

    ;(api.post as any).mockResolvedValue({
      data: {
        success: true,
        data: { template: created },
        timestamp: new Date().toISOString(),
      },
    })

    const result = await createTemplate({
      name: created.name,
      scope: created.scope,
      config: created.config,
    })

    expect(result).toEqual(created)
    expect(api.post).toHaveBeenCalledWith('/config/templates/new', {
      name: created.name,
      scope: created.scope,
      config: created.config,
    })
  })

  it('updateTemplate calls /config/templates/new/:id', async () => {
    const updated: ConfigTemplate = {
      id: 5,
      name: 'Updated',
      scope: 'global',
      config: { cloud: { enable: false } },
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-02T00:00:00Z',
    }

    ;(api.put as any).mockResolvedValue({
      data: {
        success: true,
        data: { template: updated, affected_devices: 7 },
        timestamp: new Date().toISOString(),
      },
    })

    const result = await updateTemplate(5, { name: 'Updated', config: updated.config })
    expect(result.template).toEqual(updated)
    expect(result.affected_devices).toBe(7)
    expect(api.put).toHaveBeenCalledWith('/config/templates/new/5', { name: 'Updated', config: updated.config })
  })

  it('deleteTemplate treats 204 as success', async () => {
    ;(api.delete as any).mockResolvedValue({ status: 204, data: '' })

    await deleteTemplate(123)
    expect(api.delete).toHaveBeenCalledWith('/config/templates/new/123')
  })
})
