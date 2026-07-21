import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BackupList from '../BackupList.vue'
import type { BackupItem } from '@/api/export'

const backup: BackupItem = {
  id: 1,
  backup_id: 'b1',
  name: 'Backup One',
  format: 'sma',
  device_count: 3,
  encrypted: false,
  success: true,
  created_at: '2026-01-01T00:00:00Z',
}

function mountList(props: Partial<Record<string, unknown>> = {}) {
  return mount(BackupList, {
    props: { backups: [backup], loading: false, error: '', ...props },
  })
}

describe('BackupList restore gating (#249/#260)', () => {
  it('fails closed by default: restore button disabled and emits nothing', async () => {
    const wrapper = mountList() // no restoreEnabled -> defaults to disabled
    const btn = wrapper.find('.restore-btn')

    expect(btn.exists()).toBe(true)
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.attributes('title')).toContain('#249')

    await btn.trigger('click')
    expect(wrapper.emitted('restore')).toBeFalsy()
  })

  it('emits restore only when explicitly enabled', async () => {
    const wrapper = mountList({ restoreEnabled: true })
    const btn = wrapper.find('.restore-btn')

    expect(btn.attributes('disabled')).toBeUndefined()
    await btn.trigger('click')
    expect(wrapper.emitted('restore')).toBeTruthy()
  })
})
