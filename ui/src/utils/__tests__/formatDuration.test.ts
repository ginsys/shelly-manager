import { describe, it, expect } from 'vitest'
import { formatDuration } from '../formatDuration'

// The server sends a Go time.Duration, which JSON carries as a whole number of nanoseconds.
describe('formatDuration', () => {
  it('shows zero', () => {
    expect(formatDuration(0)).toBe('0 ms')
  })

  it('shows less than a millisecond as such', () => {
    expect(formatDuration(250_000)).toBe('< 1 ms')
  })

  it('shows milliseconds', () => {
    expect(formatDuration(42_000_000)).toBe('42 ms')
    expect(formatDuration(999_000_000)).toBe('999 ms')
  })

  it('shows seconds with one decimal', () => {
    expect(formatDuration(1_500_000_000)).toBe('1.5 s')
    expect(formatDuration(12_000_000_000)).toBe('12.0 s')
  })

  it('shows minutes and seconds', () => {
    expect(formatDuration(75_000_000_000)).toBe('1 min 15 s')
    expect(formatDuration(120_000_000_000)).toBe('2 min 0 s')
  })

  it('shows a dash for anything that is not a usable duration', () => {
    for (const bad of [undefined, null, '1.5s', NaN, Infinity, -1, {}]) {
      expect(formatDuration(bad)).toBe('—')
    }
  })
})
