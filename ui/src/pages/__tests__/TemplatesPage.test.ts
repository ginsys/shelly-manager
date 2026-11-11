import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import type { ConfigSchema } from '@/api/typedConfig'

vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

vi.mock('@/api/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/templates')>()
  return {
    ...actual,
    listTemplates: vi.fn(),
    createTemplate: vi.fn(),
    updateTemplate: vi.fn(),
  }
})

import { getConfigSchema } from '@/api/typedConfig'
import { createTemplate, listTemplates, updateTemplate, TemplateSaveError } from '@/api/templates'
import TemplatesPage from '../TemplatesPage.vue'

// A cut-down template schema, in the shape GET /config/schema returns.
const schema: ConfigSchema = {
  type: 'object',
  'x-apply-supported': true,
  properties: {
    power_metering: {
      type: 'object',
      title: 'Power Metering',
      properties: {
        max_power: { type: 'integer', minimum: 0, default: 2500, 'x-applicable': true },
      },
    },
  },
}

function mountPage() {
  return mount(TemplatesPage, { global: { stubs: { 'router-link': true } } })
}

async function openCreate(wrapper: ReturnType<typeof mountPage>) {
  await wrapper.find('.toolbar .btn.primary').trigger('click')
  await flushPromises()
}

async function openSection(wrapper: ReturnType<typeof mountPage>, section: string) {
  if (!wrapper.find(`[data-section="${section}"] .section-content`).exists()) {
    await wrapper.find(`[data-section="${section}"] .section-header`).trigger('click')
  }
}

describe('TemplatesPage template form', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getConfigSchema).mockReset()
    vi.mocked(getConfigSchema).mockResolvedValue(schema)
    vi.mocked(listTemplates).mockReset()
    vi.mocked(listTemplates).mockResolvedValue([])
    vi.mocked(createTemplate).mockReset()
    vi.mocked(updateTemplate).mockReset()
  })

  it('a new global template gets the template schema and starts empty', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    expect(getConfigSchema).toHaveBeenCalledWith(undefined, undefined, 'global')
    await openSection(wrapper, 'power_metering')
    // Empty start: the field holds no value, and the default is offered.
    expect((wrapper.find('[data-path="power_metering.max_power"] input').element as HTMLInputElement).value).toBe('')
    expect(wrapper.find('[data-path="power_metering.max_power"] [data-action="use-default"]').exists()).toBe(true)
  })

  it('saves only the fields set in the form', async () => {
    vi.mocked(createTemplate).mockResolvedValue({
      id: 1, name: 'plugs', scope: 'global', config: {}, created_at: '', updated_at: '',
    })
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('.modal .form-input[type="text"]').setValue('plugs')
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] [data-action="use-default"]').trigger('click')
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(createTemplate).toHaveBeenCalledWith({
      name: 'plugs',
      description: '',
      scope: 'global',
      device_type: undefined,
      config: { power_metering: { max_power: 2500 } },
    })
  })

  it('a device_type template loads the schema of its model once the type is entered', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('[data-testid="template-scope"]').setValue('device_type')
    expect(wrapper.find('[data-testid="template-needs-type"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()

    vi.mocked(getConfigSchema).mockClear()
    const input = wrapper.find('[data-testid="template-device-type"]')
    await input.setValue('SHPLG-S') // setValue fires input, then change
    await flushPromises()
    expect(getConfigSchema).toHaveBeenCalledTimes(1)
    expect(getConfigSchema).toHaveBeenCalledWith('SHPLG-S', undefined, undefined)
    expect(wrapper.find('[data-testid="template-needs-type"]').exists()).toBe(false)
  })

  it('an invalid field blocks save', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('-1')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()
  })

  // The server returns a saved password as has_auth_password and leaves the value out. The auth
  // section requires a password once it is enabled, so a name-only edit must not be blocked by
  // the value it cannot see, and must not send the config back without that value.
  const authSchema: ConfigSchema = {
    type: 'object',
    'x-apply-supported': true,
    properties: {
      auth: {
        type: 'object',
        title: 'Authentication',
        if: { properties: { enable: { const: true } } },
        then: { required: ['user', 'pass'] },
        properties: {
          enable: { type: 'boolean', 'x-applicable': true },
          user: { type: 'string', 'x-applicable': true },
          pass: { type: 'string', format: 'password', 'x-applicable': true },
        },
      },
      power_metering: schema.properties!.power_metering,
    },
  }
  const savedAuthTemplate = {
    id: 7, name: 'locked', description: '', scope: 'global', created_at: '', updated_at: '',
    config: { auth: { enable: true, user: 'admin' } }, has_auth_password: true,
  }

  async function openSavedTemplate(wrapper: ReturnType<typeof mountPage>) {
    await flushPromises()
    await wrapper.findAll('.action-btn').find(b => b.text() === 'Edit')!.trigger('click')
    await flushPromises()
  }

  it('a name-only edit of a template with a saved password saves and keeps the config out', async () => {
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    vi.mocked(listTemplates).mockResolvedValue([savedAuthTemplate] as any)
    vi.mocked(updateTemplate).mockResolvedValue({ template: savedAuthTemplate } as any)
    const wrapper = mountPage()
    await openSavedTemplate(wrapper)
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
    await wrapper.find('.modal .form-input[type="text"]').setValue('renamed')
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(updateTemplate).toHaveBeenCalledTimes(1)
    const sent = vi.mocked(updateTemplate).mock.calls[0][1] as Record<string, unknown>
    expect(sent.name).toBe('renamed')
    expect('config' in sent).toBe(false)
  })

  // The server keeps a password the body leaves out, so the edit saves without entering it again.
  it('a config edit that leaves out the saved password still saves', async () => {
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    vi.mocked(listTemplates).mockResolvedValue([savedAuthTemplate] as any)
    vi.mocked(updateTemplate).mockResolvedValue({ template: savedAuthTemplate } as any)
    const wrapper = mountPage()
    await openSavedTemplate(wrapper)
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('1400')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    const sent = vi.mocked(updateTemplate).mock.calls[0][1] as { config: { auth?: { pass?: string } } }
    expect(sent.config.auth?.pass).toBeUndefined()
  })

  it('Clear stored password sends an empty string for that password', async () => {
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    vi.mocked(listTemplates).mockResolvedValue([{ ...savedAuthTemplate, stored_secrets: ['auth.pass'] }] as any)
    vi.mocked(updateTemplate).mockResolvedValue({ template: savedAuthTemplate } as any)
    const wrapper = mountPage()
    await openSavedTemplate(wrapper)
    await openSection(wrapper, 'auth')
    await wrapper.find('[data-path="auth.pass"] [data-action="clear-secret"]').trigger('click')
    expect(wrapper.find('[data-path="auth.pass"] [data-note="clears-secret"]').exists()).toBe(true)
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    const sent = vi.mocked(updateTemplate).mock.calls[0][1] as { config: { auth: { pass: string } } }
    expect(sent.config.auth.pass).toBe('')
  })

  // The network passwords are pointers like the other secrets: "" clears one on the server, so the
  // form offers Clear for them too.
  it('offers a Clear action for a saved network password', async () => {
    const networkSchema: ConfigSchema = {
      type: 'object',
      'x-apply-supported': true,
      properties: {
        network: {
          type: 'object',
          title: 'Network',
          properties: {
            wifi: {
              type: 'object',
              properties: {
                sta: {
                  type: 'object',
                  properties: { pass: { type: 'string', format: 'password', 'x-applicable': true } },
                },
              },
            },
          },
        },
        mqtt: {
          type: 'object',
          title: 'MQTT',
          properties: { pass: { type: 'string', format: 'password', 'x-applicable': true } },
        },
      },
    }
    const saved = {
      id: 10, name: 'net', description: '', scope: 'global', created_at: '', updated_at: '',
      config: {}, stored_secrets: ['network.wifi.sta.pass', 'mqtt.pass'],
    }
    vi.mocked(getConfigSchema).mockResolvedValue(networkSchema)
    vi.mocked(listTemplates).mockResolvedValue([saved] as any)
    const wrapper = mountPage()
    await openSavedTemplate(wrapper)
    await openSection(wrapper, 'network')
    await openSection(wrapper, 'mqtt')
    expect(wrapper.find('[data-path="mqtt.pass"] [data-action="clear-secret"]').exists()).toBe(true)
    expect(wrapper.find('[data-path="network.wifi.sta.pass"]').exists()).toBe(true)
    expect(wrapper.find('[data-path="network.wifi.sta.pass"] [data-action="clear-secret"]').exists()).toBe(true)
  })

  // The wifi password is not a required field of any schema. Setting another field and saving keeps
  // it, because the server keeps a password the body leaves out.
  it('a config edit that leaves out a saved password the schema does not require still saves', async () => {
    const wifiSchema: ConfigSchema = {
      type: 'object',
      'x-apply-supported': true,
      properties: {
        wifi: {
          type: 'object',
          title: 'WiFi',
          properties: {
            enable: { type: 'boolean', 'x-applicable': true },
            ssid: { type: 'string', 'x-applicable': true },
            password: { type: 'string', 'x-applicable': true },
          },
        },
        power_metering: schema.properties!.power_metering,
      },
    }
    const saved = {
      id: 9, name: 'office', description: '', scope: 'global', created_at: '', updated_at: '',
      config: { wifi: { enable: true, ssid: 'office' } }, has_wifi_password: true,
    }
    vi.mocked(getConfigSchema).mockResolvedValue(wifiSchema)
    vi.mocked(listTemplates).mockResolvedValue([saved] as any)
    vi.mocked(updateTemplate).mockResolvedValue({ template: saved } as any)
    const wrapper = mountPage()
    await openSavedTemplate(wrapper)
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('1400')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
  })

  // A template is one layer of the configuration: another layer can give the field the body leaves
  // out, and the server accepts such a body.
  it('a partial template body is valid: a field another layer can give is not required', async () => {
    const partial = {
      id: 8, name: 'partial', description: '', scope: 'global', created_at: '', updated_at: '',
      config: { auth: { enable: true } }, has_auth_password: false,
    }
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    vi.mocked(listTemplates).mockResolvedValue([partial] as any)
    vi.mocked(updateTemplate).mockResolvedValue({ template: partial } as any)
    const wrapper = mountPage()
    await openSavedTemplate(wrapper)
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('1400')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    const sent = vi.mocked(updateTemplate).mock.calls[0][1] as { config?: Record<string, any> }
    expect(sent.config?.auth).toEqual({ enable: true })
    expect(sent.config?.power_metering).toEqual({ max_power: 1400 })
  })

  // A save answers after the dialog it came from was closed. The answer belongs to the closed
  // dialog, so it must not show an error in, or close, the next one. While a save runs no dialog
  // opens (Create and row Edit are off), so the next dialog opens once the save has settled:
  // Save, Cancel, answer, Create.
  const plainTemplate = (id: number, name: string) => ({
    id, name, description: '', scope: 'global', created_at: '', updated_at: '', config: {},
  })

  function deferred<T>() {
    let resolve!: (value: T) => void
    let reject!: (reason: unknown) => void
    const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
    return { promise, resolve, reject }
  }

  async function clickEdit(wrapper: ReturnType<typeof mountPage>, index: number) {
    await wrapper.findAll('.action-btn').filter(b => b.text() === 'Edit')[index].trigger('click')
    await flushPromises()
  }

  async function cancelDialog(wrapper: ReturnType<typeof mountPage>) {
    await wrapper.find('.modal-actions .btn:not(.primary)').trigger('click')
    await flushPromises()
  }

  const refusal = () => new TemplateSaveError('Validation failed',
    [{ path: 'relay.auto_of', message: 'Unknown key "auto_of"' }])

  it('an update error that arrives after the dialog closed does not show in the next dialog', async () => {
    vi.mocked(listTemplates).mockResolvedValue([plainTemplate(1, 'A')] as any)
    const pending = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    await clickEdit(wrapper, 0)
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await cancelDialog(wrapper)
    pending.reject(refusal())
    await flushPromises()
    await openCreate(wrapper)
    expect(wrapper.find('[data-testid="template-save-errors"]').exists()).toBe(false)
    expect(wrapper.find('.modal').exists()).toBe(true)
  })

  it('an update that finishes after the dialog closed does not close the next dialog', async () => {
    vi.mocked(listTemplates).mockResolvedValue([plainTemplate(1, 'A')] as any)
    const pending = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    await clickEdit(wrapper, 0)
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await cancelDialog(wrapper)
    pending.resolve({ template: plainTemplate(1, 'A') })
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('.modal .form-input[type="text"]').setValue('draft')
    await flushPromises()
    expect(wrapper.find('.modal').exists()).toBe(true)
    expect((wrapper.find('.modal .form-input[type="text"]').element as HTMLInputElement).value).toBe('draft')
  })

  it('a create error that arrives after the dialog closed does not show in the next dialog', async () => {
    const pending = deferred<any>()
    vi.mocked(createTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('.modal .form-input[type="text"]').setValue('first')
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await cancelDialog(wrapper)
    pending.reject(refusal())
    await flushPromises()
    await openCreate(wrapper)
    expect(wrapper.find('[data-testid="template-save-errors"]').exists()).toBe(false)
    expect(wrapper.find('.modal').exists()).toBe(true)
  })

  it('a second Save click while a save is pending sends nothing', async () => {
    const pending = deferred<any>()
    vi.mocked(createTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('.modal .form-input[type="text"]').setValue('once')
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    pending.reject(refusal())
    await flushPromises()
    expect(vi.mocked(createTemplate)).toHaveBeenCalledTimes(1)
    // The save has settled, so Save works again.
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
  })

  it('an error shows in the dialog it came from while that dialog stays open', async () => {
    vi.mocked(listTemplates).mockResolvedValue([plainTemplate(1, 'A')] as any)
    vi.mocked(updateTemplate).mockRejectedValue(refusal())
    const wrapper = mountPage()
    await flushPromises()
    await clickEdit(wrapper, 0)
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="template-save-errors"]').text()).toContain('Unknown key "auto_of"')
  })

  it('shows each field the server refused', async () => {
    vi.mocked(createTemplate).mockRejectedValue(
      new TemplateSaveError('Validation failed', [{ path: 'roller', message: 'SHPLG-S has no roller settings' }]),
    )
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="template-save-errors"]').text()).toContain('roller: SHPLG-S has no roller settings')
    expect(wrapper.find('.modal').exists()).toBe(true)
  })
})
