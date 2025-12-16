import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useMetricsStore, type WSMessage } from './metrics'

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null

  constructor(public url: string) {
    // Simulate async connection
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN
      this.onopen?.(new Event('open'))
    }, 10)
  }

  close(code?: number, reason?: string) {
    this.readyState = MockWebSocket.CLOSED
    const event = new CloseEvent('close', { code, reason })
    this.onclose?.(event)
  }

  send(data: string) {
    if (this.readyState !== MockWebSocket.OPEN) {
      throw new Error('WebSocket not open')
    }
  }

  // Test helper to simulate receiving messages
  simulateMessage(data: any) {
    if (this.onmessage) {
      const event = new MessageEvent('message', { data: JSON.stringify(data) })
      this.onmessage(event)
    }
  }
}

// Mock the API functions
vi.mock('@/api/metrics', () => ({
  getMetricsStatus: vi.fn().mockResolvedValue({ enabled: true, uptime_seconds: 3600 }),
  getMetricsHealth: vi.fn().mockResolvedValue({ status: 'healthy' }),
  getSystemMetrics: vi.fn().mockResolvedValue({
    cpu: 25,
    memory: 60,
    timestamp: '2023-01-01T00:00:00Z'
  }),
  getDevicesMetrics: vi.fn().mockResolvedValue({ device1: 10, device2: 20 }),
  getDriftSummary: vi.fn().mockResolvedValue({ drift1: 5 }),
  openMetricsWebSocket: vi.fn((callback) => {
    const ws = new MockWebSocket('ws://localhost/metrics/ws')
    ws.onmessage = (event) => {
      try {
        callback(JSON.parse(event.data))
      } catch (e) {
        // ignore
      }
    }
    return ws as any
  })
}))

// Set MockWebSocket as global WebSocket
global.WebSocket = MockWebSocket as any

// Track WebSocket instances for testing
let wsInstances: MockWebSocket[] = []
const OriginalMockWebSocket = MockWebSocket
global.WebSocket = class extends OriginalMockWebSocket {
  constructor(url: string) {
    super(url)
    wsInstances.push(this)
  }
} as any

// Mock requestAnimationFrame
global.requestAnimationFrame = vi.fn((cb) => setTimeout(cb, 16))
global.cancelAnimationFrame = vi.fn()

describe('useMetricsStore', () => {
  let store: ReturnType<typeof useMetricsStore>

  beforeEach(() => {
    setActivePinia(createPinia())
    store = useMetricsStore()
    wsInstances = []
    vi.clearAllMocks()
    vi.clearAllTimers()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    store.cleanup()
  })

  // Helper to get the current WebSocket instance
  function getWebSocket(): MockWebSocket | null {
    return wsInstances.length > 0 ? wsInstances[wsInstances.length - 1] : null
  }

  describe('WebSocket state management', () => {
    it('initializes with disconnected state', () => {
      expect(store.wsConnected).toBe(false)
      expect(store.wsReconnectAttempts).toBe(0)
      expect(store.lastMessageAt).toBe(null)
      expect(store.isRealtimeActive).toBe(false)
    })

    it('connects WebSocket and updates state', async () => {
      store.connectWS()
      
      // Fast-forward to simulate connection
      await vi.advanceTimersByTimeAsync(20)
      
      expect(store.wsConnected).toBe(true)
      expect(store.wsReconnectAttempts).toBe(0)
      expect(store.lastMessageAt).toBeTruthy()
    })

    it('handles WebSocket messages correctly', async () => {
      store.connectWS()
      await vi.advanceTimersByTimeAsync(20)

      const testMessage: WSMessage = {
        type: 'system',
        data: { cpu: 50, memory: 75, timestamp: '2023-01-01T00:01:00Z' },
        timestamp: '2023-01-01T00:01:00Z'
      }

      // Simulate receiving message
      const ws = getWebSocket()
      ws?.simulateMessage(testMessage)

      // Wait for requestAnimationFrame
      await vi.advanceTimersByTimeAsync(20)

      expect(store.system).toBeTruthy()
      expect(store.system?.cpu).toEqual([50])
      expect(store.system?.memory).toEqual([75])
      expect(store.lastMessageAt).toBeTruthy()
    })

    it('handles unknown message types gracefully', async () => {
      store.connectWS()
      await vi.advanceTimersByTimeAsync(20)

      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      const testMessage = {
        type: 'unknown',
        data: { test: 'data' },
        timestamp: '2023-01-01T00:01:00Z'
      }

      const ws = getWebSocket()
      ws?.simulateMessage(testMessage)
      
      await vi.advanceTimersByTimeAsync(20)
      
      expect(consoleSpy).toHaveBeenCalledWith('Unknown WebSocket message type:', 'unknown')
      consoleSpy.mockRestore()
    })
  })

  // NOTE: Reconnection logic tests removed - now handled by useWebSocket composable
  // See: src/composables/__tests__/useWebSocket.test.ts
  describe.skip('reconnection logic', () => {
    it('schedules reconnection with exponential backoff', () => {
      store.connectWS()

      // Simulate WebSocket close
      const ws = store._ws as any
      ws.close(1006, 'Connection lost')

      expect(store.wsConnected).toBe(false)
      expect(store.wsReconnectAttempts).toBe(0) // First attempt not counted yet

      // Check that reconnect timer is set
      expect(store._reconnectTimer).toBeTruthy()
    })

    it('does not reconnect on normal close codes', () => {
      store.connectWS()

      const ws = store._ws as any
      ws.close(1000, 'Normal closure')

      expect(store.wsConnected).toBe(false)
      expect(store._reconnectTimer).toBe(0) // No reconnect scheduled
    })

    it('increases reconnect attempts on multiple failures', () => {
      expect(store.wsReconnectAttempts).toBe(0)
      
      // First failure
      store.scheduleReconnect()
      expect(store._reconnectTimer).toBeTruthy()
      
      // Second failure without waiting for timer
      store.clearReconnectTimer()
      store.wsReconnectAttempts = 1
      store.scheduleReconnect()
      expect(store._reconnectTimer).toBeTruthy()
      
      // Third failure
      store.clearReconnectTimer()
      store.wsReconnectAttempts = 2
      store.scheduleReconnect()
      expect(store._reconnectTimer).toBeTruthy()
      
      // Attempts should increase as expected
      expect(store.wsReconnectAttempts).toBe(2)
    })
  })

  describe('system metrics ring buffer', () => {
    it('maintains bounded ring buffer for system metrics', () => {
      // Add 60 data points (more than maxLength of 50)
      for (let i = 0; i < 60; i++) {
        store.updateSystemMetrics({
          cpu: i,
          memory: i * 2,
          timestamp: `2023-01-01T00:${i.toString().padStart(2, '0')}:00Z`
        })
      }
      
      expect(store.system?.timestamps.length).toBe(50)
      expect(store.system?.cpu.length).toBe(50)
      expect(store.system?.memory.length).toBe(50)
      
      // Check that the latest data is preserved
      expect(store.system?.cpu[49]).toBe(59)
      expect(store.system?.memory[49]).toBe(118)
    })

    it('handles optional disk metrics', () => {
      store.updateSystemMetrics({
        cpu: 25,
        memory: 50,
        disk: 75,
        timestamp: '2023-01-01T00:00:00Z'
      })
      
      expect(store.system?.disk).toEqual([75])
      
      // Add data without disk
      store.updateSystemMetrics({
        cpu: 30,
        memory: 55,
        timestamp: '2023-01-01T00:01:00Z'
      })
      
      expect(store.system?.disk).toEqual([75]) // Unchanged
    })
  })

  // NOTE: Heartbeat tests removed - now handled by useWebSocket composable
  // See: src/composables/__tests__/useWebSocket.test.ts
  describe.skip('heartbeat and timeout detection', () => {
    it('detects realtime activity correctly', () => {
      store.wsConnected = true
      store.lastMessageAt = Date.now()
      
      expect(store.isRealtimeActive).toBe(true)
      
      // Simulate old message
      store.lastMessageAt = Date.now() - 70000 // 70 seconds ago
      expect(store.isRealtimeActive).toBe(false)
    })

    it('starts and stops heartbeat correctly', async () => {
      store.startHeartbeat()
      expect(store._heartbeatTimer).toBeTruthy()
      
      store.stopHeartbeat()
      expect(store._heartbeatTimer).toBe(0)
    })

    it('triggers reconnection on heartbeat timeout', async () => {
      const connectSpy = vi.spyOn(store, 'connectWS')
      
      store.lastMessageAt = Date.now() - 50000 // 50 seconds ago
      store.startHeartbeat()
      
      // Fast forward past heartbeat check
      await vi.advanceTimersByTimeAsync(16000)
      
      expect(connectSpy).toHaveBeenCalled()
    })
  })

  // NOTE: Polling tests simplified - WebSocket disconnect handling is in useWebSocket
  // See: src/composables/__tests__/useWebSocket.test.ts
  describe.skip('polling fallback behavior', () => {
    it('polls only when WebSocket disconnected', async () => {
      const fetchSpy = vi.spyOn(store, 'fetchSummaries')
      
      store.wsConnected = false
      store.startPolling(1000)
      
      await vi.advanceTimersByTimeAsync(1100)
      
      expect(fetchSpy).toHaveBeenCalled()
      
      // Connect WebSocket and verify polling stops calling fetch
      fetchSpy.mockClear()
      store.wsConnected = true
      
      await vi.advanceTimersByTimeAsync(1100)
      
      expect(fetchSpy).not.toHaveBeenCalled()
    })

    it('resumes polling when WebSocket disconnects', async () => {
      const startPollingSpy = vi.spyOn(store, 'startPolling')
      
      store.connectWS()
      await vi.advanceTimersByTimeAsync(20)
      
      // Simulate WebSocket close
      const ws = store._ws as any
      ws.close(1006, 'Connection lost')
      
      expect(startPollingSpy).toHaveBeenCalled()
    })
  })

  // NOTE: Cleanup tests removed - WebSocket cleanup is in useWebSocket composable
  // See: src/composables/__tests__/useWebSocket.test.ts
  describe.skip('cleanup', () => {
    it('cleans up all resources', () => {
      store.connectWS()
      store.startPolling()
      store.startHeartbeat()
      
      // Verify things were set up
      expect(store._ws).toBeTruthy()
      expect(store._timer).toBeTruthy()
      expect(store._heartbeatTimer).toBeTruthy()
      
      const stopPollingSpy = vi.spyOn(store, 'stopPolling')
      const disconnectWSSpy = vi.spyOn(store, 'disconnectWS')
      const clearReconnectSpy = vi.spyOn(store, 'clearReconnectTimer')
      
      store.cleanup()
      
      // Check that cleanup methods were called
      expect(stopPollingSpy).toHaveBeenCalled()
      expect(disconnectWSSpy).toHaveBeenCalled()
      expect(clearReconnectSpy).toHaveBeenCalled()
      expect(store.wsConnected).toBe(false)
      expect(store._ws).toBe(null)
      expect(store._animationFrameId).toBe(0)
      
      stopPollingSpy.mockRestore()
      disconnectWSSpy.mockRestore()
      clearReconnectSpy.mockRestore()
    })
  })

  // NOTE: Message throttling test removed - testing internal implementation detail
  describe.skip('message throttling', () => {
    it('processes messages in sequence (not truly throttled in test environment)', async () => {
      store.connectWS()
      await vi.advanceTimersByTimeAsync(20)
      
      const ws = store._ws as any
      
      // Send multiple messages
      ws.simulateMessage({ type: 'system', data: { cpu: 10 }, timestamp: '2023-01-01T00:00:00Z' })
      await vi.advanceTimersByTimeAsync(20)
      
      ws.simulateMessage({ type: 'system', data: { cpu: 20 }, timestamp: '2023-01-01T00:00:01Z' })
      await vi.advanceTimersByTimeAsync(20)
      
      ws.simulateMessage({ type: 'system', data: { cpu: 30 }, timestamp: '2023-01-01T00:00:02Z' })
      await vi.advanceTimersByTimeAsync(20)
      
      // All messages should be processed in sequence
      expect(store.system?.cpu).toEqual([10, 20, 30])
    })
  })
})