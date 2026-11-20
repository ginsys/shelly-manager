import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn()
    }
  }
})

import api from '../client'
import {
  BULK_APPLY_TIMEOUT_MS,
  MAX_BULK_APPLY_DEVICES,
  NothingWrittenError,
  OUTCOME_UNKNOWN,
  applyViewOf,
  bulkApply,
  isApplyResult,
  type BulkApplyEntry,
  type BulkApplyResult,
  type BulkApplyStatus
} from '../bulkApply'

const mockedApi = api as unknown as { post: ReturnType<typeof vi.fn> }

function entry(over: Partial<BulkApplyEntry> = {}): BulkApplyEntry {
  return {
    device_id: 1,
    device_name: 'Kitchen',
    device_ip: '10.0.0.5',
    status: 'applied',
    stored: true,
    requires_reboot: false,
    ...over
  }
}

function result(entries: BulkApplyEntry[]): BulkApplyResult {
  const count = (s: BulkApplyStatus) => entries.filter((e) => e.status === s).length
  return {
    total: entries.length,
    applied: count('applied'),
    pending: count('pending'),
    unconfirmed: count('unconfirmed'),
    refused: count('refused'),
    failed: count('failed'),
    skipped: count('skipped'),
    results: entries
  }
}

function envelope(data: BulkApplyResult) {
  return { data: { success: true, data } }
}

// An axios error: a response when the server answered, none when it did not.
function httpError(status: number, error: { message?: string; details?: unknown } = {}) {
  return { response: { status, data: { success: false, error } } }
}

describe('bulkApply', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('posts the devices, the configuration and the options with the long timeout', async () => {
    const answer = result([entry()])
    mockedApi.post.mockResolvedValue(envelope(answer))

    const got = await bulkApply({
      device_ids: [1, 2],
      config: { name: 'x' },
      options: { stop_on_error: true }
    })

    expect(got).toEqual(answer)
    expect(mockedApi.post).toHaveBeenCalledTimes(1)
    expect(mockedApi.post).toHaveBeenCalledWith(
      '/config/bulk-apply',
      { device_ids: [1, 2], config: { name: 'x' }, options: { stop_on_error: true } },
      { timeout: BULK_APPLY_TIMEOUT_MS }
    )
  })

  it('waits longer than the 5 minutes the route allows itself', () => {
    expect(BULK_APPLY_TIMEOUT_MS).toBeGreaterThan(5 * 60 * 1000)
  })

  it('knows the cap of the route', () => {
    expect(MAX_BULK_APPLY_DEVICES).toBe(10)
  })

  // The answers the route gives before it touches a device. Only these say that nothing was written.
  it.each([400, 401, 403, 413, 415, 429])(
    'a %i says the route’s own text and that nothing was written',
    async (status) => {
      mockedApi.post.mockRejectedValue(httpError(status, { message: 'm', details: 'The reason.' }))

      const err = await bulkApply({ device_ids: [1], config: { a: 1 } }).catch((e: Error) => e)

      expect(err).toBeInstanceOf(NothingWrittenError)
      expect((err as Error).message).toBe('The reason. Nothing was written.')
    }
  )

  it('falls back to the message, then to a fixed text, when a refusal has no details', async () => {
    mockedApi.post.mockRejectedValueOnce(httpError(403, { message: 'Forbidden' }))
    await expect(bulkApply({ device_ids: [1], config: { a: 1 } })).rejects.toThrow(
      'Forbidden Nothing was written.'
    )
    mockedApi.post.mockRejectedValueOnce(httpError(429))
    await expect(bulkApply({ device_ids: [1], config: { a: 1 } })).rejects.toThrow(
      'The request was refused. Nothing was written.'
    )
  })

  // Everything else can follow writes to some devices, and one may have been cut in the middle.
  it.each([
    ['a 408 at the server’s deadline', httpError(408, { message: 'Request timeout' })],
    ['a 500', httpError(500, { message: 'boom' })],
    ['a 503', httpError(503)],
    ['no answer at all', new Error('Network Error')],
    ['a client timeout', Object.assign(new Error('timeout of 310000ms exceeded'), { code: 'ECONNABORTED' })]
  ])('%s is an unknown outcome, never a failure that changed nothing', async (_name, failure) => {
    mockedApi.post.mockRejectedValue(failure)

    const err = await bulkApply({ device_ids: [1], config: { a: 1 } }).catch((e: Error) => e)

    expect(err).toBeInstanceOf(Error)
    expect(err).not.toBeInstanceOf(NothingWrittenError)
    expect((err as Error).message).toBe(OUTCOME_UNKNOWN)
    expect((err as Error).message).not.toMatch(/nothing was written/i)
    expect((err as Error).message).not.toMatch(/failed/i)
    expect((err as Error).message).toMatch(/check each device/i)
  })

  it('an unsuccessful 200 is an unknown outcome too', async () => {
    mockedApi.post.mockResolvedValueOnce({ data: { success: false } })
    await expect(bulkApply({ device_ids: [1], config: { a: 1 } })).rejects.toThrow(OUTCOME_UNKNOWN)
    mockedApi.post.mockResolvedValueOnce({ data: { success: true } })
    await expect(bulkApply({ device_ids: [1], config: { a: 1 } })).rejects.toThrow(OUTCOME_UNKNOWN)
  })
})

// What the operator reads for one device is the row's message, so it is read through the view.
const applyMessage = (e: BulkApplyEntry) => applyViewOf(result([e])).rows[0].message

describe('the message of a device', () => {
  // One case per (status, stored) pair: each sentence is a claim about what the route did.
  const stored = {
    applied: {
      true: 'Written to the device and read back equal.',
      false: 'Written to the device and read back equal.',
      null: 'Written to the device and read back equal. It is not known whether the new settings were stored.'
    },
    pending: {
      true: 'Written and read back, but not marked applied. The new settings are stored and the device stays pending.',
      false: 'Written and read back, but not marked applied. The new settings are stored and the device stays pending.',
      null: 'Written and read back, but not marked applied. It is not known whether the new settings were stored.'
    },
    unconfirmed: {
      true: 'Written, but not verified. Find the device on its new network. The new settings are stored and the device stays pending.',
      false: 'Written, but not verified. Find the device on its new network. The new settings are stored and the device stays pending.',
      null: 'Written, but not verified. Find the device on its new network. It is not known whether the new settings were stored.'
    },
    refused: {
      true: 'Nothing was written to the device. The new settings are stored and the device stays pending.',
      false: 'Nothing was written to the device and nothing was stored.',
      null: 'Nothing was written to the device. It is not known whether the new settings were stored.'
    },
    failed: {
      true: 'The write did not end in a verified result: check the device. The new settings are stored and the device stays pending.',
      false: 'Nothing was stored and nothing was written to the device.',
      null: 'The new settings could not be stored, and it is not known whether they were. Nothing was written to the device.'
    },
    skipped: {
      true: 'Not contacted.',
      false: 'Not contacted. Nothing was stored and nothing was written to the device.',
      null: 'Not contacted.'
    }
  } as const

  for (const status of Object.keys(stored) as BulkApplyStatus[]) {
    for (const key of ['true', 'false', 'null'] as const) {
      const value = key === 'null' ? null : key === 'true'
      it(`${status} with stored ${key}`, () => {
        expect(applyMessage(entry({ status, stored: value }))).toBe(stored[status][key])
      })
    }
  }

  it('says that a status where nothing was stored does not claim the device stays pending', () => {
    for (const status of ['refused', 'failed', 'skipped'] as const) {
      expect(applyMessage(entry({ status, stored: false }))).not.toMatch(/stays pending/)
    }
  })

  it('adds the cause up to its first semicolon, the settings by name and the restart line', () => {
    const msg = applyMessage(
      entry({
        status: 'refused',
        stored: true,
        reason: 'the device refused a write; the device stays pending',
        failures: ['wifi.sta.pass', 'name'],
        requires_reboot: true
      })
    )

    expect(msg).toContain('Cause: the device refused a write.')
    // The route's own consequence after the semicolon is not repeated: the sentence above says it.
    expect(msg).not.toContain('Cause: the device refused a write; the device stays pending')
    expect(msg.match(/stays pending/g)).toHaveLength(1)
    expect(msg).toContain('Settings: wifi.sta.pass; name.')
    expect(msg.endsWith('This change requires the device to restart.')).toBe(true)
  })

  it('names the skipped device’s reason when the time limit ended the call', () => {
    const msg = applyMessage(
      entry({
        status: 'skipped',
        stored: false,
        reason: 'not contacted: the time limit of this call was reached or the request ended'
      })
    )
    expect(msg).toContain('Not contacted. Nothing was stored')
    expect(msg).toContain('Cause: not contacted: the time limit of this call was reached or the request ended.')
  })

  it('adds nothing when the route gave no cause, no settings and no restart', () => {
    expect(applyMessage(entry())).toBe('Written to the device and read back equal.')
  })
})

describe('applyViewOf', () => {
  it('gives the six counts with their tones and one row per device in the answer’s order', () => {
    const view = applyViewOf(
      result([
        entry({ device_id: 3, device_name: 'C', status: 'applied' }),
        entry({ device_id: 1, device_name: 'A', status: 'failed', stored: false }),
        entry({ device_id: 2, device_name: 'B', status: 'skipped', stored: false })
      ])
    )

    expect(view.stats).toEqual([
      { label: 'Applied', value: 1, tone: 'success' },
      { label: 'Pending', value: 0, tone: 'warning' },
      { label: 'Unconfirmed', value: 0, tone: 'warning' },
      { label: 'Refused', value: 0, tone: 'failure' },
      { label: 'Failed', value: 1, tone: 'failure' },
      { label: 'Skipped', value: 1, tone: 'skipped' }
    ])
    expect(view.rows.map((r) => [r.deviceId, r.deviceName, r.status, r.tone])).toEqual([
      [3, 'C', 'applied', 'success'],
      [1, 'A', 'failed', 'failure'],
      [2, 'B', 'skipped', 'skipped']
    ])
    expect(view.rows[2].message).toMatch(/^Not contacted\./)
  })

  it('tones pending and unconfirmed as warnings and an unknown status as a failure', () => {
    const view = applyViewOf(
      result([
        entry({ status: 'pending' }),
        entry({ status: 'unconfirmed' }),
        entry({ status: 'mystery' as BulkApplyStatus })
      ])
    )
    expect(view.rows.map((r) => r.tone)).toEqual(['warning', 'warning', 'failure'])
  })

  it('shows an empty name when the route gave none', () => {
    const view = applyViewOf(result([entry({ device_name: undefined })]))
    expect(view.rows[0].deviceName).toBe('')
  })
})

describe('isApplyResult', () => {
  it('tells the answer of this route from the other bulk answers', () => {
    expect(isApplyResult(result([entry()]))).toBe(true)
    expect(isApplyResult({ total: 1, success: 1, failed: 0, skipped: 0, results: [] })).toBe(false)
    expect(isApplyResult(null)).toBe(false)
    expect(isApplyResult('x')).toBe(false)
  })
})
