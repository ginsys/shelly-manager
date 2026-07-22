/**
 * Pure logic for the vue-tsc baseline ratchet. No I/O, no process access —
 * everything here is a total function so it can be unit-tested directly.
 *
 * Paths are always `ui/`-relative POSIX (e.g. `src/components/Foo.vue`).
 */

import path from 'node:path'

/** Thrown for any condition the gate must refuse rather than tolerate. */
export class BaselineError extends Error {
  constructor(message) {
    super(message)
    this.name = 'BaselineError'
  }
}

// A diagnostic line: `path(line,col): error TS1234: message`.
// The filename capture is greedy so paths containing `(` still parse — the
// `(line,col)` group is anchored to the end of the coordinate pair.
const DIAGNOSTIC = /^(.+)\((\d+),(\d+)\): error TS(\d+): /

/**
 * Validate a `ui/`-relative path and return it in canonical POSIX form.
 *
 * Absoluteness is tested in BOTH platform dialects *before* normalising:
 * turning `\` into `/` first would make `C:\foo` look like the innocuous
 * relative path `C:/foo`.
 */
export function normalizeRelPath(raw, { label = 'path' } = {}) {
  if (typeof raw !== 'string' || raw.length === 0) {
    throw new BaselineError(`${label}: expected a non-empty string`)
  }
  if (path.isAbsolute(raw) || path.win32.isAbsolute(raw)) {
    throw new BaselineError(`${label}: must be relative to ui/, got absolute "${raw}"`)
  }

  const posix = raw.split('\\').join('/')
  const normalized = path.posix.normalize(posix)

  if (normalized === '..' || normalized.startsWith('../')) {
    throw new BaselineError(`${label}: escapes ui/, got "${raw}"`)
  }
  if (normalized.startsWith('/')) {
    throw new BaselineError(`${label}: must be relative to ui/, got "${raw}"`)
  }
  return normalized
}

/**
 * Parse `vue-tsc --noEmit --pretty false` stdout into { path: count }.
 *
 * Fail-closed and stateful:
 *  - a top-level diagnostic opens a block;
 *  - indented lines are continuations, valid ONLY inside an open block;
 *  - a blank line closes the block, so stray indented text after a blank
 *    cannot be absorbed as a continuation of an earlier diagnostic;
 *  - anything else throws, naming the line number and content.
 */
export function parseDiagnostics(stdout) {
  if (typeof stdout !== 'string') {
    throw new BaselineError('compiler output: expected a string')
  }

  const counts = new Map()
  const lines = stdout.split(/\r\n|\n|\r/)
  let inDiagnostic = false

  lines.forEach((line, index) => {
    const lineNo = index + 1

    if (line.trim() === '') {
      inDiagnostic = false
      return
    }

    const indented = /^\s/.test(line)
    if (indented) {
      if (!inDiagnostic) {
        throw new BaselineError(
          `Unrecognized vue-tsc output at line ${lineNo}: indented text before any diagnostic: ${JSON.stringify(line)}`,
        )
      }
      return
    }

    const match = DIAGNOSTIC.exec(line)
    if (!match) {
      throw new BaselineError(
        `Unrecognized vue-tsc output at line ${lineNo}: ${JSON.stringify(line)}`,
      )
    }

    const file = normalizeRelPath(match[1], { label: `diagnostic at line ${lineNo}` })
    counts.set(file, (counts.get(file) ?? 0) + 1)
    inDiagnostic = true
  })

  return Object.fromEntries(counts)
}

/** Canonical serialization: sorted keys, 2-space indent, trailing newline. */
export function serializeBaseline(counts) {
  const sorted = {}
  for (const key of Object.keys(counts).sort()) sorted[key] = counts[key]
  return `${JSON.stringify(sorted, null, 2)}\n`
}

/**
 * Parse and validate a baseline file's raw text.
 *
 * Rejects rather than coerces. Re-serializing and comparing against the input
 * also catches duplicate keys, unsorted keys and other non-canonical hand edits.
 */
export function loadBaseline(text) {
  if (typeof text !== 'string') {
    throw new BaselineError('baseline: expected a string')
  }

  let parsed
  try {
    parsed = JSON.parse(text)
  } catch (error) {
    throw new BaselineError(`baseline: invalid JSON (${error.message})`)
  }

  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new BaselineError('baseline: expected a plain JSON object')
  }

  for (const [key, value] of Object.entries(parsed)) {
    const normalized = normalizeRelPath(key, { label: `baseline key "${key}"` })
    if (normalized !== key) {
      throw new BaselineError(`baseline key "${key}": must be canonical, expected "${normalized}"`)
    }
    if (!Number.isInteger(value) || value <= 0) {
      throw new BaselineError(`baseline key "${key}": count must be a positive integer, got ${JSON.stringify(value)}`)
    }
  }

  const canonical = serializeBaseline(parsed)
  if (canonical !== text) {
    throw new BaselineError(
      'baseline: not canonically formatted (expected sorted keys, 2-space indent, trailing newline). ' +
        'Regenerate with `npm run typecheck:update-baseline`.',
    )
  }

  return parsed
}

/** Every path mentioned by either side, sorted — gives deterministic output. */
function allPaths(actual, baseline) {
  return [...new Set([...Object.keys(actual), ...Object.keys(baseline)])].sort()
}

/**
 * Strict comparison. Any divergence in either direction is a failure:
 * increases are regressions; decreases mean the baseline is stale and must be
 * regenerated in the same PR, which is what makes the ratchet permanent.
 */
export function compare(actual, baseline) {
  const differences = []

  for (const file of allPaths(actual, baseline)) {
    const actualCount = actual[file] ?? 0
    const expectedCount = baseline[file] ?? 0
    if (actualCount === expectedCount) continue

    let kind
    if (expectedCount === 0) kind = 'new-file'
    else if (actualCount === 0) kind = 'fixed'
    else if (actualCount > expectedCount) kind = 'increased'
    else kind = 'decreased'

    differences.push({ file, expected: expectedCount, actual: actualCount, kind })
  }

  const total = (counts) => Object.values(counts).reduce((sum, n) => sum + n, 0)

  return {
    ok: differences.length === 0,
    differences,
    actualTotal: total(actual),
    expectedTotal: total(baseline),
  }
}

/**
 * Decide what `--update` may write.
 *
 * Monotonic by construction: only reductions and removals are accepted, so the
 * standard update path can never bless a regression. Renames go through
 * explicit `--move` mappings, which the caller must have already validated
 * against the filesystem.
 */
export function planUpdate(actual, baseline, { moves = [], bootstrap = false } = {}) {
  if (bootstrap) {
    if (moves.length > 0) {
      throw new BaselineError('--move cannot be used while creating the initial baseline')
    }
    return { next: { ...actual } }
  }

  const seenFrom = new Set()
  const seenTo = new Set()
  const remapped = { ...baseline }

  for (const { from, to } of moves) {
    if (seenFrom.has(from)) throw new BaselineError(`--move: duplicate source "${from}"`)
    if (seenTo.has(to)) throw new BaselineError(`--move: duplicate destination "${to}"`)
    seenFrom.add(from)
    seenTo.add(to)

    if (!(from in baseline)) {
      throw new BaselineError(`--move: "${from}" is not in the baseline`)
    }
    if (to in baseline) {
      throw new BaselineError(`--move: "${to}" is already in the baseline`)
    }
    if ((actual[from] ?? 0) !== 0) {
      throw new BaselineError(`--move: "${from}" still reports ${actual[from]} error(s); it must be gone`)
    }
    if ((actual[to] ?? 0) > baseline[from]) {
      throw new BaselineError(
        `--move: "${to}" has ${actual[to] ?? 0} error(s), more than "${from}" had (${baseline[from]})`,
      )
    }

    delete remapped[from]
    remapped[to] = baseline[from]
  }

  const next = {}
  for (const file of allPaths(actual, remapped)) {
    const actualCount = actual[file] ?? 0
    const expectedCount = remapped[file] ?? 0

    if (!(file in remapped)) {
      if (actualCount > 0) {
        throw new BaselineError(
          `refusing to add "${file}" (${actualCount} error(s)) — the updater only accepts reductions. ` +
            'Fix the errors, or use --move for a rename.',
        )
      }
      continue
    }

    if (actualCount > expectedCount) {
      throw new BaselineError(
        `refusing to raise "${file}" from ${expectedCount} to ${actualCount} — the updater only accepts reductions.`,
      )
    }

    // Files that reached zero are dropped entirely.
    if (actualCount > 0) next[file] = actualCount
  }

  return { next }
}

/** Human-readable expected-vs-actual table for failures. */
export function formatDifferences(result) {
  const label = { 'new-file': 'new file', increased: 'increased', decreased: 'improved', fixed: 'fixed' }
  const width = Math.max(...result.differences.map((d) => d.file.length), 4)

  const rows = result.differences.map(
    (d) => `  ${d.file.padEnd(width)}  expected ${String(d.expected).padStart(3)}  actual ${String(d.actual).padStart(3)}  (${label[d.kind]})`,
  )

  return [
    `Type-check baseline mismatch (expected ${result.expectedTotal}, actual ${result.actualTotal}):`,
    '',
    ...rows,
  ].join('\n')
}
