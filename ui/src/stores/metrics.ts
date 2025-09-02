import { defineStore } from 'pinia'
import { getMetricsStatus, getMetricsHealth, openMetricsWebSocket } from '@/api/metrics'

export const useMetricsStore = defineStore('metrics', {
  state: () => ({ status: null as any, health: null as any, wsConnected: false }),
  actions: {
    async fetchStatus(){ try { this.status = await getMetricsStatus() } catch {} },
    async fetchHealth(){ try { this.health = await getMetricsHealth() } catch {} },
    connectWS(){
      const ws = openMetricsWebSocket((_msg)=>{ /* integrate later */ })
      ws.onopen = ()=>{ this.wsConnected = true }
      ws.onclose = ()=>{ this.wsConnected = false }
    }
  }
})

