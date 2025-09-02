import { defineStore } from 'pinia'
import { getMetricsStatus, getMetricsHealth, getSystemMetrics, getDevicesMetrics, getDriftSummary, openMetricsWebSocket } from '@/api/metrics'


export const useMetricsStore = defineStore('metrics', {
  state: () => ({ status: null as any, health: null as any, wsConnected: false, system: null as any, devices: null as any, drift: null as any, _timer: 0 as any }),
  actions: {
    async fetchStatus(){ try { this.status = await getMetricsStatus() } catch {} },
    async fetchHealth(){ try { this.health = await getMetricsHealth() } catch {} },
    async fetchSummaries(){
      try { this.system = await getSystemMetrics() } catch {}
      try { this.devices = await getDevicesMetrics() } catch {}
      try { this.drift = await getDriftSummary() } catch {}
    },
    startPolling(intervalMs = 10000){
      if (this._timer) return
      this._timer = setInterval(() => { this.fetchSummaries() }, intervalMs)
      this.fetchSummaries()
    },
    stopPolling(){ if (this._timer) { clearInterval(this._timer); this._timer = 0 } },
    connectWS(){
      const ws = openMetricsWebSocket((_msg)=>{ /* integrate later */ })
      ws.onopen = ()=>{ this.wsConnected = true }
      ws.onclose = ()=>{ this.wsConnected = false }
    }
  }
})

