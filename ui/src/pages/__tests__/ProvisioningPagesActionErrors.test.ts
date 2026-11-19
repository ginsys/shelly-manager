import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'

const route = vi.hoisted(() => ({ params: { id: 't-1' } as { id: string } }))
vi.mock('vue-router', async () => {
  const { reactive } = await import('vue')
  const reactiveRoute = reactive(route)
  return { useRoute: () => reactiveRoute }
})

import { useRoute } from 'vue-router'
import ProvisioningAgentsPage from '../ProvisioningAgentsPage.vue'
import ProvisioningTasksPage from '../ProvisioningTasksPage.vue'
import ProvisioningTaskDetailPage from '../ProvisioningTaskDetailPage.vue'
import { useProvisioningStore } from '@/stores/provisioning'

// ginsys/shelly-manager#614 and #701: a failed provisioning action shows the error text, once, and opens
// no alert(). The store only rethrows: `agentsError` / `tasksError` stay the failure of the list load, and
// the page holds the failure of its own action. The rows stay on the page.

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
  it('ProvisioningAgentsPage: refresh failure shows once, above the table, keeps the rows, raises no alert', async () => {
    const { wrapper, store } = mountWith(ProvisioningAgentsPage, { agents: [agent], agentsLoading: false, agentsError: '' })
    vi.mocked(store.refreshAgentStatus).mockRejectedValue(new Error('Failed to refresh agent status'))
    await flushPromises()
    await wrapper.find('button.link-button').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to refresh agent status')).toBe(1)
    expect(wrapper.find('[data-testid="agent-action-error"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Agent one')
    expect(store.agentsError).toBe('')
  })

  it('ProvisioningTasksPage: cancel failure shows once, above the table, keeps the rows, raises no alert', async () => {
    const { wrapper, store } = mountWith(ProvisioningTasksPage, { tasks: [task], tasksLoading: false, tasksError: '' })
    vi.mocked(store.cancelProvisioningTask).mockRejectedValue(new Error('Failed to cancel task'))
    await flushPromises()
    await wrapper.find('button.text-red-600').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to cancel task')).toBe(1)
    expect(wrapper.find('[data-testid="task-action-error"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('plug-1')
    expect(store.tasksError).toBe('')
  })

  it('ProvisioningTaskDetailPage: a cancel that succeeds after the route moved refreshes the task now shown', async () => {
    const liveRoute = useRoute() as unknown as { params: { id: string } }
    liveRoute.params.id = 't-1'
    const { wrapper, store } = mountWith(ProvisioningTaskDetailPage, {
      currentTask: task, tasksLoading: false, tasksError: '',
    })
    let finish: () => void = () => {}
    vi.mocked(store.cancelProvisioningTask).mockImplementation(() => new Promise<void>((res) => { finish = res }))
    await flushPromises()
    vi.mocked(store.fetchTask).mockClear()
    await wrapper.find('.danger-button').trigger('click')
    liveRoute.params.id = 't-2'
    finish()
    await flushPromises()
    expect(vi.mocked(store.fetchTask).mock.calls.map((c) => c[0])).toEqual(['t-2'])
    liveRoute.params.id = 't-1'
  })

  it('ProvisioningTaskDetailPage: cancel failure shows once, keeps the details, raises no alert', async () => {
    const { wrapper, store } = mountWith(ProvisioningTaskDetailPage, {
      currentTask: task, tasksLoading: false, tasksError: '',
    })
    vi.mocked(store.cancelProvisioningTask).mockRejectedValue(new Error('Failed to cancel task'))
    await flushPromises()
    await wrapper.find('.danger-button').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(count(wrapper.text(), 'Failed to cancel task')).toBe(1)
    expect(wrapper.find('[data-testid="task-action-error"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('plug-1')
  })
})
