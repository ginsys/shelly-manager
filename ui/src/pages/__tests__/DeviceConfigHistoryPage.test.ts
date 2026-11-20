import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DeviceConfigHistoryPage from '../DeviceConfigHistoryPage.vue'

vi.mock('@/api/deviceConfig', () => ({
  getConfigHistory: vi.fn(),
}))

import { getConfigHistory } from '@/api/deviceConfig'

// ginsys/shelly-manager#747 slice 1: the page lists the history the server answers and shows an
// entry's configuration as fields. It offers no Restore: the call it made merged the entry into the
// stored row, and the history answer is masked, so no old secret could come back. A real restore is
// ginsys/shelly-manager#775.
const stubs = {
  RouterLink: true,
  ConfigView: {
    props: ['config'],
    template: '<div class="stub-view" data-testid="entry-view">{{ Object.keys(config || {}).join(",") }}</div>',
  },
}

const entry = {
  id: 1,
  deviceId: 7,
  config: { mqtt: { port: 1883 } },
  oldConfig: null,
  changes: null,
  source: 'manual',
  user: 'serge',
  timestamp: '2026-11-18T20:00:00Z',
}

async function openPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/devices/:id/config/history', component: DeviceConfigHistoryPage }],
  })
  router.push('/devices/7/config/history')
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs } })
  await flushPromises()
  return wrapper
}

describe('DeviceConfigHistoryPage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(getConfigHistory).mockResolvedValue([entry] as any)
  })

  it('lists the entries of the device', async () => {
    const wrapper = await openPage()

    expect(getConfigHistory).toHaveBeenCalledWith('7')
    const row = wrapper.get('tbody tr')
    expect(row.text()).toContain(new Date(entry.timestamp).toLocaleString())
    expect(row.text()).toContain('manual')
    expect(row.text()).toContain('serge')
    expect(wrapper.text()).not.toContain('No configuration history found')
  })

  it('shows the configuration of an entry as fields in the viewer', async () => {
    const wrapper = await openPage()

    await wrapper.findAll('button').find((b) => b.text() === 'View')!.trigger('click')

    expect(wrapper.get('.modal [data-testid="entry-view"]').text()).toBe('mqtt')
    expect(wrapper.find('.modal pre').exists()).toBe(false)
  })

  it('closes the viewer', async () => {
    const wrapper = await openPage()
    await wrapper.findAll('button').find((b) => b.text() === 'View')!.trigger('click')

    await wrapper.findAll('.modal button').find((b) => b.text() === 'Close')!.trigger('click')

    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it('offers no Restore, in the list or in the viewer', async () => {
    const wrapper = await openPage()
    expect(wrapper.text()).not.toContain('Restore')

    await wrapper.findAll('button').find((b) => b.text() === 'View')!.trigger('click')
    expect(wrapper.text()).not.toContain('Restore')
  })

  it('says there is no history when the list is empty', async () => {
    vi.mocked(getConfigHistory).mockResolvedValue([])
    const wrapper = await openPage()

    expect(wrapper.text()).toContain('No configuration history found')
  })

  it('shows the failure of the read', async () => {
    vi.mocked(getConfigHistory).mockRejectedValue(new Error('Unexpected configuration history response'))
    const wrapper = await openPage()

    expect(wrapper.get('.state.error').text()).toContain('Unexpected configuration history response')
  })

  it('reads the history again on Refresh', async () => {
    const wrapper = await openPage()
    vi.mocked(getConfigHistory).mockClear()

    await wrapper.findAll('button').find((b) => b.text() === 'Refresh')!.trigger('click')
    await flushPromises()

    expect(getConfigHistory).toHaveBeenCalledTimes(1)
  })
})
