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

const SQLITE_ONLY = 'Restore is only available for SQLite'

// What GET /import/restore answers next; a test sets it before it mounts the page.
let restoreStatus: () => unknown

function statusAnswer(data: unknown) {
  return () => Promise.resolve({ data: { success: true, data } })
}

function restoreStatusCalls() {
  return vi.mocked(api.get as any).mock.calls.filter((c: unknown[]) => c[0] === '/import/restore').length
}

function refused(status: number, message: string) {
  return { response: { status, data: { success: false, error: { message } } } }
}

describe('BackupManagementPage backup check', () => {
  beforeEach(() => {
    vi.mocked(api.get as any).mockReset()
    vi.mocked(api.post as any).mockReset()
    vi.mocked(api.delete as any).mockReset()
    restoreStatus = statusAnswer({ pending: null, last_result: null })
    vi.mocked(api.get as any).mockImplementation((path: string) => {
      if (path === '/export/backups') {
        return Promise.resolve({
          data: { success: true, data: { backups: [backup('a', 'first'), backup('b', 'second')] }, meta: {} },
        })
      }
      if (path === '/import/restore') return restoreStatus()
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

  describe('staged restore', () => {
    async function openChecked(wrapper: ReturnType<typeof mount>) {
      await wrapper.findAll('.restore-btn')[1].trigger('click')
      await wrapper.get('.restore-modal .primary-button').trigger('click')
      await flushPromises()
    }

    it('stages the checked backup and says a restart applies it', async () => {
      vi.mocked(api.post as any).mockImplementation((path: string) =>
        path === '/import/restore'
          ? Promise.resolve({ status: 202, data: { success: true, data: { backup_id: 'b', applies: 'at the next start' } } })
          : Promise.resolve(checkAnswer('b', 2048)),
      )
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)
      const before = restoreStatusCalls()
      restoreStatus = statusAnswer({ pending: { backup_id: 'b', staged_at: '2026-11-20T18:00:00Z', sha256: 'x' }, last_result: null })

      await wrapper.get('.restore-modal .stage-button').trigger('click')
      await flushPromises()

      expect(api.post).toHaveBeenCalledWith('/import/restore', { backup_id: 'b' })
      expect(wrapper.get('[data-testid="restore-staged"]').text()).toContain('Restart the server to apply it.')
      expect(restoreStatusCalls()).toBe(before + 1)
      expect(wrapper.get('[data-testid="restore-pending"]').text()).toContain('second')
      wrapper.unmount()
    })

    it('shows the reason of a refused backup and stages nothing', async () => {
      vi.mocked(api.post as any).mockImplementation((path: string) =>
        path === '/import/restore'
          ? Promise.reject(refused(422, 'The backup is not a SQLite database'))
          : Promise.resolve(checkAnswer('b', 2048)),
      )
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)
      const before = restoreStatusCalls()

      await wrapper.get('.restore-modal .stage-button').trigger('click')
      await flushPromises()

      const text = wrapper.get('.restore-modal').text()
      expect(text).toContain('The backup is not a SQLite database')
      expect(text).toContain('Nothing was staged')
      expect(wrapper.find('[data-testid="restore-staged"]').exists()).toBe(false)
      expect(restoreStatusCalls()).toBe(before)
      wrapper.unmount()
    })

    it('offers no Stage after a recheck failed, whatever the earlier check said', async () => {
      let checks = 0
      vi.mocked(api.post as any).mockImplementation(() => {
        checks++
        return checks === 1 ? Promise.resolve(checkAnswer('b', 2048)) : Promise.reject(refused(404, 'Backup not found'))
      })
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)
      expect(wrapper.find('.stage-button').exists()).toBe(true)

      await wrapper.get('.restore-modal .primary-button').trigger('click')
      await flushPromises()

      expect(wrapper.get('.restore-modal').text()).toContain('Backup not found')
      expect(wrapper.find('.stage-button').exists()).toBe(false)
      wrapper.unmount()
    })

    it('does not say nothing was staged when the answer never arrived, and reads the status', async () => {
      vi.mocked(api.post as any).mockImplementation((path: string) =>
        path === '/import/restore' ? Promise.reject(new Error('Network Error')) : Promise.resolve(checkAnswer('b', 2048)),
      )
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)
      const before = restoreStatusCalls()
      // The server did stage it; only the answer was lost.
      restoreStatus = statusAnswer({ pending: { backup_id: 'b', staged_at: '2026-11-20T18:00:00Z', sha256: 'x' }, last_result: null })

      await wrapper.get('.restore-modal .stage-button').trigger('click')
      await flushPromises()

      const text = wrapper.get('.restore-modal').text()
      expect(text).toContain('not known whether the restore was staged')
      expect(text).not.toContain('Nothing was staged')
      expect(restoreStatusCalls()).toBe(before + 1)
      expect(wrapper.find('[data-testid="restore-cancel"]').exists()).toBe(true)
      wrapper.unmount()
    })

    it('reads the status after a timeout answer, since the stage may have gone on', async () => {
      vi.mocked(api.post as any).mockImplementation((path: string) =>
        path === '/import/restore' ? Promise.reject(refused(408, 'Request processing timeout')) : Promise.resolve(checkAnswer('b', 2048)),
      )
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)
      const before = restoreStatusCalls()
      restoreStatus = statusAnswer({ pending: { backup_id: 'b', staged_at: '2026-11-20T18:00:00Z', sha256: 'x' }, last_result: null })

      await wrapper.get('.restore-modal .stage-button').trigger('click')
      await flushPromises()

      const text = wrapper.get('.restore-modal').text()
      expect(text).toContain('not known whether the restore was staged')
      expect(text).not.toContain('Nothing was staged')
      expect(restoreStatusCalls()).toBe(before + 1)
      expect(wrapper.find('[data-testid="restore-cancel"]').exists()).toBe(true)
      wrapper.unmount()
    })

    it('reads the restore status again when the page is refreshed', async () => {
      restoreStatus = statusAnswer({ pending: { backup_id: 'a', staged_at: '2026-11-20T18:00:00Z', sha256: 'x' }, last_result: null })
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      expect(wrapper.find('[data-testid="restore-pending"]').exists()).toBe(true)
      // The server was restarted meanwhile and applied the restore.
      restoreStatus = statusAnswer({ pending: null, last_result: { status: 'applied', backup_id: 'a', at: '2026-11-20T19:00:00Z' } })

      await wrapper.get('.refresh-button').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="restore-pending"]').exists()).toBe(false)
      expect(wrapper.get('[data-testid="restore-last-result"]').text()).toContain('applied')
      wrapper.unmount()
    })

    it('learns from a 501 on the stage call that the database is not SQLite', async () => {
      restoreStatus = () => Promise.reject(refused(500, 'Internal error'))
      vi.mocked(api.post as any).mockImplementation((path: string) =>
        path === '/import/restore'
          ? Promise.reject(refused(501, 'Restore is available for SQLite databases only'))
          : Promise.resolve(checkAnswer('b', 2048)),
      )
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)

      await wrapper.get('.restore-modal .stage-button').trigger('click')
      await flushPromises()

      expect(wrapper.find('.stage-button').exists()).toBe(false)
      expect(wrapper.get('[data-testid="restore-unsupported"]').text()).toContain(SQLITE_ONLY)
      expect(wrapper.findAll('.restore-btn')[1].attributes('disabled')).toBeUndefined()
      wrapper.unmount()
    })

    it('keeps the newest status when an older read answers late', async () => {
      let answerFirst: (v: unknown) => void = () => {}
      let reads = 0
      restoreStatus = () => {
        reads++
        if (reads === 1) {
          return new Promise((resolve) => {
            answerFirst = resolve
          })
        }
        return statusAnswer({ pending: { backup_id: 'b', staged_at: '2026-11-20T18:00:00Z', sha256: 'x' }, last_result: null })()
      }
      vi.mocked(api.post as any).mockImplementation((path: string) =>
        path === '/import/restore'
          ? Promise.resolve({ status: 202, data: { success: true, data: {} } })
          : Promise.resolve(checkAnswer('b', 2048)),
      )
      const wrapper = mount(BackupManagementPage)
      await flushPromises()
      await openChecked(wrapper)
      await wrapper.get('.restore-modal .stage-button').trigger('click')
      await flushPromises()
      expect(wrapper.find('[data-testid="restore-pending"]').exists()).toBe(true)

      // The first read, taken before the restore was staged, answers now.
      answerFirst({ data: { success: true, data: { pending: null, last_result: null } } })
      await flushPromises()

      expect(wrapper.find('[data-testid="restore-pending"]').exists()).toBe(true)
      wrapper.unmount()
    })

    it('shows a pending restore with its backup and time, and Cancel calls DELETE', async () => {
      restoreStatus = statusAnswer({ pending: { backup_id: 'a', staged_at: '2026-11-20T18:00:00Z', sha256: 'x' }, last_result: null })
      vi.mocked(api.delete as any).mockResolvedValue({ status: 204 })
      const wrapper = mount(BackupManagementPage)
      await flushPromises()

      const pending = wrapper.get('[data-testid="restore-pending"]')
      expect(pending.text()).toContain('first')
      expect(pending.text()).toContain('restart')
      expect(pending.text()).toContain(new Date('2026-11-20T18:00:00Z').toLocaleString())
      restoreStatus = statusAnswer({ pending: null, last_result: null })

      await wrapper.get('[data-testid="restore-cancel"]').trigger('click')
      await flushPromises()

      expect(api.delete).toHaveBeenCalledWith('/import/restore')
      expect(wrapper.find('[data-testid="restore-pending"]').exists()).toBe(false)
      wrapper.unmount()
    })

    it('shows why the last start did not apply a restore', async () => {
      restoreStatus = statusAnswer({
        pending: null,
        last_result: { status: 'failed', backup_id: 'a', at: '2026-11-20T17:00:00Z', error: 'The restored database could not be opened' },
      })
      const wrapper = mount(BackupManagementPage)
      await flushPromises()

      const last = wrapper.get('[data-testid="restore-last-result"]')
      expect(last.text()).toContain('failed')
      expect(last.text()).toContain('The restored database could not be opened')
      wrapper.unmount()
    })

    it('shows an applied last restore', async () => {
      restoreStatus = statusAnswer({ pending: null, last_result: { status: 'applied', backup_id: 'a', at: '2026-11-20T17:00:00Z' } })
      const wrapper = mount(BackupManagementPage)
      await flushPromises()

      expect(wrapper.get('[data-testid="restore-last-result"]').text()).toContain('applied')
      wrapper.unmount()
    })

    it('keeps the check on a database that is not SQLite and offers no Stage', async () => {
      restoreStatus = () => Promise.reject(refused(501, 'Restore is available for SQLite databases only'))
      vi.mocked(api.post as any).mockResolvedValue(checkAnswer('b', 2048))
      const wrapper = mount(BackupManagementPage)
      await flushPromises()

      expect(wrapper.get('[data-testid="restore-unsupported"]').text()).toContain(SQLITE_ONLY)
      const button = wrapper.findAll('.restore-btn')[1]
      expect(button.attributes('disabled')).toBeUndefined()

      await openChecked(wrapper)

      expect(wrapper.get('.restore-preview').exists()).toBe(true)
      expect(wrapper.find('.stage-button').exists()).toBe(false)
      expect(wrapper.get('[data-testid="restore-sqlite-only"]').text()).toContain(SQLITE_ONLY)
      wrapper.unmount()
    })

    it('leaves the button enabled when the status cannot be read', async () => {
      restoreStatus = () => Promise.reject(refused(500, 'Internal error'))
      const wrapper = mount(BackupManagementPage)
      await flushPromises()

      expect(wrapper.findAll('.restore-btn')[0].attributes('disabled')).toBeUndefined()
      expect(wrapper.find('[data-testid="restore-unsupported"]').exists()).toBe(false)
      wrapper.unmount()
    })
  })
})
