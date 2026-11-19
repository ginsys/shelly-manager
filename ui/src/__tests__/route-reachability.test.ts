import { describe, it, expect } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { routes } from '@/router/routes'

// A page nobody links to cannot be reached by clicking. This test reads the real route table and the
// real source, so a route added without a link (ginsys/shelly-manager#645 was exactly that) fails here.
// Detail pages (a path with `:`) are linked with a computed path and are out of scope: the Playwright
// spec `tests/e2e/nav-reachability.spec.ts` walks them. The test reads source text, not the rendered
// page: a link in a file that no page imports still counts, and the same spec is what proves the
// links are rendered.

interface Source {
  file: string // path relative to ui/src, with `/`
  text: string
}

// The static link forms the code uses: `to="/x"`, `to: '/x'`, `href="/x"`, `router.push('/x')`.
const linkRe = /(?:\bto="|\bto: '|\bhref="|\brouter\.(?:push|replace)\(\s*')(\/[^"'?#]*)/g

// What is not a link: a `<style>` block (a selector such as `.nav-link[to="/plugins"]` matches the
// pattern), an HTML or block comment, and a `//` comment, whole line or trailing (a `//` that does not
// follow a `:`, so the `//` of `https://` stays).
function withoutNonCode(text: string): string {
  return text
    .replace(/<style[\s\S]*?<\/style>/g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|[^:])\/\/.*$/gm, '$1')
}

/**
 * The routes with no `:` in their path that no file links to. A route's own page file does not count
 * as a link to it. `ownFile` maps a route path to that page file.
 */
export function unlinkedRoutes(
  paths: string[],
  sources: Source[],
  ownFile: Record<string, string>,
): string[] {
  const linksFrom = new Map<string, Set<string>>() // file -> paths it links to
  for (const { file, text } of sources) {
    const found = new Set<string>()
    for (const m of withoutNonCode(text).matchAll(linkRe)) found.add(m[1])
    linksFrom.set(file, found)
  }
  return paths
    .filter(p => !p.includes(':'))
    .filter(p => {
      for (const [file, found] of linksFrom) {
        if (file !== ownFile[p] && found.has(p)) return false
      }
      return true
    })
}

const srcDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

// Non-test `.vue` and `.ts` files under ui/src, as the coverage measurement counts them.
function readSources(dir: string, rel = ''): Source[] {
  const out: Source[] = []
  for (const entry of readdirSync(path.join(dir, rel), { withFileTypes: true })) {
    const relPath = rel ? `${rel}/${entry.name}` : entry.name
    if (entry.isDirectory()) {
      if (entry.name === '__tests__' || entry.name === 'test') continue
      out.push(...readSources(dir, relPath))
    } else if (/\.(vue|ts)$/.test(entry.name) && !/\.(test|spec)\.ts$/.test(entry.name)) {
      out.push({ file: relPath, text: readFileSync(path.join(dir, relPath), 'utf8') })
    }
  }
  return out
}

// Each route's page file, read from the `import('../pages/X.vue')` next to its path in routes.ts, so
// there is no second table to keep.
function pageFiles(routesSource: string): Record<string, string> {
  const re = /path: '([^']+)',\s*name: '[^']+',\s*component: \(\) => import\('\.\.\/(pages\/\w+\.vue)'\)/g
  const out: Record<string, string> = {}
  for (const m of routesSource.matchAll(re)) out[m[1]] = m[2]
  return out
}

describe('unlinkedRoutes (the checker)', () => {
  const sources: Source[] = [
    { file: 'layouts/MainLayout.vue', text: `<router-link to="/plugins">P</router-link>\n{ label: 'Rules', to: '/notifications/rules' }` },
    { file: 'pages/APage.vue', text: `<a href="/reports">r</a> router.push('/devices/1') router.push( '/other?x=1' )` },
    { file: 'pages/ReportsPage.vue', text: `<router-link to="/reports">me</router-link>` },
  ]
  const own = { '/reports': 'pages/ReportsPage.vue' }

  it('reports a route that nothing links to', () => {
    expect(unlinkedRoutes(['/orphan'], sources, own)).toEqual(['/orphan'])
  })

  it('does not count a page that links to itself', () => {
    const only = sources.filter(s => s.file === 'pages/ReportsPage.vue')
    expect(unlinkedRoutes(['/reports'], only, own)).toEqual(['/reports'])
  })

  it('accepts a link from the top bar, from another page, and a push with a query string', () => {
    expect(unlinkedRoutes(['/plugins', '/notifications/rules', '/reports', '/other'], sources, own)).toEqual([])
  })

  it('skips a route with a path parameter', () => {
    expect(unlinkedRoutes(['/devices/:id', '/:pathMatch(.*)*'], [], {})).toEqual([])
  })

  it('does not count a link in a style block, an HTML comment, a block comment or a // line', () => {
    const notLinks: Source[] = [
      { file: 'layouts/L.vue', text: '<style>\n.nav-link[to="/a"] { min-width: 32px; }\n</style>' },
      { file: 'pages/P.vue', text: '<!-- <router-link to="/b">b</router-link> -->' },
      { file: 'pages/Q.vue', text: '/* router.push(\'/c\') */\n  // <router-link to="/d">d</router-link>' },
      { file: 'pages/R.vue', text: "const ready = true // router.push('/e')" },
      { file: 'pages/S.vue', text: "const ready = true;// router.push('/f')" },
    ]
    expect(unlinkedRoutes(['/a', '/b', '/c', '/d', '/e', '/f'], notLinks, {})).toEqual(['/a', '/b', '/c', '/d', '/e', '/f'])
    // The same text outside the comments and the style block is a link.
    const links: Source[] = [{ file: 'layouts/L.vue', text: '<router-link to="/a">a</router-link>\n<style>\n.x {}\n</style>' }]
    expect(unlinkedRoutes(['/a'], links, {})).toEqual([])
  })

  it('does not read a dynamic or empty link as a static one', () => {
    const dynamic: Source[] = [{ file: 'pages/B.vue', text: ':to="`/x/${id}`" :to="crumb.to" to="" href="#top"' }]
    expect(unlinkedRoutes(['/x'], dynamic, {})).toEqual(['/x'])
  })
})

describe('Route reachability', () => {
  const routesSource = readFileSync(path.join(srcDir, 'router', 'routes.ts'), 'utf8')
  const own = pageFiles(routesSource)

  it('reads a page file for every route in the table', () => {
    // If the table's shape changes and the parser misses a route, its own page would count as a link.
    expect(Object.keys(own).sort()).toEqual(routes.map(r => r.path).sort())
  })

  it('links every route without a path parameter from the top bar or from another page', () => {
    const sources = readSources(srcDir)
    expect(sources.some(s => s.file === 'layouts/MainLayout.vue')).toBe(true)
    expect(
      unlinkedRoutes(routes.map(r => r.path), sources, own),
      'these routes have no top-bar entry and no link from another page',
    ).toEqual([])
  })
})
