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

// ginsys/shelly-manager#747 slice 1: the page shows the configuration an import stored, and the
// history of that configuration, as fields.
const stubs = {
  'router-link': true,
  ConfigEditor: true,
  ConfigView: {
    props: ['config'],
    template: '<div class="stub-view" data-testid="stub-view">{{ Object.keys(config || {}).join(",") }}</div>',
  },
}

const stored = {
  id: 3,
  device_id: 7,
  template_id: null,
  config: { mqtt: { user: 'iot' }, wifi: { ssid: 'office' } },
  last_synced: '2026-11-20T09:00:00Z',
  sync_status: 'synced',
  created_at: '2026-11-19T09:00:00Z',
  updated_at: '2026-11-20T09:30:00Z',
}

const entry = (over: Record<string, unknown> = {}) => ({
  id: 11,
  deviceId: 7,
  timestamp: '2026-11-20T10:00:00Z',
  source: 'manual',
  user: 'user',
  config: { relays: { a: 1 } },
  oldConfig: null,
  changes: null,
  ...over,
})

async function mountPage(id = '7', hash = '', refresh?: (store: ReturnType<typeof useDeviceConfigNewStore>) => Promise<void>) {
  const store = useDeviceConfigNewStore()
  vi.spyOn(store, 'refresh').mockImplementation((refresh ? () => refresh(store) : async () => undefined) as never)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
  })
  router.push(`/devices/${id}/config${hash}`)
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs }, attachTo: document.body })
  await flushPromises()
  return { wrapper, router, store }
}

describe('DeviceConfigPage last imported configuration', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(null)
    vi.mocked(getConfigHistory).mockResolvedValue([])
  })

  it('shows the stored import as fields, with its import time and status', async () => {
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(stored as any)
    const { wrapper } = await mountPage()

    expect(getStoredDeviceConfig).toHaveBeenCalledWith('7')
    expect(wrapper.get('[data-testid="stored-card"]').text()).toContain('Last imported configuration')
    const card = wrapper.get('[data-testid="stored-config"]')
    expect(card.get('[data-testid="stub-view"]').text()).toBe('mqtt,wifi')
    expect(card.get('[data-testid="stored-config-time"]').text()).toBe(new Date(stored.last_synced).toLocaleString())
    expect(card.get('[data-testid="stored-config-status"]').text()).toContain('synced')
    expect(card.find('pre').exists()).toBe(false)
  })

  // The stored configuration can be changed after the import (PUT /devices/{id}/config keeps
  // last_synced), so the card names both times and never reads an edit as the import.
  it('names the import time and the time of the last change apart', async () => {
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(stored as any)
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="stored-config-time"]').text()).toBe(new Date(stored.last_synced).toLocaleString())
    expect(wrapper.get('[data-testid="stored-config-changed"]').text()).toBe(new Date(stored.updated_at).toLocaleString())
    expect(stored.last_synced).not.toBe(stored.updated_at)
  })

  it('says the import time is unknown when the row has none, and still shows the change time', async () => {
    vi.mocked(getStoredDeviceConfig).mockResolvedValue({ ...stored, last_synced: null } as any)
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="stored-config-time"]').text()).toBe('unknown')
    expect(wrapper.get('[data-testid="stored-config-changed"]').text()).toBe(new Date(stored.updated_at).toLocaleString())
  })

  it('says nothing was imported when the device has no stored configuration', async () => {
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="stored-config-empty"]').text()).toContain('Nothing was imported')
    expect(wrapper.find('[data-testid="stored-config"]').exists()).toBe(false)
  })

  it('shows a failed read in the card, not as an empty state', async () => {
    vi.mocked(getStoredDeviceConfig).mockRejectedValue(new Error('database is locked'))
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="stored-config-error"]').text()).toContain('database is locked')
    expect(wrapper.find('[data-testid="stored-config-empty"]').exists()).toBe(false)
  })

  it('shows a configuration that came as text as text', async () => {
    vi.mocked(getStoredDeviceConfig).mockResolvedValue({ ...stored, config: '********' } as any)
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="stored-config"] pre').text()).toBe('********')
  })

  it('drops the import of device 7 when the page moves to device 8, and reads device 8', async () => {
    vi.mocked(getStoredDeviceConfig).mockImplementation(async (id: any) =>
      id === '7' ? (stored as any) : null,
    )
    const { wrapper, router } = await mountPage()
    expect(wrapper.find('[data-testid="stored-config"]').exists()).toBe(true)

    await router.push('/devices/8/config')
    await flushPromises()

    expect(getStoredDeviceConfig).toHaveBeenLastCalledWith('8')
    expect(wrapper.find('[data-testid="stored-config"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="stored-config-empty"]').exists()).toBe(true)
  })

  it('ignores an import of device 7 that answers after the page moved to device 8', async () => {
    let answerSeven: (v: any) => void = () => {}
    vi.mocked(getStoredDeviceConfig).mockImplementation((id: any) =>
      id === '7' ? new Promise((resolve) => (answerSeven = resolve)) : Promise.resolve(null),
    )
    const { wrapper, router } = await mountPage()

    await router.push('/devices/8/config')
    await flushPromises()
    answerSeven(stored)
    await flushPromises()

    expect(wrapper.find('[data-testid="stored-config"]').exists()).toBe(false)
  })
})

describe('DeviceConfigPage history', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(null)
    vi.mocked(getConfigHistory).mockResolvedValue([])
  })

  it('lists each entry with its time, action and who changed it', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([entry(), entry({ id: 10, source: 'updated', user: 'api' })] as any)
    const { wrapper } = await mountPage()

    expect(getConfigHistory).toHaveBeenCalledWith('7')
    const rows = wrapper.findAll('[data-testid="history-entry"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain(new Date('2026-11-20T10:00:00Z').toLocaleString())
    expect(rows[0].text()).toContain('manual')
    expect(rows[0].text()).toContain('user')
    expect(rows[1].text()).toContain('updated')
    expect(rows[1].text()).toContain('api')
  })

  it('opens an entry to its configuration as fields, and closes it again', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([entry()] as any)
    const { wrapper } = await mountPage()
    const history = () => wrapper.get('[data-testid="history-card"]')

    expect(history().find('[data-testid="stub-view"]').exists()).toBe(false)

    await wrapper.get('[data-testid="history-toggle-11"]').trigger('click')
    expect(history().get('[data-testid="stub-view"]').text()).toBe('relays')
    expect(history().find('pre').exists()).toBe(false)

    await wrapper.get('[data-testid="history-toggle-11"]').trigger('click')
    expect(history().find('[data-testid="stub-view"]').exists()).toBe(false)
  })

  it('opens one entry without opening the others', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([entry(), entry({ id: 10, config: { wifi: {} } })] as any)
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="history-toggle-10"]').trigger('click')

    const views = wrapper.findAll('[data-testid="history-card"] [data-testid="stub-view"]')
    expect(views).toHaveLength(1)
    expect(views[0].text()).toBe('wifi')
  })

  it('says an entry holds no configuration when it recorded none', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([entry({ config: null })] as any)
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="history-toggle-11"]').trigger('click')

    expect(wrapper.get('[data-testid="history-card"]').text()).toContain('No configuration was recorded')
  })

  it('shows a configuration that came as text as text', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([entry({ config: 'not json: mqtt on' })] as any)
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="history-toggle-11"]').trigger('click')

    const card = wrapper.get('[data-testid="history-card"]')
    expect(card.get('pre').text()).toBe('not json: mqtt on')
    expect(card.find('[data-testid="stub-view"]').exists()).toBe(false)
  })

  it('says there is no history when the list is empty', async () => {
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="history-empty"]').text()).toContain('No history')
  })

  it('shows a failed read in the card and leaves the rest of the page alone', async () => {
    vi.mocked(getConfigHistory).mockRejectedValue(new Error('Forbidden'))
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(stored as any)
    const { wrapper } = await mountPage()

    expect(wrapper.get('[data-testid="history-error"]').text()).toContain('Forbidden')
    expect(wrapper.find('[data-testid="stored-config"]').exists()).toBe(true)
  })

  it('drops the history of device 7 when the page moves to device 8', async () => {
    vi.mocked(getConfigHistory).mockImplementation(async (id: any) => (id === '7' ? ([entry()] as any) : []))
    const { wrapper, router } = await mountPage()
    expect(wrapper.findAll('[data-testid="history-entry"]')).toHaveLength(1)

    await router.push('/devices/8/config')
    await flushPromises()

    expect(getConfigHistory).toHaveBeenLastCalledWith('8')
    expect(wrapper.findAll('[data-testid="history-entry"]')).toHaveLength(0)
  })

  // Two refreshes of one device can overlap: the older answer must not replace the newer one.
  it('keeps the answer of the newest refresh when an older one answers last', async () => {
    const pending: Array<{ stored: (v: any) => void; history: (v: any) => void }> = []
    vi.mocked(getStoredDeviceConfig).mockImplementation(
      () => new Promise((resolve) => { (pending[pending.length] ??= {} as any).stored = resolve }),
    )
    vi.mocked(getConfigHistory).mockImplementation(
      () => new Promise((resolve) => { (pending[pending.length - 1] as any).history = resolve }),
    )
    const { wrapper } = await mountPage() // refresh A, still open
    const refresh = () => wrapper.findAll('button').find((b) => b.text() === 'Refresh')!
    await refresh().trigger('click') // refresh B
    await flushPromises()
    expect(pending).toHaveLength(2)

    pending[1].stored(stored)
    pending[1].history([entry()])
    await flushPromises()
    pending[0].stored(null)
    pending[0].history([])
    await flushPromises()

    expect(wrapper.find('[data-testid="stored-config"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="stored-config-empty"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="history-entry"]')).toHaveLength(1)
  })

  it('reloads both cards on Refresh', async () => {
    const { wrapper } = await mountPage()
    vi.mocked(getStoredDeviceConfig).mockClear()
    vi.mocked(getConfigHistory).mockClear()

    await wrapper.findAll('button').find((b) => b.text() === 'Refresh')!.trigger('click')
    await flushPromises()

    expect(getStoredDeviceConfig).toHaveBeenCalledTimes(1)
    expect(getConfigHistory).toHaveBeenCalledTimes(1)
  })

  it('offers no Restore: the call it would make merges into the stored row and cannot bring back a secret', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([entry()] as any)
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="history-toggle-11"]').trigger('click')

    expect(wrapper.text()).not.toContain('Restore')
  })
})

// "View History" and the old history address open the page at #history-card (ginsys/shelly-manager#775).
describe('DeviceConfigPage landing at the History card', () => {
  const scrolled = vi.fn()

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(null)
    vi.mocked(getConfigHistory).mockResolvedValue([entry()] as any)
    scrolled.mockReset()
    Element.prototype.scrollIntoView = scrolled
  })

  it('gives the card the id that the address names', async () => {
    const { wrapper } = await mountPage('7', '#history-card')
    expect(wrapper.get('[data-testid="history-card"]').attributes('id')).toBe('history-card')
  })

  it('moves to the card once its history has been read', async () => {
    const { wrapper } = await mountPage('7', '#history-card')
    expect(scrolled).toHaveBeenCalledTimes(1)
    expect(scrolled.mock.contexts[0]).toBe(wrapper.get('[data-testid="history-card"]').element)
  })

  it('moves to the card when the history read failed, so the error is in view', async () => {
    vi.mocked(getConfigHistory).mockRejectedValue(new Error('Forbidden'))
    await mountPage('7', '#history-card')
    expect(scrolled).toHaveBeenCalledTimes(1)
  })

  it('does not move again at a later read while the address keeps the hash', async () => {
    const { wrapper } = await mountPage('7', '#history-card')
    expect(scrolled).toHaveBeenCalledTimes(1)

    await wrapper.findAll('button').find((b) => b.text() === 'Refresh')!.trigger('click')
    await flushPromises()

    expect(getConfigHistory).toHaveBeenCalledTimes(2)
    expect(scrolled).toHaveBeenCalledTimes(1)
  })

  it('moves once more for the next device when the page is moved to it at the card', async () => {
    const { router } = await mountPage('7', '#history-card')
    expect(scrolled).toHaveBeenCalledTimes(1)

    await router.push('/devices/8/config#history-card')
    await flushPromises()

    expect(getConfigHistory).toHaveBeenLastCalledWith('8')
    expect(scrolled).toHaveBeenCalledTimes(2)
  })

  // The page's own read fills the cards above the History card, and they push it down: the landing waits
  // for them, whichever of the reads answers first.
  it('waits for the page read when the history answers first, then moves once', async () => {
    let finish: () => void = () => {}
    const { store } = await mountPage('7', '#history-card', async (s) => {
      s.loading = true
      await new Promise<void>((resolve) => (finish = resolve))
      s.loading = false
    })
    expect(getConfigHistory).toHaveBeenCalledTimes(1)
    expect(scrolled).not.toHaveBeenCalled()
    expect(store.loading).toBe(true)

    finish()
    await flushPromises()

    expect(scrolled).toHaveBeenCalledTimes(1)
  })

  it('waits for the history when the page read answers first, then moves once', async () => {
    let release: (rows: unknown) => void = () => {}
    vi.mocked(getConfigHistory).mockReturnValueOnce(new Promise((resolve) => (release = resolve)) as any)
    await mountPage('7', '#history-card', async (s) => {
      s.loading = true
      await Promise.resolve()
      s.loading = false
    })
    expect(scrolled).not.toHaveBeenCalled()

    release([entry()])
    await flushPromises()

    expect(scrolled).toHaveBeenCalledTimes(1)
  })

  it('moves when the page read ends in failure, after the history has answered', async () => {
    let fail: () => void = () => {}
    await mountPage('7', '#history-card', async (s) => {
      s.loading = true
      await new Promise<void>((_, reject) => (fail = () => reject(new Error('down'))))
        .finally(() => (s.loading = false))
    })
    vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(scrolled).not.toHaveBeenCalled()

    fail()
    await flushPromises()

    expect(scrolled).toHaveBeenCalledTimes(1)
  })

  it('does not move a page opened with no hash', async () => {
    await mountPage('7')
    expect(scrolled).not.toHaveBeenCalled()
  })

  it('does not move for an answer of an older read that a newer one replaced', async () => {
    let releaseFirst: (rows: unknown) => void = () => {}
    vi.mocked(getConfigHistory)
      .mockReturnValueOnce(new Promise((resolve) => (releaseFirst = resolve)) as any)
      .mockResolvedValue([entry()] as any)
    const { wrapper } = await mountPage('7', '#history-card')
    expect(scrolled).not.toHaveBeenCalled()

    await wrapper.findAll('button').find((b) => b.text() === 'Refresh')!.trigger('click')
    await flushPromises()
    expect(scrolled).toHaveBeenCalledTimes(1)

    releaseFirst([entry({ id: 99 })])
    await flushPromises()
    expect(scrolled).toHaveBeenCalledTimes(1)
  })
})
