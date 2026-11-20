import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

vi.mock('@/api/client', () => ({
  default: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

import api from '@/api/client'
import GitOpsExportPage from '../GitOpsExportPage.vue'

function record(id: string, name: string) {
  return {
    id: 1,
    export_id: id,
    plugin_name: 'gitops',
    format: 'yaml',
    name,
    file_path: '/srv/gitops',
    success: true,
    record_count: 4,
    file_size: 2048,
    created_at: '2026-11-20T10:00:00Z',
  }
}

function historyAnswer(rows: unknown[], totalPages = 1) {
  return { data: { success: true, data: { history: rows }, meta: { pagination: { page: 1, page_size: 20, total_pages: totalPages, has_next: false, has_previous: false } } } }
}

describe('GitOpsExportPage', () => {
  beforeEach(() => {
    vi.mocked(api.get as any).mockReset()
    vi.mocked(api.delete as any).mockReset()
  })

  it('lists the gitops history and shows what the record has', async () => {
    vi.mocked(api.get as any).mockResolvedValue(historyAnswer([record('a', 'nightly')]))

    const wrapper = mount(GitOpsExportPage)
    await flushPromises()

    const listCall = vi.mocked(api.get as any).mock.calls.find((c) => c[1]?.params?.page_size === 20)
    expect(listCall?.[0]).toBe('/export/history')
    expect(listCall?.[1].params.plugin).toBe('gitops')
    expect(wrapper.text()).toContain('nightly')
    expect(wrapper.text()).toContain('/srv/gitops')
    expect(wrapper.text()).toContain('2 KB')
  })

  it('says the files stay when a record is deleted, and deletes the record only', async () => {
    vi.mocked(api.get as any).mockResolvedValue(historyAnswer([record('a', 'nightly')]))
    vi.mocked(api.delete as any).mockResolvedValue({ status: 204 })

    const wrapper = mount(GitOpsExportPage)
    await flushPromises()
    await wrapper.findAll('.action-btn')[1].trigger('click')
    expect(wrapper.text()).toContain('The exported files stay on the server')

    await wrapper.get('.danger-button').trigger('click')
    await flushPromises()

    expect(api.delete).toHaveBeenCalledWith('/export/gitops/a')
  })

  it('loads every page of devices for the selection', async () => {
    const device = (id: number) => ({ id, name: `dev${id}`, ip: `10.0.0.${id}` })
    vi.mocked(api.get as any).mockImplementation((path: string, config: { params: { page: number; page_size: number } }) => {
      if (path === '/devices') {
        const page = config.params.page
        return Promise.resolve({
          data: { success: true, data: { devices: [device(page)] }, meta: { pagination: { page, has_next: page < 3 } } },
        })
      }
      return Promise.resolve(historyAnswer([]))
    })

    const wrapper = mount(GitOpsExportPage)
    await flushPromises()
    await wrapper.get('.primary-button').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="gitops-devices"] option')).toHaveLength(3)
  })

  it('keeps the preview of the newest request when an older one answers last', async () => {
    const sample = (text: string) => ({ data: { success: true, data: { preview: { sample_data: Buffer.from(text).toString('base64') } } } })
    let answerFirst: (v: unknown) => void = () => {}
    let previews = 0
    vi.mocked(api.get as any).mockResolvedValue(historyAnswer([]))
    vi.mocked(api.post as any).mockImplementation(() => {
      previews++
      return previews === 1
        ? new Promise((resolve) => {
            answerFirst = resolve
          })
        : Promise.resolve(sample('second tree'))
    })

    const wrapper = mount(GitOpsExportPage)
    await flushPromises()
    await wrapper.get('.primary-button').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="gitops-name"]').setValue('nightly')
    const previewButton = wrapper.findAll('.actions .secondary').find((b) => b.text() === 'Preview files')!
    await previewButton.trigger('click')
    await previewButton.trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="gitops-preview"]').text()).toBe('second tree')

    answerFirst(sample('first tree'))
    await flushPromises()

    expect(wrapper.get('[data-testid="gitops-preview"]').text()).toBe('second tree')
  })

  it('drops a preview when an option changes, also one still on its way', async () => {
    const sample = (text: string) => ({ data: { success: true, data: { preview: { sample_data: Buffer.from(text).toString('base64') } } } })
    let answer: (v: unknown) => void = () => {}
    vi.mocked(api.get as any).mockResolvedValue(historyAnswer([]))
    vi.mocked(api.post as any).mockImplementation(
      () =>
        new Promise((resolve) => {
          answer = resolve
        }),
    )

    const wrapper = mount(GitOpsExportPage)
    await flushPromises()
    await wrapper.get('.primary-button').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="gitops-name"]').setValue('nightly')
    const previewButton = wrapper.findAll('.actions .secondary').find((b) => b.text() === 'Preview files')!
    await previewButton.trigger('click')

    // The grouping changes while the answer for the old one is pending.
    await wrapper.get('[data-testid="gitops-group-by"]').setValue('type')
    answer(sample('location tree'))
    await flushPromises()
    expect(wrapper.find('[data-testid="gitops-preview"]').exists()).toBe(false)

    // A preview that arrived is dropped by the next change too.
    const second = previewButton.trigger('click')
    answer(sample('type tree'))
    await second
    await flushPromises()
    expect(wrapper.get('[data-testid="gitops-preview"]').text()).toBe('type tree')
    await wrapper.get('[data-testid="gitops-group-by"]').setValue('location')
    expect(wrapper.find('[data-testid="gitops-preview"]').exists()).toBe(false)
  })

  it('drops the list answer of a request that a newer one replaced', async () => {
    let answerFirst: (v: unknown) => void = () => {}
    let calls = 0
    vi.mocked(api.get as any).mockImplementation((_path: string, config: { params: { page_size: number } }) => {
      if (config?.params?.page_size !== 20) return Promise.resolve(historyAnswer([]))
      calls++
      return calls === 1
        ? new Promise((resolve) => {
            answerFirst = resolve
          })
        : Promise.resolve(historyAnswer([record('b', 'second')]))
    })

    const wrapper = mount(GitOpsExportPage)
    await flushPromises()
    // The first list request is still pending: change the status filter, which starts a newer one.
    await wrapper.get('.filter-select').setValue('true')
    await flushPromises()
    expect(wrapper.text()).toContain('second')

    answerFirst(historyAnswer([record('a', 'first')]))
    await flushPromises()

    expect(wrapper.text()).toContain('second')
    expect(wrapper.text()).not.toContain('first')
  })
})
