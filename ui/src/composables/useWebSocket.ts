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
  // Monotonic connection generation. Every connect()/disconnect() bumps it; each
  // socket's event handlers capture the generation active when they were bound and
  // bail if it no longer matches. This stops a superseded socket (e.g. the stale
  // one left OPEN during a heartbeat-timeout recycle) from mutating state after we
  // have moved on — late onmessage/onclose from an old socket become no-ops.
  let generation = 0
  // Wall-clock time the current socket opened, used as the heartbeat baseline until
  // the first message arrives so a socket that opens but never delivers still times
  // out and gets recycled.
  let connectedAt = 0

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

    // Open a new generation and tear down any lingering (closing/closed) socket
    // plus a pending reconnect. Handlers below capture `myGen`; a later
    // connect()/disconnect() bumps `generation`, neutralising this socket.
    const myGen = ++generation
    clearReconnectTimer()
    stopHeartbeat()
    if (socket) {
      try { socket.close() } catch { /* already closing */ }
      socket = null
    }

    try {
      status.value = 'connecting'
      const wsUrl = getUrl()
      socket = new WebSocket(wsUrl, protocols)

      socket.onopen = (event) => {
        if (myGen !== generation) return
        status.value = 'open'
        reconnectAttempts.value = 0
        connectedAt = Date.now()
        // Deliberately do NOT stamp lastMessageAt here: opening a socket is not
        // receiving data. The "live" indicator downstream must reflect applied
        // payloads, never mere connection state (#247).
        startHeartbeat()
        onOpen?.(event)
      }

      socket.onmessage = (event) => {
        if (myGen !== generation) return
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
        if (myGen !== generation) return
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
        if (myGen !== generation) return
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
    // Bump the generation first so the closing socket's pending onclose/onmessage
    // (and any in-flight recycle) can't mutate state once we intend to tear down.
    generation++
    stopHeartbeat()
    clearReconnectTimer()
    if (socket) {
      status.value = 'closing'
      try { socket.close(1000, 'Client disconnect') } catch { /* already closing */ }
      socket = null
    }
    status.value = 'closed'
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
      // Baseline is the last message, or the connect time if none has arrived yet.
      const since = lastMessageAt.value ?? connectedAt
      if (since && Date.now() - since > heartbeatTimeout) {
        console.warn('WebSocket heartbeat timeout, recycling socket')
        // Force-close the stale (still-OPEN) socket before reconnecting. A bare
        // connect() would early-return because the old socket is still OPEN,
        // leaving it to linger forever (#247).
        disconnect()
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
