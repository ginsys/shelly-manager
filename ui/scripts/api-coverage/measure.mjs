// API coverage of the UI: how many functions exported by `ui/src/api` are referenced by another file.
// This is the measurement that section 1 of `docs/frontend/frontend-review.md` describes and quotes;
// `ui/src/__tests__/api-coverage.test.ts` fails when the two differ. It reads files with Node only, so
// it runs wherever the unit tests run, with no extra tool installed.
//
// Definition (the same as the document's):
// - A function is *defined* when a non-test file under `ui/src/api` has `export function NAME` or
//   `export async function NAME` at the start of a line.
// - It is *referenced* when NAME appears as a whole word on a non-comment line of another non-test file
//   under `ui/src`. A line is a comment when its first characters after blanks are `//`, `*`, `/*` or
//   `<!--`. A function used only inside its own module is unreferenced.
// - Test files are `*.test.ts`, `*.spec.ts`, and anything under a `__tests__` or `test` directory.
// - Hidden files and files a `.gitignore` excludes are not read, as `rg` does not read them: in a git
//   checkout the file list is git's own (tracked or untracked, minus what an ignore rule matches, a
//   tracked file included); in a tree with no `.git` (a fixture) it is every file.
import { execFileSync } from 'node:child_process'
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'

const TEST_DIRS = new Set(['__tests__', 'test'])
const TEST_FILE = /\.(test|spec)\.ts$/
const DEFINITION = /^export (?:async )?function (\w+)/gm
const COMMENT_START = ['//', '*', '/*', '<!--']

// Every file under `dir`, as paths relative to `dir` with `/`.
function walk(dir, rel = '') {
  const out = []
  for (const entry of readdirSync(path.join(dir, rel), { withFileTypes: true })) {
    const relPath = rel ? `${rel}/${entry.name}` : entry.name
    if (entry.isDirectory()) out.push(...walk(dir, relPath))
    else if (entry.isFile()) out.push(relPath)
  }
  return out
}

// The files under `ui/src` that no `.gitignore` excludes, tracked or not, or null when `repoRoot` has
// no `.git` (a fixture tree). In a checkout a git failure is an error, never a quiet fall back to a
// walk that would read ignored files.
function gitFiles(repoRoot) {
  if (!existsSync(path.join(repoRoot, '.git'))) return null
  const list = (...flags) => {
    try {
      return execFileSync('git', ['-C', repoRoot, 'ls-files', '-z', ...flags, '--exclude-standard', '--', 'ui/src'], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
      })
        .split('\0')
        .filter(Boolean)
    } catch (error) {
      throw new Error(`api coverage: git ls-files failed in ${repoRoot} (git is needed to read the same files as rg): ${error.message}`)
    }
  }
  // `--cached` lists a tracked file even when an ignore rule matches it, as rg would not read it.
  const trackedButIgnored = new Set(list('--cached', '--ignored'))
  return list('--cached', '--others')
    .filter(file => !trackedButIgnored.has(file))
    .map(file => file.slice('ui/src/'.length))
    .filter(file => existsSync(path.join(repoRoot, 'ui', 'src', file)))
}

// The non-test, non-hidden, non-ignored files under `ui/src`, as paths relative to it, sorted.
function sourceFiles(repoRoot) {
  const files = gitFiles(repoRoot) ?? walk(path.join(repoRoot, 'ui', 'src'))
  return files
    .filter(file => {
      const parts = file.split('/')
      return (
        !parts.some(part => part.startsWith('.')) &&
        !parts.slice(0, -1).some(part => TEST_DIRS.has(part)) &&
        !TEST_FILE.test(parts[parts.length - 1])
      )
    })
    .sort()
}

function isCommentLine(line) {
  const start = line.trimStart()
  return COMMENT_START.some(prefix => start.startsWith(prefix))
}

/**
 * Measures the repository at `repoRoot`. Returns
 * `{ modules: [{ file, functions, referenced, unreferenced: [name] }], total, referenced, functions }`
 * where `functions` lists `{ name, file, referenced }` for every defined function.
 */
export function measureApiCoverage(repoRoot) {
  const src = path.join(repoRoot, 'ui', 'src')
  const files = sourceFiles(repoRoot).map(file => ({
    file,
    lines: readFileSync(path.join(src, file), 'utf8').split('\n'),
  }))

  const functions = []
  for (const { file, lines } of files.filter(f => f.file.startsWith('api/'))) {
    for (const match of lines.join('\n').matchAll(DEFINITION)) {
      functions.push({ name: match[1], file, referenced: false })
    }
  }

  for (const fn of functions) {
    const word = new RegExp(`(?<!\\w)${fn.name}(?!\\w)`)
    fn.referenced = files.some(
      other =>
        other.file !== fn.file &&
        other.lines.some(line => word.test(line) && !isCommentLine(line)),
    )
  }

  const byFile = new Map()
  for (const fn of functions) {
    if (!byFile.has(fn.file)) byFile.set(fn.file, [])
    byFile.get(fn.file).push(fn)
  }
  const modules = [...byFile].map(([file, fns]) => ({
    file: path.posix.basename(file),
    functions: fns.length,
    referenced: fns.filter(f => f.referenced).length,
    unreferenced: fns.filter(f => !f.referenced).map(f => f.name),
  }))
  return {
    modules,
    functions,
    total: functions.length,
    referenced: functions.filter(f => f.referenced).length,
  }
}

// The table `docs/frontend/frontend-review.md` section 1 holds, as the rows a person reads. The first
// cell names one or more `*.ts` files in backticks (a trailing "(...)" note is ignored).
function parseSection1(docText) {
  const start = docText.indexOf('## 1. API coverage')
  if (start < 0) return { error: 'section "## 1. API coverage" not found' }
  const end = docText.indexOf('\n---', start)
  const section = docText.slice(start, end < 0 ? undefined : end)

  const rows = []
  let total = null
  for (const line of section.split('\n')) {
    const cells = line.split('|').slice(1, -1).map(cell => cell.trim())
    if (cells.length !== 4) continue
    if (cells[0] === '**Total**') {
      total = {
        functions: Number(cells[1].replace(/\*/g, '')),
        referenced: Number(cells[2].replace(/\*/g, '')),
        unreferenced: Number(cells[3].replace(/\*/g, '')),
      }
      continue
    }
    const files = [...cells[0].split(' (')[0].matchAll(/`([\w.-]+\.ts)`/g)].map(m => m[1])
    if (files.length === 0 || !/^\d+$/.test(cells[1])) continue
    rows.push({
      files,
      functions: Number(cells[1]),
      referenced: Number(cells[2]),
      unreferenced: [...cells[3].matchAll(/`(\w+)`/g)].map(m => m[1]).sort(),
    })
  }

  return { rows, total }
}

/**
 * Compares the document with a measurement. Returns the differences as readable lines; an empty list
 * means every figure in section 1 and the Status line equals the measurement.
 */
export function checkApiCoverageDoc(docText, measurement) {
  const parsed = parseSection1(docText)
  if (parsed.error) return [parsed.error]
  const problems = []

  const measured = new Map(measurement.modules.map(m => [m.file, m]))
  const named = new Set()
  for (const row of parsed.rows) {
    const label = row.files.join(', ')
    const parts = row.files.map(file => measured.get(file))
    const missing = row.files.filter((file, i) => {
      named.add(file)
      return !parts[i]
    })
    if (missing.length > 0) {
      problems.push(`row ${label}: no exported function in ${missing.join(', ')}`)
      continue
    }
    const functions = parts.reduce((n, m) => n + m.functions, 0)
    const referenced = parts.reduce((n, m) => n + m.referenced, 0)
    const unreferenced = parts.flatMap(m => m.unreferenced).sort()
    if (row.functions !== functions) problems.push(`row ${label}: functions ${row.functions}, measured ${functions}`)
    if (row.referenced !== referenced) problems.push(`row ${label}: referenced ${row.referenced}, measured ${referenced}`)
    if (row.unreferenced.join(' ') !== unreferenced.join(' ')) {
      problems.push(`row ${label}: unreferenced [${row.unreferenced.join(', ')}], measured [${unreferenced.join(', ')}]`)
    }
  }
  for (const m of measurement.modules) {
    if (!named.has(m.file)) problems.push(`module ${m.file} (${m.functions} functions) has no row`)
  }

  if (!parsed.total) {
    problems.push('the Total row is missing')
  } else {
    const unreferenced = measurement.total - measurement.referenced
    if (parsed.total.functions !== measurement.total) problems.push(`Total functions ${parsed.total.functions}, measured ${measurement.total}`)
    if (parsed.total.referenced !== measurement.referenced) problems.push(`Total referenced ${parsed.total.referenced}, measured ${measurement.referenced}`)
    if (parsed.total.unreferenced !== unreferenced) problems.push(`Total unreferenced ${parsed.total.unreferenced}, measured ${unreferenced}`)
  }

  const percent = measurement.total === 0 ? 0 : Math.round((100 * measurement.referenced) / measurement.total)
  const wanted = `${measurement.referenced} of ${measurement.total} exported API functions are referenced by the UI (${percent}%)`
  // The Status line itself: a copy of the sentence elsewhere in the file (a comment, a quotation) does not count.
  const statusLine = docText.split('\n').find(line => line.startsWith('**Status:**'))
  if (statusLine === undefined) {
    problems.push('the Status line (a line that starts with "**Status:**") is missing')
  } else if (!statusLine.includes(wanted)) {
    problems.push(`the Status line does not say "${wanted}"`)
  }

  return problems
}
