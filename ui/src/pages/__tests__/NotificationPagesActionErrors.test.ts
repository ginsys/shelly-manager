import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'ch-1' } }),
}))

import NotificationChannelsPage from '../NotificationChannelsPage.vue'
import NotificationRulesPage from '../NotificationRulesPage.vue'
import NotificationChannelDetailPage from '../NotificationChannelDetailPage.vue'
import { useNotificationsStore } from '@/stores/notifications'
import { dismissNotice, notice } from '@/composables/useNotice'

// ginsys/shelly-manager#614: a failed notification action shows the store's own error text, once, where
// the user acted, and opens no alert(). The store sets `channelsError` / `rulesError` and rethrows.

const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }

const channel = {
  id: 'ch-1', name: 'Ops mail', type: 'email', config: {}, enabled: true,
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}
const rule = {
  id: 'r-1', name: 'Offline', channelId: 'ch-1', eventTypes: ['device.offline'], filters: {}, enabled: true,
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}

// The shared test setup answers document.createElement('a') with a plain object, which Vue cannot render.
const realCreateElement = Document.prototype.createElement.bind(document)

let alertSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
  alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})
afterEach(() => {
  vi.restoreAllMocks()
})

function mountWith(page: any, notifications: Record<string, unknown>) {
  const wrapper = mount(page, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { notifications } })],
      stubs: { 'router-link': RouterLinkStub },
    },
  })
  return { wrapper, store: useNotificationsStore() }
}

const count = (text: string, needle: string) => text.split(needle).length - 1

describe('NotificationChannelsPage', () => {
  it('shows the create failure inside the open modal and raises no alert', async () => {
    const { wrapper, store } = mountWith(NotificationChannelsPage, { channels: [channel], channelsError: '' })
    vi.mocked(store.addChannel).mockImplementation(async () => {
      store.channelsError = 'Failed to create channel'
      throw new Error('Failed to create channel')
    })
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    const inModal = wrapper.find('.modal [data-testid="channel-create-error"]')
    expect(inModal.exists()).toBe(true)
    expect(inModal.text()).toBe('Failed to create channel')
  })

  it('shows no error text in the modal when nothing failed', async () => {
    const { wrapper } = mountWith(NotificationChannelsPage, { channels: [channel], channelsError: '' })
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    expect(wrapper.find('[data-testid="channel-create-error"]').exists()).toBe(false)
  })

  it('does not show an earlier failure of another action in a newly opened modal', async () => {
    const { wrapper } = mountWith(NotificationChannelsPage, { channels: [channel], channelsError: 'Failed to delete channel' })
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(true)
    expect(wrapper.find('[data-testid="channel-create-error"]').exists()).toBe(false)
  })

  it('shows the delete failure once, in the table, and raises no alert', async () => {
    const { wrapper, store } = mountWith(NotificationChannelsPage, { channels: [channel], channelsError: '' })
    vi.mocked(store.removeChannel).mockImplementation(async () => {
      store.channelsError = 'Failed to delete channel'
      throw new Error('Failed to delete channel')
    })
    await flushPromises()
    await wrapper.find('button.text-red-600').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to delete channel')).toBe(1)
  })
})

describe('NotificationRulesPage', () => {
  it('shows the create failure inside the open modal and raises no alert', async () => {
    const { wrapper, store } = mountWith(NotificationRulesPage, { rules: [rule], channels: [channel], rulesError: '' })
    vi.mocked(store.addRule).mockImplementation(async () => {
      store.rulesError = 'Failed to create rule'
      throw new Error('Failed to create rule')
    })
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    const inModal = wrapper.find('.modal [data-testid="rule-create-error"]')
    expect(inModal.exists()).toBe(true)
    expect(inModal.text()).toBe('Failed to create rule')
  })

  it('does not show an earlier failure of another action in a newly opened modal', async () => {
    const { wrapper } = mountWith(NotificationRulesPage, { rules: [rule], channels: [channel], rulesError: 'Failed to delete rule' })
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(true)
    expect(wrapper.find('[data-testid="rule-create-error"]').exists()).toBe(false)
  })

  it('shows the delete failure once, in the table, and raises no alert', async () => {
    const { wrapper, store } = mountWith(NotificationRulesPage, { rules: [rule], channels: [channel], rulesError: '' })
    vi.mocked(store.removeRule).mockImplementation(async () => {
      store.rulesError = 'Failed to delete rule'
      throw new Error('Failed to delete rule')
    })
    await flushPromises()
    await wrapper.find('button.text-red-600').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to delete rule')).toBe(1)
  })
})

describe('NotificationChannelDetailPage', () => {
  it('shows the update failure once and raises no alert', async () => {
    const { wrapper, store } = mountWith(NotificationChannelDetailPage, {
      currentChannel: channel, channelsLoading: false, channelsError: '',
    })
    vi.mocked(store.modifyChannel).mockImplementation(async () => {
      store.channelsError = 'Failed to update channel'
      throw new Error('Failed to update channel')
    })
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to update channel')).toBe(1)
  })

  // Review round 2: the channel can be deleted, and the page left, while the update is pending.
  it('reports the failure although the channel was deleted and the page left meanwhile', async () => {
    dismissNotice()
    const { wrapper, store } = mountWith(NotificationChannelDetailPage, {
      currentChannel: channel, channelsLoading: false, channelsError: '',
    })
    const fail = deferredAction(store, 'modifyChannel')
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    store.currentChannel = null
    wrapper.unmount()
    fail(new Error('Failed to update channel'))
    await flushPromises()
    expect(notice.value?.message).toContain('Failed to update channel')
  })
})

// Races between actions and dialogs (review round 2).
function deferredAction(store: any, name: string) {
  let fail: (e: Error) => void = () => {}
  vi.mocked(store[name]).mockImplementation(() => new Promise((_resolve, reject) => { fail = reject }) as any)
  return (e: Error) => fail(e)
}

describe.each([
  ['NotificationChannelsPage', NotificationChannelsPage, 'removeChannel', 'addChannel',
    { channels: [channel], channelsError: '' }, 'channel-create-error'],
  ['NotificationRulesPage', NotificationRulesPage, 'removeRule', 'addRule',
    { rules: [rule], channels: [channel], rulesError: '' }, 'rule-create-error'],
] as const)('%s dialog races', (_name, page, removeName, addName, state, testId) => {
  it('shows a delete that fails behind the open create dialog in the app notice', async () => {
    dismissNotice()
    const { wrapper, store } = mountWith(page, state)
    const fail = deferredAction(store, removeName)
    await flushPromises()
    await wrapper.find('button.text-red-600').trigger('click')
    await wrapper.find('.primary-button').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(true)
    fail(new Error('Failed to delete'))
    await flushPromises()
    expect(notice.value?.message).toContain('Failed to delete')
    wrapper.unmount()
  })

  it('keeps the failure of a cancelled submission out of the dialog that was opened again', async () => {
    const { wrapper, store } = mountWith(page, state)
    const fail = deferredAction(store, addName)
    await flushPromises()
    await wrapper.find('.primary-button').trigger('click')
    await wrapper.find('form').trigger('submit')
    await wrapper.find('.secondary-button').trigger('click')
    await wrapper.find('.primary-button').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(true)
    fail(new Error('Failed to create'))
    await flushPromises()
    expect(wrapper.find(`[data-testid="${testId}"]`).exists()).toBe(false)
    wrapper.unmount()
  })
})
