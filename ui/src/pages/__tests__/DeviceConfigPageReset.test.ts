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

// ginsys/shelly-manager#385: Reset on the last saved override of a section leaves {}, and saving {}
// would delete a saved password while a template keeps authentication on. The page unmounts the
// editor while it loads (Verify Device does), so the memory that keeps the section checked must
// live in the page, not in the editor.
describe('DeviceConfigPage keeps the Reset memory across a reload of the editor', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getConfigSchema).mockResolvedValue(authSchema)
    api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
    api.getDeviceOverridesNew.mockImplementation(async () => ({ overrides: { auth: { pass: '********' } } }) as any)
    api.getDesiredConfigNew.mockImplementation(
      async () =>
        ({
          config: { auth: { enable: true, user: 'admin' } },
          sources: { 'auth.enable': 'Global', 'auth.user': 'Global' },
          inherited: { auth: { enable: true, user: 'admin' } },
          inherited_sources: { 'auth.enable': 'Global', 'auth.user': 'Global' },
        }) as any,
    )
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: true, pending_changes: false }) as any)
    api.verifyDeviceConfigNew.mockImplementation(async () => ({ match: true, differences: [] }) as any)
  })

  it('Verify Device after Reset leaves Save disabled', async () => {
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
    const save = () => wrapper.find('[data-testid="save-overrides"]')
    expect(save().attributes('disabled')).toBeUndefined()

    // The editor opens a section that holds an override.
    if (!wrapper.find('[data-section="auth"] .section-content').exists()) {
      await wrapper.find('[data-section="auth"] .section-header').trigger('click')
    }
    await wrapper.find('[data-path="auth.pass"] [data-action="reset"]').trigger('click')
    await flushPromises()
    expect(save().attributes('disabled')).toBeDefined()

    // Verify Device flips the store's loading flag, which removes and re-creates the editor with
    // the edited body.
    await button('Verify Device')!.trigger('click')
    await flushPromises()
    expect(api.verifyDeviceConfigNew).toHaveBeenCalled()
    expect(wrapper.find('[data-section="auth"]').exists()).toBe(true)
    expect(save().attributes('disabled')).toBeDefined()
  })
})
