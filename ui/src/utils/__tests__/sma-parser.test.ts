import { describe, expect, it } from 'vitest'
import { generateSMAFile } from '../sma-generator'
import { parseSMAFile, parseSMAFromFile } from '../sma-parser'
import { validDevice } from './sma-fixture'

describe('strict SMA 2026.1 parser', () => {
  it('parses generated gzip and verifies integrity', async () => {
    const generated = await generateSMAFile({ devices: [validDevice] })
    const result = await parseSMAFile(await generated.blob!.arrayBuffer())
    expect(result.success, result.error).toBe(true)
    expect(result.archive?.format_version).toBe('2026.1')
    expect(result.archive?.devices[0].name).toBe('Kitchen')
  })

  it('accepts raw JSON as an import representation', async () => {
    const generated = await generateSMAFile({ devices: [validDevice] })
    const { ungzip } = await import('pako')
    const raw = ungzip(new Uint8Array(await generated.blob!.arrayBuffer()))
    const buffer = raw.buffer.slice(raw.byteOffset, raw.byteOffset + raw.byteLength) as ArrayBuffer
    const result = await parseSMAFile(buffer)
    expect(result.success, result.error).toBe(true)
  })

  it('applies maxSizeBytes to normalized raw and gzip data', async () => {
    const generated = await generateSMAFile({ devices: [validDevice] })
    const result = await parseSMAFile(await generated.blob!.arrayBuffer(), { maxSizeBytes: 8 })
    expect(result.success).toBe(false)
    expect(result.error).toContain('configured limit')
  })

  it('rejects malformed gzip, invalid UTF-8, and legacy versions', async () => {
    const malformed = new Uint8Array([0x1f, 0x8b, 0, 1]).buffer
    expect((await parseSMAFile(malformed)).success).toBe(false)
    expect((await parseSMAFile(new Uint8Array([0xff]).buffer)).success).toBe(false)
    const legacy = new TextEncoder().encode('{"format_version":"2024.1"}')
    expect((await parseSMAFile(legacy.buffer)).error).toContain('2026.1')
  })

  it('reads File objects', async () => {
    const generated = await generateSMAFile({ devices: [validDevice] })
    const file = new File([generated.blob!], 'archive.sma')
    expect((await parseSMAFromFile(file)).success).toBe(true)
  })
})
