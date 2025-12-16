import { defineStore } from 'pinia'
import { getMetricsStatus, getMetricsHealth, getSystemMetrics, getDevicesMetrics, getDriftSummary } from '@/api/metrics'
import { useWebSocket } from '@/composables/useWebSocket'
import { ref, computed } from 'vue'

// WebSocket message types from backend
export interface WSMessage {
  type: 'status' | 'health' | 'system' | 'devices' | 'drift' | 'heartbeat'
  data: any
  timestamp: string
}

// State shape for bounded ring buffers
export interface MetricsState {
  status: any
  health: any

  // Time-series data with bounded ring buffers
  system: {
    timestamps: string[]
    cpu: number[]
    memory: number[]
    disk?: number[]
    maxLength: number
  } | null

  devices: any
  drift: any

  // Internals
  _timer: number
  _animationFrameId: number
}

// Helper to get WebSocket URL
function getMetricsWsUrl(): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return `${protocol}//${host}/api/v1/metrics/ws`
}

export const useMetricsStore = defineStore('metrics', () => {
  // State
  const status = ref<any>(null)
  const health = ref<any>(null)
  const system = ref<MetricsState['system']>(null)
  const devices = ref<any>(null)
  const drift = ref<any>(null)

  // Internals
  const _timer = ref(0)
  const _animationFrameId = ref(0)

  // WebSocket composable
  const ws = useWebSocket<WSMessage>({
    url: getMetricsWsUrl,
    onMessage: handleWSMessage,
    heartbeatInterval: 15000,
    heartbeatTimeout: 45000,
    onOpen: () => {
      stopPolling() // Stop REST polling when WS active
    },
    onClose: () => {
      startPolling() // Resume REST polling
    }
  })

  // Computed - expose WebSocket state
  const wsConnected = computed(() => ws.isConnected.value)
  const wsReconnectAttempts = computed(() => ws.reconnectAttempts.value)
  const lastMessageAt = computed(() => ws.lastMessageAt.value)
  const isRealtimeActive = computed(() => ws.isRealtimeActive.value)

  // Actions - REST API fallback methods
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
      if (systemData && !wsConnected.value) {
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
      if (!wsConnected.value) {
        fetchSummaries()
      }
    }, intervalMs)
    fetchSummaries()
  }

  function stopPolling() {
    if (_timer.value) {
      clearInterval(_timer.value)
      _timer.value = 0
    }
  }

  // WebSocket connection management (delegated to composable)
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

    // Computed
    wsConnected,
    wsReconnectAttempts,
    lastMessageAt,
    isRealtimeActive,

    // Actions
    fetchStatus,
    fetchHealth,
    fetchSummaries,
    startPolling,
    stopPolling,
    connectWS,
    disconnectWS,
    updateSystemMetrics,
    cleanup
  }
})

