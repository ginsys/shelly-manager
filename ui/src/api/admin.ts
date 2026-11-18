import api from './client'
import { setAdminKey } from './adminKey'
import type { APIResponse } from './types'

export async function rotateAdminKey(newKey: string): Promise<boolean> {
  const res = await api.post<APIResponse<{ rotated: boolean }>>('/admin/rotate-admin-key', { new_key: newKey })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to rotate admin key')
  }
  // The new key is what the next request and the metrics socket send
  setAdminKey(newKey)
  return !!res.data.data.rotated
}

