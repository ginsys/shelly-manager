import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PluginDetailsView from '../PluginDetailsView.vue'
import type { Plugin } from '@/api/plugin'

// The plugin store is only used for getPluginStatusClass (the view is read-only).
const getPluginStatusClass = (status: Plugin['status']) => {
  if (!status.available) return 'unavailable'
  if (status.error) return 'error'
  if (!status.configured) return 'not-configured'
  if (!status.enabled) return 'disabled'
  return 'ready'
}

vi.mock('@/stores/plugin', () => ({
  usePluginStore: () => ({ getPluginStatusClass }),
}))

// Slot-passthrough stubs for the Quasar components (the app never installs Quasar).
const passthrough = (tag: string) => ({
  props: ['label', 'name', 'icon', 'color'],
  template: `<${tag}><slot/>{{ label }}</${tag}>`,
})
const stubs: Record<string, unknown> = { 'q-btn': passthrough('button') }
for (const name of [
  'q-card', 'q-card-section', 'q-card-actions', 'q-separator', 'q-icon',
  'q-avatar', 'q-chip', 'q-space', 'q-tabs', 'q-tab', 'q-tab-panels',
  'q-tab-panel', 'q-list', 'q-item', 'q-item-section', 'q-item-label',
]) {
  stubs[name] = passthrough('div')
}

// A realistic GET /export/plugins list-DTO item. Note: NO `health` — the list
// endpoint does not return it (that field only exists on the unused detail shape).
function makePlugin(overrides: Partial<Plugin> = {}): Plugin {
  return {
    name: 'json-export',
    display_name: 'JSON Export',
    description: 'Export device data as JSON or YAML',
    version: '1.2.3',
    category: 'backup',
    capabilities: ['json', 'yaml'],
    status: {
      available: true,
      configured: true,
      enabled: true,
    },
    ...overrides,
  }
}

function mountView(plugin: Plugin | null = makePlugin()) {
  return mount(PluginDetailsView, { props: { plugin }, global: { stubs } })
}

describe('PluginDetailsView (#260)', () => {
  it('renders list-DTO fields without throwing', () => {
    const text = mountView().text()
    expect(text).toContain('JSON Export')
    expect(text).toContain('v1.2.3')
    expect(text).toContain('json')
    expect(text).toContain('yaml')
  })

  it('derives the status label from the status object, not a string', () => {
    // 'ready' comes from getPluginStatusClass, proving status is treated as an object
    expect(mountView(makePlugin()).text()).toContain('ready')
    expect(mountView(makePlugin({ status: { available: true, configured: true, enabled: false } })).text())
      .toContain('disabled')
  })

  it('is read-only: no configure/toggle actions and no phantom or non-list-DTO fields', () => {
    const text = mountView().text()
    for (const absent of [
      // read-only: configuration/enable-disable deferred to #264/PR3
      'Configure', 'Enable', 'Disable',
      // health is not part of the list DTO
      'Health',
      // dropped phantom fields with no backend source
      'Author', 'License', 'Supported Formats', 'Usage', 'Dependencies',
    ]) {
      expect(text, `should not render "${absent}"`).not.toContain(absent)
    }
  })

  it('emits close from the close action', async () => {
    const wrapper = mountView()
    await wrapper.findAll('button').find((b) => b.text().includes('Close'))!.trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
