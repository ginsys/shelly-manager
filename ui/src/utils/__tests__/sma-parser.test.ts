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

  describe('template scope', () => {
    const template = {
      id: 1, generation: 1, name: 't', description: '', device_type: 'SHPLG-S', config: {}, variables: {},
      is_default: false, created_at: '2026-01-02T03:04:05Z', updated_at: '2026-01-02T03:04:05Z',
    }

    it('keeps a scope through generate and parse, and leaves an absent one out', async () => {
      const generated = await generateSMAFile({
        templates: [{ ...template, scope: 'group' }, { ...template, id: 2, name: 'old' }],
      })
      const result = await parseSMAFile(await generated.blob!.arrayBuffer())
      expect(result.success, result.error).toBe(true)
      expect(result.archive?.templates[0].scope).toBe('group')
      expect(result.archive?.templates[1]).not.toHaveProperty('scope')
    })

    it('refuses a scope a template cannot hold', async () => {
      const generated = await generateSMAFile({ templates: [{ ...template, scope: 'team' as never }] })
      expect(generated.success).toBe(false)
      expect(generated.error).toContain('scope')
    })

    it('refuses a device_type scope with no device type, as the server does', async () => {
      const generated = await generateSMAFile({ templates: [{ ...template, scope: 'device_type', device_type: '' }] })
      expect(generated.success).toBe(false)
      expect(generated.error).toContain('device_type')
      const group = await generateSMAFile({ templates: [{ ...template, scope: 'group', device_type: '' }] })
      expect(group.success, group.error).toBe(true)
    })
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
