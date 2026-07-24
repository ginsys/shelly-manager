import { describe, expect, it } from 'vitest'
import { ungzip } from 'pako'
import { generateSMAFile } from '../sma-generator'
import { parseStrictJSON } from '../strict-json'
import type { SMADevice } from '../sma-parser'
import { validDevice } from './sma-fixture'

describe('strict SMA 2026.1 generator', () => {
  it('always produces a gzip-compressed closed archive', async () => {
    const result = await generateSMAFile({ devices: [validDevice] })
    expect(result.success).toBe(true)
    const compressed = new Uint8Array(await result.blob!.arrayBuffer())
    expect([...compressed.slice(0, 2)]).toEqual([0x1f, 0x8b])
    const archive = parseStrictJSON(new TextDecoder().decode(ungzip(compressed))) as Record<string, unknown>
    expect(archive.format_version).toBe('2026.1')
    expect(archive).not.toHaveProperty('sma_version')
    expect((archive.metadata as any).created_by).toBe('shelly-manager-ui')
    expect((archive.metadata as any).integrity.file_count).toBe(1)
    expect(result.metadata.checksum).toMatch(/^sha256:[0-9a-f]{64}$/)
    expect(result.metadata.checksum).toBe((archive.metadata as any).integrity.checksum)
    expect(result.metadata.recordCount).toBe(1)
  })

  it('rejects empty archives and invalid compression levels', async () => {
    expect((await generateSMAFile({})).success).toBe(false)
    expect((await generateSMAFile({ devices: [validDevice] }, { compressionLevel: 0 })).success).toBe(false)
  })

  it('normalizes missing required maps', async () => {
    const device = { ...validDevice, settings: undefined } as unknown as SMADevice
    const result = await generateSMAFile({ devices: [device] })
    expect(result.success).toBe(true)
    const archive = parseStrictJSON(
      new TextDecoder().decode(ungzip(new Uint8Array(await result.blob!.arrayBuffer()))),
    ) as any
    expect(archive.devices[0].settings).toEqual(Object.create(null))
  })

  it('rejects cycles in open maps', async () => {
    const settings: Record<string, unknown> = {}
    settings.self = settings
    const result = await generateSMAFile({ devices: [{ ...validDevice, settings }] })
    expect(result.success).toBe(false)
    expect(result.error).toContain('cycle')
  })

  it('accepts container depth 64 and rejects depth 65', async () => {
    let depth64: unknown = 'leaf'
    for (let index = 0; index < 60; index++) depth64 = [depth64]
    const accepted = await generateSMAFile({
      devices: [{ ...validDevice, settings: { deep: depth64 } }],
    })
    expect(accepted.success, accepted.error).toBe(true)

    const depth65 = [depth64]
    const rejected = await generateSMAFile({
      devices: [{ ...validDevice, settings: { deep: depth65 } }],
    })
    expect(rejected.success).toBe(false)
    expect(rejected.error).toContain('depth')
  })
})
