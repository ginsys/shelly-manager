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
    mqtt: {
      type: 'object',
      title: 'MQTT',
      properties: {
        server: { type: 'string', maxLength: 128, 'x-applicable': true },
        pass: { type: 'string', format: 'password', 'x-applicable': true },
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

  it('shows the inherited value and its source, without the saved override', async () => {
    const wrapper = mountEditor({}, {
      inherited: { system: { device: { name: 'tv-office' } }, power_metering: { max_power: 1800 } },
      sources: { 'system.device.name': 'Global', 'power_metering.max_power': 'device-override' },
    })
    await flushPromises()
    await openSection(wrapper, 'system')
    await openSection(wrapper, 'power_metering')
    expect(wrapper.find('[data-path="system.device.name"] .field-inherited').text()).toBe('Inherited: tv-office from Global')
    // max_power came from the saved override, so it is no baseline.
    expect(wrapper.find('[data-path="power_metering.max_power"] .field-inherited').exists()).toBe(false)
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
  describe('stored secrets (ginsys/shelly-manager#395)', () => {
    async function openMqtt(extra: Record<string, any> = {}, model: Record<string, any> = {}) {
      const wrapper = mountEditor(model, { storedSecrets: ['mqtt.pass'], ...extra })
      await flushPromises()
      await openSection(wrapper, 'mqtt')
      return wrapper
    }

    it('shows a stored password as kept and leaves it out of the body when untouched', async () => {
      const wrapper = await openMqtt()
      const pass = wrapper.find('[data-path="mqtt.pass"]')
      expect(pass.find('input').attributes('placeholder')).toBe('Stored, leave empty to keep')
      await wrapper.find('[data-path="mqtt.server"] input').setValue('broker.local')
      expect(lastModel(wrapper)).toEqual({ mqtt: { server: 'broker.local' } })
      expect(JSON.stringify(lastModel(wrapper))).not.toContain('pass')
    })

    it('an emptied box keeps the stored password, it does not send an empty string', async () => {
      const wrapper = await openMqtt()
      const input = wrapper.find('[data-path="mqtt.pass"] input')
      await input.setValue('typed')
      expect(lastModel(wrapper)).toEqual({ mqtt: { pass: 'typed' } })
      await input.setValue('')
      expect(lastModel(wrapper)).toEqual({})
    })

    it('clear sends an empty string on purpose, and reset goes back to keeping it', async () => {
      const wrapper = await openMqtt()
      const field = wrapper.find('[data-path="mqtt.pass"]')
      await field.find('[data-action="clear-secret"]').trigger('click')
      expect(lastModel(wrapper)).toEqual({ mqtt: { pass: '' } })
      expect(field.find('[data-note="clears-secret"]').exists()).toBe(true)
      expect(lastValid(wrapper)).toBe(true)

      await field.find('[data-action="reset"]').trigger('click')
      expect(lastModel(wrapper)).toEqual({})
      expect(field.find('[data-note="clears-secret"]').exists()).toBe(false)
    })

    it('a password field with no stored value has no hint and no clear action', async () => {
      const wrapper = await openMqtt({ storedSecrets: [] })
      const field = wrapper.find('[data-path="mqtt.pass"]')
      expect(field.find('input').attributes('placeholder') ?? '').not.toContain('Stored')
      expect(field.find('[data-action="clear-secret"]').exists()).toBe(false)
    })

    it('a stored secret does not change the text fields next to it', async () => {
      const wrapper = await openMqtt()
      const server = wrapper.find('[data-path="mqtt.server"]')
      expect(server.find('[data-action="clear-secret"]').exists()).toBe(false)
    })
  })
})
