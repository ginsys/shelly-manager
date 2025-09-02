import { describe, it, expect, vi, beforeEach } from 'vitest'
import { listExportHistory, getExportStatistics } from './export'
import api from './client'

vi.mock('./client', () => {
  const axiosLike = {
    get: vi.fn(),
    interceptors: { request: { use: vi.fn() } },
  }
  return { default: axiosLike, __esModule: true }
})

const mockedApi = api as unknown as { get: any }

describe('export api', () => {
  beforeEach(() => { vi.resetAllMocks() })
  it('lists export history', async () => {
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { history: [{ export_id: 'exp-1', plugin_name:'mockfile', format:'txt', success:true, created_at: new Date().toISOString() }] }, meta: { pagination: { page:1,page_size:20,total_pages:1,has_next:false,has_previous:false } }, timestamp: new Date().toISOString() } })
    const res = await listExportHistory({ page:1, pageSize: 20 })
    expect(res.items.length).toBe(1)
    expect(res.meta?.pagination?.page).toBe(1)
  })

  it('gets export statistics', async () => {
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { total:3, success:2, failure:1, by_plugin: { mockfile:3 } }, timestamp: new Date().toISOString() } })
    const stats = await getExportStatistics()
    expect(stats.total).toBe(3)
    expect(stats.by_plugin.mockfile).toBe(3)
  })
  it('gets export result', async () => {
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { export_id:'exp-1', plugin_name:'mockfile', format:'txt' }, timestamp: new Date().toISOString() } })
    const { getExportResult } = await import('./export')
    const res = await getExportResult('exp-1')
    expect(res.export_id).toBe('exp-1')
  })
  it('previews export', async () => {
    mockedApi.get.mockReset()
    mockedApi.get.mockResolvedValueOnce({}) // avoid matching previous mock
    ;(mockedApi.get as any) = vi.fn() // isolate
    ;(api as any).post = vi.fn().mockResolvedValue({ data: { success:true, data: { preview: { success:true, record_count: 5 }, summary: { record_count:5 } }, timestamp: new Date().toISOString() } })
    const { previewExport } = await import('./export')
    const data = await previewExport({ plugin_name:'mockfile', format:'txt' })
    expect(data.preview.record_count).toBe(5)
  })
})
