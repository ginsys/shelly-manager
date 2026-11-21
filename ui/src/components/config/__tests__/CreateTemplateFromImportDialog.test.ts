import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

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

// The global test setup replaces every <a> with a plain object, so the stub for the router link renders a span.
const LinkStub = defineComponent({
  props: ['to'],
  setup: (p, { slots }) => () => h('span', { 'data-to': JSON.stringify(p.to) }, slots.default?.()),
})

async function ready() {
  const wrapper = mount(CreateTemplateFromImportDialog, {
    props: { deviceId: 7 },
    global: { stubs: { RouterLink: LinkStub } },
  })
  await flushPromises()
  return wrapper
}

describe('CreateTemplateFromImportDialog', () => {
  beforeEach(() => {
    ;(getTypedDeviceConfig as any).mockReset().mockResolvedValue(typed)
    ;(getConfigSchema as any).mockReset().mockResolvedValue(schema)
    ;(listDeviceTypes as any)
      .mockReset()
      .mockResolvedValue([{ device_type: 'SHPLG-S', name: 'Shelly Plug S', generation: 1 }])
    ;(createTemplate as any).mockReset().mockResolvedValue({ id: 42, name: 'Good' })
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
    expect(note).toContain('access-point name, channel names, coordinates and static address')
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

    // Saving navigates nowhere: the dialog shows the result and links to the new template.
    expect(wrapper.get('[data-testid="from-import-saved"]').text()).toContain('Template saved')
    expect(wrapper.get('[data-testid="from-import-open"]').attributes('data-to')).toBe(
      JSON.stringify({ name: 'template-detail', params: { id: 42 } }),
    )
    expect(wrapper.find('[data-testid="from-import-save"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="from-import-name"]').exists()).toBe(false)
    expect(wrapper.emitted('close')).toBeFalsy()

    await wrapper.get('[data-testid="from-import-cancel"]').trigger('click')
    expect(wrapper.get('[data-testid="from-import-cancel"]').text()).toBe('Close')
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

    finish({ id: 9, name: 'Slow' })
    await flushPromises()
    expect(wrapper.get('[data-testid="from-import-saved"]').exists()).toBe(true)
  })

  // After the user left the page nothing of the form is shown any more, so the app notice says only that
  // the save failed or went through, never a name or a field reason that the form or the answer might not hold.
  it('tells the app notice, in general words, when a save ends after the user left the page', async () => {
    dismissNotice()
    let fail: (e: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((_, reject) => (fail = reject)))
    const wrapper = await ready()
    await wrapper.get('[data-testid="from-import-name"]').setValue('Gone')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    wrapper.unmount()
    fail(new TemplateSaveError('Validation failed', [{ path: 'mqtt.server', message: 'Not a host name' }]))
    await flushPromises()
    expect(notice.value?.message).toBe('Template save failed. Open the Templates page to check.')
    expect(noticeSeverity.value).toBe('error')

    dismissNotice()
    let finish: (v: unknown) => void = () => {}
    ;(createTemplate as any).mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const second = await ready()
    await second.get('[data-testid="from-import-name"]').setValue('Kitchen')
    await second.get('[data-testid="from-import-save"]').trigger('click')
    second.unmount()
    finish({ id: 5 })
    await flushPromises()
    expect(notice.value?.message).toBe('Template saved. Find it on the Templates page.')
    expect(noticeSeverity.value).toBe('info')
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

// The common-settings wizard hands the dialog the settings the devices share, so the dialog reads no device.
describe('CreateTemplateFromImportDialog with common settings', () => {
  const common = {
    typed: { mqtt: { enable: true, server: 'broker:1883' }, system: { tz: 'Europe/Brussels' } },
    deviceType: 'SHPLG-S',
    differing: { mqtt: 2 },
  }

  async function readyCommon(props: Record<string, unknown> = { common }) {
    const wrapper = mount(CreateTemplateFromImportDialog, {
      props: { title: 'Create template from the common settings', ...props },
      global: { stubs: { RouterLink: LinkStub } },
    })
    await flushPromises()
    return wrapper
  }

  beforeEach(() => {
    ;(getTypedDeviceConfig as any).mockReset()
    ;(getConfigSchema as any).mockReset().mockResolvedValue(schema)
    ;(listDeviceTypes as any)
      .mockReset()
      .mockResolvedValue([{ device_type: 'SHPLG-S', name: 'Shelly Plug S', generation: 1 }])
    ;(createTemplate as any).mockReset().mockResolvedValue({ id: 42, name: 'Good' })
  })

  it('reads no device, takes its sections from the common settings and the schema of their model', async () => {
    const wrapper = await readyCommon()

    expect(getTypedDeviceConfig).not.toHaveBeenCalled()
    expect(getConfigSchema).toHaveBeenCalledWith('SHPLG-S', undefined, undefined)
    expect(wrapper.get('#from-import-title').text()).toBe('Create template from the common settings')
    expect(wrapper.find('[data-testid="section-mqtt"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="section-system"]').exists()).toBe(true)
  })

  it('says how many values of a section differ, and saves what was ticked for that model', async () => {
    const wrapper = await readyCommon()

    expect(wrapper.get('[data-testid="section-differing-mqtt"]').text()).toContain('2 differ')
    expect(wrapper.find('[data-testid="section-differing-system"]').exists()).toBe(false)

    await wrapper.get('[data-testid="section-system"]').setValue(false)
    await wrapper.get('[data-testid="from-import-name"]').setValue('Shared')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    const sent = (createTemplate as any).mock.calls[0][0]
    expect(sent).toMatchObject({ name: 'Shared', scope: 'device_type', device_type: 'SHPLG-S' })
    expect(sent.config).toEqual({ mqtt: { enable: true, server: 'broker:1883' } })
  })

  it('asks for the global schema and offers only all devices when the devices are not one model', async () => {
    ;(getConfigSchema as any).mockResolvedValue({ ...schema, 'x-device-type': undefined })
    const wrapper = await readyCommon({ common: { ...common, deviceType: '' } })

    expect(getConfigSchema).toHaveBeenCalledWith(undefined, undefined, 'global')
    expect(wrapper.find('option[value="device_type"]').exists()).toBe(false)
  })
})
