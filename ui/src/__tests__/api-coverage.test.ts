import { afterEach, describe, expect, it } from 'vitest'
import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { measureApiCoverage } from '../../scripts/api-coverage/measure.mjs'

// Section 1 of docs/frontend/frontend-review.md describes the API coverage of the UI and holds no figure
// of it: `node ui/scripts/api-coverage/cli.mjs` prints the current table. The first blocks prove the
// measurement on a fixture tree and on four known functions; the last keeps one fact, the names of the
// functions no file references, so wiring one or orphaning one is a visible edit.

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..')

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

// The API functions no file references. A function that gains its first caller, or loses its last one,
// changes the measurement and fails the case below until this list says so: reference the function,
// delete it, or change the list. Names only; the counts are never committed.
const NO_UI_CALLER = [
  'downloadExport',
  'getBackupResult',
  'getPlugin',
  'isDashboardMessage',
  'updateDeviceConfig',
]

describe('the API functions with no UI caller', () => {
  it('are the ones NO_UI_CALLER names', () => {
    const measured = measureApiCoverage(repoRoot)
      .functions.filter((f: { referenced: boolean }) => !f.referenced)
      .map((f: { name: string }) => f.name)
      .sort()
    const wiredOrGone = NO_UI_CALLER.filter(name => !measured.includes(name))
    const orphaned = measured.filter((name: string) => !NO_UI_CALLER.includes(name))
    expect(
      { wiredOrGone, orphaned },
      'wiredOrGone: named in NO_UI_CALLER but now referenced, or no longer exported (remove it from the list); orphaned: unreferenced but not in the list (reference it, delete it, or add it to NO_UI_CALLER)',
    ).toEqual({ wiredOrGone: [], orphaned: [] })
  })
})
