import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { setActivePinia, createPinia } from 'pinia'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
}))
vi.mock('@/api/notification', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/notification')>()
  return { ...actual, testChannel: vi.fn(), getChannel: vi.fn(), setChannelEnabled: vi.fn(), getHistory: vi.fn() }
})

import * as notificationApi from '@/api/notification'
import NotificationRulesPage from '../NotificationRulesPage.vue'
import NotificationHistoryPage from '../NotificationHistoryPage.vue'
import NotificationChannelDetailPage from '../NotificationChannelDetailPage.vue'
import { useNotificationsStore } from '@/stores/notifications'

// ginsys/shelly-manager#250: the pages show and send what the server holds. The page models below are what
// src/api/notification.ts produces from the server's objects (its own test pins that mapping).

const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }
const realCreateElement = Document.prototype.createElement.bind(document)

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
})
afterEach(() => {
  vi.restoreAllMocks()
})

const channel = {
  id: '1', name: 'Ops mail', type: 'email', config: {}, enabled: true, description: '',
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}
const rule = {
  id: '7', name: 'Critical Drift', description: '', channelId: '1', enabled: true,
  alertLevel: 'critical', minSeverity: 'warning', categories: ['security', 'device'],
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}

function mountWith(page: any, notifications: Record<string, unknown>) {
  const wrapper = mount(page, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { notifications } })],
      stubs: { 'router-link': RouterLinkStub },
    },
  })
  return { wrapper, store: useNotificationsStore() }
}

describe('NotificationRulesPage', () => {
  it('lists a rule with its alert level, minimum severity and categories', async () => {
    const { wrapper } = mountWith(NotificationRulesPage, { rules: [rule], channels: [channel] })
    await flushPromises()
    const row = wrapper.find('tbody tr').text()
    expect(row).toContain('Critical Drift')
    expect(row).toContain('Ops mail')
    expect(row).toContain('critical')
    expect(row).toContain('warning')
    expect(row).toContain('security, device')
  })

  it('shows "all" for a rule without categories and a dash for no minimum severity', async () => {
    const { wrapper } = mountWith(NotificationRulesPage, {
      rules: [{ ...rule, categories: [], minSeverity: '' }], channels: [channel],
    })
    await flushPromises()
    const cells = wrapper.findAll('tbody tr td').map((c) => c.text())
    expect(cells).toContain('all')
    expect(cells).toContain('-')
  })

  it('creates a rule with the alert level, severity and categories the form holds', async () => {
    const { wrapper, store } = mountWith(NotificationRulesPage, { rules: [], channels: [channel] })
    vi.mocked(store.addRule).mockResolvedValue(rule as any)
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')

    const form = wrapper.find('form')
    const selects = form.findAll('select')
    await form.find('input.form-input').setValue('Night alerts')
    await selects[0].setValue('1') // channel
    await selects[1].setValue('warning') // alert level
    await selects[2].setValue('critical') // minimum severity
    await form.find('input[placeholder="security,device"]').setValue(' security , ,device ')
    await form.trigger('submit')
    await flushPromises()

    expect(store.addRule).toHaveBeenCalledWith({
      name: 'Night alerts', channelId: '1', alertLevel: 'warning', minSeverity: 'critical',
      enabled: true, categories: ['security', 'device'],
    })
  })

  it('creates a rule with the defaults: every alert level, no minimum, no categories', async () => {
    const { wrapper, store } = mountWith(NotificationRulesPage, { rules: [], channels: [channel] })
    vi.mocked(store.addRule).mockResolvedValue(rule as any)
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    const form = wrapper.find('form')
    await form.find('input.form-input').setValue('Everything')
    await form.findAll('select')[0].setValue('1')
    await form.trigger('submit')
    await flushPromises()

    expect(store.addRule).toHaveBeenCalledWith({
      name: 'Everything', channelId: '1', alertLevel: 'all', minSeverity: '', enabled: true, categories: [],
    })
  })
})

describe('NotificationHistoryPage', () => {
  const row = (over: Record<string, unknown> = {}) => ({
    id: '10', channelId: '1', ruleId: '7', triggerType: 'drift_detected', status: 'sent',
    sentAt: '2026-11-01T10:00:10Z', createdAt: '2026-11-01T10:00:00Z', ...over,
  })

  it('shows the trigger, and the creation time of a row that was never sent', async () => {
    const { wrapper } = mountWith(NotificationHistoryPage, {
      history: [row(), row({ id: '11', status: 'pending', sentAt: undefined, triggerType: 'test' })],
      channels: [channel], rules: [rule], historyTotal: 2,
    })
    await flushPromises()
    expect(wrapper.text()).toContain('drift_detected')
    expect(wrapper.text()).not.toContain('Invalid Date')
    const dates = wrapper.findAll('tbody tr').map((r) => r.findAll('td')[4].text())
    expect(dates[0]).toBe(new Date('2026-11-01T10:00:10Z').toLocaleString())
    expect(dates[1]).toBe(new Date('2026-11-01T10:00:00Z').toLocaleString())
  })

  it('reads the pager from the server: page of pages, and the buttons follow has_previous and has_next', async () => {
    const { wrapper } = mountWith(NotificationHistoryPage, {
      history: [row()], channels: [channel], rules: [rule], historyPage: 2, historyTotal: 120,
      historyPagination: { page: 2, page_size: 50, total_pages: 3, has_next: true, has_previous: true },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('Page 2 of 3')
    const [prev, next] = wrapper.findAll('button.secondary-button')
    expect(prev.attributes('disabled')).toBeUndefined()
    expect(next.attributes('disabled')).toBeUndefined()
  })

  it('disables Next on the last page and Previous on the first', async () => {
    const last = mountWith(NotificationHistoryPage, {
      history: [row()], channels: [], rules: [], historyPage: 3, historyTotal: 120,
      historyPagination: { page: 3, page_size: 50, total_pages: 3, has_next: false, has_previous: true },
    })
    await flushPromises()
    const [, next] = last.wrapper.findAll('button.secondary-button')
    expect(next.attributes('disabled')).toBeDefined()

    const first = mountWith(NotificationHistoryPage, {
      history: [row()], channels: [], rules: [], historyPage: 1, historyTotal: 120,
      historyPagination: { page: 1, page_size: 50, total_pages: 3, has_next: true, has_previous: false },
    })
    await flushPromises()
    expect(first.wrapper.findAll('button.secondary-button')[0].attributes('disabled')).toBeDefined()
  })

  it('shows the total of all rows and says the status counts are for this page', async () => {
    const { wrapper } = mountWith(NotificationHistoryPage, {
      history: [row(), row({ id: '12', status: 'failed' }), row({ id: '13', status: 'retry' })],
      channels: [], rules: [], historyTotal: 120,
    })
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('Total: 120')
    expect(text).toContain('Sent on this page: 1')
    expect(text).toContain('Failed on this page: 1')
    expect(text).toContain('Pending on this page: 1')
  })
})

describe('NotificationChannelDetailPage', () => {
  it('Send Test calls the server and shows its message beside the button', async () => {
    vi.mocked(notificationApi.testChannel).mockResolvedValue('Test notification sent successfully')
    const { wrapper } = mountWith(NotificationChannelDetailPage, {
      currentChannel: channel, channelsLoading: false, channelsError: '',
    })
    await flushPromises()
    await wrapper.find('[data-testid="channel-test-button"]').trigger('click')
    await flushPromises()
    expect(notificationApi.testChannel).toHaveBeenCalledWith('1')
    expect(wrapper.find('[data-testid="channel-test-result"]').text()).toBe('Test notification sent successfully')
  })

  it('shows a failed test once in the error panel and no success line', async () => {
    vi.mocked(notificationApi.testChannel).mockRejectedValue(new Error('webhook request failed'))
    const { wrapper } = mountWith(NotificationChannelDetailPage, {
      currentChannel: channel, channelsLoading: false, channelsError: '',
    })
    await flushPromises()
    await wrapper.find('[data-testid="channel-test-button"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="channel-action-error"]').exists()).toBe(true)
    expect(wrapper.text().split('webhook request failed').length - 1).toBe(1)
    expect(wrapper.find('[data-testid="channel-test-result"]').exists()).toBe(false)
  })

  it('the toggle asks the store to flip enabled', async () => {
    const { wrapper, store } = mountWith(NotificationChannelDetailPage, {
      currentChannel: channel, channelsLoading: false, channelsError: '',
    })
    vi.mocked(store.setChannelEnabled).mockResolvedValue(undefined as any)
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    expect(store.setChannelEnabled).toHaveBeenCalledWith('1', false)
  })
})

describe('notifications store, against the api layer', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('setChannelEnabled sends the toggle, then replaces the stored copy with a fresh read', async () => {
    const store = useNotificationsStore()
    store.channels = [channel as any]
    store.currentChannel = channel as any
    vi.mocked(notificationApi.setChannelEnabled).mockResolvedValue(undefined)
    vi.mocked(notificationApi.getChannel).mockResolvedValue({ ...channel, enabled: false } as any)

    await store.setChannelEnabled('1', false)

    expect(notificationApi.setChannelEnabled).toHaveBeenCalledWith('1', false)
    expect(store.channels[0].enabled).toBe(false)
    expect(store.currentChannel?.enabled).toBe(false)
  })

  it('setChannelEnabled that fails keeps the copy and does not read again', async () => {
    const store = useNotificationsStore()
    store.channels = [channel as any]
    vi.mocked(notificationApi.setChannelEnabled).mockRejectedValue(new Error('no'))

    await expect(store.setChannelEnabled('1', false)).rejects.toThrow('no')

    expect(store.channels[0].enabled).toBe(true)
    expect(notificationApi.getChannel).not.toHaveBeenCalled()
  })

  it('fetchHistory keeps the server\'s page description and the count of all rows', async () => {
    const pagination = { page: 2, page_size: 25, total_pages: 4, has_next: true, has_previous: true }
    vi.mocked(notificationApi.getHistory).mockResolvedValue({ items: [], pagination, total: 90 })
    const store = useNotificationsStore()

    await store.fetchHistory({ page: 2, limit: 25 })

    expect(notificationApi.getHistory).toHaveBeenCalledWith({ page: 2, limit: 25 })
    expect(store.historyPagination).toEqual(pagination)
    expect(store.historyTotal).toBe(90)
    expect(store.historyPage).toBe(2)
  })
})
