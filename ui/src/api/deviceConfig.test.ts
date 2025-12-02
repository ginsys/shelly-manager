import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('./client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import api from './client'
import {
  getStoredConfig,
  updateStoredConfig,
  getLiveConfig,
  getLiveConfigNormalized,
  getTypedNormalizedConfig,
  importConfig,
  getImportStatus,
  exportConfig,
  detectDrift,
  applyTemplate,
  getConfigHistory,
} from './deviceConfig'

describe('deviceConfig api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('gets stored config', async () => {
    ;(api.get as any).mockResolvedValue({ data: { success: true, data: { foo: 'bar' }, timestamp: '' } })
    const r = await getStoredConfig(1)
    expect(r).toEqual({ foo: 'bar' })
    expect(api.get).toHaveBeenCalledWith('/devices/1/config')
  })

  it('updates stored config', async () => {
    ;(api.put as any).mockResolvedValue({ data: { success: true, data: { a: 1 }, timestamp: '' } })
    const r = await updateStoredConfig(2, { a: 1 })
    expect(r).toEqual({ a: 1 })
    expect(api.put).toHaveBeenCalledWith('/devices/2/config', { a: 1 })
  })

  it('gets live and normalized configs', async () => {
    ;(api.get as any).mockResolvedValueOnce({ data: { success: true, data: { live: true }, timestamp: '' } })
    const live = await getLiveConfig(3)
    expect(live).toEqual({ live: true })
    expect(api.get).toHaveBeenCalledWith('/devices/3/config/current')

    ;(api.get as any).mockResolvedValueOnce({ data: { success: true, data: { norm: true }, timestamp: '' } })
    const norm = await getLiveConfigNormalized(3)
    expect(norm).toEqual({ norm: true })
    expect(api.get).toHaveBeenCalledWith('/devices/3/config/current/normalized')

    ;(api.get as any).mockResolvedValueOnce({ data: { success: true, data: { typed: true }, timestamp: '' } })
    const typed = await getTypedNormalizedConfig(3)
    expect(typed).toEqual({ typed: true })
    expect(api.get).toHaveBeenCalledWith('/devices/3/config/typed/normalized')
  })

  it('imports config and checks status', async () => {
    ;(api.post as any).mockResolvedValueOnce({ data: { success: true, data: { accepted: true }, timestamp: '' } })
    const r = await importConfig(4, { foo: 'bar' })
    expect(r.accepted).toBe(true)
    expect(api.post).toHaveBeenCalledWith('/devices/4/config/import', { foo: 'bar' })

    ;(api.get as any).mockResolvedValueOnce({ data: { success: true, data: { status: 'running' }, timestamp: '' } })
    const s = await getImportStatus(4)
    expect(s.status).toBe('running')
    expect(api.get).toHaveBeenCalledWith('/devices/4/config/status')
  })

  it('exports config and detects drift', async () => {
    ;(api.post as any).mockResolvedValueOnce({ data: { success: true, data: { export_id: 'e1' }, timestamp: '' } })
    const e = await exportConfig(5)
    expect(e.export_id).toBe('e1')
    expect(api.post).toHaveBeenCalledWith('/devices/5/config/export', {})

    ;(api.get as any).mockResolvedValueOnce({ data: { success: true, data: { has_drift: false }, timestamp: '' } })
    const d = await detectDrift(5)
    expect(d.has_drift).toBe(false)
    expect(api.get).toHaveBeenCalledWith('/devices/5/config/drift')
  })

  it('applies template and gets history', async () => {
    ;(api.post as any).mockResolvedValueOnce({ data: { success: true, data: { applied: true }, timestamp: '' } })
    const a = await applyTemplate(6, 10, { foo: 'x' })
    expect(a.applied).toBe(true)
    expect(api.post).toHaveBeenCalledWith('/devices/6/config/apply-template', { template_id: 10, variables: { foo: 'x' } })

    ;(api.get as any).mockResolvedValueOnce({ data: { success: true, data: { history: [{ id: 'h1', timestamp: '' }] }, timestamp: '' } })
    const h = await getConfigHistory(6)
    expect(h.items.length).toBe(1)
    expect(api.get).toHaveBeenCalledWith('/devices/6/config/history', { params: { page: 1, page_size: 20 } })
  })

  it('handles API error gracefully', async () => {
    ;(api.get as any).mockResolvedValue({ data: { success: false, error: { message: 'x' }, timestamp: '' } })
    await expect(getStoredConfig(7)).rejects.toThrow('x')
  })
})

