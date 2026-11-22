import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'
import type { ConfigSchema } from '@/api/typedConfig'

vi.mock('@/api/deviceConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/deviceConfig')>()
  return { ...actual, getStoredDeviceConfig: vi.fn(), getConfigHistory: vi.fn() }
})
vi.mock('@/api/desiredRollback', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/desiredRollback')>()
  return { ...actual, getDesiredHistory: vi.fn(), rollbackDesired: vi.fn() }
})
vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

import { getStoredDeviceConfig, getConfigHistory } from '@/api/deviceConfig'
import { getDesiredHistory } from '@/api/desiredRollback'
import { getConfigSchema } from '@/api/typedConfig'

// ginsys/shelly-manager#858: text in the overrides editor's JSON view that does not parse yet survives a page
// refresh. The page removes and re-creates the editor while it loads, so the page keeps the draft. The editor
// here is the real one; a refresh is simulated the way the rollback tests do, by toggling the store's loading.
const stubs = { 'router-link': true, ConfigView: true }
enableAutoUnmount(afterEach)

const schema: ConfigSchema = {
  type: 'object',
  'x-device-type': 'SHPLG-S',
  'x-apply-supported': true,
  properties: {
    power_metering: {
      type: 'object',
      title: 'Power Metering',
      properties: {
        max_power: { type: 'integer', minimum: 0, maximum: 3500, default: 2500, 'x-applicable': true },
      },
    },
  },
}

const DRAFT = '{"mqtt": {'
const SAVED = { power_metering: { max_power: 2000 } }

async function mountPage(id = '7') {
  const store = useDeviceConfigNewStore()
  vi.spyOn(store, 'refresh').mockResolvedValue(undefined as never)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
  })
  router.push(`/devices/${id}/config`)
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs }, attachTo: document.body })
  await flushPromises()
  // The mocked refresh reads nothing: say the page has read its state and its overrides, as a real refresh does.
  store.status = { config_applied: true, pending_changes: false } as any
  store.overrides = JSON.parse(JSON.stringify(SAVED))
  await flushPromises()
  return { wrapper, router, store }
}

const button = (wrapper: any, label: string) => wrapper.findAll('button').find((b: any) => b.text() === label)!
const textarea = (wrapper: any) => wrapper.find('textarea')
const saveOff = (wrapper: any) => wrapper.get('[data-testid="save-overrides"]').attributes('disabled') !== undefined

// Opens the edit, switches to the JSON view and types the draft.
async function typeDraft(wrapper: any, text = DRAFT) {
  await button(wrapper, 'Edit').trigger('click')
  await flushPromises()
  await wrapper.findAll('.mode-btn')[1].trigger('click')
  await textarea(wrapper).setValue(text)
}

// A page refresh: the store sets loading, the body shows "Loading...", the editor is removed, then it comes back.
async function refresh(store: any, outcome: 'ok' | 'fails' = 'ok') {
  store.loading = true
  await flushPromises()
  if (outcome === 'fails') store.error = 'Failed to load new config state'
  store.loading = false
  await flushPromises()
}

describe('DeviceConfigPage unparsed overrides draft', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(null)
    vi.mocked(getConfigHistory).mockResolvedValue([])
    vi.mocked(getDesiredHistory).mockResolvedValue([] as any)
    vi.mocked(getConfigSchema).mockResolvedValue(schema)
  })

  // check 1 and 2
  it('keeps text that does not parse through a page refresh, with Save off before and after', async () => {
    const { wrapper, store } = await mountPage()
    await typeDraft(wrapper)
    expect(saveOff(wrapper)).toBe(true)

    await refresh(store)

    expect(textarea(wrapper).exists()).toBe(true)
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe(DRAFT)
    expect(wrapper.find('.json-error').exists()).toBe(true)
    expect(saveOff(wrapper)).toBe(true)
  })

  // check 3: Cancel
  it('Cancel drops the draft: the next Edit shows the saved overrides', async () => {
    const { wrapper, store } = await mountPage()
    await typeDraft(wrapper)
    await refresh(store)

    await button(wrapper, 'Cancel').trigger('click')
    await button(wrapper, 'Edit').trigger('click')
    await flushPromises()

    expect(textarea(wrapper).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('draft')).toBeFalsy()
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('modelValue')).toEqual(SAVED)
    expect(saveOff(wrapper)).toBe(false)
  })

  // check 3: Clear All. A Save ends the edit the same way but cannot hold a draft at all: Save returns while the
  // editor is invalid, and a parse empties the draft, so there is no case for it that could fail.
  it('Clear All drops the draft', async () => {
    const { wrapper, store } = await mountPage()
    vi.spyOn(store, 'clearOverrides').mockImplementation((async () => {
      store.overrides = {}
    }) as never)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    await typeDraft(wrapper)
    await refresh(store)

    await button(wrapper, 'Clear All').trigger('click')
    await flushPromises()
    await button(wrapper, 'Edit').trigger('click')
    await flushPromises()

    expect(textarea(wrapper).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('draft')).toBeFalsy()
  })

  // check 4
  it('never carries a draft from one device to the next', async () => {
    const { wrapper, router, store } = await mountPage('7')
    await typeDraft(wrapper)
    await refresh(store)

    await router.push('/devices/8/config')
    await flushPromises()
    store.status = { config_applied: true, pending_changes: false } as any
    store.overrides = { power_metering: { max_power: 1000 } }
    await flushPromises()
    await button(wrapper, 'Edit').trigger('click')
    await flushPromises()

    expect(textarea(wrapper).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('draft')).toBeFalsy()
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('modelValue')).toEqual({ power_metering: { max_power: 1000 } })
  })

  // check 5 is the existing editor and page tests, unchanged; this one pins the valid path through a refresh.
  it('still keeps a JSON edit that parses through a refresh', async () => {
    const { wrapper, store } = await mountPage()
    await typeDraft(wrapper, '{"power_metering": {"max_power": 1500}}')
    await refresh(store)
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('modelValue')).toEqual({ power_metering: { max_power: 1500 } })
    expect(wrapper.findComponent({ name: 'ConfigEditor' }).props('draft')).toBeFalsy()
  })

  // check 6
  it('keeps the draft when the refresh fails, and gives it back at the next good load', async () => {
    const { wrapper, store } = await mountPage()
    await typeDraft(wrapper)

    await refresh(store, 'fails')
    expect(wrapper.text()).toContain('Failed to load new config state')
    expect(textarea(wrapper).exists()).toBe(false)

    store.error = null
    await flushPromises()
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe(DRAFT)
    expect(saveOff(wrapper)).toBe(true)
  })
})
