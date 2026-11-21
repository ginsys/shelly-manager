import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'

vi.mock('@/api/deviceConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/deviceConfig')>()
  return { ...actual, getStoredDeviceConfig: vi.fn(), getConfigHistory: vi.fn() }
})
vi.mock('@/api/desiredRollback', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/desiredRollback')>()
  return { ...actual, getDesiredHistory: vi.fn(), rollbackDesired: vi.fn() }
})

import { getStoredDeviceConfig, getConfigHistory } from '@/api/deviceConfig'
import { getDesiredHistory, rollbackDesired, RollbackRefused } from '@/api/desiredRollback'

// ginsys/shelly-manager#775 slice 3: the rollback card on the page. A rollback changes the desired
// configuration only: the page reads everything again and never applies or verifies.
const stubs = { 'router-link': true, ConfigEditor: true, ConfigView: true }
const SENTINEL = 'sentinel-secret-9f3a'
enableAutoUnmount(afterEach)

const row = (id: number) => ({
  id,
  created_at: '2026-11-20T10:00:00Z',
  action: 'recompute',
  changed_by: 'user',
  restored_from: null,
  new_desired: '{"mqtt":{"server":"old"}}',
})

async function mountPage(id = '7') {
  const store = useDeviceConfigNewStore()
  const refresh = vi.spyOn(store, 'refresh').mockResolvedValue(undefined as never)
  const apply = vi.spyOn(store, 'apply').mockResolvedValue({ success: true } as never)
  const verify = vi.spyOn(store, 'verify').mockResolvedValue(undefined as never)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
  })
  router.push(`/devices/${id}/config`)
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs }, attachTo: document.body })
  await flushPromises()
  // The mocked refresh reads nothing: say the page has read its state, as a real refresh does.
  store.status = { config_applied: true, pending_changes: false } as any
  await flushPromises()
  return { wrapper, router, refresh, apply, verify, store }
}

describe('DeviceConfigPage rollback card', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(getStoredDeviceConfig).mockResolvedValue(null)
    vi.mocked(getConfigHistory).mockResolvedValue([])
    vi.mocked(getDesiredHistory).mockResolvedValue([row(5)] as any)
  })

  it('shows the card beside the History card, and reads the versions of the device on screen', async () => {
    const { wrapper } = await mountPage()
    expect(getDesiredHistory).toHaveBeenCalledWith('7')
    expect(wrapper.find('[data-testid="versions-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="history-card"]').exists()).toBe(true)
  })

  it('reads the desired configuration, status, overrides, templates and versions again after a rollback, and never applies or verifies', async () => {
    vi.mocked(rollbackDesired).mockResolvedValue({ version_id: 9, no_change: false, detached_template_ids: [], message: 'done' } as any)
    const { wrapper, refresh, apply, verify, store } = await mountPage()
    // the real store sets loading around its read: that is the card's cue to read its versions
    refresh.mockImplementation((async () => {
      store.loading = true
      await Promise.resolve()
      store.loading = false
    }) as never)
    const loads = { refresh: refresh.mock.calls.length, versions: vi.mocked(getDesiredHistory).mock.calls.length }

    await wrapper.get('[data-testid="rollback-5"]').trigger('click')
    await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
    await flushPromises()

    expect(rollbackDesired).toHaveBeenCalledWith('7', 5, undefined)
    // store.refresh reads templates, overrides, desired configuration and status in one call
    expect(refresh.mock.calls.length).toBe(loads.refresh + 1)
    expect(refresh).toHaveBeenLastCalledWith('7')
    expect(vi.mocked(getDesiredHistory).mock.calls.length).toBe(loads.versions + 1)
    expect(apply).not.toHaveBeenCalled()
    expect(verify).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain('done')
  })

  // The card sits outside the part of the page that waits for its data, so it must wait for it itself.
  it('keeps Roll back off while the page is reading, or when its read failed or gave nothing, and reads the versions again after each read', async () => {
    const { wrapper, store } = await mountPage()
    const off = () => wrapper.get('[data-testid="rollback-5"]').attributes('disabled') !== undefined
    const reads = () => vi.mocked(getDesiredHistory).mock.calls.length
    expect(off()).toBe(false)

    const before = reads()
    store.loading = true
    await flushPromises()
    expect(off()).toBe(true)
    expect(reads()).toBe(before)
    store.loading = false
    await flushPromises()
    expect(off()).toBe(false)
    expect(reads()).toBe(before + 1)

    store.error = 'Failed to load new config state'
    await flushPromises()
    expect(off()).toBe(true)
    store.error = null
    store.status = null
    await flushPromises()
    expect(off()).toBe(true)
  })

  // The real store sets loading around a refresh, which is what tells the card to read its versions.
  describe('the versions after a rollback', () => {
    const answer = { version_id: 9, no_change: false, detached_template_ids: [], message: 'done' } as any
    const reads = () => vi.mocked(getDesiredHistory).mock.calls.length
    const succeeds = (store: any) => async () => {
      store.error = null
      store.loading = true
      await Promise.resolve()
      store.loading = false
    }
    const fails = (store: any) => async () => {
      store.error = 'Failed to load new config state'
      store.loading = true
      await Promise.resolve()
      store.loading = false
      throw new Error('read failed')
    }

    it('reads them exactly once, through the page refresh', async () => {
      vi.mocked(rollbackDesired).mockResolvedValue(answer)
      const { wrapper, refresh, store } = await mountPage()
      refresh.mockImplementation(succeeds(store) as never)
      const before = reads()
      await wrapper.get('[data-testid="rollback-5"]').trigger('click')
      await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
      await flushPromises()
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain('done')
      expect(reads()).toBe(before + 1)
    })

    it('reads them exactly once after a 409 without paths too', async () => {
      vi.mocked(rollbackDesired).mockRejectedValue(new RollbackRefused(409, 'X', 'Changed while the rollback was prepared.', []))
      const { wrapper, refresh, store } = await mountPage()
      refresh.mockImplementation(succeeds(store) as never)
      const before = reads()
      await wrapper.get('[data-testid="rollback-5"]').trigger('click')
      await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
      await flushPromises()
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain('Changed while the rollback was prepared.')
      expect(reads()).toBe(before + 1)
    })

    // Save clears the store's loading itself and the page then refreshes, so the card is told twice; the second
    // read may fail, and the list it showed stays.
    it('keeps the list after a Save whose second read of the versions fails', async () => {
      const { wrapper, refresh, store } = await mountPage()
      refresh.mockImplementation(succeeds(store) as never)
      vi.spyOn(store, 'saveOverrides').mockImplementation((async () => {
        store.loading = true
        await Promise.resolve()
        store.loading = false
      }) as never)
      vi.mocked(getDesiredHistory).mockResolvedValueOnce([row(6), row(5)] as any).mockRejectedValueOnce(new Error('second read failed'))
      await wrapper.findAll('button').find((b) => b.text() === 'Edit')!.trigger('click')
      await wrapper.get('[data-testid="save-overrides"]').trigger('click')
      await flushPromises()
      const card = wrapper.get('[data-testid="versions-card"]')
      expect(card.findAll('[data-testid="version-entry"]')).toHaveLength(2)
      expect(card.get('[data-testid="versions-error"]').text()).toContain('second read failed')
    })

    it('keeps the success notice and Roll back off when the page refresh fails, and reads once after the next good Refresh', async () => {
      vi.mocked(rollbackDesired).mockResolvedValue(answer)
      vi.spyOn(console, 'error').mockImplementation(() => {})
      const { wrapper, refresh, store } = await mountPage()
      refresh.mockImplementationOnce(fails(store) as never)
      const before = reads()
      await wrapper.get('[data-testid="rollback-5"]').trigger('click')
      await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
      await flushPromises()
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain('done')
      expect(wrapper.get('[data-testid="rollback-5"]').attributes('disabled')).toBeDefined()
      expect(reads()).toBe(before)

      refresh.mockImplementation(succeeds(store) as never)
      await wrapper.findAll('button').find((b) => b.text() === 'Refresh')!.trigger('click')
      await flushPromises()
      expect(reads()).toBe(before + 1)
      expect(wrapper.get('[data-testid="rollback-5"]').attributes('disabled')).toBeUndefined()
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain('done')
    })
  })

  // A rollback replaces the overrides, so it is not started over an open edit.
  it('keeps Roll back off while the overrides editor is open, and on again once it is closed', async () => {
    const { wrapper } = await mountPage()
    const toggle = wrapper.findAll('button').find((b) => b.text() === 'Edit')!
    await toggle.trigger('click')
    expect(wrapper.get('[data-testid="rollback-5"]').attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find((b) => b.text() === 'Cancel')!.trigger('click')
    expect(wrapper.get('[data-testid="rollback-5"]').attributes('disabled')).toBeUndefined()
  })

  // ginsys/shelly-manager#855: a refresh never rewrites an open overrides edit.
  describe('the overrides editor', () => {
    const page = (wrapper: any) => wrapper.findComponent(DeviceConfigPage).vm as any
    const button = (wrapper: any, label: string) => wrapper.findAll('button').find((b: any) => b.text() === label)
    const openEditor = async (wrapper: any, typed: Record<string, any>) => {
      await button(wrapper, 'Edit')!.trigger('click')
      page(wrapper).overridesData = typed
      await flushPromises()
    }
    const typed = { mqtt: { server: 'typed' } }

    it('keeps an open edit through Refresh and Apply, and through a change of the store that holds the same overrides', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true)
      const { wrapper } = await mountPage()
      const store = useDeviceConfigNewStore()
      store.overrides = { mqtt: { server: 'saved' } } as any
      await flushPromises()
      await openEditor(wrapper, typed)

      await button(wrapper, 'Refresh')!.trigger('click')
      await flushPromises()
      await page(wrapper).handleApply()
      await flushPromises()
      store.overrides = { mqtt: { server: 'saved' } } as any
      await flushPromises()

      expect(page(wrapper).overridesData).toEqual(typed)
      expect(page(wrapper).overridesHeld).toEqual({ mqtt: { server: 'saved' } })
      expect(wrapper.find('[data-testid="overrides-stale"]').exists()).toBe(false)
    })

    it('shows a rollback that landed under the open editor as a notice, keeps the edit, refuses Save and lets Cancel load it', async () => {
      const { wrapper } = await mountPage()
      const store = useDeviceConfigNewStore()
      const save = vi.spyOn(store, 'saveOverrides').mockResolvedValue(undefined as never)
      store.overrides = { mqtt: { server: 'saved' } } as any
      await flushPromises()
      await openEditor(wrapper, typed)

      store.overrides = { mqtt: { server: 'rolled-back' } } as any
      await flushPromises()
      expect(wrapper.get('[data-testid="overrides-stale"]').text()).toContain('saved overrides changed')
      expect(wrapper.get('[data-testid="save-overrides"]').attributes('disabled')).toBeDefined()
      expect(page(wrapper).overridesData).toEqual(typed)
      await page(wrapper).handleSaveOverrides()
      expect(save).not.toHaveBeenCalled()

      await button(wrapper, 'Cancel')!.trigger('click')
      expect(page(wrapper).overridesData).toEqual({ mqtt: { server: 'rolled-back' } })
      expect(wrapper.find('[data-testid="overrides-stale"]').exists()).toBe(false)
    })

    it('does not call two objects with the same members in another order a change', async () => {
      const { wrapper } = await mountPage()
      const store = useDeviceConfigNewStore()
      store.overrides = { a: 1, b: { c: 2, d: 3 } } as any
      await flushPromises()
      await openEditor(wrapper, typed)
      store.overrides = { b: { d: 3, c: 2 }, a: 1 } as any
      await flushPromises()
      expect(wrapper.find('[data-testid="overrides-stale"]').exists()).toBe(false)
      expect(wrapper.get('[data-testid="save-overrides"]').attributes('disabled')).toBeUndefined()
      // the edit and its baseline were not replaced by the reordered saved overrides
      expect(page(wrapper).overridesData).toEqual(typed)
      expect(page(wrapper).overridesHeld).toEqual({ a: 1, b: { c: 2, d: 3 } })
    })

    // With saved overrides a device change reloads through the watch anyway, so only empty ones tell a guard in
    // loadOverrides (which would keep the old edit) from a guard at the refresh callers.
    it('drops the typed edit on a device change and on Cancel when no overrides are saved', async () => {
      const { wrapper, router } = await mountPage()
      await openEditor(wrapper, typed)
      await router.push('/devices/8/config')
      await flushPromises()
      expect(page(wrapper).overridesData).toEqual({})
      expect(page(wrapper).editOverrides).toBe(false)

      await openEditor(wrapper, typed)
      await button(wrapper, 'Cancel')!.trigger('click')
      expect(page(wrapper).overridesData).toEqual({})
    })

    it('leaves the buffer equal to the saved overrides after a Save whose refresh fails', async () => {
      const { wrapper, refresh } = await mountPage()
      const store = useDeviceConfigNewStore()
      vi.spyOn(store, 'saveOverrides').mockImplementation(async () => {
        store.overrides = { mqtt: { server: 'typed', normalised: true } } as any
      })
      refresh.mockRejectedValue(new Error('read failed'))
      vi.spyOn(console, 'error').mockImplementation(() => {})
      await openEditor(wrapper, typed)
      await wrapper.get('[data-testid="save-overrides"]').trigger('click')
      await flushPromises()
      expect(page(wrapper).editOverrides).toBe(false)
      expect(page(wrapper).overridesData).toEqual({ mqtt: { server: 'typed', normalised: true } })
      expect(page(wrapper).overridesHeld).toEqual({ mqtt: { server: 'typed', normalised: true } })
    })

    it('closes the editor and loads the cleared overrides after Clear All, which is a confirmed discard', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true)
      const { wrapper } = await mountPage()
      const store = useDeviceConfigNewStore()
      store.overrides = { mqtt: { server: 'saved' } } as any
      vi.spyOn(store, 'clearOverrides').mockImplementation(async () => {
        store.overrides = null
      })
      await flushPromises()
      await openEditor(wrapper, typed)
      await button(wrapper, 'Clear All')!.trigger('click')
      await flushPromises()
      expect(page(wrapper).editOverrides).toBe(false)
      expect(page(wrapper).overridesData).toEqual({})
    })
  })

  it('leaves nothing of the old device on the new one: a late answer fills nothing and the typed secrets are gone', async () => {
    let release: (v: unknown) => void = () => {}
    vi.mocked(rollbackDesired)
      .mockRejectedValueOnce(new RollbackRefused(409, 'X', 'This version holds secrets.', ['mqtt.pass']))
      .mockReturnValueOnce(new Promise((resolve) => (release = resolve)) as any)
    vi.mocked(getDesiredHistory).mockImplementation(async (id: number | string) => (String(id) === '7' ? [row(5)] : [row(50)]) as any)
    const { wrapper, router } = await mountPage()

    await wrapper.get('[data-testid="rollback-5"]').trigger('click')
    await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue(SENTINEL)
    await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')

    await router.push('/devices/8/config')
    await flushPromises()
    expect(getDesiredHistory).toHaveBeenCalledWith('8')
    release({ version_id: 9, no_change: false, detached_template_ids: [], message: 'Desired configuration set to version 9.' })
    await flushPromises()

    const card = wrapper.get('[data-testid="versions-card"]')
    expect(card.text()).toContain('Version 50')
    expect(card.text()).not.toContain('Version 5 ')
    expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(false)
    // the late success is a result for the app notice, not a notice on the card of the new device
    expect(wrapper.find('[data-testid="rollback-notice"]').exists()).toBe(false)
    expect(document.body.innerHTML).not.toContain(SENTINEL)
    expect([...document.querySelectorAll('input, textarea')].map((i) => (i as HTMLInputElement).value)).not.toContain(SENTINEL)
  })
})
