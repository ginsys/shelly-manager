import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it, expect } from 'vitest'
import {
  countOverrides,
  deleteAt,
  formErrorKey,
  getAt,
  hasAt,
  nullRemovedSecrets,
  savedSecretBaseline,
  setAt,
  sourceKey,
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

// A list whose items carry an int `id` is a list of channels (relays, inputs). The server merges it by
// id, not by position (internal/configuration/merge.go, channelIDs and mergeChannelsByID), so a path
// names an item by its id, written "#<id>".
describe('config-form paths through a list of channels', () => {
  const relay = (id: number, name: string) => ({ id, name })

  it('getAt and hasAt follow an item by its id, not by its position', () => {
    const body = { relay: { relays: [relay(1, 'lamp'), relay(3, 'fan')] } }
    expect(getAt(body, ['relay', 'relays', '#3', 'name'])).toBe('fan')
    expect(hasAt(body, ['relay', 'relays', '#3', 'name'])).toBe(true)
    expect(hasAt(body, ['relay', 'relays', '#2', 'name'])).toBe(false)
    expect(hasAt(body, ['relay', 'relays', '#2'])).toBe(false)
    expect(getAt(body, ['relay', 'relays'])).toEqual(body.relay.relays)
  })

  it('an id the items leave out is 0, and all ids 0 in a list of more than one is the position', () => {
    // The Go rule (channelIDs): an id left out of a stored template decodes as 0, so in a list of
    // more than one entry whose ids are all 0 the id is the list index.
    const omitted = { relay: { relays: [{ name: 'a' }, { name: 'b' }] } }
    expect(getAt(omitted, ['relay', 'relays', '#1', 'name'])).toBe('b')
    const single = { relay: { relays: [{ name: 'only' }] } }
    expect(getAt(single, ['relay', 'relays', '#0', 'name'])).toBe('only')
  })

  it('a list with two items of one id is not addressable', () => {
    const body = { relay: { relays: [relay(1, 'a'), relay(1, 'b')] } }
    expect(getAt(body, ['relay', 'relays', '#1', 'name'])).toBeUndefined()
    expect(hasAt(body, ['relay', 'relays', '#1'])).toBe(false)
  })

  it('setAt creates the list and the item with its id, in id order, and does not change its input', () => {
    const body = { relay: { relays: [relay(2, 'fan')] } }
    const next = setAt(body, ['relay', 'relays', '#1', 'name'], 'lamp')
    expect(next).toEqual({ relay: { relays: [{ id: 1, name: 'lamp' }, relay(2, 'fan')] } })
    expect(body).toEqual({ relay: { relays: [relay(2, 'fan')] } })
    expect(setAt({}, ['relay', 'relays', '#0', 'name'], 'x')).toEqual({ relay: { relays: [{ id: 0, name: 'x' }] } })
  })

  it('setAt changes one field of an existing item and keeps its others', () => {
    const body = { relay: { relays: [{ id: 1, name: 'lamp', auto_on: 5 }] } }
    expect(setAt(body, ['relay', 'relays', '#1', 'name'], 'desk')).toEqual({
      relay: { relays: [{ id: 1, name: 'desk', auto_on: 5 }] },
    })
  })

  it('deleteAt removes the field, then an item left with only its id, then the empty list and section', () => {
    const body = { relay: { relays: [{ id: 1, name: 'lamp' }, { id: 2, name: 'fan', auto_on: 5 }] } }
    const one = deleteAt(body, ['relay', 'relays', '#1', 'name'])
    expect(one).toEqual({ relay: { relays: [{ id: 2, name: 'fan', auto_on: 5 }] } })
    expect(deleteAt(one, ['relay', 'relays', '#2', 'name'])).toEqual({ relay: { relays: [{ id: 2, auto_on: 5 }] } })
    expect(deleteAt({ relay: { relays: [{ id: 1, name: 'lamp' }] } }, ['relay', 'relays', '#1', 'name'])).toEqual({})
    expect(body.relay.relays).toHaveLength(2)
  })

  it('countOverrides counts the fields of each item, not the list as one', () => {
    const body = { relay: { relays: [{ id: 1, name: 'lamp', auto_on: 5 }, { id: 2, name: 'fan' }] } }
    expect(countOverrides(body, ['relay'])).toBe(3)
    expect(countOverrides({ relay: { relays: [{ id: 1 }] } }, ['relay'])).toBe(0)
  })

  const relaySchema: SchemaProperty = {
    type: 'object',
    properties: {
      relays: {
        type: 'array',
        items: {
          type: 'object',
          properties: {
            id: { type: 'integer' },
            name: { type: 'string', maxLength: 4 },
            auto_on: { type: 'integer', minimum: 0 },
          },
        },
      },
    },
  }

  it('validateOverrides checks each item, keyed by the id of the item', () => {
    const errors = validateOverrides({ relay: relaySchema }, {
      relay: { relays: [{ id: 1, name: 'lamp' }, { id: 2, name: 'toolong', auto_on: -1 }] },
    }, null)
    expect(errors).toEqual({
      'relay.relays.2.name': 'Must be at most 4 characters',
      'relay.relays.2.auto_on': 'Must be at least 0',
    })
  })

  it('items that all leave the id out are pinned to their positions before one is removed or added', () => {
    // Two items without an id are relay 0 and relay 1. Removing the first must not turn the second into 0.
    const body = { relay: { relays: [{ id: 0, name: 'Lamp' }, { id: 0, name: 'Fan' }] } }
    expect(deleteAt(body, ['relay', 'relays', '#0', 'name'])).toEqual({ relay: { relays: [{ id: 1, name: 'Fan' }] } })
    expect(setAt(body, ['relay', 'relays', '#2', 'name'], 'Fan 2')).toEqual({
      relay: { relays: [{ id: 0, name: 'Lamp' }, { id: 1, name: 'Fan' }, { id: 2, name: 'Fan 2' }] },
    })
    expect(body.relay.relays).toEqual([{ id: 0, name: 'Lamp' }, { id: 0, name: 'Fan' }])
  })

  it('formErrorKey maps a server position to the id of that item in the body that was sent', () => {
    const sent = { relay: { relays: [{ id: 1, name: 'x' }] } }
    expect(formErrorKey(sent, 'relay.relays.0.name')).toBe('relay.relays.1.name')
    expect(formErrorKey(sent, 'relay.relays.5.name')).toBe('relay.relays.5.name')
    expect(formErrorKey(sent, 'relay.default_state')).toBe('relay.default_state')
    // A list of one control (two items of one id, plain values) takes the error on the list.
    expect(formErrorKey({ relay: { relays: [{ id: 1 }, { id: 1 }] } }, 'relay.relays.0.name')).toBe('relay.relays')
    const rules = { relay: { relays: [{ id: 1, schedule_rules: [42] }] } }
    expect(formErrorKey(rules, 'relay.relays.0.schedule_rules.0')).toBe('relay.relays.1.schedule_rules')
  })

  it('sourceKey turns an id into the position the server keys sources by', () => {
    const inherited = { relay: { relays: [{ id: 1, name: 'Fan' }, { id: 3, name: 'Lamp' }] } }
    expect(sourceKey(inherited, ['relay', 'relays', '#3', 'name'])).toBe('relay.relays.1.name')
    expect(sourceKey(inherited, ['relay', 'relays', '#2', 'name'])).toBeUndefined()
    expect(sourceKey(inherited, ['relay', 'default_state'])).toBe('relay.default_state')
  })
})

describe('nullRemovedSecrets', () => {
  const mask = '********'
  const secret = { type: 'string', format: 'password' } as const
  const schema = {
    properties: {
      mqtt: { type: 'object', properties: { user: { type: 'string' }, pass: secret } },
      wifi: { type: 'object', properties: { ap: { type: 'object', properties: { pass: secret } } } },
      auth: { type: 'object', properties: { pass: secret } },
    },
  } as any

  it('sends null where a saved secret is gone from the edited body', () => {
    const saved = { mqtt: { user: 'iot', pass: mask }, wifi: { ap: { pass: mask } } }
    expect(nullRemovedSecrets({ mqtt: { user: 'iot' }, wifi: { ap: { pass: mask } } }, saved, schema)).toEqual({
      mqtt: { user: 'iot', pass: null },
      wifi: { ap: { pass: mask } },
    })
  })

  it('also removes a saved empty secret, which an omitted key would keep', () => {
    expect(nullRemovedSecrets({ mqtt: { user: 'iot' } }, { mqtt: { user: 'iot', pass: '' } }, schema)).toEqual({
      mqtt: { user: 'iot', pass: null },
    })
  })

  it('also removes a masked secret that the schema leaves out, such as the AP password and key', () => {
    const saved = { wifi: { ap: { pass: mask, key: mask } } }
    const bare = { properties: { wifi: { type: 'object', properties: { ap: { type: 'object', properties: {} } } } } } as any
    expect(nullRemovedSecrets({}, saved, bare)).toEqual({ wifi: { ap: { pass: null, key: null } } })
    expect(nullRemovedSecrets({ wifi: { ap: { key: mask } } }, saved, bare)).toEqual({
      wifi: { ap: { key: mask, pass: null } },
    })
  })

  it('creates the section again when a reset removed it whole', () => {
    expect(nullRemovedSecrets({}, { auth: { pass: mask } }, schema)).toEqual({ auth: { pass: null } })
  })

  it('leaves a kept mask, a new value and a field that is not a secret alone', () => {
    const saved = { mqtt: { user: 'iot', pass: mask }, auth: { pass: mask } }
    const edited = { mqtt: { user: 'iot', pass: mask }, auth: { pass: 'new' } }
    expect(nullRemovedSecrets(edited, saved, schema)).toEqual(edited)
    expect(nullRemovedSecrets({}, { mqtt: { user: '' } }, schema)).toEqual({})
  })

  it('does not change its input and accepts no saved body or schema', () => {
    const edited = { mqtt: { user: 'iot' } }
    nullRemovedSecrets(edited, { mqtt: { pass: mask } }, schema)
    expect(edited).toEqual({ mqtt: { user: 'iot' } })
    expect(nullRemovedSecrets(edited, null, schema)).toEqual(edited)
    expect(nullRemovedSecrets(edited, { mqtt: { user: 'iot' } }, null)).toEqual(edited)
  })

  it('finds a masked secret by its mask when there is no schema yet', () => {
    expect(nullRemovedSecrets({}, { mqtt: { pass: mask } }, null)).toEqual({ mqtt: { pass: null } })
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

  // The server holds the same rule (isIPv4): an empty string reads as "not set".
  it('checks the ipv4 format and lets an empty string pass', () => {
    const ip: SchemaProperty = { type: 'string', format: 'ipv4' }
    for (const bad of ['300.1.1.1', '1.2.3', '1.2.3.4.5', '010.0.0.1', ' 10.0.0.9', '10.0.0.9 ', 'fe80::1', 'dns.example', '1.2.3.-4']) {
      expect(validateValue(ip, bad), bad).toBe('Must be an IPv4 address')
    }
    for (const good of ['', '0.0.0.0', '192.0.2.1', '255.255.255.255']) {
      expect(validateValue(ip, good), good).toBeNull()
    }
  })

  // The server holds the same rules (validTimeServer, isHostPort); both sides read one list.
  it('checks the hostname and hostport formats on the list the server reads', () => {
    const cases = JSON.parse(readFileSync(
      resolve(process.cwd(), '../testdata/config/hosts.json'),
      'utf8',
    )) as Record<'hostname' | 'hostport' | 'endpoint', { valid: string[]; invalid: string[] }>
    const hostname: SchemaProperty = { type: 'string', format: 'hostname' }
    const hostport: SchemaProperty = { type: 'string', format: 'hostport' }
    const endpoint: SchemaProperty = { type: 'string', format: 'endpoint' }
    // An endpoint is a host, with or without a port, so every valid host is valid there too.
    const endpointList = { valid: [...cases.endpoint.valid, ...cases.hostname.valid], invalid: cases.endpoint.invalid }
    for (const [schema, list] of [
      [hostname, cases.hostname],
      [hostport, cases.hostport],
      [endpoint, endpointList],
    ] as const) {
      expect(list.valid.length).toBeGreaterThan(0)
      expect(list.invalid.length).toBeGreaterThan(0)
      for (const good of [...list.valid, '']) {
        expect(validateValue(schema, good), `${schema.format} ${good}`).toBeNull()
      }
      for (const bad of list.invalid) {
        expect(validateValue(schema, bad), `${schema.format} ${bad}`).toEqual(expect.stringMatching(/^Must be /))
      }
    }
    expect(validateValue(hostname, '192.168.1.300')).toBe('Must be a host name or an IPv4 address')
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
  it('a template layer is not required to be complete', () => {
    const partial = { auth: { enable: true } }
    expect(validateOverrides(authSections, partial, {})).toEqual({ 'auth.user': 'Required', 'auth.pass': 'Required' })
    expect(validateOverrides(authSections, partial, {}, null, true)).toEqual({})
    // The same holds for a section-level required list.
    const wifi: Record<string, SchemaProperty> = {
      wifi: { type: 'object', required: ['ssid'], properties: { ssid: { type: 'string' }, ipv4_method: { type: 'string' } } },
    }
    expect(validateOverrides(wifi, { wifi: { ipv4_method: 'dhcp' } }, {}, null, true)).toEqual({})
    // Values are still checked.
    const range: Record<string, SchemaProperty> = {
      power_metering: { type: 'object', properties: { max_power: { type: 'integer', minimum: 0 } } },
    }
    expect(validateOverrides(range, { power_metering: { max_power: -1 } }, {}, null, true))
      .toEqual({ 'power_metering.max_power': 'Must be at least 0' })
  })

  // The server keeps a saved password that a body leaves out, so a changed body need not give it again.
  it('a changed layer body that leaves out a saved password is valid', () => {
    const sections: Record<string, SchemaProperty> = {
      relay: { type: 'object', properties: { auto_off: { type: 'integer', minimum: 0 } } },
      mqtt: { type: 'object', properties: { pass: { type: 'string' } } },
    }
    const saved = { mqtt: { pass: '********' } }
    expect(validateOverrides(sections, { relay: { auto_off: 60 } }, saved, null, true)).toEqual({})
  })

  it('savedSecretBaseline masks each path of stored_secrets, and falls back to the flags', () => {
    const mask = '********'
    expect(savedSecretBaseline({ stored_secrets: ['wifi.ap.pass', 'auth.pass'] }))
      .toEqual({ wifi: { ap: { pass: mask } }, auth: { pass: mask } })
    // The list is exact: an access point password does not mask the station password.
    expect(savedSecretBaseline({ stored_secrets: ['wifi.ap.pass'], has_wifi_password: true }))
      .toEqual({ wifi: { ap: { pass: mask } } })
    expect(savedSecretBaseline({ has_mqtt_password: true })).toEqual({ mqtt: { pass: mask } })
    expect(savedSecretBaseline({ stored_secrets: [] })).toBeNull()
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
