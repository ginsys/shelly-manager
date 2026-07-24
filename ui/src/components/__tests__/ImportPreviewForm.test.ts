import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  listPlugins: vi.fn(),
  previewImport: vi.fn(),
}))

vi.mock('@/api/plugin', () => ({ listPlugins: mocks.listPlugins }))
vi.mock('@/api/import', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/import')>(),
  previewImport: mocks.previewImport,
}))

import ImportPreviewForm from '../ImportPreviewForm.vue'

describe('ImportPreviewForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listPlugins.mockResolvedValue({
      plugins: [
        {
          name: 'sma',
          display_name: 'SMA',
          description: '',
          version: '2026.1',
          category: 'backup',
          capabilities: ['sma'],
          status: { available: true, configured: true, enabled: true },
        },
        {
          name: 'json',
          display_name: 'JSON',
          description: '',
          version: '1',
          category: 'custom',
          capabilities: ['json'],
          status: { available: true, configured: true, enabled: true },
        },
      ],
      categories: [],
    })
    mocks.previewImport.mockResolvedValue({
      preview: {
        success: true,
        import_id: 'id',
        plugin_name: 'sma',
        format: 'sma',
        records_imported: 0,
        records_skipped: 0,
        changes: [],
        warnings: [],
      },
      changes_count: 0,
      summary: { will_create: 0, will_update: 0, will_delete: 0 },
    })
  })

  it('offers only registered SMA data import and sends the exact payload', async () => {
    const wrapper = mount(ImportPreviewForm)
    await flushPromises()
    const options = wrapper.findAll('#import-plugin option').map(option => option.text())
    expect(options).toEqual(['Select a plugin…', 'SMA'])

    await wrapper.get('#import-plugin').setValue('sma')
    await wrapper.get('#import-text').setValue('{}')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.previewImport).toHaveBeenCalledWith({
      plugin_name: 'sma',
      format: 'sma',
      source: { type: 'data', data: 'e30=' },
      config: {},
      options: { dry_run: true, validate_only: true },
    })
    const result = wrapper.get('[data-testid="preview-section"]').text()
    expect(result).toContain('Will create0')
    expect(result).toContain('Imported0')
    expect(result).toContain('Skipped0')
  })

  it('shows an explicit state when no compatible importer is registered', async () => {
    mocks.listPlugins.mockResolvedValueOnce({
      plugins: [{
        name: 'json',
        display_name: 'JSON',
        description: '',
        version: '1',
        category: 'custom',
        capabilities: ['json'],
        status: { available: true, configured: true, enabled: true },
      }],
      categories: [],
    })
    const wrapper = mount(ImportPreviewForm)
    await flushPromises()
    expect(wrapper.get('[data-testid="no-compatible-importer"]').text()).toContain('No compatible')
    expect(wrapper.get('[data-testid="preview-button"]').attributes('disabled')).toBeDefined()
  })

  it('renders changes_count and plugin-returned errors including zero summaries', async () => {
    mocks.previewImport.mockResolvedValueOnce({
      preview: {
        success: false,
        import_id: 'id',
        plugin_name: 'sma',
        format: 'sma',
        records_imported: 0,
        records_skipped: 0,
        changes: [],
        warnings: [],
        errors: ['device conflict'],
      },
      changes_count: 2,
      summary: { will_create: 0, will_update: 0, will_delete: 0 },
    })
    const wrapper = mount(ImportPreviewForm)
    await flushPromises()
    await wrapper.get('#import-plugin').setValue('sma')
    await wrapper.get('#import-text').setValue('{}')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const result = wrapper.get('[data-testid="preview-section"]').text()
    expect(result).toContain('Changes2')
    expect(result).toContain('device conflict')
    expect(result).toContain('Will create0')
  })

  it('keeps registry and preview failures visible', async () => {
    mocks.previewImport.mockRejectedValueOnce(new Error('preview unavailable'))
    const wrapper = mount(ImportPreviewForm)
    await flushPromises()
    await wrapper.get('#import-plugin').setValue('sma')
    await wrapper.get('#import-text').setValue('{}')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('preview unavailable')
  })

  it('rejects oversized files before reading their bytes', async () => {
    const wrapper = mount(ImportPreviewForm)
    await flushPromises()
    await wrapper.get('#import-plugin').setValue('sma')
    const arrayBuffer = vi.fn()
    const oversized = {
      name: 'oversized.sma',
      size: 7 * 1024 * 1024 + 1,
      arrayBuffer,
    } as unknown as File
    const input = wrapper.get('#import-file')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [oversized],
    })
    await input.trigger('change')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(arrayBuffer).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('7 MiB')
    expect(mocks.previewImport).not.toHaveBeenCalled()
  })
})
