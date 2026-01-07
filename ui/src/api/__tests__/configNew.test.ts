import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      patch: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import api from '../client'
import {
  getDeviceTemplatesNew,
  setDeviceTemplatesNew,
  addDeviceTemplateNew,
  removeDeviceTemplateNew,
  getDeviceOverridesNew,
  setDeviceOverridesNew,
  patchDeviceOverridesNew,
  deleteDeviceOverridesNew,
  getDesiredConfigNew,
  getConfigStatusNew,
  applyDeviceConfigNew,
  verifyDeviceConfigNew,
} from '../configNew'

describe('configNew API', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
    ;(api.patch as any).mockReset()
    ;(api.delete as any).mockReset()
  })

  it('getDeviceTemplatesNew calls /devices/:id/templates/new', async () => {
    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          templates: [{ id: 1, name: 'Test', scope: 'global', config: {} }],
          template_ids: [1],
        },
      },
    })

    const result = await getDeviceTemplatesNew(123)
    expect(result.template_ids).toEqual([1])
    expect(api.get).toHaveBeenCalledWith('/devices/123/templates/new')
  })

  it('setDeviceTemplatesNew puts template IDs', async () => {
    ;(api.put as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          templates: [],
          template_ids: [1, 2],
        },
      },
    })

    const result = await setDeviceTemplatesNew(123, [1, 2])
    expect(result.template_ids).toEqual([1, 2])
    expect(api.put).toHaveBeenCalledWith('/devices/123/templates/new', { template_ids: [1, 2] })
  })

  it('addDeviceTemplateNew posts to add template', async () => {
    ;(api.post as any).mockResolvedValue({
      data: {
        success: true,
        data: { templates: [] },
      },
    })

    await addDeviceTemplateNew({ deviceId: 123, templateId: 5 })
    expect(api.post).toHaveBeenCalledWith('/devices/123/templates/new/5', null, { params: { position: undefined } })
  })

  it('removeDeviceTemplateNew deletes template from device', async () => {
    ;(api.delete as any).mockResolvedValue({
      data: {
        success: true,
        data: { templates: [] },
      },
    })

    await removeDeviceTemplateNew(123, 5)
    expect(api.delete).toHaveBeenCalledWith('/devices/123/templates/new/5')
  })

  it('getDeviceOverridesNew gets overrides', async () => {
    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: { overrides: { mqtt: { enable: true } } },
      },
    })

    const result = await getDeviceOverridesNew(123)
    expect(result.overrides).toEqual({ mqtt: { enable: true } })
    expect(api.get).toHaveBeenCalledWith('/devices/123/overrides/new')
  })

  it('setDeviceOverridesNew puts overrides', async () => {
    ;(api.put as any).mockResolvedValue({
      data: {
        success: true,
        data: { overrides: { mqtt: { enable: false } } },
      },
    })

    const result = await setDeviceOverridesNew(123, { mqtt: { enable: false } })
    expect(result.overrides).toEqual({ mqtt: { enable: false } })
    expect(api.put).toHaveBeenCalledWith('/devices/123/overrides/new', { mqtt: { enable: false } })
  })

  it('patchDeviceOverridesNew patches overrides', async () => {
    ;(api.patch as any).mockResolvedValue({
      data: {
        success: true,
        data: { overrides: { mqtt: { server: 'mqtt.local' } } },
      },
    })

    const result = await patchDeviceOverridesNew(123, { mqtt: { server: 'mqtt.local' } })
    expect(result).toEqual({ mqtt: { server: 'mqtt.local' } })
    expect(api.patch).toHaveBeenCalledWith('/devices/123/overrides/new', { mqtt: { server: 'mqtt.local' } })
  })

  it('deleteDeviceOverridesNew expects 204', async () => {
    ;(api.delete as any).mockResolvedValue({ status: 204, data: '' })

    await deleteDeviceOverridesNew(123)
    expect(api.delete).toHaveBeenCalledWith('/devices/123/overrides/new')
  })

  it('getDesiredConfigNew returns config and sources', async () => {
    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          config: { mqtt: { enable: true } },
          sources: { 'mqtt.enable': 'Global MQTT' },
        },
      },
    })

    const result = await getDesiredConfigNew(123)
    expect(result.config).toEqual({ mqtt: { enable: true } })
    expect(result.sources).toEqual({ 'mqtt.enable': 'Global MQTT' })
    expect(api.get).toHaveBeenCalledWith('/devices/123/desired-config')
  })

  it('getConfigStatusNew returns status', async () => {
    ;(api.get as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          device_id: 123,
          config_applied: true,
          has_overrides: false,
          template_count: 2,
          pending_changes: false,
        },
      },
    })

    const result = await getConfigStatusNew(123)
    expect(result.config_applied).toBe(true)
    expect(result.template_count).toBe(2)
    expect(api.get).toHaveBeenCalledWith('/devices/123/config/new/status')
  })

  it('applyDeviceConfigNew posts to apply', async () => {
    ;(api.post as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          success: true,
          applied_count: 3,
          failed_count: 0,
          requires_reboot: false,
        },
      },
    })

    const result = await applyDeviceConfigNew(123)
    expect(result.success).toBe(true)
    expect(result.applied_count).toBe(3)
    expect(api.post).toHaveBeenCalledWith('/devices/123/config/new/apply', null)
  })

  it('verifyDeviceConfigNew posts to verify', async () => {
    ;(api.post as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          match: true,
        },
      },
    })

    const result = await verifyDeviceConfigNew(123)
    expect(result.match).toBe(true)
    expect(api.post).toHaveBeenCalledWith('/devices/123/config/new/verify', null)
  })
})
