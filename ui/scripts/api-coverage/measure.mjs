// API coverage of the UI: how many functions exported by `ui/src/api` are referenced by another file.
// This is the measurement that section 1 of `docs/frontend/frontend-review.md` describes; the document
// holds no figure of it (`node ui/scripts/api-coverage/cli.mjs` prints the current table), and
// `ui/src/__tests__/api-coverage.test.ts` proves the measurement. It reads files with Node only, so it
// runs wherever the unit tests run, with no extra tool installed.
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
