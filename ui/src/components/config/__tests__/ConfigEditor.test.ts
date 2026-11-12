import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import ConfigEditor from '../ConfigEditor.vue'
import type { ConfigSchema } from '@/api/typedConfig'

vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

import { getConfigSchema } from '@/api/typedConfig'

// A cut-down device-type schema in the shape GET /config/schema?device_id= returns.
const schema: ConfigSchema = {
  type: 'object',
  'x-device-type': 'SHPLG-S',
  'x-apply-supported': true,
  properties: {
    system: {
      type: 'object',
      title: 'System',
      properties: {
        device: {
          type: 'object',
          title: 'Device',
          properties: {
            name: { type: 'string', maxLength: 64, 'x-applicable': true },
            eco_mode: { type: 'boolean', default: false, 'x-applicable': true },
          },
        },
      },
    },
    power_metering: {
      type: 'object',
      title: 'Power Metering',
      properties: {
        max_power: { type: 'integer', minimum: 0, maximum: 3500, default: 2500, 'x-applicable': true },
      },
    },
    relay: {
      type: 'object',
      title: 'Relay',
      properties: {
        default_state: { type: 'string', enum: ['off', 'on', 'last'], default: 'off', 'x-applicable': false },
      },
    },
  },
}

function mountEditor(modelValue: Record<string, any>, extra: Record<string, any> = {}) {
  return mount(ConfigEditor, {
    props: { modelValue, deviceId: 7, ...extra },
  })
}

function lastModel(wrapper: ReturnType<typeof mountEditor>) {
  const events = wrapper.emitted('update:modelValue') || []
  return events[events.length - 1]?.[0]
}

function lastValid(wrapper: ReturnType<typeof mountEditor>) {
  const events = wrapper.emitted('update:valid') || []
  return events[events.length - 1]?.[0]
}

async function openSection(wrapper: ReturnType<typeof mountEditor>, section: string) {
  const header = wrapper.find(`[data-section="${section}"] .section-header`)
  if (!wrapper.find(`[data-section="${section}"] .section-content`).exists()) {
    await header.trigger('click')
  }
}

describe('ConfigEditor', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getConfigSchema).mockReset()
    vi.mocked(getConfigSchema).mockResolvedValue(schema)
  })

  it('asks for the schema of the device', async () => {
    mountEditor({})
    await flushPromises()
    expect(getConfigSchema).toHaveBeenCalledWith(undefined, 7, undefined)
  })

  it('says in plain text that a section apply never writes is not applied', async () => {
    const wrapper = mountEditor({})
    await flushPromises()
    await openSection(wrapper, 'relay')
    await openSection(wrapper, 'power_metering')
    expect(wrapper.find('[data-section="relay"] [data-testid="section-not-applied"]').text())
      .toContain('Not applied to the device')
    expect(wrapper.find('[data-section="power_metering"] [data-testid="section-not-applied"]').exists())
      .toBe(false)
  })

  it('reloads the schema when the template scope or model changes', async () => {
    const wrapper = mount(ConfigEditor, { props: { modelValue: {}, scope: 'global' } })
    await flushPromises()
    expect(getConfigSchema).toHaveBeenLastCalledWith(undefined, undefined, 'global')
    await wrapper.setProps({ scope: undefined, deviceType: 'SHPLG-S' })
    await flushPromises()
    expect(getConfigSchema).toHaveBeenLastCalledWith('SHPLG-S', undefined, undefined)
  })

  it('renders a nested object as fields, not a JSON box', async () => {
    const wrapper = mountEditor({})
    await flushPromises()
    await openSection(wrapper, 'system')
    expect(wrapper.find('fieldset[data-path="system.device"]').exists()).toBe(true)
    expect(wrapper.find('[data-path="system.device.name"] input[type="text"]').exists()).toBe(true)
    expect(wrapper.find('[data-path="system.device.eco_mode"] input[type="checkbox"]').exists()).toBe(true)
    expect(wrapper.find('[data-section="system"] textarea').exists()).toBe(false)
  })

  it('shows the inherited value and its source', async () => {
    const wrapper = mountEditor({}, {
      inherited: { system: { device: { name: 'tv-office' } } },
      sources: { 'system.device.name': 'Global' },
    })
    await flushPromises()
    await openSection(wrapper, 'system')
    expect(wrapper.find('[data-path="system.device.name"] .field-inherited').text()).toBe('Inherited: tv-office from Global')
  })

  it('reset on a field that a template and a saved override both set shows the template value', async () => {
    // The page passes the template layers alone, so the override under the field hides nothing.
    const wrapper = mountEditor({ power_metering: { max_power: 2000 } }, {
      inherited: { power_metering: { max_power: 1800 } },
      sources: { 'power_metering.max_power': 'Global' },
    })
    await flushPromises()
    await openSection(wrapper, 'power_metering')
    const field = '[data-path="power_metering.max_power"]'
    expect(wrapper.find(`${field} .field-inherited`).exists()).toBe(false)
    await wrapper.find(`${field} [data-action="reset"]`).trigger('click')
    expect(lastModel(wrapper)).toEqual({})
    await wrapper.setProps({ modelValue: lastModel(wrapper) })
    expect(wrapper.find(`${field} .field-inherited`).text()).toBe('Inherited: 1800 from Global')
  })

  it('setting the field to the template value, under a saved override, leaves no override', async () => {
    const wrapper = mountEditor({ power_metering: { max_power: 2000 } }, {
      inherited: { power_metering: { max_power: 1800 } },
      sources: { 'power_metering.max_power': 'Global' },
    })
    await flushPromises()
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('1800')
    expect(lastModel(wrapper)).toEqual({})
  })

  it('reset on the last saved override keeps checking the section against what it inherits', async () => {
    // A template turns authentication on and gives no password; the saved override holds one, as the
    // mask. Reset leaves {}, and saving {} would delete the password with authentication still on.
    const authSchema: ConfigSchema = {
      type: 'object',
      'x-device-type': 'SHPLG-S',
      'x-apply-supported': true,
      properties: {
        auth: {
          type: 'object',
          title: 'Auth',
          properties: {
            enable: { type: 'boolean', 'x-applicable': true },
            user: { type: 'string', 'x-applicable': true },
            pass: { type: 'string', 'x-applicable': true },
          },
          if: { properties: { enable: { const: true } } },
          then: { required: ['user', 'pass'] },
        },
      },
    }
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    const wrapper = mountEditor({ auth: { pass: '********' } }, {
      held: { auth: { pass: '********' } },
      inherited: { auth: { enable: true, user: 'admin' } },
      sources: { 'auth.enable': 'Global', 'auth.user': 'Global' },
    })
    await flushPromises()
    expect(lastValid(wrapper)).toBe(true)
    await openSection(wrapper, 'auth')
    await wrapper.find('[data-path="auth.pass"] [data-action="reset"]').trigger('click')
    expect(lastModel(wrapper)).toEqual({})
    await wrapper.setProps({ modelValue: lastModel(wrapper) })
    expect(lastValid(wrapper)).toBe(false)
    expect(wrapper.find('[data-path="auth.pass"]').text()).toContain('Required')

    // A new password makes it valid again.
    await wrapper.find('[data-path="auth.pass"] input').setValue('new-secret')
    expect(lastModel(wrapper)).toEqual({ auth: { pass: 'new-secret' } })
    await wrapper.setProps({ modelValue: lastModel(wrapper) })
    expect(lastValid(wrapper)).toBe(true)
  })

  it('a section nobody touched is not checked when the template leaves it incomplete', async () => {
    const authSchema: ConfigSchema = {
      type: 'object',
      'x-device-type': 'SHPLG-S',
      'x-apply-supported': true,
      properties: {
        auth: {
          type: 'object',
          title: 'Auth',
          properties: {
            enable: { type: 'boolean', 'x-applicable': true },
            pass: { type: 'string', 'x-applicable': true },
          },
          if: { properties: { enable: { const: true } } },
          then: { required: ['pass'] },
        },
      },
    }
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    const wrapper = mountEditor({}, { inherited: { auth: { enable: true } } })
    await flushPromises()
    expect(lastValid(wrapper)).toBe(true)
  })

  it('a network switch the server omits shows as off, and an override of false is kept', async () => {
    // The server no longer sends `enable: false` for a Network field nobody set. The form must read
    // the missing key as off, and an explicit false must stay in the override body.
    const networkSchema: ConfigSchema = {
      type: 'object',
      'x-device-type': 'SHPLG-S',
      'x-apply-supported': true,
      properties: {
        network: {
          type: 'object',
          title: 'Network',
          properties: {
            wifi: {
              type: 'object',
              title: 'Wifi',
              properties: {
                sta: {
                  type: 'object',
                  title: 'Sta',
                  properties: {
                    enable: { type: 'boolean', 'x-applicable': true },
                    ssid: { type: 'string', 'x-applicable': true },
                  },
                },
              },
            },
          },
        },
      },
    }
    vi.mocked(getConfigSchema).mockResolvedValue(networkSchema)
    const path = '[data-path="network.wifi.sta.enable"] input[type="checkbox"]'

    const wrapper = mountEditor({}, { inherited: { network: { wifi: { sta: { ssid: 'home' } } } } })
    await flushPromises()
    await openSection(wrapper, 'network')
    expect((wrapper.find(path).element as HTMLInputElement).checked).toBe(false)

    // An inherited true is shown on, and switching it off is an override of false.
    const inheritedOn = mountEditor({}, { inherited: { network: { wifi: { sta: { enable: true } } } } })
    await flushPromises()
    await openSection(inheritedOn, 'network')
    expect((inheritedOn.find(path).element as HTMLInputElement).checked).toBe(true)
    await inheritedOn.find(path).setValue(false)
    expect(lastModel(inheritedOn)).toEqual({ network: { wifi: { sta: { enable: false } } } })
  })

  it('puts only the changed field into the override body', async () => {
    const wrapper = mountEditor({}, { inherited: { system: { device: { name: 'tv-office' } } } })
    await flushPromises()
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('2000')
    expect(lastModel(wrapper)).toEqual({ power_metering: { max_power: 2000 } })
  })

  it('reset removes the key and its empty section from the override body', async () => {
    const wrapper = mountEditor({ power_metering: { max_power: 2000 }, system: { device: { name: 'x' } } })
    await flushPromises()
    await wrapper.find('[data-path="power_metering.max_power"] [data-action="reset"]').trigger('click')
    expect(lastModel(wrapper)).toEqual({ system: { device: { name: 'x' } } })
  })

  it('use default sets the schema default', async () => {
    const wrapper = mountEditor({})
    await flushPromises()
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] [data-action="use-default"]').trigger('click')
    expect(lastModel(wrapper)).toEqual({ power_metering: { max_power: 2500 } })
  })

  it('an out-of-range number is an error and makes the editor invalid', async () => {
    const wrapper = mountEditor({})
    await flushPromises()
    expect(lastValid(wrapper)).toBe(true)
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('9000')
    expect(wrapper.find('[data-path="power_metering.max_power"] .field-error').text()).toBe('Must be at most 3500')
    expect(lastValid(wrapper)).toBe(false)
  })

  it('a bad time server, MQTT server or CoIoT peer is an error under the field before Save', async () => {
    vi.mocked(getConfigSchema).mockResolvedValue({
      type: 'object',
      'x-device-type': 'SHPLG-S',
      'x-apply-supported': true,
      properties: {
        system: {
          type: 'object',
          title: 'System',
          properties: {
            sntp: {
              type: 'object',
              title: 'Time Server',
              properties: { server: { type: 'string', format: 'hostname', 'x-applicable': true } },
            },
          },
        },
        mqtt: {
          type: 'object',
          title: 'MQTT',
          properties: { server: { type: 'string', format: 'endpoint', 'x-applicable': true } },
        },
        coiot: {
          type: 'object',
          title: 'CoIoT',
          properties: { peer: { type: 'string', format: 'hostport', 'x-applicable': true } },
        },
      },
    } as ConfigSchema)
    const wrapper = mountEditor({})
    await flushPromises()
    const cases = [
      { section: 'system', path: 'system.sntp.server', bad: 'not a host!', message: 'Must be a host name or an IPv4 address', good: 'time.google.com' },
      {
        section: 'mqtt',
        path: 'mqtt.server',
        bad: '192.168.1.300',
        message: 'Must be a host name or an IP address, optionally followed by :port (an IPv6 address takes its port in the Port field)',
        good: 'broker.local:1883',
      },
      {
        section: 'coiot',
        path: 'coiot.peer',
        bad: '192.168.1.5:99999',
        message: 'Must be host:port, with a host name or IPv4 address and a port from 1 to 65535',
        good: '192.168.1.5:5683',
      },
    ]
    for (const c of cases) {
      await openSection(wrapper, c.section)
      const input = wrapper.find(`[data-path="${c.path}"] input`)
      await input.setValue(c.bad)
      expect(wrapper.find(`[data-path="${c.path}"] .field-error`).text(), c.path).toBe(c.message)
      expect(lastValid(wrapper), c.path).toBe(false)
      await input.setValue(c.good)
      expect(wrapper.find(`[data-path="${c.path}"] .field-error`).exists(), c.path).toBe(false)
      expect(lastValid(wrapper), c.path).toBe(true)
    }
  })

  it('setting a field back to its inherited value leaves no override', async () => {
    const wrapper = mountEditor({}, {
      inherited: { power_metering: { max_power: 1800 } },
      sources: { 'power_metering.max_power': 'Global' },
    })
    await flushPromises()
    await openSection(wrapper, 'power_metering')
    const input = wrapper.find('[data-path="power_metering.max_power"] input')
    await input.setValue('2000')
    expect(lastModel(wrapper)).toEqual({ power_metering: { max_power: 2000 } })
    await input.setValue('1800')
    expect(lastModel(wrapper)).toEqual({})
  })

  it('typing through the inherited value keeps the typed text in the box', async () => {
    const wrapper = mountEditor({}, {
      inherited: { system: { device: { name: 'plug' } }, power_metering: { max_power: 18 } },
      sources: { 'system.device.name': 'Global', 'power_metering.max_power': 'Global' },
    })
    await flushPromises()
    await openSection(wrapper, 'system')
    await openSection(wrapper, 'power_metering')
    const name = wrapper.find('[data-path="system.device.name"] input')
    // A keystroke fires input only; setValue would also fire change, which ends the draft.
    const typeInto = async (input: typeof name, text: string) => {
      ;(input.element as HTMLInputElement).value = text
      await input.trigger('input')
    }
    await typeInto(name, 'plug')
    expect(lastModel(wrapper)).toEqual({})
    expect((name.element as HTMLInputElement).value).toBe('plug')
    await typeInto(name, 'plug2')
    expect(lastModel(wrapper)).toEqual({ system: { device: { name: 'plug2' } } })
    const power = wrapper.find('[data-path="power_metering.max_power"] input')
    await typeInto(power, '18')
    expect(lastModel(wrapper)).toEqual({ system: { device: { name: 'plug2' } } })
    expect((power.element as HTMLInputElement).value).toBe('18')
    await typeInto(power, '180')
    expect(lastModel(wrapper)).toEqual({ system: { device: { name: 'plug2' } }, power_metering: { max_power: 180 } })
  })

  it('a model changed from outside replaces the typed text in the box', async () => {
    const wrapper = mountEditor({}, {
      inherited: { system: { device: { name: 'plug' } }, power_metering: { max_power: 18 } },
      sources: { 'system.device.name': 'Global', 'power_metering.max_power': 'Global' },
    })
    await flushPromises()
    await openSection(wrapper, 'system')
    await openSection(wrapper, 'power_metering')
    const name = wrapper.find('[data-path="system.device.name"] input')
    const power = wrapper.find('[data-path="power_metering.max_power"] input')
    ;(name.element as HTMLInputElement).value = 'plug2'
    await name.trigger('input')
    ;(power.element as HTMLInputElement).value = '180'
    await power.trigger('input')
    await wrapper.setProps({ modelValue: { system: { device: { name: 'server' } }, power_metering: { max_power: 2500 } } })
    expect((name.element as HTMLInputElement).value).toBe('server')
    expect((power.element as HTMLInputElement).value).toBe('2500')
  })

  it('is not valid while the schema request is pending', async () => {
    vi.mocked(getConfigSchema).mockReturnValue(new Promise(() => {}))
    const wrapper = mountEditor({ power_metering: { max_power: 70000 } })
    await flushPromises()
    expect(lastValid(wrapper)).toBe(false)
  })

  it('a late schema response does not replace the schema of the current selection', async () => {
    const globalSchema: ConfigSchema = {
      type: 'object',
      'x-apply-supported': true,
      properties: { mqtt: { type: 'object', title: 'MQTT', properties: { server: { type: 'string', 'x-applicable': true } } } },
    }
    let resolvePlug!: (value: ConfigSchema) => void
    let resolveGlobal!: (value: ConfigSchema) => void
    vi.mocked(getConfigSchema)
      .mockReturnValueOnce(new Promise<ConfigSchema>((resolve) => { resolvePlug = resolve }))
      .mockReturnValueOnce(new Promise<ConfigSchema>((resolve) => { resolveGlobal = resolve }))
    const wrapper = mountEditor({}, { deviceId: undefined, deviceType: 'SHPLG-S' })
    await wrapper.setProps({ deviceType: undefined, scope: 'global' })
    // The global answer arrives first, the plug answer last.
    resolveGlobal(globalSchema)
    await flushPromises()
    resolvePlug(schema)
    await flushPromises()
    expect(wrapper.find('[data-section="mqtt"]').exists()).toBe(true)
    expect(wrapper.find('[data-section="relay"]').exists()).toBe(false)
  })

  it('a late failed schema response does not end the loading state of the current selection', async () => {
    let rejectPlug!: (reason: Error) => void
    vi.mocked(getConfigSchema)
      .mockReturnValueOnce(new Promise<ConfigSchema>((_, reject) => { rejectPlug = reject }))
      .mockReturnValueOnce(new Promise<ConfigSchema>(() => {}))
    const wrapper = mountEditor({}, { deviceId: undefined, deviceType: 'SHPLG-S' })
    await wrapper.setProps({ deviceType: undefined, scope: 'global' })
    rejectPlug(new Error('plug gone'))
    await flushPromises()
    // The current request is still pending: nothing is checked yet, and no error shows.
    expect(lastValid(wrapper)).toBe(false)
    expect(wrapper.find('.no-schema').exists()).toBe(false)
  })

  it('is valid again, and says so, when the schema request fails', async () => {
    vi.mocked(getConfigSchema).mockRejectedValue(new Error('boom'))
    const wrapper = mountEditor({})
    await flushPromises()
    expect(lastValid(wrapper)).toBe(true)
    expect(wrapper.find('.no-schema').text()).toContain('Changes are not validated here')
  })

  it('a corrected JSON error no longer blocks save', async () => {
    const wrapper = mountEditor({})
    await flushPromises()
    await wrapper.findAll('.mode-btn')[1].trigger('click')
    const box = wrapper.find('textarea')
    await box.setValue('{')
    expect(lastValid(wrapper)).toBe(false)
    await wrapper.findAll('.mode-btn')[0].trigger('click')
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('2000')
    expect(lastModel(wrapper)).toEqual({ power_metering: { max_power: 2000 } })
    expect(lastValid(wrapper)).toBe(true)
  })

  it('a section that is not an object makes the editor invalid', async () => {
    const wrapper = mountEditor({})
    await flushPromises()
    await wrapper.findAll('.mode-btn')[1].trigger('click')
    await wrapper.find('textarea').setValue('{"power_metering":"invalid"}')
    expect(lastValid(wrapper)).toBe(false)
  })

  // The required-field guard reads what the page loaded, never what the editor saw. A template
  // turns authentication on and gives no password; the saved override holds one as the mask, so
  // saving {} would delete it. No edit in the JSON view may make {} valid.
  describe('the required-field guard does not depend on editor memory', () => {
    const authSchema: ConfigSchema = {
      type: 'object',
      'x-device-type': 'SHPLG-S',
      'x-apply-supported': true,
      properties: {
        auth: {
          type: 'object',
          title: 'Auth',
          properties: {
            enable: { type: 'boolean', 'x-applicable': true },
            user: { type: 'string', 'x-applicable': true },
            pass: { type: 'string', 'x-applicable': true },
          },
          if: { properties: { enable: { const: true } } },
          then: { required: ['user', 'pass'] },
        },
      },
    }
    const saved = { auth: { pass: '********' } }
    const inherited = { auth: { enable: true, user: 'admin' } }

    // The editor as the page uses it: the page owns the saved body (`held`) and echoes every
    // update back as the model, the way v-model does.
    async function mountSaved() {
      vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
      const wrapper = mountEditor(saved, { inherited, held: saved })
      await flushPromises()
      expect(lastValid(wrapper)).toBe(true)
      return wrapper
    }
    async function echo(wrapper: ReturnType<typeof mountEditor>) {
      await wrapper.setProps({ modelValue: lastModel(wrapper) })
    }
    async function typeJson(wrapper: ReturnType<typeof mountEditor>, text: string) {
      await wrapper.find('textarea').setValue(text)
      await echo(wrapper)
    }
    async function openJson(wrapper: ReturnType<typeof mountEditor>) {
      await wrapper.findAll('.mode-btn')[1].trigger('click')
    }

    it.each(['null', '0', 'false', '""'])('a whole body of %s, then {}, keeps Save off', async (text) => {
      const wrapper = await mountSaved()
      await openJson(wrapper)
      await typeJson(wrapper, text)
      await typeJson(wrapper, '{}')
      expect(lastValid(wrapper)).toBe(false)
    })

    it.each(['null', '5', '[]', '"x"'])('the auth section set to %s, then {}, keeps Save off', async (value) => {
      const wrapper = await mountSaved()
      await openJson(wrapper)
      await typeJson(wrapper, `{"auth":${value}}`)
      await typeJson(wrapper, '{}')
      expect(lastValid(wrapper)).toBe(false)
    })

    it('Reset of the saved password keeps Save off', async () => {
      const wrapper = await mountSaved()
      await openSection(wrapper, 'auth')
      await wrapper.find('[data-path="auth.pass"] [data-action="reset"]').trigger('click')
      await echo(wrapper)
      expect(lastModel(wrapper)).toEqual({})
      expect(lastValid(wrapper)).toBe(false)
    })

    it('a new password after the whole body was cleared makes Save valid again', async () => {
      const wrapper = await mountSaved()
      await openJson(wrapper)
      await typeJson(wrapper, 'null')
      await typeJson(wrapper, '{"auth":{"pass":"new-secret"}}')
      expect(lastValid(wrapper)).toBe(true)
    })

    it('a new saved body from the page starts the guard again', async () => {
      const wrapper = await mountSaved()
      await openJson(wrapper)
      await typeJson(wrapper, '{}')
      expect(lastValid(wrapper)).toBe(false)
      // The page saved and reloaded: the stored override no longer holds a password.
      await wrapper.setProps({ held: {}, modelValue: {} })
      expect(lastValid(wrapper)).toBe(true)
    })

    // A reload that brings a password the editor had not seen: the page gives the new saved body, so
    // Reset of that password is guarded, with no earlier mount snapshot to go stale.
    it('a reload that brings a saved password guards its Reset', async () => {
      vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
      const wrapper = mountEditor({}, { inherited, held: {} })
      await flushPromises()
      expect(lastValid(wrapper)).toBe(true)
      await wrapper.setProps({ held: saved, modelValue: saved })
      await openSection(wrapper, 'auth')
      await wrapper.find('[data-path="auth.pass"] [data-action="reset"]').trigger('click')
      await echo(wrapper)
      expect(lastModel(wrapper)).toEqual({})
      expect(lastValid(wrapper)).toBe(false)
    })

    it('without held nothing is remembered, so no reload can leave a stale memory', async () => {
      vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
      const wrapper = mountEditor(saved, { inherited })
      await flushPromises()
      await openSection(wrapper, 'auth')
      await wrapper.find('[data-path="auth.pass"] [data-action="reset"]').trigger('click')
      await echo(wrapper)
      expect(lastValid(wrapper)).toBe(true)
    })
  })

  it('a field apply cannot write is read-only but its override can be reset', async () => {
    const wrapper = mountEditor({ relay: { default_state: 'on' } })
    await flushPromises()
    const field = wrapper.find('[data-path="relay.default_state"]')
    expect(field.find('select').attributes('disabled')).toBeDefined()
    expect(field.find('.field-note').text()).toContain('Apply cannot write this field')
    expect(field.find('[data-action="use-default"]').exists()).toBe(false)
    await field.find('[data-action="reset"]').trigger('click')
    expect(lastModel(wrapper)).toEqual({})
  })
})
