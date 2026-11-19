import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => ({
  default: { get: vi.fn() },
}))

import api from '../client'
import { downloadBackupWithName, downloadExportWithName } from '../export'

const envelope = (message: string) =>
  new Blob([JSON.stringify({ success: false, error: { code: 'UNAUTHORIZED', message } })])

// A content export and a backup download share one request shape and one failure shape.
describe.each([
  ['downloadExportWithName', downloadExportWithName, '/export/e1/download'],
  ['downloadBackupWithName', downloadBackupWithName, '/export/backup/e1/download'],
] as const)('%s', (_name, download, path) => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
  })

  it('returns the blob and the file name the server sends', async () => {
    const blob = new Blob(['x'])
    ;(api.get as any).mockResolvedValue({
      data: blob,
      headers: { 'content-disposition': 'attachment; filename="export-1.json"' },
    })
    expect(await download('e1')).toEqual({ blob, filename: 'export-1.json' })
    expect(api.get).toHaveBeenCalledWith(path, { responseType: 'blob' })
  })

  it('throws the message of the server error envelope, which a blob request delivers as a Blob', async () => {
    ;(api.get as any).mockRejectedValue(
      Object.assign(new Error('Request failed with status code 401'), {
        response: { status: 401, data: envelope('Admin authorization required') },
      }),
    )
    const err = await download('e1').catch((e) => e)
    expect(err.message).toBe('Admin authorization required')
    expect(err.message).not.toContain('status code')
  })

  it('shows the string details of the envelope when it has them: the message alone says only that validation failed', async () => {
    const withDetails = new Blob([
      JSON.stringify({
        success: false,
        error: { code: 'VALIDATION_FAILED', message: 'Validation failed', details: 'Missing export ID' },
      }),
    ])
    ;(api.get as any).mockRejectedValue(
      Object.assign(new Error('Request failed with status code 400'), { response: { status: 400, data: withDetails } }),
    )
    const err = await download('e1').catch((e) => e)
    expect(err.message).toBe('Missing export ID')
  })

  it('keeps the message when the details are not a string', async () => {
    const mapDetails = new Blob([
      JSON.stringify({
        success: false,
        error: { code: 'VALIDATION_FAILED', message: 'Validation failed', details: { field: 'why' } },
      }),
    ])
    ;(api.get as any).mockRejectedValue(
      Object.assign(new Error('Request failed with status code 400'), { response: { status: 400, data: mapDetails } }),
    )
    const err = await download('e1').catch((e) => e)
    expect(err.message).toBe('Validation failed')
  })

  it('rethrows the original error when the body is not an error envelope', async () => {
    const original = Object.assign(new Error('Network Error'), { response: { data: new Blob(['not json']) } })
    ;(api.get as any).mockRejectedValue(original)
    await expect(download('e1')).rejects.toBe(original)
  })

  it('rethrows the original error when there is no response', async () => {
    const original = new Error('Network Error')
    ;(api.get as any).mockRejectedValue(original)
    await expect(download('e1')).rejects.toBe(original)
  })
})
