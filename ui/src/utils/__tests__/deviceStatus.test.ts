import { describe, it, expect } from 'vitest'
import { shownDeviceStatus, LAST_SEEN_FRESH_MS, NOT_SEEN_RECENTLY } from '../deviceStatus'

const now = Date.parse('2026-11-20T12:00:00Z')
const seenAgo = (ms: number) => new Date(now - ms).toISOString()

describe('shownDeviceStatus', () => {
  it('shows online for a device seen inside the limit, and at the limit itself', () => {
    expect(shownDeviceStatus({ status: 'online', last_seen: seenAgo(60_000) }, now).label).toBe('online')
    expect(shownDeviceStatus({ status: 'online', last_seen: seenAgo(LAST_SEEN_FRESH_MS) }, now)).toEqual({
      label: 'online',
      cssClass: 'online',
    })
  })

  it('shows not seen recently one millisecond past the limit, with the last seen time as title', () => {
    const shown = shownDeviceStatus({ status: 'online', last_seen: seenAgo(LAST_SEEN_FRESH_MS + 1) }, now)
    expect(shown.label).toBe(NOT_SEEN_RECENTLY)
    expect(shown.cssClass).toBe('stale')
    expect(shown.title).toMatch(/^Last seen: /)
  })

  it('treats a missing, unreadable or zero last seen time as stale', () => {
    for (const last_seen of ['', 'garbage', '0001-01-01T00:00:00Z', undefined]) {
      const shown = shownDeviceStatus({ status: 'online', last_seen }, now)
      expect(shown.label, String(last_seen)).toBe(NOT_SEEN_RECENTLY)
      expect(shown.title, String(last_seen)).toBe('Last seen: never')
    }
  })

  it('shows any other stored status as stored, whatever last seen says', () => {
    expect(shownDeviceStatus({ status: 'offline', last_seen: seenAgo(0) }, now)).toEqual({
      label: 'offline',
      cssClass: 'offline',
    })
    expect(shownDeviceStatus({ status: '', last_seen: '' }, now)).toEqual({ label: 'unknown', cssClass: '' })
  })
})
