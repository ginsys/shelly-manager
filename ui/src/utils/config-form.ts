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

export function pathKey(path: ConfigPath): string {
  return path.join('.')
}

export function getAt(body: ConfigBody | null | undefined, path: ConfigPath): any {
  let node: any = body
  for (const key of path) {
    if (node === null || typeof node !== 'object' || Array.isArray(node)) return undefined
    node = node[key]
  }
  return node
}

export function hasAt(body: ConfigBody | null | undefined, path: ConfigPath): boolean {
  if (path.length === 0) return false
  const parent = getAt(body, path.slice(0, -1))
  return parent !== null && typeof parent === 'object' && !Array.isArray(parent) &&
    Object.prototype.hasOwnProperty.call(parent, path[path.length - 1])
}

// setAt returns a copy of body with value at path. It creates missing parent objects.
export function setAt(body: ConfigBody, path: ConfigPath, value: any): ConfigBody {
  const copy: ConfigBody = clone(body)
  let node: any = copy
  for (const key of path.slice(0, -1)) {
    if (node[key] === null || typeof node[key] !== 'object' || Array.isArray(node[key])) {
      node[key] = {}
    }
    node = node[key]
  }
  node[path[path.length - 1]] = value
  return copy
}

// deleteAt returns a copy of body without the key at path. Parent objects that become empty are
// removed too, so a reset field leaves no empty section in the override body.
export function deleteAt(body: ConfigBody, path: ConfigPath): ConfigBody {
  const copy: ConfigBody = clone(body)
  const parents: any[] = [copy]
  let node: any = copy
  for (const key of path.slice(0, -1)) {
    node = node?.[key]
    if (node === null || typeof node !== 'object' || Array.isArray(node)) return copy
    parents.push(node)
  }
  delete node[path[path.length - 1]]
  for (let i = path.length - 2; i >= 0; i--) {
    const child = parents[i + 1]
    if (Object.keys(child).length > 0) break
    delete parents[i][path[i]]
  }
  return copy
}

export function clone<T>(value: T): T {
  return value === undefined ? value : JSON.parse(JSON.stringify(value))
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
  }
  if (schema.enum && !schema.enum.some(option => option === value)) {
    return `Must be one of: ${schema.enum.join(', ')}`
  }
  return null
}

// validateOverrides walks the schema and returns the errors of the override body, keyed by
// dotted path. A required field fails only when its object holds an override and neither the
// override nor the inherited configuration gives the field a value.
export function validateOverrides(
  sections: Record<string, SchemaProperty>,
  overrides: ConfigBody,
  inherited: ConfigBody | null | undefined,
): Record<string, string> {
  const errors: Record<string, string> = {}
  const walk = (schema: SchemaProperty, path: ConfigPath) => {
    if (schema.type === 'object' && schema.properties) {
      for (const [key, child] of Object.entries(schema.properties)) {
        walk(child, [...path, key])
      }
      const required = schema.required
      const own = getAt(overrides, path)
      if (required && own !== null && typeof own === 'object' && Object.keys(own).length > 0) {
        for (const key of required) {
          const childPath = [...path, key]
          if (getAt(overrides, childPath) === undefined && getAt(inherited, childPath) === undefined) {
            errors[pathKey(childPath)] = 'Required'
          }
        }
      }
      return
    }
    const error = validateValue(schema, getAt(overrides, path))
    if (error) errors[pathKey(path)] = error
  }
  for (const [key, section] of Object.entries(sections)) {
    walk(section, [key])
  }
  return errors
}

// OVERRIDE_SOURCE is the layer name the backend gives saved device overrides in the
// desired-configuration sources map (internal/configuration/config_service.go).
export const OVERRIDE_SOURCE = 'device-override'

// inheritedBaseline returns the desired configuration without the values that came from the
// saved device overrides. What remains is what the device inherits from its templates, so a reset
// field shows the value it falls back to, not the override it had.
export function inheritedBaseline(
  desired: ConfigBody | null | undefined,
  sources: Record<string, string> | undefined,
): ConfigBody {
  let baseline: ConfigBody = clone(desired || {})
  for (const [key, source] of Object.entries(sources || {})) {
    if (source === OVERRIDE_SOURCE) baseline = deleteAt(baseline, key.split('.'))
  }
  return baseline
}

// countOverrides counts the leaf values set in the override body under path.
export function countOverrides(body: ConfigBody, path: ConfigPath): number {
  const node = getAt(body, path)
  if (node === undefined) return 0
  if (node === null || typeof node !== 'object' || Array.isArray(node)) return 1
  return Object.keys(node).reduce((sum, key) => sum + countOverrides(body, [...path, key]), 0)
}

export function formatValue(value: any): string {
  if (value === undefined) return ''
  if (typeof value === 'string') return value === '' ? '(empty)' : value
  return JSON.stringify(value)
}
