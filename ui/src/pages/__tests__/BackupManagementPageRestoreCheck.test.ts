import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

vi.mock('@/api/client', () => ({
  default: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

import api from '@/api/client'
import BackupManagementPage from '../BackupManagementPage.vue'

function backup(id: string, name: string) {
  return {
    id: 1,
    backup_id: id,
    name,
    format: 'sqlite',
    device_count: 0,
    encrypted: false,
    success: true,
    created_at: '2026-11-19T10:00:00Z',
  }
}

function checkAnswer(id: string, size: number) {
  return {
    data: {
      success: true,
      data: { backup_id: id, name: id, valid: true, file_size: size, record_count: 1, created_at: '', errors: [], warnings: [] },
    },
  }
}

describe('BackupManagementPage backup check', () => {
  beforeEach(() => {
    vi.mocked(api.get as any).mockReset()
    vi.mocked(api.post as any).mockReset()
    vi.mocked(api.get as any).mockImplementation((path: string) => {
      if (path === '/export/backups') {
        return Promise.resolve({
          data: { success: true, data: { backups: [backup('a', 'first'), backup('b', 'second')] }, meta: {} },
        })
      }
      return Promise.resolve({ data: { success: true, data: {} } })
    })
  })

  it('drops the answer of a check that was started for another backup', async () => {
    let answerA: (v: unknown) => void = () => {}
    vi.mocked(api.post as any).mockImplementation((_path: string, body: { backup_id: string }) =>
      body.backup_id === 'a'
        ? new Promise((resolve) => {
            answerA = resolve
          })
        : Promise.resolve(checkAnswer('b', 2048)),
    )

    const wrapper = mount(BackupManagementPage)
    await flushPromises()
    const buttons = wrapper.findAll('.restore-btn')
    expect(buttons).toHaveLength(2)

    // Start a check of A, close the dialog while it is pending, then open B.
    await buttons[0].trigger('click')
    await wrapper.get('.restore-modal .primary-button').trigger('click')
    await wrapper.get('.restore-modal .close-button').trigger('click')
    await buttons[1].trigger('click')
    expect(wrapper.get('.restore-modal h4').text()).toBe('second')

    // A's answer arrives now: it must not appear in B's dialog.
    answerA(checkAnswer('a', 999999))
    await flushPromises()
    expect(wrapper.find('.restore-preview').exists()).toBe(false)
    expect(wrapper.get('.restore-modal .primary-button').text()).toBe('Check backup')

    // B's own check still works.
    await wrapper.get('.restore-modal .primary-button').trigger('click')
    await flushPromises()
    expect(wrapper.get('.restore-preview').text()).toContain('2 KB')
    wrapper.unmount()
  })
})
