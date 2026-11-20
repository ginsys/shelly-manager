import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  createGitOpsExport,
  deleteGitOpsExport,
  previewGitOpsExport,
  countGitOpsExports,
  decodePreviewSample,
  type GitOpsExportOptions,
} from './export'
import api from './client'

vi.mock('./client', () => ({
  default: { post: vi.fn(), get: vi.fn(), delete: vi.fn() },
}))

const mockApi = api as any

const options: GitOpsExportOptions = {
  name: 'nightly',
  description: 'all devices',
  group_by: 'type',
  include_common: true,
  include_templates: false,
  device_ids: [3, 4],
}

describe('GitOps export API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('posts only what the server reads: config and a device filter', async () => {
    mockApi.post.mockResolvedValue({ data: { success: true, data: { export_id: 'e1' } } })

    const result = await createGitOpsExport(options)

    expect(mockApi.post).toHaveBeenCalledWith('/export/gitops', {
      config: {
        name: 'nightly',
        description: 'all devices',
        group_by: 'type',
        include_common: true,
        include_templates: false,
      },
      filters: { device_ids: [3, 4] },
    })
    expect(result).toEqual({ export_id: 'e1' })
  })

  it('sends no device filter when none is chosen', async () => {
    mockApi.post.mockResolvedValue({ data: { success: true, data: { export_id: 'e1' } } })

    await createGitOpsExport({ ...options, device_ids: [] })

    expect(mockApi.post.mock.calls[0][1].filters).toEqual({ device_ids: undefined })
  })

  it('throws the server message when the create fails', async () => {
    mockApi.post.mockResolvedValue({ data: { success: false, error: { message: 'output_path is not empty' } } })

    await expect(createGitOpsExport(options)).rejects.toThrow('output_path is not empty')
  })

  it('previews through the generic preview route, for the gitops plugin', async () => {
    mockApi.post.mockResolvedValue({ data: { success: true, data: { preview: { sample_data: 'eA==' } } } })

    await previewGitOpsExport(options)

    const [path, body] = mockApi.post.mock.calls[0]
    expect(path).toBe('/export/preview')
    expect(body.plugin_name).toBe('gitops')
    expect(body.config.group_by).toBe('type')
    expect(body.filters).toEqual({ device_ids: [3, 4] })
  })

  it('decodes the preview sample as UTF-8, box-drawing characters included', () => {
    const tree = 'gitops/\n├── common.yaml\n└── kitchen-é.yaml\n'
    const base64 = Buffer.from(tree, 'utf8').toString('base64')

    expect(decodePreviewSample(base64)).toBe(tree)
  })

  it('deletes the record through the gitops delete route', async () => {
    mockApi.delete.mockResolvedValue({ status: 204 })

    await deleteGitOpsExport('e1')

    expect(mockApi.delete).toHaveBeenCalledWith('/export/gitops/e1')
  })

  it('throws when the delete is refused', async () => {
    mockApi.delete.mockResolvedValue({ status: 500 })

    await expect(deleteGitOpsExport('e1')).rejects.toThrow('Failed to delete GitOps export')
  })

  it('counts exports from the page count of a one-item history page', async () => {
    mockApi.get.mockResolvedValue({ data: { success: true, data: { history: [] }, meta: { pagination: { total_pages: 7 } } } })

    expect(await countGitOpsExports(true)).toBe(7)
    expect(mockApi.get).toHaveBeenCalledWith('/export/history', {
      params: { page: 1, page_size: 1, plugin: 'gitops', success: true },
    })
  })

  it('counts zero when the response has no page count', async () => {
    mockApi.get.mockResolvedValue({ data: { success: true, data: { history: [] } } })

    expect(await countGitOpsExports()).toBe(0)
  })
})
