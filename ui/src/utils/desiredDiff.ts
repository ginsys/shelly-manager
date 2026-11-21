import { SAVED_SECRET_MASK, isSettings } from '@/utils/config-form'

// The difference between a stored version of the desired configuration and the current one
// (ginsys/shelly-manager#775). Both sides are walked down to their leaves, lists by position, so key
// order never matters. A stored version holds every secret masked and the current desired
// configuration holds none, so a mask with nothing at its path in the current one is no difference.

export type DiffLine = { path: string; kind: 'added' | 'removed' | 'changed'; from?: unknown; to?: unknown }

// The settings a stored text holds, or null for no text, text that is not JSON (the server masks a text it
// cannot read as the bare mask) and JSON that is not an object.
export function parseVersion(text: string | null): Record<string, any> | null {
  if (text === null) return null
  try {
    const value = JSON.parse(text)
    return isSettings(value) ? value : null
  } catch {
    return null
  }
}

// One entry per leaf: a scalar, or an empty list or object. The key is the path as a JSON list of parts, so
// a member name that holds a dot cannot collide with a nested one.
function leaves(value: unknown, path: string[], out: Map<string, { path: string[]; value: unknown }>) {
  if (Array.isArray(value) && value.length > 0) {
    value.forEach((item, i) => leaves(item, [...path, String(i)], out))
  } else if (isSettings(value) && Object.keys(value).length > 0) {
    for (const [key, item] of Object.entries(value)) leaves(item, [...path, key], out)
  } else {
    out.set(JSON.stringify(path), { path, value })
  }
}

function flatten(tree: Record<string, any>) {
  const out = new Map<string, { path: string[]; value: unknown }>()
  leaves(tree, [], out)
  return out
}

export function diffDesired(target: Record<string, any>, current: Record<string, any>): DiffLine[] {
  const t = flatten(target)
  const c = flatten(current)
  const lines: DiffLine[] = []
  for (const [key, { path, value }] of t) {
    const there = c.get(key)
    if (!there) {
      if (value !== SAVED_SECRET_MASK) lines.push({ path: path.join('.'), kind: 'added', to: value })
    } else if (JSON.stringify(there.value) !== JSON.stringify(value)) {
      lines.push({ path: path.join('.'), kind: 'changed', from: there.value, to: value })
    }
  }
  for (const [key, { path, value }] of c) {
    if (!t.has(key)) lines.push({ path: path.join('.'), kind: 'removed', from: value })
  }
  return lines
}
