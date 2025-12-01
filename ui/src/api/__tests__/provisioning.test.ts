import { describe, it, expect, vi, beforeEach } from 'vitest'
import api from '../client'

vi.mock('../client', () => {
  const axiosLike = { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn(), interceptors: { request: { use: vi.fn() } } }
  return { default: axiosLike, __esModule: true }
})
const mockedApi = api as unknown as { get: any; post: any }

describe('provisioning api', () => {
  beforeEach(() => vi.resetAllMocks())

  it('lists tasks with pagination meta', async () => {
    const { listTasks } = await import('../provisioning')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { tasks: [{ id:'t1', deviceId:'d1', deviceName:'dev1', status:'pending', taskType:'configure', config:{}, createdAt:'', updatedAt:'' }] }, meta: { pagination: { page:1,page_size:20,total_pages:1,has_next:false,has_previous:false } }, timestamp: new Date().toISOString() } })
    const res = await listTasks({ page: 1 })
    expect(res.items[0].id).toBe('t1')
    expect(res.meta?.pagination?.page).toBe(1)
  })

  it('gets a task', async () => {
    const { getTask } = await import('../provisioning')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { id:'t1', deviceId:'d1', deviceName:'dev1', status:'running', taskType:'configure', config:{}, createdAt:'', updatedAt:'' }, timestamp: new Date().toISOString() } })
    const task = await getTask('t1')
    expect(task.status).toBe('running')
  })

  it('creates and cancels a task', async () => {
    const { createTask, cancelTask } = await import('../provisioning')
    mockedApi.post.mockResolvedValueOnce({ data: { success: true, data: { id:'t2', deviceId:'d2', deviceName:'dev2', status:'pending', taskType:'update', config:{}, createdAt:'', updatedAt:'' }, timestamp: new Date().toISOString() } })
    const created = await createTask({ deviceId:'d2', taskType:'update' } as any)
    expect(created.id).toBe('t2')

    mockedApi.post.mockResolvedValueOnce({ data: { success: true, timestamp: new Date().toISOString() } })
    await expect(cancelTask('t2')).resolves.toBeUndefined()
  })

  it('bulk provisions devices', async () => {
    const { bulkProvision } = await import('../provisioning')
    mockedApi.post.mockResolvedValueOnce({ data: { success: true, data: { accepted: 3 }, timestamp: new Date().toISOString() } })
    const res = await bulkProvision(['d1','d2','d3'], { wifi_ssid:'demo' })
    expect(res?.accepted).toBe(3)
  })

  it('lists agents and gets status', async () => {
    const { listAgents, getAgentStatus } = await import('../provisioning')
    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { agents: [{ id:'a1', name:'agent-1', status:'online', version:'1.0.0', capabilities:['wifi'], lastSeen:'' }] }, timestamp: new Date().toISOString() } })
    const agents = await listAgents()
    expect(agents.items[0].status).toBe('online')

    mockedApi.get.mockResolvedValueOnce({ data: { success: true, data: { status:'online', lastSeen:'' }, timestamp: new Date().toISOString() } })
    const st = await getAgentStatus('a1')
    expect(st.status).toBe('online')
  })
})

