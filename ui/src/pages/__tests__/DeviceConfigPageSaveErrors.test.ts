import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import * as configNewApi from '@/api/configNew'
import { TemplateSaveError } from '@/api/templates'
import type { ConfigSchema } from '@/api/typedConfig'

vi.mock('@/api/configNew', () => ({
  getDeviceTemplatesNew: vi.fn(),
  setDeviceTemplatesNew: vi.fn(),
  addDeviceTemplateNew: vi.fn(),
  removeDeviceTemplateNew: vi.fn(),
  getDeviceOverridesNew: vi.fn(),
  setDeviceOverridesNew: vi.fn(),
  patchDeviceOverridesNew: vi.fn(),
  deleteDeviceOverridesNew: vi.fn(),
  getDesiredConfigNew: vi.fn(),
  getConfigStatusNew: vi.fn(),
  applyDeviceConfigNew: vi.fn(),
  verifyDeviceConfigNew: vi.fn(),
}))

vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

import { getConfigSchema } from '@/api/typedConfig'

const api = vi.mocked(configNewApi)

const mqttSchema: ConfigSchema = {
  type: 'object',
  'x-device-type': 'SHPLG-S',
  'x-apply-supported': true,
  properties: {
    mqtt: {
      type: 'object',
      title: 'MQTT',
      properties: {
        port: { type: 'integer', 'x-applicable': true },
      },
    },
  },
}

// ginsys/shelly-manager#522: the server now refuses an override value outside the device schema.
// The page lists each refused field beside the editor and keeps the edit open, as the template
// page does, instead of replacing the page with an error card.
describe('DeviceConfigPage shows the fields the server refused on Save', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getConfigSchema).mockResolvedValue(mqttSchema)
    api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
    api.getDeviceOverridesNew.mockImplementation(async () => ({ overrides: { mqtt: { port: 1883 } } }) as any)
    api.getDesiredConfigNew.mockImplementation(
      async () => ({ config: { mqtt: { port: 1883 } }, sources: {}, inherited: {}, inherited_sources: {} }) as any,
    )
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: true, pending_changes: false }) as any)
  })

  async function openEditor() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
    })
    router.push('/devices/7/config')
    await router.isReady()
    const wrapper = mount(RouterView, {
      global: { plugins: [router], stubs: { ConfigView: true, RouterLink: true } },
    })
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text().trim() === 'Edit')!.trigger('click')
    await flushPromises()
    return wrapper
  }

  it('lists the refused fields, keeps the editor open and shows no error card', async () => {
    api.setDeviceOverridesNew.mockRejectedValue(
      new TemplateSaveError('Validation failed', [
        { path: 'mqtt.port', message: 'Must be at most 65535' },
        { path: 'location.lng', message: 'Must be at most 180' },
      ]),
    )
    const alert = vi.spyOn(window, 'alert').mockImplementation(() => {})
    const wrapper = await openEditor()

    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()

    const list = wrapper.find('[data-testid="override-save-errors"]')
    expect(list.exists()).toBe(true)
    expect(list.text()).toContain('mqtt.port')
    expect(list.text()).toContain('Must be at most 65535')
    expect(list.text()).toContain('location.lng')
    expect(wrapper.find('.card.state.error').exists()).toBe(false)
    expect(wrapper.find('[data-testid="save-overrides"]').exists()).toBe(true)
    expect(alert).not.toHaveBeenCalled()
  })

  it('clears the list on the next Save and on Cancel', async () => {
    api.setDeviceOverridesNew.mockRejectedValueOnce(
      new TemplateSaveError('Validation failed', [{ path: 'mqtt.port', message: 'Must be at most 65535' }]),
    )
    api.setDeviceOverridesNew.mockResolvedValue({ overrides: { mqtt: { port: 1883 } } } as any)
    const wrapper = await openEditor()

    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="override-save-errors"]').exists()).toBe(true)

    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="override-save-errors"]').exists()).toBe(false)
  })

  // The server names a relay by its position in the body it was sent. Only relay 1 is in the override,
  // so relay 1 is item 0 there. The mark stays under relay 1 after an edit of relay 0 and a remount
  // of the editor (Verify Device removes and re-creates it).
  it('keeps a refusal under the relay it names after another relay is edited and the editor remounts', async () => {
    vi.mocked(getConfigSchema).mockResolvedValue({
      type: 'object',
      'x-device-type': 'SHSW-25',
      'x-apply-supported': true,
      properties: {
        relay: {
          type: 'object',
          title: 'Relay',
          properties: {
            relays: {
              type: 'array',
              items: {
                type: 'object',
                properties: {
                  id: { type: 'integer', 'x-applicable': true },
                  name: { type: 'string', 'x-applicable': true },
                },
              },
            },
          },
        },
      },
    } as ConfigSchema)
    const inherited = { relay: { relays: [{ id: 0, name: 'Lamp' }, { id: 1, name: 'Fan' }] } }
    api.getDeviceOverridesNew.mockImplementation(async () => ({ overrides: { relay: { relays: [{ id: 1, name: 'bad' }] } } }) as any)
    api.getDesiredConfigNew.mockImplementation(
      async () => ({ config: inherited, sources: {}, inherited, inherited_sources: {} }) as any,
    )
    api.verifyDeviceConfigNew.mockImplementation(async () => ({ match: true, differences: [] }) as any)
    api.setDeviceOverridesNew.mockRejectedValue(
      new TemplateSaveError('Validation failed', [{ path: 'relay.relays.0.name', message: 'name is refused' }]),
    )
    const wrapper = await openEditor()
    const mark = (id: number) => wrapper.find(`[data-path="relay.relays.${id}.name"] .field-error`)

    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()
    expect(mark(1).text()).toBe('name is refused')

    await wrapper.find('[data-path="relay.relays.0.name"] input').setValue('Lamp 2')
    await wrapper.findAll('button').find((b) => b.text().trim() === 'Verify Device')!.trigger('click')
    await flushPromises()
    expect(api.verifyDeviceConfigNew).toHaveBeenCalled()
    expect(wrapper.find('[data-path="relay.relays.0.name"] input').exists()).toBe(true)
    expect(mark(1).text()).toBe('name is refused')
    expect(mark(0).exists()).toBe(false)
  })

  // Preparing the body can throw on a body that is no object; the failure belongs in the error panel.
  it('a body that cannot be prepared for Save shows the error panel and sends nothing', async () => {
    api.getDeviceOverridesNew.mockImplementation(async () => ({ overrides: { mqtt: { port: 1883, pass: '********' } } }) as any)
    const wrapper = await openEditor()
    await wrapper.findAll('.mode-btn')[1].trigger('click')
    await wrapper.find('textarea').setValue('5')
    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(true)
    expect(api.setDeviceOverridesNew).not.toHaveBeenCalled()
  })

  it('a refusal that is not a field list shows an error panel, not a field list or an alert', async () => {
    api.setDeviceOverridesNew.mockRejectedValue(new Error('device not found'))
    const alert = vi.spyOn(window, 'alert').mockImplementation(() => {})
    const wrapper = await openEditor()

    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()

    const panel = wrapper.find('[data-testid="config-action-error"]')
    expect(panel.exists()).toBe(true)
    expect(panel.text()).toContain('device not found')
    expect(panel.text()).toContain('Save overrides')
    expect(wrapper.find('[data-testid="override-save-errors"]').exists()).toBe(false)
    expect(alert).not.toHaveBeenCalled()
  })
})
