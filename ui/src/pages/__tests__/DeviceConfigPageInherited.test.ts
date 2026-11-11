import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import * as configNewApi from '@/api/configNew'

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

const api = vi.mocked(configNewApi)

// ginsys/shelly-manager#385: the override editor needs what a field falls back to once its
// override is removed. The desired configuration cannot give that: where a template and an
// override set the same field, it holds only the override.
describe('DeviceConfigPage feeds the override editor the inherited configuration', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
    api.getDeviceOverridesNew.mockImplementation(async () => ({ overrides: { mqtt: { port: 8883 } } }) as any)
    api.getDesiredConfigNew.mockImplementation(
      async () =>
        ({
          config: { mqtt: { server: 'global.broker', port: 8883 } },
          sources: { 'mqtt.server': 'Global', 'mqtt.port': 'device-override' },
          inherited: { mqtt: { server: 'global.broker', port: 1883 } },
          inherited_sources: { 'mqtt.server': 'Global', 'mqtt.port': 'Global' },
        }) as any,
    )
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: true, pending_changes: false }) as any)
  })

  it('passes the template layers, not the desired configuration, to the editor', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
    })
    router.push('/devices/7/config')
    await router.isReady()
    const wrapper = mount(RouterView, {
      global: { plugins: [router], stubs: { ConfigEditor: true, ConfigView: true, RouterLink: true } },
    })
    await flushPromises()

    const edit = wrapper.findAll('button').find((b) => b.text().trim() === 'Edit')
    if (!edit) throw new Error('no Edit button')
    await edit.trigger('click')

    const editor = wrapper.findComponent({ name: 'ConfigEditor' })
    expect(editor.props('inherited')).toEqual({ mqtt: { server: 'global.broker', port: 1883 } })
    expect(editor.props('sources')).toEqual({ 'mqtt.server': 'Global', 'mqtt.port': 'Global' })
  })
})
