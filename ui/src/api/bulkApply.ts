import api from './client'
import type { APIResponse } from './types'
import type { BulkRequest, BulkTone, BulkView } from './bulk'

// The bulk apply route writes one partial configuration to each named device as that device's
// overrides, then applies it (ginsys/shelly-manager#594). It is the only bulk route that writes a
// device, so it has its own module: what the operator is told about a write matters more than the
// shape of the call.

// The most devices one call names. The route refuses more with a 400 that names the cap
// (`maxBulkApplyDevices` in the route). The operator must learn it before typing JSON, so the
// number is repeated here; the route stays the authority and a mismatch only shows its own text.
export const MAX_BULK_APPLY_DEVICES = 10

// The browser waits a little longer than the server's own deadline for this route, which is 5
// minutes (ginsys/shelly-manager#725: `bulkApplyDeadline` in the route). Every other route is cut at
// 30 s. If the route's deadline moves, this moves with it.
export const BULK_APPLY_TIMEOUT_MS = 310_000

export type BulkApplyStatus =
  | 'applied'
  | 'pending'
  | 'unconfirmed'
  | 'refused'
  | 'failed'
  | 'skipped'

export interface BulkApplyRequest extends BulkRequest {
  // A partial device configuration, as the body of PATCH /devices/{id}/overrides/new. `null` removes
  // an override and a secret sent as the mask keeps the stored one.
  config: Record<string, unknown>
}

export interface BulkApplyEntry {
  device_id: number
  device_name?: string
  device_ip?: string
  status: BulkApplyStatus
  // Whether the new overrides are stored; null when that is not known.
  stored: boolean | null
  // A fixed text that names what happened and never a value or a device's own error text.
  reason?: string
  // Setting paths only.
  failures?: string[]
  requires_reboot: boolean
}

export interface BulkApplyResult {
  total: number
  applied: number
  pending: number
  unconfirmed: number
  refused: number
  failed: number
  skipped: number
  results: BulkApplyEntry[]
}

// The answers the route gives before it touches a device: the admin check, a bad request, the cap,
// the size limit, the content type and the rate limit. Only these mean that nothing was written.
// Anything else (a 408 at the server's deadline, a 5xx, no answer at all) can follow writes to some
// devices, so it is never reported as a failure that changed nothing.
const REFUSED_BEFORE_ANY_WRITE = new Set([400, 401, 403, 413, 415, 429])

// An answer the route gave before it touched a device. The dialog keeps what the operator typed after
// this one, so it can be corrected, and forgets it after any other outcome.
export class NothingWrittenError extends Error {}

export const OUTCOME_UNKNOWN =
  'No answer arrived in time, or it could not be read. Some devices may have been written, and one may have been cut in the middle of the write. Check each device before sending again.'

// Write one partial configuration to the named devices and apply it.
export async function bulkApply(request: BulkApplyRequest): Promise<BulkApplyResult> {
  let body: APIResponse<BulkApplyResult>
  try {
    body = (
      await api.post<APIResponse<BulkApplyResult>>('/config/bulk-apply', request, {
        timeout: BULK_APPLY_TIMEOUT_MS
      })
    ).data
  } catch (e: any) {
    const status: number | undefined = e?.response?.status
    if (status !== undefined && REFUSED_BEFORE_ANY_WRITE.has(status)) {
      const error = e.response.data?.error
      const details = typeof error?.details === 'string' ? error.details : undefined
      throw new NothingWrittenError(
        `${details || error?.message || 'The request was refused.'} Nothing was written.`
      )
    }
    throw new Error(OUTCOME_UNKNOWN)
  }
  if (!body?.success || !body.data) throw new Error(OUTCOME_UNKNOWN)
  return body.data
}

const tone: Record<BulkApplyStatus, BulkTone> = {
  applied: 'success',
  pending: 'warning',
  unconfirmed: 'warning',
  refused: 'failure',
  failed: 'failure',
  skipped: 'skipped'
}

// What the operator reads for one device: what happened to the device and what is stored, from the
// status and `stored` alone, then the cause the route gave. Each sentence is a claim about the
// route, so each (status, stored) pair has a case in the tests.
function consequence(status: BulkApplyStatus, stored: boolean | null): string {
  const unknownStored = ' It is not known whether the new settings were stored.'
  const storedPending = ' The new settings are stored and the device stays pending.'
  switch (status) {
    case 'applied':
      return 'Written to the device and read back equal.' + (stored === null ? unknownStored : '')
    case 'pending':
      return (
        'Written and read back, but not marked applied.' + (stored === null ? unknownStored : storedPending)
      )
    case 'unconfirmed':
      return (
        'Written, but not verified. Find the device on its new network.' +
        (stored === null ? unknownStored : storedPending)
      )
    case 'refused':
      if (stored) return 'Nothing was written to the device.' + storedPending
      if (stored === false) return 'Nothing was written to the device and nothing was stored.'
      return 'Nothing was written to the device.' + unknownStored
    case 'failed':
      if (stored) {
        return 'The write did not end in a verified result: check the device.' + storedPending
      }
      if (stored === false) return 'Nothing was stored and nothing was written to the device.'
      return 'The new settings could not be stored, and it is not known whether they were. Nothing was written to the device.'
    case 'skipped':
      return stored === false
        ? 'Not contacted. Nothing was stored and nothing was written to the device.'
        : 'Not contacted.'
  }
}

function applyMessage(entry: BulkApplyEntry): string {
  // The route puts the cause first and the consequence after a `;`: the consequence is said above.
  const cause = entry.reason?.split(';')[0].trim()
  const parts = [consequence(entry.status, entry.stored)]
  if (cause) parts.push(`Cause: ${cause}.`)
  if (entry.failures?.length) parts.push(`Settings: ${entry.failures.join('; ')}.`)
  if (entry.requires_reboot) parts.push('This change requires the device to restart.')
  return parts.join(' ')
}

// The counts and rows the dialog shows for an answer of the route, in the order it gave them.
export function applyViewOf(result: BulkApplyResult): BulkView {
  return {
    stats: [
      { label: 'Applied', value: result.applied, tone: 'success' },
      { label: 'Pending', value: result.pending, tone: 'warning' },
      { label: 'Unconfirmed', value: result.unconfirmed, tone: 'warning' },
      { label: 'Refused', value: result.refused, tone: 'failure' },
      { label: 'Failed', value: result.failed, tone: 'failure' },
      { label: 'Skipped', value: result.skipped, tone: 'skipped' }
    ],
    rows: result.results.map((entry) => ({
      deviceId: entry.device_id,
      deviceName: entry.device_name ?? '',
      status: entry.status,
      tone: tone[entry.status] ?? 'failure',
      message: applyMessage(entry)
    }))
  }
}

// A result of this route, among the answers the dialog can hold.
export function isApplyResult(answer: unknown): answer is BulkApplyResult {
  return typeof answer === 'object' && answer !== null && 'applied' in answer && 'unconfirmed' in answer
}
