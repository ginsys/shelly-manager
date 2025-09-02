import { describe, it, expect, vi, beforeEach } from 'vitest'
import { listImportHistory, getImportStatistics } from './import'
import api from './client'

vi.mock('./client', () => {
  const axiosLike = { get: vi.fn(), interceptors: { request: { use: vi.fn() } } }
  return { default: axiosLike, __esModule: true }
})
const mockedApi = api as unknown as { get: any }

describe('import api', () => {
  beforeEach(() => vi.resetAllMocks())
  it('lists import history', async () => {
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { history: [{ import_id: 'imp-1', plugin_name:'mockfile', format:'txt', success:true, created_at: new Date().toISOString() }] }, meta: { pagination: { page:1,page_size:20,total_pages:1,has_next:false,has_previous:false } }, timestamp: new Date().toISOString() } })
    const res = await listImportHistory({})
    expect(res.items[0].import_id).toBe('imp-1')
  })
  it('gets import statistics', async () => {
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { total:3, success:1, failure:2, by_plugin: { other:3 } }, timestamp: new Date().toISOString() } })
    const stats = await getImportStatistics()
    expect(stats.failure).toBe(2)
  })
  it('gets import result', async () => {
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { import_id:'imp-1', plugin_name:'mockfile', format:'txt', success:true }, timestamp: new Date().toISOString() } })
    const { getImportResult } = await import('./import')
    const res = await getImportResult('imp-1')
    expect(res.import_id).toBe('imp-1')
  })
  it('previews import', async () => {
    ;(api as any).post = vi.fn().mockResolvedValue({ data: { success:true, data: { preview: { success:true }, summary: { will_create:1 } }, timestamp: new Date().toISOString() } })
    const { previewImport } = await import('./import')
    const res = await previewImport({ plugin_name:'mockfile', format:'txt' })
    expect(res.summary.will_create).toBe(1)
  })
})
