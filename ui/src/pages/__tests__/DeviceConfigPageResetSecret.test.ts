import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import * as configNewApi from '@/api/configNew'
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
        user: { type: 'string', 'x-applicable': true },
        pass: { type: 'string', format: 'password', 'x-applicable': true },
      },
    },
  },
}

// The server keeps a secret that a saved body leaves out, so Reset on an override secret has to send
// null (remove the override, inherit the template again). Omitting the key would do nothing.
describe('DeviceConfigPage sends null for a secret override that Reset removed', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getConfigSchema).mockResolvedValue(mqttSchema)
    api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
    api.getDeviceOverridesNew.mockImplementation(
      async () => ({ overrides: { mqtt: { user: 'iot', pass: '********' } } }) as any,
    )
    api.getDesiredConfigNew.mockImplementation(
      async () =>
        ({
          config: { mqtt: { user: 'iot', pass: '********' } },
          sources: { 'mqtt.user': 'Device', 'mqtt.pass': 'Device' },
          inherited: { mqtt: { user: 'tpl', pass: '********' } },
          inherited_sources: { 'mqtt.user': 'Global', 'mqtt.pass': 'Global' },
        }) as any,
    )
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: true, pending_changes: false }) as any)
    api.setDeviceOverridesNew.mockImplementation(async () => ({ overrides: {} }) as any)
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
    const button = (label: string) => wrapper.findAll('button').find((b) => b.text().trim() === label)
    await button('Edit')!.trigger('click')
    await flushPromises()
    if (!wrapper.find('[data-section="mqtt"] .section-content').exists()) {
      await wrapper.find('[data-section="mqtt"] .section-header').trigger('click')
    }
    return wrapper
  }

  it('Reset on the saved password saves null at its path and leaves the other fields alone', async () => {
    const wrapper = await openEditor()

    await wrapper.find('[data-path="mqtt.pass"] [data-action="reset"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()

    expect(api.setDeviceOverridesNew).toHaveBeenCalledTimes(1)
    expect(api.setDeviceOverridesNew.mock.calls[0][1]).toEqual({ mqtt: { user: 'iot', pass: null } })
  })

  it('a save that does not touch the password sends the mask, never null', async () => {
    const wrapper = await openEditor()

    await wrapper.find('[data-path="mqtt.user"] input').setValue('iot2')
    await flushPromises()
    await wrapper.find('[data-testid="save-overrides"]').trigger('click')
    await flushPromises()

    expect(api.setDeviceOverridesNew.mock.calls[0][1]).toEqual({ mqtt: { user: 'iot2', pass: '********' } })
  })
})
