import { describe, it, expect, vi, beforeEach } from 'vitest'
import api from '../client'

vi.mock('../client', () => {
  const axiosLike = { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn(), interceptors: { request: { use: vi.fn() } } }
  return { default: axiosLike, __esModule: true }
})
const mockedApi = api as unknown as { get: any; post: any; put: any; delete: any }

describe('notification api', () => {
  beforeEach(() => vi.resetAllMocks())

  it('lists channels', async () => {
    const { listChannels } = await import('../notification')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { channels: [{ id: 'ch1', name: 'Ops', type: 'email', config: {}, enabled: true, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }] }, meta: { pagination: { page:1,page_size:20,total_pages:1,has_next:false,has_previous:false } }, timestamp: new Date().toISOString() } })
    const res = await listChannels()
    expect(res.items[0].id).toBe('ch1')
  })

  it('gets a channel', async () => {
    const { getChannel } = await import('../notification')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { id: 'ch1', name: 'Ops', type: 'email', config: {}, enabled: true, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }, timestamp: new Date().toISOString() } })
    const ch = await getChannel('ch1')
    expect(ch.name).toBe('Ops')
  })

  it('creates, updates, deletes channel', async () => {
    const { createChannel, updateChannel, deleteChannel } = await import('../notification')
    mockedApi.post.mockResolvedValueOnce({ data: { success: true, data: { id: 'ch2', name: 'Web', type: 'webhook', config: {}, enabled: true, createdAt: '', updatedAt: '' }, timestamp: new Date().toISOString() } })
    const created = await createChannel({ name: 'Web', type: 'webhook' } as any)
    expect(created.id).toBe('ch2')

    mockedApi.put.mockResolvedValueOnce({ data: { success: true, data: { id: 'ch2', name: 'Web2', type: 'webhook', config: {}, enabled: false, createdAt: '', updatedAt: '' }, timestamp: new Date().toISOString() } })
    const updated = await updateChannel('ch2', { name: 'Web2' })
    expect(updated.name).toBe('Web2')

    mockedApi.delete.mockResolvedValueOnce({ data: { success: true, timestamp: new Date().toISOString() } })
    await expect(deleteChannel('ch2')).resolves.toBeUndefined()
  })

  it('lists rules and creates/deletes rule', async () => {
    const { listRules, createRule, deleteRule } = await import('../notification')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { rules: [{ id: 'r1', name: 'OnExport', channelId: 'ch1', eventTypes: ['export.success'], filters: {}, enabled: true, createdAt: '', updatedAt: '' }] }, timestamp: new Date().toISOString() } })
    const rules = await listRules()
    expect(rules.items[0].id).toBe('r1')

    mockedApi.post.mockResolvedValueOnce({ data: { success: true, data: { id: 'r2', name: 'OnFail', channelId: 'ch1', eventTypes: ['export.failure'], filters: {}, enabled: true, createdAt: '', updatedAt: '' }, timestamp: new Date().toISOString() } })
    const created = await createRule({ name: 'OnFail' })
    expect(created.id).toBe('r2')

    mockedApi.delete.mockResolvedValueOnce({ data: { success: true, timestamp: new Date().toISOString() } })
    await expect(deleteRule('r2')).resolves.toBeUndefined()
  })

  it('lists history with pagination', async () => {
    const { listHistory } = await import('../notification')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { history: [{ id:'h1', channelId:'ch1', ruleId:'r1', eventType:'export.success', status:'sent', sentAt: new Date().toISOString() }] }, meta: { pagination: { page:1,page_size:20,total_pages:1,has_next:false,has_previous:false } }, timestamp: new Date().toISOString() } })
    const res = await listHistory({ page: 1, pageSize: 20 })
    expect(res.items[0].status).toBe('sent')
    expect(res.meta?.pagination?.page).toBe(1)
  })
})

