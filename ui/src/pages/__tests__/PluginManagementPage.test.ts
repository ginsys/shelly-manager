import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import PluginManagementPage from '../PluginManagementPage.vue'
import type { Plugin, PluginSchema } from '@/api/plugin'

// Real getPluginCategoryInfo (used in the template); mock the network calls.
vi.mock('@/api/plugin', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/plugin')>()
  return { ...actual, listPlugins: vi.fn(), getPluginSchema: vi.fn() }
})

import { listPlugins, getPluginSchema } from '@/api/plugin'

function plugin(name: string, display: string): Plugin {
  return {
    name,
    display_name: display,
    description: `${display} desc`,
    version: '1.0.0',
    category: 'backup',
    capabilities: ['x'],
    status: { available: true, configured: true, enabled: true },
  }
}

const schemaFor = (name: string): PluginSchema => ({
  type: 'object',
  title: `schema-${name}`,
  properties: { [name]: { type: 'string' } },
})

// Stub the async schema viewer so we can read which schema it received.
const stubs = {
  PluginSchemaViewer: {
    props: ['schema'],
    template: '<div class="stub-schema">{{ schema?.title }}</div>',
  },
  PluginDetailsView: true,
}

describe('PluginManagementPage — schema modal race (#264)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(listPlugins).mockResolvedValue({
      plugins: [plugin('aaa', 'Aaa'), plugin('bbb', 'Bbb')],
      categories: [{ name: 'backup', display_name: 'Backup', description: '', plugin_count: 2 }],
      meta: undefined,
    })
  })

  it('does not render an earlier plugin\'s schema when a later one is opened first', async () => {
    // Control resolution order per plugin.
    const resolvers: Record<string, (s: PluginSchema) => void> = {}
    vi.mocked(getPluginSchema).mockImplementation(
      (name: string | number) =>
        new Promise<PluginSchema>(res => { resolvers[String(name)] = res })
    )

    const wrapper = mount(PluginManagementPage, { global: { stubs } })
    await flushPromises() // let listPlugins resolve, render cards

    const viewButtons = wrapper.findAll('button').filter(b => b.text().includes('View schema'))
    expect(viewButtons).toHaveLength(2)

    // Open Aaa, then Bbb — both requests in flight.
    await viewButtons[0].trigger('click')
    await viewButtons[1].trigger('click')

    // Aaa resolves LATE, after Bbb was opened: must be ignored.
    resolvers['aaa'](schemaFor('aaa'))
    await flushPromises()
    expect(wrapper.text()).not.toContain('schema-aaa')

    // Bbb resolves: its schema is shown, under the Bbb heading.
    resolvers['bbb'](schemaFor('bbb'))
    await flushPromises()
    expect(wrapper.find('.stub-schema').text()).toBe('schema-bbb')
    expect(wrapper.text()).toContain('Bbb — configuration schema')
    expect(wrapper.text()).not.toContain('schema-aaa')
  })

  it('closing the modal invalidates an in-flight schema request', async () => {
    const resolvers: Record<string, (s: PluginSchema) => void> = {}
    vi.mocked(getPluginSchema).mockImplementation(
      (name: string | number) =>
        new Promise<PluginSchema>(res => { resolvers[String(name)] = res })
    )

    const wrapper = mount(PluginManagementPage, { global: { stubs } })
    await flushPromises()

    const viewButtons = wrapper.findAll('button').filter(b => b.text().includes('View schema'))
    await viewButtons[0].trigger('click')

    // Close before the schema resolves, then let it resolve.
    await wrapper.findAll('button').find(b => b.text() === '×')!.trigger('click')
    resolvers['aaa'](schemaFor('aaa'))
    await flushPromises()

    // Modal is closed and no stale schema leaked into the DOM.
    expect(wrapper.find('.stub-schema').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('schema-aaa')
  })
})
