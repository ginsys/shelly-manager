import { vi, describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'exp-1' } }),
}))
vi.mock('@/api/export', () => ({
  getExportResult: vi.fn(),
  downloadExportWithName: vi.fn(),
}))

import { getExportResult, downloadExportWithName } from '@/api/export'
import ExportDetailPage from '../ExportDetailPage.vue'

const RouterLinkStub = {
  props: ['to'],
  render(this: any) {
    return h('a', { 'data-to': this.to }, this.$slots.default?.())
  },
}
const mountPage = () => mount(ExportDetailPage, { global: { stubs: { 'router-link': RouterLinkStub } } })

const result = {
  success: true,
  export_id: 'exp-1',
  plugin_name: 'sma',
  format: 'json',
  record_count: 12,
  file_size: 2048,
  checksum: 'abc123',
  duration: 1_500_000_000,
  created_at: '2026-11-19T10:15:00Z',
}

// The shared test setup (src/__tests__/setup.ts) answers document.createElement('a') with a plain object,
// which Vue cannot render. Use real elements and watch the click of the download anchor instead.
const realCreateElement = Document.prototype.createElement.bind(document)
let clicked: HTMLAnchorElement[] = []

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
  clicked = []
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    clicked.push(this)
  })
  vi.mocked(getExportResult).mockReset().mockResolvedValue(result)
  vi.mocked(downloadExportWithName).mockReset()
})

describe('ExportDetailPage', () => {
  it('shows what the server returned', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-status"]').text()).toContain('Succeeded')
    expect(w.find('[data-testid="result-duration"]').text()).toContain('1.5 s')
    expect(w.find('[data-testid="result-created"]').text()).toContain(new Date(result.created_at).toLocaleString())
    const text = w.text()
    expect(text).toContain('exp-1')
    expect(text).toContain('12')
    expect(text).toContain('2048 bytes')
    expect(text).toContain('abc123')
  })

  it('says when an export failed and lists its errors and warnings', async () => {
    vi.mocked(getExportResult).mockResolvedValue({
      ...result,
      success: false,
      errors: ['disk full'],
      warnings: ['one device skipped'],
    })
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-status"]').text()).toContain('Failed')
    expect(w.find('[data-testid="result-errors"]').text()).toContain('disk full')
    expect(w.find('[data-testid="result-warnings"]').text()).toContain('one device skipped')
  })

  it('leaves out the error and warning lists when there are none', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-errors"]').exists()).toBe(false)
    expect(w.find('[data-testid="result-warnings"]').exists()).toBe(false)
  })

  it('shows a dash for a value the server did not send', async () => {
    vi.mocked(getExportResult).mockResolvedValue({
      success: true, export_id: 'exp-1', plugin_name: 'sma', format: 'json',
    })
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-duration"]').text()).toContain('—')
    expect(w.find('[data-testid="result-created"]').text()).toContain('—')
  })

  it('links back to the history', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="back-link"]').attributes('data-to')).toBe('/export/history')
  })

  it('has no plain link to the download route, which would send no admin key', async () => {
    const w = mountPage()
    await flushPromises()
    for (const a of w.findAll('a')) {
      expect(a.attributes('href') ?? '').not.toContain('/download')
    }
  })

  it('downloads through the authed client and saves under the server file name', async () => {
    const blob = new Blob(['x'])
    vi.mocked(downloadExportWithName).mockResolvedValue({ blob, filename: 'export-2026.json' })
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="download-btn"]').trigger('click')
    await flushPromises()
    expect(downloadExportWithName).toHaveBeenCalledTimes(1)
    expect(downloadExportWithName).toHaveBeenCalledWith('exp-1')
    expect(URL.createObjectURL).toHaveBeenCalledWith(blob)
    expect(clicked).toHaveLength(1)
    expect(clicked[0].download).toBe('export-2026.json')
    expect(URL.revokeObjectURL).toHaveBeenCalled()
  })

  it('falls back to a fixed file name when the server sends none', async () => {
    vi.mocked(downloadExportWithName).mockResolvedValue({ blob: new Blob(['x']), filename: undefined })
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="download-btn"]').trigger('click')
    await flushPromises()
    expect(clicked).toHaveLength(1)
    expect(clicked[0].download).toBe('export-exp-1')
  })

  it('shows the message when the download fails', async () => {
    vi.mocked(downloadExportWithName).mockRejectedValue(new Error('Request failed with status code 401'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="download-btn"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="download-error"]').text()).toContain('Request failed with status code 401')
  })

  it('shows the message when the result cannot be loaded', async () => {
    vi.mocked(getExportResult).mockRejectedValue(new Error('Export result not found'))
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('Export result not found')
    expect(w.find('[data-testid="download-btn"]').exists()).toBe(false)
  })
})
