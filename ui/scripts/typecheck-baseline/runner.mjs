/**
 * I/O orchestration for the baseline ratchet.
 *
 * All side effects arrive through `deps` so the failure modes that matter most
 * — compiler crash, signal kill, unexpected stderr, write/rename failure — can
 * be asserted directly instead of via fake binaries.
 */

import { spawn } from 'node:child_process'
import fsp from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  BaselineError,
  compare,
  formatDifferences,
  loadBaseline,
  normalizeRelPath,
  parseDiagnostics,
  planUpdate,
  serializeBaseline,
} from './lib.mjs'

export const UI_ROOT = path.resolve(fileURLToPath(new URL('../..', import.meta.url)))
export const BASELINE_PATH = path.join(UI_ROOT, 'typecheck-baseline.json')
export const VUE_TSC_BIN = path.join(UI_ROOT, 'node_modules', 'vue-tsc', 'bin', 'vue-tsc.js')

/** vue-tsc exits 2 when it reports diagnostics, 0 when clean. Nothing else is valid. */
const EXIT_CLEAN = 0
const EXIT_DIAGNOSTICS = 2

/**
 * Run the pinned compiler with the current Node binary.
 *
 * Resolved absolutely rather than via PATH/npx so behaviour is identical under
 * `npm run`, a bare `node cli.mjs`, and the tests. No shell: filenames and move
 * arguments can never become shell input.
 */
async function runCompiler() {
  try {
    await fsp.access(VUE_TSC_BIN)
  } catch {
    throw new BaselineError(
      `vue-tsc not found at ${VUE_TSC_BIN} — run \`npm ci\` in ui/ first.`,
    )
  }

  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [VUE_TSC_BIN, '--noEmit', '--pretty', 'false'], {
      cwd: UI_ROOT,
      stdio: ['ignore', 'pipe', 'pipe'],
    })

    let stdout = ''
    let stderr = ''
    child.stdout.on('data', (chunk) => { stdout += chunk })
    child.stderr.on('data', (chunk) => { stderr += chunk })
    child.on('error', (error) => reject(new BaselineError(`failed to start vue-tsc: ${error.message}`)))
    child.on('close', (code, signal) => resolve({ code, signal, stdout, stderr }))
  })
}

/**
 * Validate the compiler result before anything is parsed, compared or written.
 * A crashed or killed compiler must never be mistaken for "no errors".
 */
function parseCompilerResult({ code, signal, stdout, stderr }) {
  if (signal) {
    throw new BaselineError(`vue-tsc was terminated by signal ${signal}`)
  }
  if (stderr && stderr.trim() !== '') {
    throw new BaselineError(`vue-tsc wrote to stderr:\n${stderr.trim()}`)
  }
  if (code !== EXIT_CLEAN && code !== EXIT_DIAGNOSTICS) {
    throw new BaselineError(`vue-tsc exited with unexpected status ${code}`)
  }

  const actual = parseDiagnostics(stdout)
  const total = Object.values(actual).reduce((sum, n) => sum + n, 0)

  if (code === EXIT_CLEAN && total > 0) {
    throw new BaselineError(`vue-tsc exited 0 but reported ${total} diagnostic(s)`)
  }
  if (code === EXIT_DIAGNOSTICS && total === 0) {
    throw new BaselineError('vue-tsc exited 2 but reported no diagnostics')
  }

  return actual
}

/** Atomic write: temp file beside the target, then rename; cleaned up on failure. */
async function writeBaselineAtomic(fs, targetPath, contents) {
  const dir = path.dirname(targetPath)
  const tmp = path.join(dir, `.typecheck-baseline.${process.pid}.${Math.random().toString(36).slice(2)}.tmp`)

  try {
    await fs.writeFile(tmp, contents, 'utf8')
    await fs.rename(tmp, targetPath)
  } finally {
    // If the rename succeeded this is a no-op; if anything failed it removes the
    // stray temp file. Either way the existing baseline is left untouched.
    await fs.rm(tmp, { force: true }).catch(() => {})
  }
}

/** Validate one `--move from=to` mapping against the filesystem. */
async function validateMove({ from, to }, fs, uiRoot) {
  const fromAbs = path.join(uiRoot, from)
  const toAbs = path.join(uiRoot, to)

  const exists = async (p) => {
    try { await fs.access(p); return true } catch { return false }
  }

  if (await exists(fromAbs)) {
    throw new BaselineError(`--move: source "${from}" still exists on disk; a move requires it to be gone`)
  }
  if (!(await exists(toAbs))) {
    throw new BaselineError(`--move: destination "${to}" does not exist on disk`)
  }
}

/**
 * @param {{update?: boolean, moves?: {from: string, to: string}[]}} options
 * @param {object} deps - injectable I/O: { compiler, fs, log, error }
 * @returns {Promise<number>} process exit code
 */
export async function main(options = {}, deps = {}) {
  const {
    compiler = async () => parseCompilerResult(await runCompiler()),
    fs = fsp,
    log = console.log,
    error = console.error,
    baselinePath = BASELINE_PATH,
    uiRoot = UI_ROOT,
  } = deps

  try {
    // Read the baseline first: a malformed baseline must fail before the
    // compiler runs and long before any temp file could be created.
    let baseline = null
    let baselineExists = true
    try {
      baseline = loadBaseline(await fs.readFile(baselinePath, 'utf8'))
    } catch (err) {
      if (err && err.code === 'ENOENT') {
        baselineExists = false
      } else {
        throw err
      }
    }

    const actual = await compiler()

    if (!baselineExists) {
      if (!options.update) {
        error(
          `No baseline at ${path.relative(UI_ROOT, baselinePath)}.\n` +
            'Create it with `npm run typecheck:update-baseline`.',
        )
        return 1
      }
      const { next } = planUpdate(actual, {}, { bootstrap: true })
      await writeBaselineAtomic(fs, baselinePath, serializeBaseline(next))
      const total = Object.values(next).reduce((sum, n) => sum + n, 0)
      log(`Created baseline with ${total} error(s) across ${Object.keys(next).length} file(s).`)
      return 0
    }

    if (options.update) {
      for (const move of options.moves ?? []) await validateMove(move, fs, uiRoot)

      const { next } = planUpdate(actual, baseline, { moves: options.moves ?? [] })
      const serialized = serializeBaseline(next)
      const current = serializeBaseline(baseline)

      if (serialized === current) {
        log('Baseline already up to date; nothing written.')
        return 0
      }

      await writeBaselineAtomic(fs, baselinePath, serialized)
      const before = Object.values(baseline).reduce((sum, n) => sum + n, 0)
      const after = Object.values(next).reduce((sum, n) => sum + n, 0)
      log(`Baseline updated: ${before} -> ${after} error(s) across ${Object.keys(next).length} file(s).`)
      return 0
    }

    const result = compare(actual, baseline)
    if (result.ok) {
      log(`Type-check baseline OK: ${result.actualTotal} known error(s) across ${Object.keys(baseline).length} file(s).`)
      return 0
    }

    error(formatDifferences(result))
    error('')
    error(
      result.differences.some((d) => d.kind === 'increased' || d.kind === 'new-file')
        ? 'New type errors were introduced. Fix them — the baseline may not be raised.'
        : 'Type errors were fixed. Lock the improvement in with `npm run typecheck:update-baseline` and commit the baseline.',
    )
    return 1
  } catch (err) {
    if (err instanceof BaselineError) {
      error(`typecheck-baseline: ${err.message}`)
      return 1
    }
    throw err
  }
}

/** Parse argv. Only `--update` and `--move from=to` are accepted. */
export function parseArgs(argv) {
  const options = { update: false, moves: [] }

  for (const arg of argv) {
    if (arg === '--update') {
      options.update = true
      continue
    }
    if (arg.startsWith('--move=') || arg.startsWith('--move')) {
      const value = arg.startsWith('--move=') ? arg.slice('--move='.length) : null
      if (value === null) {
        throw new BaselineError('--move requires `--move=<old>=<new>`')
      }
      const separator = value.indexOf('=')
      if (separator <= 0 || separator === value.length - 1) {
        throw new BaselineError(`--move: expected <old>=<new>, got ${JSON.stringify(value)}`)
      }
      options.moves.push({
        from: normalizeRelPath(value.slice(0, separator), { label: '--move source' }),
        to: normalizeRelPath(value.slice(separator + 1), { label: '--move destination' }),
      })
      continue
    }
    throw new BaselineError(`unknown argument ${JSON.stringify(arg)}`)
  }

  if (options.moves.length > 0 && !options.update) {
    throw new BaselineError('--move may only be used together with --update')
  }

  return options
}

/** Entry point used by cli.mjs. */
export async function runCli(argv, deps = {}) {
  const { error = console.error } = deps
  let options
  try {
    options = parseArgs(argv)
  } catch (err) {
    if (err instanceof BaselineError) {
      error(`typecheck-baseline: ${err.message}`)
      return 1
    }
    throw err
  }
  return main(options, deps)
}
