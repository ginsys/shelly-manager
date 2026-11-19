import { vi, describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'imp-1' } }),
}))
vi.mock('@/api/import', () => ({
  getImportResult: vi.fn(),
}))

import { getImportResult } from '@/api/import'
import ImportDetailPage from '../ImportDetailPage.vue'

const RouterLinkStub = {
  props: ['to'],
  render(this: any) {
    return h('a', { 'data-to': this.to }, this.$slots.default?.())
  },
}
const mountPage = () => mount(ImportDetailPage, { global: { stubs: { 'router-link': RouterLinkStub } } })

const result = {
  success: true,
  import_id: 'imp-1',
  plugin_name: 'sma',
  format: 'json',
  records_imported: 5,
  records_skipped: 2,
  duration: 42_000_000,
  created_at: '2026-11-19T10:15:00Z',
}

// The shared test setup answers document.createElement('a') with a plain object, which Vue cannot
// render: use real elements here.
const realCreateElement = Document.prototype.createElement.bind(document)

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
  vi.mocked(getImportResult).mockReset().mockResolvedValue(result)
})

describe('ImportDetailPage', () => {
  it('shows what the server returned', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-status"]').text()).toBe('Status: Succeeded')
    expect(w.find('[data-testid="result-duration"]').text()).toContain('42 ms')
    expect(w.find('[data-testid="result-created"]').text()).toContain(new Date(result.created_at).toLocaleString())
    expect(w.find('[data-testid="result-imported"]').text()).toContain('5')
    expect(w.find('[data-testid="result-skipped"]').text()).toContain('2')
    expect(w.text()).toContain('imp-1')
  })

  it('says when an import failed and lists its errors and warnings', async () => {
    vi.mocked(getImportResult).mockResolvedValue({
      ...result,
      success: false,
      errors: ['row 3: bad id'],
      warnings: ['unknown field ignored'],
    })
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-status"]').text()).toContain('Failed')
    expect(w.find('[data-testid="result-errors"]').text()).toContain('row 3: bad id')
    expect(w.find('[data-testid="result-warnings"]').text()).toContain('unknown field ignored')
  })

  it('leaves out the lists and the changes section when there is nothing to show', async () => {
    const w = mountPage()
    await flushPromises()
    for (const t of ['result-errors', 'result-warnings', 'result-changes']) {
      expect(w.find(`[data-testid="${t}"]`).exists()).toBe(false)
    }
  })

  it('counts the changes by type and lists each one without its values', async () => {
    vi.mocked(getImportResult).mockResolvedValue({
      ...result,
      changes: [
        { type: 'create', resource: 'device', resource_id: 'd1', new_value: { password: 'hunter2-new' } },
        { type: 'create', resource: 'device', resource_id: 'd2', new_value: { password: 'x' } },
        { type: 'update', resource: 'config', resource_id: 'c1', field: 'mqtt.pass', old_value: 'OLD-SECRET', new_value: 'NEW-SECRET' },
        { type: 'delete', resource: 'template', resource_id: 't1', new_value: null },
      ],
    })
    const w = mountPage()
    await flushPromises()
    const box = w.find('[data-testid="result-changes"]')
    expect(box.find('[data-testid="count-create"]').text()).toContain('2')
    expect(box.find('[data-testid="count-update"]').text()).toContain('1')
    expect(box.find('[data-testid="count-delete"]').text()).toContain('1')
    const rows = box.findAll('tbody tr')
    expect(rows).toHaveLength(4)
    expect(rows[2].text()).toContain('config')
    expect(rows[2].text()).toContain('c1')
    expect(rows[2].text()).toContain('mqtt.pass')
    // old_value and new_value can hold secrets and any amount of data: never on the page
    for (const secret of ['hunter2-new', 'OLD-SECRET', 'NEW-SECRET']) {
      expect(w.html()).not.toContain(secret)
    }
  })

  it('shows a dash for a value the server did not send', async () => {
    vi.mocked(getImportResult).mockResolvedValue({
      success: true, import_id: 'imp-1', plugin_name: 'sma', format: 'json',
    })
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="result-duration"]').text()).toContain('—')
    expect(w.find('[data-testid="result-created"]').text()).toContain('—')
    expect(w.find('[data-testid="result-imported"]').text()).toContain('—')
  })

  it('links back to the history and offers no download', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="back-link"]').attributes('data-to')).toBe('/import/history')
    // the API has no import download route
    expect(w.text().toLowerCase()).not.toContain('download')
  })

  it('shows the message when the result cannot be loaded', async () => {
    vi.mocked(getImportResult).mockRejectedValue(new Error('Import result not found'))
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('Import result not found')
  })
})
