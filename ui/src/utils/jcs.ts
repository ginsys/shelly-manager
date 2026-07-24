const encoder = new TextEncoder()

function assertScalarString(value: string): void {
  for (let index = 0; index < value.length; index++) {
    const code = value.charCodeAt(index)
    if (code >= 0xd800 && code <= 0xdbff) {
      const low = value.charCodeAt(index + 1)
      if (!(low >= 0xdc00 && low <= 0xdfff)) throw new Error('lone high surrogate')
      index++
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      throw new Error('lone low surrogate')
    }
  }
}

function serialize(value: unknown, active: Set<object>, depth: number): string {
  if (value === null) return 'null'
  if (typeof value === 'string') {
    assertScalarString(value)
    return JSON.stringify(value)
  }
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) throw new Error('numbers must be finite')
    return JSON.stringify(value)
  }
  if (typeof value !== 'object') throw new Error(`unsupported JSON value: ${typeof value}`)
  if (depth > 64) throw new Error('maximum JSON depth 64 exceeded')
  if (active.has(value)) throw new Error('cycle detected')
  active.add(value)
  try {
    if (Array.isArray(value)) {
      return `[${value.map(item => serialize(item, active, depth + 1)).join(',')}]`
    }
    const object = value as Record<string, unknown>
    const keys = Object.keys(object)
    for (const key of keys) assertScalarString(key)
    keys.sort()
    return `{${keys.map(key => `${JSON.stringify(key)}:${serialize(object[key], active, depth + 1)}`).join(',')}}`
  } finally {
    active.delete(value)
  }
}

export function canonicalize(value: unknown): string {
  return serialize(value, new Set(), 1)
}

export function canonicalizeBytes(value: unknown): Uint8Array {
  return encoder.encode(canonicalize(value))
}
