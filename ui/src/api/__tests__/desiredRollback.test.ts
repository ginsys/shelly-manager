import { describe, it, expect, beforeEach, vi } from 'vitest'

// The global test setup replaces axios with a stub. The rollback must meet the real client: its admin-key
// retry replays the request (and so the secrets in its body), and axios puts that body on the error it throws.
vi.mock('axios', async () => await vi.importActual('axios'))

import { AxiosError, type InternalAxiosRequestConfig } from 'axios'

type Answer = { status: number; body: unknown } | 'no-answer'

const SENTINEL = 'sentinel-secret-9f3a'

async function load() {
  vi.resetModules()
  const key = await import('../adminKey')
  const { api } = await import('../client')
  const mod = await import('../desiredRollback')
  const calls: { auth: string | undefined; url: string | undefined; data: unknown }[] = []
  let answer: (auth: string | undefined) => Answer = () => ({ status: 200, body: { success: true } })
  api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
    const auth = (config.headers as any)?.Authorization as string | undefined
    calls.push({ auth, url: config.url, data: config.data })
    const a = answer(auth)
    if (a === 'no-answer') throw new AxiosError('Network Error', AxiosError.ERR_NETWORK, config, null)
    const response = { data: a.body, status: a.status, statusText: '', headers: {}, config } as any
    if (a.status >= 200 && a.status < 300) return response
    throw new AxiosError(`Request failed with status code ${a.status}`, AxiosError.ERR_BAD_REQUEST, config, null, response)
  }
  return { key, mod, calls, setAnswer: (f: typeof answer) => (answer = f) }
}

const envelope = (code: string, message: string, details?: unknown) => ({
  success: false,
  error: { code, message, details },
})

async function failure(p: Promise<unknown>): Promise<any> {
  try {
    await p
  } catch (e) {
    return e
  }
  throw new Error('the call did not fail')
}

describe('desired configuration versions and rollback client', () => {
  beforeEach(() => {
    sessionStorage.clear()
    delete (window as any).__ADMIN_KEY__
  })

  it('reads the versions from the history route', async () => {
    const { mod, calls, setAnswer } = await load()
    const row = { id: 3, created_at: 'x', action: 'recompute', changed_by: 'u', restored_from: null, new_desired: '{}' }
    setAnswer(() => ({ status: 200, body: { success: true, data: { history: [row] } } }))
    expect(await mod.getDesiredHistory(7)).toEqual([row])
    expect(calls[0].url).toBe('/devices/7/desired-config/history')
  })

  it('posts the version and only the secrets it is given, and returns the answer', async () => {
    const { mod, calls, setAnswer } = await load()
    const data = { version_id: 9, no_change: false, detached_template_ids: [2], message: 'Desired configuration set to version 9.' }
    setAnswer(() => ({ status: 200, body: { success: true, data } }))
    expect(await mod.rollbackDesired(7, 4)).toEqual(data)
    await mod.rollbackDesired(7, 4, { 'mqtt.pass': 'x' })
    expect(calls[0].url).toBe('/devices/7/config/new/rollback')
    expect(JSON.parse(calls[0].data as string)).toEqual({ history_id: 4 })
    expect(JSON.parse(calls[1].data as string)).toEqual({ history_id: 4, secrets: { 'mqtt.pass': 'x' } })
  })

  it('throws a refusal that holds the status, the server text and the paths, and nothing of the request', async () => {
    const { mod, setAnswer } = await load()
    setAnswer(() => ({ status: 409, body: envelope('CONFLICT', 'This version holds secrets.', { paths: ['mqtt.pass', 'wifi.password'] }) }))
    const err = await failure(mod.rollbackDesired(7, 4, { 'mqtt.pass': SENTINEL }))

    expect(err).toBeInstanceOf(mod.RollbackRefused)
    expect([err.status, err.code, err.message, err.paths]).toEqual([409, 'CONFLICT', 'This version holds secrets.', ['mqtt.pass', 'wifi.password']])
    expect(err.config).toBeUndefined()
    expect(err.cause).toBeUndefined()
    expect(err.isAxiosError).toBeUndefined()
    expect(JSON.stringify(err)).not.toContain(SENTINEL)
    expect(String(err)).not.toContain(SENTINEL)
    expect(err.stack ?? '').not.toContain(SENTINEL)
  })

  it('gives no paths for a refusal that names none, and ignores a paths member that is no list of texts', async () => {
    const { mod, setAnswer } = await load()
    setAnswer(() => ({ status: 409, body: envelope('CONFLICT', 'Changed while prepared.') }))
    expect((await failure(mod.rollbackDesired(7, 4))).paths).toEqual([])
    setAnswer(() => ({ status: 422, body: envelope('CONFIGURATION_ERROR', 'Loss.', { paths: [1, 'a.b', null] }) }))
    expect((await failure(mod.rollbackDesired(7, 4))).paths).toEqual(['a.b'])
  })

  it('answers a request with no answer in one fixed text, never the transport message', async () => {
    const { mod, setAnswer } = await load()
    setAnswer(() => 'no-answer')
    const err = await failure(mod.rollbackDesired(7, 4, { 'mqtt.pass': SENTINEL }))
    expect(err).toBeInstanceOf(mod.RollbackRefused)
    expect(err.status).toBe(0)
    expect(err.message).toBe('The rollback request got no answer from the server.')
    expect(JSON.stringify(err)).not.toContain(SENTINEL)
  })

  it('names the status when the refusal has no envelope text', async () => {
    const { mod, setAnswer } = await load()
    setAnswer(() => ({ status: 502, body: '<html>bad gateway</html>' }))
    const err = await failure(mod.rollbackDesired(7, 4))
    expect([err.status, err.message]).toEqual([502, 'The rollback was refused (status 502).'])
  })

  it('opens the admin-key prompt on a 401 and retries once with the same body, secrets included', async () => {
    const { mod, key, calls, setAnswer } = await load()
    const data = { version_id: 9, no_change: false, detached_template_ids: [], message: 'done' }
    setAnswer((auth) =>
      auth === 'Bearer good'
        ? { status: 200, body: { success: true, data } }
        : { status: 401, body: envelope('UNAUTHORIZED', 'Admin authorization required') },
    )
    const pending = mod.rollbackDesired(7, 4, { 'mqtt.pass': SENTINEL })
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('good')
    expect(await pending).toEqual(data)
    expect(calls.map((c) => c.auth)).toEqual([undefined, 'Bearer good'])
    expect(calls[1].data).toBe(calls[0].data)
    expect(JSON.parse(calls[1].data as string).secrets).toEqual({ 'mqtt.pass': SENTINEL })
  })
})
