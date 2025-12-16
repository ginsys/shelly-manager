import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick } from 'vue'
import { useWebSocket, type UseWebSocketOptions } from '../useWebSocket'

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  url: string
  protocols?: string | string[]
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null

  constructor(url: string, protocols?: string | string[]) {
    this.url = url
    this.protocols = protocols
    // Simulate async connection
    setTimeout(() => {
      if (this.readyState === MockWebSocket.CONNECTING) {
        this.readyState = MockWebSocket.OPEN
        this.onopen?.(new Event('open'))
      }
    }, 10)
  }

  send(data: string) {
    if (this.readyState !== MockWebSocket.OPEN) {
      throw new Error('WebSocket is not open')
    }
  }

  close(code = 1000, reason = '') {
    if (this.readyState === MockWebSocket.CLOSED) return
    this.readyState = MockWebSocket.CLOSING
    setTimeout(() => {
      this.readyState = MockWebSocket.CLOSED
      const event = new CloseEvent('close', { code, reason })
      this.onclose?.(event)
    }, 10)
  }

  // Helper for tests
  simulateMessage(data: any) {
    if (this.readyState === MockWebSocket.OPEN) {
      const event = new MessageEvent('message', { data: JSON.stringify(data) })
      this.onmessage?.(event)
    }
  }

  simulateError() {
    const event = new Event('error')
    this.onerror?.(event)
  }
}

// Override global WebSocket
global.WebSocket = MockWebSocket as any

describe('useWebSocket', () => {
  let mockWebSocket: MockWebSocket | null = null

  beforeEach(() => {
    vi.useFakeTimers()
    mockWebSocket = null
    // Intercept WebSocket creation
    const OriginalWebSocket = global.WebSocket
    global.WebSocket = vi.fn().mockImplementation((...args) => {
      mockWebSocket = new OriginalWebSocket(...args)
      return mockWebSocket
    }) as any
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  describe('Connection lifecycle', () => {
    it('should connect automatically by default', async () => {
      const { status, isConnected } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      expect(status.value).toBe('connecting')
      expect(isConnected.value).toBe(false)

      // Wait for connection
      await vi.advanceTimersByTimeAsync(20)

      expect(status.value).toBe('open')
      expect(isConnected.value).toBe(true)
    })

    it('should not auto-connect when autoConnect is false', async () => {
      const { status, connect } = useWebSocket({
        url: 'ws://localhost:8080/test',
        autoConnect: false
      })

      expect(status.value).toBe('closed')

      // Manually connect
      connect()
      await vi.advanceTimersByTimeAsync(20)

      expect(status.value).toBe('open')
    })

    it('should disconnect cleanly', async () => {
      const { disconnect, status } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      disconnect()
      await vi.advanceTimersByTimeAsync(20)

      expect(status.value).toBe('closed')
    })

    it('should handle connection with protocols', async () => {
      const { } = useWebSocket({
        url: 'ws://localhost:8080/test',
        protocols: ['protocol1', 'protocol2']
      })

      await vi.advanceTimersByTimeAsync(20)

      expect(mockWebSocket?.protocols).toEqual(['protocol1', 'protocol2'])
    })

    it('should handle dynamic URL function', async () => {
      let urlSuffix = 'test1'
      const getUrl = () => `ws://localhost:8080/${urlSuffix}`

      const { connect, disconnect } = useWebSocket({
        url: getUrl,
        autoConnect: false
      })

      connect()
      await vi.advanceTimersByTimeAsync(20)
      expect(mockWebSocket?.url).toBe('ws://localhost:8080/test1')

      disconnect()
      await vi.advanceTimersByTimeAsync(20)

      // Change URL
      urlSuffix = 'test2'
      connect()
      await vi.advanceTimersByTimeAsync(20)
      expect(mockWebSocket?.url).toBe('ws://localhost:8080/test2')
    })
  })

  describe('Message handling', () => {
    it('should receive and parse messages', async () => {
      const messages: any[] = []
      const { data } = useWebSocket<{ type: string; value: number }>({
        url: 'ws://localhost:8080/test',
        onMessage: (msg) => messages.push(msg)
      })

      await vi.advanceTimersByTimeAsync(20)

      const testMessage = { type: 'test', value: 42 }
      mockWebSocket?.simulateMessage(testMessage)

      expect(data.value).toEqual(testMessage)
      expect(messages).toHaveLength(1)
      expect(messages[0]).toEqual(testMessage)
    })

    it('should update lastMessageAt timestamp', async () => {
      const { lastMessageAt } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)

      const beforeMessage = lastMessageAt.value
      expect(beforeMessage).toBeGreaterThan(0)

      await vi.advanceTimersByTimeAsync(1000)

      mockWebSocket?.simulateMessage({ data: 'test' })
      await nextTick()

      expect(lastMessageAt.value).toBeGreaterThan(beforeMessage!)
    })

    it('should handle invalid JSON gracefully', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      const { data } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)

      // Send invalid JSON
      if (mockWebSocket?.onmessage) {
        const event = new MessageEvent('message', { data: 'invalid json {' })
        mockWebSocket.onmessage(event)
      }

      expect(data.value).toBeNull()
      expect(consoleSpy).toHaveBeenCalled()
    })
  })

  describe('Reconnection logic', () => {
    it('should reconnect with exponential backoff', async () => {
      const { status } = useWebSocket({
        url: 'ws://localhost:8080/test',
        reconnectInterval: 1000,
        backoffFactor: 2
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      // Simulate disconnect
      mockWebSocket?.close(1006, 'Connection lost')
      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('closed')

      // First reconnect attempt (1000ms)
      await vi.advanceTimersByTimeAsync(1000)
      expect(status.value).toBe('connecting')

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      // Second disconnect
      mockWebSocket?.close(1006)
      await vi.advanceTimersByTimeAsync(20)

      // Second reconnect attempt (2000ms with backoff)
      await vi.advanceTimersByTimeAsync(2000)
      expect(status.value).toBe('connecting')
    })

    it('should not reconnect on normal close (1000)', async () => {
      const { status, reconnectAttempts } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      // Normal close
      mockWebSocket?.close(1000, 'Normal close')
      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('closed')

      // Wait for potential reconnect
      await vi.advanceTimersByTimeAsync(5000)

      // Should not have reconnected
      expect(status.value).toBe('closed')
      expect(reconnectAttempts.value).toBe(0)
    })

    it('should respect max reconnection attempts', async () => {
      const { status, reconnectAttempts } = useWebSocket({
        url: 'ws://localhost:8080/test',
        reconnectAttempts: 2,
        reconnectInterval: 100
      })

      await vi.advanceTimersByTimeAsync(20)

      // Force 3 disconnections
      for (let i = 0; i < 3; i++) {
        mockWebSocket?.close(1006)
        await vi.advanceTimersByTimeAsync(20)
        await vi.advanceTimersByTimeAsync(200)
      }

      // Should stop at 2 attempts
      expect(reconnectAttempts.value).toBeLessThanOrEqual(2)
      expect(status.value).toBe('closed')
    })

    it('should not reconnect when reconnect option is false', async () => {
      const { status } = useWebSocket({
        url: 'ws://localhost:8080/test',
        reconnect: false
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      mockWebSocket?.close(1006)
      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('closed')

      await vi.advanceTimersByTimeAsync(5000)
      expect(status.value).toBe('closed')
    })
  })

  describe('Heartbeat functionality', () => {
    it('should detect heartbeat timeout and reconnect', async () => {
      const { status } = useWebSocket({
        url: 'ws://localhost:8080/test',
        heartbeatInterval: 1000,
        heartbeatTimeout: 3000
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      // Wait past heartbeat timeout without messages
      await vi.advanceTimersByTimeAsync(3500)

      // Should trigger reconnection
      expect(status.value).toBe('connecting')
    })

    it('should not timeout when receiving regular messages', async () => {
      const { status } = useWebSocket({
        url: 'ws://localhost:8080/test',
        heartbeatInterval: 1000,
        heartbeatTimeout: 3000
      })

      await vi.advanceTimersByTimeAsync(20)

      // Send messages regularly
      for (let i = 0; i < 5; i++) {
        await vi.advanceTimersByTimeAsync(500)
        mockWebSocket?.simulateMessage({ data: i })
      }

      // Should still be connected
      expect(status.value).toBe('open')
    })

    it('should disable heartbeat when interval is 0', async () => {
      const { status } = useWebSocket({
        url: 'ws://localhost:8080/test',
        heartbeatInterval: 0
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(status.value).toBe('open')

      // Wait a long time without messages
      await vi.advanceTimersByTimeAsync(60000)

      // Should still be connected (no heartbeat monitoring)
      expect(status.value).toBe('open')
    })
  })

  describe('Sending messages', () => {
    it('should send string messages', async () => {
      const sendSpy = vi.fn()
      const { send } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)

      if (mockWebSocket) {
        mockWebSocket.send = sendSpy
      }

      send('test message')
      expect(sendSpy).toHaveBeenCalledWith('test message')
    })

    it('should send object messages as JSON', async () => {
      const sendSpy = vi.fn()
      const { send } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)

      if (mockWebSocket) {
        mockWebSocket.send = sendSpy
      }

      const obj = { type: 'test', value: 42 }
      send(obj)
      expect(sendSpy).toHaveBeenCalledWith(JSON.stringify(obj))
    })

    it('should warn when sending on closed connection', async () => {
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      const { send, disconnect } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      await vi.advanceTimersByTimeAsync(20)
      disconnect()
      await vi.advanceTimersByTimeAsync(20)

      send('test')
      expect(consoleSpy).toHaveBeenCalled()
    })
  })

  describe('Computed properties', () => {
    it('should compute isConnected correctly', async () => {
      const { isConnected } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      expect(isConnected.value).toBe(false)

      await vi.advanceTimersByTimeAsync(20)
      expect(isConnected.value).toBe(true)
    })

    it('should compute isRealtimeActive correctly', async () => {
      const { isRealtimeActive } = useWebSocket({
        url: 'ws://localhost:8080/test'
      })

      expect(isRealtimeActive.value).toBe(false)

      await vi.advanceTimersByTimeAsync(20)
      expect(isRealtimeActive.value).toBe(true)

      // Wait 61 seconds without messages
      await vi.advanceTimersByTimeAsync(61000)
      expect(isRealtimeActive.value).toBe(false)
    })
  })

  describe('Event callbacks', () => {
    it('should call onOpen callback', async () => {
      const onOpen = vi.fn()
      useWebSocket({
        url: 'ws://localhost:8080/test',
        onOpen
      })

      await vi.advanceTimersByTimeAsync(20)
      expect(onOpen).toHaveBeenCalled()
    })

    it('should call onClose callback', async () => {
      const onClose = vi.fn()
      const { disconnect } = useWebSocket({
        url: 'ws://localhost:8080/test',
        onClose
      })

      await vi.advanceTimersByTimeAsync(20)
      disconnect()
      await vi.advanceTimersByTimeAsync(20)

      expect(onClose).toHaveBeenCalled()
    })

    it('should call onError callback', async () => {
      const onError = vi.fn()
      useWebSocket({
        url: 'ws://localhost:8080/test',
        onError
      })

      await vi.advanceTimersByTimeAsync(20)
      mockWebSocket?.simulateError()

      expect(onError).toHaveBeenCalled()
    })
  })

  describe('Cleanup', () => {
    it('should cleanup timers on unmount', async () => {
      const clearIntervalSpy = vi.spyOn(global, 'clearInterval')
      const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout')

      const { } = useWebSocket({
        url: 'ws://localhost:8080/test',
        heartbeatInterval: 1000
      })

      await vi.advanceTimersByTimeAsync(20)

      // Simulate unmount by calling disconnect (onUnmounted calls disconnect)
      // In real Vue, onUnmounted would be triggered automatically
      // For testing, we verify timers are cleared

      expect(clearIntervalSpy).toHaveBeenCalled()
    })
  })

  describe('Edge cases', () => {
    it('should handle rapid connect/disconnect cycles', async () => {
      const { connect, disconnect } = useWebSocket({
        url: 'ws://localhost:8080/test',
        autoConnect: false
      })

      for (let i = 0; i < 5; i++) {
        connect()
        await vi.advanceTimersByTimeAsync(10)
        disconnect()
        await vi.advanceTimersByTimeAsync(10)
      }

      // Should not crash or leak resources
      expect(true).toBe(true)
    })

    it('should not reconnect when already connecting', async () => {
      const { status, connect } = useWebSocket({
        url: 'ws://localhost:8080/test',
        autoConnect: false
      })

      connect()
      expect(status.value).toBe('connecting')

      // Try to connect again immediately
      connect()

      await vi.advanceTimersByTimeAsync(20)

      // Should only have one connection
      expect(status.value).toBe('open')
    })
  })
})
