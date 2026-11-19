import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => ({
  default: { get: vi.fn() },
}))

import api from '../client'
import { downloadExportWithName } from '../export'

const envelope = (message: string) =>
  new Blob([JSON.stringify({ success: false, error: { code: 'UNAUTHORIZED', message } })])

describe('downloadExportWithName', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
  })

  it('returns the blob and the file name the server sends', async () => {
    const blob = new Blob(['x'])
    ;(api.get as any).mockResolvedValue({
      data: blob,
      headers: { 'content-disposition': 'attachment; filename="export-1.json"' },
    })
    expect(await downloadExportWithName('e1')).toEqual({ blob, filename: 'export-1.json' })
    expect(api.get).toHaveBeenCalledWith('/export/e1/download', { responseType: 'blob' })
  })

  it('throws the message of the server error envelope, which a blob request delivers as a Blob', async () => {
    ;(api.get as any).mockRejectedValue(
      Object.assign(new Error('Request failed with status code 401'), {
        response: { status: 401, data: envelope('Admin authorization required') },
      }),
    )
    const err = await downloadExportWithName('e1').catch((e) => e)
    expect(err.message).toBe('Admin authorization required')
    expect(err.message).not.toContain('status code')
  })

  it('rethrows the original error when the body is not an error envelope', async () => {
    const original = Object.assign(new Error('Network Error'), { response: { data: new Blob(['not json']) } })
    ;(api.get as any).mockRejectedValue(original)
    await expect(downloadExportWithName('e1')).rejects.toBe(original)
  })

  it('rethrows the original error when there is no response', async () => {
    const original = new Error('Network Error')
    ;(api.get as any).mockRejectedValue(original)
    await expect(downloadExportWithName('e1')).rejects.toBe(original)
  })
})
