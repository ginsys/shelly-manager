import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/templates')>()
  return { ...actual, listTemplates: vi.fn(), updateTemplate: vi.fn() }
})

import { listTemplates, updateTemplate } from '@/api/templates'
import { useTemplatesStore } from '../templates'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('templates store loading', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(listTemplates).mockReset()
    vi.mocked(updateTemplate).mockReset()
  })

  // Each request releases only its own share: the first to finish must not end the busy state of
  // the others, or a page that reads `loading` shows its actions while a save is still pending.
  it('stays loading until every overlapping request has finished', async () => {
    const first = deferred<any[]>()
    const second = deferred<any[]>()
    vi.mocked(listTemplates).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const store = useTemplatesStore()
    const a = store.fetchTemplates()
    const b = store.fetchTemplates()
    expect(store.loading).toBe(true)
    first.resolve([])
    await a
    expect(store.loading).toBe(true)
    second.resolve([])
    await b
    expect(store.loading).toBe(false)
  })

  it('a list request that ends does not end the loading of a pending update', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    vi.mocked(listTemplates).mockResolvedValueOnce([])
    const store = useTemplatesStore()
    const saving = store.update(1, { name: 'x' })
    await store.fetchTemplates()
    expect(store.loading).toBe(true)
    save.resolve({ template: { id: 1 } })
    await saving
    expect(store.loading).toBe(false)
  })

  it('a failed request still releases its share', async () => {
    vi.mocked(listTemplates).mockRejectedValueOnce(new Error('down'))
    const store = useTemplatesStore()
    await expect(store.fetchTemplates()).rejects.toThrow('down')
    expect(store.loading).toBe(false)
  })
})
