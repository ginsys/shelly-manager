import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PluginSchemaViewer from '../PluginSchemaViewer.vue'
import type { PluginSchema } from '@/api/plugin'

const schema: PluginSchema = {
  type: 'object',
  title: 'Backup config',
  properties: {
    endpoint: {
      type: 'string',
      title: 'Endpoint',
      description: 'API endpoint URL',
      minLength: 1,
      format: 'url',
    },
    retention_days: {
      type: 'integer',
      description: 'Days to retain',
      minimum: 1,
      maximum: 365,
      default: 30,
    },
  },
  required: ['endpoint'],
}

describe('PluginSchemaViewer (read-only #264)', () => {
  it('renders schema-declared fields, constraints and defaults', () => {
    const text = mount(PluginSchemaViewer, { props: { schema } }).text()
    expect(text).toContain('endpoint')
    expect(text).toContain('retention_days')
    expect(text).toContain('Days to retain')
    expect(text).toContain('Max: 365')
    // schema-declared default is shown as-is (not a generated value)
    expect(text).toContain('30')
  })

  it('is genuinely read-only: no configure/test/save or example-generating controls', () => {
    const wrapper = mount(PluginSchemaViewer, { props: { schema } })
    // No form inputs and no action buttons at all.
    expect(wrapper.findAll('input').length).toBe(0)
    expect(wrapper.findAll('button').length).toBe(0)
    const text = wrapper.text()
    for (const absent of ['Save', 'Test', 'Example', 'Load', 'Configure', 'Reconfigure']) {
      expect(text, `should not offer "${absent}"`).not.toContain(absent)
    }
    expect(wrapper.text()).toContain('Read-only')
  })

  it('treats an empty schema truthfully as "No configuration schema published"', () => {
    const empty: PluginSchema = { type: 'object', properties: {} }
    const text = mount(PluginSchemaViewer, { props: { schema: empty } }).text()
    expect(text).toContain('No configuration schema published')
    // Never suggests manual/custom configuration or a test feature.
    expect(text).not.toContain('custom JSON')
    expect(text).not.toContain('test feature')
  })
})
