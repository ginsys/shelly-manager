import type { ConfigSchema, SchemaProperty } from '@/api/typedConfig'

// The string the server sends in place of a saved secret (internal/redact Placeholder). It is the
// only thing this file knows about secrets: the server decides what is one, and a template body must
// never hold this string, or applying the template would write it to a device.
export const MASK = '********'

// What is one device's own, so a template shared by a model or by every device must not carry it.
// Always left out, and the dialog says so. An address mode goes with its address: "static" with no
// address would misconfigure the device.
const PER_DEVICE_PATHS = [
  'system.device.name',
  'system.device.hostname',
  'system.device.mac',
  'mqtt.id',
  'mqtt.topic_prefix',
  'wifi.static_ip',
  'wifi.ipv4mode',
  'network.eth.ip',
  'network.eth.ipv4mode',
  'network.wifi.sta.ip',
  'network.wifi.sta.ipv4mode',
  'network.wifi.ap.ssid',
  // `*` stands for any list index, so a channel keeps its id and loses its name.
  'relay.relays.*.name',
  'input.inputs.*.name',
]
export const PER_DEVICE_NOTE =
  'per-device name, hostname, MAC, MQTT client id and topic prefix, access-point name, channel names and static address'

// Sections that start unticked: they hold the Wi-Fi network and the login of one site.
const UNTICKED_SECTIONS = ['wifi', 'network', 'auth']

export function defaultTicked(section: string): boolean {
  return !UNTICKED_SECTIONS.includes(section)
}

interface Walk {
  omittedSecrets: string[]
  leftOut: string[]
}

// Copies the part of value that the schema names, at every depth, and nothing else: a key the schema
// lacks is dropped (the server refuses a template key it does not know), a masked value is dropped
// and recorded, and a per-device path is dropped and recorded. It answers undefined when nothing is
// left, so an empty object or list never reaches the body.
function project(value: unknown, schema: SchemaProperty | undefined, path: string, walk: Walk): unknown {
  const generic = path.replace(/\.\d+(?=\.|$)/g, '.*')
  if (PER_DEVICE_PATHS.includes(generic)) {
    walk.leftOut.push(generic)
    return undefined
  }
  if (value === MASK) {
    walk.omittedSecrets.push(path)
    return undefined
  }
  if (!schema || value === null || value === undefined) return undefined

  if (Array.isArray(value)) {
    if (!schema.items) return undefined
    const items = value
      .map((item, i) => project(item, schema.items, `${path}.${i}`, walk))
      .filter((item) => item !== undefined)
    return items.length > 0 ? items : undefined
  }
  if (typeof value === 'object') {
    if (!schema.properties) return undefined
    const out: Record<string, unknown> = {}
    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
      const kept = project(child, schema.properties[key], `${path}.${key}`, walk)
      if (kept !== undefined) out[key] = kept
    }
    return Object.keys(out).length > 0 ? out : undefined
  }
  return value
}

// The sections of the typed import that a template for this device can hold: named in the device's
// schema, and not empty once secrets, per-device values and keys the schema lacks are gone.
export function importSections(typed: Record<string, any>, schema: ConfigSchema): string[] {
  const scratch: Walk = { omittedSecrets: [], leftOut: [] }
  return Object.keys(typed).filter((key) => project(typed[key], schema.properties[key], key, scratch) !== undefined)
}

export interface TemplateBody {
  config: Record<string, any>
  // Schema paths of the secrets the body leaves out, and of the per-device values it leaves out.
  omittedSecrets: string[]
  leftOut: string[]
}

// Builds the config of a new template from the sections the user ticked. The import is not changed.
export function buildTemplateBody(typed: Record<string, any>, schema: ConfigSchema, ticked: string[]): TemplateBody {
  const walk: Walk = { omittedSecrets: [], leftOut: [] }
  const config: Record<string, any> = {}
  for (const key of Object.keys(typed)) {
    if (!ticked.includes(key)) continue
    const kept = project(typed[key], schema.properties[key], key, walk)
    if (kept !== undefined) config[key] = kept
  }
  return { config, omittedSecrets: [...new Set(walk.omittedSecrets)], leftOut: [...new Set(walk.leftOut)] }
}
