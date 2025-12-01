import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getMetricsStatus, getMetricsHealth, getSystemMetrics, getDevicesMetrics, getDriftSummary } from '@/api/metrics'
import { useWebSocket } from '@/composables/useWebSocket'

// WebSocket message types from backend
export interface WSMessage {
  type: 'status' | 'health' | 'system' | 'devices' | 'drift' | 'heartbeat'
  data: any
  timestamp: string
}

export const useMetricsStore = defineStore('metrics', () => {
  // State
  const status = ref<any>(null)
  const health = ref<any>(null)

  // Time-series data with bounded ring buffers
  const system = ref<{
    timestamps: string[]
    cpu: number[]
    memory: number[]
    disk?: number[]
    maxLength: number
  } | null>(null)

  const devices = ref<any>(null)
  const drift = ref<any>(null)

  // Polling timer
  const _timer = ref<ReturnType<typeof setInterval> | null>(null)
  const _animationFrameId = ref<number>(0)

  // WebSocket URL generator
  function getWebSocketUrl(): string {
    const base = (window as any).__API_BASE__ || '/api/v1'
    const loc = window.location
    const proto = loc.protocol === 'https:' ? 'wss' : 'ws'
    const token = (window as any).__ADMIN_KEY__
    return `${proto}://${loc.host}${base.replace('/api/v1', '')}/metrics/ws${token ? `?token=${encodeURIComponent(token)}` : ''}`
  }

  // Initialize WebSocket with composable
  const ws = useWebSocket<WSMessage>({
    url: getWebSocketUrl,
    autoConnect: false,
    reconnect: true,
    reconnectInterval: 1000,
    heartbeatInterval: 15000,
    heartbeatTimeout: 45000,
    onMessage: (msg) => handleWSMessage(msg),
    onOpen: () => {
      console.log('WebSocket connected')
      stopPolling() // Stop REST polling when WS active
    },
    onClose: (event) => {
      console.log('WebSocket closed:', event.code, event.reason)
      startPolling() // Resume REST polling
    },
    onError: (error) => {
      console.error('WebSocket error:', error)
    }
  })

  // Getters
  const wsConnected = computed(() => ws.isConnected.value)
  const wsReconnectAttempts = computed(() => ws.reconnectAttempts.value)
  const lastMessageAt = computed(() => ws.lastMessageAt.value)

  // Connection status with timeout detection
  const isRealtimeActive = computed(() => {
    if (!ws.isConnected.value || !ws.lastMessageAt.value) return false
    return Date.now() - ws.lastMessageAt.value < 60000 // 1 minute timeout
  })

  // REST API fallback methods
  async function fetchStatus() {
    try {
      status.value = await getMetricsStatus()
    } catch (e) {
      console.warn('Failed to fetch metrics status:', e)
    }
  }

  async function fetchHealth() {
    try {
      health.value = await getMetricsHealth()
    } catch (e) {
      console.warn('Failed to fetch metrics health:', e)
    }
  }

  async function fetchSummaries() {
    try {
      const systemData = await getSystemMetrics()
      if (systemData && !ws.isConnected.value) {
        // Only update from REST if WebSocket is not active
        updateSystemMetrics(systemData)
      }
    } catch (e) {
      console.warn('Failed to fetch system metrics:', e)
    }

    try {
      devices.value = await getDevicesMetrics()
    } catch (e) {
      console.warn('Failed to fetch devices metrics:', e)
    }

    try {
      drift.value = await getDriftSummary()
    } catch (e) {
      console.warn('Failed to fetch drift summary:', e)
    }
  }

  // Polling for fallback when WebSocket unavailable
  function startPolling(intervalMs = 30000) {
    if (_timer.value) return
    _timer.value = setInterval(() => {
      if (!ws.isConnected.value) {
        fetchSummaries()
      }
    }, intervalMs)
    fetchSummaries()
  }

  function stopPolling() {
    if (_timer.value) {
      clearInterval(_timer.value)
      _timer.value = null
    }
  }

  // WebSocket connection management
  function connectWS() {
    ws.connect()
  }

  function disconnectWS() {
    ws.disconnect()
  }

  // Message handling with throttling
  function handleWSMessage(msg: WSMessage) {
    // Cancel pending animation frame
    if (_animationFrameId.value) {
      cancelAnimationFrame(_animationFrameId.value)
    }

    // Throttle updates using requestAnimationFrame
    _animationFrameId.value = requestAnimationFrame(() => {
      switch (msg.type) {
        case 'status':
          status.value = msg.data
          break
        case 'health':
          health.value = msg.data
          break
        case 'system':
          updateSystemMetrics(msg.data)
          break
        case 'devices':
          devices.value = msg.data
          break
        case 'drift':
          drift.value = msg.data
          break
        case 'heartbeat':
          // Just update lastMessageAt (handled by composable)
          break
        default:
          console.warn('Unknown WebSocket message type:', msg.type)
      }
    })
  }

  // Update system metrics with bounded ring buffer
  function updateSystemMetrics(data: any) {
    const maxLength = 50 // Configurable window size

    if (!system.value) {
      system.value = {
        timestamps: [],
        cpu: [],
        memory: [],
        disk: data.disk ? [] : undefined,
        maxLength
      }
    }

    const timestamp = data.timestamp || new Date().toISOString()

    // Add new data
    system.value.timestamps.push(timestamp)
    system.value.cpu.push(data.cpu || 0)
    system.value.memory.push(data.memory || 0)
    if (data.disk !== undefined && system.value.disk) {
      system.value.disk.push(data.disk)
    }

    // Trim to max length
    if (system.value.timestamps.length > maxLength) {
      system.value.timestamps.splice(0, system.value.timestamps.length - maxLength)
      system.value.cpu.splice(0, system.value.cpu.length - maxLength)
      system.value.memory.splice(0, system.value.memory.length - maxLength)
      if (system.value.disk) {
        system.value.disk.splice(0, system.value.disk.length - maxLength)
      }
    }
  }

  // Cleanup method
  function cleanup() {
    stopPolling()
    disconnectWS()
    if (_animationFrameId.value) {
      cancelAnimationFrame(_animationFrameId.value)
      _animationFrameId.value = 0
    }
  }

  return {
    // State
    status,
    health,
    system,
    devices,
    drift,

    // WebSocket state
    wsConnected,
    wsReconnectAttempts,
    lastMessageAt,

    // Getters
    isRealtimeActive,

    // Actions
    fetchStatus,
    fetchHealth,
    fetchSummaries,
    startPolling,
    stopPolling,
    connectWS,
    disconnectWS,
    cleanup,

    // Internal (exposed for testing)
    _ws: ws,
    _timer,
    _animationFrameId,
    // Expose these for testing compatibility
    updateSystemMetrics,
    handleWSMessage,
    scheduleReconnect: () => {
      /* Reconnection is now handled by useWebSocket composable */
    }
  }
})
