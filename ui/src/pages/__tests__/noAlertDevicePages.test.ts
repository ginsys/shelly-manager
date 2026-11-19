import { describe, it, expect } from 'vitest'

// ginsys/shelly-manager#614, first group: the four device pages open no browser dialog with alert().
// Other pages still do and are tracked in the same issue, so this guard names these four.
const pages = import.meta.glob(
  [
    '../DevicesPage.vue',
    '../DeviceDetailPage.vue',
    '../DeviceConfigPage.vue',
    '../DeviceConfigHistoryPage.vue',
  ],
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>

// A word boundary before `alert`, so `window.alert(` and `alert (` match and `showalert(` does not.
const ALERT_CALL = /\balert\s*\(/

describe('device pages open no alert() dialog', () => {
  it('matcher control: it flags alert, window.alert and a spaced call, and skips an identifier that merely ends in alert', () => {
    expect(ALERT_CALL.test("window.alert('x')")).toBe(true)
    expect(ALERT_CALL.test("alert('x')")).toBe(true)
    expect(ALERT_CALL.test("alert ('x')")).toBe(true)
    expect(ALERT_CALL.test("showalert('x')")).toBe(false)
    expect(ALERT_CALL.test("alertCount(1)")).toBe(false)
  })

  it('reads all four pages', () => {
    expect(Object.keys(pages).sort()).toEqual([
      '../DeviceConfigHistoryPage.vue',
      '../DeviceConfigPage.vue',
      '../DeviceDetailPage.vue',
      '../DevicesPage.vue',
    ])
  })

  for (const [path, source] of Object.entries(pages)) {
    it(`${path} has no alert() call`, () => {
      const lines = source.split('\n')
      const hits = lines
        .map((line, i) => ({ line, n: i + 1 }))
        .filter(({ line }) => ALERT_CALL.test(line))
        .map(({ n, line }) => `${n}: ${line.trim()}`)
      expect(hits).toEqual([])
    })
  }
})
