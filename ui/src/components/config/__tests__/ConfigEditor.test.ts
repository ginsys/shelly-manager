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
    expect(getConfigSchema).toHaveBeenCalledWith(undefined, 7)
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
})
