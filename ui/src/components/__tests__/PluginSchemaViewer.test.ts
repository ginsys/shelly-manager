import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PluginSchemaViewer from '../PluginSchemaViewer.vue'
import type { PluginSchema } from '@/api/plugin'

describe('plugin schema viewer', () => {
  it('shows backend fields recursively without invented schema attributes', () => {
    const schema: PluginSchema = {
      version: '2026.1',
      required: ['endpoint'],
      properties: {
        endpoint: { type: 'string', description: 'API endpoint', sensitive: true, pattern: '^https://' },
        options: {
          type: 'object',
          description: 'Options',
          properties: { retries: { type: 'number', description: 'Retries', minimum: 0, maximum: 3 } },
        },
      },
    }
    const text = mount(PluginSchemaViewer, { props: { schema } }).text()
    expect(text).toContain('Version 2026.1')
    expect(text).toContain('API endpoint')
    expect(text).toContain('Sensitive')
    expect(text).toContain('Maximum: 3')
  })

  it('handles null required and an empty schema', () => {
    const schema: PluginSchema = { version: '1', required: null, properties: {} }
    expect(mount(PluginSchemaViewer, { props: { schema } }).text()).toContain('No configuration properties')
  })
})
