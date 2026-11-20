import type { SchemaProperty } from '@/api/typedConfig'

// Helpers for the device configuration form (ConfigEditor, ConfigSchemaField).
//
// The form edits an override body: a partial DeviceConfiguration that holds only the fields the
// user changed. Every other field shows its inherited value (the desired configuration, from
// templates) as the baseline. Paths are arrays of keys; their dotted form matches the backend's
// desired-configuration sources map (internal/configuration/merge.go), for example
// "system.device.name".

export type ConfigPath = string[]
export type ConfigBody = Record<string, any>

export const APPLICABLE_KEY = 'x-applicable' as const

// A list whose items are objects with an int `id` is a list of channels (relays, inputs). The server
// merges it by id, not by position (internal/configuration/merge.go: channelIDs, mergeChannelsByID),
// so a path names one item by its id, written "#<id>": ["relay", "relays", "#1", "name"].
const ITEM = '#'

export function isItem(key: string): boolean {
  return key.startsWith(ITEM)
}

// Items of a list that all leave the id out are addressed by position. Adding or removing an item
// would move the others, so each one takes its position as an explicit id first.
function pinPositions(list: any[]) {
  if (list.length > 1 && list.every(item => isPlainObject(item) && !(typeof item.id === 'number' && item.id !== 0))) {
    list.forEach((item, i) => { item.id = i })
  }
}

export function itemKey(id: number): string {
  return `${ITEM}${id}`
}

// listIds is channelIDs in internal/configuration/merge.go: the id of each item, an id left out
// being 0, and in a list of more than one item whose ids are all 0 the position. It is null for a
// list that cannot be addressed by id: an item that is no object, or two items of one id.
export function listIds(list: any[]): number[] | null {
  if (!list.every(isPlainObject)) return null
  const ids = list.map(item => (typeof item.id === 'number' ? item.id : 0))
  if (ids.length > 1 && ids.every(id => id === 0)) ids.forEach((_, i) => { ids[i] = i })
  return new Set(ids).size === ids.length ? ids : null
}

function itemIndex(list: any[], key: string): number {
  return isItem(key) ? (listIds(list) ?? []).indexOf(Number(key.slice(ITEM.length))) : -1
}

// child is the value under key: a field of an object, or an item of a list by its "#id".
function child(node: any, key: string): any {
  if (Array.isArray(node)) return node[itemIndex(node, key)]
  if (node === null || typeof node !== 'object' || isItem(key)) return undefined
  return node[key]
}

// The key of a path as the form shows it and as errors are keyed: an item is its id, so
// "relay.relays.1.name". The server's sources map keys an item by its position, which is the id in
// a list in id order (the Gen1 apply requires that).
export function pathKey(path: ConfigPath): string {
  return path.map(key => (isItem(key) ? key.slice(ITEM.length) : key)).join('.')
}

// The server names a list item by its position in the body it was sent ("relay.relays.0.name"). The
// form keys an item by its id, so each position becomes the id of that item in the body. A list the
// form cannot address by id (a list of plain values, two items of one id) is one control, so the
// path ends at that list. A position the body lacks keeps the path as the server gave it.
export function formErrorKey(body: ConfigBody, serverPath: string): string {
  let node: any = body
  const keys: string[] = []
  for (const key of serverPath.split('.')) {
    if (Array.isArray(node) && /^\d+$/.test(key)) {
      const ids = listIds(node)
      if (ids === null) return keys.join('.')
      if (Number(key) >= ids.length) return serverPath
      keys.push(String(ids[Number(key)]))
      node = node[Number(key)]
    } else {
      keys.push(key)
      node = isPlainObject(node) ? node[key] : undefined
    }
  }
  return keys.join('.')
}

// The server's sources map keys a list item by its position in the merged list, so a path of the form
// becomes that key: "relay.relays.1.name" for item #3 when it is the second item of the inherited
// list. An item the inherited list lacks has no source.
export function sourceKey(inherited: ConfigBody | null | undefined, path: ConfigPath): string | undefined {
  let node: any = inherited
  const keys: string[] = []
  for (const key of path) {
    if (isItem(key)) {
      const at = Array.isArray(node) ? itemIndex(node, key) : -1
      if (at < 0) return undefined
      keys.push(String(at))
      node = node[at]
    } else {
      keys.push(key)
      node = isPlainObject(node) ? node[key] : undefined
    }
  }
  return keys.join('.')
}

export function getAt(body: ConfigBody | null | undefined, path: ConfigPath): any {
  let node: any = body
  for (const key of path) {
    node = child(node, key)
    if (node === undefined) return undefined
  }
  return node
}

export function hasAt(body: ConfigBody | null | undefined, path: ConfigPath): boolean {
  if (path.length === 0) return false
  const parent = getAt(body, path.slice(0, -1))
  const last = path[path.length - 1]
  if (Array.isArray(parent)) return itemIndex(parent, last) >= 0
  return parent !== null && typeof parent === 'object' && !isItem(last) &&
    Object.prototype.hasOwnProperty.call(parent, last)
}

// put stores value under key: a field of an object, or an item of a list, which takes its id and
// stands before the first item of a higher id.
function put(container: any, key: string, value: any) {
  if (!Array.isArray(container)) {
    container[key] = value
    return
  }
  const id = Number(key.slice(ITEM.length))
  const at = itemIndex(container, key)
  if (at >= 0) {
    container[at] = value
    return
  }
  pinPositions(container)
  const ids = listIds(container) ?? []
  const before = ids.findIndex(other => other > id)
  container.splice(before < 0 ? container.length : before, 0, { id, ...value })
}

// setAt returns a copy of body with value at path. It creates missing parents: an object, or a
// list where the next key is an item.
export function setAt(body: ConfigBody, path: ConfigPath, value: any): ConfigBody {
  const copy: ConfigBody = clone(body)
  let node: any = copy
  path.slice(0, -1).forEach((key, i) => {
    const wantList = isItem(path[i + 1])
    let next = child(node, key)
    if (next === null || typeof next !== 'object' || Array.isArray(next) !== wantList) {
      next = wantList ? [] : {}
      put(node, key, next)
      next = child(node, key)
    }
    node = next
  })
  put(node, path[path.length - 1], value)
  return copy
}

function remove(container: any, key: string) {
  if (!Array.isArray(container)) {
    delete container[key]
    return
  }
  const at = itemIndex(container, key)
  if (at < 0) return
  pinPositions(container)
  container.splice(at, 1)
}

// An empty object or list, or an item left with only its id, holds no override.
function holdsNothing(node: any, key: string): boolean {
  if (Array.isArray(node)) return node.length === 0
  const keys = Object.keys(node)
  return keys.length === 0 || (isItem(key) && keys.length === 1 && keys[0] === 'id')
}

// deleteAt returns a copy of body without the key at path. Parent objects that become empty are
// removed too, so a reset field leaves no empty section in the override body.
export function deleteAt(body: ConfigBody, path: ConfigPath): ConfigBody {
  const copy: ConfigBody = clone(body)
  const parents: any[] = [copy]
  let node: any = copy
  for (const key of path.slice(0, -1)) {
    node = child(node, key)
    if (node === null || typeof node !== 'object') return copy
    parents.push(node)
  }
  remove(node, path[path.length - 1])
  for (let i = path.length - 2; i >= 0; i--) {
    if (!holdsNothing(parents[i + 1], path[i])) break
    remove(parents[i], path[i])
  }
  return copy
}

export function clone<T>(value: T): T {
  return value === undefined ? value : JSON.parse(JSON.stringify(value))
}

// isSettings is true for a configuration that is an object of settings. A configuration the server
// could not read as settings arrives as text, and a history entry may hold none (null).
export function isSettings(config: unknown): config is Record<string, any> {
  return typeof config === 'object' && config !== null && !Array.isArray(config)
}

// sameValue compares two JSON values. Objects compare by their serialised form, so only use it on
// leaves and lists, where key order does not vary.
export function sameValue(a: any, b: any): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

// isApplicable is false only when the device schema marks the field x-applicable: false. The
// generic schema carries no mark, so every field stays editable there.
export function isApplicable(schema: SchemaProperty): boolean {
  return schema[APPLICABLE_KEY] !== false
}

// subtreeApplicable is true when any leaf under schema is applicable. An array or object whose
// leaves all are not applicable renders read-only.
export function subtreeApplicable(schema: SchemaProperty): boolean {
  if (schema.type === 'object' && schema.properties) {
    return Object.values(schema.properties).some(subtreeApplicable)
  }
  if (schema.type === 'array') {
    return schema.items ? subtreeApplicable(schema.items) : isApplicable(schema)
  }
  return isApplicable(schema)
}

// IPV4 is four decimal parts of 0 to 255 with no leading zero, as Go's netip.ParseAddr reads one.
const IPV4 = /^(?:(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$/

const DNS_LABEL_CHARS = /^[A-Za-z0-9-]+$/
const ALL_DIGITS = /^\d+$/
const PORT = /^[1-9]\d{0,4}$/

// isHostName is validTimeServer in internal/configuration/apply_service.go: an IPv4 address or a
// DNS name of letters, digits and hyphens, no label empty, over 63 characters or edged by a hyphen,
// 253 characters at most, one terminal dot allowed. A name whose last label is all digits is a
// mistyped address, so it must be a valid IPv4 address. Both sides read testdata/config/hosts.json.
export function isHostName(value: string): boolean {
  const name = value.endsWith('.') ? value.slice(0, -1) : value
  if (name.length > 253) return false
  const labels = name.split('.')
  for (const label of labels) {
    if (label.length === 0 || label.length > 63 || label.startsWith('-') || label.endsWith('-')) return false
    if (!DNS_LABEL_CHARS.test(label)) return false
  }
  if (ALL_DIGITS.test(labels[labels.length - 1])) return IPV4.test(name)
  return true
}

// isHostPort is isHostPort in internal/configuration/schema_validate.go: a host name or IPv4
// address, one colon, and a decimal port from 1 to 65535 with no sign and no leading zero.
export function isHostPort(value: string): boolean {
  const colon = value.indexOf(':')
  if (colon < 0) return false
  const port = value.slice(colon + 1)
  return isHostName(value.slice(0, colon)) && PORT.test(port) && Number(port) <= 65535
}

const HEX_GROUP = /^[0-9a-fA-F]{1,4}$/

// isIPv6Literal is isIPv6Literal in internal/configuration/schema_validate.go: an IPv6 address in
// colon-hex form, with no brackets, zone, embedded IPv4 part or port. One "::" stands for at least
// one group of zeros. Both sides read testdata/config/hosts.json.
export function isIPv6Literal(value: string): boolean {
  if (!/^[0-9a-fA-F:]+$/.test(value) || !value.includes(':')) return false
  const halves = value.split('::')
  if (halves.length > 2) return false
  const groups = halves.map(half => (half === '' ? [] : half.split(':')))
  if (!groups.every(list => list.every(group => HEX_GROUP.test(group)))) return false
  const count = groups.reduce((sum, list) => sum + list.length, 0)
  return halves.length === 2 ? count <= 7 : count === 8
}

// validateValue returns an error message for one leaf value, or null when it is valid. An
// undefined value is not checked here: required checks need the inherited value too.
export function validateValue(schema: SchemaProperty, value: any): string | null {
  if (value === undefined) return null
  switch (schema.type) {
    case 'integer':
      if (typeof value !== 'number' || !Number.isInteger(value)) return 'Must be a whole number'
      break
    case 'number':
      if (typeof value !== 'number' || Number.isNaN(value)) return 'Must be a number'
      break
    case 'boolean':
      if (typeof value !== 'boolean') return 'Must be true or false'
      break
    case 'string':
      if (typeof value !== 'string') return 'Must be text'
      break
    case 'array':
      if (!Array.isArray(value)) return 'Must be a JSON list'
      if (schema.maxItems !== undefined && value.length > schema.maxItems) return `Must have at most ${schema.maxItems} items`
      break
  }
  if (typeof value === 'number') {
    if (schema.minimum !== undefined && value < schema.minimum) return `Must be at least ${schema.minimum}`
    if (schema.maximum !== undefined && value > schema.maximum) return `Must be at most ${schema.maximum}`
  }
  if (typeof value === 'string') {
    if (schema.maxLength !== undefined && value.length > schema.maxLength) {
      return `Must be at most ${schema.maxLength} characters`
    }
    if (schema.pattern && !new RegExp(`^(?:${schema.pattern})$`).test(value)) {
      return 'Does not match the required format'
    }
    // An empty string reads as "not set" (and an empty peer means multicast). The server holds the
    // same rules (internal/configuration/schema_validate.go).
    if (value !== '') {
      if (schema.format === 'ipv4' && !IPV4.test(value)) return 'Must be an IPv4 address'
      if (schema.format === 'hostname' && !isHostName(value)) return 'Must be a host name or an IPv4 address'
      if (schema.format === 'endpoint' && !isHostName(value) && !isHostPort(value) && !isIPv6Literal(value)) {
        return 'Must be a host name or an IP address, optionally followed by :port (an IPv6 address takes its port in the Port field)'
      }
      if (schema.format === 'hostport' && !isHostPort(value)) {
        return 'Must be host:port, with a host name or IPv4 address and a port from 1 to 65535'
      }
    }
  }
  if (schema.enum && !schema.enum.some(option => option === value)) {
    return `Must be one of: ${schema.enum.join(', ')}`
  }
  return null
}

// isPlainObject is true for a JSON object, not for null or a list.
function isPlainObject(value: any): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

// requiredKeys returns the child keys an object schema requires for the effective configuration
// (the override over the inherited value): its own `required`, plus `then.required` when the
// `if` condition holds. The condition reads each `if.properties.<key>.const` against the
// effective value of that key.
function requiredKeys(
  schema: SchemaProperty,
  path: ConfigPath,
  overrides: ConfigBody,
  inherited: ConfigBody | null | undefined,
): string[] {
  const keys = [...(schema.required || [])]
  const conditions = Object.entries(schema.if?.properties || {})
  if (schema.then?.required && conditions.length > 0) {
    const holds = conditions.every(([key, rule]) => {
      const childPath = [...path, key]
      const effective = getAt(overrides, childPath) !== undefined
        ? getAt(overrides, childPath)
        : getAt(inherited, childPath)
      return rule.const !== undefined && effective === rule.const
    })
    if (holds) keys.push(...schema.then.required)
  }
  return keys
}

// validateOverrides walks the schema and returns the errors of the override body, keyed by
// dotted path. A section that holds an override must be an object. A required field fails only
// when its object holds an override, or `held` does (the overrides as they were saved, which no
// edit changes), and neither the override nor the inherited configuration gives the field a value.
// A section that was saved with an override and lost the last one by Reset is still checked: saving
// would remove what was saved, and the effective configuration must stay complete. A section
// nobody saved or touched is not checked, so a template that leaves it incomplete does not block Save.
//
// A template is one layer of the configuration: another layer can supply a field this one leaves
// out, and the server accepts such a body. With `layer` a field is therefore never required for
// being incomplete. A saved password is no exception: the server keeps a password the body leaves out.
export function validateOverrides(
  sections: Record<string, SchemaProperty>,
  overrides: ConfigBody,
  inherited: ConfigBody | null | undefined,
  held?: ConfigBody | null,
  layer?: boolean,
): Record<string, string> {
  const errors: Record<string, string> = {}
  const walk = (schema: SchemaProperty, path: ConfigPath) => {
    if (schema.type === 'object' && schema.properties) {
      const own = getAt(overrides, path)
      if (own !== undefined && !isPlainObject(own)) {
        errors[pathKey(path)] = 'Must be an object'
        return
      }
      for (const [key, child] of Object.entries(schema.properties)) {
        walk(child, [...path, key])
      }
      const required = layer ? [] : requiredKeys(schema, path, overrides, inherited)
      const touched = (node: any) => isPlainObject(node) && Object.keys(node).length > 0
      if (required.length > 0 && (touched(own) || touched(getAt(held, path)))) {
        for (const key of required) {
          const childPath = [...path, key]
          if (getAt(overrides, childPath) === undefined && getAt(inherited, childPath) === undefined) {
            errors[pathKey(childPath)] = 'Required'
          }
        }
      }
      return
    }
    // A list of channels in the body is checked item by item, each keyed by its id.
    const ids = schema.type === 'array' && schema.items?.properties && Array.isArray(getAt(overrides, path))
      ? listIds(getAt(overrides, path))
      : null
    if (ids) {
      ids.forEach(id => walk(schema.items!, [...path, itemKey(id)]))
      return
    }
    const value = getAt(overrides, path)
    const error = validateValue(schema, value)
    if (error) errors[pathKey(path)] = error
  }
  for (const [key, section] of Object.entries(sections)) {
    walk(section, [key])
  }
  return errors
}

// templateSchemaTarget names the schema a template form uses: the model's schema for a
// device_type template, the template schema for a global or group template. It is null while a
// device_type template has no device type yet, because no model schema exists for it.
export function templateSchemaTarget(
  scope: string | undefined,
  deviceType: string | undefined,
): { deviceType?: string; scope?: string } | null {
  if (scope === 'device_type') {
    const model = (deviceType || '').trim()
    return model ? { deviceType: model } : null
  }
  return { scope: scope || 'global' }
}

// The template API leaves a saved password out of the body and lists its path in stored_secrets
// (for example "wifi.ap.pass"). Older answers carry three flags instead (has_wifi_password,
// has_mqtt_password, has_auth_password), and one flag covers both Wi-Fi passwords. savedSecretBaseline
// turns the list, or the flags when there is no list, into a body that holds a mask at each such
// path, for the editor's `inherited` input: a field that requires a password is then not reported
// as missing while the password is saved. It is null when nothing is saved. The mask is never
// sent; it only satisfies the required check. A save that leaves a password out keeps it.
export const SAVED_SECRET_MASK = '********'

export function savedSecretBaseline(saved: {
  stored_secrets?: string[]
  has_wifi_password?: boolean
  has_mqtt_password?: boolean
  has_auth_password?: boolean
}): ConfigBody | null {
  const paths = saved.stored_secrets ?? [
    ...(saved.has_wifi_password ? ['wifi.password'] : []),
    ...(saved.has_mqtt_password ? ['mqtt.pass'] : []),
    ...(saved.has_auth_password ? ['auth.pass'] : []),
  ]
  let body: ConfigBody = {}
  for (const path of paths) body = setAt(body, path.split('.'), SAVED_SECRET_MASK)
  return Object.keys(body).length > 0 ? body : null
}

// secretPathsOf lists the path of every password field in a schema.
function secretPathsOf(properties: Record<string, SchemaProperty> | undefined, prefix: ConfigPath = []): ConfigPath[] {
  return Object.entries(properties ?? {}).flatMap(([key, prop]) =>
    prop.format === 'password' ? [[...prefix, key]] : secretPathsOf(prop.properties, [...prefix, key]),
  )
}

// maskedPathsOf lists the path of every mask in a saved body. The server writes the mask only for a
// secret, so this finds the secrets that the device schema leaves out (the AP password and key).
function maskedPathsOf(body: any, prefix: ConfigPath = []): ConfigPath[] {
  if (body === SAVED_SECRET_MASK) return [prefix]
  if (!isPlainObject(body)) return []
  return Object.entries(body).flatMap(([key, child]) => maskedPathsOf(child, [...prefix, key]))
}

// nullRemovedSecrets returns edited with null at each password field that the saved overrides hold
// (masked, or saved as an empty string) and the edited body no longer has. The server keeps a secret
// that a saved body leaves out, so null is how a removed override (Reset, or a deleted line in the
// JSON view) reaches it: the device then inherits the template secret again. A secret that the
// schema leaves out is found by its mask only, so a saved empty one of those is not covered.
export function nullRemovedSecrets(
  edited: ConfigBody,
  saved: ConfigBody | null | undefined,
  schema: { properties?: Record<string, SchemaProperty> } | null | undefined,
): ConfigBody {
  let body = edited
  for (const path of [...secretPathsOf(schema?.properties), ...maskedPathsOf(saved)]) {
    if (typeof getAt(saved, path) === 'string' && !hasAt(edited, path)) body = setAt(body, path, null)
  }
  return body
}

// countOverrides counts the leaf values set in the override body under path.
export function countOverrides(body: ConfigBody, path: ConfigPath): number {
  const node = getAt(body, path)
  if (node === undefined) return 0
  if (Array.isArray(node)) {
    const ids = node.length > 0 ? listIds(node) : null
    return ids ? ids.reduce((sum, id) => sum + countOverrides(body, [...path, itemKey(id)]), 0) : 1
  }
  if (node === null || typeof node !== 'object') return 1
  // The id of an item names it and is no override.
  const fields = Object.keys(node).filter(key => !(isItem(path[path.length - 1] ?? '') && key === 'id'))
  return fields.reduce((sum, key) => sum + countOverrides(body, [...path, key]), 0)
}

// formatLabel turns a key into words: "auto_on" is "Auto on", "maxPower" is "Max Power".
export function formatLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim()
}

export function formatValue(value: any): string {
  if (value === undefined) return ''
  if (typeof value === 'string') return value === '' ? '(empty)' : value
  return JSON.stringify(value)
}
