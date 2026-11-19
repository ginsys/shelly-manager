import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

// The module holds state (the prompt and a declined flag), so each test gets a fresh copy.
async function load() {
  vi.resetModules()
  return await import('../adminKey')
}

// Some Node versions leave localStorage undefined under jsdom, so the test supplies its own and
// records every write: the key must never reach it.
const localWrites: string[] = []
const fakeLocalStorage = {
  getItem: () => null,
  setItem: (k: string) => void localWrites.push(k),
  removeItem: () => {},
  clear: () => {},
}

// jsdom's sessionStorage is not reachable through Storage.prototype spies, so a failing storage is a
// stand-in global with the same three methods.
function failingStorage(opts: { get?: boolean; set?: boolean; remove?: boolean; initial?: string }) {
  let stored: string | null = opts.initial ?? null
  const boom = () => {
    throw new Error('denied')
  }
  return {
    getItem: opts.get ? boom : () => stored,
    setItem: opts.set ? boom : (_k: string, v: string) => void (stored = v),
    removeItem: opts.remove ? boom : () => void (stored = null),
    clear: () => {},
  }
}

describe('adminKey store', () => {
  beforeEach(() => {
    sessionStorage.clear()
    localWrites.length = 0
    vi.stubGlobal('localStorage', fakeLocalStorage)
    delete (window as any).__ADMIN_KEY__
  })
  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('keeps an entered key in sessionStorage and never in localStorage', async () => {
    const m = await load()
    m.setAdminKey('s3cret')
    expect(m.getAdminKey()).toBe('s3cret')
    expect(Object.values({ ...sessionStorage })).toContain('s3cret')
    expect(localWrites).toEqual([])
  })

  it('reads the key back after a page reload (a new module copy, same tab)', async () => {
    const first = await load()
    first.setAdminKey('s3cret')
    const second = await load()
    expect(second.getAdminKey()).toBe('s3cret')
    expect(second.hasSessionKey.value).toBe(true)
  })

  it('prefers the entered key over VITE_ADMIN_KEY, and that over the window global', async () => {
    const m = await load()
    ;(window as any).__ADMIN_KEY__ = 'window-key'
    expect(m.getAdminKey()).toBe('window-key')
    vi.stubEnv('VITE_ADMIN_KEY', 'env-key')
    expect(m.getAdminKey()).toBe('env-key')
    m.setAdminKey('entered')
    expect(m.getAdminKey()).toBe('entered')
  })

  it('forget clears the entered key and falls back to the dev sources', async () => {
    const m = await load()
    ;(window as any).__ADMIN_KEY__ = 'window-key'
    m.setAdminKey('entered')
    m.forgetAdminKey()
    expect(m.getAdminKey()).toBe('window-key')
    expect(m.hasSessionKey.value).toBe(false)
    expect(Object.values({ ...sessionStorage })).toHaveLength(0)
  })

  it('has no key when nothing is set', async () => {
    const m = await load()
    expect(m.getAdminKey()).toBeUndefined()
  })

  it('keeps the key in memory when sessionStorage throws', async () => {
    vi.stubGlobal('sessionStorage', failingStorage({ get: true, set: true, remove: true }))
    const m = await load()
    m.setAdminKey('mem-key')
    expect(m.getAdminKey()).toBe('mem-key')
    m.forgetAdminKey()
    expect(m.getAdminKey()).toBeUndefined()
  })

  it('opens one shared prompt for any number of waiting requests', async () => {
    const m = await load()
    const a = m.requestAdminKey()
    const b = m.requestAdminKey()
    expect(a).toBe(b)
    expect(m.promptOpen.value).toBe(true)
    m.submitAdminKey('k1')
    await expect(a).resolves.toBe('k1')
    expect(m.promptOpen.value).toBe(false)
    expect(m.getAdminKey()).toBe('k1')
  })

  it('cancel resolves the waiting requests with null and closes the prompt', async () => {
    const m = await load()
    const p = m.requestAdminKey()
    m.cancelAdminKeyPrompt()
    await expect(p).resolves.toBeNull()
    expect(m.promptOpen.value).toBe(false)
    expect(m.hasSessionKey.value).toBe(false)
  })

  it('after cancel a later request does not reopen the prompt', async () => {
    const m = await load()
    const first = m.requestAdminKey()
    m.cancelAdminKeyPrompt()
    await first
    await expect(m.requestAdminKey()).resolves.toBeNull()
    expect(m.promptOpen.value).toBe(false)
  })

  it('opening the prompt by hand lifts the cancel, and so does submitting a key', async () => {
    const m = await load()
    m.requestAdminKey()
    m.cancelAdminKeyPrompt()

    m.openAdminKeyPrompt()
    expect(m.promptOpen.value).toBe(true)
    const p = m.requestAdminKey()
    m.submitAdminKey('k2')
    await expect(p).resolves.toBe('k2')

    // a submit with no waiting request also lifts it
    m.cancelAdminKeyPrompt()
    m.submitAdminKey('k3')
    const q = m.requestAdminKey()
    expect(m.promptOpen.value).toBe(true)
    m.submitAdminKey('k4')
    await expect(q).resolves.toBe('k4')
  })

  it('rejecting a key forgets it, says so and reopens the prompt', async () => {
    const m = await load()
    m.setAdminKey('wrong')
    m.rejectAdminKey('wrong')
    expect(m.getAdminKey()).toBeUndefined()
    expect(m.promptOpen.value).toBe(true)
    expect(m.promptMessage.value).toBe('That key was not accepted.')
  })

  it('a late refusal of an older key leaves the newer key and the dialog alone', async () => {
    const m = await load()
    m.setAdminKey('newer')
    m.rejectAdminKey('older')
    expect(m.getAdminKey()).toBe('newer')
    expect(m.promptOpen.value).toBe(false)
    expect(m.promptMessage.value).toBe('')
  })

  it('a new key wins when the storage write fails and an older key is still stored', async () => {
    vi.stubGlobal('sessionStorage', failingStorage({ set: true, initial: 'old' }))
    const m = await load()
    expect(m.getAdminKey()).toBe('old') // a reload still reads what was stored
    m.setAdminKey('new')
    expect(m.getAdminKey()).toBe('new')
  })

  it('forget wins when the storage removal fails and the key is still stored', async () => {
    vi.stubGlobal('sessionStorage', failingStorage({ remove: true }))
    const m = await load()
    m.setAdminKey('k')
    m.forgetAdminKey()
    expect(m.getAdminKey()).toBeUndefined()
    expect(m.hasSessionKey.value).toBe(false)
  })

  it('submitting clears the rejection message', async () => {
    const m = await load()
    m.setAdminKey('wrong')
    m.rejectAdminKey('wrong')
    m.submitAdminKey('right')
    expect(m.promptMessage.value).toBe('')
  })

  it('keeps the key exactly as typed, spaces included (the server compares exactly)', async () => {
    const m = await load()
    const p = m.requestAdminKey()
    m.submitAdminKey('  pad key  ')
    await expect(p).resolves.toBe('  pad key  ')
    expect(m.getAdminKey()).toBe('  pad key  ')
  })

  it('ignores a blank key', async () => {
    const m = await load()
    const p = m.requestAdminKey()
    m.submitAdminKey('   ')
    expect(m.promptOpen.value).toBe(true)
    expect(m.hasSessionKey.value).toBe(false)
    m.cancelAdminKeyPrompt()
    await p
  })
})
