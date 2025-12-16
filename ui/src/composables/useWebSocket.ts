import { ref, readonly, computed, onMounted, onScopeDispose, type Ref, type ComputedRef } from 'vue'

export type WebSocketStatus = 'connecting' | 'open' | 'closing' | 'closed'

export interface UseWebSocketOptions<T> {
  url: string | (() => string)
  protocols?: string[]
  autoConnect?: boolean
  reconnect?: boolean
  reconnectAttempts?: number
  reconnectInterval?: number
  heartbeatInterval?: number
  heartbeatTimeout?: number
  onMessage?: (data: T) => void
  onOpen?: (event: Event) => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
}

export interface UseWebSocketReturn<T> {
  status: Ref<WebSocketStatus>
  data: Ref<T | null>
  error: Ref<Event | null>
  isConnected: ComputedRef<boolean>
  lastMessageAt: Ref<number | null>
  reconnectAttempts: Ref<number>
  connect: () => void
  disconnect: () => void
  send: (data: string | object) => void
}

/**
 * Reusable WebSocket composable with automatic reconnection, heartbeat detection,
 * and connection status tracking.
 *
 * Features:
 * - Automatic reconnection with exponential backoff
 * - Heartbeat/ping-pong support
 * - Connection status tracking
 * - TypeScript generic support for typed messages
 * - Event-based message handling
 *
 * @example
 * ```ts
 * const { data, status, connect, disconnect } = useWebSocket<MetricsMessage>({
 *   url: '/api/metrics/ws',
 *   reconnect: true,
 *   heartbeatInterval: 30000,
 *   onMessage: (msg) => console.log('Received:', msg)
 * })
 * ```
 */
export function useWebSocket<T = unknown>(
  options: UseWebSocketOptions<T>
): UseWebSocketReturn<T> {
  const {
    url,
    protocols,
    autoConnect = true,
    reconnect = true,
    reconnectAttempts: maxReconnectAttempts = Infinity,
    reconnectInterval = 1000,
    heartbeatInterval = 15000,
    heartbeatTimeout = 45000,
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
  const reconnectAttempts = ref(0)

  // Internals
  let socket: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null

  // Computed
  const isConnected = computed(() => status.value === 'open')

  /**
   * Get WebSocket URL (resolve function if provided)
   */
  function getUrl(): string {
    return typeof url === 'function' ? url() : url
  }

  /**
   * Connect to WebSocket server
   */
  function connect() {
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
        reconnectAttempts.value = 0
        lastMessageAt.value = Date.now()
        startHeartbeat()
        onOpen?.(event)
      }

      socket.onmessage = (event) => {
        lastMessageAt.value = Date.now()
        try {
          const parsed = JSON.parse(event.data) as T
          data.value = parsed
          onMessage?.(parsed)
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e)
        }
      }

      socket.onclose = (event) => {
        status.value = 'closed'
        socket = null
        stopHeartbeat()
        onClose?.(event)

        // Auto-reconnect logic
        if (reconnect && reconnectAttempts.value < maxReconnectAttempts) {
          // Skip reconnection for normal close codes
          if ([1000, 1001, 1005].includes(event.code)) {
            return
          }
          scheduleReconnect()
        }
      }

      socket.onerror = (event) => {
        error.value = event
        onError?.(event)
      }
    } catch (e) {
      console.error('Failed to create WebSocket:', e)
      if (reconnect) {
        scheduleReconnect()
      }
    }
  }

  /**
   * Disconnect from WebSocket server
   */
  function disconnect() {
    if (socket) {
      status.value = 'closing'
      socket.close(1000, 'Client disconnect')
      socket = null
    }
    status.value = 'closed'
    stopHeartbeat()
    clearReconnectTimer()
  }

  /**
   * Send data through WebSocket
   */
  function send(payload: string | object) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket is not connected')
      return
    }

    const message = typeof payload === 'string' ? payload : JSON.stringify(payload)
    socket.send(message)
  }

  /**
   * Schedule reconnection with exponential backoff and jitter
   */
  function scheduleReconnect() {
    clearReconnectTimer()

    const baseDelay = reconnectInterval
    const maxDelay = 30000 // 30 seconds
    const backoffFactor = 2
    const jitter = 0.5

    let delay = Math.min(baseDelay * Math.pow(backoffFactor, reconnectAttempts.value), maxDelay)
    delay = delay * (1 + (Math.random() - 0.5) * jitter) // Add jitter

    reconnectTimer = setTimeout(() => {
      reconnectAttempts.value++
      connect()
    }, delay)
  }

  /**
   * Clear reconnection timer
   */
  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  /**
   * Start heartbeat monitoring
   */
  function startHeartbeat() {
    stopHeartbeat()
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
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = null
    }
  }

  // Auto-connect if enabled
  if (autoConnect) {
    onMounted(() => {
      connect()
    })
  }

  // Cleanup on scope dispose (works in both components and effect scopes)
  onScopeDispose(() => {
    disconnect()
  })

  return {
    status: readonly(status) as Ref<WebSocketStatus>,
    data: readonly(data) as Ref<T | null>,
    error: readonly(error) as Ref<Event | null>,
    isConnected,
    lastMessageAt: readonly(lastMessageAt) as Ref<number | null>,
    reconnectAttempts: readonly(reconnectAttempts) as Ref<number>,
    connect,
    disconnect,
    send
  }
}
