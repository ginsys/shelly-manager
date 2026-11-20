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
