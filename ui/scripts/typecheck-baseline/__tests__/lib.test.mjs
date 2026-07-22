import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  BaselineError,
  compare,
  formatDifferences,
  loadBaseline,
  normalizeRelPath,
  parseDiagnostics,
  planUpdate,
  serializeBaseline,
} from '../lib.mjs'

const diag = (file, line = 1, col = 1, code = 2322) =>
  `${file}(${line},${col}): error TS${code}: Type 'a' is not assignable to type 'b'.`

describe('parseDiagnostics', () => {
  it('counts diagnostics per file', () => {
    const out = [diag('src/a.vue'), diag('src/a.vue', 2), diag('src/b.ts')].join('\n')
    assert.deepEqual(parseDiagnostics(out), { 'src/a.vue': 2, 'src/b.ts': 1 })
  })

  it('accepts indented continuation lines inside a diagnostic block', () => {
    const out = [diag('src/a.vue'), "  Type 'x' is not assignable.", '    Nested detail.'].join('\n')
    assert.deepEqual(parseDiagnostics(out), { 'src/a.vue': 1 })
  })

  it('treats a blank line as closing the block, so later indented text is rejected', () => {
    const out = [diag('src/a.vue'), '', '  stray indented text'].join('\n')
    assert.throws(() => parseDiagnostics(out), (e) => e instanceof BaselineError && /line 3/.test(e.message))
  })

  it('rejects indented text before any diagnostic', () => {
    assert.throws(
      () => parseDiagnostics('  leading indented text\n'),
      (e) => e instanceof BaselineError && /line 1/.test(e.message) && /before any diagnostic/.test(e.message),
    )
  })

  it('rejects unrecognized top-level output, naming line and content', () => {
    const out = [diag('src/a.vue'), 'Something unexpected happened'].join('\n')
    assert.throws(
      () => parseDiagnostics(out),
      (e) => e instanceof BaselineError && /line 2/.test(e.message) && /Something unexpected/.test(e.message),
    )
  })

  it('handles CRLF input', () => {
    const out = [diag('src/a.vue'), "  detail", diag('src/b.ts')].join('\r\n')
    assert.deepEqual(parseDiagnostics(out), { 'src/a.vue': 1, 'src/b.ts': 1 })
  })

  it('parses filenames containing parentheses (greedy capture)', () => {
    assert.deepEqual(parseDiagnostics(diag('src/we(i)rd.vue')), { 'src/we(i)rd.vue': 1 })
  })

  it('returns an empty map for clean output', () => {
    assert.deepEqual(parseDiagnostics(''), {})
  })

  it('rejects absolute diagnostic paths', () => {
    assert.throws(() => parseDiagnostics(diag('/etc/passwd.ts')), BaselineError)
  })
})

describe('normalizeRelPath', () => {
  it('rejects POSIX absolute paths', () => {
    assert.throws(() => normalizeRelPath('/abs/x.ts'), BaselineError)
  })

  it('rejects Windows absolute paths before separator normalization', () => {
    // Normalizing first would turn this into the innocuous-looking "C:/foo.ts".
    assert.throws(() => normalizeRelPath('C:\\foo.ts'), BaselineError)
  })

  it('rejects paths escaping ui/', () => {
    assert.throws(() => normalizeRelPath('../outside.ts'), BaselineError)
  })

  it('normalizes backslashes to POSIX', () => {
    assert.equal(normalizeRelPath('src\\a\\b.ts'), 'src/a/b.ts')
  })
})

describe('loadBaseline', () => {
  const canonical = serializeBaseline({ 'src/b.ts': 1, 'src/a.vue': 2 })

  it('accepts a canonical baseline', () => {
    assert.deepEqual(loadBaseline(canonical), { 'src/a.vue': 2, 'src/b.ts': 1 })
  })

  it('rejects invalid JSON', () => {
    assert.throws(() => loadBaseline('{nope'), BaselineError)
  })

  it('rejects non-objects', () => {
    assert.throws(() => loadBaseline('[]\n'), BaselineError)
    assert.throws(() => loadBaseline('null\n'), BaselineError)
  })

  it('rejects zero and negative and non-integer counts', () => {
    assert.throws(() => loadBaseline('{\n  "src/a.vue": 0\n}\n'), BaselineError)
    assert.throws(() => loadBaseline('{\n  "src/a.vue": -1\n}\n'), BaselineError)
    assert.throws(() => loadBaseline('{\n  "src/a.vue": 1.5\n}\n'), BaselineError)
  })

  it('rejects unsorted (non-canonical) keys', () => {
    assert.throws(
      () => loadBaseline('{\n  "src/b.ts": 1,\n  "src/a.vue": 2\n}\n'),
      (e) => e instanceof BaselineError && /canonical/.test(e.message),
    )
  })

  it('rejects a missing trailing newline', () => {
    assert.throws(() => loadBaseline(canonical.trimEnd()), BaselineError)
  })

  it('rejects absolute or escaping keys', () => {
    assert.throws(() => loadBaseline('{\n  "/abs.ts": 1\n}\n'), BaselineError)
  })
})

describe('compare', () => {
  it('passes on an exact match', () => {
    const r = compare({ 'src/a.vue': 2 }, { 'src/a.vue': 2 })
    assert.equal(r.ok, true)
    assert.deepEqual(r.differences, [])
  })

  it('flags an increased count', () => {
    const r = compare({ 'src/a.vue': 3 }, { 'src/a.vue': 2 })
    assert.equal(r.ok, false)
    assert.equal(r.differences[0].kind, 'increased')
  })

  it('flags a new file', () => {
    const r = compare({ 'src/new.ts': 1 }, {})
    assert.equal(r.differences[0].kind, 'new-file')
  })

  it('flags a decreased count', () => {
    const r = compare({ 'src/a.vue': 1 }, { 'src/a.vue': 2 })
    assert.equal(r.differences[0].kind, 'decreased')
  })

  it('flags a baseline file that reached zero', () => {
    const r = compare({}, { 'src/a.vue': 2 })
    assert.equal(r.differences[0].kind, 'fixed')
  })

  it('orders multiple differences deterministically by path', () => {
    const r = compare({ 'src/z.ts': 1, 'src/a.ts': 1 }, {})
    assert.deepEqual(r.differences.map((d) => d.file), ['src/a.ts', 'src/z.ts'])
  })

  it('reports totals', () => {
    const r = compare({ 'src/a.vue': 1 }, { 'src/a.vue': 2, 'src/b.ts': 1 })
    assert.equal(r.actualTotal, 1)
    assert.equal(r.expectedTotal, 3)
  })
})

describe('planUpdate', () => {
  it('bootstraps from actual when no baseline exists', () => {
    const { next } = planUpdate({ 'src/a.vue': 2 }, {}, { bootstrap: true })
    assert.deepEqual(next, { 'src/a.vue': 2 })
  })

  it('refuses --move during bootstrap', () => {
    assert.throws(
      () => planUpdate({}, {}, { bootstrap: true, moves: [{ from: 'a', to: 'b' }] }),
      BaselineError,
    )
  })

  it('accepts reductions', () => {
    const { next } = planUpdate({ 'src/a.vue': 1 }, { 'src/a.vue': 3 })
    assert.deepEqual(next, { 'src/a.vue': 1 })
  })

  it('drops files that reached zero', () => {
    const { next } = planUpdate({}, { 'src/a.vue': 3 })
    assert.deepEqual(next, {})
  })

  it('refuses an increase', () => {
    assert.throws(
      () => planUpdate({ 'src/a.vue': 4 }, { 'src/a.vue': 3 }),
      (e) => e instanceof BaselineError && /only accepts reductions/.test(e.message),
    )
  })

  it('refuses a new file', () => {
    assert.throws(
      () => planUpdate({ 'src/new.ts': 1 }, { 'src/a.vue': 3 }),
      (e) => e instanceof BaselineError && /refusing to add/.test(e.message),
    )
  })

  describe('--move', () => {
    const baseline = { 'src/old.vue': 3 }

    it('transfers the entry on a valid rename', () => {
      const { next } = planUpdate({ 'src/new.vue': 3 }, baseline, {
        moves: [{ from: 'src/old.vue', to: 'src/new.vue' }],
      })
      assert.deepEqual(next, { 'src/new.vue': 3 })
    })

    it('writes the lower count when the move also improved', () => {
      const { next } = planUpdate({ 'src/new.vue': 1 }, baseline, {
        moves: [{ from: 'src/old.vue', to: 'src/new.vue' }],
      })
      assert.deepEqual(next, { 'src/new.vue': 1 })
    })

    it('refuses when the source still has errors', () => {
      assert.throws(
        () => planUpdate({ 'src/old.vue': 3, 'src/new.vue': 1 }, baseline, {
          moves: [{ from: 'src/old.vue', to: 'src/new.vue' }],
        }),
        (e) => e instanceof BaselineError && /must be gone/.test(e.message),
      )
    })

    it('refuses when the destination has more errors than the source had', () => {
      assert.throws(
        () => planUpdate({ 'src/new.vue': 4 }, baseline, {
          moves: [{ from: 'src/old.vue', to: 'src/new.vue' }],
        }),
        (e) => e instanceof BaselineError && /more than/.test(e.message),
      )
    })

    it('refuses an unknown source', () => {
      assert.throws(
        () => planUpdate({}, baseline, { moves: [{ from: 'src/ghost.vue', to: 'src/new.vue' }] }),
        BaselineError,
      )
    })

    it('refuses a destination already in the baseline', () => {
      assert.throws(
        () => planUpdate({}, { 'src/old.vue': 3, 'src/new.vue': 1 }, {
          moves: [{ from: 'src/old.vue', to: 'src/new.vue' }],
        }),
        BaselineError,
      )
    })

    it('refuses duplicate sources and destinations', () => {
      assert.throws(
        () => planUpdate({}, baseline, {
          moves: [{ from: 'src/old.vue', to: 'src/a.vue' }, { from: 'src/old.vue', to: 'src/b.vue' }],
        }),
        (e) => e instanceof BaselineError && /duplicate source/.test(e.message),
      )
      assert.throws(
        () => planUpdate({}, { 'src/x.vue': 1, 'src/y.vue': 1 }, {
          moves: [{ from: 'src/x.vue', to: 'src/same.vue' }, { from: 'src/y.vue', to: 'src/same.vue' }],
        }),
        (e) => e instanceof BaselineError && /duplicate destination/.test(e.message),
      )
    })

    it('cannot conceal an unrelated regression', () => {
      assert.throws(
        () => planUpdate({ 'src/new.vue': 3, 'src/unrelated.ts': 2 }, baseline, {
          moves: [{ from: 'src/old.vue', to: 'src/new.vue' }],
        }),
        (e) => e instanceof BaselineError && /refusing to add "src\/unrelated.ts"/.test(e.message),
      )
    })
  })
})

describe('serializeBaseline', () => {
  it('sorts keys and ends with a newline', () => {
    const out = serializeBaseline({ b: 1, a: 2 })
    assert.equal(out, '{\n  "a": 2,\n  "b": 1\n}\n')
  })
})

describe('formatDifferences', () => {
  it('renders an expected-vs-actual table', () => {
    const text = formatDifferences(compare({ 'src/a.vue': 3 }, { 'src/a.vue': 2 }))
    assert.match(text, /expected\s+2/)
    assert.match(text, /actual\s+3/)
    assert.match(text, /src\/a\.vue/)
  })
})
