import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PluginDetailsView from '../PluginDetailsView.vue'
import type { Plugin } from '@/api/plugin'

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

  it('presents registration only, not configured/enabled status (#266)', () => {
    // Backend hardcodes status; the view shows a neutral "Registered" chip and
    // never the fictional Ready/Disabled/Not Configured labels.
    const text = mountView(makePlugin()).text()
    expect(text).toContain('Registered')
    for (const fiction of ['Ready', 'Not Configured', 'Disabled', 'unavailable']) {
      expect(text, `should not render fictional status "${fiction}"`).not.toContain(fiction)
    }
  })

  it('is read-only: no configure/toggle actions and no phantom or non-list-DTO fields', () => {
    const text = mountView().text()
    for (const absent of [
      // read-only: configuration/enable-disable deferred to #264
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
