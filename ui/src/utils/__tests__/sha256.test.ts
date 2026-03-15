import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { sha256Hex, isWebCryptoAvailable } from '../sha256'

// NIST test vector: SHA-256("abc")
const EXPECTED_ABC = 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad'

describe('sha256Hex', () => {
  describe('Web Crypto path', () => {
    it('produces correct SHA-256 for known input', async () => {
      // In the test environment (Node/jsdom), crypto.subtle is available
      expect(isWebCryptoAvailable()).toBe(true)
      const result = await sha256Hex('abc')
      expect(result).toBe(EXPECTED_ABC)
    })

    it('produces correct SHA-256 for empty string', async () => {
      const result = await sha256Hex('')
      expect(result).toBe('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855')
    })
  })

  describe('pure-JS fallback path', () => {
    let originalSubtle: SubtleCrypto | undefined

    beforeEach(() => {
      originalSubtle = globalThis.crypto?.subtle
      // Stub crypto.subtle to undefined to force the fallback path
      Object.defineProperty(globalThis.crypto, 'subtle', {
        value: undefined,
        writable: true,
        configurable: true,
      })
    })

    afterEach(() => {
      Object.defineProperty(globalThis.crypto, 'subtle', {
        value: originalSubtle,
        writable: true,
        configurable: true,
      })
    })

    it('isWebCryptoAvailable returns false when subtle is undefined', () => {
      expect(isWebCryptoAvailable()).toBe(false)
    })

    it('produces correct SHA-256 for known input (NIST vector)', async () => {
      const result = await sha256Hex('abc')
      expect(result).toBe(EXPECTED_ABC)
    })

    it('produces correct SHA-256 for empty string', async () => {
      const result = await sha256Hex('')
      expect(result).toBe('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855')
    })

    it('produces correct SHA-256 for longer input', async () => {
      // NIST vector: "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq"
      const result = await sha256Hex('abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq')
      expect(result).toBe('248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1')
    })
  })

  describe('consistency', () => {
    it('Web Crypto and fallback produce identical output', async () => {
      const testInputs = ['abc', '', 'hello world', 'The quick brown fox jumps over the lazy dog']

      for (const input of testInputs) {
        // Get Web Crypto result
        const webCryptoResult = await sha256Hex(input)

        // Stub subtle to force fallback
        const originalSubtle = globalThis.crypto.subtle
        Object.defineProperty(globalThis.crypto, 'subtle', {
          value: undefined,
          writable: true,
          configurable: true,
        })

        const fallbackResult = await sha256Hex(input)

        // Restore
        Object.defineProperty(globalThis.crypto, 'subtle', {
          value: originalSubtle,
          writable: true,
          configurable: true,
        })

        expect(fallbackResult).toBe(webCryptoResult)
      }
    })
  })
})
