import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  listPlugins: vi.fn(),
  getPluginSchema: vi.fn(),
  previewExport: vi.fn(),
}))

vi.mock('@/api/plugin', () => ({
  listPlugins: mocks.listPlugins,
  getPluginSchema: mocks.getPluginSchema,
}))
vi.mock('@/api/export', () => ({
  previewExport: mocks.previewExport,
}))

import ExportPreviewForm from '../ExportPreviewForm.vue'

describe('ExportPreviewForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listPlugins.mockResolvedValue({
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
    mocks.getPluginSchema.mockResolvedValue({
      version: '1',
      properties: {},
      required: null,
    })
    mocks.previewExport.mockResolvedValue({
      preview: { success: true, sample_data: '', record_count: 0, estimated_size: 0 },
      summary: { record_count: 0, estimated_size: 0 },
    })
  })

  it('loads the registry and sends the exact generic preview payload', async () => {
    const wrapper = mount(ExportPreviewForm)
    await flushPromises()
    await wrapper.get('[data-testid="plugin-select"]').setValue('json')
    await flushPromises()
    await wrapper.get('#export-format').setValue('json')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.previewExport).toHaveBeenCalledWith({
      plugin_name: 'json',
      format: 'json',
      config: {},
      filters: {},
      output: { type: 'response' },
      options: {
        dry_run: true,
        include_history: false,
        validate_only: true,
        compact_output: false,
        include_metadata: true,
      },
    })
    expect(wrapper.get('[data-testid="preview-section"]').text()).toContain('0 B')
    expect(wrapper.get('[data-testid="preview-section"]').text()).toContain('Records0')
  })

  it('keeps registry failures visible and disables preview', async () => {
    mocks.listPlugins.mockRejectedValueOnce(new Error('registry unavailable'))
    const wrapper = mount(ExportPreviewForm)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('registry unavailable')
    expect(wrapper.get('[data-testid="preview-button"]').attributes('disabled')).toBeDefined()
  })

  it('keeps schema failures visible and disables preview', async () => {
    mocks.getPluginSchema.mockRejectedValueOnce(new Error('schema unavailable'))
    const wrapper = mount(ExportPreviewForm)
    await flushPromises()
    await wrapper.get('[data-testid="plugin-select"]').setValue('json')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('schema unavailable')
    expect(wrapper.get('[data-testid="preview-button"]').attributes('disabled')).toBeDefined()
  })

  it('shows preview request failures and clears stale results', async () => {
    mocks.previewExport.mockRejectedValueOnce(new Error('preview unavailable'))
    const wrapper = mount(ExportPreviewForm)
    await flushPromises()
    await wrapper.get('[data-testid="plugin-select"]').setValue('json')
    await flushPromises()
    await wrapper.get('#export-format').setValue('json')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('preview unavailable')
    expect(wrapper.find('[data-testid="preview-section"]').exists()).toBe(false)
  })

  it('ignores a registry response that resolves after unmount', async () => {
    let resolveList!: (value: Awaited<ReturnType<typeof mocks.listPlugins>>) => void
    mocks.listPlugins.mockImplementationOnce(() => new Promise(resolve => {
      resolveList = resolve
    }))
    const wrapper = mount(ExportPreviewForm)
    const vm = wrapper.vm as unknown as { plugins: unknown[] }
    wrapper.unmount()
    resolveList({
      plugins: [{
        name: 'late',
        display_name: 'Late plugin',
        description: '',
        version: '1',
        category: 'custom',
        capabilities: ['json'],
        status: { available: true, configured: true, enabled: true },
      }],
      categories: [],
    })
    await flushPromises()
    expect(vm.plugins).toEqual([])
  })
})
