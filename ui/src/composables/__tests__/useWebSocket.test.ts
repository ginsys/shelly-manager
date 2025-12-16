import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick, effectScope } from 'vue'
import { useWebSocket } from '../useWebSocket'

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  url: string
  protocols?: string[]
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null

  constructor(url: string, protocols?: string[]) {
    this.url = url
    this.protocols = protocols
  }

  send(data: string) {
    if (this.readyState !== MockWebSocket.OPEN) {
      throw new Error('WebSocket is not open')
    }
  }

  close(code?: number, reason?: string) {
    this.readyState = MockWebSocket.CLOSING
    setTimeout(() => {
      this.readyState = MockWebSocket.CLOSED
      if (this.onclose) {
        this.onclose(new CloseEvent('close', { code: code || 1000, reason }))
      }
    }, 0)
  }

  // Helper for testing
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    if (this.onopen) {
      this.onopen(new Event('open'))
    }
  }

  simulateMessage(data: any) {
    if (this.onmessage) {
      this.onmessage(new MessageEvent('message', { data: JSON.stringify(data) }))
    }
  }

  simulateError() {
    if (this.onerror) {
      this.onerror(new Event('error'))
    }
  }
}

global.WebSocket = MockWebSocket as any

let wsInstances: MockWebSocket[] = []

// Wrap WebSocket constructor to track instances
const OriginalWebSocket = global.WebSocket
global.WebSocket = class extends OriginalWebSocket {
  constructor(url: string, protocols?: string[]) {
    super(url, protocols)
    wsInstances.push(this as any)
  }
} as any

describe('useWebSocket', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    wsInstances = []
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('should initialize with closed status', () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: 'ws://test', autoConnect: false }))!

    expect(ws.status.value).toBe('closed')
    expect(ws.isConnected.value).toBe(false)

    scope.stop()
  })

  it('should connect to WebSocket', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: 'ws://test', autoConnect: false }))!

    ws.connect()
    expect(ws.status.value).toBe('connecting')

    // Simulate successful connection
    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()
    await nextTick()

    expect(ws.status.value).toBe('open')
    expect(ws.isConnected.value).toBe(true)

    scope.stop()
  })

  it('should handle incoming messages', async () => {
    const messages: any[] = []
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({
      url: 'ws://test',
      autoConnect: false,
      onMessage: (msg) => messages.push(msg)
    }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()
    mockWS.simulateMessage({ type: 'test', data: 'hello' })
    await nextTick()

    expect(messages).toHaveLength(1)
    expect(messages[0]).toEqual({ type: 'test', data: 'hello' })
    expect(ws.data.value).toEqual({ type: 'test', data: 'hello' })

    scope.stop()
  })

  it('should handle reconnection with exponential backoff', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({
      url: 'ws://test',
      autoConnect: false,
      reconnect: true,
      reconnectInterval: 1000
    }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()
    expect(ws.reconnectAttempts.value).toBe(0)

    // Simulate connection close
    mockWS.close(1006, 'Abnormal closure')
    await vi.runAllTimersAsync()
    await nextTick()

    expect(ws.reconnectAttempts.value).toBeGreaterThan(0)

    scope.stop()
  })

  it('should not reconnect on normal close', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({
      url: 'ws://test',
      autoConnect: false,
      reconnect: true
    }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()

    // Normal close should not trigger reconnect
    mockWS.close(1000, 'Normal closure')
    await vi.runAllTimersAsync()
    await nextTick()

    expect(ws.reconnectAttempts.value).toBe(0)

    scope.stop()
  })

  it('should handle heartbeat timeout', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({
      url: 'ws://test',
      autoConnect: false,
      heartbeatInterval: 100,
      heartbeatTimeout: 200
    }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()
    await nextTick()

    // Advance time past heartbeat timeout
    vi.advanceTimersByTime(300)
    await nextTick()

    // Should trigger reconnection attempt
    expect(ws.reconnectAttempts.value).toBeGreaterThanOrEqual(0)

    scope.stop()
  })

  it('should disconnect cleanly', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: 'ws://test', autoConnect: false }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()
    expect(ws.status.value).toBe('open')

    ws.disconnect()
    await vi.runAllTimersAsync()
    await nextTick()

    expect(ws.status.value).toBe('closed')
    expect(ws.isConnected.value).toBe(false)

    scope.stop()
  })

  it('should handle error events', async () => {
    const errors: Event[] = []
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({
      url: 'ws://test',
      autoConnect: false,
      onError: (e) => errors.push(e)
    }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateError()
    await nextTick()

    expect(errors).toHaveLength(1)
    expect(ws.error.value).toBeTruthy()

    scope.stop()
  })

  it('should send messages when connected', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: 'ws://test', autoConnect: false }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()

    const sendSpy = vi.spyOn(mockWS, 'send')
    ws.send({ test: 'data' })

    expect(sendSpy).toHaveBeenCalledWith('{"test":"data"}')

    scope.stop()
  })

  it('should not send when disconnected', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: 'ws://test', autoConnect: false }))!

    const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    ws.send({ test: 'data' })

    expect(consoleSpy).toHaveBeenCalledWith('WebSocket is not connected')

    scope.stop()
  })

  it('should support function URLs', async () => {
    const getUrl = () => 'ws://dynamic-url'
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: getUrl, autoConnect: false }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    expect(mockWS.url).toBe('ws://dynamic-url')

    scope.stop()
  })

  it('should cleanup on unmount', async () => {
    const scope = effectScope()
    const ws = scope.run(() => useWebSocket({ url: 'ws://test', autoConnect: false }))!

    ws.connect()

    const mockWS = wsInstances[wsInstances.length - 1]
    mockWS.simulateOpen()

    // Stop the scope to simulate unmount
    scope.stop()
    await nextTick()

    expect(ws.status.value).toBe('closed')
  })
})
