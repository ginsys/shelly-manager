import type { PluginSchema, PluginSchemaProperty } from '@/api/plugin'

export const EMPTY_FIELD = Symbol('empty-field')
export const EMPTY_ENUM = Symbol('empty-enum')
export const ABSENT_NUMBER = Symbol('absent-number')

export type FormValue = unknown | typeof EMPTY_FIELD | typeof EMPTY_ENUM | typeof ABSENT_NUMBER
export type PluginFormValues = Record<string, FormValue>
export type PluginFormTouched = Record<string, boolean>

export interface FieldSchema extends PluginSchemaProperty {
  name: string
  required: boolean
}

export class PluginFormValidationError extends Error {
  constructor(public readonly errors: Record<string, string>) {
    super('Plugin configuration is invalid')
  }
}

export function toFieldSchema(schema: PluginSchema): FieldSchema[] {
  const required = new Set(schema.required ?? [])
  return Object.entries(schema.properties).map(([name, property]) => ({
    ...property,
    name,
    required: required.has(name),
  }))
}

function hasDefault(property: PluginSchemaProperty): boolean {
  return Object.prototype.hasOwnProperty.call(property, 'default')
}

function displayContainerDefault(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

export function initialPluginConfig(schema: PluginSchema): PluginFormValues {
  const required = new Set(schema.required ?? [])
  const values: PluginFormValues = {}
  for (const [name, property] of Object.entries(schema.properties)) {
    if (hasDefault(property)) {
      values[name] = property.type === 'array' || property.type === 'object'
        ? displayContainerDefault(property.default)
        : property.default
      continue
    }
    if (property.enum) {
      values[name] = EMPTY_ENUM
    } else if (property.type === 'boolean' && required.has(name)) {
      values[name] = false
    } else if (property.type === 'string' && required.has(name)) {
      values[name] = ''
    } else if (property.type === 'number' && required.has(name)) {
      values[name] = ABSENT_NUMBER
    } else {
      values[name] = EMPTY_FIELD
    }
  }
  return values
}

function sameEnumValue(left: unknown, right: unknown): boolean {
  return typeof left === typeof right && Object.is(left, right)
}

function validateTypedValue(
  name: string,
  property: PluginSchemaProperty,
  value: unknown,
): string | undefined {
  if (property.type === 'string') {
    if (typeof value !== 'string') return `${name} must be a string`
    if (property.pattern) {
      try {
        if (!new RegExp(property.pattern).test(value)) {
          return `${name} does not match the required pattern`
        }
      } catch {
        return `${name} has an invalid schema pattern`
      }
    }
  } else if (property.type === 'number') {
    if (typeof value !== 'number' || !Number.isFinite(value)) return `${name} must be a finite number`
    if (property.minimum !== undefined && value < property.minimum) {
      return `${name} must be at least ${property.minimum}`
    }
    if (property.maximum !== undefined && value > property.maximum) {
      return `${name} must be at most ${property.maximum}`
    }
  } else if (property.type === 'boolean') {
    if (typeof value !== 'boolean') return `${name} must be a boolean`
  } else if (property.type === 'array') {
    if (!Array.isArray(value)) return `${name} must contain a JSON array`
    if (property.items) {
      for (let index = 0; index < value.length; index++) {
        const error = validateTypedValue(`${name}[${index}]`, property.items, value[index])
        if (error) return error
      }
    }
  } else if (
    value === null
    || Array.isArray(value)
    || typeof value !== 'object'
  ) {
    return `${name} must contain a JSON object`
  } else if (property.properties) {
    const object = value as Record<string, unknown>
    for (const [childName, child] of Object.entries(property.properties)) {
      if (!Object.prototype.hasOwnProperty.call(object, childName)) continue
      const error = validateTypedValue(`${name}.${childName}`, child, object[childName])
      if (error) return error
    }
  }
  if (property.enum && !property.enum.some(option => sameEnumValue(option, value))) {
    return `${name} must be one of the advertised values`
  }
  return undefined
}

function parseField(
  name: string,
  property: PluginSchemaProperty,
  value: FormValue,
  required: boolean,
): { value?: unknown; error?: string } {
  if (value === EMPTY_FIELD || value === EMPTY_ENUM || value === ABSENT_NUMBER) {
    return required ? { error: `${name} is required` } : {}
  }

  let parsed = value
  if (property.type === 'array' || property.type === 'object') {
    if (typeof value !== 'string') {
      return { error: `${name} must be valid JSON text` }
    }
    try {
      parsed = JSON.parse(value)
    } catch {
      return { error: `${name} must contain valid JSON` }
    }
    if (property.type === 'array' && !Array.isArray(parsed)) {
      return { error: `${name} must contain a JSON array` }
    }
    if (
      property.type === 'object'
      && (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object')
    ) {
      return { error: `${name} must contain a JSON object` }
    }
  }

  if (property.type === 'string' && required && parsed === '') return { error: `${name} is required` }
  const typedError = validateTypedValue(name, property, parsed)
  if (typedError) return { error: typedError }
  return { value: parsed }
}

export function pluginFormErrors(
  schema: PluginSchema,
  values: PluginFormValues,
  touched: PluginFormTouched,
  showAll = false,
): Record<string, string> {
  const required = new Set(schema.required ?? [])
  const errors: Record<string, string> = {}
  for (const [name, property] of Object.entries(schema.properties)) {
    if (!showAll && !touched[name]) continue
    const result = parseField(name, property, values[name], required.has(name))
    if (result.error) errors[name] = result.error
  }
  return errors
}

export function pluginConfigFromForm(
  schema: PluginSchema,
  values: PluginFormValues,
  touched: PluginFormTouched,
): Record<string, unknown> {
  const required = new Set(schema.required ?? [])
  const errors: Record<string, string> = {}
  const config: Record<string, unknown> = {}
  for (const [name, property] of Object.entries(schema.properties)) {
    const include = required.has(name) || hasDefault(property) || touched[name]
    if (!include) continue
    const result = parseField(name, property, values[name], required.has(name))
    if (result.error) {
      errors[name] = result.error
    } else if ('value' in result) {
      config[name] = result.value
    }
  }
  if (Object.keys(errors).length) throw new PluginFormValidationError(errors)
  return config
}
