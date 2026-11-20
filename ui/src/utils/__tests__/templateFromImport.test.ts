import { describe, it, expect } from 'vitest'
import type { ConfigSchema } from '@/api/typedConfig'
import {
  MASK,
  PER_DEVICE_NOTE,
  buildTemplateBody,
  defaultTicked,
  importSections,
} from '../templateFromImport'

// The schema of the device, as GET /config/schema?device_id=... answers: the sections a template for
// this model accepts, and the keys inside each.
const schema = {
  type: 'object',
  properties: {
    wifi: {
      type: 'object',
      properties: {
        enable: { type: 'boolean' },
        ssid: { type: 'string' },
        password: { type: 'string' },
        static_ip: { type: 'object', properties: { ip: { type: 'string' } } },
        ap: { type: 'object', properties: { enable: { type: 'boolean' }, key: { type: 'string' } } },
      },
    },
    mqtt: {
      type: 'object',
      properties: {
        enable: { type: 'boolean' },
        server: { type: 'string' },
        user: { type: 'string' },
        pass: { type: 'string' },
      },
    },
    auth: {
      type: 'object',
      properties: { enable: { type: 'boolean' }, user: { type: 'string' }, pass: { type: 'string' } },
    },
    network: {
      type: 'object',
      properties: { wifi: { type: 'object', properties: { sta: { type: 'object', properties: { pass: { type: 'string' } } } } } },
    },
    system: {
      type: 'object',
      properties: {
        device: {
          type: 'object',
          properties: {
            name: { type: 'string' },
            mac: { type: 'string' },
            hostname: { type: 'string' },
            eco_mode: { type: 'boolean' },
          },
        },
        tz: { type: 'string' },
      },
    },
    coiot: {
      type: 'object',
      properties: { enable: { type: 'boolean' }, update_period: { type: 'integer' }, peer: { type: 'string' } },
    },
    relay: {
      type: 'object',
      properties: {
        relays: {
          type: 'array',
          items: { type: 'object', properties: { name: { type: 'string' }, default_state: { type: 'string' } } },
        },
      },
    },
  },
} as unknown as ConfigSchema

// The typed form of an import, as GET /devices/{id}/config/typed answers it, secrets masked.
function typedImport(): Record<string, any> {
  return {
    wifi: {
      enable: true,
      ssid: 'Home',
      password: MASK,
      static_ip: { ip: '192.0.2.9' },
      ap: { enable: false, key: MASK },
    },
    mqtt: { enable: true, server: 'broker:1883', user: 'iot', pass: MASK },
    auth: { enable: true, user: 'admin', pass: MASK },
    network: { wifi: { sta: { pass: MASK } } },
    system: { device: { name: 'plug-1', mac: 'AA:BB:CC:DD:EE:01', hostname: 'shelly1', eco_mode: true }, tz: 'Europe/Brussels' },
    coiot: { enabled: true, server: 'coiot:5683', port: 5683, period: 15 },
    color: { mode: 'white' },
    raw: { unknown: MASK },
    relay: { relays: [{ name: 'Light', default_state: 'off', secret_token: MASK }] },
  }
}

function hasMask(value: unknown): boolean {
  if (typeof value === 'string') return value === MASK
  if (Array.isArray(value)) return value.some(hasMask)
  if (value && typeof value === 'object') return Object.values(value).some(hasMask)
  return false
}

describe('importSections', () => {
  it('offers only sections the device schema names, and none that is empty after the projection', () => {
    const offered = importSections(typedImport(), schema)

    expect(offered).toEqual(expect.arrayContaining(['wifi', 'mqtt', 'auth', 'system', 'relay']))
    // color and raw are not in the schema; coiot is, but its typed keys (enabled, server, port, period)
    // are not the template's (enable, update_period, peer), so nothing of it survives. network holds
    // only a masked secret.
    expect(offered).not.toContain('color')
    expect(offered).not.toContain('raw')
    expect(offered).not.toContain('coiot')
    expect(offered).not.toContain('network')
  })
})

describe('defaultTicked', () => {
  it('starts wifi, network and auth unticked and every other section ticked', () => {
    expect(defaultTicked('wifi')).toBe(false)
    expect(defaultTicked('network')).toBe(false)
    expect(defaultTicked('auth')).toBe(false)
    expect(defaultTicked('mqtt')).toBe(true)
    expect(defaultTicked('system')).toBe(true)
    expect(defaultTicked('relay')).toBe(true)
  })
})

describe('buildTemplateBody', () => {
  it('never holds the placeholder at any depth, with every section ticked', () => {
    const { config } = buildTemplateBody(typedImport(), schema, Object.keys(schema.properties))

    expect(hasMask(config)).toBe(false)
    expect(JSON.stringify(config)).not.toContain(MASK)
    // A secret held under a key the schema names, inside an array item, is dropped too.
    expect(config.relay.relays[0]).toEqual({ name: 'Light', default_state: 'off' })
  })

  it('keeps what is not a secret and lists the secrets it left out', () => {
    const { config, omittedSecrets } = buildTemplateBody(typedImport(), schema, ['mqtt', 'auth', 'wifi'])

    expect(config.mqtt).toEqual({ enable: true, server: 'broker:1883', user: 'iot' })
    expect(config.auth).toEqual({ enable: true, user: 'admin' })
    expect(omittedSecrets).toEqual(expect.arrayContaining(['mqtt.pass', 'auth.pass', 'wifi.password', 'wifi.ap.key']))
  })

  it('leaves out a section that is not ticked', () => {
    const { config } = buildTemplateBody(typedImport(), schema, ['mqtt'])

    expect(Object.keys(config)).toEqual(['mqtt'])
  })

  it('projects the body onto the device schema at every depth: no coiot, no color, no raw', () => {
    const { config } = buildTemplateBody(typedImport(), schema, ['coiot', 'color', 'raw', 'system'])

    expect(config).not.toHaveProperty('coiot')
    expect(config).not.toHaveProperty('color')
    expect(config).not.toHaveProperty('raw')
    // Inside a section the schema names, a key it lacks is dropped too.
    const { config: withExtra } = buildTemplateBody(
      { mqtt: { enable: true, extra_key: 'x', server: 's' } },
      schema,
      ['mqtt'],
    )
    expect(withExtra.mqtt).toEqual({ enable: true, server: 's' })
  })

  it('always leaves out the per-device name, hostname, MAC and static IP, and says so', () => {
    const { config, leftOut } = buildTemplateBody(typedImport(), schema, ['system', 'wifi'])

    expect(config.system.device).toEqual({ eco_mode: true })
    expect(config.system.tz).toBe('Europe/Brussels')
    expect(config.wifi).not.toHaveProperty('static_ip')
    expect(leftOut).toEqual(
      expect.arrayContaining(['system.device.name', 'system.device.hostname', 'system.device.mac', 'wifi.static_ip']),
    )
    expect(PER_DEVICE_NOTE).toContain('MAC')
  })

  it('does not change the import it was given', () => {
    const typed = typedImport()
    const before = JSON.stringify(typed)

    buildTemplateBody(typed, schema, Object.keys(schema.properties))

    expect(JSON.stringify(typed)).toBe(before)
  })
})
