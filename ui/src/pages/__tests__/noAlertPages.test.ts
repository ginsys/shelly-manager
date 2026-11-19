import { describe, it, expect } from 'vitest'

// ginsys/shelly-manager#614: no page opens a browser dialog with alert(). A failure shows on the page
// (an error panel, or the store's own error text), and a success shows as a line of text. This reads
// every page, so a new alert() fails here.
const pages = import.meta.glob('../*.vue', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>

// A word boundary before `alert`, so `window.alert(` and `alert (` match and `showalert(` does not.
const ALERT_CALL = /\balert\s*\(/

// The pages that held alert() calls before ginsys/shelly-manager#614 closed: the guard must read them.
const FORMER_ALERT_PAGES = [
  '../DriftReportsPage.vue',
  '../DriftSchedulesPage.vue',
  '../MetricsDashboardPage.vue',
  '../NotificationChannelDetailPage.vue',
  '../NotificationChannelsPage.vue',
  '../NotificationRulesPage.vue',
  '../ProvisioningAgentsPage.vue',
  '../ProvisioningTaskDetailPage.vue',
  '../ProvisioningTasksPage.vue',
  '../TemplateDetailPage.vue',
  '../TemplatesPage.vue',
]

describe('pages open no alert() dialog', () => {
  it('matcher control: it flags alert, window.alert and a spaced call, and skips an identifier that merely ends in alert', () => {
    expect(ALERT_CALL.test("window.alert('x')")).toBe(true)
    expect(ALERT_CALL.test("alert('x')")).toBe(true)
    expect(ALERT_CALL.test("alert ('x')")).toBe(true)
    expect(ALERT_CALL.test("showalert('x')")).toBe(false)
    expect(ALERT_CALL.test('alertCount(1)')).toBe(false)
  })

  it('reads every page, including the ones that used to call alert()', () => {
    const read = Object.keys(pages)
    for (const former of FORMER_ALERT_PAGES) expect(read).toContain(former)
    expect(read.length).toBeGreaterThanOrEqual(FORMER_ALERT_PAGES.length)
  })

  for (const [path, source] of Object.entries(pages)) {
    it(`${path} has no alert() call`, () => {
      const hits = source
        .split('\n')
        .map((line, i) => ({ line, n: i + 1 }))
        .filter(({ line }) => ALERT_CALL.test(line))
        .map(({ n, line }) => `${n}: ${line.trim()}`)
      expect(hits).toEqual([])
    })
  }
})
