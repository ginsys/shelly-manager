import { vi, describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useImportStore } from '../import'
import { useExportStore } from '../export'
import * as importApi from '@/api/import'
import * as exportApi from '@/api/export'

vi.mock('@/api/import', () => ({
  listImportHistory: vi.fn(),
  getImportStatistics: vi.fn(),
}))
vi.mock('@/api/export', () => ({
  listExportHistory: vi.fn(),
  getExportStatistics: vi.fn(),
}))

const stats = { total: 5, success: 3, failure: 2, by_plugin: { sma: 5 } }

// A statistics fetch that fails must say so: the page used to show "Total: 0" for a failed fetch,
// which reads as "nothing was ever imported or exported".
describe.each([
  ['import', useImportStore, importApi.getImportStatistics],
  ['export', useExportStore, exportApi.getExportStatistics],
] as const)('%s store statistics', (_name, useStore, fetcher) => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(fetcher as any).mockReset()
  })

  it('keeps the numbers and no error after a good fetch', async () => {
    vi.mocked(fetcher as any).mockResolvedValue(stats)
    const store = useStore()
    await store.fetchStats()
    expect(store.stats).toEqual(stats)
    expect(store.statsError).toBe('')
  })

  it('reports the failure and leaves the numbers alone', async () => {
    vi.mocked(fetcher as any).mockRejectedValue(new Error('boom'))
    const store = useStore()
    await store.fetchStats()
    expect(store.statsError).toBe('boom')
    expect(store.stats.total).toBe(0)
  })

  it('names a failure that carries no message', async () => {
    vi.mocked(fetcher as any).mockRejectedValue({})
    const store = useStore()
    await store.fetchStats()
    expect(store.statsError).toBe('Failed to load statistics')
  })

  it('clears the error when the next fetch works', async () => {
    const mock = vi.mocked(fetcher as any)
    mock.mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(stats)
    const store = useStore()
    await store.fetchStats()
    expect(store.statsError).toBe('boom')
    await store.fetchStats()
    expect(store.statsError).toBe('')
    expect(store.stats).toEqual(stats)
  })
})
