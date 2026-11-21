import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => ({ default: { post: vi.fn() } }))

import api from '../client'
import { getCommonSettings } from '../commonSettings'

describe('getCommonSettings', () => {
  beforeEach(() => {
    ;(api.post as any).mockReset()
  })

  it('posts the device ids and maps the answer', async () => {
    ;(api.post as any).mockResolvedValue({
      data: {
        success: true,
        data: {
          compared: [1, 2],
          skipped: [{ device_id: 3, device_name: 'Hall', reason: 'no stored import' }],
          device_type: 'SHPLG-S',
          common: { mqtt: { enable: true } },
          differing: { mqtt: 2 },
        },
      },
    })

    const result = await getCommonSettings([1, 2, 3])

    expect(api.post).toHaveBeenCalledWith('/config/common-settings', { device_ids: [1, 2, 3] })
    expect(result).toEqual({
      compared: [1, 2],
      skipped: [{ deviceId: 3, deviceName: 'Hall', reason: 'no stored import' }],
      deviceType: 'SHPLG-S',
      common: { mqtt: { enable: true } },
      differing: { mqtt: 2 },
    })
  })

  it('fills what the answer leaves out', async () => {
    ;(api.post as any).mockResolvedValue({ data: { success: true, data: { compared: [1] } } })

    expect(await getCommonSettings([1])).toEqual({ compared: [1], skipped: [], deviceType: '', common: {}, differing: {} })
  })

  it('throws the server message when the answer is not a success', async () => {
    ;(api.post as any).mockResolvedValue({ data: { success: false, error: { message: 'common-settings takes at most 200 devices' } } })

    await expect(getCommonSettings([1])).rejects.toThrow('common-settings takes at most 200 devices')
  })
})
