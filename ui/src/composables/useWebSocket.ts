import { ref, computed, onUnmounted, readonly, type Ref, type ComputedRef } from 'vue'

/**
 * WebSocket connection status
 */
export type WebSocketStatus = 'connecting' | 'open' | 'closing' | 'closed'

/**
 * Options for configuring the WebSocket composable
 */
export interface UseWebSocketOptions<T> {
  /**
   * WebSocket URL (can be a function for dynamic URLs)
   */
  url: string | (() => string)

  /**
   * WebSocket protocols
   */
  protocols?: string | string[]

  /**
   * Auto-connect on mount (default: true)
   */
  autoConnect?: boolean

  /**
   * Enable automatic reconnection (default: true)
   */
  reconnect?: boolean

  /**
   * Maximum number of reconnection attempts (0 = infinite, default: 0)
   */
  reconnectAttempts?: number

  /**
   * Base reconnection delay in milliseconds (default: 1000)
   */
  reconnectInterval?: number

  /**
   * Maximum reconnection delay in milliseconds (default: 30000)
   */
  reconnectMaxDelay?: number

  /**
   * Backoff factor for exponential backoff (default: 2)
   */
  backoffFactor?: number

  /**
   * Jitter factor for randomizing delay (0-1, default: 0.5)
   */
  jitter?: number

  /**
   * Heartbeat check interval in milliseconds (0 = disabled, default: 15000)
   */
  heartbeatInterval?: number

  /**
   * Heartbeat timeout in milliseconds (default: 45000)
   */
  heartbeatTimeout?: number

  /**
   * Close codes that should not trigger reconnection (default: [1000, 1001, 1005])
   */
  noReconnectCodes?: number[]

  /**
   * Callback for message handling
   */
  onMessage?: (data: T) => void

  /**
   * Callback for connection open
   */
  onOpen?: (event: Event) => void

  /**
   * Callback for connection close
   */
  onClose?: (event: CloseEvent) => void

  /**
   * Callback for errors
   */
  onError?: (event: Event) => void
}

/**
 * Return type for the useWebSocket composable
 */
export interface UseWebSocketReturn<T> {
  /**
   * Current connection status
   */
  status: Ref<WebSocketStatus>

  /**
   * Latest received data
   */
  data: Ref<T | null>

  /**
   * Last error event
   */
  error: Ref<Event | null>

  /**
   * Timestamp of last received message
   */
  lastMessageAt: Ref<number | null>

  /**
   * Number of reconnection attempts
   */
  reconnectAttempts: Ref<number>

  /**
   * Connect to WebSocket
   */
  connect: () => void

  /**
   * Disconnect from WebSocket
   */
  disconnect: (code?: number, reason?: string) => void

  /**
   * Send data through WebSocket
   */
  send: (data: string | object) => void

  /**
   * Computed: Is connection currently open
   */
  isConnected: ComputedRef<boolean>

  /**
   * Computed: Is realtime active (connected and receiving messages)
   */
  isRealtimeActive: ComputedRef<boolean>
}

/**
 * Generic WebSocket composable with auto-reconnection and heartbeat support
 *
 * @example
 * ```ts
 * const { status, data, connect, disconnect, send, isConnected } = useWebSocket<MyMessage>({
 *   url: 'ws://localhost:8080/ws',
 *   onMessage: (msg) => console.log('Received:', msg),
 *   heartbeatInterval: 15000
 * })
 * ```
 */
export function useWebSocket<T = unknown>(
  options: UseWebSocketOptions<T>
): UseWebSocketReturn<T> {
  // Default options
  const {
    url,
    protocols,
    autoConnect = true,
    reconnect = true,
    reconnectAttempts: maxReconnectAttempts = 0,
    reconnectInterval = 1000,
    reconnectMaxDelay = 30000,
    backoffFactor = 2,
    jitter = 0.5,
    heartbeatInterval = 15000,
    heartbeatTimeout = 45000,
    noReconnectCodes = [1000, 1001, 1005],
    onMessage,
    onOpen,
    onClose,
    onError
  } = options

  // State
  const status = ref<WebSocketStatus>('closed')
  const data = ref<T | null>(null)
  const error = ref<Event | null>(null)
  const lastMessageAt = ref<number | null>(null)
  const reconnectAttemptsRef = ref(0)

  // Internal state
  let socket: WebSocket | null = null
  let reconnectTimer: number | undefined
  let heartbeatTimer: number | undefined

  // Computed properties
  const isConnected = computed(() => status.value === 'open')

  const isRealtimeActive = computed(() => {
    if (!isConnected.value || !lastMessageAt.value) return false
    return Date.now() - lastMessageAt.value < 60000 // 1 minute timeout
  })

  /**
   * Get WebSocket URL (handle function or string)
   */
  function getUrl(): string {
    return typeof url === 'function' ? url() : url
  }

  /**
   * Clear reconnection timer
   */
  function clearReconnectTimer() {
    if (reconnectTimer !== undefined) {
      clearTimeout(reconnectTimer)
      reconnectTimer = undefined
    }
  }

  /**
   * Clear heartbeat timer
   */
  function clearHeartbeatTimer() {
    if (heartbeatTimer !== undefined) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = undefined
    }
  }

  /**
   * Schedule reconnection with exponential backoff and jitter
   */
  function scheduleReconnect(closeEvent?: CloseEvent) {
    if (!reconnect) return

    clearReconnectTimer()

    // Don't reconnect on certain close codes
    if (closeEvent && noReconnectCodes.includes(closeEvent.code)) {
      return
    }

    // Check max attempts
    if (maxReconnectAttempts > 0 && reconnectAttemptsRef.value >= maxReconnectAttempts) {
      console.warn(`Max reconnection attempts (${maxReconnectAttempts}) reached`)
      return
    }

    // Calculate delay with exponential backoff
    let delay = Math.min(
      reconnectInterval * Math.pow(backoffFactor, reconnectAttemptsRef.value),
      reconnectMaxDelay
    )

    // Add jitter
    delay = delay * (1 + (Math.random() - 0.5) * jitter)

    console.log(`Reconnecting WebSocket in ${Math.round(delay)}ms (attempt ${reconnectAttemptsRef.value + 1})`)

    reconnectTimer = setTimeout(() => {
      reconnectAttemptsRef.value++
      connect()
    }, delay)
  }

  /**
   * Start heartbeat monitoring
   */
  function startHeartbeat() {
    if (heartbeatInterval <= 0) return

    clearHeartbeatTimer()

    heartbeatTimer = setInterval(() => {
      if (lastMessageAt.value && Date.now() - lastMessageAt.value > heartbeatTimeout) {
        console.warn('WebSocket heartbeat timeout, reconnecting...')
        connect()
      }
    }, heartbeatInterval)
  }

  /**
   * Stop heartbeat monitoring
   */
  function stopHeartbeat() {
    clearHeartbeatTimer()
  }

  /**
   * Connect to WebSocket
   */
  function connect() {
    // Close existing connection if any
    if (socket?.readyState === WebSocket.OPEN || socket?.readyState === WebSocket.CONNECTING) {
      return
    }

    disconnect()

    try {
      status.value = 'connecting'

      const wsUrl = getUrl()
      socket = new WebSocket(wsUrl, protocols)

      socket.onopen = (event) => {
        status.value = 'open'
        reconnectAttemptsRef.value = 0
        lastMessageAt.value = Date.now()
        startHeartbeat()

        console.log('WebSocket connected:', wsUrl)

        onOpen?.(event)
      }

      socket.onmessage = (event) => {
        lastMessageAt.value = Date.now()

        try {
          const parsedData = JSON.parse(event.data) as T
          data.value = parsedData
          onMessage?.(parsedData)
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e)
        }
      }

      socket.onclose = (event) => {
        status.value = 'closed'
        lastMessageAt.value = null
        stopHeartbeat()
        socket = null

        console.log('WebSocket closed:', event.code, event.reason)

        onClose?.(event)
        scheduleReconnect(event)
      }

      socket.onerror = (event) => {
        error.value = event
        console.error('WebSocket error:', event)
        onError?.(event)
      }

    } catch (err) {
      console.error('Failed to create WebSocket:', err)
      status.value = 'closed'
      scheduleReconnect()
    }
  }

  /**
   * Disconnect from WebSocket
   */
  function disconnect(code = 1000, reason = 'Client disconnect') {
    clearReconnectTimer()
    stopHeartbeat()

    if (socket) {
      status.value = 'closing'
      socket.close(code, reason)
      socket = null
    }

    status.value = 'closed'
  }

  /**
   * Send data through WebSocket
   */
  function send(payload: string | object) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket is not connected, cannot send message')
      return
    }

    const message = typeof payload === 'string' ? payload : JSON.stringify(payload)
    socket.send(message)
  }

  // Auto-connect on mount
  if (autoConnect) {
    connect()
  }

  // Cleanup on unmount (only register if in component context)
  try {
    onUnmounted(() => {
      disconnect()
    })
  } catch (e) {
    // Not in component context (e.g., during testing), skip onUnmounted registration
  }

  return {
    status: readonly(status) as Ref<WebSocketStatus>,
    data: readonly(data) as Ref<T | null>,
    error: readonly(error) as Ref<Event | null>,
    lastMessageAt: readonly(lastMessageAt) as Ref<number | null>,
    reconnectAttempts: readonly(reconnectAttemptsRef) as Ref<number>,
    connect,
    disconnect,
    send,
    isConnected,
    isRealtimeActive
  }
}
