import { describe, it, expect } from 'vitest'
import { shouldShowVersionBanner } from '../version-banner'

const FULL = '0123456789abcdef0123456789abcdef01234567'
const OTHER = 'fedcba9876543210fedcba9876543210fedcba98'

describe('shouldShowVersionBanner', () => {
  it('hides the banner when both sides carry the same revision', () => {
    expect(shouldShowVersionBanner(FULL, FULL)).toBe(false)
  })

  it('hides the banner when a short UI revision is the prefix of the full server revision', () => {
    expect(shouldShowVersionBanner(FULL.slice(0, 7), FULL)).toBe(false)
    expect(shouldShowVersionBanner(FULL, FULL.slice(0, 7))).toBe(false)
  })

  it('compares revisions case-insensitively and ignores surrounding space', () => {
    expect(shouldShowVersionBanner(` ${FULL.toUpperCase()} `, FULL)).toBe(false)
  })

  it('shows the banner when both revisions are real and differ', () => {
    expect(shouldShowVersionBanner(FULL, OTHER)).toBe(true)
    expect(shouldShowVersionBanner(FULL.slice(0, 7), OTHER)).toBe(true)
  })

  it('hides the banner when the server reports no revision', () => {
    expect(shouldShowVersionBanner(FULL, '')).toBe(false)
    expect(shouldShowVersionBanner(FULL, undefined)).toBe(false)
    expect(shouldShowVersionBanner(FULL, null)).toBe(false)
  })

  it('hides the banner when the UI reports no revision', () => {
    expect(shouldShowVersionBanner('', FULL)).toBe(false)
    expect(shouldShowVersionBanner(undefined, FULL)).toBe(false)
  })

  it('hides the banner for a dev UI against a server with a revision', () => {
    expect(shouldShowVersionBanner('dev', FULL)).toBe(false)
  })

  it('hides the banner for a dev server revision against a real UI revision', () => {
    expect(shouldShowVersionBanner(FULL, 'dev')).toBe(false)
    expect(shouldShowVersionBanner(FULL, 'unknown')).toBe(false)
  })

  it('hides the banner when neither side has a revision', () => {
    expect(shouldShowVersionBanner('dev', '')).toBe(false)
    expect(shouldShowVersionBanner('', '')).toBe(false)
  })
})
