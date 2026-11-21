import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'x-1' } }),
}))

import DriftReportsPage from '../DriftReportsPage.vue'
import DriftSchedulesPage from '../DriftSchedulesPage.vue'
import NotificationChannelsPage from '../NotificationChannelsPage.vue'
import NotificationRulesPage from '../NotificationRulesPage.vue'
import NotificationChannelDetailPage from '../NotificationChannelDetailPage.vue'
import ProvisioningAgentsPage from '../ProvisioningAgentsPage.vue'
import ProvisioningTasksPage from '../ProvisioningTasksPage.vue'
import ProvisioningTaskDetailPage from '../ProvisioningTaskDetailPage.vue'
import { useDriftStore } from '@/stores/drift'
import { useNotificationsStore } from '@/stores/notifications'
import { useProvisioningStore } from '@/stores/provisioning'
import { dismissNotice, notice } from '@/composables/useNotice'

// ginsys/shelly-manager#614: these pages show a store action's failure where the store keeps it (the
// table, a card, a dialog). When the user has left the page by the time the action fails, that place no
// longer exists, so the failure shows in the app notice instead of nowhere.

const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }

const trend = {
  id: 7, device_id: 3, path: 'wifi.ssid', category: 'config', severity: 'high', occurrences: 2,
  first_seen: '2026-11-01T00:00:00Z', last_seen: '2026-11-02T00:00:00Z', resolved: false,
}
const schedule = {
  id: 1, name: 'Nightly drift', description: '', enabled: true, cron_spec: '0 0 * * *',
  device_ids: [1], device_filter: {}, last_run: null, next_run: null, run_count: 0, revision: 0,
  created_at: '2026-07-01T00:00:00Z', updated_at: '2026-07-01T00:00:00Z',
}
const channel = {
  id: 'ch-1', name: 'Ops mail', type: 'email', config: {}, enabled: true, description: '',
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}
const rule = {
  id: 'r-1', name: 'Offline', description: '', channelId: 'ch-1', enabled: true,
  alertLevel: 'critical', minSeverity: '', categories: ['device'],
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}
const agent = {
  id: 'a-1', name: 'Agent one', status: 'online', version: '1.0.0', capabilities: ['wifi'],
  lastSeen: '2026-11-19T10:00:00Z',
}
const task = {
  id: 't-1', deviceName: 'plug-1', taskType: 'provision', status: 'running',
  createdAt: '2026-11-19T10:00:00Z', updatedAt: '2026-11-19T10:01:00Z',
}

// The shared test setup answers document.createElement('a') with a plain object, which Vue cannot render.
const realCreateElement = Document.prototype.createElement.bind(document)

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
  vi.spyOn(window, 'alert').mockImplementation(() => {})
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  dismissNotice()
})
afterEach(() => {
  vi.restoreAllMocks()
})

interface Case {
  name: string
  page: any
  initialState: Record<string, Record<string, unknown>>
  // the store action that fails, and how the user starts it
  action: (pinia: unknown) => any
  start: (wrapper: any) => Promise<void>
  message: string
}

const cases: Case[] = [
  {
    name: 'DriftReportsPage resolve',
    page: DriftReportsPage,
    initialState: { drift: { trends: [trend], loading: false, error: null } },
    action: () => useDriftStore().resolveTrend,
    start: async (w) => {
      await w.find('.btn-small').trigger('click')
      await w.find('.form-actions .btn.primary').trigger('click')
    },
    message: 'Failed to resolve drift item',
  },
  {
    name: 'DriftSchedulesPage delete',
    page: DriftSchedulesPage,
    initialState: { drift: { schedules: [schedule], loading: false, error: null } },
    action: () => useDriftStore().remove,
    start: (w) => w.find('button.btn-icon.danger').trigger('click'),
    message: 'Failed to delete drift schedule',
  },
  {
    name: 'NotificationChannelsPage create',
    page: NotificationChannelsPage,
    initialState: { notifications: { channels: [channel], channelsError: '' } },
    action: () => useNotificationsStore().addChannel,
    start: async (w) => {
      await w.find('.primary-button').trigger('click')
      await w.find('form').trigger('submit')
    },
    message: 'Failed to create channel',
  },
  {
    name: 'NotificationChannelsPage delete',
    page: NotificationChannelsPage,
    initialState: { notifications: { channels: [channel], channelsError: '' } },
    action: () => useNotificationsStore().removeChannel,
    start: (w) => w.find('button.text-red-600').trigger('click'),
    message: 'Failed to delete channel',
  },
  {
    name: 'NotificationRulesPage create',
    page: NotificationRulesPage,
    initialState: { notifications: { rules: [rule], channels: [channel], rulesError: '' } },
    action: () => useNotificationsStore().addRule,
    start: async (w) => {
      await w.find('.primary-button').trigger('click')
      await w.find('form').trigger('submit')
    },
    message: 'Failed to create rule',
  },
  {
    name: 'NotificationRulesPage delete',
    page: NotificationRulesPage,
    initialState: { notifications: { rules: [rule], channels: [channel], rulesError: '' } },
    action: () => useNotificationsStore().removeRule,
    start: (w) => w.find('button.text-red-600').trigger('click'),
    message: 'Failed to delete rule',
  },
  {
    name: 'NotificationChannelDetailPage update',
    page: NotificationChannelDetailPage,
    initialState: { notifications: { currentChannel: channel, channelsLoading: false, channelsError: '' } },
    action: () => useNotificationsStore().setChannelEnabled,
    start: (w) => w.find('.primary-button').trigger('click'),
    message: 'Failed to update channel',
  },
  {
    name: 'ProvisioningAgentsPage refresh',
    page: ProvisioningAgentsPage,
    initialState: { provisioning: { agents: [agent], agentsLoading: false, agentsError: '' } },
    action: () => useProvisioningStore().refreshAgentStatus,
    start: (w) => w.find('button.link-button').trigger('click'),
    message: 'Failed to refresh agent status',
  },
  {
    name: 'ProvisioningTasksPage cancel',
    page: ProvisioningTasksPage,
    initialState: { provisioning: { tasks: [task], tasksLoading: false, tasksError: '' } },
    action: () => useProvisioningStore().cancelProvisioningTask,
    start: (w) => w.find('button.text-red-600').trigger('click'),
    message: 'Failed to cancel task',
  },
  {
    name: 'ProvisioningTaskDetailPage cancel',
    page: ProvisioningTaskDetailPage,
    initialState: { provisioning: { currentTask: task, tasksLoading: false, tasksError: '' } },
    action: () => useProvisioningStore().cancelProvisioningTask,
    start: (w) => w.find('.danger-button').trigger('click'),
    message: 'Failed to cancel task',
  },
]

describe('a store action that fails after the page was left', () => {
  it.each(cases)('$name shows the failure in the app notice', async (c) => {
    const pinia = createTestingPinia({ createSpy: vi.fn, initialState: c.initialState })
    const wrapper = mount(c.page, { global: { plugins: [pinia], stubs: { 'router-link': RouterLinkStub } } })
    let fail: (e: Error) => void = () => {}
    vi.mocked(c.action(pinia)).mockImplementation(
      () => new Promise((_resolve, reject) => { fail = reject }) as any,
    )
    await flushPromises()
    await c.start(wrapper)

    wrapper.unmount()
    fail(new Error(c.message))
    await flushPromises()

    expect(notice.value?.message).toContain(c.message)
  })

  it('stays out of the app notice while the page is still open', async () => {
    const c = cases[3]
    const pinia = createTestingPinia({ createSpy: vi.fn, initialState: c.initialState })
    const wrapper = mount(c.page, { global: { plugins: [pinia], stubs: { 'router-link': RouterLinkStub } } })
    vi.mocked(c.action(pinia)).mockRejectedValue(new Error(c.message))
    await flushPromises()
    await c.start(wrapper)
    await flushPromises()

    expect(notice.value).toBeNull()
    wrapper.unmount()
  })
})
