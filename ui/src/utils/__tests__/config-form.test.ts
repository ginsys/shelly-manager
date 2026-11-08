import { describe, it, expect } from 'vitest'
import {
  countOverrides,
  deleteAt,
  inheritedBaseline,
  setAt,
  subtreeApplicable,
  templateSchemaTarget,
  validateOverrides,
  validateValue,
} from '../config-form'
import type { SchemaProperty } from '@/api/typedConfig'

describe('templateSchemaTarget', () => {
  it('uses the model schema for a device_type template, and none until the type is set', () => {
    expect(templateSchemaTarget('device_type', ' SHPLG-S ')).toEqual({ deviceType: 'SHPLG-S' })
    expect(templateSchemaTarget('device_type', '')).toBeNull()
  })
  it('uses the template schema for a global or group template', () => {
    expect(templateSchemaTarget('group', 'SHPLG-S')).toEqual({ scope: 'group' })
    expect(templateSchemaTarget(undefined, undefined)).toEqual({ scope: 'global' })
  })
})

describe('config-form paths', () => {
  it('setAt creates parents and does not change its input', () => {
    const body = { mqtt: { enable: true } }
    const next = setAt(body, ['system', 'device', 'name'], 'plug')
    expect(next).toEqual({ mqtt: { enable: true }, system: { device: { name: 'plug' } } })
    expect(body).toEqual({ mqtt: { enable: true } })
  })

  it('deleteAt prunes parents that become empty', () => {
    const body = { system: { device: { name: 'plug' } }, mqtt: { enable: true } }
    expect(deleteAt(body, ['system', 'device', 'name'])).toEqual({ mqtt: { enable: true } })
  })

  it('deleteAt keeps a parent that still holds keys', () => {
    const body = { system: { device: { name: 'plug', eco_mode: true } } }
    expect(deleteAt(body, ['system', 'device', 'name'])).toEqual({ system: { device: { eco_mode: true } } })
  })

  it('countOverrides counts leaves', () => {
    expect(countOverrides({ mqtt: { enable: true, server: 'b', tls: {} } }, ['mqtt'])).toBe(2)
    expect(countOverrides({}, ['mqtt'])).toBe(0)
  })
})

describe('config-form validation', () => {
  it('checks type, range, length, pattern and enum', () => {
    expect(validateValue({ type: 'integer' }, 1.5)).toBe('Must be a whole number')
    expect(validateValue({ type: 'integer', minimum: 1 }, 0)).toBe('Must be at least 1')
    expect(validateValue({ type: 'number', maximum: 2 }, 2)).toBeNull()
    expect(validateValue({ type: 'string', maxLength: 3 }, 'abcd')).toBe('Must be at most 3 characters')
    expect(validateValue({ type: 'string', pattern: '[a-z]+' }, 'ab1')).toBe('Does not match the required format')
    expect(validateValue({ type: 'string', enum: ['off', 'on'] }, 'last')).toBe('Must be one of: off, on')
    expect(validateValue({ type: 'array' }, 'not json')).toBe('Must be a JSON list')
    expect(validateValue({ type: 'integer', minimum: 1 }, undefined)).toBeNull()
  })

  it('required fails only when the object is overridden and nothing gives a value', () => {
    const sections: Record<string, SchemaProperty> = {
      wifi: { type: 'object', required: ['ssid'], properties: { ssid: { type: 'string' }, ipv4_method: { type: 'string' } } },
    }
    expect(validateOverrides(sections, {}, {})).toEqual({})
    expect(validateOverrides(sections, { wifi: { ipv4_method: 'dhcp' } }, {})).toEqual({ 'wifi.ssid': 'Required' })
    expect(validateOverrides(sections, { wifi: { ipv4_method: 'dhcp' } }, { wifi: { ssid: 'home' } })).toEqual({})
  })
})

describe('config-form applicability and baseline', () => {
  it('a subtree is applicable when any leaf is', () => {
    const relays: SchemaProperty = {
      type: 'array',
      items: { type: 'object', properties: { name: { type: 'string', 'x-applicable': false } } },
    }
    expect(subtreeApplicable(relays)).toBe(false)
    expect(subtreeApplicable({ type: 'string' })).toBe(true)
  })

  it('the baseline drops values that came from the saved override', () => {
    const desired = { mqtt: { server: 'broker', port: 1884 } }
    const sources = { 'mqtt.server': 'Global', 'mqtt.port': 'device-override' }
    expect(inheritedBaseline(desired, sources)).toEqual({ mqtt: { server: 'broker' } })
  })
})
