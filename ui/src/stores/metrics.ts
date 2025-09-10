import { defineStore } from 'pinia'
import { getMetricsStatus, getMetricsHealth, getSystemMetrics, getDevicesMetrics, getDriftSummary, openMetricsWebSocket } from '@/api/metrics'

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
  wsConnected: boolean
  wsReconnectAttempts: number
  lastMessageAt: number | null
  
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
  _ws: WebSocket | null
  _reconnectTimer: number
  _heartbeatTimer: number
  _animationFrameId: number
}

export const useMetricsStore = defineStore('metrics', {
  state: (): MetricsState => ({
    status: null,
    health: null,
    wsConnected: false,
    wsReconnectAttempts: 0,
    lastMessageAt: null,
    
    system: null,
    devices: null,
    drift: null,
    
    _timer: 0,
    _ws: null,
    _reconnectTimer: 0,
    _heartbeatTimer: 0,
    _animationFrameId: 0
  }),
  
  getters: {
    // Connection status with timeout detection
    isRealtimeActive(): boolean {
      if (!this.wsConnected || !this.lastMessageAt) return false
      return Date.now() - this.lastMessageAt < 60000 // 1 minute timeout
    }
  },
  
  actions: {
    // REST API fallback methods
    async fetchStatus(){ 
      try { 
        this.status = await getMetricsStatus() 
      } catch (e) {
        console.warn('Failed to fetch metrics status:', e)
      }
    },
    
    async fetchHealth(){ 
      try { 
        this.health = await getMetricsHealth() 
      } catch (e) {
        console.warn('Failed to fetch metrics health:', e)
      }
    },
    
    async fetchSummaries(){
      try { 
        const systemData = await getSystemMetrics()
        if (systemData && !this.wsConnected) {
          // Only update from REST if WebSocket is not active
          this.updateSystemMetrics(systemData)
        }
      } catch (e) {
        console.warn('Failed to fetch system metrics:', e)
      }
      
      try { 
        this.devices = await getDevicesMetrics() 
      } catch (e) {
        console.warn('Failed to fetch devices metrics:', e)
      }
      
      try { 
        this.drift = await getDriftSummary() 
      } catch (e) {
        console.warn('Failed to fetch drift summary:', e)
      }
    },

    // Polling for fallback when WebSocket unavailable
    startPolling(intervalMs = 30000){ // Reduced frequency when WS available
      if (this._timer) return
      this._timer = setInterval(() => { 
        if (!this.wsConnected) {
          this.fetchSummaries() 
        }
      }, intervalMs)
      this.fetchSummaries()
    },
    
    stopPolling(){ 
      if (this._timer) { 
        clearInterval(this._timer)
        this._timer = 0 
      }
    },

    // WebSocket connection management with exponential backoff
    connectWS(){
      if (this._ws?.readyState === WebSocket.OPEN) return
      
      this.disconnectWS()
      
      try {
        this._ws = openMetricsWebSocket((msg: WSMessage) => {
          this.handleWSMessage(msg)
        })
        
        this._ws.onopen = () => {
          console.log('WebSocket connected')
          this.wsConnected = true
          this.wsReconnectAttempts = 0
          this.lastMessageAt = Date.now()
          this.stopPolling() // Stop REST polling when WS active
          this.startHeartbeat()
        }
        
        this._ws.onclose = (event) => {
          console.log('WebSocket closed:', event.code, event.reason)
          this.wsConnected = false
          this.lastMessageAt = null
          this.stopHeartbeat()
          this.startPolling() // Resume REST polling
          this.scheduleReconnect(event)
        }
        
        this._ws.onerror = (error) => {
          console.error('WebSocket error:', error)
        }
        
      } catch (error) {
        console.error('Failed to create WebSocket:', error)
        this.scheduleReconnect()
      }
    },
    
    disconnectWS(){
      if (this._ws) {
        this._ws.close(1000, 'Client disconnect')
        this._ws = null
      }
      this.wsConnected = false
      this.stopHeartbeat()
      this.clearReconnectTimer()
    },
    
    // Exponential backoff with jitter
    scheduleReconnect(closeEvent?: CloseEvent){
      this.clearReconnectTimer()
      
      // Don't reconnect on certain close codes
      if (closeEvent && [1000, 1001, 1005].includes(closeEvent.code)) {
        return
      }
      
      const baseDelay = 1000 // 1 second
      const maxDelay = 30000 // 30 seconds
      const backoffFactor = 2
      const jitter = 0.5
      
      let delay = Math.min(baseDelay * Math.pow(backoffFactor, this.wsReconnectAttempts), maxDelay)
      delay = delay * (1 + (Math.random() - 0.5) * jitter) // Add jitter
      
      console.log(`Reconnecting WebSocket in ${Math.round(delay)}ms (attempt ${this.wsReconnectAttempts + 1})`)
      
      this._reconnectTimer = setTimeout(() => {
        this.wsReconnectAttempts++
        this.connectWS()
      }, delay)
    },
    
    clearReconnectTimer(){
      if (this._reconnectTimer) {
        clearTimeout(this._reconnectTimer)
        this._reconnectTimer = 0
      }
    },
    
    // Heartbeat to detect connection issues
    startHeartbeat(){
      this.stopHeartbeat()
      this._heartbeatTimer = setInterval(() => {
        if (this.lastMessageAt && Date.now() - this.lastMessageAt > 45000) {
          console.warn('WebSocket heartbeat timeout, reconnecting...')
          this.connectWS()
        }
      }, 15000) // Check every 15 seconds
    },
    
    stopHeartbeat(){
      if (this._heartbeatTimer) {
        clearInterval(this._heartbeatTimer)
        this._heartbeatTimer = 0
      }
    },

    // Message handling with throttling
    handleWSMessage(msg: WSMessage){
      this.lastMessageAt = Date.now()
      
      // Cancel pending animation frame
      if (this._animationFrameId) {
        cancelAnimationFrame(this._animationFrameId)
      }
      
      // Throttle updates using requestAnimationFrame
      this._animationFrameId = requestAnimationFrame(() => {
        switch (msg.type) {
          case 'status':
            this.status = msg.data
            break
          case 'health':
            this.health = msg.data
            break
          case 'system':
            this.updateSystemMetrics(msg.data)
            break
          case 'devices':
            this.devices = msg.data
            break
          case 'drift':
            this.drift = msg.data
            break
          case 'heartbeat':
            // Just update lastMessageAt
            break
          default:
            console.warn('Unknown WebSocket message type:', msg.type)
        }
      })
    },
    
    // Update system metrics with bounded ring buffer
    updateSystemMetrics(data: any){
      const maxLength = 50 // Configurable window size
      
      if (!this.system) {
        this.system = {
          timestamps: [],
          cpu: [],
          memory: [],
          disk: data.disk ? [] : undefined,
          maxLength
        }
      }
      
      const timestamp = data.timestamp || new Date().toISOString()
      
      // Add new data
      this.system.timestamps.push(timestamp)
      this.system.cpu.push(data.cpu || 0)
      this.system.memory.push(data.memory || 0)
      if (data.disk !== undefined && this.system.disk) {
        this.system.disk.push(data.disk)
      }
      
      // Trim to max length
      if (this.system.timestamps.length > maxLength) {
        this.system.timestamps.splice(0, this.system.timestamps.length - maxLength)
        this.system.cpu.splice(0, this.system.cpu.length - maxLength)
        this.system.memory.splice(0, this.system.memory.length - maxLength)
        if (this.system.disk) {
          this.system.disk.splice(0, this.system.disk.length - maxLength)
        }
      }
    },
    
    // Cleanup method
    cleanup(){
      this.stopPolling()
      this.disconnectWS()
      this.clearReconnectTimer()
      if (this._animationFrameId) {
        cancelAnimationFrame(this._animationFrameId)
        this._animationFrameId = 0
      }
    }
  }
})

