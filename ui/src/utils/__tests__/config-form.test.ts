import { describe, it, expect } from 'vitest'
import {
  countOverrides,
  deleteAt,
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

  // The shape the backend emits for auth (internal/configuration/typed_models.go): the keys are
  // required only while the section is enabled.
  const authSections: Record<string, SchemaProperty> = {
    auth: {
      type: 'object',
      properties: { enable: { type: 'boolean' }, user: { type: 'string' }, pass: { type: 'string' } },
      if: { properties: { enable: { const: true } } },
      then: { required: ['user', 'pass'] },
    },
  }

  it('then.required applies while the if condition holds', () => {
    const body = { auth: { enable: true, user: 'admin' } }
    expect(validateOverrides(authSections, body, {})).toEqual({ 'auth.pass': 'Required' })
    // The condition reads the effective value, so an inherited enable counts.
    expect(validateOverrides(authSections, { auth: { user: 'admin' } }, { auth: { enable: true } }))
      .toEqual({ 'auth.pass': 'Required' })
    expect(validateOverrides(authSections, { auth: { enable: true, user: 'admin' } }, { auth: { pass: 'x' } }))
      .toEqual({})
  })

  // A template is one layer: another layer can give the field, and the server accepts the body.
  it('a template layer is not required to be complete, except for a saved value it would drop', () => {
    const layer = { kept: null }
    const partial = { auth: { enable: true } }
    expect(validateOverrides(authSections, partial, {})).toEqual({ 'auth.user': 'Required', 'auth.pass': 'Required' })
    expect(validateOverrides(authSections, partial, {}, null, layer)).toEqual({})
    // The same holds for a section-level required list.
    const wifi: Record<string, SchemaProperty> = {
      wifi: { type: 'object', required: ['ssid'], properties: { ssid: { type: 'string' }, ipv4_method: { type: 'string' } } },
    }
    expect(validateOverrides(wifi, { wifi: { ipv4_method: 'dhcp' } }, {}, null, layer)).toEqual({})
    // A password the server holds is kept out of the body: a changed body must give it again.
    const kept = { auth: { pass: '********' } }
    expect(validateOverrides(authSections, partial, {}, null, { kept })).toEqual({ 'auth.pass': 'Required' })
    expect(validateOverrides(authSections, { auth: { enable: true, pass: 'new' } }, {}, null, { kept })).toEqual({})
    // Values are still checked.
    const range: Record<string, SchemaProperty> = {
      power_metering: { type: 'object', properties: { max_power: { type: 'integer', minimum: 0 } } },
    }
    expect(validateOverrides(range, { power_metering: { max_power: -1 } }, {}, null, layer))
      .toEqual({ 'power_metering.max_power': 'Must be at least 0' })
  })

  // A saved password is kept out of the template body whatever the schema requires: none of
  // wifi.password, mqtt.pass or auth.pass is a required field, yet a changed body that leaves
  // one out would replace the stored config and delete it.
  it.each([
    ['wifi.password', { wifi: { enable: true, ssid: 'office' } }, 'wifi', 'password', { wifi: { password: '********' } }],
    ['mqtt.pass', { mqtt: { enable: true, server: 'broker' } }, 'mqtt', 'pass', { mqtt: { pass: '********' } }],
    ['auth.pass', { auth: { user: 'admin' } }, 'auth', 'pass', { auth: { pass: '********' } }],
  ])('a changed layer body must give a saved %s again, even if the schema does not require it', (_n, body, section, key, kept) => {
    const sections: Record<string, SchemaProperty> = {
      relay: { type: 'object', properties: { auto_off: { type: 'integer', minimum: 0 } } },
      [section]: { type: 'object', properties: { [key]: { type: 'string' } } },
    }
    const changed = { ...body, relay: { auto_off: 60 } }
    expect(validateOverrides(sections, changed, {}, null, { kept })).toEqual({ [`${section}.${key}`]: 'Required' })
    const given = { ...changed, [section]: { ...(changed as any)[section], [key]: 'new' } }
    expect(validateOverrides(sections, given, {}, null, { kept })).toEqual({})
    // Nothing is saved for the field: no protection is owed.
    expect(validateOverrides(sections, changed, {}, null, { kept: null })).toEqual({})
  })

  it('a template secret that arrives masked still counts as the value the section inherits', () => {
    // The server never sends a set password: the inherited configuration holds a mask in its place.
    const inherited = { auth: { enable: true, user: 'admin', pass: '********' } }
    expect(validateOverrides(authSections, { auth: { user: 'operator' } }, inherited)).toEqual({})
  })

  it('a saved secret that the overrides response masks is not asked for again', () => {
    // GET overrides gives a set secret as the mask. The body the form edits then holds it, so an
    // unrelated edit is valid and the save sends the mask back, which keeps the stored value.
    const saved = { auth: { enable: true, user: 'admin', pass: '********' } }
    expect(validateOverrides(authSections, saved, {})).toEqual({})
    // Without it, the section is incomplete: that is the case the mask exists to tell apart.
    expect(validateOverrides(authSections, { auth: { enable: true, user: 'admin' } }, {}))
      .toEqual({ 'auth.pass': 'Required' })
  })

  it('a section whose last override was reset is still checked against the effective configuration', () => {
    // The section held a saved override (the masked password). Reset removed it, so the body is
    // {} and the section looks untouched, but saving {} deletes the password while the template
    // keeps authentication on: the effective configuration lacks the password.
    const inherited = { auth: { enable: true, user: 'admin' } }
    const held = { auth: { pass: '********' } }
    expect(validateOverrides(authSections, {}, inherited, held)).toEqual({ 'auth.pass': 'Required' })
    // Without the memory of the override, a section nobody touched stays unchecked.
    expect(validateOverrides(authSections, {}, inherited)).toEqual({})
    expect(validateOverrides(authSections, {}, inherited, {})).toEqual({})
    // The effective configuration decides: a password, or authentication off, makes it valid.
    expect(validateOverrides(authSections, {}, { auth: { enable: true, user: 'admin', pass: '********' } }, held)).toEqual({})
    expect(validateOverrides(authSections, {}, { auth: { enable: false } }, held)).toEqual({})
    expect(validateOverrides(authSections, { auth: { pass: 'new' } }, inherited, held)).toEqual({})
  })

  it('then.required is not asked while the if condition fails', () => {
    expect(validateOverrides(authSections, { auth: { enable: false, user: 'admin' } }, {})).toEqual({})
    expect(validateOverrides(authSections, { auth: { user: 'admin' } }, {})).toEqual({})
  })

  it('a section that is not an object is an error', () => {
    const sections: Record<string, SchemaProperty> = {
      mqtt: { type: 'object', properties: { enable: { type: 'boolean' } } },
    }
    expect(validateOverrides(sections, { mqtt: 'invalid' }, {})).toEqual({ mqtt: 'Must be an object' })
    expect(validateOverrides(sections, { mqtt: [] }, {})).toEqual({ mqtt: 'Must be an object' })
    expect(validateOverrides(sections, { mqtt: null }, {})).toEqual({ mqtt: 'Must be an object' })
    expect(validateOverrides(sections, { mqtt: { enable: true } }, {})).toEqual({})
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
})
