import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

vi.mock('../client', () => ({ default: { post: vi.fn() } }))

import api from '../client'
import { rotateAdminKey } from '../admin'
import { getAdminKey, forgetAdminKey } from '../adminKey'

const post = vi.mocked(api.post)

describe('rotateAdminKey', () => {
  beforeEach(() => {
    sessionStorage.clear()
    forgetAdminKey()
    delete (window as any).__ADMIN_KEY__
    post.mockReset()
  })
  afterEach(() => forgetAdminKey())

  it('makes the new key the one the next request sends', async () => {
    post.mockResolvedValue({ data: { success: true, data: { rotated: true } } })
    await expect(rotateAdminKey('fresh')).resolves.toBe(true)
    expect(getAdminKey()).toBe('fresh')
  })

  it('does not write the window global', async () => {
    post.mockResolvedValue({ data: { success: true, data: { rotated: true } } })
    await rotateAdminKey('fresh')
    expect((window as any).__ADMIN_KEY__).toBeUndefined()
  })

  it('keeps the old key when the rotation fails', async () => {
    post.mockResolvedValue({ data: { success: false, error: { message: 'no' } } })
    await expect(rotateAdminKey('fresh')).rejects.toThrow('no')
    expect(getAdminKey()).toBeUndefined()
  })
})
