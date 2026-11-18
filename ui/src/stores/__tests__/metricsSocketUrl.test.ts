import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { ref, computed } from 'vue'
import { setActivePinia, createPinia } from 'pinia'

// Capture the options the store gives the socket composable; the url function is the thing under test.
let captured: { url: () => string } | null = null
vi.mock('@/composables/useWebSocket', () => ({
  useWebSocket: (opts: { url: () => string }) => {
    captured = opts
    return {
      status: ref('closed'),
      data: ref(null),
      error: ref(null),
      isConnected: computed(() => false),
      lastMessageAt: ref(null),
      reconnectAttempts: ref(0),
      connect: vi.fn(),
      disconnect: vi.fn(),
      send: vi.fn(),
    }
  },
}))
vi.mock('@/api/metrics', () => ({
  getMetricsStatus: vi.fn(),
  getMetricsHealth: vi.fn(),
  getSystemMetrics: vi.fn(),
  getDevicesMetrics: vi.fn(),
  getDriftSummary: vi.fn(),
}))

import { useMetricsStore } from '../metrics'
import { setAdminKey, forgetAdminKey } from '@/api/adminKey'

describe('metrics socket URL', () => {
  beforeEach(() => {
    sessionStorage.clear()
    forgetAdminKey()
    delete (window as any).__ADMIN_KEY__
    setActivePinia(createPinia())
    captured = null
    useMetricsStore()
  })
  afterEach(() => {
    forgetAdminKey()
  })

  it('carries the entered session key as the token', () => {
    setAdminKey('a b&c')
    expect(captured!.url()).toMatch(/\/metrics\/ws\?token=a%20b%26c$/)
  })

  it('reads the key when the socket connects, not when the store was built', () => {
    expect(captured!.url()).not.toContain('token=')
    setAdminKey('later')
    expect(captured!.url()).toContain('token=later')
    forgetAdminKey()
    expect(captured!.url()).not.toContain('token=')
  })

  it('still honours the window global from /app-config.js when nothing was entered', () => {
    ;(window as any).__ADMIN_KEY__ = 'dev-key'
    expect(captured!.url()).toContain('token=dev-key')
  })
})
