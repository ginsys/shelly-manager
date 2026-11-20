import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
}))

import NotificationChannelDetailPage from '../NotificationChannelDetailPage.vue'
import { useNotificationsStore } from '@/stores/notifications'

// ginsys/shelly-manager#783: edit a channel's config from the channel page. The read config is what the server
// sends (internal/notification/config_secrets.go): `url`, `webhook_url` and `headers` masked whole and `secret`
// by the key rule, each as the placeholder. The server takes a placeholder back as "keep the stored value", so
// a credential the user left alone goes back as the placeholder and one they typed goes as typed. No request
// is made: the store action is a spy.

const MASK = '********'
const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }
const realCreateElement = Document.prototype.createElement.bind(document)

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
})
afterEach(() => {
  vi.restoreAllMocks()
})

const base = {
  id: '7', name: 'Ops hook', enabled: true, description: 'Pages the on-call',
  createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
}
const webhook = {
  ...base, type: 'webhook',
  config: { url: MASK, method: 'PUT', headers: MASK, secret: MASK, template: '{"text":"x"}', timeout: 10, retries: 5 },
}
const slack = { ...base, type: 'slack', config: { webhook_url: MASK, channel: '#ops', username: 'shelly' } }
const email = { ...base, type: 'email', config: { recipients: ['ops@example.com', 'oncall@example.com'], subject: 'Alert' } }

async function mountEdit(channel: Record<string, unknown>) {
  const wrapper = mount(NotificationChannelDetailPage, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { notifications: { currentChannel: channel, channelsError: '' } } })],
      stubs: { 'router-link': RouterLinkStub },
    },
  })
  const store = useNotificationsStore()
  vi.mocked(store.saveChannel).mockResolvedValue(undefined as any)
  await flushPromises()
  return { wrapper, store }
}

const field = (wrapper: any, id: string) => wrapper.find(`[data-testid="${id}"]`)

async function openEdit(wrapper: any) {
  await field(wrapper, 'channel-edit').trigger('click')
}
async function save(wrapper: any) {
  await wrapper.find('form').trigger('submit')
  await flushPromises()
}

describe('NotificationChannelDetailPage edit form', () => {
  it('opens with the plain fields filled and every stored credential kept, without its input', async () => {
    const { wrapper } = await mountEdit(webhook)
    expect(wrapper.find('form').exists()).toBe(false)
    await openEdit(wrapper)

    expect((field(wrapper, 'channel-webhook-method').element as HTMLSelectElement).value).toBe('PUT')
    for (const slot of ['url', 'headers', 'secret']) {
      expect((field(wrapper, `channel-keep-${slot}`).element as HTMLInputElement).checked).toBe(true)
    }
    // A kept slot is not rendered: a hidden required input would block the browser's submit.
    expect(field(wrapper, 'channel-webhook-url').exists()).toBe(false)
    expect(field(wrapper, 'channel-webhook-secret').exists()).toBe(false)
    expect(field(wrapper, 'channel-header-name').exists()).toBe(false)
    expect(wrapper.find('form').text()).not.toContain(MASK)
    // The type is shown and cannot change.
    expect(field(wrapper, 'channel-type').exists()).toBe(false)
  })

  it('sends the placeholder for every credential the user left alone, and posts with no required input present', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    await openEdit(wrapper)
    expect(wrapper.find('form').element.querySelectorAll('[required]').length).toBe(0)
    await field(wrapper, 'channel-webhook-method').setValue('PATCH')
    await save(wrapper)

    expect(store.saveChannel).toHaveBeenCalledTimes(1)
    expect(store.saveChannel).toHaveBeenCalledWith('7', {
      id: '7', name: 'Ops hook', type: 'webhook', enabled: true, description: 'Pages the on-call',
      createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
      config: { url: MASK, method: 'PATCH', headers: MASK, secret: MASK, template: '{"text":"x"}', timeout: 10, retries: 5 },
    })
  })

  it('sends a typed URL and secret as typed and keeps the headers', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    await openEdit(wrapper)
    await field(wrapper, 'channel-keep-url').setValue(false)
    await field(wrapper, 'channel-webhook-url').setValue('https://hooks.example.com/new')
    await field(wrapper, 'channel-keep-secret').setValue(false)
    await field(wrapper, 'channel-webhook-secret').setValue('new-signing-secret')
    await save(wrapper)

    const sent = vi.mocked(store.saveChannel).mock.calls[0][1]
    expect(sent.config).toMatchObject({ url: 'https://hooks.example.com/new', secret: 'new-signing-secret', headers: MASK })
  })

  it('replaces the headers with the rows typed when keep is turned off', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    await openEdit(wrapper)
    await field(wrapper, 'channel-keep-headers').setValue(false)
    await field(wrapper, 'channel-header-name').setValue('Authorization')
    await field(wrapper, 'channel-header-value').setValue('Bearer new')
    await save(wrapper)

    const sent = vi.mocked(store.saveChannel).mock.calls[0][1]
    expect(sent.config.headers).toEqual({ Authorization: 'Bearer new' })
    expect(sent.config.url).toBe(MASK)
  })

  it('replaces the headers with an empty list as no headers key', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    await openEdit(wrapper)
    await field(wrapper, 'channel-keep-headers').setValue(false)
    await save(wrapper)

    const sent = vi.mocked(store.saveChannel).mock.calls[0][1]
    expect(sent.config).not.toHaveProperty('headers')
  })

  it('edits a slack channel: the webhook URL is kept, the plain fields are sent as typed', async () => {
    const { wrapper, store } = await mountEdit(slack)
    await openEdit(wrapper)
    expect((field(wrapper, 'channel-slack-channel').element as HTMLInputElement).value).toBe('#ops')
    expect(field(wrapper, 'channel-slack-url').exists()).toBe(false)
    await field(wrapper, 'channel-slack-channel').setValue('#alerts')
    await save(wrapper)

    expect(vi.mocked(store.saveChannel).mock.calls[0][1].config).toEqual({
      webhook_url: MASK, channel: '#alerts', username: 'shelly',
    })
  })

  it('edits an email channel with its recipients filled one per line', async () => {
    const { wrapper, store } = await mountEdit(email)
    await openEdit(wrapper)
    expect((field(wrapper, 'channel-recipients').element as HTMLTextAreaElement).value).toBe('ops@example.com\noncall@example.com')
    await field(wrapper, 'channel-recipients').setValue('new@example.com')
    await save(wrapper)

    expect(vi.mocked(store.saveChannel).mock.calls[0][1].config).toEqual({ recipients: ['new@example.com'], subject: 'Alert' })
  })

  it('shows the server\'s reason in the form, keeps what was typed and stays in edit mode', async () => {
    const { wrapper, store } = await mountEdit(email)
    vi.mocked(store.saveChannel).mockRejectedValue(
      new Error('invalid channel configuration: email config must have at least one recipient')
    )
    await openEdit(wrapper)
    await field(wrapper, 'channel-recipients').setValue('')
    await field(wrapper, 'channel-subject').setValue('Typed subject')
    await save(wrapper)

    const error = wrapper.find('form [data-testid="channel-save-error"]')
    expect(error.text()).toBe('invalid channel configuration: email config must have at least one recipient')
    expect((field(wrapper, 'channel-subject').element as HTMLInputElement).value).toBe('Typed subject')
  })

  // The URL input is `required`, as in the create form: the browser stops an empty one before save() runs. A
  // blank one passes that check, is sent empty and the server says why.
  it('leaves an empty URL to the browser and a blank one to the server', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    vi.mocked(store.saveChannel).mockRejectedValue(new Error('invalid channel configuration: webhook config must have a URL'))
    await openEdit(wrapper)
    await field(wrapper, 'channel-keep-url').setValue(false)
    const form = wrapper.find('form').element as HTMLFormElement
    expect(form.checkValidity()).toBe(false)
    form.requestSubmit()
    await flushPromises()
    expect(store.saveChannel).not.toHaveBeenCalled()

    await field(wrapper, 'channel-webhook-url').setValue('   ')
    expect(form.checkValidity()).toBe(true)
    await save(wrapper)
    expect(vi.mocked(store.saveChannel).mock.calls[0][1].config.url).toBe('')
    expect(field(wrapper, 'channel-save-error').text()).toBe('invalid channel configuration: webhook config must have a URL')
  })

  it('refuses two headers of one name and posts nothing', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    await openEdit(wrapper)
    await field(wrapper, 'channel-keep-headers').setValue(false)
    await field(wrapper, 'channel-add-header').trigger('click')
    const names = wrapper.findAll('[data-testid="channel-header-name"]')
    await names[0].setValue('X-Team')
    await names[1].setValue('x-team')
    await save(wrapper)

    expect(store.saveChannel).not.toHaveBeenCalled()
    expect(field(wrapper, 'channel-save-error').text()).toBe('Header x-team appears twice')
  })

  // setChannelEnabled reads the channel and PUTs its whole config back, so one that runs while a save is
  // pending can send the old config after the save and undo it.
  it('cannot enable or disable the channel while a save is pending', async () => {
    const { wrapper, store } = await mountEdit(email)
    let finish: () => void = () => {}
    vi.mocked(store.saveChannel).mockReturnValue(new Promise<any>((resolve) => { finish = () => resolve(undefined) }))
    await openEdit(wrapper)
    await save(wrapper)

    const toggle = field(wrapper, 'channel-toggle')
    expect((toggle.element as HTMLButtonElement).disabled).toBe(true)
    await toggle.trigger('click')
    expect(store.setChannelEnabled).not.toHaveBeenCalled()

    finish()
    await flushPromises()
    expect((field(wrapper, 'channel-toggle').element as HTMLButtonElement).disabled).toBe(false)
  })

  it('leaves edit mode after a save', async () => {
    const { wrapper } = await mountEdit(email)
    await openEdit(wrapper)
    await save(wrapper)
    expect(wrapper.find('form').exists()).toBe(false)
    expect(field(wrapper, 'channel-edit').exists()).toBe(true)
  })

  it('Cancel restores the read view and sends nothing, and a typed credential does not come back', async () => {
    const { wrapper, store } = await mountEdit(webhook)
    await openEdit(wrapper)
    await field(wrapper, 'channel-keep-secret').setValue(false)
    await field(wrapper, 'channel-webhook-secret').setValue('typed-then-cancelled')
    await field(wrapper, 'channel-edit-cancel').trigger('click')

    expect(store.saveChannel).not.toHaveBeenCalled()
    expect(wrapper.find('form').exists()).toBe(false)

    await openEdit(wrapper)
    expect((field(wrapper, 'channel-keep-secret').element as HTMLInputElement).checked).toBe(true)
    expect(field(wrapper, 'channel-webhook-secret').exists()).toBe(false)
    expect(wrapper.html()).not.toContain('typed-then-cancelled')
  })

  it('does not offer to keep a credential the channel does not store', async () => {
    const { wrapper, store } = await mountEdit({ ...webhook, config: { url: MASK, method: 'POST' } })
    await openEdit(wrapper)
    expect(field(wrapper, 'channel-keep-headers').exists()).toBe(false)
    expect(field(wrapper, 'channel-keep-secret').exists()).toBe(false)
    expect(field(wrapper, 'channel-webhook-secret').exists()).toBe(true)
    await save(wrapper)

    expect(vi.mocked(store.saveChannel).mock.calls[0][1].config).toEqual({ url: MASK, method: 'POST' })
  })
})
