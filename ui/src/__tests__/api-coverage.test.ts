import { afterEach, describe, expect, it } from 'vitest'
import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { checkApiCoverageDoc, measureApiCoverage } from '../../scripts/api-coverage/measure.mjs'

// Section 1 of docs/frontend/frontend-review.md quotes the API coverage of the UI. The first block proves
// the measurement on a fixture tree and on four known functions; the second makes the document equal
// the measurement, so a change to `ui/src/api` or its callers that moves a figure fails here until the
// document is re-measured.

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..')
const docPath = path.join(repoRoot, 'docs', 'frontend', 'frontend-review.md')

const made: string[] = []
afterEach(() => {
  while (made.length > 0) rmSync(made.pop()!, { recursive: true, force: true })
})

function fixture(files: Record<string, string>): string {
  const root = mkdtempSync(path.join(tmpdir(), 'api-coverage-'))
  made.push(root)
  for (const [file, text] of Object.entries(files)) {
    const full = path.join(root, 'ui', 'src', file)
    mkdirSync(path.dirname(full), { recursive: true })
    writeFileSync(full, text)
  }
  return root
}

function find(result: ReturnType<typeof measureApiCoverage>, name: string) {
  return result.functions.find((f: { name: string }) => f.name === name)
}

describe('measureApiCoverage on a fixture tree', () => {
  const root = fixture({
    'api/things.ts': [
      'export function used() {}',
      'export async function usedAsync() {}',
      'export function onlyInLineComment() {}',
      'export function onlyInBlockComment() {}',
      'export function onlyInStarComment() {}',
      'export function onlyInHtmlComment() {}',
      'export function onlyInTest() {}',
      'export function onlyInOwnModule() {}',
      'export function prefixOfOther() {}',
      'const x = onlyInOwnModule()',
    ].join('\n'),
    'pages/Page.vue': [
      'import { used, usedAsync } from "@/api/things"',
      '// onlyInLineComment()',
      '  /* onlyInBlockComment */',
      ' * onlyInStarComment',
      '<!-- onlyInHtmlComment -->',
      'prefixOfOtherAndMore()',
    ].join('\n'),
    '__tests__/things.test.ts': 'onlyInTest()',
    'test/helper.ts': 'onlyInTest()',
    'pages/Page.spec.ts': 'onlyInTest()',
  })
  const result = measureApiCoverage(root)

  it('counts a name used on a code line, plain or async, as referenced', () => {
    expect(find(result, 'used')?.referenced).toBe(true)
    expect(find(result, 'usedAsync')?.referenced).toBe(true)
  })

  it('counts a name only in a comment, a test, its own module or a longer name as unreferenced', () => {
    for (const name of [
      'onlyInLineComment',
      'onlyInBlockComment',
      'onlyInStarComment',
      'onlyInHtmlComment',
      'onlyInTest',
      'onlyInOwnModule',
      'prefixOfOther',
    ]) {
      expect(find(result, name)?.referenced, name).toBe(false)
    }
    expect(result.total).toBe(9)
    expect(result.referenced).toBe(2)
  })
})

describe('measureApiCoverage in a git checkout', () => {
  // `rg` does not read a file a .gitignore excludes, so neither does the measurement (the repository's
  // own .gitignore excludes `coverage/` directories and `*.log`).
  it('does not read an ignored file, and reads the same file once it is not ignored', () => {
    const root = fixture({
      'api/demo.ts': 'export function onlyIgnored() {}',
      'coverage/fixture.ts': 'onlyIgnored()',
    })
    const git = (...args: string[]) => execFileSync('git', ['-C', root, ...args], { stdio: 'ignore' })
    git('init', '-q')
    writeFileSync(path.join(root, '.gitignore'), '**/coverage/\n')
    expect(find(measureApiCoverage(root), 'onlyIgnored')?.referenced).toBe(false)

    writeFileSync(path.join(root, '.gitignore'), '')
    expect(find(measureApiCoverage(root), 'onlyIgnored')?.referenced).toBe(true)
  })

  it('does not read a tracked file that a .gitignore rule matches', () => {
    const root = fixture({
      'api/demo.ts': 'export function onlyIgnored() {}',
      'coverage/fixture.ts': 'onlyIgnored()',
    })
    const git = (...args: string[]) => execFileSync('git', ['-C', root, ...args], { stdio: 'ignore' })
    git('init', '-q')
    writeFileSync(path.join(root, '.gitignore'), '**/coverage/\n')
    git('add', '-f', 'ui/src/coverage/fixture.ts')
    expect(find(measureApiCoverage(root), 'onlyIgnored')?.referenced).toBe(false)
  })
})

describe('measureApiCoverage on this repository', () => {
  const result = measureApiCoverage(repoRoot)

  // The four controls section 1 names. A measurement that gets one wrong is not trusted.
  it('counts bulkImport, used by the bulk dialog, as referenced', () => {
    expect(find(result, 'bulkImport')?.referenced).toBe(true)
  })

  it('counts rotateAdminKey, used by the admin page, as referenced', () => {
    expect(find(result, 'rotateAdminKey')?.referenced).toBe(true)
  })

  it('counts getBackupResult, named only in its own module and in a test, as unreferenced', () => {
    expect(find(result, 'getBackupResult')?.referenced).toBe(false)
  })

  it('finds every api module (a count of zero is a broken measurement)', () => {
    expect(result.modules.length).toBeGreaterThan(10)
    expect(result.total).toBeGreaterThan(100)
  })
})

describe('section 1 of the frontend review', () => {
  const doc = readFileSync(docPath, 'utf8')
  const result = measureApiCoverage(repoRoot)

  it('states the figures the measurement gives', () => {
    expect(
      checkApiCoverageDoc(doc, result),
      'docs/frontend/frontend-review.md section 1 differs from the measurement: run `node ui/scripts/api-coverage/cli.mjs`, update the table, the Status line and the header',
    ).toEqual([])
  })

  // The check itself is proven by changing one figure at a time in a copy of the text.
  it('reports a changed module count', () => {
    const changed = doc.replace(/(\| `devices\.ts` \| )10( \| )10/, '$111$211')
    expect(changed).not.toBe(doc)
    expect(checkApiCoverageDoc(changed, result).join('\n')).toContain('devices.ts')
  })

  it('reports a changed Total row', () => {
    const changed = doc.replace(/\| \*\*Total\*\* \| \*\*\d+\*\*/, '| **Total** | **1**')
    expect(changed).not.toBe(doc)
    expect(checkApiCoverageDoc(changed, result).join('\n')).toContain('Total functions 1')
  })

  it('reports a changed unreferenced name', () => {
    const changed = doc.replace('`getPlugin`', '`getPlugins`')
    expect(changed).not.toBe(doc)
    expect(checkApiCoverageDoc(changed, result).join('\n')).toContain('plugin.ts')
  })

  it('reports a module with no row', () => {
    const changed = doc.replace(/\| `errorBody\.ts` .*\n/, '')
    expect(changed).not.toBe(doc)
    expect(checkApiCoverageDoc(changed, result).join('\n')).toContain('module errorBody.ts')
  })

  it('reports a changed Status line', () => {
    const changed = doc.replace(/\*\*Status:\*\* \d+ of \d+/, '**Status:** 1 of 2')
    expect(changed).not.toBe(doc)
    expect(checkApiCoverageDoc(changed, result).join('\n')).toContain('Status line')
  })

  it('reports a wrong Status line even when the right sentence stands elsewhere in the file', () => {
    const sentence = `${result.referenced} of ${result.total} exported API functions are referenced by the UI`
    const changed = doc.replace(/\*\*Status:\*\* \d+ of \d+/, '**Status:** 1 of 2') + `\n<!-- ${sentence} (97%) -->\n`
    expect(changed).toContain(sentence)
    expect(checkApiCoverageDoc(changed, result).join('\n')).toContain('Status line')
  })

  it('reports a missing section', () => {
    expect(checkApiCoverageDoc('# nothing', result)).toHaveLength(1)
  })
})
