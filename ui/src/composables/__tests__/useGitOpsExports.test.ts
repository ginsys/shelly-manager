import { describe, it, expect, vi } from 'vitest'

vi.mock('@/api/export', () => ({
  listGitOpsExports: vi.fn(),
  getGitOpsExportStatistics: vi.fn().mockResolvedValue({ total: 0, success: 0, failure: 0, by_format: {}, by_structure: {}, total_files: 0, total_size: 0 }),
  getGitOpsExportResult: vi.fn().mockResolvedValue({ files: [], file_count: 0, total_size: 0 }),
  createGitOpsExport: vi.fn().mockResolvedValue({ export_id: 'exp-1' }),
  deleteGitOpsExport: vi.fn().mockResolvedValue({}),
  downloadGitOpsExport: vi.fn().mockResolvedValue(new Blob()),
  previewGitOpsExport: vi.fn().mockResolvedValue({ preview: { success: true } }),
}))

describe('useGitOpsExports', async () => {
  it('sets error on fetchExports failure', async () => {
    const { useGitOpsExports } = await import('../useGitOpsExports')
    const api = await import('@/api/export')
    ;(api.listGitOpsExports as any).mockRejectedValueOnce(new Error('load failed'))
    const vm = useGitOpsExports()
    await vm.fetchExports()
    expect(vm.error.value).toContain('load failed')
  })

  it('handleCreateExport shows success message', async () => {
    const { useGitOpsExports } = await import('../useGitOpsExports')
    const vm = useGitOpsExports()
    await vm.handleCreateExport({ name: 'My Export', description: '', format: 'terraform', devices: [], repository_structure: 'flat', template_options: {}, git_config: {}, variable_substitution: {}, include_secrets: false, generate_readme: true })
    expect(vm.message.text).toContain('created successfully')
  })
})

