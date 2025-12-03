import { describe, it, expect, vi } from 'vitest'

vi.mock('@/api/export', () => ({
  listBackups: vi.fn(),
  getBackupStatistics: vi.fn().mockResolvedValue({ total: 0, success: 0, failure: 0, total_size: 0, by_format: {} }),
  createBackup: vi.fn().mockResolvedValue({}),
  createJSONExport: vi.fn().mockResolvedValue({}),
  createYAMLExport: vi.fn().mockResolvedValue({}),
  createSMAExport: vi.fn().mockResolvedValue({ export_id: 'sma-1' }),
  deleteBackup: vi.fn().mockResolvedValue({}),
  downloadBackupWithName: vi.fn(),
  downloadExportWithName: vi.fn(),
  previewRestore: vi.fn().mockRejectedValue(new Error('preview failed')),
  executeRestore: vi.fn().mockRejectedValue(new Error('execute failed')),
}))

vi.mock('@/api/client', () => ({
  default: {
    get: vi.fn((path: string) => {
      if (path === '/devices') return Promise.resolve({ data: { data: { devices: [] } } })
      if (path === '/version') return Promise.resolve({ data: { data: { database_provider_name: 'SQLite', database_provider_version: '3.x' } } })
      return Promise.resolve({ data: {} })
    })
  }
}))

describe('useBackups', async () => {
  it('sets error on fetchBackups failure', async () => {
    const { useBackups } = await import('../useBackups')
    const api = await import('@/api/export')
    ;(api.listBackups as any).mockRejectedValueOnce(new Error('boom'))
    const vm = useBackups()
    await vm.fetchBackups()
    expect(vm.error.value).toContain('boom')
  })

  it('creates backup via createBackupPanel and sets success message', async () => {
    const { useBackups } = await import('../useBackups')
    const vm = useBackups()
    vm.createType.value = 'backup' as any
    vm.createName.value = 'Nightly'
    await vm.createBackupPanel()
    expect(vm.createSubmitting.value).toBe(false)
    expect(vm.message.text).toContain('created successfully')
  })
})

