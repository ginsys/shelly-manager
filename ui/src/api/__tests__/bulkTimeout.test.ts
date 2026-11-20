import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// The global test setup replaces axios with a stub; this file needs the real client.
vi.mock('axios', async () => await vi.importActual('axios'))

import type { AxiosRequestConfig, AxiosResponse } from 'axios'
import api from '../client'
import { bulkDownload, bulkDriftDetect, bulkImport, type BulkConfigResult } from '../bulk'
import { DEVICE_CALL_TIMEOUT_MS } from '../deviceCallTimeout'

// The real shared client, with a fake server in place of the network. The server answers a bulk read
// after `SERVER_ANSWERS_AFTER_MS`, the way a read of two devices does when one of them does not
// answer (ginsys/shelly-manager#752). The fake enforces `config.timeout` as axios does: the call ends
// with a timeout error when the browser gives up first.

const SERVER_ANSWERS_AFTER_MS = 12_000

const answer: BulkConfigResult = {
  total: 2,
  success: 1,
  failed: 1,
  skipped: 0,
  results: [
    { device_id: 1, device_name: 'Kitchen', device_ip: '10.0.0.4', status: 'success', config_id: 1 },
    {
      device_id: 2,
      device_name: 'Garage',
      device_ip: '10.0.0.5',
      status: 'failed',
      error: 'failed to get device info: context deadline exceeded'
    }
  ]
}

let seen: AxiosRequestConfig[] = []

function slowServer(config: AxiosRequestConfig): Promise<AxiosResponse> {
  seen.push(config)
  return new Promise((resolve, reject) => {
    const reply = setTimeout(() => {
      resolve({
        data: { success: true, data: answer, timestamp: '2026-11-20T09:13:41Z' },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: config as never
      })
    }, SERVER_ANSWERS_AFTER_MS)
    const limit = Number(config.timeout)
    if (limit > 0) {
      setTimeout(() => {
        clearTimeout(reply)
        reject(
          Object.assign(new Error(`timeout of ${limit}ms exceeded`), {
            isAxiosError: true,
            code: 'ECONNABORTED',
            config
          })
        )
      }, limit)
    }
  })
}

describe('a bulk read that contacts devices', () => {
  const original = api.defaults.adapter

  beforeEach(() => {
    vi.useFakeTimers()
    seen = []
    api.defaults.adapter = slowServer as never
  })

  afterEach(() => {
    api.defaults.adapter = original
    vi.useRealTimers()
  })

  it('shows a 12 s answer as a result, not as a timeout of 10000ms', async () => {
    const pending = bulkImport({ device_ids: [1, 2] })
    // Observe the outcome now, so a rejection at the 10 s mark is never an unhandled one.
    const outcome = pending.then(
      (value) => ({ value }),
      (error: Error) => ({ error })
    )

    await vi.advanceTimersByTimeAsync(SERVER_ANSWERS_AFTER_MS)
    const settled = await outcome

    expect(settled).toEqual({ value: answer })
    expect(seen[0].timeout).toBe(DEVICE_CALL_TIMEOUT_MS)
  })

  it('also waits for the drift read', async () => {
    const outcome = bulkDriftDetect({ device_ids: [1, 2] }).then(
      () => 'answered',
      () => 'failed'
    )

    await vi.advanceTimersByTimeAsync(SERVER_ANSWERS_AFTER_MS)

    expect(await outcome).toBe('answered')
  })

  it('ends with the axios timeout error, not a plain Error, when nothing answers in time', async () => {
    api.defaults.adapter = ((config: AxiosRequestConfig) => {
      seen.push(config)
      return new Promise((_resolve, reject) => {
        setTimeout(
          () =>
            reject(
              Object.assign(new Error(`timeout of ${config.timeout}ms exceeded`), {
                isAxiosError: true,
                code: 'ECONNABORTED'
              })
            ),
          Number(config.timeout)
        )
      })
    }) as never

    const outcome = bulkImport({ device_ids: [1, 2] }).catch((e: any) => e)
    await vi.advanceTimersByTimeAsync(DEVICE_CALL_TIMEOUT_MS)
    const error = await outcome

    expect(error.isAxiosError).toBe(true)
    expect(error.code).toBe('ECONNABORTED')
  })

  it('leaves the download, which reads only the database, at the shared 10 s', async () => {
    const outcome = bulkDownload({ device_ids: [1] }).catch((e: any) => e)

    await vi.advanceTimersByTimeAsync(10_000)
    await outcome

    expect(seen[0].timeout).toBe(10_000)
  })
})
