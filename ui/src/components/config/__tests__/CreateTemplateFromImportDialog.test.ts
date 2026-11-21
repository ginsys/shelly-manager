import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

vi.mock('@/api/deviceConfig', () => ({ getTypedDeviceConfig: vi.fn() }))
vi.mock('@/api/typedConfig', () => ({ getConfigSchema: vi.fn() }))
vi.mock('@/api/templates', async (importActual) => {
  const actual = await importActual<typeof import('@/api/templates')>()
  return { ...actual, createTemplate: vi.fn(), listDeviceTypes: vi.fn() }
})

import { getTypedDeviceConfig } from '@/api/deviceConfig'
import { getConfigSchema } from '@/api/typedConfig'
import { createTemplate, listDeviceTypes, TemplateSaveError } from '@/api/templates'
import { dismissNotice, notice, noticeSeverity } from '@/composables/useNotice'
import CreateTemplateFromImportDialog from '../CreateTemplateFromImportDialog.vue'

// The schema the server builds for the device names the model it was built for.
const schema = {
  type: 'object',
  'x-device-type': 'SHPLG-S',
  properties: {
    wifi: { type: 'object', properties: { ssid: { type: 'string' }, password: { type: 'string' } } },
    mqtt: {
      type: 'object',
      properties: { enable: { type: 'boolean' }, server: { type: 'string' }, pass: { type: 'string' } },
    },
    auth: { type: 'object', properties: { user: { type: 'string' }, pass: { type: 'string' } } },
    system: { type: 'object', properties: { tz: { type: 'string' } } },
  },
}

const typed = {
  wifi: { ssid: 'Home', password: '********' },
  mqtt: { enable: true, server: 'broker:1883', pass: '********' },
  auth: { user: 'admin', pass: '********' },
  system: { tz: 'Europe/Brussels' },
  raw: { anything: '********' },
}

async function ready() {
  const wrapper = mount(CreateTemplateFromImportDialog, { props: { deviceId: 7 } })
  await flushPromises()
  return wrapper
}

describe('CreateTemplateFromImportDialog', () => {
  beforeEach(() => {
    push.mockReset()
    ;(getTypedDeviceConfig as any).mockReset().mockResolvedValue(typed)
    ;(getConfigSchema as any).mockReset().mockResolvedValue(schema)
    ;(listDeviceTypes as any)
      .mockReset()
      .mockResolvedValue([{ device_type: 'SHPLG-S', name: 'Shelly Plug S', generation: 1 }])
    ;(createTemplate as any).mockReset().mockResolvedValue({ id: 42 })
  })

  it('starts wifi and auth unticked and the other sections ticked, and offers no section the schema lacks', async () => {
    const wrapper = await ready()

    const checked = (s: string) => (wrapper.get(`[data-testid="section-${s}"]`).element as HTMLInputElement).checked
    expect(checked('wifi')).toBe(false)
    expect(checked('auth')).toBe(false)
    expect(checked('mqtt')).toBe(true)
    expect(checked('system')).toBe(true)
    expect(wrapper.find('[data-testid="section-raw"]').exists()).toBe(false)
  })

  it('sends a body that never holds the placeholder, even with every section ticked', async () => {
    const wrapper = await ready()
    await wrapper.get('[data-testid="section-wifi"]').setValue(true)
    await wrapper.get('[data-testid="section-auth"]').setValue(true)
    await wrapper.get('[data-testid="from-import-name"]').setValue('Home defaults')

    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    expect(createTemplate).toHaveBeenCalledTimes(1)
    const sent = (createTemplate as any).mock.calls[0][0]
    expect(JSON.stringify(sent)).not.toContain('********')
    expect(sent.config.wifi).toEqual({ ssid: 'Home' })
    expect(sent.config.mqtt).toEqual({ enable: true, server: 'broker:1883' })
  })

  it('leaves out a section that is not ticked', async () => {
    const wrapper = await ready()
    await wrapper.get('[data-testid="section-system"]').setValue(false)
    await wrapper.get('[data-testid="from-import-name"]').setValue('Only mqtt')

    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    const sent = (createTemplate as any).mock.calls[0][0]
    expect(Object.keys(sent.config)).toEqual(['mqtt'])
  })

  it('names what it leaves out: per-device values and each secret', async () => {
    const wrapper = await ready()

    const note = wrapper.get('[data-testid="from-import-left-out"]').text()
    expect(note).toContain('per-device name, hostname, MAC, MQTT client id and topic prefix')
    expect(note).toContain('access-point name, channel names and static address')
    expect(note).toContain('mqtt.pass')
  })

  it('fills the scope from the model of the device schema, and falls back to all devices for a model the server does not list', async () => {
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('By model')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()
    expect((createTemplate as any).mock.calls[0][0]).toMatchObject({ scope: 'device_type', device_type: 'SHPLG-S' })

    ;(createTemplate as any).mockClear()
    ;(getConfigSchema as any).mockResolvedValue({ ...schema, 'x-device-type': 'SHXX-9' })
    const other = await ready()
    expect(other.find('option[value="device_type"]').exists()).toBe(false)
    await other.get('[data-testid="from-import-name"]').setValue('All')
    await other.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()
    const sent = (createTemplate as any).mock.calls[0][0]
    expect(sent.scope).toBe('global')
    expect(sent.device_type).toBeUndefined()

    // A schema that names no model (an older answer) gives no device-type scope either.
    const { 'x-device-type': _model, ...unnamed } = schema
    ;(getConfigSchema as any).mockResolvedValue(unnamed)
    const none = await ready()
    expect(none.find('option[value="device_type"]').exists()).toBe(false)
  })

  it('shows the server reason of a 400, with its field list', async () => {
    ;(createTemplate as any).mockRejectedValue(
      new TemplateSaveError('Validation failed', [{ path: 'mqtt.server', message: 'Not a host name' }]),
    )
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('Bad')

    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="from-import-save-error"]').text()).toBe('Validation failed')
    expect(wrapper.get('[data-testid="from-import-field-errors"]').text()).toContain('mqtt.server')
    expect(wrapper.get('[data-testid="from-import-field-errors"]').text()).toContain('Not a host name')
    expect(push).not.toHaveBeenCalled()
  })

  it('shows the message of any other refusal', async () => {
    ;(createTemplate as any).mockRejectedValue(new Error('name is required'))
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('x')

    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="from-import-save-error"]').text()).toBe('name is required')
  })

  it('links to the new template after a save', async () => {
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('Good')

    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    expect(push).toHaveBeenCalledWith({ name: 'template-detail', params: { id: 42 } })
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('shows a failed read as an error and offers no form: nothing stored answers 500, never an empty import', async () => {
    ;(getTypedDeviceConfig as any).mockRejectedValue({ message: 'Request failed with status code 500' })
    const wrapper = await ready()

    expect(wrapper.get('[data-testid="from-import-load-error"]').text()).toContain('Request failed with status code 500')
    expect(wrapper.find('[data-testid="from-import-name"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="from-import-save"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('cannot be dismissed while the save is pending, so its outcome is never lost', async () => {
    let finish: (v: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('Slow')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')

    expect((wrapper.get('[data-testid="from-import-cancel"]').element as HTMLButtonElement).disabled).toBe(true)
    await wrapper.get('[data-testid="from-import-cancel"]').trigger('click')
    await wrapper.get('[data-testid="from-import-dialog"]').trigger('click')
    expect(wrapper.emitted('close')).toBeFalsy()

    finish({ id: 9 })
    await flushPromises()
    expect(push).toHaveBeenCalledWith({ name: 'template-detail', params: { id: 9 } })
  })

  it('tells the app notice when a save fails after the user left the page, and when it succeeds', async () => {
    dismissNotice()
    let fail: (e: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((_, reject) => (fail = reject)))
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('Gone')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    wrapper.unmount()
    fail(new Error('Database unavailable'))
    await flushPromises()
    expect(notice.value?.message).toContain('Database unavailable')
    expect(noticeSeverity.value).toBe('error')

    dismissNotice()
    let finish: (v: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const second = await ready()
    await second.get('[data-testid="from-import-name"]').setValue('Made')
    await second.get('[data-testid="from-import-save"]').trigger('click')
    second.unmount()
    finish({ id: 5, name: 'Made' })
    await flushPromises()
    expect(notice.value?.message).toContain('Made')
    expect(noticeSeverity.value).toBe('info')
    expect(push).not.toHaveBeenCalled()
  })

  it('keeps the field reasons of a late 400, and names the template the server made rather than the form', async () => {
    dismissNotice()
    let fail: (e: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((_, reject) => (fail = reject)))
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('Bad')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    wrapper.unmount()
    fail(new TemplateSaveError('Validation failed', [{ path: 'mqtt.server', message: 'Not a host name' }]))
    await flushPromises()
    expect(notice.value?.message).toContain('Validation failed')
    expect(notice.value?.message).toContain('mqtt.server')
    expect(notice.value?.message).toContain('Not a host name')

    dismissNotice()
    let finish: (v: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const second = await ready()
    const input = second.get('[data-testid="from-import-name"]')
    await input.setValue('Kitchen')
    await second.get('[data-testid="from-import-save"]').trigger('click')
    await input.setValue('Bedroom')
    second.unmount()
    finish({ id: 42, name: 'Kitchen' })
    await flushPromises()
    expect(notice.value?.message).toContain('Kitchen')
    expect(notice.value?.message).not.toContain('Bedroom')
  })

  it('keeps Save off until there is a name and something ticked', async () => {
    const wrapper = await ready()
    const save = () => (wrapper.get('[data-testid="from-import-save"]').element as HTMLButtonElement).disabled

    expect(save()).toBe(true)
    await wrapper.get('[data-testid="from-import-name"]').setValue('Named')
    expect(save()).toBe(false)
    await wrapper.get('[data-testid="section-mqtt"]').setValue(false)
    await wrapper.get('[data-testid="section-system"]').setValue(false)
    expect(save()).toBe(true)
  })
})
