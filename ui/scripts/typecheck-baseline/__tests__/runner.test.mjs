import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { BaselineError, serializeBaseline } from '../lib.mjs'
import { main, parseArgs, runCli } from '../runner.mjs'
import { isExecutedAsMain } from '../cli.mjs'

const BASELINE = '/virtual/typecheck-baseline.json'

/**
 * In-memory filesystem recording every write so "nothing was written" can be
 * asserted precisely, and so stray temp files are detectable.
 */
function makeFs(initial = {}) {
  const files = new Map(Object.entries(initial))
  const writes = []
  return {
    files,
    writes,
    async readFile(p) {
      if (!files.has(p)) {
        const err = new Error(`ENOENT: ${p}`)
        err.code = 'ENOENT'
        throw err
      }
      return files.get(p)
    },
    async writeFile(p, contents) {
      writes.push(p)
      files.set(p, contents)
    },
    async rename(from, to) {
      if (!files.has(from)) throw new Error(`ENOENT: ${from}`)
      files.set(to, files.get(from))
      files.delete(from)
    },
    async rm(p) { files.delete(p) },
    async access(p) {
      if (!files.has(p)) throw new Error(`ENOENT: ${p}`)
    },
  }
}

const silent = () => {}
const capture = () => {
  const lines = []
  return { sink: (msg) => lines.push(String(msg)), lines, text: () => lines.join('\n') }
}

const deps = (fs, compiler, out = capture(), err = capture()) => ({
  fs,
  compiler,
  log: out.sink,
  error: err.sink,
  baselinePath: BASELINE,
  uiRoot: '/virtual',
})

/** Temp files live beside the baseline and are prefixed `.typecheck-baseline.` */
const strayTempFiles = (fs) =>
  [...fs.files.keys()].filter((p) => p.includes('.typecheck-baseline.') && p.endsWith('.tmp'))

describe('main — check mode', () => {
  it('passes when actual matches the baseline', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 2 }) })
    const code = await main({}, deps(fs, async () => ({ 'src/a.vue': 2 })))
    assert.equal(code, 0)
    assert.deepEqual(fs.writes, [])
  })

  it('fails and writes nothing on a regression', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 2 }) })
    const err = capture()
    const code = await main({}, deps(fs, async () => ({ 'src/a.vue': 5 }), capture(), err))
    assert.equal(code, 1)
    assert.deepEqual(fs.writes, [])
    assert.match(err.text(), /may not be raised/)
  })

  it('fails on an improvement and asks for regeneration', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 5 }) })
    const err = capture()
    const code = await main({}, deps(fs, async () => ({ 'src/a.vue': 2 }), capture(), err))
    assert.equal(code, 1)
    assert.match(err.text(), /typecheck:update-baseline/)
    assert.deepEqual(fs.writes, [])
  })

  it('fails without creating a baseline when none exists', async () => {
    const fs = makeFs()
    const err = capture()
    const code = await main({}, deps(fs, async () => ({ 'src/a.vue': 1 }), capture(), err))
    assert.equal(code, 1)
    assert.deepEqual(fs.writes, [])
    assert.equal(fs.files.has(BASELINE), false)
    assert.match(err.text(), /No baseline/)
  })

  it('fails on a malformed baseline before running the compiler', async () => {
    const fs = makeFs({ [BASELINE]: '{ not json' })
    let compilerRan = false
    const code = await main({}, deps(fs, async () => { compilerRan = true; return {} }))
    assert.equal(code, 1)
    assert.equal(compilerRan, false, 'compiler must not run when the baseline is unreadable')
    assert.deepEqual(strayTempFiles(fs), [])
  })
})

describe('main — compiler failure modes', () => {
  const baselineFs = () => makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 2 }) })

  const failing = (message) => async () => { throw new BaselineError(message) }

  for (const [name, message] of [
    ['spawn failure', 'failed to start vue-tsc: ENOENT'],
    ['signal termination', 'vue-tsc was terminated by signal SIGKILL'],
    ['unexpected stderr', 'vue-tsc wrote to stderr:\ninternal error'],
    ['unexpected exit status', 'vue-tsc exited with unexpected status 137'],
    ['exit 0 with diagnostics', 'vue-tsc exited 0 but reported 3 diagnostic(s)'],
    ['exit 2 without diagnostics', 'vue-tsc exited 2 but reported no diagnostics'],
    ['missing compiler entry', 'vue-tsc not found at /virtual/vue-tsc.js'],
  ]) {
    it(`${name} fails without touching the baseline`, async () => {
      const fs = baselineFs()
      const before = fs.files.get(BASELINE)
      const err = capture()
      const code = await main({ update: true }, deps(fs, failing(message), capture(), err))
      assert.equal(code, 1)
      assert.deepEqual(fs.writes, [], 'nothing may be written')
      assert.equal(fs.files.get(BASELINE), before, 'baseline bytes must be unchanged')
      assert.deepEqual(strayTempFiles(fs), [], 'no temp files may remain')
    })
  }
})

describe('main — update mode', () => {
  it('creates the initial baseline when none exists', async () => {
    const fs = makeFs()
    const code = await main({ update: true }, deps(fs, async () => ({ 'src/a.vue': 2 })))
    assert.equal(code, 0)
    assert.equal(fs.files.get(BASELINE), serializeBaseline({ 'src/a.vue': 2 }))
    assert.deepEqual(strayTempFiles(fs), [])
  })

  it('applies reductions and drops fixed files', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 5, 'src/b.ts': 2 }) })
    const code = await main({ update: true }, deps(fs, async () => ({ 'src/a.vue': 1 })))
    assert.equal(code, 0)
    assert.equal(fs.files.get(BASELINE), serializeBaseline({ 'src/a.vue': 1 }))
  })

  it('refuses an increase and leaves the file untouched', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 2 }) })
    const before = fs.files.get(BASELINE)
    const code = await main({ update: true }, deps(fs, async () => ({ 'src/a.vue': 9 })))
    assert.equal(code, 1)
    assert.equal(fs.files.get(BASELINE), before)
    assert.deepEqual(fs.writes, [])
  })

  it('refuses a new file and leaves the file untouched', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 2 }) })
    const before = fs.files.get(BASELINE)
    const code = await main({ update: true }, deps(fs, async () => ({ 'src/a.vue': 2, 'src/new.ts': 1 })))
    assert.equal(code, 1)
    assert.equal(fs.files.get(BASELINE), before)
  })

  it('is a no-op when already current', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 2 }) })
    const code = await main({ update: true }, deps(fs, async () => ({ 'src/a.vue': 2 })))
    assert.equal(code, 0)
    assert.deepEqual(fs.writes, [], 'an unchanged baseline must not be rewritten')
  })

  it('cleans up the temp file and preserves the baseline when rename fails', async () => {
    const fs = makeFs({ [BASELINE]: serializeBaseline({ 'src/a.vue': 5 }) })
    const before = fs.files.get(BASELINE)
    fs.rename = async () => { throw new Error('EXDEV: rename failed') }

    await assert.rejects(
      () => main({ update: true }, deps(fs, async () => ({ 'src/a.vue': 1 }))),
      /rename failed/,
    )
    assert.equal(fs.files.get(BASELINE), before, 'baseline bytes must be unchanged')
    assert.deepEqual(strayTempFiles(fs), [], 'temp file must be cleaned up')
  })
})

describe('main — --move filesystem preconditions', () => {
  const setup = (extraFiles = {}) =>
    makeFs({ [BASELINE]: serializeBaseline({ 'src/old.vue': 3 }), ...extraFiles })

  it('accepts a genuine rename (source gone, destination present)', async () => {
    const fs = setup({ '/virtual/src/new.vue': 'x' })
    const code = await main(
      { update: true, moves: [{ from: 'src/old.vue', to: 'src/new.vue' }] },
      deps(fs, async () => ({ 'src/new.vue': 3 })),
    )
    assert.equal(code, 0)
    assert.equal(fs.files.get(BASELINE), serializeBaseline({ 'src/new.vue': 3 }))
  })

  it('refuses when the source file still exists on disk', async () => {
    // Without this check a "move" could shift debt onto an unrelated new file.
    const fs = setup({ '/virtual/src/old.vue': 'x', '/virtual/src/new.vue': 'x' })
    const before = fs.files.get(BASELINE)
    const err = capture()
    const code = await main(
      { update: true, moves: [{ from: 'src/old.vue', to: 'src/new.vue' }] },
      deps(fs, async () => ({ 'src/new.vue': 3 }), capture(), err),
    )
    assert.equal(code, 1)
    assert.match(err.text(), /still exists on disk/)
    assert.equal(fs.files.get(BASELINE), before)
  })

  it('refuses when the destination file does not exist on disk', async () => {
    const fs = setup()
    const err = capture()
    const code = await main(
      { update: true, moves: [{ from: 'src/old.vue', to: 'src/new.vue' }] },
      deps(fs, async () => ({ 'src/new.vue': 3 }), capture(), err),
    )
    assert.equal(code, 1)
    assert.match(err.text(), /does not exist on disk/)
  })
})

describe('parseArgs', () => {
  it('defaults to check mode', () => {
    assert.deepEqual(parseArgs([]), { update: false, moves: [] })
  })

  it('accepts --update', () => {
    assert.equal(parseArgs(['--update']).update, true)
  })

  it('accepts --move=old=new together with --update', () => {
    const o = parseArgs(['--update', '--move=src/old.vue=src/new.vue'])
    assert.deepEqual(o.moves, [{ from: 'src/old.vue', to: 'src/new.vue' }])
  })

  it('rejects --move without --update', () => {
    assert.throws(() => parseArgs(['--move=a.vue=b.vue']), (e) => e instanceof BaselineError && /only be used together with --update/.test(e.message))
  })

  it('rejects malformed --move', () => {
    assert.throws(() => parseArgs(['--update', '--move']), BaselineError)
    assert.throws(() => parseArgs(['--update', '--move=noequals']), BaselineError)
    assert.throws(() => parseArgs(['--update', '--move==b.vue']), BaselineError)
    assert.throws(() => parseArgs(['--update', '--move=a.vue=']), BaselineError)
  })

  it('rejects absolute or escaping --move paths', () => {
    assert.throws(() => parseArgs(['--update', '--move=/abs.vue=src/new.vue']), BaselineError)
    assert.throws(() => parseArgs(['--update', '--move=src/old.vue=../out.vue']), BaselineError)
  })

  it('rejects unknown flags', () => {
    assert.throws(() => parseArgs(['--force']), (e) => e instanceof BaselineError && /unknown argument/.test(e.message))
    assert.throws(() => parseArgs(['--allow-regression']), BaselineError)
  })
})

describe('runCli', () => {
  it('returns 1 and reports bad arguments without running the compiler', async () => {
    const fs = makeFs()
    let compilerRan = false
    const err = capture()
    const code = await runCli(['--bogus'], deps(fs, async () => { compilerRan = true; return {} }, capture(), err))
    assert.equal(code, 1)
    assert.equal(compilerRan, false)
    assert.match(err.text(), /unknown argument/)
  })
})

describe('cli entry-point guard', () => {
  it('is false when the module is merely imported', () => {
    assert.equal(isExecutedAsMain('file:///a/cli.mjs', '/b/other.mjs'), false)
    assert.equal(isExecutedAsMain('file:///a/cli.mjs', undefined), false)
  })

  it('is true when the module is the entry point', () => {
    assert.equal(isExecutedAsMain('file:///a/cli.mjs', '/a/cli.mjs'), true)
  })
})
