import { describe, it, expect } from 'vitest'
import { generateSMAFile } from '../sma-generator'
import { parseSMAFile } from '../sma-parser'
import type { SMADevice } from '../sma-parser'

// Unmocked end-to-end guard for the SMA round-trip (#260). Uses REAL pako and
// REAL sha256 (no mocks), so it catches every layer the mocked suites hid:
//   - the old pako.gunzip / { to: 'string' } decompression bug,
//   - the checksum mismatch (generator hashed pre-checksum JSON, parser hashed
//     post-checksum JSON), which failed the default checksum-on parse, and
//   - byte-vs-code-unit size accounting for non-ASCII content.
describe('SMA generate -> parse round-trip (real pako + sha256)', () => {
  it('a generated archive with non-ASCII data parses back under full defaults', async () => {
    // Non-ASCII, non-empty record so record_count is nonzero and UTF-8 sizing
    // differs from UTF-16 .length.
    const device = {
      id: 1,
      mac: 'AA:BB:CC:DD:EE:FF',
      ip: '192.168.1.50',
      type: 'SHSW-1',
      name: 'Wohnzimmer – Lämpchen ☀',
      status: 'online',
      last_seen: '2026-01-01T00:00:00Z',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    } as SMADevice

    const gen = await generateSMAFile({ devices: [device] }, {})
    expect(gen.success, `generate error=${gen.error}`).toBe(true)
    expect(gen.blob).toBeDefined()

    const buffer = await gen.blob!.arrayBuffer()

    // No overrides: checksum AND structure validation both run.
    const res = await parseSMAFile(buffer)

    expect(res.success, `parse error=${res.error}`).toBe(true)
    expect(res.archive?.devices?.[0]?.name).toBe(device.name)
    // Generator and parser must agree on the uncompressed byte size.
    expect(res.parseInfo.originalSize).toBe(gen.metadata.originalSize)
    expect(res.parseInfo.originalSize).toBeGreaterThan(0)
  })
})
