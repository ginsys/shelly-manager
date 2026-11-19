import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/notification', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/notification')>()
  return {
    ...actual,
    getChannels: vi.fn(),
    createChannel: vi.fn(),
    updateChannel: vi.fn(),
    deleteChannel: vi.fn(),
    getRules: vi.fn(),
    createRule: vi.fn(),
    deleteRule: vi.fn(),
  }
})
vi.mock('@/api/provisioning', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/provisioning')>()
  return {
    ...actual,
    getTasks: vi.fn(),
    createTask: vi.fn(),
    cancelTask: vi.fn(),
    bulkProvision: vi.fn(),
    getAgents: vi.fn(),
    getAgentStatus: vi.fn(),
  }
})
vi.mock('@/api/drift', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/drift')>()
  return {
    ...actual,
    listDriftSchedules: vi.fn(),
    deleteDriftSchedule: vi.fn(),
    getDriftTrends: vi.fn(),
    resolveDriftTrend: vi.fn(),
    generateDeviceDriftReport: vi.fn(),
  }
})
vi.mock('@/api/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/templates')>()
  return {
    ...actual,
    listTemplates: vi.fn(),
    createTemplate: vi.fn(),
    updateTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
  }
})

import * as notificationApi from '@/api/notification'
import * as provisioningApi from '@/api/provisioning'
import * as driftApi from '@/api/drift'
import * as templatesApi from '@/api/templates'
import { useNotificationsStore } from '../notifications'
import { useProvisioningStore } from '../provisioning'
import { useDriftStore } from '../drift'
import { useTemplatesStore } from '../templates'

// ginsys/shelly-manager#701: a failed action rethrows and leaves the list and the list-load error alone.
// The page that started the action holds the failure; a store field is shared by every page.

const boom = () => new Error('backend said no')

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('notifications store', () => {
  const channel = { id: 'ch-1', name: 'Ops', type: 'email', config: {}, enabled: true } as any
  const rule = { id: 'r-1', name: 'Offline', channelId: 'ch-1', eventTypes: [], filters: {}, enabled: true } as any

  it.each([
    ['addChannel', () => vi.mocked(notificationApi.createChannel), (s: any) => s.addChannel({ name: 'x' })],
    ['modifyChannel', () => vi.mocked(notificationApi.updateChannel), (s: any) => s.modifyChannel('ch-1', { enabled: false })],
    ['removeChannel', () => vi.mocked(notificationApi.deleteChannel), (s: any) => s.removeChannel('ch-1')],
  ])('%s that fails keeps the channels and sets no error', async (_name, api, run) => {
    const store = useNotificationsStore()
    store.channels = [channel]
    api().mockRejectedValue(boom())
    await expect(run(store)).rejects.toThrow('backend said no')
    expect(store.channels).toEqual([channel])
    expect(store.channelsError).toBe('')
    expect(store.channelsLoading).toBe(false)
  })

  it.each([
    ['addRule', () => vi.mocked(notificationApi.createRule), (s: any) => s.addRule({ name: 'x' })],
    ['removeRule', () => vi.mocked(notificationApi.deleteRule), (s: any) => s.removeRule('r-1')],
  ])('%s that fails keeps the rules and sets no error', async (_name, api, run) => {
    const store = useNotificationsStore()
    store.rules = [rule]
    api().mockRejectedValue(boom())
    await expect(run(store)).rejects.toThrow('backend said no')
    expect(store.rules).toEqual([rule])
    expect(store.rulesError).toBe('')
    expect(store.rulesLoading).toBe(false)
  })

  it('an action does not clear the error of the last list load', async () => {
    const store = useNotificationsStore()
    vi.mocked(notificationApi.getChannels).mockRejectedValue(new Error('list is down'))
    await store.fetchChannels()
    expect(store.channelsError).toBe('list is down')
    vi.mocked(notificationApi.deleteChannel).mockRejectedValue(boom())
    await expect(store.removeChannel('ch-1')).rejects.toThrow()
    expect(store.channelsError).toBe('list is down')
  })

  it('a failed list load still sets the error', async () => {
    const store = useNotificationsStore()
    vi.mocked(notificationApi.getRules).mockRejectedValue(new Error('rules are down'))
    await store.fetchRules()
    expect(store.rulesError).toBe('rules are down')
  })
})

describe('provisioning store', () => {
  const task = { id: 't-1', status: 'running' } as any
  const agent = { id: 'a-1', status: 'online' } as any

  it.each([
    ['addTask', () => vi.mocked(provisioningApi.createTask), (s: any) => s.addTask({}) ],
    ['cancelProvisioningTask', () => vi.mocked(provisioningApi.cancelTask), (s: any) => s.cancelProvisioningTask('t-1')],
    ['createBulkTasks', () => vi.mocked(provisioningApi.bulkProvision), (s: any) => s.createBulkTasks({ deviceIds: [1], config: {} })],
  ])('%s that fails keeps the tasks and sets no error', async (_name, api, run) => {
    const store = useProvisioningStore()
    store.tasks = [task]
    api().mockRejectedValue(boom())
    await expect(run(store)).rejects.toThrow('backend said no')
    expect(store.tasks).toEqual([task])
    expect(store.tasksError).toBe('')
    expect(store.tasksLoading).toBe(false)
  })

  it('refreshAgentStatus that fails keeps the agents and sets no error', async () => {
    const store = useProvisioningStore()
    store.agents = [agent]
    vi.mocked(provisioningApi.getAgentStatus).mockRejectedValue(boom())
    await expect(store.refreshAgentStatus('a-1')).rejects.toThrow('backend said no')
    expect(store.agents).toEqual([agent])
    expect(store.agentsError).toBe('')
    expect(store.agentsLoading).toBe(false)
  })

  it('a failed list load still sets the error', async () => {
    const store = useProvisioningStore()
    vi.mocked(provisioningApi.getTasks).mockRejectedValue(new Error('tasks are down'))
    await store.fetchTasks()
    expect(store.tasksError).toBe('tasks are down')
  })
})

describe('drift store', () => {
  it.each([
    ['remove', () => vi.mocked(driftApi.deleteDriftSchedule), (s: any) => s.remove(1)],
    ['resolveTrend', () => vi.mocked(driftApi.resolveDriftTrend), (s: any) => s.resolveTrend(1)],
    ['generateReport', () => vi.mocked(driftApi.generateDeviceDriftReport), (s: any) => s.generateReport(1)],
  ])('%s that fails keeps the list and sets no error', async (_name, api, run) => {
    const store = useDriftStore()
    store.schedules = [{ id: 1 } as any]
    store.trends = [{ id: 1, resolved: false } as any]
    api().mockRejectedValue(boom())
    await expect(run(store)).rejects.toThrow('backend said no')
    expect(store.schedules).toEqual([{ id: 1 }])
    expect(store.trends).toEqual([{ id: 1, resolved: false }])
    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
  })

  it('a failed list load still sets the error', async () => {
    const store = useDriftStore()
    vi.mocked(driftApi.getDriftTrends).mockRejectedValue(new Error('trends are down'))
    await expect(store.fetchTrends()).rejects.toThrow()
    expect(store.error).toBe('trends are down')
  })
})

describe('templates store', () => {
  const template = { id: 1, name: 'T' } as any

  it.each([
    ['create', () => vi.mocked(templatesApi.createTemplate), (s: any) => s.create({ name: 'x', scope: 'global', config: {} })],
    ['update', () => vi.mocked(templatesApi.updateTemplate), (s: any) => s.update(1, { name: 'x' })],
    ['remove', () => vi.mocked(templatesApi.deleteTemplate), (s: any) => s.remove(1)],
  ])('%s that fails keeps the templates and sets no error', async (_name, api, run) => {
    const store = useTemplatesStore()
    store.templates = [template]
    api().mockRejectedValue(boom())
    await expect(run(store)).rejects.toThrow('backend said no')
    expect(store.templates).toEqual([template])
    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
  })

  it('a failed list load still sets the error', async () => {
    const store = useTemplatesStore()
    vi.mocked(templatesApi.listTemplates).mockRejectedValue(new Error('list is down'))
    await expect(store.fetchTemplates()).rejects.toThrow()
    expect(store.error).toBe('list is down')
  })
})
