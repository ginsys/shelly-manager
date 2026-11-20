import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
}))

import NotificationChannelsPage from '../NotificationChannelsPage.vue'
import { useNotificationsStore } from '@/stores/notifications'

// ginsys/shelly-manager#744: the create form sent `config: {}`, which the server refuses for every type. The
// configs below have the fields of the Go structs in internal/notification/models.go (EmailConfig,
// WebhookConfig, SlackConfig). No request is made: the store action is a spy.

const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }
const realCreateElement = Document.prototype.createElement.bind(document)

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
})
afterEach(() => {
  vi.restoreAllMocks()
})

const created = {
  id: '1', name: 'x', type: 'email', config: {}, enabled: true, description: '',
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}

async function openForm() {
  const wrapper = mount(NotificationChannelsPage, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { notifications: { channels: [], channelsError: '' } } })],
      stubs: { 'router-link': RouterLinkStub },
    },
  })
  const store = useNotificationsStore()
  vi.mocked(store.addChannel).mockResolvedValue(created as any)
  await flushPromises()
  await wrapper.find('.primary-button').trigger('click')
  return { wrapper, store }
}

const field = (wrapper: any, id: string) => wrapper.find(`[data-testid="${id}"]`)

async function submit(wrapper: any) {
  await wrapper.find('form').trigger('submit')
  await flushPromises()
}

describe('NotificationChannelsPage create form', () => {
  it('creates an email channel with its recipients and subject', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Ops mail')
    await field(wrapper, 'channel-recipients').setValue('ops@example.com, oncall@example.com\n sec@example.com \n')
    await field(wrapper, 'channel-subject').setValue('Shelly alert')
    await submit(wrapper)

    expect(store.addChannel).toHaveBeenCalledWith({
      name: 'Ops mail', type: 'email', enabled: true,
      config: { recipients: ['ops@example.com', 'oncall@example.com', 'sec@example.com'], subject: 'Shelly alert' },
    })
  })

  it('creates an email channel without a subject and sends no empty key', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Ops mail')
    await field(wrapper, 'channel-recipients').setValue('ops@example.com')
    await submit(wrapper)

    const sent = vi.mocked(store.addChannel).mock.calls[0][0]
    expect(sent.config).toEqual({ recipients: ['ops@example.com'] })
  })

  it('creates a webhook channel with url, method, headers and secret', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Hook')
    await field(wrapper, 'channel-type').setValue('webhook')
    await field(wrapper, 'channel-webhook-url').setValue(' https://hooks.example.com/shelly ')
    await field(wrapper, 'channel-webhook-method').setValue('PUT')
    await field(wrapper, 'channel-header-name').setValue('X-Team')
    await field(wrapper, 'channel-header-value').setValue('ops')
    await field(wrapper, 'channel-add-header').trigger('click')
    await wrapper.findAll('[data-testid="channel-header-name"]')[1].setValue('Authorization')
    await wrapper.findAll('[data-testid="channel-header-value"]')[1].setValue('Bearer abc')
    await field(wrapper, 'channel-add-header').trigger('click') // a third row, left blank
    await field(wrapper, 'channel-webhook-secret').setValue('abc 123')
    await submit(wrapper)

    expect(store.addChannel).toHaveBeenCalledWith({
      name: 'Hook', type: 'webhook', enabled: true,
      config: {
        url: 'https://hooks.example.com/shelly', method: 'PUT',
        headers: { 'X-Team': 'ops', Authorization: 'Bearer abc' }, secret: 'abc 123',
      },
    })
  })

  it('creates a slack channel with its webhook url, channel and username', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Chat')
    await field(wrapper, 'channel-type').setValue('slack')
    await field(wrapper, 'channel-slack-url').setValue('https://hooks.slack.com/services/T0/B0/xyz')
    await field(wrapper, 'channel-slack-channel').setValue('#ops')
    await field(wrapper, 'channel-slack-username').setValue('shelly')
    await submit(wrapper)

    expect(store.addChannel).toHaveBeenCalledWith({
      name: 'Chat', type: 'slack', enabled: true,
      config: { webhook_url: 'https://hooks.slack.com/services/T0/B0/xyz', channel: '#ops', username: 'shelly' },
    })
  })

  it('clears the fields of the other type when the type changes', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Switch')
    await field(wrapper, 'channel-recipients').setValue('ops@example.com')
    await field(wrapper, 'channel-subject').setValue('Typed for email')
    await field(wrapper, 'channel-type').setValue('slack')
    expect(field(wrapper, 'channel-recipients').exists()).toBe(false)

    // back to email: what was typed for it is gone
    await field(wrapper, 'channel-type').setValue('email')
    expect((field(wrapper, 'channel-recipients').element as HTMLTextAreaElement).value).toBe('')
    expect((field(wrapper, 'channel-subject').element as HTMLInputElement).value).toBe('')

    await field(wrapper, 'channel-type').setValue('slack')
    await field(wrapper, 'channel-slack-url').setValue('https://hooks.slack.com/services/T0/B0/xyz')
    await submit(wrapper)
    const sent = vi.mocked(store.addChannel).mock.calls[0][0]
    expect(sent.type).toBe('slack')
    expect(sent.config).toEqual({ webhook_url: 'https://hooks.slack.com/services/T0/B0/xyz' })
  })

  it('marks the required field of each type, and shows the server\'s reason when it refuses the config', async () => {
    const { wrapper, store } = await openForm()
    expect(field(wrapper, 'channel-recipients').attributes('required')).toBeDefined()
    await field(wrapper, 'channel-type').setValue('webhook')
    expect(field(wrapper, 'channel-webhook-url').attributes('required')).toBeDefined()
    await field(wrapper, 'channel-type').setValue('slack')
    expect(field(wrapper, 'channel-slack-url').attributes('required')).toBeDefined()

    // blanks only: the form sends what it has and the server's own sentence is shown
    await field(wrapper, 'channel-type').setValue('email')
    await field(wrapper, 'channel-name').setValue('Empty')
    await field(wrapper, 'channel-recipients').setValue(' , \n ')
    vi.mocked(store.addChannel).mockRejectedValue(
      new Error('invalid channel configuration: email config must have at least one recipient')
    )
    await submit(wrapper)
    expect(vi.mocked(store.addChannel).mock.calls[0][0].config).toEqual({ recipients: [] })
    expect(wrapper.find('[data-testid="channel-create-error"]').text()).toBe(
      'invalid channel configuration: email config must have at least one recipient'
    )
  })

  it('refuses two headers of the same name, in any case, and posts nothing', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Dup')
    await field(wrapper, 'channel-type').setValue('webhook')
    await field(wrapper, 'channel-webhook-url').setValue('https://hooks.example.com/x')
    await field(wrapper, 'channel-header-name').setValue('X-Token')
    await field(wrapper, 'channel-header-value').setValue('a')
    await field(wrapper, 'channel-add-header').trigger('click')
    await wrapper.findAll('[data-testid="channel-header-name"]')[1].setValue('x-token')
    await wrapper.findAll('[data-testid="channel-header-value"]')[1].setValue('b')
    await submit(wrapper)

    expect(store.addChannel).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="channel-create-error"]').text()).toContain('x-token')
  })

  it('Add header and Remove do not submit the form', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Hook')
    await field(wrapper, 'channel-type').setValue('webhook')
    await field(wrapper, 'channel-webhook-url').setValue('https://hooks.example.com/x')
    await field(wrapper, 'channel-add-header').trigger('click')
    expect(wrapper.findAll('[data-testid="channel-header-name"]').length).toBe(2)
    await field(wrapper, 'channel-remove-header').trigger('click')
    expect(wrapper.findAll('[data-testid="channel-header-name"]').length).toBe(1)
    expect(store.addChannel).not.toHaveBeenCalled()
    for (const id of ['channel-add-header', 'channel-remove-header']) {
      expect(field(wrapper, id).attributes('type')).toBe('button')
    }
  })

  it('opens empty after a create and posts a typed secret exactly as typed', async () => {
    const { wrapper, store } = await openForm()
    await field(wrapper, 'channel-name').setValue('Hook')
    await field(wrapper, 'channel-type').setValue('webhook')
    await field(wrapper, 'channel-webhook-url').setValue('https://hooks.example.com/x')
    await field(wrapper, 'channel-webhook-secret').setValue('********abc')
    await submit(wrapper)
    expect(vi.mocked(store.addChannel).mock.calls[0][0].config).toEqual({
      url: 'https://hooks.example.com/x', method: 'POST', secret: '********abc',
    })

    await wrapper.find('.primary-button').trigger('click')
    expect((field(wrapper, 'channel-name').element as HTMLInputElement).value).toBe('')
    expect(field(wrapper, 'channel-recipients').exists()).toBe(true)
    await field(wrapper, 'channel-type').setValue('webhook')
    expect((field(wrapper, 'channel-webhook-secret').element as HTMLInputElement).value).toBe('')
    expect((field(wrapper, 'channel-webhook-url').element as HTMLInputElement).value).toBe('')
  })
})
