import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 't-1' } }),
}))

import ProvisioningAgentsPage from '../ProvisioningAgentsPage.vue'
import ProvisioningTasksPage from '../ProvisioningTasksPage.vue'
import ProvisioningTaskDetailPage from '../ProvisioningTaskDetailPage.vue'
import { useProvisioningStore } from '@/stores/provisioning'

// ginsys/shelly-manager#614: a failed provisioning action shows the store's own error text, once, and
// opens no alert(). The store sets `agentsError` / `tasksError` and rethrows.

const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }

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

let alertSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
  alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})
afterEach(() => {
  vi.restoreAllMocks()
})

function mountWith(page: any, provisioning: Record<string, unknown>) {
  const wrapper = mount(page, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { provisioning } })],
      stubs: { 'router-link': RouterLinkStub },
    },
  })
  return { wrapper, store: useProvisioningStore() }
}

const count = (text: string, needle: string) => text.split(needle).length - 1

describe('provisioning pages: a failed action', () => {
  it('ProvisioningAgentsPage: refresh failure shows once, in the table, and raises no alert', async () => {
    const { wrapper, store } = mountWith(ProvisioningAgentsPage, { agents: [agent], agentsLoading: false, agentsError: '' })
    vi.mocked(store.refreshAgentStatus).mockImplementation(async () => {
      store.agentsError = 'Failed to refresh agent status'
      throw new Error('Failed to refresh agent status')
    })
    await flushPromises()
    await wrapper.find('button.link-button').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to refresh agent status')).toBe(1)
  })

  it('ProvisioningTasksPage: cancel failure shows once, in the table, and raises no alert', async () => {
    const { wrapper, store } = mountWith(ProvisioningTasksPage, { tasks: [task], tasksLoading: false, tasksError: '' })
    vi.mocked(store.cancelProvisioningTask).mockImplementation(async () => {
      store.tasksError = 'Failed to cancel task'
      throw new Error('Failed to cancel task')
    })
    await flushPromises()
    await wrapper.find('button.text-red-600').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to cancel task')).toBe(1)
  })

  it('ProvisioningTaskDetailPage: cancel failure shows once and raises no alert', async () => {
    const { wrapper, store } = mountWith(ProvisioningTaskDetailPage, {
      currentTask: task, tasksLoading: false, tasksError: '',
    })
    vi.mocked(store.cancelProvisioningTask).mockImplementation(async () => {
      store.tasksError = 'Failed to cancel task'
      throw new Error('Failed to cancel task')
    })
    await flushPromises()
    await wrapper.find('.danger-button').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to cancel task')).toBe(1)
  })
})
