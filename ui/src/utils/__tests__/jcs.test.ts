import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { canonicalize } from '../jcs'
import { parseStrictJSON } from '../strict-json'
import { sha256Hex } from '../sha256'

describe('JCS and strict JSON', () => {
  it('sorts object names by UTF-16 code units and preserves arrays', () => {
    const value = Object.assign(Object.create(null), { z: 1, a: [3, 2, 1] })
    expect(canonicalize(value)).toBe('{"a":[3,2,1],"z":1}')
  })

  it('uses ECMAScript number serialization', () => {
    expect(canonicalize({ minusZero: -0, value: 1e-7 })).toBe('{"minusZero":0,"value":1e-7}')
  })

  it('rejects non-finite values, cycles, and lone surrogates', () => {
    expect(() => canonicalize({ value: Infinity })).toThrow()
    const cyclic: Record<string, unknown> = {}
    cyclic.self = cyclic
    expect(() => canonicalize(cyclic)).toThrow('cycle')
    expect(() => canonicalize({ value: '\ud800' })).toThrow('surrogate')
  })

  it('strictly rejects duplicates and enforces depth 64', () => {
    expect(() => parseStrictJSON('{"a":1,"a":2}')).toThrow('duplicate')
    let depth64 = '0'
    for (let index = 0; index < 64; index++) depth64 = `[${depth64}]`
    const parsedDepth64 = parseStrictJSON(depth64)
    expect(parsedDepth64).toBeTruthy()
    expect(canonicalize(parsedDepth64)).toBe(depth64)
    expect(() => parseStrictJSON(`[${depth64}]`)).toThrow('depth')
    expect(() => canonicalize([parsedDepth64])).toThrow('depth')
  })

  it('matches the shared canonical archive and digest sidecar', async () => {
    const canonical = readFileSync(
      resolve(process.cwd(), '../testdata/sma/archive-2026.1.canonical.json'),
      'utf8',
    )
    const digest = readFileSync(
      resolve(process.cwd(), '../testdata/sma/archive-2026.1.sha256'),
      'utf8',
    ).trim()
    expect(canonical.endsWith('\n')).toBe(false)
    const tree = parseStrictJSON(canonical) as {
      metadata: { integrity: { checksum: string } }
    }
    expect(canonicalize(tree)).toBe(canonical)
    expect(tree.metadata.integrity.checksum).toBe(digest)
    tree.metadata.integrity.checksum = ''
    expect(`sha256:${await sha256Hex(canonicalize(tree))}`).toBe(digest)
  })

  it('matches shared SMA numeric admission and IEEE-754 vectors', () => {
    const vectors = JSON.parse(readFileSync(
      resolve(process.cwd(), '../testdata/sma/numeric-vectors.json'),
      'utf8',
    )) as Array<{ text: string; binary64: string; admitted: boolean }>
    const view = new DataView(new ArrayBuffer(8))
    for (const vector of vectors) {
      view.setFloat64(0, Number(vector.text))
      const bits = `${view.getUint32(0).toString(16).padStart(8, '0')}${view.getUint32(4).toString(16).padStart(8, '0')}`
      expect(bits, vector.text).toBe(vector.binary64)
      if (vector.admitted) {
        expect(() => parseStrictJSON(vector.text), vector.text).not.toThrow()
      } else {
        expect(() => parseStrictJSON(vector.text), vector.text).toThrow()
      }
    }
  })
})
