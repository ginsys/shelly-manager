import { ref, computed, onMounted, onUnmounted, readonly, type Ref, type ComputedRef } from 'vue'

export type WebSocketStatus = 'connecting' | 'open' | 'closing' | 'closed'

export interface UseWebSocketOptions<T> {
  url: string | (() => string)
  protocols?: string[]
  autoConnect?: boolean
  reconnect?: boolean
  reconnectAttempts?: number
  reconnectInterval?: number // base ms
  heartbeatInterval?: number // ms
  heartbeatMessage?: string | (() => string)
  onMessage?: (data: T) => void
  onOpen?: (event: Event) => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
}

export interface UseWebSocketReturn<T> {
  status: Ref<WebSocketStatus>
  data: Ref<T | null>
  error: Ref<Event | null>
  connect: () => void
  disconnect: () => void
  send: (data: string | object) => void
  isConnected: ComputedRef<boolean>
}

export function useWebSocket<T = unknown>(options: UseWebSocketOptions<T>): UseWebSocketReturn<T> {
  const status = ref<WebSocketStatus>('closed')
  const data = ref<T | null>(null)
  const error = ref<Event | null>(null)

  const reconnectEnabled = options.reconnect !== false
  const maxAttempts = options.reconnectAttempts ?? 5
  const baseInterval = options.reconnectInterval ?? 1000
  const hbInterval = options.heartbeatInterval ?? 30000

  let socket: WebSocket | null = null
  let attempts = 0
  let reconnectTimer: any = 0
  let heartbeatTimer: any = 0

  function resolveUrl(): string {
    return typeof options.url === 'function' ? options.url() : options.url
  }

  function scheduleReconnect(ev?: CloseEvent) {
    if (!reconnectEnabled) return
    if (maxAttempts >= 0 && attempts >= maxAttempts) return
    // do not reconnect on normal closure
    if (ev && [1000, 1001, 1005].includes(ev.code)) return
    const delay = Math.min(baseInterval * Math.pow(2, attempts), 30000) * (1 + (Math.random() - 0.5) * 0.5)
    clearTimeout(reconnectTimer)
    reconnectTimer = setTimeout(() => {
      attempts++
      connect()
    }, delay)
  }

  function clearReconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = 0
    }
  }

  function startHeartbeat() {
    stopHeartbeat()
    if (!hbInterval) return
    heartbeatTimer = setInterval(() => {
      if (socket && socket.readyState === WebSocket.OPEN) {
        const msg = typeof options.heartbeatMessage === 'function' ? options.heartbeatMessage?.() : options.heartbeatMessage || 'ping'
        try { socket.send(msg) } catch {}
      }
    }, hbInterval)
  }

  function stopHeartbeat() {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = 0
    }
  }

  function connect() {
    try {
      const url = resolveUrl()
      status.value = 'connecting'
      socket = new WebSocket(url, options.protocols)

      socket.onopen = (ev) => {
        status.value = 'open'
        attempts = 0
        clearReconnect()
        startHeartbeat()
        options.onOpen?.(ev)
      }

      socket.onmessage = (ev) => {
        try {
          const parsed = ((): T => {
            // Try parse JSON, fallback to raw string
            try { return JSON.parse(ev.data) } catch { return ev.data as unknown as T }
          })()
          data.value = parsed
          options.onMessage?.(parsed)
        } catch {}
      }

      socket.onclose = (ev) => {
        status.value = 'closed'
        stopHeartbeat()
        options.onClose?.(ev)
        scheduleReconnect(ev)
      }

      socket.onerror = (ev) => {
        error.value = ev
        options.onError?.(ev)
      }
    } catch (e) {
      // schedule reconnect if construction fails
      scheduleReconnect()
    }
  }

  function disconnect() {
    try {
      status.value = 'closing'
      clearReconnect()
      stopHeartbeat()
      socket?.close(1000, 'Client disconnect')
    } finally {
      status.value = 'closed'
      socket = null
    }
  }

  function send(payload: string | object) {
    const msg = typeof payload === 'string' ? payload : JSON.stringify(payload)
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(msg)
    }
  }

  if (options.autoConnect !== false) {
    onMounted(connect)
  }
  onUnmounted(disconnect)

  return {
    status: readonly(status),
    data: readonly(data),
    error: readonly(error),
    connect,
    disconnect,
    send,
    isConnected: computed(() => status.value === 'open'),
  }
}

