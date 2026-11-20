import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'

vi.mock('@/api/deviceConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/deviceConfig')>()
  return { ...actual, getStoredDeviceConfig: vi.fn(), getConfigHistory: vi.fn() }
})

import { getStoredDeviceConfig, getConfigHistory } from '@/api/deviceConfig'

// ginsys/shelly-manager#747 slice 2: "Create template from this import" sits on the stored import card.
const stubs = {
  'router-link': true,
  ConfigEditor: true,
  ConfigView: { props: ['config'], template: '<div />' },
  CreateTemplateFromImportDialog: {
    props: ['deviceId'],
    emits: ['close'],
    template:
      '<div data-testid="stub-dialog">device {{ deviceId }}<button data-testid="stub-close" @click="$emit(\'close\')">x</button></div>',
  },
}

const stored = {
  id: 3,
  device_id: 7,
  template_id: null,
  config: { mqtt: { user: 'iot' } },
  last_synced: '2026-11-20T09:00:00Z',
  sync_status: 'synced',
  created_at: '2026-11-19T09:00:00Z',
  updated_at: '2026-11-20T09:30:00Z',
}

async function mountPage() {
  const store = useDeviceConfigNewStore()
  vi.spyOn(store, 'refresh').mockResolvedValue(undefined as never)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
  })
  router.push('/devices/7/config')
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs } })
  await flushPromises()
  return wrapper
}

describe('DeviceConfigPage create template from import', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(stored as any)
    vi.mocked(getConfigHistory).mockResolvedValue([])
  })

  it('shows the button only when an import of settings is stored', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="create-template-from-import"]').exists()).toBe(true)

    vi.mocked(getStoredDeviceConfig).mockResolvedValue(null)
    const empty = await mountPage()
    expect(empty.find('[data-testid="create-template-from-import"]').exists()).toBe(false)

    vi.mocked(getStoredDeviceConfig).mockResolvedValue({ ...stored, config: 'not settings' } as any)
    const text = await mountPage()
    expect(text.find('[data-testid="create-template-from-import"]').exists()).toBe(false)
  })

  it('opens the dialog for the device on screen, and closes it again', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="stub-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-testid="create-template-from-import"]').trigger('click')

    expect(wrapper.get('[data-testid="stub-dialog"]').text()).toContain('device 7')

    await wrapper.get('[data-testid="stub-close"]').trigger('click')
    expect(wrapper.find('[data-testid="stub-dialog"]').exists()).toBe(false)
  })
})
