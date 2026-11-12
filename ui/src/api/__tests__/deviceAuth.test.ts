import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => ({ default: { put: vi.fn() } }))

import api from '../client'
import { updateDeviceAuth } from '../devices'

const put = vi.mocked(api.put)

describe('updateDeviceAuth', () => {
  beforeEach(() => {
    put.mockReset()
  })

  it('puts the login to the device auth route', async () => {
    put.mockResolvedValue({ data: { success: true, data: { status: 'success' } } })
    await updateDeviceAuth(7, { username: 'admin', password: '********' })
    expect(put).toHaveBeenCalledWith('/devices/7/config/auth', { username: 'admin', password: '********' })
  })

  it('throws the server reason from a 400 body', async () => {
    put.mockRejectedValue({
      response: { data: { success: false, error: { message: 'A device login needs a password' } } },
    })
    await expect(updateDeviceAuth(7, { username: 'admin', password: '' })).rejects.toThrow(
      'A device login needs a password',
    )
  })

  it('throws the message of an unsuccessful body', async () => {
    put.mockResolvedValue({ data: { success: false, error: { message: 'nope' } } })
    await expect(updateDeviceAuth(7, { username: '', password: '' })).rejects.toThrow('nope')
  })

  it('rethrows an error that carries no server body', async () => {
    put.mockRejectedValue(new Error('Network Error'))
    await expect(updateDeviceAuth(7, { username: '', password: '' })).rejects.toThrow('Network Error')
  })
})
