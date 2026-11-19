import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

// The global test setup replaces axios with a stub. This file needs the real one, so the 401 path is
// exercised the way a browser meets it: axios builds the error (its own code, ERR_BAD_REQUEST) and
// the server's envelope sits in response.data.
vi.mock('axios', async () => await vi.importActual('axios'))

import { AxiosError, type InternalAxiosRequestConfig } from 'axios'

type Answer = { status: number; body: unknown }

async function load() {
  vi.resetModules()
  const key = await import('../adminKey')
  const { api } = await import('../client')
  const calls: { auth: string | undefined; url: string | undefined; data: unknown; responseType?: string }[] = []
  let answer: (auth: string | undefined, url: string | undefined) => Answer | Promise<Answer> = () => ({
    status: 200,
    body: { success: true },
  })
  api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
    const auth = (config.headers as any)?.Authorization as string | undefined
    calls.push({ auth, url: config.url, data: config.data, responseType: config.responseType })
    const a = await answer(auth, config.url)
    const response = { data: a.body, status: a.status, statusText: '', headers: {}, config } as any
    if (a.status >= 200 && a.status < 300) return response
    throw new AxiosError(
      `Request failed with status code ${a.status}`,
      a.status >= 500 ? AxiosError.ERR_BAD_RESPONSE : AxiosError.ERR_BAD_REQUEST,
      config,
      null,
      response,
    )
  }
  return { key, api, calls, setAnswer: (f: typeof answer) => (answer = f) }
}

const unauthorized = { status: 401, body: { success: false, error: { code: 'UNAUTHORIZED', message: 'Admin authorization required' } } }
const ok = { status: 200, body: { success: true, data: { ok: true } } }

describe('client admin-key handling', () => {
  beforeEach(() => {
    sessionStorage.clear()
    delete (window as any).__ADMIN_KEY__
  })
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('sends no Authorization header when no key is known', async () => {
    const { api, calls } = await load()
    await api.get('/devices')
    expect(calls[0].auth).toBeUndefined()
  })

  it('sends the entered key as a bearer token', async () => {
    const { api, key, calls } = await load()
    key.setAdminKey('abc')
    await api.get('/devices')
    expect(calls[0].auth).toBe('Bearer abc')
  })

  // The rotation call goes through the real client: the request that rotates carries the old key, and
  // every request after it carries the new one, whichever development source held the old key.
  it.each([
    ['VITE_ADMIN_KEY', () => vi.stubEnv('VITE_ADMIN_KEY', 'old-key')],
    ['window.__ADMIN_KEY__', () => ((window as any).__ADMIN_KEY__ = 'old-key')],
  ])('after a rotation the next request sends the new key, not the one from %s', async (_name, setOldKey) => {
    setOldKey()
    const { api, calls, setAnswer } = await load()
    const { rotateAdminKey } = await import('../admin')
    setAnswer((_auth, url) =>
      url === '/admin/rotate-admin-key' ? { status: 200, body: { success: true, data: { rotated: true } } } : ok,
    )

    await rotateAdminKey('fresh')
    await api.get('/devices')

    expect(calls.map((c) => [c.url, c.auth])).toEqual([
      ['/admin/rotate-admin-key', 'Bearer old-key'],
      ['/devices', 'Bearer fresh'],
    ])
  })

  it('opens the prompt on the first admin 401 and retries once with the entered key', async () => {
    const { api, key, calls, setAnswer } = await load()
    setAnswer((auth) => (auth === 'Bearer good' ? ok : unauthorized))

    const pending = api.get('/devices')
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('good')
    const res = await pending

    expect(res.data.data.ok).toBe(true)
    expect(calls.map((c) => c.auth)).toEqual([undefined, 'Bearer good'])
  })

  it('retries a POST body unchanged', async () => {
    const { api, key, calls, setAnswer } = await load()
    setAnswer((auth) => (auth === 'Bearer good' ? ok : unauthorized))
    const pending = api.post('/devices', { name: 'x' })
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('good')
    await pending
    expect(calls).toHaveLength(2)
    expect(calls[1].url).toBe('/devices')
    // the retry carries the same body as the first attempt
    expect(JSON.parse(calls[0].data as string)).toEqual({ name: 'x' })
    expect(calls[1].data).toBe(calls[0].data)
  })

  it('a stale 401 from a request sent without the new key retries with it and opens no dialog', async () => {
    const { api, key, calls, setAnswer } = await load()
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => (release = resolve))
    setAnswer(async (auth, url) => {
      if (auth === 'Bearer good') return ok
      if (url === '/b') await gate // /b was sent without a key and is refused late
      return unauthorized
    })

    const a = api.get('/a')
    const b = api.get('/b')
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('good')
    await a
    expect(key.promptOpen.value).toBe(false)

    release() // /b's 401 arrives after the key was entered and /a already retried with it
    const res = await b
    expect(res.data.data.ok).toBe(true)
    expect(key.promptOpen.value).toBe(false)
    expect(calls.filter((c) => c.url === '/b').map((c) => c.auth)).toEqual([undefined, 'Bearer good'])
  })

  it('a stale 401 retries once: a refused newer key still forgets it and asks again', async () => {
    const { api, key, calls, setAnswer } = await load()
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => (release = resolve))
    setAnswer(async (auth) => {
      if (auth === undefined) await gate // sent with no key, refused late
      return unauthorized
    })
    const pending = api.get('/old').catch((e) => e)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    key.setAdminKey('newer') // entered while the request was in flight
    release()
    const err: any = await pending

    // the current key is the one retry; it is refused too, so it is forgotten and the dialog asks again
    expect(err.response.status).toBe(401)
    expect(calls.map((c) => c.auth)).toEqual([undefined, 'Bearer newer'])
    expect(key.getAdminKey()).toBeUndefined()
    expect(key.promptOpen.value).toBe(true)
    expect(key.promptMessage.value).toBe('That key was not accepted.')
  })

  it('after Cancel, a late 401 from an older key whose retry is refused stays quiet', async () => {
    const { api, key, calls, setAnswer } = await load()
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => (release = resolve))
    setAnswer(async (auth, url) => {
      if (url === '/old' && auth === 'Bearer old') await gate // refused late
      return unauthorized
    })
    key.setAdminKey('old')
    const old = api.get('/old').catch((e) => e)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    key.setAdminKey('newer') // entered while /old was in flight
    const fresh = api.get('/fresh').catch((e) => e) // sent with newer, refused: the dialog opens
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.cancelAdminKeyPrompt()
    await fresh

    release() // /old's 401 arrives; its one retry goes out with newer and is refused too
    const err: any = await old

    expect(err.response.status).toBe(401)
    expect(calls.map((c) => c.auth)).toEqual(['Bearer old', 'Bearer newer', 'Bearer newer'])
    expect(key.promptOpen.value).toBe(false)
    expect(key.promptMessage.value).toBe('')
    expect(key.getAdminKey()).toBeUndefined() // the server refused it, so it is forgotten
  })

  it('after Cancel, a retry already in flight that is refused stays quiet', async () => {
    const { api, key, calls, setAnswer } = await load()
    let releaseOld: () => void = () => {}
    let releaseRetry: () => void = () => {}
    const oldGate = new Promise<void>((resolve) => (releaseOld = resolve))
    const retryGate = new Promise<void>((resolve) => (releaseRetry = resolve))
    setAnswer(async (auth, url) => {
      if (url === '/r' && auth === 'Bearer old') await oldGate
      if (url === '/r' && auth === 'Bearer newer') await retryGate
      return unauthorized
    })
    key.setAdminKey('old')
    const r = api.get('/r').catch((e) => e)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    key.setAdminKey('newer')
    releaseOld() // /r's 401 takes the shortcut: one retry with newer, held in flight
    await vi.waitFor(() => expect(calls.filter((c) => c.url === '/r')).toHaveLength(2))

    const x = api.get('/x').catch((e) => e) // another request, refused with newer: the dialog opens
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.cancelAdminKeyPrompt()
    await x

    releaseRetry() // now the retry is refused
    const err: any = await r

    expect(err.response.status).toBe(401)
    expect(key.promptOpen.value).toBe(false)
    expect(key.promptMessage.value).toBe('')
  })

  it('without a Cancel a refused retry still reopens the dialog, and again after the header button', async () => {
    const { api, key, setAnswer } = await load()
    setAnswer(() => unauthorized)
    const a = api.get('/a').catch((e) => e)
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.cancelAdminKeyPrompt()
    await a

    key.openAdminKeyPrompt() // the header button lifts the Cancel
    key.setAdminKey('wrong')
    key.promptOpen.value = false
    const b = api.get('/b').catch((e) => e) // sent with wrong, refused: the dialog opens
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('wrong2') // the retry with wrong2 is refused
    await b
    expect(key.promptOpen.value).toBe(true)
    expect(key.promptMessage.value).toBe('That key was not accepted.')
  })

  describe('a blob response (downloads)', () => {
    const blobOf = (text: string) => new Blob([text], { type: 'application/json' })
    const adminBlob = () => ({ status: 401, body: blobOf(JSON.stringify(unauthorized.body)) })

    it('reads the admin 401 from a JSON blob, prompts and retries with the same response type', async () => {
      const { api, key, calls, setAnswer } = await load()
      setAnswer((auth) => (auth === 'Bearer good' ? { status: 200, body: blobOf('file') } : adminBlob()))
      const pending = api.get('/export/1/download', { responseType: 'blob' })
      await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
      key.submitAdminKey('good')
      await pending
      expect(calls.map((c) => [c.auth, c.responseType])).toEqual([
        [undefined, 'blob'],
        ['Bearer good', 'blob'],
      ])
    })

    it('does not prompt for a blob that is not the admin envelope', async () => {
      const { api, key, setAnswer } = await load()
      for (const text of ['plain text', '{"success":false,"error":{"code":"DEVICE_AUTH"}}', '']) {
        setAnswer(() => ({ status: 401, body: blobOf(text) }))
        await expect(api.get('/export/1/download', { responseType: 'blob' })).rejects.toMatchObject({
          response: { status: 401 },
        })
        expect(key.promptOpen.value).toBe(false)
      }
    })

    it('hands the caller the original blob when the operator cancels', async () => {
      const { api, key, setAnswer } = await load()
      setAnswer(() => adminBlob())
      const settled = api.get('/export/1/download', { responseType: 'blob' }).catch((e) => e)
      await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
      key.cancelAdminKeyPrompt()
      const err: any = await settled
      expect(err.response.data).toBeInstanceOf(Blob)
    })
  })

  it('a slow refusal of a wrong key does not delete the key typed after it', async () => {
    const { api, key, setAnswer } = await load()
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => (release = resolve))
    setAnswer(async (auth, url) => {
      if (auth === 'Bearer good') return ok
      if (auth === 'Bearer wrong' && url === '/b') await gate // the second retry answers late
      return unauthorized
    })

    const a = api.get('/a').catch((e) => e)
    const b = api.get('/b').catch((e) => e)
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('wrong')
    await a // /a's retry is refused at once: the key is forgotten and the dialog reopens
    expect(key.promptMessage.value).toBe('That key was not accepted.')

    key.submitAdminKey('good') // the operator types the right key
    release() // now /b's retry, sent with the wrong key, is refused
    await b
    expect(key.getAdminKey()).toBe('good')
    expect(key.hasSessionKey.value).toBe(true)
  })

  it('a second 401 forgets the key, reopens the prompt with a message and rejects; no loop', async () => {
    const { api, key, calls, setAnswer } = await load()
    setAnswer(() => unauthorized)

    const pending = api.get('/devices')
    const settled = pending.then(
      () => 'resolved',
      (e) => e,
    )
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('wrong')
    const err: any = await settled

    expect(err.response.status).toBe(401)
    expect(calls).toHaveLength(2) // the original and exactly one retry
    expect(key.getAdminKey()).toBeUndefined()
    expect(key.hasSessionKey.value).toBe(false)
    expect(key.promptOpen.value).toBe(true)
    expect(key.promptMessage.value).toBe('That key was not accepted.')
  })

  it('does not prompt for a 401 whose body is not the admin guard', async () => {
    const { api, key, setAnswer } = await load()
    setAnswer(() => ({ status: 401, body: { success: false, error: { code: 'DEVICE_AUTH', message: 'device said no' } } }))
    await expect(api.get('/devices/1/status')).rejects.toMatchObject({ response: { status: 401 } })
    expect(key.promptOpen.value).toBe(false)
  })

  it('does not prompt for a 401 with a plain-text body', async () => {
    const { api, key, setAnswer } = await load()
    setAnswer(() => ({ status: 401, body: 'Agent not registered' }))
    await expect(api.get('/x')).rejects.toMatchObject({ response: { status: 401 } })
    expect(key.promptOpen.value).toBe(false)
  })

  it('reads the code from the body, never from axios error.code', async () => {
    const { api, key, setAnswer } = await load()
    // A forged shape: error.code is the string UNAUTHORIZED, the body code is not the admin guard.
    setAnswer(() => ({ status: 401, body: { success: false, error: { code: 'SOMETHING_ELSE' } } }))
    const err: any = await api.get('/x').catch((e) => e)
    expect(err.code).toBe('ERR_BAD_REQUEST')
    err.code = 'UNAUTHORIZED'
    // drive the response handler directly with that error
    const handler = (api.interceptors.response as any).handlers.find((h: any) => h?.rejected)?.rejected
    await expect(handler(err)).rejects.toBe(err)
    expect(key.promptOpen.value).toBe(false)
  })

  it('does not prompt for a non-401 failure', async () => {
    const { api, key, setAnswer } = await load()
    setAnswer(() => ({ status: 500, body: { success: false, error: { code: 'UNAUTHORIZED' } } }))
    await expect(api.get('/x')).rejects.toMatchObject({ response: { status: 500 } })
    expect(key.promptOpen.value).toBe(false)
  })

  it('concurrent 401s share one prompt and all retry with the key', async () => {
    const { api, key, calls, setAnswer } = await load()
    setAnswer((auth) => (auth === 'Bearer good' ? ok : unauthorized))
    const all = Promise.all([api.get('/a'), api.get('/b'), api.get('/c')])
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.submitAdminKey('good')
    await all
    expect(calls).toHaveLength(6)
    expect(calls.filter((c) => c.auth === 'Bearer good')).toHaveLength(3)
  })

  it('cancel rejects with the original 401', async () => {
    const { api, key, setAnswer } = await load()
    setAnswer(() => unauthorized)
    const pending = api.get('/devices')
    const settled = pending.catch((e) => e)
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.cancelAdminKeyPrompt()
    const err: any = await settled
    expect(err.response.status).toBe(401)
  })

  it('after cancel a further 401 rejects without reopening the prompt (a polling page stays quiet)', async () => {
    const { api, key, calls, setAnswer } = await load()
    setAnswer(() => unauthorized)
    const first = api.get('/devices/1/status').catch((e) => e)
    await vi.waitFor(() => expect(key.promptOpen.value).toBe(true))
    key.cancelAdminKeyPrompt()
    await first

    const before = calls.length
    await expect(api.get('/devices/1/status')).rejects.toMatchObject({ response: { status: 401 } })
    expect(calls.length).toBe(before + 1) // one call, no retry
    expect(key.promptOpen.value).toBe(false)

    // the header button lifts the cancel
    key.openAdminKeyPrompt()
    key.cancelAdminKeyPrompt()
    key.openAdminKeyPrompt()
    const again = api.get('/devices/1/status').catch((e) => e)
    expect(key.promptOpen.value).toBe(true)
    key.cancelAdminKeyPrompt()
    await again
  })
})
