import { vi, describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/import', () => ({
  listImportHistory: vi.fn(),
  getImportStatistics: vi.fn(),
  previewImport: vi.fn(),
  BROWSER_DATA_IMPORT_PLUGINS: ['sma'],
}))
vi.mock('@/api/export', () => ({
  listExportHistory: vi.fn(),
  getExportStatistics: vi.fn(),
}))

import { listImportHistory, getImportStatistics } from '@/api/import'
import { listExportHistory, getExportStatistics } from '@/api/export'
import ImportHistoryPage from '../ImportHistoryPage.vue'
import ExportHistoryPage from '../ExportHistoryPage.vue'

const stubs = {
  FilterBar: true,
  ImportPreviewForm: true,
  ExportPreviewForm: true,
  DataTable: true,
  PaginationBar: true,
  'router-link': true,
}
const stats = { total: 7, success: 5, failure: 2, by_plugin: {} }

beforeEach(() => {
  setActivePinia(createPinia())
  vi.mocked(listImportHistory).mockReset().mockResolvedValue({ items: [] })
  vi.mocked(listExportHistory).mockReset().mockResolvedValue({ items: [] })
  vi.mocked(getImportStatistics).mockReset()
  vi.mocked(getExportStatistics).mockReset()
})

describe('ImportHistoryPage statistics', () => {
  it('shows the import totals the way the export page does', async () => {
    vi.mocked(getImportStatistics).mockResolvedValue(stats)
    const w = mount(ImportHistoryPage, { global: { stubs } })
    await flushPromises()
    const text = w.find('[data-testid="history-stats"]').text()
    expect(text).toContain('Total: 7')
    expect(text).toContain('Success: 5')
    expect(text).toContain('Failure: 2')
  })

  it('shows the error in place of the numbers when the fetch fails', async () => {
    vi.mocked(getImportStatistics).mockRejectedValue(new Error('stats are down'))
    const w = mount(ImportHistoryPage, { global: { stubs } })
    await flushPromises()
    const box = w.find('[data-testid="history-stats"]')
    expect(box.text()).toContain('stats are down')
    expect(box.text()).not.toContain('Total: 0')
  })
})

describe('ExportHistoryPage statistics', () => {
  it('shows the totals', async () => {
    vi.mocked(getExportStatistics).mockResolvedValue(stats)
    const w = mount(ExportHistoryPage, { global: { stubs } })
    await flushPromises()
    expect(w.find('[data-testid="history-stats"]').text()).toContain('Total: 7')
  })

  it('shows the error in place of the numbers when the fetch fails', async () => {
    vi.mocked(getExportStatistics).mockRejectedValue(new Error('stats are down'))
    const w = mount(ExportHistoryPage, { global: { stubs } })
    await flushPromises()
    const box = w.find('[data-testid="history-stats"]')
    expect(box.text()).toContain('stats are down')
    expect(box.text()).not.toContain('Total: 0')
  })
})
