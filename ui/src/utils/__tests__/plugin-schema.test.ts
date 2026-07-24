import { describe, expect, it } from 'vitest'
import type { PluginSchema } from '@/api/plugin'
import {
  ABSENT_NUMBER,
  EMPTY_FIELD,
  initialPluginConfig,
  pluginConfigFromForm,
} from '../plugin-schema'

const schema: PluginSchema = {
  version: '1',
  required: ['name', 'enabled'],
  properties: {
    name: { type: 'string', description: 'Name' },
    enabled: { type: 'boolean', description: 'Enabled' },
    zero: { type: 'number', description: 'Zero', default: 0 },
    blank: { type: 'string', description: 'Blank', default: '' },
    optional: { type: 'boolean', description: 'Optional' },
    object: { type: 'object', description: 'Object', default: {} },
  },
}

describe('plugin schema adapter', () => {
  it('applies falsy defaults and initializes required values', () => {
    const values = initialPluginConfig(schema)
    expect(values).toMatchObject({ name: '', enabled: false, zero: 0, blank: '', optional: EMPTY_FIELD })
    expect(values.object).toBe('{}')
  })

  it('omits only untouched optional fields', () => {
    const values = initialPluginConfig(schema)
    values.name = 'example'
    values.optional = false
    const config = pluginConfigFromForm(schema, values, { optional: true })
    expect(config).toMatchObject({ name: 'example', enabled: false, zero: 0, blank: '', optional: false, object: {} })
  })

  it('distinguishes a cleared number from zero', () => {
    const numberSchema: PluginSchema = {
      version: '1',
      required: ['count'],
      properties: { count: { type: 'number', description: 'Count' } },
    }
    expect(() => pluginConfigFromForm(numberSchema, { count: ABSENT_NUMBER }, {})).toThrow()
    expect(pluginConfigFromForm(numberSchema, { count: 0 }, {})).toEqual({ count: 0 })
  })

  it('validates recursive array and object constraints', () => {
    const recursive: PluginSchema = {
      version: '1',
      required: ['ports', 'connection'],
      properties: {
        ports: {
          type: 'array',
          description: 'Ports',
          items: { type: 'number', description: 'Port', minimum: 1, maximum: 65535 },
        },
        connection: {
          type: 'object',
          description: 'Connection',
          properties: {
            mode: { type: 'string', description: 'Mode', enum: ['safe', 'fast'] },
          },
        },
      },
    }
    expect(() => pluginConfigFromForm(recursive, {
      ports: '[0]',
      connection: '{"mode":"unsafe"}',
    }, {})).toThrow()
    expect(pluginConfigFromForm(recursive, {
      ports: '[443]',
      connection: '{"mode":"safe"}',
    }, {})).toEqual({ ports: [443], connection: { mode: 'safe' } })
  })
})
