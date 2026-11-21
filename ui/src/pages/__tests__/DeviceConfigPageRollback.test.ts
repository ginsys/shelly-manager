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
  return { wrapper, router, refresh, apply, verify }
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
    const { wrapper, refresh, apply, verify } = await mountPage()
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

  // The overrides watch rewrites an open edit on every refresh, so a rollback would drop it silently.
  it('keeps Roll back off while the overrides editor is open, and on again once it is closed', async () => {
    const { wrapper } = await mountPage()
    const toggle = wrapper.findAll('button').find((b) => b.text() === 'Edit')!
    await toggle.trigger('click')
    expect(wrapper.get('[data-testid="rollback-5"]').attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find((b) => b.text() === 'Cancel')!.trigger('click')
    expect(wrapper.get('[data-testid="rollback-5"]').attributes('disabled')).toBeUndefined()
  })

  // The dialog does not trap the keyboard and the admin-key prompt replays the POST, so an edit could be started
  // while the request is in flight and its answer's refresh would overwrite it.
  it('keeps the overrides editor shut while a rollback request is in flight, and frees it after, also when the device changes', async () => {
    let release: (v: unknown) => void = () => {}
    vi.mocked(rollbackDesired).mockReturnValueOnce(new Promise((resolve) => (release = resolve)) as any)
    const { wrapper, router } = await mountPage()
    const edit = () => wrapper.findAll('button').find((b) => b.text() === 'Edit')!
    expect(edit().attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="rollback-5"]').trigger('click')
    await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
    expect(edit().attributes('disabled')).toBeDefined()
    expect(edit().attributes('title')).toBe('A rollback is running')
    release({ version_id: 9, no_change: false, detached_template_ids: [], message: 'done' })
    await flushPromises()
    expect(edit().attributes('disabled')).toBeUndefined()

    vi.mocked(rollbackDesired).mockReturnValueOnce(new Promise(() => {}) as any)
    await wrapper.get('[data-testid="rollback-5"]').trigger('click')
    await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
    expect(edit().attributes('disabled')).toBeDefined()
    await router.push('/devices/8/config')
    await flushPromises()
    expect(edit().attributes('disabled')).toBeUndefined()
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
