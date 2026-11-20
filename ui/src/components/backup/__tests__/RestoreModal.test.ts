import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import RestoreModal from '../RestoreModal.vue'
import type { BackupItem, RestorePreview } from '@/api/export'

const backup: BackupItem = {
  id: 1,
  backup_id: 'b1',
  name: 'Nightly',
  format: 'sqlite',
  device_count: 3,
  encrypted: false,
  success: true,
  created_at: '2026-11-20T08:00:00Z',
}

const WARNING = "Only the file's presence and size were checked; its contents were not read."

function preview(valid: boolean): RestorePreview {
  return {
    backup_id: 'b1',
    name: 'Nightly',
    valid,
    file_size: 4096,
    record_count: 3,
    created_at: '2026-11-20T08:00:00Z',
    warnings: [WARNING],
    errors: valid ? [] : ['The backup file is empty or cannot be opened'],
  }
}

function mountModal(props: Record<string, unknown> = {}) {
  return mount(RestoreModal, {
    props: { show: true, backup, preview: null, loading: false, error: '', ...props },
  })
}

describe('RestoreModal', () => {
  it('offers no Stage restore before the backup is checked', () => {
    const wrapper = mountModal()

    expect(wrapper.find('.stage-button').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('not offered')
  })

  it('offers no Stage restore for a backup that is not usable', () => {
    const wrapper = mountModal({ preview: preview(false) })

    expect(wrapper.find('.stage-button').exists()).toBe(false)
  })

  it('offers Stage restore after a usable check and emits it', async () => {
    const wrapper = mountModal({ preview: preview(true) })

    await wrapper.get('.stage-button').trigger('click')

    expect(wrapper.emitted('stage-restore')).toHaveLength(1)
  })

  it('does not take a usable check for proof that the backup restores', () => {
    const wrapper = mountModal({ preview: preview(true) })

    expect(wrapper.text()).toContain(WARNING)
    expect(wrapper.text()).not.toMatch(/will restore|safe to restore/i)
  })

  it('says a restart applies the restore once it is staged, and offers nothing more', () => {
    const wrapper = mountModal({ preview: preview(true), staged: true })

    expect(wrapper.get('[data-testid="restore-staged"]').text()).toBe(
      'The restore is applied when the server next starts. Restart the server to apply it.',
    )
    expect(wrapper.find('.stage-button').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Check backup')
  })

  it('keeps the check and withholds Stage where the database is not SQLite', () => {
    const wrapper = mountModal({ preview: preview(true), restoreSupported: false })

    expect(wrapper.get('[data-testid="restore-sqlite-only"]').text()).toContain('only available for SQLite')
    expect(wrapper.find('.stage-button').exists()).toBe(false)
    expect(wrapper.text()).toContain('Check backup')
  })

  it('says a staged restore is replaced', () => {
    const wrapper = mountModal({ preview: preview(true), replacesPending: true })

    expect(wrapper.text()).toContain('This replaces the restore already staged.')
  })

  it('disables Stage restore while it runs', () => {
    const wrapper = mountModal({ preview: preview(true), staging: true })

    expect(wrapper.get('.stage-button').attributes('disabled')).toBeDefined()
  })
})
