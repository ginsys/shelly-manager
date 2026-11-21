import { SAVED_SECRET_MASK, isSettings } from '@/utils/config-form'

// The difference between a stored version of the desired configuration and the current one
// (ginsys/shelly-manager#775). Both sides are walked down to their leaves, lists by position, so key
// order never matters. A stored version holds every secret masked and the current desired configuration
// holds none, so a mask is no difference where the current one has nothing at its path, or only an empty
// list or object there: the desired read keeps an empty `secret_urls` list, and the history masks a
// populated one.

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

type Leaf = { path: string[]; value: unknown }

// One entry per leaf: a scalar, or an empty list or object. The key is the path as a JSON list of parts, so
// a member name that holds a dot cannot collide with a nested one.
function leaves(value: unknown, path: string[], out: Map<string, Leaf>) {
  if (Array.isArray(value) && value.length > 0) {
    value.forEach((item, i) => leaves(item, [...path, String(i)], out))
  } else if (isSettings(value) && Object.keys(value).length > 0) {
    for (const [key, item] of Object.entries(value)) leaves(item, [...path, key], out)
  } else {
    out.set(JSON.stringify(path), { path, value })
  }
}

function flatten(tree: Record<string, any>) {
  const out = new Map<string, Leaf>()
  leaves(tree, [], out)
  return out
}

const isEmptyContainer = (value: unknown) =>
  (Array.isArray(value) && value.length === 0) || (isSettings(value) && Object.keys(value).length === 0)

export function diffDesired(target: Record<string, any>, current: Record<string, any>): DiffLine[] {
  const t = flatten(target)
  const c = flatten(current)
  const masked = [...t.values()].filter((l) => l.value === SAVED_SECRET_MASK).map((l) => l.path)
  const lines: DiffLine[] = []
  for (const [key, { path, value }] of t) {
    const there = c.get(key)
    if (!there) {
      if (value !== SAVED_SECRET_MASK) lines.push({ path: path.join('.'), kind: 'added', to: value })
    } else if (JSON.stringify(there.value) !== JSON.stringify(value)) {
      if (!(value === SAVED_SECRET_MASK && isEmptyContainer(there.value))) {
        lines.push({ path: path.join('.'), kind: 'changed', from: there.value, to: value })
      }
    }
  }
  for (const [key, { path, value }] of c) {
    if (t.has(key)) continue
    // An empty list or object here, with a mask at or under its path in the version, is a secret list.
    const coversMask = isEmptyContainer(value) && masked.some((m) => path.every((part, i) => m[i] === part))
    if (!coversMask) lines.push({ path: path.join('.'), kind: 'removed', from: value })
  }
  return lines
}
