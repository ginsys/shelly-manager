import api from './client'
import type { APIResponse } from './types'

// The versions of a device's desired configuration and the rollback to one (ginsys/shelly-manager#775).
// `new_desired` is the stored text with every secret masked, or null where no text could be stored.
export interface DesiredVersion {
  id: number
  created_at: string
  action: string
  changed_by: string
  restored_from: number | null
  new_desired: string | null
}

export interface RollbackResult {
  version_id: number
  no_change: boolean
  detached_template_ids: number[]
  message: string
}

// Secrets by path: a text, or a list of texts for an action URL list.
export type RollbackSecrets = Record<string, string | string[]>

// A refused or failed rollback. It is built from the server's own `error` and nothing else: an axios error
// holds the request, which holds the secrets, so it is never thrown on or kept as a `cause`.
export class RollbackRefused extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly paths: string[],
  ) {
    super(message)
    this.name = 'RollbackRefused'
  }
}

export async function getDesiredHistory(id: number | string): Promise<DesiredVersion[]> {
  const res = await api.get<APIResponse<{ history: DesiredVersion[] }>>(`/devices/${id}/desired-config/history`)
  const rows = res?.data?.data?.history
  if (!Array.isArray(rows)) throw new Error('Unexpected desired configuration versions response')
  return rows
}

export async function rollbackDesired(
  id: number | string,
  historyId: number,
  secrets?: RollbackSecrets,
): Promise<RollbackResult> {
  let res
  try {
    res = await api.post<APIResponse<RollbackResult>>(`/devices/${id}/config/new/rollback`, {
      history_id: historyId,
      ...(secrets ? { secrets } : {}),
    })
  } catch (e: any) {
    const status: number = e?.response?.status ?? 0
    const error = e?.response?.data?.error
    const paths: unknown = error?.details?.paths
    const text = typeof error?.message === 'string' && error.message !== '' ? error.message : ''
    throw new RollbackRefused(
      status,
      typeof error?.code === 'string' ? error.code : '',
      text || (status === 0 ? 'The rollback request got no answer from the server.' : `The rollback was refused (status ${status}).`),
      Array.isArray(paths) ? paths.filter((p): p is string => typeof p === 'string') : [],
    )
  }
  const data = res?.data?.data
  if (!res?.data?.success || !data) throw new RollbackRefused(res?.status ?? 0, '', 'The rollback answer was not understood.', [])
  return data
}
